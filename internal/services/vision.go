// Package services uygulamanın iş mantığını (business logic) barındırır.
// Bu dosya Wiro AI moondream3-preview/query modeli aracılığıyla sokak görüntüsünden
// deprem enkaz riski analizi yapar ve 0-100 arasında bir risk skoru döndürür.
//
// Wiro AI asenkron çalışır:
//  1. POST /v1/Run/moondream3-preview/query  → görsel + soru gönderilir, taskid + socketaccesstoken döner
//  2. POST /v1/Task/Detail                   → task tamamlanana kadar periyodik sorgulanır (polling)
//  3. Sonuç metni debugoutput / outputs[].content alanından okunur
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// wiroRunURL Wiro AI moondream3-preview/query modelini çalıştıran endpoint'tir.
	wiroRunURL = "https://api.wiro.ai/v1/Run/moondream3-preview/query"
	// wiroTaskDetailURL bir task'ın durumunu ve çıktısını sorgulayan endpoint'tir.
	wiroTaskDetailURL = "https://api.wiro.ai/v1/Task/Detail"
	// wiroMaxPolls task tamamlanması için yapılacak maksimum sorgu sayısıdır.
	wiroMaxPolls = 30
	// wiroPollInterval her sorgu arasındaki bekleme süresidir.
	wiroPollInterval = 2 * time.Second
)

// riskAnalysisPrompt modele gönderilecek analiz sorusudur.
// Model yalnızca {"risk_score": <sayı>} formatında JSON döndürmesi için yönlendirilir.
const riskAnalysisPrompt = `Analyze this street image. Estimate the street width and the height of buildings on both sides. Based on these, return a debris risk score between 0 and 100 for a potential earthquake scenario. Respond ONLY with valid JSON in this exact format and nothing else: {"risk_score": <number>}`

// wiroError Wiro AI yanıtlarındaki hata yapısını temsil eder.
type wiroError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// wiroRunResponse /Run endpoint'inin döndürdüğü yanıtı temsil eder.
type wiroRunResponse struct {
	Result            bool        `json:"result"`
	TaskID            string      `json:"taskid"`
	SocketAccessToken string      `json:"socketaccesstoken"`
	Errors            []wiroError `json:"errors"`
}

// wiroTaskDetailResponse /Task/Detail endpoint'inin döndürdüğü yanıtı temsil eder.
type wiroTaskDetailResponse struct {
	Result   bool        `json:"result"`
	TaskList []wiroTask  `json:"tasklist"`
	Errors   []wiroError `json:"errors"`
}

// wiroTask tek bir task'ın durum ve çıktı bilgilerini tutar.
type wiroTask struct {
	Status      string       `json:"status"`      // Task durumu (örn. task_postprocess_end)
	Pexit       string       `json:"pexit"`       // İşlem çıkış kodu ("0" = başarılı)
	DebugOutput string       `json:"debugoutput"` // LLM modellerinde birleştirilmiş düz metin yanıt
	Outputs     []wiroOutput `json:"outputs"`     // Yapılandırılmış çıktı listesi
}

// wiroOutput bir task çıktısını temsil eder; LLM modellerinde content alanı doludur.
type wiroOutput struct {
	ContentType string          `json:"contenttype"`
	Content     json.RawMessage `json:"content"`
}

// wiroOutputContent LLM çıktısının yapılandırılmış içeriğini temsil eder.
// answer alanı bazı modellerde dizi, bazılarında düz metin olabileceğinden esnek tutulmuştur.
type wiroOutputContent struct {
	Answer json.RawMessage `json:"answer"`
	Raw    string          `json:"raw"`
}

// riskScoreOutput modelin döndürdüğü JSON içindeki risk skorunu tutar.
type riskScoreOutput struct {
	RiskScore int `json:"risk_score"`
}

