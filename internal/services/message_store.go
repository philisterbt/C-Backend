// Package services uygulamanın iş mantığını (business logic) barındırır.
// Bu dosya offline mesajlaşma için SQLite tabanlı kalıcı depolama sağlar.
package services

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"c-backend/internal/models"

	_ "modernc.org/sqlite" // CGO gerektirmeyen SQLite sürücüsü (Render uyumlu)
)

// MessageStore mesajları SQLite veritabanında saklar ve sorgular.
type MessageStore struct {
	db *sql.DB
}

// NewMessageStore veritabanı dosyasını açar, şemayı oluşturur ve MessageStore döndürür.
// dbPath boşsa varsayılan olarak ./data/messages.db kullanılır.
func NewMessageStore(dbPath string) (*MessageStore, error) {
	if dbPath == "" {
		dbPath = "./data/messages.db"
	}

	// Veritabanı dizinini oluştur (yoksa)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("veritabanı dizini oluşturulamadı: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("veritabanı açılamadı: %w", err)
	}

	// Tek bağlantı yeterli (Render free tier tek instance)
	db.SetMaxOpenConns(1)

	store := &MessageStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

// migrate veritabanı tablolarını ve indekslerini oluşturur.
func (s *MessageStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id         TEXT PRIMARY KEY,
		client_id  TEXT NOT NULL DEFAULT '',
		room_id    TEXT NOT NULL,
		device_id  TEXT NOT NULL,
		sender     TEXT NOT NULL,
		content    TEXT NOT NULL,
		created_at TEXT NOT NULL
	);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_client_id ON messages(client_id) WHERE client_id != '';
	CREATE INDEX IF NOT EXISTS idx_messages_room_created ON messages(room_id, created_at);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Close veritabanı bağlantısını kapatır.
func (s *MessageStore) Close() error {
	return s.db.Close()
}

// SaveMessage yeni bir mesajı veritabanına kaydeder.
// Aynı client_id ile tekrar gönderilirse çift kayıt oluşturmaz (idempotent).
func (s *MessageStore) SaveMessage(msg models.Message) (models.Message, error) {
	msg.ID = generateMessageID()
	msg.CreatedAt = msg.CreatedAt.UTC()

	// client_id varsa ve zaten kayıtlıysa mevcut kaydı döndür
	if msg.ClientID != "" {
		existing, err := s.findByClientID(msg.ClientID)
		if err != nil {
			return models.Message{}, err
		}
		if existing != nil {
			return *existing, nil
		}
	}

	_, err := s.db.Exec(
		`INSERT INTO messages (id, client_id, room_id, device_id, sender, content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ClientID, msg.RoomID, msg.DeviceID, msg.Sender, msg.Content,
		msg.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return models.Message{}, fmt.Errorf("mesaj kaydedilemedi: %w", err)
	}
	return msg, nil
}

// GetMessages bir odadaki mesajları döndürür.
// since verilmişse yalnızca o zamandan sonraki mesajlar gelir.
func (s *MessageStore) GetMessages(roomID string, since *time.Time, limit int) ([]models.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var rows *sql.Rows
	var err error

	if since != nil {
		rows, err = s.db.Query(
			`SELECT id, client_id, room_id, device_id, sender, content, created_at
			 FROM messages WHERE room_id = ? AND created_at > ?
			 ORDER BY created_at ASC LIMIT ?`,
			roomID, since.UTC().Format(time.RFC3339Nano), limit,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, client_id, room_id, device_id, sender, content, created_at
			 FROM messages WHERE room_id = ?
			 ORDER BY created_at DESC LIMIT ?`,
			roomID, limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("mesajlar sorgulanamadı: %w", err)
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}

	// since olmadan DESC sıralandı; mobil için ASC'ye çevir
	if since == nil {
		reverseMessages(messages)
	}
	return messages, nil
}

// SyncMessages mobil cihazdan gelen bekleyen mesajları kaydeder ve
// sunucudaki yeni mesajları döndürür.
func (s *MessageStore) SyncMessages(req models.SyncMessagesRequest) (models.SyncMessagesResponse, error) {
	acked := make([]string, 0, len(req.Outgoing))

	// Gelen bekleyen mesajları kaydet
	for _, out := range req.Outgoing {
		if strings.TrimSpace(out.Content) == "" || out.ClientID == "" {
			continue
		}
		_, err := s.SaveMessage(models.Message{
			ClientID:  out.ClientID,
			RoomID:    req.RoomID,
			DeviceID:  req.DeviceID,
			Sender:    out.Sender,
			Content:   out.Content,
			CreatedAt: out.CreatedAt,
		})
		if err != nil {
			return models.SyncMessagesResponse{}, err
		}
		acked = append(acked, out.ClientID)
	}

	// Sunucudaki yeni mesajları çek
	incoming, err := s.GetMessages(req.RoomID, req.LastSyncAt, 200)
	if err != nil {
		return models.SyncMessagesResponse{}, err
	}

	return models.SyncMessagesResponse{
		Incoming:       incoming,
		AckedClientIDs: acked,
		ServerTime:     time.Now().UTC(),
	}, nil
}

// findByClientID client_id ile mevcut mesajı arar.
func (s *MessageStore) findByClientID(clientID string) (*models.Message, error) {
	row := s.db.QueryRow(
		`SELECT id, client_id, room_id, device_id, sender, content, created_at
		 FROM messages WHERE client_id = ?`, clientID,
	)
	msg, err := scanMessage(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// scanMessage tek bir satırı Message yapısına çevirir.
func scanMessage(scanner interface{ Scan(...any) error }) (models.Message, error) {
	var msg models.Message
	var createdAtStr string
	err := scanner.Scan(
		&msg.ID, &msg.ClientID, &msg.RoomID, &msg.DeviceID,
		&msg.Sender, &msg.Content, &createdAtStr,
	)
	if err != nil {
		return msg, err
	}
	msg.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtStr)
	return msg, err
}

// scanMessages birden fazla satırı Message dizisine çevirir.
func scanMessages(rows *sql.Rows) ([]models.Message, error) {
	var messages []models.Message
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("mesaj satırı okunamadı: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

// generateMessageID sunucu tarafı benzersiz mesaj kimliği üretir.
func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// reverseMessages mesaj dizisini ters çevirir (sıralama düzeltmesi).
func reverseMessages(msgs []models.Message) {
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
}
