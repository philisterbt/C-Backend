// Package handlers HTTP istek/yanıt döngülerini yönetir.
// Bu dosya offline mesajlaşma endpoint'lerini işler:
// mesaj gönderme, listeleme ve senkronizasyon.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c-backend/internal/models"
	"c-backend/internal/services"
)

// MessagesHandler mesajlaşma endpoint'leri için gerekli bağımlılıkları barındırır.
type MessagesHandler struct {
	store *services.MessageStore
}

// NewMessagesHandler yeni bir MessagesHandler örneği döndürür.
func NewMessagesHandler(store *services.MessageStore) *MessagesHandler {
	return &MessagesHandler{store: store}
}

// HandleSend POST /api/v1/messages isteğini işler.
// Tek bir mesajı sunucuya kaydeder (online gönderim).
func (h *MessagesHandler) HandleSend(w http.ResponseWriter, r *http.Request) {
	var req models.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Geçersiz istek gövdesi: "+err.Error())
		return
	}

	if err := validateSendRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	msg, err := h.store.SaveMessage(models.Message{
		ClientID:  req.ClientID,
		RoomID:    req.RoomID,
		DeviceID:  req.DeviceID,
		Sender:    req.Sender,
		Content:   req.Content,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Mesaj kaydedilemedi: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}

// HandleList GET /api/v1/messages isteğini işler.
// Query parametreleri: room_id (zorunlu), since (opsiyonel ISO 8601), limit (opsiyonel)
func (h *MessagesHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room_id")
	if roomID == "" {
		writeError(w, http.StatusBadRequest, "room_id parametresi zorunludur")
		return
	}

	var since *time.Time
	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "since parametresi geçersiz ISO 8601 formatında olmalı")
			return
		}
		since = &t
	}

	limit := parseLimit(r.URL.Query().Get("limit"), 50)

	messages, err := h.store.GetMessages(roomID, since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Mesajlar alınamadı: "+err.Error())
		return
	}

	if messages == nil {
		messages = []models.Message{}
	}
	writeJSON(w, http.StatusOK, messages)
}

// HandleSync POST /api/v1/messages/sync isteğini işler.
// Offline-first senkronizasyon: bekleyen mesajları gönderir, yeni mesajları çeker.
func (h *MessagesHandler) HandleSync(w http.ResponseWriter, r *http.Request) {
	var req models.SyncMessagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Geçersiz istek gövdesi: "+err.Error())
		return
	}

	if req.DeviceID == "" || req.RoomID == "" {
		writeError(w, http.StatusBadRequest, "device_id ve room_id alanları zorunludur")
		return
	}

	if req.Outgoing == nil {
		req.Outgoing = []models.OutgoingMessage{}
	}

	resp, err := h.store.SyncMessages(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Senkronizasyon başarısız: "+err.Error())
		return
	}

	if resp.Incoming == nil {
		resp.Incoming = []models.Message{}
	}
	writeJSON(w, http.StatusOK, resp)
}

// validateSendRequest mesaj gönderme isteğinin zorunlu alanlarını doğrular.
func validateSendRequest(req models.SendMessageRequest) error {
	if req.ClientID == "" {
		return errString("client_id alanı zorunludur")
	}
	if req.RoomID == "" {
		return errString("room_id alanı zorunludur")
	}
	if req.DeviceID == "" {
		return errString("device_id alanı zorunludur")
	}
	if strings.TrimSpace(req.Sender) == "" {
		return errString("sender alanı zorunludur")
	}
	if strings.TrimSpace(req.Content) == "" {
		return errString("content alanı zorunludur")
	}
	return nil
}

// errString basit doğrulama hatası oluşturur.
type errString string

func (e errString) Error() string { return string(e) }

// parseLimit query parametresinden limit değerini çözümler.
func parseLimit(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return defaultVal
	}
	if n > 200 {
		return 200
	}
	return n
}
