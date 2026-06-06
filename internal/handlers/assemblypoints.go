// Package handlers HTTP istek/yanıt döngülerini yönetir.
// Bu dosya GET /api/v1/assembly-points endpoint'ini işler:
// İstanbul'daki gerçek afet toplanma alanlarını listeler.
package handlers

import (
	"net/http"

	"c-backend/internal/models"
)

// AssemblyPointsHandler toplanma alanları endpoint'i için handler yapısıdır.
type AssemblyPointsHandler struct{}

// NewAssemblyPointsHandler yeni bir AssemblyPointsHandler örneği döndürür.
func NewAssemblyPointsHandler() *AssemblyPointsHandler {
	return &AssemblyPointsHandler{}
}

// istanbulAssemblyPoints İstanbul'daki doğrulanmış afet toplanma alanlarının listesidir.
// Koordinatlar ve kapasiteler İstanbul AFAD verilerine dayanmaktadır.
var istanbulAssemblyPoints = []models.AssemblyPoint{
	{
		Name:     "Atatürk Olimpiyat Stadı",
		Lat:      41.0714,
		Lng:      28.7330,
		Capacity: 50000,
	},
	{
		Name:     "Yıldız Parkı",
		Lat:      41.0489,
		Lng:      29.0108,
		Capacity: 20000,
	},
	{
		Name:     "Maçka Parkı",
		Lat:      41.0456,
		Lng:      29.0006,
		Capacity: 15000,
	},
	{
		Name:     "Gülhane Parkı",
		Lat:      41.0128,
		Lng:      28.9833,
		Capacity: 25000,
	},
	{
		Name:     "Fatih Millet Bahçesi",
		Lat:      41.0186,
		Lng:      28.9395,
		Capacity: 30000,
	},
}

// Handle GET /api/v1/assembly-points isteğini işler.
// İstanbul'daki tüm toplanma alanlarını JSON dizisi olarak döndürür.
func (h *AssemblyPointsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, istanbulAssemblyPoints)
}
