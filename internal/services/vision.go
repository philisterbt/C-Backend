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
// Tüm talimatlar Türkçe verilir; model yalnızca Türkçe comment ve öneriler üretmelidir.
const riskAnalysisPrompt = `Sen bir deprem enkaz riski uzmanısın. Bu sokak görüntüsünü analiz et: sokak genişliğini ve iki yandaki bina yüksekliklerini tahmin et. Olası bir deprem senaryosunda enkaz (bina çöküşü) riskini 0-100 arasında değerlendir.

Yanıtını YALNIZCA aşağıdaki JSON formatında ve TEK SATIR halinde ver. Başka hiçbir metin ekleme:
{"risk_score": <0-100 arası sayı>, "comment": "<Türkçe yorum>", "recommendations": ["<Türkçe öneri 1>", "<Türkçe öneri 2>", "<Türkçe öneri 3>"]}

ZORUNLU KURALLAR:
1. DİL: "comment" alanı ve "recommendations" dizisindeki HER madde kesinlikle TÜRKÇE olmalıdır. İngilizce kelime, cümle veya karışık dil KULLANMA.
2. comment: Bu bölgenin deprem enkaz riski hakkında 2-3 cümlelik Türkçe açıklama yaz (sokak darlığı, bina yüksekliği, tahliye zorluğu vb.).
3. recommendations: Tam olarak 3 adet, bu bölgeye özel, uygulanabilir Türkçe öneri yaz (ör. bina güçlendirme, tahliye güzergahı, toplanma alanı, dar geçitler).
4. JSON dışında markdown, kod bloğu veya açıklama ekleme. String değerlerin içine satır sonu koyma.

Örnek (formatı birebir takip et, içeriği görüntüye göre değiştir):
{"risk_score": 65, "comment": "Sokak oldukça dar ve her iki yanda yüksek katlı binalar bulunuyor. Depremde oluşacak enkaz tahliyeyi zorlaştırabilir ve çıkış yollarını kapatabilir.", "recommendations": ["Binaların deprem güçlendirmesi için yapı denetimi yaptırın", "Dar sokaklar için alternatif geniş tahliye güzergahı belirleyin", "En yakın toplanma alanına yürüme mesafesini önceden öğrenin"]}`

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

// riskAnalysisOutput modelin döndürdüğü JSON'u temsil eder:
// risk skoru, bölge yorumu ve öneriler.
type riskAnalysisOutput struct {
	RiskScore       int      `json:"risk_score"`
	Comment         string   `json:"comment"`
	Recommendations []string `json:"recommendations"`
}

// RiskAnalysis Wiro AI analizinin nihai sonucunu çağıran katmana taşır.
type RiskAnalysis struct {
	Score           int      // 0-100 arası enkaz risk skoru
	Comment         string   // Bölgenin deprem riski hakkında Türkçe yorum
	Recommendations []string // Riski azaltmaya yönelik Türkçe öneriler
}

// httpClient Wiro AI istekleri için makul timeout'a sahip paylaşımlı istemcidir.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// AnalyzeRisk verilen görüntü baytlarını Wiro AI moondream3-preview/query modeline
// göndererek deprem senaryosunda bina yüksekliği ve sokak genişliğine göre hesaplanan
// enkaz risk skorunu, bölge yorumunu ve önerileri döndürür.
func AnalyzeRisk(imageBytes []byte) (RiskAnalysis, error) {
	// Ortam değişkeninden Wiro AI API anahtarını oku
	apiKey := os.Getenv("WIRO_API_KEY")
	if apiKey == "" {
		return RiskAnalysis{}, fmt.Errorf("WIRO_API_KEY ortam değişkeni tanımlı değil")
	}

	// 1. Adım: modeli çalıştır ve task token'ını al
	taskToken, err := runWiroModel(imageBytes, apiKey)
	if err != nil {
		return RiskAnalysis{}, err
	}

	// 2. Adım: task tamamlanana kadar sonucu sorgula (polling)
	answerText, err := pollWiroTask(taskToken, apiKey)
	if err != nil {
		return RiskAnalysis{}, err
	}

	// 3. Adım: dönen metinden risk skoru, yorum ve önerileri çıkar
	analysis, err := parseRiskAnalysis(answerText)
	if err != nil {
		return RiskAnalysis{}, err
	}

	// 4. Adım: bozuk Türkçe karakterleri ve yazım hatalarını düzelt
	analysis = polishRiskAnalysis(apiKey, analysis)

	return analysis, nil
}

