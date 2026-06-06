// Package services uygulamanın iş mantığını (business logic) barındırır.
// Bu dosya Google Street View Static API ile haberleşmeyi sağlar.
package services

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// streetViewBaseURL Google Street View Static API'nin temel URL'sidir.
const streetViewBaseURL = "https://maps.googleapis.com/maps/api/streetview"

// FetchStreetView verilen enlem (lat) ve boylam (lng) koordinatlarına göre
// Google Street View Static API'den 640x640 boyutunda bir görüntü indirir.
// Başarı durumunda görüntü verisi []byte olarak döner.
// Hata durumunda hata mesajı ile birlikte nil döner.
func FetchStreetView(lat, lng float64) ([]byte, error) {
	// Ortam değişkeninden API anahtarını oku
	apiKey := os.Getenv("STREET_VIEW_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("STREET_VIEW_API_KEY ortam değişkeni tanımlı değil")
	}

	// API istek URL'sini koordinatlar ve anahtar ile oluştur
	requestURL := fmt.Sprintf(
		"%s?size=640x640&location=%f,%f&key=%s",
		streetViewBaseURL,
		lat,
		lng,
		apiKey,
	)

	// HTTP GET isteği gönder
	resp, err := http.Get(requestURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("Street View API isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	// HTTP durum kodu 200 değilse hata döndür
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Street View API beklenmeyen durum kodu döndürdü: %d", resp.StatusCode)
	}

	// Yanıt gövdesini (response body) byte dizisine oku
	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Street View API yanıtı okunamadı: %w", err)
	}

	return imageBytes, nil
}
