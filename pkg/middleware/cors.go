// Package middleware HTTP ara katmanlarını (middleware) barındırır.
// Bu dosya Next.js frontend dahil tüm kaynaklardan gelen CORS isteklerine
// izin veren middleware fonksiyonunu içerir.
package middleware

import "net/http"

// CORS tüm kaynaklardan (*) gelen isteklere izin veren bir HTTP middleware'idir.
// Next.js frontend ile iletişim kurabilmek için gerekli başlıkları (header) ayarlar.
// OPTIONS (preflight) isteklerine 204 No Content yanıtı döndürür.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Tüm kaynaklara izin ver (Next.js frontend için)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// İzin verilen HTTP metodları
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		// İzin verilen istek başlıkları
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		// OPTIONS (preflight) isteğine boş başarılı yanıt döndür
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Sonraki middleware veya handler'a devam et
		next.ServeHTTP(w, r)
	})
}
