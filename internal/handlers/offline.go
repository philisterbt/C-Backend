// Package handlers HTTP istek/yanıt döngülerini yönetir.
// Bu dosya offline harita indirme ve acil durum veri paketi endpoint'lerini işler.
package handlers

import (
	"net/http"
	"strings"

	"c-backend/internal/services"
)

// OfflineHandler offline veri paketi endpoint'leri için handler yapısıdır.
type OfflineHandler struct{}

// NewOfflineHandler yeni bir OfflineHandler örneği döndürür.
func NewOfflineHandler() *OfflineHandler {
	return &OfflineHandler{}
}

// HandleRegions GET /api/v1/offline/regions isteğini işler.
// İndirilebilir harita bölgelerinin listesini döndürür.
func (h *OfflineHandler) HandleRegions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, services.GetOfflineRegions())
}

// HandleBundle GET /api/v1/offline/bundle/{region_id} isteğini işler.
// Belirtilen bölge için tam offline veri paketini döndürür.
func (h *OfflineHandler) HandleBundle(w http.ResponseWriter, r *http.Request) {
	// Go 1.22+ path parametresi: /api/v1/offline/bundle/{region_id}
	regionID := services.NormalizeRegionID(r.PathValue("region_id"))
	if regionID == "" {
		writeError(w, http.StatusBadRequest, "region_id parametresi zorunludur")
		return
	}

	bundle, ok := services.GetOfflineBundle(regionID)
	if !ok {
		writeError(w, http.StatusNotFound, "Bölge bulunamadı: "+regionID)
		return
	}

	writeJSON(w, http.StatusOK, bundle)
}

// HandleAssemblyGeoJSON GET /api/v1/offline/assembly-points.geojson isteğini işler.
// Toplanma alanlarını GeoJSON FeatureCollection olarak döndürür (offline harita katmanı).
func (h *OfflineHandler) HandleAssemblyGeoJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/geo+json")
	writeJSON(w, http.StatusOK, services.GetAssemblyPointsGeoJSON())
}

// HandleBundleLegacy eski path formatı için yönlendirme desteği.
// GET /api/v1/offline/bundles?id=istanbul-merkez
func (h *OfflineHandler) HandleBundleLegacy(w http.ResponseWriter, r *http.Request) {
	regionID := services.NormalizeRegionID(r.URL.Query().Get("id"))
	if regionID == "" {
		writeError(w, http.StatusBadRequest, "id parametresi zorunludur")
		return
	}
	r.SetPathValue("region_id", regionID)
	h.HandleBundle(w, r)
}

// AvailableRegionIDs mevcut bölge kimliklerini virgülle ayrılmış string olarak döndürür (hata mesajları için).
func AvailableRegionIDs() string {
	regions := services.GetOfflineRegions()
	ids := make([]string, len(regions))
	for i, reg := range regions {
		ids[i] = reg.ID
	}
	return strings.Join(ids, ", ")
}
