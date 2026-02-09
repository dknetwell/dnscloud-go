#!/bin/bash
set -e

echo "========================================"
echo "DNS Security Proxy - Setup Script"
echo "========================================"
echo "CoreDNS 1.14.1 + Go DNS Proxy"
echo ""

# Устанавливаем обработчик прерывания для Ctrl+C
trap 'echo ""; echo "🛑 Script interrupted by user"; kill 0; exit 1' INT TERM

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
LOG_LEVEL=info
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

# Создаем конфигурационный файл если его нет
if [ ! -f config/config.yaml ]; then
    echo "📝 Creating config.yaml..."
    cat > config/config.yaml << 'EOF'
# DNS Proxy конфигурация
dns_listen: ":5353"
http_listen: ":8054"
log_level: "${LOG_LEVEL:-info}"

# ТАЙМАУТЫ ДЛЯ КОНТРОЛЯ SLA 100ms
timeouts:
  total: 95ms
  cloud_api: 50ms
  cache_read: 5ms
  cache_write: 100ms

cloud_api:
  url: "${CLOUD_API_URL}"
  key: "${CLOUD_API_KEY}"
  rate_limit: 5
  timeout: 50ms

sinkholes:
  categories:
    1: "0.0.0.0"
    2: "0.0.0.1"
    3: "0.0.0.2"
    8: "0.0.0.3"
  default: "0.0.0.0"

ttl:
  by_category:
    0: 300
    1: 3600
    2: 3600
    3: 3600
    8: 3600
    9: 86400
  fallback: 300

cache:
  strategy: "hybrid"
  valkey_address: "valkey:6379"
  valkey_password: "${VALKEY_PASSWORD}"
  memory_max_size_mb: 100
EOF
fi

# Исправляем права на сертификаты
echo "🔐 Setting up TLS certificates..."
if [ ! -f certs/server.crt ]; then
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout certs/server.key -out certs/server.crt \
        -days 365 -subj "/CN=dns.localhost" 2>/dev/null || \
    echo "⚠️  Could not generate certificates"
fi

# Исправляем права доступа к сертификатам
chmod 644 certs/* 2>/dev/null || true

# Убираем version из docker-compose.yml если есть
if [ -f docker-compose.yml ] && grep -q "^version:" docker-compose.yml; then
    echo "📝 Removing version from docker-compose.yml..."
    sed -i '/^version:/d' docker-compose.yml
fi

# Обновляем Go зависимости
echo "🔄 Updating Go dependencies..."
rm -f go.sum 2>/dev/null || true
if command -v go &> /dev/null; then
    go mod tidy 2>/dev/null || true
fi

# Исправляем common errors
echo "🔧 Fixing compilation errors..."
if [ -f cache.go ]; then
    sed -i 's/\.SetEx(/\.Set(/g' cache.go 2>/dev/null || true
fi

echo "🐳 Building and starting containers..."
docker compose down 2>/dev/null || true
docker compose up -d --build

echo ""
echo "⏳ Waiting for services to start (30 seconds)..."
sleep 30

echo ""
echo "✅ Services Status:"
timeout 5 docker compose ps || echo "⚠️  Could not get service status"

echo ""
echo "🧪 Testing services..."
echo ""

# Проверка CoreDNS - с таймаутом
echo "Testing CoreDNS health..."
if timeout 10 curl -s -f http://localhost:8080/health 2>/dev/null | grep -q "OK"; then
    echo "  ✅ CoreDNS health: OK"
else
    echo "  ⚠️  CoreDNS health: FAILED or starting"
    timeout 5 docker compose logs coredns --tail=3 2>/dev/null || true
fi

# Проверка DNS Proxy health
echo "Testing DNS Proxy health..."
if timeout 10 curl -s -f http://localhost:8054/health 2>/dev/null | grep -q "healthy"; then
    echo "  ✅ DNS Proxy health: OK"
else
    echo "  ⚠️  DNS Proxy health: FAILED or starting"
    timeout 5 docker compose logs dns-proxy --tail=3 2>/dev/null || true
fi

# Проверка Valkey - НЕ ИСПОЛЬЗУЕМ docker exec который зависает
echo "Testing Valkey connection..."
VALKEY_STATUS=$(timeout 5 docker compose ps valkey --format json 2>/dev/null | grep -o '"Status":"[^"]*"' | cut -d'"' -f4 || echo "")
if [[ "$VALKEY_STATUS" == "healthy" ]]; then
    echo "  ✅ Valkey: Container is healthy"
else
    echo "  ⚠️  Valkey: Status is '$VALKEY_STATUS'"
fi

echo ""
echo "🧪 Testing DNS service..."
if command -v dig &> /dev/null; then
    echo "Testing with dig..."
    
    # Даем больше времени
    sleep 10
    
    # Тест UDP с таймаутом
    if UDP_OUTPUT=$(timeout 10 dig @127.0.0.1 example.com +short 2>&1); then
        if echo "$UDP_OUTPUT" | head -1 | grep -q -E "^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$|^[a-f0-9:]+$"; then
            echo "  ✅ DNS over UDP: Responding"
        else
            echo "  ⚠️  DNS over UDP: Got response but no IP"
        fi
    else
        echo "  ⚠️  DNS over UDP: Timeout or error"
    fi
    
    # Тест TCP с таймаутом
    if TCP_OUTPUT=$(timeout 10 dig @127.0.0.1 example.com +short +tcp 2>&1); then
        if echo "$TCP_OUTPUT" | head -1 | grep -q -E "^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$|^[a-f0-9:]+$"; then
            echo "  ✅ DNS over TCP: Responding"
        else
            echo "  ⚠️  DNS over TCP: Got response but no IP"
        fi
    else
        echo "  ⚠️  DNS over TCP: Timeout or error"
    fi
else
    echo "  ℹ️  dig not installed, skipping DNS tests"
fi

echo ""
echo "========================================"
echo "🚀 Setup attempt completed!"
echo "========================================"
echo ""
echo "🌐 Services configured:"
echo "  Basic DNS (UDP):    udp://127.0.0.1:53"
echo "  Basic DNS (TCP):    tcp://127.0.0.1:53"
echo "  DNS-over-TLS:       tls://127.0.0.1:853"
echo "  DNS-over-HTTPS:     https://127.0.0.1/dns-query"
echo "  CoreDNS Health:     http://localhost:8080/health"
echo "  CoreDNS Metrics:    http://localhost:9091/metrics"
echo "  DNS Proxy Health:   http://localhost:8054/health"
echo "  DNS Proxy Metrics:  http://localhost:8054/metrics"
echo ""
echo "🔧 Next steps:"
echo "  1. Check if services are running: docker compose ps"
echo "  2. View logs: docker compose logs -f"
echo "  3. Test DNS: dig @127.0.0.1 google.com"
echo ""
echo "⚠️  If DoH/DoT don't work:"
echo "  Check certificate permissions: chmod 644 certs/*"
echo ""
echo "⏹️  To stop: docker compose down"
echo "========================================"
