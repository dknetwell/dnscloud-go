# Многостадийная сборка
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Устанавливаем зависимости для сборки
RUN apk add --no-cache git ca-certificates

# Копируем зависимости
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходный код
COPY *.go ./

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -o dns-proxy .

# Финальный образ
FROM alpine:3.18

# Устанавливаем wget для health checks и libcap для NET_BIND_SERVICE
RUN apk add --no-cache ca-certificates tzdata libcap wget && \
    addgroup -g 1000 dns && \
    adduser -D -u 1000 -G dns dns

WORKDIR /app

# Копируем бинарник
COPY --from=builder /app/dns-proxy /app/

# Копируем конфигурацию
COPY config /app/config/

# Настраиваем пользователя
USER dns

# Порты
EXPOSE 5353 8054

# Запуск
CMD ["/app/dns-proxy"]
