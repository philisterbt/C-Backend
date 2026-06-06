// Package services uygulamanın iş mantığını (business logic) barındırır.
// Bu dosya offline harita bölgeleri ve acil durum veri paketlerini sağlar.
package services

import (
	"strings"
	"time"

	"c-backend/internal/models"
)

// offlineRegions indirilebilir İstanbul bölgelerinin sabit listesidir.
// Mobil uygulama bu bölgeleri seçerek harita tile'larını önceden indirir.
var offlineRegions = []models.OfflineRegion{
	{
		ID: "istanbul-merkez", Name: "İstanbul Merkez",
		Description: "Taksim, Şişli, Beşiktaş, Fatih merkez bölgeleri",
		MinLat: 41.005, MinLng: 28.920, MaxLat: 41.070, MaxLng: 29.020,
		MinZoom: 12, MaxZoom: 16, EstimatedSize: 45,
	},
	{
		ID: "istanbul-anadolu", Name: "İstanbul Anadolu Yakası",
		Description: "Kadıköy, Üsküdar, Ataşehir, Maltepe bölgeleri",
		MinLat: 40.920, MinLng: 29.000, MaxLat: 41.020, MaxLng: 29.150,
		MinZoom: 12, MaxZoom: 16, EstimatedSize: 55,
	},
	{
		ID: "istanbul-avrupa-kuzey", Name: "İstanbul Avrupa Kuzey",
		Description: "Sarıyer, Beşiktaş kuzey, Kağıthane bölgeleri",
		MinLat: 41.050, MinLng: 28.950, MaxLat: 41.200, MaxLng: 29.100,
		MinZoom: 12, MaxZoom: 16, EstimatedSize: 40,
	},
	{
		ID: "istanbul-bati", Name: "İstanbul Batı",
		Description: "Bakırköy, Başakşehir, Küçükçekmece bölgeleri",
		MinLat: 41.000, MinLng: 28.700, MaxLat: 41.120, MaxLng: 28.900,
		MinZoom: 12, MaxZoom: 16, EstimatedSize: 50,
	},
}

// defaultTileConfig OpenStreetMap tile indirme yapılandırmasıdır.
// Mobil uygulama bu şablonu kullanarak tile'ları yerel depolamaya indirir.
var defaultTileConfig = models.TileConfig{
	URLTemplate: "https://tile.openstreetmap.org/{z}/{x}/{y}.png",
	Attribution: "© OpenStreetMap contributors",
	MaxZoom:     16,
}

// emergencyTips offline erişim için sabit acil durum önerileridir.
var emergencyTips = []string{
	"Deprem sırasında çök-kapan-tutun pozisyonunu alın.",
	"Elektrik, su ve gaz vanalarını kapatın.",
	"Merdiven ve asansör kullanmayın, merdiven boşluklarından uzak durun.",
	"En yakın toplanma alanına yürüyerek gidin; araç kullanmayın.",
	"Telefon hattını meşgul etmeyin; SMS tercih edin.",
	"Yaralı veya sıkışmış kişilere ilk yardım uygulayın.",
	"Bina hasarlı görünüyorsa içeri girmeyin.",
}

// GetOfflineRegions indirilebilir tüm bölgelerin listesini döndürür.
func GetOfflineRegions() []models.OfflineRegion {
	return offlineRegions
}

// GetOfflineBundle belirtilen bölge için tam offline veri paketini oluşturur.
func GetOfflineBundle(regionID string) (models.OfflineBundle, bool) {
	var region *models.OfflineRegion
	for i := range offlineRegions {
		if offlineRegions[i].ID == regionID {
			region = &offlineRegions[i]
			break
		}
	}
	if region == nil {
		return models.OfflineBundle{}, false
	}

	// Bölge sınırları içindeki toplanma alanlarını filtrele
	points := filterAssemblyPointsInRegion(*region)

	return models.OfflineBundle{
		Region:         *region,
		TileConfig:     defaultTileConfig,
		AssemblyPoints: points,
		EmergencyTips:  emergencyTips,
		Version:        "1.0.0",
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}, true
}

// filterAssemblyPointsInRegion verilen bölge sınırları içindeki toplanma alanlarını döndürür.
func filterAssemblyPointsInRegion(region models.OfflineRegion) []models.AssemblyPoint {
	allPoints := getAllAssemblyPoints()
	var filtered []models.AssemblyPoint
	for _, p := range allPoints {
		if p.Lat >= region.MinLat && p.Lat <= region.MaxLat &&
			p.Lng >= region.MinLng && p.Lng <= region.MaxLng {
			filtered = append(filtered, p)
		}
	}
	if filtered == nil {
		filtered = []models.AssemblyPoint{}
	}
	return filtered
}

// getAllAssemblyPoints tüm toplanma alanlarını döndürür (assemblypoints handler ile paylaşılan veri).
func getAllAssemblyPoints() []models.AssemblyPoint {
	return []models.AssemblyPoint{
		{Name: "Atatürk Olimpiyat Stadı", Lat: 41.0714, Lng: 28.7330, Capacity: 50000},
		{Name: "Yıldız Parkı", Lat: 41.0489, Lng: 29.0108, Capacity: 20000},
		{Name: "Maçka Parkı", Lat: 41.0456, Lng: 29.0006, Capacity: 15000},
		{Name: "Gülhane Parkı", Lat: 41.0128, Lng: 28.9833, Capacity: 25000},
		{Name: "Fatih Millet Bahçesi", Lat: 41.0186, Lng: 28.9395, Capacity: 30000},
	}
}

// GetAssemblyPointsGeoJSON toplanma alanlarını GeoJSON FeatureCollection olarak döndürür.
// Mobil harita uygulaması bunu offline katman olarak kullanabilir.
func GetAssemblyPointsGeoJSON() map[string]any {
	points := getAllAssemblyPoints()
	features := make([]map[string]any, 0, len(points))
	for _, p := range points {
		features = append(features, map[string]any{
			"type": "Feature",
			"geometry": map[string]any{
				"type":        "Point",
				"coordinates": []float64{p.Lng, p.Lat},
			},
			"properties": map[string]any{
				"name":     p.Name,
				"capacity": p.Capacity,
			},
		})
	}
	return map[string]any{
		"type":     "FeatureCollection",
		"features": features,
	}
}

// NormalizeRegionID URL'den gelen bölge kimliğini normalize eder.
func NormalizeRegionID(id string) string {
	return strings.TrimSpace(strings.ToLower(id))
}
