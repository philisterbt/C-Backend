# ---- Derleme aşaması ----
FROM golang:1.25-alpine AS builder

# Çalışma dizinini ayarla
WORKDIR /app

# Bağımlılık dosyalarını kopyala ve indir
COPY go.mod go.sum* ./
RUN go mod download

# Tüm kaynak kodunu kopyala
COPY . .

# Uygulamayı derle; CGO devre dışı (Alpine uyumluluğu için)
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/main.go

# ---- Çalışma aşaması ----
FROM alpine:latest

# Güvenlik güncellemelerini yükle
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Derlenmiş binary'yi kopyala
COPY --from=builder /app/main .

# Render.com'un beklediği port
EXPOSE 8080

# Uygulamayı başlat
CMD ["./main"]
