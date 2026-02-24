# ===== BUILD STAGE =====
FROM golang:1.24.13-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates

# Копируем весь проект сразу
COPY . .

# Если go.sum нет — создастся автоматически
RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dns-proxy .

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