// httpClient Wiro AI istekleri için makul timeout'a sahip paylaşımlı istemcidir.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// AnalyzeRisk verilen görüntü baytlarını Wiro AI moondream3-preview/query modeline
// göndererek deprem senaryosunda bina yüksekliği ve sokak genişliğine göre hesaplanan
// 0-100 arası enkaz risk skorunu döndürür.
func AnalyzeRisk(imageBytes []byte) (int, error) {
	// Ortam değişkeninden Wiro AI API anahtarını oku
	apiKey := os.Getenv("WIRO_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("WIRO_API_KEY ortam değişkeni tanımlı değil")
	}

	// 1. Adım: modeli çalıştır ve task token'ını al
	taskToken, err := runWiroModel(imageBytes, apiKey)
	if err != nil {
		return 0, err
	}

	// 2. Adım: task tamamlanana kadar sonucu sorgula (polling)
	answerText, err := pollWiroTask(taskToken, apiKey)
	if err != nil {
		return 0, err
	}

	// 3. Adım: dönen metinden risk_score değerini çıkar
	score, err := parseRiskScore(answerText)
	if err != nil {
		return 0, err
	}

	return score, nil
}

// runWiroModel görüntüyü multipart/form-data olarak Wiro AI'ye yükler ve
// asenkron task için kullanılacak socketaccesstoken değerini döndürür.
func runWiroModel(imageBytes []byte, apiKey string) (string, error) {
	// Multipart form gövdesini hazırla
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Görüntüyü inputImage dosya alanı olarak ekle
	filePart, err := writer.CreateFormFile("inputImage", "street.jpg")
	if err != nil {
		return "", fmt.Errorf("Wiro AI form dosyası oluşturulamadı: %w", err)
	}
	if _, err := filePart.Write(imageBytes); err != nil {
		return "", fmt.Errorf("Wiro AI görüntü verisi yazılamadı: %w", err)
	}

	// Analiz sorusunu ve üretim parametrelerini ekle
	_ = writer.WriteField("prompt", riskAnalysisPrompt)
	_ = writer.WriteField("temperature", "0.2")
	_ = writer.WriteField("top_p", "0.95")

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("Wiro AI form gövdesi kapatılamadı: %w", err)
	}

	// POST isteğini oluştur
	req, err := http.NewRequest(http.MethodPost, wiroRunURL, &requestBody)
	if err != nil {
		return "", fmt.Errorf("Wiro AI Run isteği oluşturulamadı: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-api-key", apiKey)

	// İsteği gönder
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Wiro AI Run isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Wiro AI Run yanıtı okunamadı: %w", err)
	}

	var runResp wiroRunResponse
	if err := json.Unmarshal(bodyBytes, &runResp); err != nil {
		return "", fmt.Errorf("Wiro AI Run yanıtı çözümlenemedi (ham: %s): %w", string(bodyBytes), err)
	}

	// Başarısızlık veya token eksikliği kontrolü
	if !runResp.Result || runResp.SocketAccessToken == "" {
		return "", fmt.Errorf("Wiro AI Run başarısız: %s", formatWiroErrors(runResp.Errors, string(bodyBytes)))
	}

	return runResp.SocketAccessToken, nil
}

