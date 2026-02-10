#!/bin/bash
set -e

echo "========================================"
echo "DNS Security Proxy - Setup Script"
echo "========================================"
echo "CoreDNS 1.14.1 + Go DNS Proxy"
echo ""

# Устанавливаем обработчик прерывания
trap 'echo ""; echo "🛑 Script interrupted by user"; exit 1' INT TERM

# Проверка Docker
if ! command -v docker &> /dev/null; then
    echo "❌ ERROR: Docker is not installed"
    exit 1
fi

echo "📁 Creating directories..."
mkdir -p config certs

# Создаем .env если его нет
if [ ! -f .env ]; then
    echo "⚙️  Creating .env file from template..."
    if [ -f .env.example ]; then
        cp .env.example .env
    else
        cat > .env << 'EOF'
# Обязательные параметры
CLOUD_API_KEY=your_api_key_here

# Опциональные
VALKEY_PASSWORD=SecurePass123!
CLOUD_API_URL=https://172.16.10.33/api/
LOG_LEVEL=debug
RATE_LIMIT_RPS=5
EOF
    fi

    echo ""
    echo "⚠️  IMPORTANT: Please edit .env file and set your CLOUD_API_KEY"
    echo "   Then run this script again."
    echo ""
    exit 0
fi

# Загружаем переменные
source .env 2>/dev/null || true

# Проверка обязательных переменных
if [ -z "$CLOUD_API_KEY" ] || [ "$CLOUD_API_KEY" = "your_api_key_here" ]; then
    echo "❌ ERROR: CLOUD_API_KEY is not set in .env file"
    exit 1
fi

# Проверяем наличие config.yaml
if [ ! -f config/config.yaml ]; then
    echo "❌ ERROR: config/config.yaml not found!"
    exit 1
fi

# Проверяем наличие go.mod и делаем go mod tidy если нужно
if [ -f go.mod ]; then
    echo "📦 Checking Go dependencies..."
    if command -v go &> /dev/null; then
        echo "  Running go mod tidy..."
        go mod tidy 2>&1 | grep -v "warning: " || true
        echo "  ✅ Dependencies updated"
    else
        echo "  ⚠️  Go not installed, skipping go mod tidy"
    fi
fi

# Создаем сертификаты если нет
if [ ! -f certs/server.crt ]; then
    echo "🔐 Generating self-signed certificates..."
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout certs/server.key -out certs/server.crt \
        -days 365 -subj "/CN=dns.localhost" 2>/dev/null || true
fi

chmod 644 certs/* 2>/dev/null || true

echo "🐳 Building and starting containers..."
docker compose down 2>/dev/null || true
docker compose build --no-cache
docker compose up -d

echo ""
echo "⏳ Waiting for services to start (60 seconds)..."
sleep 60

echo ""
echo "✅ Services Status:"
docker compose ps

echo ""
echo "🧪 Testing services..."
echo ""

# Проверка DNS Proxy
echo "Testing DNS Proxy health..."
if docker compose exec -T dns-proxy wget -q -O- http://localhost:8054/health 2>/dev/null | grep -q "healthy"; then
    echo "  ✅ DNS Proxy health: OK"
else
    echo "  ⚠️  DNS Proxy health: FAILED"
fi

echo ""
echo "🧪 Testing DNS..."
if command -v dig &> /dev/null; then
    echo "Testing DNS with dig..."

    # Тестируем DNS запросы
    for test_domain in "yandex.ru" "google.com" "example.com" "malware.com"; do
        echo -n "  DNS query $test_domain: "
        if result=$(timeout 5 dig @127.0.0.1 $test_domain +short 2>&1); then
            if echo "$result" | grep -q -E "^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$"; then
                echo "✅ Got IP: $result"
            else
                echo "⚠️  No IP found (response: $result)"
            fi
        else
            echo "❌ No response"
        fi
    done
fi

echo ""
echo "========================================"
echo "🚀 Setup completed!"
echo "========================================"
echo ""
echo "🌐 Services:"
echo "  DNS (UDP/TCP): 127.0.0.1:53"
echo "  DoT: tls://127.0.0.1:853"
echo "  DoH: https://127.0.0.1/dns-query"
echo "  Health checks:"
echo "    CoreDNS: http://localhost:8080/health"
echo "    DNS Proxy: http://localhost:8054/health"
echo ""
echo "🔍 Debug commands:"
echo "  Check logs: docker compose logs -f dns-proxy"
echo "  Test DNS: dig @127.0.0.1 google.com +short"
echo "  Check cache: docker exec dns-proxy wget -q -O- http://localhost:8054/stats"
echo ""
echo "⏹️  To stop: docker compose down"
echo "========================================"
