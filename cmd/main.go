// Afet Yolu Backend — Ana giriş noktası
// Bu dosya HTTP sunucusunu başlatır, tüm route'ları ve middleware'leri bağlar.
package main

import (
	"log"
	"net/http"

	"c-backend/config"
	"c-backend/internal/handlers"
	"c-backend/pkg/middleware"
)

func main() {
	// Ortam değişkenlerinden yapılandırmayı yükle
	cfg := config.Load()

	// Handler'ları başlat (servisler inject edilerek)
	riskHandler := handlers.NewRiskHandler()
	routeHandler := handlers.NewRouteHandler()
	assemblyPointsHandler := handlers.NewAssemblyPointsHandler()

	// HTTP yönlendirici (mux) oluştur
	mux := http.NewServeMux()

	// Sistem sağlık kontrolü endpoint'i
	mux.HandleFunc("GET /health", healthCheck)

	// API v1 endpoint'leri
	mux.HandleFunc("POST /api/v1/risk", riskHandler.Handle)
	mux.HandleFunc("POST /api/v1/route", routeHandler.Handle)
	mux.HandleFunc("GET /api/v1/assembly-points", assemblyPointsHandler.Handle)

	// CORS middleware'ini tüm route'lara uygula
	handler := middleware.CORS(mux)

	// Sunucuyu yapılandır ve başlat
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: handler,
	}

	log.Printf("Afet Yolu backend %s portunda çalışıyor", cfg.Port)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Sunucu başlatılamadı: %v", err)
	}
}

// healthCheck GET /health isteğine sistem durumunu döndürür.
// Render.com ve diğer hosting servislerinin canlılık kontrolü için kullanılır.
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"afet-yolu-backend"}`)) //nolint:errcheck
}
