// Package models uygulamada kullanılan veri yapılarını (data transfer objects) tanımlar.
// Bu dosya offline harita indirme ve acil durum veri paketi modellerini içerir.
package models

// OfflineRegion indirilebilir bir harita bölgesini temsil eder.
// Mobil uygulama bu bilgileri kullanarak tile'ları önceden indirir.
type OfflineRegion struct {
	ID            string  `json:"id"`             // Bölge kimliği (ör. istanbul-merkez)
	Name          string  `json:"name"`           // Kullanıcıya gösterilen bölge adı
	Description   string  `json:"description"`    // Bölge açıklaması
	MinLat        float64 `json:"min_lat"`        // Bounding box alt sınır (enlem)
	MinLng        float64 `json:"min_lng"`        // Bounding box sol sınır (boylam)
	MaxLat        float64 `json:"max_lat"`        // Bounding box üst sınır (enlem)
	MaxLng        float64 `json:"max_lng"`        // Bounding box sağ sınır (boylam)
	MinZoom       int     `json:"min_zoom"`       // İndirilecek minimum zoom seviyesi
	MaxZoom       int     `json:"max_zoom"`       // İndirilecek maksimum zoom seviyesi
	EstimatedSize int     `json:"estimated_size_mb"` // Tahmini indirme boyutu (MB)
}

// TileConfig harita tile indirme yapılandırmasını temsil eder.
// Mobil uygulama bu URL şablonunu kullanarak tile'ları önbelleğe alır.
type TileConfig struct {
	URLTemplate string `json:"url_template"` // Tile URL şablonu: {z}/{x}/{y}
	Attribution string `json:"attribution"`  // Harita kaynağı atıf metni
	MaxZoom     int    `json:"max_zoom"`     // İzin verilen maksimum zoom
}

// OfflineBundle bir bölge için indirilecek tüm offline veri paketini temsil eder.
// Mobil uygulama bu paketi indirip yerel depolamaya kaydeder.
type OfflineBundle struct {
	Region         OfflineRegion   `json:"region"`          // Bölge sınırları ve zoom bilgisi
	TileConfig     TileConfig      `json:"tile_config"`     // Harita tile indirme yapılandırması
	AssemblyPoints []AssemblyPoint `json:"assembly_points"` // Bölgedeki toplanma alanları
	EmergencyTips  []string        `json:"emergency_tips"`  // Acil durum ipuçları (offline erişim için)
	Version        string          `json:"version"`         // Paket sürümü (güncelleme kontrolü için)
	GeneratedAt    string          `json:"generated_at"`    // Paketin oluşturulma zamanı (ISO 8601)
}
