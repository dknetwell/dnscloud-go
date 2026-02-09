#!/bin/bash
set -e

echo "========================================"
echo "DNS Security Proxy - Setup Script"
echo "========================================"
echo "CoreDNS 1.14.1 + Standalone Go Proxy"
echo ""

# Проверка Docker
if ! command -v docker &> /dev/null; then
    echo "❌ ERROR: Docker is not installed"
    exit 1
fi

# Создаем директории
echo "📁 Creating directories..."
mkdir -p config data/blocklists certs

# Создаем .env если его нет
if [ ! -f .env ]; then
    echo "⚙️  Creating .env file from template..."
    cp .env.example .env
    echo ""
    echo "⚠️  Please edit .env file and set your CLOUD_API_KEY"
    echo "   Then run this script again."
    echo ""
    exit 0
fi

# Загружаем переменные
source .env

# Проверка обязательных переменных
if [ -z "$CLOUD_API_KEY" ] || [ "$CLOUD_API_KEY" = "your_api_key_here" ]; then
    echo "❌ ERROR: CLOUD_API_KEY is not set in .env file"
    exit 1
fi

# Создаем тестовые сертификаты (для DoH/DoT)
if [ ! -f certs/server.crt ]; then
    echo "🔐 Generating test TLS certificates..."
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout certs/server.key -out certs/server.crt \
        -days 365 -subj "/CN=dns.localhost"
fi

echo "🐳 Building and starting containers..."
docker compose up -d --build

echo ""
echo "⏳ Waiting for services to start..."
sleep 15

# Проверка
echo ""
echo "✅ Services Status:"
docker compose ps

echo ""
echo "🧪 Testing DNS service..."
if command -v dig &> /dev/null; then
    echo "Testing with dig..."
    if timeout 5 dig @127.0.0.1 google.com +short > /dev/null 2>&1; then
        echo "✅ DNS service is responding"
    else
        echo "⚠️  DNS test failed"
    fi
fi

echo ""
echo "🧪 Testing health endpoints..."
if curl -s http://localhost:8080/health | grep -q "healthy"; then
    echo "✅ CoreDNS health: OK"
else
    echo "⚠️  CoreDNS health: FAILED"
fi

if curl -s http://localhost:8054/health | grep -q "healthy"; then
    echo "✅ DNS Proxy health: OK"
else
    echo "⚠️  DNS Proxy health: FAILED"
fi

echo ""
echo "========================================"
echo "🚀 Setup completed successfully!"
echo "========================================"
echo ""
echo "🌐 Services:"
echo "  Basic DNS:      udp://127.0.0.1:53"
echo "  DNS-over-TCP:   tcp://127.0.0.1:53"
echo "  DNS-over-TLS:   tls://127.0.0.1:853"
echo "  DNS-over-HTTPS: https://127.0.0.1/dns-query"
echo "  Health:         http://localhost:8080/health"
echo "  Metrics:        http://localhost:9091/metrics"
echo "  Proxy Health:   http://localhost:8054/health"
echo "  Proxy Metrics:  http://localhost:8054/metrics"
echo ""
echo "🔧 Test commands:"
echo "  dig @127.0.0.1 google.com"
echo "  curl http://localhost:8054/stats"
echo "  docker compose logs -f dns-proxy"
echo ""
echo "⏹️  Stop services:"
echo "  docker compose down"
echo "========================================"
