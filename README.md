# Afet Yolu — Backend

Deprem ve acil durum senaryolarında **enkaz riski analizi**, **güvenli rota**, **toplanma alanları**, **offline harita paketleri** ve **mesajlaşma** sunan Go backend API'si.

**Canlı API:** https://c-backend-2enq.onrender.com

---

## Özellikler

- **Risk analizi** — Konumdaki sokak görüntüsünü analiz ederek 0–100 arası enkaz risk skoru, Türkçe bölge yorumu ve öneriler üretir
- **Güvenli rota** — Başlangıç ve varış arasında segment bazlı risk değerlendirmesi (şu an mock algoritma)
- **Toplanma alanları** — İstanbul'daki 5 gerçek toplanma noktası
- **Offline harita paketleri** — İndirilebilir bölge tanımları, tile yapılandırması ve acil durum ipuçları
- **Offline mesajlaşma** — SQLite tabanlı mesaj depolama ve senkronizasyon API'si
- **CORS** — Mobil ve web istemcileri için açık

---

## Tech Stack

| Katman | Teknoloji |
|--------|-----------|
| Dil | Go 1.25+ |
| HTTP | `net/http` (stdlib) |
| Sokak görüntüsü | [Mapillary Graph API v4](https://www.mapillary.com/developer/api-documentation) |
| Risk analizi | [Wiro AI](https://wiro.ai) — `moondream3-preview/query` |
| Mesaj depolama | SQLite (`modernc.org/sqlite`) |
| Hosting | [Render.com](https://render.com) |

---

## Proje Yapısı

```
C-Backend/
├── cmd/main.go                 # Uygulama giriş noktası
├── config/config.go            # Ortam değişkenleri
├── internal/
│   ├── handlers/               # HTTP endpoint'leri
│   ├── models/                 # İstek/yanıt modelleri
│   └── services/               # İş mantığı (Mapillary, Wiro AI, mesaj DB)
├── pkg/middleware/cors.go      # CORS middleware
├── render.yaml                 # Render deploy yapılandırması
├── Dockerfile
└── .env.example
```

---

## Kurulum (Yerel)

### Gereksinimler

- Go 1.25 veya üzeri

### Adımlar

```bash
# Repoyu klonla
git clone https://github.com/philisterbt/C-Backend.git
cd C-Backend

# Ortam değişkenlerini ayarla
cp .env.example .env
# .env dosyasını düzenle (API anahtarlarını gir)

# Bağımlılıkları indir
go mod tidy

# Çalıştır
go run ./cmd/main.go
```

Sunucu varsayılan olarak `http://localhost:8080` adresinde başlar.

### Sağlık kontrolü

```bash
curl http://localhost:8080/health
```

Beklenen yanıt:

```json
{"status":"ok","service":"afet-yolu-backend"}
```

---

## Ortam Değişkenleri

| Değişken | Zorunlu | Açıklama |
|----------|---------|----------|
| `PORT` | Hayır | Sunucu portu (varsayılan: `8080`) |
| `APP_ENV` | Hayır | Ortam (`development` / `production`) |
| `MAPILLARY_ACCESS_TOKEN` | Evet* | Mapillary erişim token'ı |
| `WIRO_API_KEY` | Evet* | Wiro AI API anahtarı |
| `WIRO_SECRET` | Hayır | Wiro AI secret |
| `WIRO_TEXT_MODEL` | Hayır | Türkçe yazım düzeltme modeli (ör. `openai/gpt-oss-20b`) |
| `WIRO_TEXT_POLISH` | Hayır | `false` ise metin modeli düzeltmesi kapatılır |
| `DB_PATH` | Hayır | SQLite dosya yolu (varsayılan: `./data/messages.db`) |

\* Risk analizi endpoint'i için gerekli.

---

## API Endpoint'leri

### Sistem

| Metod | Endpoint | Açıklama |
|-------|----------|----------|
| `GET` | `/health` | Sistem sağlık kontrolü |

### Risk ve Rota

| Metod | Endpoint | Açıklama |
|-------|----------|----------|
| `POST` | `/api/v1/risk` | Deprem enkaz risk analizi |
| `POST` | `/api/v1/route` | Güvenli rota hesaplama |
| `GET` | `/api/v1/assembly-points` | Toplanma alanları listesi |

### Mesajlaşma

| Metod | Endpoint | Açıklama |
|-------|----------|----------|
| `POST` | `/api/v1/messages` | Tek mesaj gönder |
| `GET` | `/api/v1/messages?room_id=&since=&limit=` | Mesajları listele |
| `POST` | `/api/v1/messages/sync` | Offline senkronizasyon |

### Offline Harita

| Metod | Endpoint | Açıklama |
|-------|----------|----------|
| `GET` | `/api/v1/offline/regions` | İndirilebilir bölgeler |
| `GET` | `/api/v1/offline/bundle/{region_id}` | Bölge veri paketi |
| `GET` | `/api/v1/offline/assembly-points.geojson` | Toplanma alanları (GeoJSON) |

---

## Örnek İstekler

### Risk analizi

```bash
curl -X POST https://c-backend-2enq.onrender.com/api/v1/risk \
  -H "Content-Type: application/json" \
  -d '{"lat":41.0369,"lng":28.9850}'
```

Örnek yanıt:

```json
{
  "score": 70,
  "level": "YÜKSEK",
  "comment": "Bu bölgede yüksek enkaz riski bulunmaktadır...",
  "recommendations": [
    "Binaların deprem güçlendirmesi için yapı denetimi yaptırın",
    "Dar sokaklar için alternatif tahliye güzergahı belirleyin",
    "En yakın toplanma alanına giden yolu önceden öğrenin"
  ],
  "analyzed_at": "2026-06-06T10:37:37Z"
}
```

### Toplanma alanları

```bash
curl https://c-backend-2enq.onrender.com/api/v1/assembly-points
```

### Mesaj senkronizasyonu

```bash
curl -X POST https://c-backend-2enq.onrender.com/api/v1/messages/sync \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "cihaz-uuid",
    "room_id": "aile",
    "last_sync_at": null,
    "outgoing": [{
      "client_id": "msg-uuid",
      "sender": "Ahmet",
      "content": "İyiyim",
      "created_at": "2026-06-06T10:00:00Z"
    }]
  }'
```

---

## Risk Analizi Akışı

```
Konum (lat, lng)
    ↓
Mapillary → Sokak görüntüsü (50m + bbox fallback)
    ↓
Blur → KVKK altyapısı (mock)
    ↓
Wiro AI → Risk skoru + Türkçe yorum + öneriler
    ↓
Türkçe düzeltme → Prompt sızıntısı filtresi + yazım düzeltme
    ↓
RiskResponse JSON
```

**Risk seviyeleri:**

| Skor | Seviye |
|------|--------|
| 0–30 | DÜŞÜK |
| 31–60 | ORTA |
| 61–100 | YÜKSEK |

---

## Render.com Deploy

1. GitHub reposunu Render'a bağla
2. `render.yaml` otomatik yapılandırmayı uygular
3. Environment sekmesinden API anahtarlarını gir:
   - `MAPILLARY_ACCESS_TOKEN`
   - `WIRO_API_KEY`
   - `WIRO_SECRET`
4. Deploy tamamlandığında `/health` ile kontrol et

> **Not:** Render free tier'da SQLite verileri redeploy sonrası silinebilir. Kalıcı mesajlaşma için Persistent Disk veya harici veritabanı kullanın.

---

## Mobil Entegrasyon

Mobil uygulama bu API'yi REST üzerinden kullanır. Temel `BASE_URL`:

```
https://c-backend-2enq.onrender.com
```

- Risk endpoint'i yavaştır (~15–60 sn); timeout'u 90 sn tutun
- `404` yanıtı = konumda sokak görüntüsü yok (sunucu hatası değil)
- Offline harita tile'ları mobilde indirilir; backend yalnızca paket manifest'i sağlar

---

## Lisans

Bu proje eğitim ve afet farkındalığı amaçlı geliştirilmiştir.
