// Package services uygulamanın iş mantığını (business logic) barındırır.
// Bu dosya Mapillary Graph API v4 ile sokak seviyesi görüntü çekmeyi sağlar.
package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// mapillaryBaseURL Mapillary Graph API v4'ün temel adresidir.
const mapillaryBaseURL = "https://graph.mapillary.com/images"

// mapillaryImageResponse API'den dönen görüntü listesini temsil eder.
type mapillaryImageResponse struct {
	Data []mapillaryImage `json:"data"`
}

// mapillaryImage tek bir Mapillary görüntüsünün meta verilerini tutar.
type mapillaryImage struct {
	ID           string `json:"id"`
	Thumb2048URL string `json:"thumb_2048_url"`
}

// FetchStreetView verilen enlem (lat) ve boylam (lng) koordinatlarına en yakın
// sokak görüntüsünü Mapillary API üzerinden indirir ve []byte olarak döndürür.
//
// Akış:
//  1. Koordinat etrafında 50 metrelik yarıçapla en iyi 1 görüntüyü sorgula.
//  2. Dönen meta veriden thumb_2048_url al.
//  3. Gerçek görüntüyü o URL'den indir ve byte dizisi olarak döndür.
func FetchStreetView(lat, lng float64) ([]byte, error) {
	// Ortam değişkeninden Mapillary erişim token'ını oku
	accessToken := os.Getenv("MAPILLARY_ACCESS_TOKEN")
	if accessToken == "" {
		return nil, fmt.Errorf("MAPILLARY_ACCESS_TOKEN ortam değişkeni tanımlı değil")
	}

	// 1. Adım: koordinata yakın en iyi görüntünün meta verisini getir
	metaURL := fmt.Sprintf(
		"%s?access_token=%s&fields=id,thumb_2048_url&lat=%f&lng=%f&radius=50&limit=1",
		mapillaryBaseURL,
		accessToken,
		lat,
		lng,
	)

	metaResp, err := http.Get(metaURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("Mapillary meta veri isteği başarısız: %w", err)
	}
	defer metaResp.Body.Close()

	if metaResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mapillary API beklenmeyen durum kodu döndürdü: %d", metaResp.StatusCode)
	}

	// API yanıtını parse et
	var result mapillaryImageResponse
	if err := json.NewDecoder(metaResp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("Mapillary API yanıtı çözümlenemedi: %w", err)
	}

	// Belirtilen koordinat yakınında hiç görüntü yoksa hata döndür
	if len(result.Data) == 0 || result.Data[0].Thumb2048URL == "" {
		return nil, fmt.Errorf("belirtilen koordinatlara yakın Mapillary görüntüsü bulunamadı: lat=%f, lng=%f", lat, lng)
	}

	imageURL := result.Data[0].Thumb2048URL

	// 2. Adım: gerçek görüntüyü thumb URL'sinden indir
	imgResp, err := http.Get(imageURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("Mapillary görüntü indirme isteği başarısız: %w", err)
	}
	defer imgResp.Body.Close()

	if imgResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mapillary görüntü indirme beklenmeyen durum kodu döndürdü: %d", imgResp.StatusCode)
	}

	// Görüntüyü belleğe oku ve döndür
	imageBytes, err := io.ReadAll(imgResp.Body)
	if err != nil {
		return nil, fmt.Errorf("Mapillary görüntüsü okunamadı: %w", err)
	}

	return imageBytes, nil
}
