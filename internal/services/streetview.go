// Package services uygulamanın iş mantığını (business logic) barındırır.
// Bu dosya Mapillary Graph API v4 ile sokak seviyesi görüntü çekmeyi sağlar.
package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
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
	// Ortam değişkeninden Mapillary erişim token'ını oku.
	// Panoya yapıştırma sırasında oluşabilecek baştaki/sondaki boşluk ve
	// satır sonu karakterlerini temizle (yaygın 400 hata sebebi).
	accessToken := strings.TrimSpace(os.Getenv("MAPILLARY_ACCESS_TOKEN"))
	if accessToken == "" {
		return nil, fmt.Errorf("MAPILLARY_ACCESS_TOKEN ortam değişkeni tanımlı değil")
	}

	// 1. Adım: koordinata yakın en iyi görüntünün meta verisini getir.
	// Sorgu parametrelerini url.Values ile düzgünce encode et.
	query := url.Values{}
	query.Set("access_token", accessToken)
	query.Set("fields", "id,thumb_2048_url")
	query.Set("lat", fmt.Sprintf("%f", lat))
	query.Set("lng", fmt.Sprintf("%f", lng))
	query.Set("radius", "50")
	query.Set("limit", "1")
	metaURL := mapillaryBaseURL + "?" + query.Encode()

	metaResp, err := http.Get(metaURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("Mapillary meta veri isteği başarısız: %w", err)
	}
	defer metaResp.Body.Close()

	if metaResp.StatusCode != http.StatusOK {
		// Mapillary hata gövdesini de oku; sorunun kök sebebini (ör. geçersiz token) açıklar
		errBody, _ := io.ReadAll(metaResp.Body)
		return nil, fmt.Errorf("Mapillary API beklenmeyen durum kodu döndürdü: %d (yanıt: %s)", metaResp.StatusCode, strings.TrimSpace(string(errBody)))
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
