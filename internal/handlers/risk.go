// Package handlers HTTP istek/yanıt döngülerini yönetir.
// Bu dosya POST /api/v1/risk endpoint'ini işler:
// koordinat alır → sokak görüntüsü çeker → blur uygular → risk analizi yapar → sonuç döndürür.
package handlers

import (
	"encoding/json"
	"net/http"

	"c-backend/internal/models"
	"c-backend/internal/services"
)

// RiskHandler risk analizi endpoint'i için gerekli servisleri barındırır.
type RiskHandler struct {
	fetchStreetView   func(lat, lng float64) ([]byte, error)
	blurSensitiveData func(imageBytes []byte) ([]byte, error)
	analyzeRisk       func(imageBytes []byte) (int, error)
}

// NewRiskHandler bağımlılıkları (dependency injection) ile yeni bir RiskHandler döndürür.
func NewRiskHandler() *RiskHandler {
	return &RiskHandler{
		fetchStreetView:   services.FetchStreetView,
		blurSensitiveData: services.BlurSensitiveData,
		analyzeRisk:       services.AnalyzeRisk,
	}
}

// Handle POST /api/v1/risk isteğini işler.
// İstek gövdesinden koordinatları okur ve şu sırayla çalışır:
//  1. Mapillary'den sokak görüntüsü çek
//  2. Görüntü üzerinde KVKK blur uygula
//  3. Wiro AI ile risk skorunu hesapla
//  4. RiskResponse JSON olarak döndür
func (h *RiskHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// İstek gövdesini RiskRequest yapısına çözümle
	var req models.RiskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Geçersiz istek gövdesi: "+err.Error())
		return
	}

	// Koordinat doğrulaması
	if req.Lat == 0 && req.Lng == 0 {
		writeError(w, http.StatusBadRequest, "Lat ve Lng alanları zorunludur")
		return
	}

	// 1. Adım: Mapillary'den sokak görüntüsünü indir
	imageBytes, err := h.fetchStreetView(req.Lat, req.Lng)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Sokak görüntüsü alınamadı: "+err.Error())
		return
	}

	// 2. Adım: KVKK gereği yüz ve plaka blur uygula
	blurredBytes, err := h.blurSensitiveData(imageBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Görüntü blur işlemi başarısız: "+err.Error())
		return
	}

	// 3. Adım: Wiro AI ile enkaz risk skorunu hesapla
	score, err := h.analyzeRisk(blurredBytes)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Risk analizi başarısız: "+err.Error())
		return
	}

	// 4. Adım: Risk seviyesini belirleyip yanıt döndür
	resp := models.NewRiskResponse(score)
	writeJSON(w, http.StatusOK, resp)
}
