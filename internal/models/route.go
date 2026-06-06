// Package models uygulamada kullanılan veri yapılarını (data transfer objects) tanımlar.
// Bu dosya güvenli rota hesaplama ile ilgili istek ve yanıt modellerini içerir.
package models

// Coordinate coğrafi bir noktanın enlem ve boylam koordinatlarını tutar.
type Coordinate struct {
	Lat float64 `json:"lat"` // Enlem (latitude)
	Lng float64 `json:"lng"` // Boylam (longitude)
}

// RouteRequest istemciden gelen rota hesaplama isteğini temsil eder.
type RouteRequest struct {
	Origin      Coordinate `json:"origin"`      // Başlangıç noktası koordinatları
	Destination Coordinate `json:"destination"` // Varış noktası koordinatları
}

// RouteSegment rotanın iki nokta arasındaki bir dilimini ve o dilime ait
// enkaz risk skorunu temsil eder.
type RouteSegment struct {
	Start     Coordinate `json:"start"`      // Dilimin başlangıç koordinatı
	End       Coordinate `json:"end"`        // Dilimin bitiş koordinatı
	RiskScore int        `json:"risk_score"` // Bu dilim için hesaplanan risk skoru (0-100)
}

// RouteResponse istemciye döndürülen hesaplanmış rota sonucunu temsil eder.
type RouteResponse struct {
	Segments      []RouteSegment `json:"segments"`       // Rotayı oluşturan risk değerlendirilmiş dilimler
	TotalDistance float64        `json:"total_distance"` // Toplam rota mesafesi (km)
	SafetyScore   int            `json:"safety_score"`   // Tüm rota için genel güvenlik skoru (0-100, yüksek = güvenli)
	AssemblyPoint Coordinate     `json:"assembly_point"` // Önerilen en yakın toplanma alanı koordinatı
}
