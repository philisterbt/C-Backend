// Package models uygulamada kullanılan veri yapılarını (data transfer objects) tanımlar.
// Bu dosya deprem enkaz riski analizi ile ilgili istek ve yanıt modellerini içerir.
package models

import "time"

// RiskRequest istemciden gelen risk analizi isteğini temsil eder.
// Lat ve Lng alanları analize konu olan konumun koordinatlarıdır.
type RiskRequest struct {
	Lat float64 `json:"lat"` // Enlem (latitude), -90 ile 90 arasında
	Lng float64 `json:"lng"` // Boylam (longitude), -180 ile 180 arasında
}

// RiskResponse istemciye döndürülen risk analizi sonucunu temsil eder.
type RiskResponse struct {
	Score      int       `json:"score"`       // 0-100 arası enkaz risk skoru
	Level      string    `json:"level"`       // Risk seviyesi: DÜŞÜK, ORTA veya YÜKSEK
	AnalyzedAt time.Time `json:"analyzed_at"` // Analizin gerçekleştirildiği zaman damgası
}

// NewRiskResponse verilen skora göre Level alanını otomatik dolduran
// bir RiskResponse nesnesi oluşturur ve döndürür.
//
// Risk seviyesi belirleme kuralı:
//
//	0-30  → DÜŞÜK
//	31-60 → ORTA
//	61-100 → YÜKSEK
func NewRiskResponse(score int) RiskResponse {
	return RiskResponse{
		Score:      score,
		Level:      determineLevel(score),
		AnalyzedAt: time.Now().UTC(),
	}
}

// determineLevel skora göre Türkçe risk seviyesi döndürür.
func determineLevel(score int) string {
	switch {
	case score <= 30:
		return "DÜŞÜK"
	case score <= 60:
		return "ORTA"
	default:
		return "YÜKSEK"
	}
}
