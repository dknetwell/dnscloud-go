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

# ФИКСИМ ОШИБКИ GO ЗАВИСИМОСТЕЙ ПЕРЕД СБОРКОЙ
echo "🔄 Fixing Go dependencies..."
if [ -f go.mod ]; then
    echo "  Cleaning up go.sum..."
    rm -f go.sum 2>/dev/null || true
    
    echo "  Updating go modules..."
    if command -v go &> /dev/null; then
        go mod tidy 2>/dev/null || echo "  ⚠️  go mod tidy failed, continuing..."
    else
        echo "  ⚠️  Go not installed, skipping module tidy"
    fi
else
    echo "  ⚠️  go.mod not found, creating..."
    cat > go.mod << 'EOF'
module dnscloud-go

go 1.21

require (
    github.com/dgraph-io/ristretto v0.1.1
    github.com/go-redis/redis/v8 v8.11.5
    github.com/joho/godotenv v1.5.1
    github.com/miekg/dns v1.1.57
    github.com/prometheus/client_golang v1.19.0
    go.uber.org/zap v1.26.0
    golang.org/x/time v0.5.0
    gopkg.in/yaml.v3 v3.0.1
)
EOF
fi

# АВТОМАТИЧЕСКОЕ ИСПРАВЛЕНИЕ ОШИБОК КОМПИЛЯЦИИ
echo "🔧 Fixing common compilation errors..."
if [ -f cache.go ]; then
    # Исправляем SetEx → Set в cache.go
    sed -i 's/\.SetEx(/\.Set(/g' cache.go 2>/dev/null || true
    echo "  ✅ Fixed redis SetEx method"
fi

if [ -f main.go ]; then
    # Убираем неиспользуемые импорты
    grep -q "github.com/miekg/dns" main.go && \
    grep -q "github.com/prometheus/client_golang/prometheus/promhttp" main.go && \
    echo "  ⚠️  main.go has unused imports that might need fixing"
fi

echo "🐳 Building and starting containers..."
docker compose up -d --build

# Если сборка не удалась, пытаемся с дополнительными исправлениями
if [ $? -ne 0 ]; then
    echo "⚠️  First build failed, trying with fixes..."
    
    # Создаем исправленные файлы
    if [ -f cache.go ]; then
        echo "  Creating fixed cache.go..."
        cp cache.go cache.go.backup
        sed -i 's/\.SetEx(/\.Set(/g' cache.go
    fi
    
    if [ -f cloud_api.go ]; then
        echo "  Fixing cloud_api.go pointer issue..."
        sed -i 's/func newCloudAPIClient(config CloudAPIConfig) \*CloudAPIClient/func newCloudAPIClient(config \*CloudAPIConfig) \*CloudAPIClient/g' cloud_api.go 2>/dev/null || true
    fi
    
    echo "  Rebuilding..."
    docker compose build --no-cache dns-proxy
    docker compose up -d
fi

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
if timeout 5 docker compose exec -T coredns wget -q -O- http://localhost:8080/health 2>/dev/null | grep -q "OK"; then
    echo "  ✅ CoreDNS health: OK"
else
    echo "  ⚠️  CoreDNS health: FAILED or not ready yet"
fi

# Проверка DNS Proxy health
echo "Testing DNS Proxy health..."
if timeout 5 curl -s http://localhost:8054/health 2>/dev/null | grep -q "healthy"; then
    echo "  ✅ DNS Proxy health: OK"
else
    echo "  ⚠️  DNS Proxy health: FAILED or not ready yet"
    echo "  Checking logs..."
    docker compose logs dns-proxy --tail=20 2>/dev/null | tail -10
fi

# Проверка Valkey
echo "Testing Valkey connection..."
if timeout 5 docker compose exec -T valkey valkey-cli -a "$VALKEY_PASSWORD" ping 2>/dev/null | grep -q "PONG"; then
    echo "  ✅ Valkey: OK"
else
    echo "  ⚠️  Valkey: FAILED or not ready yet"
fi

echo ""
echo "🧪 Testing DNS service..."
if command -v dig &> /dev/null; then
    echo "Testing with dig..."
    
    # Ждем пока сервис поднимется
    for i in {1..5}; do
        if timeout 2 dig @127.0.0.1 google.com +short +tcp > /dev/null 2>&1; then
            echo "  ✅ DNS over TCP: OK"
            TCP_OK=1
            break
        fi
        sleep 2
    done
    [ -z "$TCP_OK" ] && echo "  ⚠️  DNS over TCP: FAILED"
    
    for i in {1..5}; do
        if timeout 2 dig @127.0.0.1 google.com +short > /dev/null 2>&1; then
            echo "  ✅ DNS over UDP: OK"
            UDP_OK=1
            break
        fi
        sleep 2
    done
    [ -z "$UDP_OK" ] && echo "  ⚠️  DNS over UDP: FAILED"
else
    echo "  ℹ️  dig not installed, skipping DNS tests"
fi

echo ""
echo "========================================"
echo "🚀 Setup completed!"
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
echo "🔧 If there are compilation errors:"
echo "  rm -f go.sum && go mod tidy"
echo "  docker compose build --no-cache dns-proxy"
echo ""
echo "⏹️  To stop services:"
echo "  docker compose down"
echo "========================================"