// polishRiskAnalysis AI çıktısındaki Türkçe metinleri düzeltir.
// Bozukluk tespit edilirse önce metin modeli, ardından yerel kurallar uygulanır.
func polishRiskAnalysis(apiKey string, analysis RiskAnalysis) RiskAnalysis {
	// Ciddi bozulma varsa metin modeli ile yeniden yaz (WIRO_TEXT_POLISH=false ile kapatılabilir)
	if os.Getenv("WIRO_TEXT_POLISH") != "false" && NeedsLLMPolish(analysis.Comment, analysis.Recommendations) {
		if polished, err := polishWithTextModel(apiKey, analysis); err == nil {
			analysis = polished
		}
	}

	// Yerel düzeltme (hızlı, her zaman son adım)
	comment, recs := PolishTurkishTexts(analysis.Comment, analysis.Recommendations)
	analysis.Comment = comment
	analysis.Recommendations = recs

	return analysis
}

// polishTurkishPrompt metin modeline gönderilecek yazım düzeltme talimatıdır.
func buildPolishPrompt(analysis RiskAnalysis) string {
	draft, _ := json.Marshal(map[string]any{
		"risk_score":      analysis.Score,
		"comment":         analysis.Comment,
		"recommendations": analysis.Recommendations,
	})
	return `Sen Türkçe yazım düzeltme uzmanısın. Aşağıdaki JSON'daki Türkçe metinlerdeki yazım hatalarını, bozuk karakterleri (ş/ç/ğ/ü/ö/ı karışıklığı) ve gereksiz harf tekrarlarını düzelt.

KURALLAR:
- risk_score sayısını AYNEN koru, değiştirme.
- comment ve recommendations içeriğini anlamı koruyarak düzgün Türkçe ile yeniden yaz.
- Türkçe karakterleri doğru kullan: ç, ğ, ı, ö, ş, ü
- Sadece TEK SATIR geçerli JSON döndür, başka metin ekleme.

Girdi JSON:
` + string(draft) + `

Yanıt formatı:
{"risk_score": <aynı sayı>, "comment": "<düzeltilmiş Türkçe>", "recommendations": ["<öneri 1>", "<öneri 2>", "<öneri 3>"]}`
}

// polishWithTextModel Wiro AI metin modeli ile yorum ve önerileri yeniden yazar.
// WIRO_TEXT_MODEL ortam değişkeni "owner/slug" formatında olmalıdır (ör. openai/gpt-oss-20b).
// Model erişilemezse hata döner; çağıran yerel düzeltilmiş metni kullanmaya devam eder.
func polishWithTextModel(apiKey string, analysis RiskAnalysis) (RiskAnalysis, error) {
	modelPath := strings.TrimSpace(os.Getenv("WIRO_TEXT_MODEL"))
	if modelPath == "" {
		// Varsayılan metin modeli; erişim yoksa yerel düzeltme yeterli kalır
		modelPath = "openai/gpt-oss-20b"
	}

	parts := strings.SplitN(modelPath, "/", 2)
	if len(parts) != 2 {
		return analysis, fmt.Errorf("WIRO_TEXT_MODEL geçersiz format (owner/slug bekleniyor)")
	}

	runURL := fmt.Sprintf("https://api.wiro.ai/v1/Run/%s/%s", parts[0], parts[1])
	taskToken, err := runWiroTextPrompt(runURL, apiKey, buildPolishPrompt(analysis))
	if err != nil {
		return analysis, err
	}

	answerText, err := pollWiroTask(taskToken, apiKey)
	if err != nil {
		return analysis, err
	}

	polished, err := parseRiskAnalysis(answerText)
	if err != nil {
		return analysis, err
	}

	// Skor orijinal analizden korunur (metin modeli skoru değiştirmesin)
	polished.Score = analysis.Score
	polished.Comment, polished.Recommendations = PolishTurkishTexts(
		polished.Comment, polished.Recommendations,
	)

	return polished, nil
}

