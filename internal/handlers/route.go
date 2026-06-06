// Package handlers HTTP istek/yanıt döngülerini yönetir.
// Bu dosya POST /api/v1/route endpoint'ini işler:
// başlangıç ve varış koordinatları alır → güvenli rota hesaplar → RouteResponse döndürür.
package handlers

import (
	"encoding/json"
	"math"
	"math/rand"
	"net/http"

	"c-backend/internal/models"
)

// RouteHandler güvenli rota hesaplama endpoint'i için gerekli bağımlılıkları barındırır.
type RouteHandler struct{}

// NewRouteHandler yeni bir RouteHandler örneği döndürür.
func NewRouteHandler() *RouteHandler {
	return &RouteHandler{}
}

// Handle POST /api/v1/route isteğini işler.
// Origin ve Destination koordinatlarını okur, aralarında 3 segment oluşturur
// ve her segmente rastgele bir risk skoru atar.
//
// TODO (Gerçek Algoritma): Aşağıdaki adımlar uygulanacaktır:
//   - OpenStreetMap veya Google Routes API ile gerçek yol ağı üzerinden rota çizimi
//   - Her segment için StreetView + Vision servislerinden gerçek risk skoru hesabı
//   - Dijkstra veya A* algoritması ile minimum riskli güzergah seçimi
//   - En yakın toplanma alanına yönlendirme mantığı
func (h *RouteHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// İstek gövdesini RouteRequest yapısına çözümle
	var req models.RouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Geçersiz istek gövdesi: "+err.Error())
		return
	}

	// Koordinat doğrulaması
	if req.Origin.Lat == 0 && req.Origin.Lng == 0 {
		writeError(w, http.StatusBadRequest, "Origin koordinatları zorunludur")
		return
	}
	if req.Destination.Lat == 0 && req.Destination.Lng == 0 {
		writeError(w, http.StatusBadRequest, "Destination koordinatları zorunludur")
		return
	}

	// Origin ile Destination arasındaki adım büyüklüğünü hesapla (3 eşit segment)
	latStep := (req.Destination.Lat - req.Origin.Lat) / 3
	lngStep := (req.Destination.Lng - req.Origin.Lng) / 3

	// 3 adet rota segmenti oluştur; her birine mock risk skoru ata
	segments := make([]models.RouteSegment, 3)
	totalRisk := 0
	for i := 0; i < 3; i++ {
		start := models.Coordinate{
			Lat: req.Origin.Lat + float64(i)*latStep,
			Lng: req.Origin.Lng + float64(i)*lngStep,
		}
		end := models.Coordinate{
			Lat: req.Origin.Lat + float64(i+1)*latStep,
			Lng: req.Origin.Lng + float64(i+1)*lngStep,
		}
		// Mock risk skoru: 10-80 arasında rastgele
		riskScore := 10 + rand.Intn(71)
		totalRisk += riskScore

		segments[i] = models.RouteSegment{
			Start:     start,
			End:       end,
			RiskScore: riskScore,
		}
	}

	// Genel güvenlik skoru: segment risk ortalamsının tersi (yüksek = güvenli)
	avgRisk := totalRisk / len(segments)
	safetyScore := 100 - avgRisk

	// Toplam mesafe: Haversine formülü ile yaklaşık hesaplama (km)
	totalDistance := haversineKm(req.Origin, req.Destination)

	// Mock toplanma noktası: İstanbul Atatürk Olimpiyat Stadı
	assemblyPoint := models.Coordinate{Lat: 41.0714, Lng: 28.7330}

	resp := models.RouteResponse{
		Segments:      segments,
		TotalDistance: math.Round(totalDistance*100) / 100,
		SafetyScore:   safetyScore,
		AssemblyPoint: assemblyPoint,
	}

	writeJSON(w, http.StatusOK, resp)
}

// haversineKm iki koordinat arasındaki mesafeyi Haversine formülü ile km cinsinden hesaplar.
func haversineKm(a, b models.Coordinate) float64 {
	const earthRadiusKm = 6371.0
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180

	x := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(x), math.Sqrt(1-x))
	return earthRadiusKm * c
}
