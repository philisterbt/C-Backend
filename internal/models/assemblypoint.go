// Package models uygulamada kullanılan veri yapılarını (data transfer objects) tanımlar.
// Bu dosya afet toplanma alanı (muster point) veri modelini içerir.
package models

// AssemblyPoint bir afet toplanma alanını temsil eder.
// Kapasite alanı, alanın aynı anda barındırabileceği maksimum kişi sayısını belirtir.
type AssemblyPoint struct {
	Name     string  `json:"name"`     // Toplanma alanının adı
	Lat      float64 `json:"lat"`      // Enlem koordinatı
	Lng      float64 `json:"lng"`      // Boylam koordinatı
	Capacity int     `json:"capacity"` // Maksimum kişi kapasitesi
}