// runWiroTextPrompt görüntü gerektirmeyen metin modellerine JSON isteği gönderir.
func runWiroTextPrompt(runURL, apiKey, prompt string) (string, error) {
	bodyBytes, err := json.Marshal(map[string]string{
		"prompt": prompt,
	})
	if err != nil {
		return "", fmt.Errorf("Wiro metin isteği oluşturulamadı: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, runURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("Wiro metin HTTP isteği oluşturulamadı: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Wiro metin isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Wiro metin yanıtı okunamadı: %w", err)
	}

	var runResp wiroRunResponse
	if err := json.Unmarshal(respBytes, &runResp); err != nil {
		return "", fmt.Errorf("Wiro metin yanıtı çözümlenemedi: %w", err)
	}
	if !runResp.Result || runResp.SocketAccessToken == "" {
		return "", fmt.Errorf("Wiro metin modeli başarısız: %s", formatWiroErrors(runResp.Errors, string(respBytes)))
	}

	return runResp.SocketAccessToken, nil
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

// parseRiskAnalysis modelin metin yanıtından risk skoru, bölge yorumu ve
// önerileri ayıklar; skoru 0-100 aralığına sınırlar.
func parseRiskAnalysis(answerText string) (RiskAnalysis, error) {
	jsonStr := extractJSON(answerText)
	if jsonStr == "" {
		return RiskAnalysis{}, fmt.Errorf("model yanıtında JSON bulunamadı (ham: %s)", answerText)
	}

	// Modelin çıktısını temizle: bazı modeller string değerlerinin içine ham
	// (escape edilmemiş) newline/tab/satır başı koyar ve bu, standart JSON
	// ayrıştırmasını bozar. Ham kontrol karakterlerini boşluğa çeviriyoruz.
	jsonStr = sanitizeModelJSON(jsonStr)

	var output riskAnalysisOutput
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		return RiskAnalysis{}, fmt.Errorf("risk analizi çözümlenemedi (ham: %s): %w", jsonStr, err)
	}

	// Skoru 0-100 aralığına sınırla
	score := output.RiskScore
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return RiskAnalysis{
		Score:           score,
		Comment:         strings.TrimSpace(output.Comment),
		Recommendations: output.Recommendations,
	}, nil
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

// sanitizeModelJSON modelin döndürdüğü JSON metnini json.Unmarshal'dan önce temizler.
// Bazı modeller string değerlerinin içine ham (escape edilmemiş) newline, satır başı
// veya tab gibi kontrol karakterleri koyar; standart JSON bunları string içinde kabul
// etmediği için ayrıştırma başarısız olur. Olası markdown kod bloğu işaretlerini kaldırır
// ve tüm ham kontrol karakterlerini (0x00-0x1F) boşluğa çevirerek geçerli JSON üretir.
//
// Not: Bu işlem yalnızca GERÇEK kontrol karakterlerini hedefler; modelin zaten doğru
// şekilde escape ettiği "\n" gibi iki karakterli diziler olduğu gibi korunur.
func sanitizeModelJSON(s string) string {
	// Markdown kod bloğu işaretlerini kaldır
	s = strings.ReplaceAll(s, "```json", "")
	s = strings.ReplaceAll(s, "```", "")

	// Ham kontrol karakterlerini boşluğa çevir
	s = strings.Map(func(r rune) rune {
		if r < 0x20 {
			return ' '
		}
		return r
	}, s)

	return strings.TrimSpace(s)
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
