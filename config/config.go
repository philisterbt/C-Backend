// Package config uygulamanın yapılandırma ayarlarını yönetir.
// Ortam değişkenlerini (environment variables) okuyup uygulamaya sunar.
package config

import (
	"os"
)

// Config yapısı uygulamanın çalışması için gerekli tüm ayarları tutar.
type Config struct {
	Port   string // Sunucunun dinleyeceği port numarası
	AppEnv string // Uygulama ortamı (development, production vb.)
}

// Load fonksiyonu ortam değişkenlerini okur ve bir Config nesnesi döndürür.
// Değişken tanımlı değilse varsayılan (default) değer kullanılır.
func Load() *Config {
	return &Config{
		Port:   getEnv("PORT", "8080"),
		AppEnv: getEnv("APP_ENV", "development"),
	}
}

// getEnv yardımcı fonksiyonu; istenen ortam değişkenini okur.
// Değişken boş veya tanımsız ise verilen varsayılan değeri döndürür.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}