// pollWiroTask task tamamlanana kadar /Task/Detail endpoint'ini periyodik sorgular
// ve modelin metin yanıtını döndürür.
func pollWiroTask(taskToken, apiKey string) (string, error) {
	// Sorgu gövdesi: tasktoken (socketaccesstoken)
	bodyBytes, err := json.Marshal(map[string]string{"tasktoken": taskToken})
	if err != nil {
		return "", fmt.Errorf("Wiro AI Task/Detail gövdesi oluşturulamadı: %w", err)
	}

	for attempt := 0; attempt < wiroMaxPolls; attempt++ {
		// İlk denemeden önce de kısa bekleme yaparak modele işlem zamanı tanı
		time.Sleep(wiroPollInterval)

		req, err := http.NewRequest(http.MethodPost, wiroTaskDetailURL, bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("Wiro AI Task/Detail isteği oluşturulamadı: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", apiKey)

		resp, err := httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("Wiro AI Task/Detail isteği başarısız: %w", err)
		}

		respBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return "", fmt.Errorf("Wiro AI Task/Detail yanıtı okunamadı: %w", readErr)
		}

		var detail wiroTaskDetailResponse
		if err := json.Unmarshal(respBytes, &detail); err != nil {
			return "", fmt.Errorf("Wiro AI Task/Detail yanıtı çözümlenemedi (ham: %s): %w", string(respBytes), err)
		}

		// Henüz task listesi gelmediyse tekrar dene
		if len(detail.TaskList) == 0 {
			continue
		}

		task := detail.TaskList[0]

		// Task tamamlanmadıysa bir sonraki sorguya geç
		if task.Status != "task_postprocess_end" {
			continue
		}

		// Task tamamlandı; çıkış kodunu kontrol et
		if task.Pexit != "0" {
			return "", fmt.Errorf("Wiro AI task hata ile sonuçlandı (pexit=%s)", task.Pexit)
		}

		// Metin yanıtını çıkar ve döndür
		answer := extractAnswer(task)
		if answer == "" {
			return "", fmt.Errorf("Wiro AI task tamamlandı ancak metin çıktısı bulunamadı")
		}
		return answer, nil
	}

	return "", fmt.Errorf("Wiro AI task zaman aşımına uğradı (%d sorgu sonrası tamamlanmadı)", wiroMaxPolls)
}

// extractAnswer tamamlanmış bir task'tan modelin metin yanıtını çıkarır.
// Önce yapılandırılmış outputs içeriği, ardından düz metin debugoutput denenir.
func extractAnswer(task wiroTask) string {
	// Yapılandırılmış çıktıdan answer/raw alanlarını dene
	for _, out := range task.Outputs {
		if len(out.Content) == 0 {
			continue
		}
		var content wiroOutputContent
		if err := json.Unmarshal(out.Content, &content); err == nil {
			// answer alanı dizi veya düz metin olabilir
			if answer := flattenJSONText(content.Answer); answer != "" {
				return answer
			}
			if content.Raw != "" {
				return content.Raw
			}
		}
	}

	// Son çare: birleştirilmiş düz metin debugoutput
	return strings.TrimSpace(task.DebugOutput)
}

// flattenJSONText answer alanını (dizi veya düz metin olabilir) tek bir metne dönüştürür.
func flattenJSONText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Önce düz metin olarak dene
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
	}
	// Ardından metin dizisi olarak dene ve birleştir
	var asSlice []string
	if err := json.Unmarshal(raw, &asSlice); err == nil {
		return strings.TrimSpace(strings.Join(asSlice, ""))
	}
	return ""
}

// parseRiskScore modelin metin yanıtından {"risk_score": <sayı>} değerini ayıklar
// ve 0-100 aralığına sınırlanmış tamsayı skoru döndürür.
func parseRiskScore(answerText string) (int, error) {
	jsonStr := extractJSON(answerText)
	if jsonStr == "" {
		return 0, fmt.Errorf("model yanıtında JSON bulunamadı (ham: %s)", answerText)
	}

	var scoreOutput riskScoreOutput
	if err := json.Unmarshal([]byte(jsonStr), &scoreOutput); err != nil {
		return 0, fmt.Errorf("risk_score değeri çözümlenemedi (ham: %s): %w", jsonStr, err)
	}

	// Skoru 0-100 aralığına sınırla
	score := scoreOutput.RiskScore
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, nil
}

// extractJSON verilen metinden ilk JSON nesnesini ({...}) çıkarır.
// Model çıktısı JSON öncesinde veya sonrasında açıklayıcı metin içerebilir.
func extractJSON(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return text[start : end+1]
}

// formatWiroErrors Wiro AI hata listesini okunabilir tek bir metne dönüştürür.
func formatWiroErrors(errs []wiroError, raw string) string {
	if len(errs) == 0 {
		return raw
	}
	messages := make([]string, 0, len(errs))
	for _, e := range errs {
		messages = append(messages, e.Message)
	}
	return strings.Join(messages, "; ")
}
