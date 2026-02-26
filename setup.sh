#!/bin/bash
set -e

echo "========================================"
echo "DNS Security Proxy - Setup"
echo "========================================"

trap 'echo ""; echo "Interrupted"; exit 1' INT TERM

if ! command -v docker &> /dev/null; then
    echo "Docker not installed"
    exit 1
fi

mkdir -p config certs

# config.yaml — единственное место истины для всех настроек.
# Не создаём автоматически — он должен быть в репозитории.
if [ ! -f config/config.yaml ]; then
    echo "ERROR: config/config.yaml not found"
    echo "  Copy from repository: cp config/config.yaml.example config/config.yaml"
    exit 1
fi

# .env — секреты и инфраструктурные параметры (не в репозитории).
if [ ! -f .env ]; then
    echo "ERROR: .env not found"
    echo "  Copy and fill: cp .env.example .env && vim .env"
    exit 1
fi

if [ ! -f certs/server.crt ]; then
    echo "Generating self-signed certificates..."
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout certs/server.key \
        -out certs/server.crt \
        -days 365 \
        -subj "/CN=dns.local"
fi

chmod 644 certs/* || true

docker compose down || true
docker compose build --no-cache
docker compose up -d

echo "Waiting for services..."
sleep 15

docker compose ps

echo ""
echo "DNS: udp/tcp 53"
echo "DoT: 853"
echo "DoH: https://localhost/dns-query"
echo "Stats: http://localhost:8080/stats"
echo "Metrics: http://localhost:8080/metrics"
