// Package services uygulamanın iş mantığını (business logic) barındırır.
// Bu dosya Mapillary Graph API v4 ile sokak seviyesi görüntü çekmeyi sağlar.
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// mapillaryBaseURL Mapillary Graph API v4'ün görüntü endpoint'idir.
const mapillaryBaseURL = "https://graph.mapillary.com/images"

// mapillaryRadius yarıçap aramasında kullanılan metre cinsinden değerdir (API maksimumu 50).
const mapillaryRadius = 50

// mapillaryBBoxHalf bbox geri dönüş (fallback) aramasında merkez koordinatın
// her yönüne eklenen derece miktarıdır. ~0.0045° ≈ 500 m.
// Toplam kenar 0.009° < 0.01° olduğundan Mapillary bbox sınırına uygundur.
const mapillaryBBoxHalf = 0.0045

// ErrNoStreetViewImage belirtilen konumda (yarıçap + bbox fallback dahil)
// hiç sokak görüntüsü bulunamadığında döndürülür.
// Çağıran katman bunu errors.Is ile yakalayıp kullanıcıya "kapsama yok" yanıtı verebilir.
var ErrNoStreetViewImage = errors.New("bu konumda sokak görüntüsü bulunamadı")

// mapillaryImageResponse API'den dönen görüntü listesini temsil eder.
type mapillaryImageResponse struct {
	Data []mapillaryImage `json:"data"`
}

// mapillaryImage tek bir Mapillary görüntüsünün meta verilerini tutar.
type mapillaryImage struct {
	ID           string             `json:"id"`
	Thumb2048URL string             `json:"thumb_2048_url"`
	IsPano       bool               `json:"is_pano"`
	Geometry     *mapillaryGeometry `json:"geometry"`
}

// mapillaryGeometry görüntünün GeoJSON Point konumunu temsil eder.
// Coordinates dizisi [boylam, enlem] sırasındadır.
type mapillaryGeometry struct {
	Coordinates []float64 `json:"coordinates"`
}

// FetchStreetView verilen enlem (lat) ve boylam (lng) koordinatlarına en uygun
// sokak görüntüsünü Mapillary API üzerinden indirir ve []byte olarak döndürür.
//
// Akış:
//  1. 50 m yarıçapta görüntü ara; 360° panorama olmayan en iyi görseli seç.
//  2. Bulunamazsa ~500 m'lik bbox ile genişlet ve en yakın görseli seç (fallback).
//  3. Yine bulunamazsa ErrNoStreetViewImage döndür.
//  4. Seçilen görselin thumb_2048_url'sinden gerçek görüntüyü indir.
func FetchStreetView(lat, lng float64) ([]byte, error) {
	// Ortam değişkeninden Mapillary erişim token'ını oku.
	// Panoya yapıştırma sırasında oluşabilecek baştaki/sondaki boşluk ve
	// satır sonu karakterlerini temizle (yaygın 400 hata sebebi).
	accessToken := strings.TrimSpace(os.Getenv("MAPILLARY_ACCESS_TOKEN"))
	if accessToken == "" {
		return nil, fmt.Errorf("MAPILLARY_ACCESS_TOKEN ortam değişkeni tanımlı değil")
	}

	// 1. Adım: 50 m yarıçapta ara
	images, err := queryMapillaryRadius(accessToken, lat, lng)
	if err != nil {
		return nil, err
	}

	// 2. Adım: yarıçapta bir şey yoksa bbox fallback ile genişlet
	if len(images) == 0 {
		images, err = queryMapillaryBBox(accessToken, lat, lng)
		if err != nil {
			return nil, err
		}
	}

	// 3. Adım: hâlâ görüntü yoksa "kapsama yok" hatası döndür
	chosen := pickBestImage(images, lat, lng)
	if chosen == nil || chosen.Thumb2048URL == "" {
		return nil, ErrNoStreetViewImage
	}

	// 4. Adım: seçilen görüntüyü indir
	return downloadImage(chosen.Thumb2048URL)
}

