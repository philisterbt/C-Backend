// Package handlers HTTP istek/yanıt döngülerini yönetir.
// Bu dosya tüm handler'lar tarafından ortak kullanılan yardımcı fonksiyonları içerir.
package handlers

import (
	"encoding/json"
	"net/http"
)

// errorResponse hata yanıtları için standart JSON yapısıdır.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON verilen veriyi JSON formatında HTTP yanıtı olarak yazar.
func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data) //nolint:errcheck
}

// writeError standart hata yanıtı formatında JSON yazar.
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, errorResponse{Error: message})
}
