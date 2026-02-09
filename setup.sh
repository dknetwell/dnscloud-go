#!/bin/bash
set -e

echo "========================================"
echo "DNS Security Proxy - Setup Script"
echo "========================================"
echo "CoreDNS 1.14.1 + Go DNS Proxy"
echo ""

# Проверка Docker
if ! command -v docker &> /dev/null; then
    echo "❌ ERROR: Docker is not installed"
    echo "  Please install Docker first:"
    echo "  https://docs.docker.com/engine/install/"
    exit 1
fi

# Проверка Docker Compose
if ! docker compose version &> /dev/null; then
    echo "❌ ERROR: Docker Compose is not installed"
    echo "  Please install Docker Compose first:"
    echo "  https://docs.docker.com/compose/install/"
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
    echo "   nano .env  # Edit the file"
    echo "   # Set CLOUD_API_KEY=your_actual_api_key"
    echo ""
    exit 0
fi

# Загружаем переменные
if [ -f .env ]; then
    export $(grep -v '^#' .env | xargs)
fi

# Проверка обязательных переменных
if [ -z "$CLOUD_API_KEY" ] || [ "$CLOUD_API_KEY" = "your_api_key_here" ]; then
    echo "❌ ERROR: CLOUD_API_KEY is not set in .env file"
    echo "   Please edit .env and set your API key"
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
  total: 95ms          # Общий таймаут (SLA 95ms + 5ms CoreDNS = 100ms)
  cloud_api: 50ms      # Таймаут Cloud API
  cache_read: 5ms      # Чтение из кеша
  cache_write: 100ms   # Запись в кеш (async)

cloud_api:
  url: "${CLOUD_API_URL}"
  key: "${CLOUD_API_KEY}"
  rate_limit: 5
  timeout: 50ms

sinkholes:
  categories:
    1: "0.0.0.0"    # malware
    2: "0.0.0.1"    # phishing
    3: "0.0.0.2"    # C&C
    8: "0.0.0.3"    # proxy
  default: "0.0.0.0"

ttl:
  by_category:
    0: 300      # benign (5 минут)
    1: 3600     # malware (1 час)
    2: 3600     # phishing (1 час)
    3: 3600     # C&C (1 час)
    8: 3600     # proxy (1 час)
    9: 86400    # allowlist (24 часа)
  fallback: 300

cache:
  strategy: "hybrid"
  valkey_address: "valkey:6379"
  valkey_password: "${VALKEY_PASSWORD}"
  memory_max_size_mb: 100
EOF
fi

# Создаем тестовые сертификаты (для DoH/DoT)
if [ ! -f certs/server.crt ]; then
    echo "🔐 Generating test TLS certificates..."
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout certs/server.key -out certs/server.crt \
        -days 365 -subj "/CN=dns.localhost" 2>/dev/null || \
    echo "⚠️  Could not generate certificates (openssl not available)"
    echo "   Using self-signed certificates from repository if available..."
fi

echo "🐳 Building and starting containers..."
docker compose up -d --build

echo ""
echo "⏳ Waiting for services to start..."
sleep 10

echo ""
echo "✅ Services Status:"
docker compose ps

echo ""
echo "🧪 Testing services..."
echo ""

# Проверка CoreDNS health
echo "Testing CoreDNS health..."
if docker compose exec -T coredns wget -q -O- http://localhost:8080/health 2>/dev/null | grep -q "OK"; then
    echo "  ✅ CoreDNS health: OK"
else
    echo "  ⚠️  CoreDNS health: FAILED"
fi

# Проверка DNS Proxy health
echo "Testing DNS Proxy health..."
if curl -s http://localhost:8054/health 2>/dev/null | grep -q "healthy"; then
    echo "  ✅ DNS Proxy health: OK"
else
    echo "  ⚠️  DNS Proxy health: FAILED"
fi

# Проверка Valkey
echo "Testing Valkey connection..."
if docker compose exec -T valkey valkey-cli -a "$VALKEY_PASSWORD" ping 2>/dev/null | grep -q "PONG"; then
    echo "  ✅ Valkey: OK"
else
    echo "  ⚠️  Valkey: FAILED"
fi

echo ""
echo "🧪 Testing DNS service..."
if command -v dig &> /dev/null; then
    echo "Testing with dig..."
    if timeout 5 dig @127.0.0.1 google.com +short +tcp > /dev/null 2>&1; then
        echo "  ✅ DNS over TCP: OK"
    else
        echo "  ⚠️  DNS over TCP: FAILED"
    fi
    
    if timeout 5 dig @127.0.0.1 google.com +short > /dev/null 2>&1; then
        echo "  ✅ DNS over UDP: OK"
    else
        echo "  ⚠️  DNS over UDP: FAILED"
    fi
else
    echo "  ℹ️  dig not installed, skipping DNS tests"
fi

echo ""
echo "========================================"
echo "🚀 Setup completed successfully!"
echo "========================================"
echo ""
echo "🌐 Services:"
echo "  Basic DNS (UDP):    udp://127.0.0.1:53"
echo "  Basic DNS (TCP):    tcp://127.0.0.1:53"
echo "  DNS-over-TLS:       tls://127.0.0.1:853"
echo "  DNS-over-HTTPS:     https://127.0.0.1/dns-query"
echo "  CoreDNS Health:     http://localhost:8080/health"
echo "  CoreDNS Metrics:    http://localhost:9091/metrics"
echo "  DNS Proxy Health:   http://localhost:8054/health"
echo "  DNS Proxy Metrics:  http://localhost:8054/metrics"
echo ""
echo "🔧 Management commands:"
echo "  docker compose ps           # Status"
echo "  docker compose logs -f      # View logs"
echo "  docker compose restart      # Restart"
echo "  docker compose down         # Stop"
echo ""
echo "📊 Test commands:"
echo "  dig @127.0.0.1 google.com           # Test DNS"
echo "  dig @127.0.0.1 google.com +tcp      # Test DNS over TCP"
echo "  curl http://localhost:8054/stats    # View statistics"
echo "  curl http://localhost:8054/metrics  # View metrics"
echo ""
echo "⚠️  Next steps:"
echo "  1. Configure your DNS clients to use 127.0.0.1"
echo "  2. Monitor logs: docker compose logs -f dns-proxy"
echo "  3. Check metrics: http://localhost:8054/metrics"
echo ""
echo "⏹️  To stop services:"
echo "  docker compose down"
echo "========================================"
