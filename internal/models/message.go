// Package models uygulamada kullanılan veri yapılarını (data transfer objects) tanımlar.
// Bu dosya offline mesajlaşma ve senkronizasyon modellerini içerir.
package models

import "time"

// Message sunucuda saklanan tek bir mesajı temsil eder.
type Message struct {
	ID        string    `json:"id"`         // Sunucu tarafı benzersiz mesaj kimliği
	ClientID  string    `json:"client_id"`  // Mobil cihazın ürettiği yerel kimlik (çift kayıt önleme)
	RoomID    string    `json:"room_id"`    // Mesaj odası (ör. aile, mahalle)
	DeviceID  string    `json:"device_id"`  // Gönderen cihaz kimliği
	Sender    string    `json:"sender"`     // Gönderen görünen adı
	Content   string    `json:"content"`    // Mesaj metni
	CreatedAt time.Time `json:"created_at"` // Mesajın oluşturulma zamanı (UTC)
}

// SendMessageRequest yeni mesaj gönderme isteğini temsil eder.
type SendMessageRequest struct {
	ClientID string `json:"client_id"` // Mobil tarafın ürettiği benzersiz kimlik (zorunlu)
	RoomID   string `json:"room_id"`   // Hedef oda kimliği (zorunlu)
	DeviceID string `json:"device_id"` // Gönderen cihaz kimliği (zorunlu)
	Sender   string `json:"sender"`    // Gönderen görünen adı (zorunlu)
	Content  string `json:"content"`   // Mesaj içeriği (zorunlu)
}

// SyncMessagesRequest offline mesaj senkronizasyon isteğini temsil eder.
// Mobil cihaz internete bağlandığında bekleyen mesajlarını gönderir ve
// sunucudaki yeni mesajları çeker.
type SyncMessagesRequest struct {
	DeviceID   string              `json:"device_id"`    // Senkronize eden cihaz kimliği (zorunlu)
	RoomID     string              `json:"room_id"`      // Senkronize edilecek oda (zorunlu)
	LastSyncAt *time.Time          `json:"last_sync_at"` // Son başarılı senkron zamanı (nil = ilk senkron)
	Outgoing   []OutgoingMessage   `json:"outgoing"`     // Cihazda bekleyen, henüz sunucuya gitmemiş mesajlar
}

// OutgoingMessage mobil cihazdan sunucuya gönderilecek bekleyen mesajı temsil eder.
type OutgoingMessage struct {
	ClientID  string    `json:"client_id"`  // Yerel benzersiz kimlik
	Sender    string    `json:"sender"`     // Gönderen adı
	Content   string    `json:"content"`    // Mesaj metni
	CreatedAt time.Time `json:"created_at"` // Cihazda oluşturulma zamanı
}

// SyncMessagesResponse senkronizasyon sonucunu temsil eder.
type SyncMessagesResponse struct {
	Incoming       []Message `json:"incoming"`         // Sunucudan gelen yeni mesajlar
	AckedClientIDs []string  `json:"acked_client_ids"` // Sunucuya başarıyla kaydedilen client_id listesi
	ServerTime     time.Time `json:"server_time"`      // Sunucu zamanı (sonraki senkron için referans)
}
