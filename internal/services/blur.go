// Package services uygulamanın iş mantığını (business logic) barındırır.
// Bu dosya görüntü içindeki hassas verileri bulanıklaştırma (blur) işlemini sağlar.
package services

// BlurSensitiveData verilen görüntü baytları üzerinde yüz ve plaka tespiti yaparak
// bu bölgeleri KVKK kapsamında bulanıklaştırır ve işlenmiş görüntüyü döndürür.
//
// TODO (KVKK): Aşağıdaki işlemler gerçek bir bilgisayarlı görü (computer vision)
// kütüphanesi ile uygulanacaktır:
//   - Yüz tanıma (face detection) → tespit edilen yüzler Gaussian blur ile gizlenecek
//   - Plaka tanıma (license plate recognition) → plakalar mozaik efektiyle örtülecek
//
// Şu an için görüntü değiştirilmeden geri döndürülmektedir (mock uygulama).
func BlurSensitiveData(imageBytes []byte) ([]byte, error) {
	// Mock: görüntü olduğu gibi döndürülüyor
	// Gerçek implementasyonda OpenCV veya benzeri bir kütüphane kullanılacak
	return imageBytes, nil
}