// queryMapillaryRadius koordinat etrafında 50 m yarıçapta görüntü arar.
// Sonuçlar Mapillary tarafından yakınlık/güncellik/360° tercihine göre sıralanır.
func queryMapillaryRadius(token string, lat, lng float64) ([]mapillaryImage, error) {
	query := url.Values{}
	query.Set("access_token", token)
	query.Set("fields", "id,thumb_2048_url,is_pano,geometry")
	query.Set("lat", fmt.Sprintf("%f", lat))
	query.Set("lng", fmt.Sprintf("%f", lng))
	query.Set("radius", fmt.Sprintf("%d", mapillaryRadius))
	query.Set("limit", "10")
	return fetchMapillaryImages(mapillaryBaseURL + "?" + query.Encode())
}

// queryMapillaryBBox merkez koordinat etrafında ~500 m'lik bir bounding box
// içindeki görüntüleri arar (yarıçap aramasının fallback'i).
func queryMapillaryBBox(token string, lat, lng float64) ([]mapillaryImage, error) {
	// bbox sırası: minLon, minLat, maxLon, maxLat (sol, alt, sağ, üst)
	bbox := fmt.Sprintf("%f,%f,%f,%f",
		lng-mapillaryBBoxHalf,
		lat-mapillaryBBoxHalf,
		lng+mapillaryBBoxHalf,
		lat+mapillaryBBoxHalf,
	)
	query := url.Values{}
	query.Set("access_token", token)
	query.Set("fields", "id,thumb_2048_url,is_pano,geometry")
	query.Set("bbox", bbox)
	query.Set("limit", "50")
	return fetchMapillaryImages(mapillaryBaseURL + "?" + query.Encode())
}

// fetchMapillaryImages verilen URL'ye istek atar ve görüntü listesini çözümler.
func fetchMapillaryImages(requestURL string) ([]mapillaryImage, error) {
	resp, err := http.Get(requestURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("Mapillary meta veri isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Mapillary hata gövdesini de oku; sorunun kök sebebini (ör. geçersiz token) açıklar
		errBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Mapillary API beklenmeyen durum kodu döndürdü: %d (yanıt: %s)", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var result mapillaryImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("Mapillary API yanıtı çözümlenemedi: %w", err)
	}
	return result.Data, nil
}

// pickBestImage görüntü listesinden analiz için en uygun olanı seçer.
// Öncelik sırası:
//  1. 360° panorama OLMAYAN ve merkeze en yakın görüntü (bina yükseklik/sokak
//     genişliği tahmini panoramada bozulduğu için panoramadan kaçınılır).
//  2. Hiç düz görüntü yoksa merkeze en yakın panorama.
//
// Konum (geometry) bilgisi olmayan görüntüler için liste sırası korunur.
func pickBestImage(images []mapillaryImage, lat, lng float64) *mapillaryImage {
	if len(images) == 0 {
		return nil
	}

	var bestFlat, bestPano *mapillaryImage
	bestFlatDist, bestPanoDist := math.MaxFloat64, math.MaxFloat64

	for i := range images {
		img := &images[i]
		if img.Thumb2048URL == "" {
			continue
		}
		dist := imageDistance(img, lat, lng)
		if img.IsPano {
			if dist < bestPanoDist {
				bestPanoDist, bestPano = dist, img
			}
		} else {
			if dist < bestFlatDist {
				bestFlatDist, bestFlat = dist, img
			}
		}
	}

	// Önce düz (panorama olmayan) görüntüyü tercih et
	if bestFlat != nil {
		return bestFlat
	}
	return bestPano
}

// imageDistance görüntünün merkez koordinata uzaklığını hesaplar.
// Konum bilgisi yoksa sıralamayı bozmamak için 0 döndürülür.
func imageDistance(img *mapillaryImage, lat, lng float64) float64 {
	if img.Geometry == nil || len(img.Geometry.Coordinates) < 2 {
		return 0
	}
	imgLng, imgLat := img.Geometry.Coordinates[0], img.Geometry.Coordinates[1]
	return haversineMeters(lat, lng, imgLat, imgLng)
}

// haversineMeters iki koordinat arasındaki mesafeyi metre cinsinden hesaplar.
func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// downloadImage verilen thumb URL'sinden görüntüyü indirir ve byte dizisi döndürür.
func downloadImage(imageURL string) ([]byte, error) {
	resp, err := http.Get(imageURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("Mapillary görüntü indirme isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Mapillary görüntü indirme beklenmeyen durum kodu döndürdü: %d", resp.StatusCode)
	}

	imageBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Mapillary görüntüsü okunamadı: %w", err)
	}
	return imageBytes, nil
}
