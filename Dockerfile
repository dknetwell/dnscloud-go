# ===== BUILD STAGE =====
FROM golang:1.24.13-alpine AS builder

WORKDIR /app

# git не нужен — код копируется через COPY, не клонируется
RUN apk add --no-cache ca-certificates

# Копируем go.mod и go.sum отдельно для кэширования зависимостей
COPY go.mod go.sum* ./
RUN go mod download

# Копируем весь проект
COPY . .

RUN go mod tidy && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dns-proxy .

# ===== RUNTIME STAGE =====
FROM alpine:3.18

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -g 1000 dns \
    && adduser -D -u 1000 -G dns dns

COPY --from=builder /app/dns-proxy /app/dns-proxy

USER dns

EXPOSE 53/udp
EXPOSE 53/tcp
EXPOSE 8080

ENTRYPOINT ["/app/dns-proxy"]
