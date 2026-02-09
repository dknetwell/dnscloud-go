# Только добавить эту строку в самом начале Dockerfile
# Это гарантирует что go.sum будет создан

# Многостадийная сборка
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Устанавливаем зависимости для сборки
RUN apk add --no-cache git ca-certificates

# Копируем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o dns-proxy main.go

# Финальный образ
FROM alpine:latest

WORKDIR /app

# Копируем бинарник из builder
COPY --from=builder /app/dns-proxy /app/

# Копируем конфигурацию
COPY config/config.yaml /app/config/config.yaml

# Запуск
CMD ["/app/dns-proxy"]
