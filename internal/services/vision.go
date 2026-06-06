// Package services uygulamanın iş mantığını (business logic) barındırır.
// Bu dosya Wiro AI API aracılığıyla sokak görüntüsünden deprem enkaz riski
// analizi yapar ve 0-100 arasında bir risk skoru döndürür.
package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// wiroAPIURL Wiro AI API'nin çalıştırma (run) endpoint'idir.
const wiroAPIURL = "https://api.wiro.ai/v1/Run"

// wiroModel kullanılacak Wiro AI modelinin tam adıdır.
const wiroModel = "moondream/moondream3"

// riskAnalysisPrompt modele gönderilecek analiz sorusudur.
// Model yalnızca JSON formatında {"risk_score": <sayı>} döndürmesi istenmektedir.
const riskAnalysisPrompt = `Analyze this street image. Estimate the street width and the height of buildings on both sides. Based on these, return a debris risk score between 0 and 100 for a potential earthquake scenario. Respond only in JSON: {"risk_score": <number>}`

// wiroRequest Wiro AI API'ye gönderilecek istek gövdesini temsil eder.
type wiroRequest struct {
	Model string     `json:"model"`
	Input wiroInput  `json:"input"`
}

// wiroInput modele iletilecek girdi alanlarını içerir.
type wiroInput struct {
	Image    string `json:"image"`
	Question string `json:"question"`
}

// wiroResponse Wiro AI API'den dönen yanıt zarfını temsil eder.
type wiroResponse struct {
	Output string `json:"output"` // Modelin metin çıktısı
}

// riskScoreOutput modelin döndürdüğü JSON içindeki risk skorunu tutar.
type riskScoreOutput struct {
	RiskScore int `json:"risk_score"`
}

// AnalyzeRisk verilen görüntü baytlarını Wiro AI moondream3 modeline göndererek
// deprem senaryosunda bina yüksekliği ve sokak genişliğine göre hesaplanan
// 0-100 arası enkaz risk skorunu döndürür.
func AnalyzeRisk(imageBytes []byte) (int, error) {
	// Ortam değişkeninden Wiro AI API anahtarını oku
	apiKey := os.Getenv("WIRO_API_KEY")
	if apiKey == "" {
		return 0, fmt.Errorf("WIRO_API_KEY ortam değişkeni tanımlı değil")
	}

	// Görüntüyü Base64 formatına dönüştür
	encodedImage := base64.StdEncoding.EncodeToString(imageBytes)

	// İstek gövdesini hazırla
	requestBody := wiroRequest{
		Model: wiroModel,
		Input: wiroInput{
			Image:    encodedImage,
			Question: riskAnalysisPrompt,
		},
	}

	// İstek gövdesini JSON'a çevir
	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return 0, fmt.Errorf("Wiro AI istek gövdesi oluşturulamadı: %w", err)
	}

	// HTTP POST isteği oluştur
	req, err := http.NewRequest(http.MethodPost, wiroAPIURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return 0, fmt.Errorf("Wiro AI HTTP isteği oluşturulamadı: %w", err)
	}

	// Gerekli başlıkları ayarla
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	// İsteği gönder
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("Wiro AI API isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	// Beklenmedik HTTP durum kodu kontrolü
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Wiro AI API beklenmeyen durum kodu döndürdü: %d", resp.StatusCode)
	}

	// Yanıt gövdesini oku
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("Wiro AI yanıtı okunamadı: %w", err)
	}

	// Önce API zarfını parse et (output alanı modelin metin çıktısını içerir)
	var wiroResp wiroResponse
	if err := json.Unmarshal(respBytes, &wiroResp); err != nil {
		return 0, fmt.Errorf("Wiro AI yanıt zarfı çözümlenemedi: %w", err)
	}

	// Modelin çıktısından risk_score JSON'unu çıkar
	// Çıktı bazen ek metin içerebileceğinden JSON bloğunu ayıkla
	jsonStr := extractJSON(wiroResp.Output)
	if jsonStr == "" {
		// Zarf output alanı boşsa tüm yanıtı dene
		jsonStr = string(respBytes)
	}

	var scoreOutput riskScoreOutput
	if err := json.Unmarshal([]byte(jsonStr), &scoreOutput); err != nil {
		return 0, fmt.Errorf("risk_score değeri çözümlenemedi (ham yanıt: %s): %w", jsonStr, err)
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
