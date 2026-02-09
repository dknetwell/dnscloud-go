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

# Проверяем и исправляем Corefile
echo "🔧 Checking Corefile..."
if [ -f Corefile ]; then
    if grep -q "import .:53" Corefile; then
        echo "  Fixing Corefile syntax..."
        cat > Corefile << 'EOF'
.:53 {
    forward . dns-proxy:5353 {
        max_concurrent 10000
        expire 30s
        health_check 10s
        prefer_udp
    }
    
    cache {
        success 10000 3600 300
        denial 10000 3600 300
        prefetch 1000 10m 80%
    }
    
    log . {
        class all
        format combined
    }
    
    errors {
        consolidate 5s 100
    }
    
    prometheus :9091
    health :8080
    bind 0.0.0.0
    bufsize 1232
}

tls://.:853 {
    forward . dns-proxy:5353 {
        max_concurrent 10000
        expire 30s
        health_check 10s
    }
    
    cache {
        success 10000 3600 300
        denial 10000 3600 300
    }
    
    tls /certs/server.crt /certs/server.key
    log
    errors
    prometheus :9091
}

https://.:443 {
    forward . dns-proxy:5353 {
        max_concurrent 10000
        expire 30s
        health_check 10s
    }
    
    cache {
        success 10000 3600 300
        denial 10000 3600 300
    }
    
    tls /certs/server.crt /certs/server.key
    log
    errors
    prometheus :9091
}
EOF
    fi
fi

# Создаем тестовые сертификаты
if [ ! -f certs/server.crt ]; then
    echo "🔐 Generating test TLS certificates..."
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout certs/server.key -out certs/server.crt \
        -days 365 -subj "/CN=dns.localhost" 2>/dev/null || \
    echo "⚠️  Using existing certificates"
fi

# Убираем version из docker-compose.yml чтобы избежать warning
if [ -f docker-compose.yml ] && grep -q "^version:" docker-compose.yml; then
    echo "📝 Updating docker-compose.yml..."
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
echo "⏳ Waiting for services to start (15 seconds)..."
sleep 15

echo ""
echo "✅ Services Status:"
docker compose ps

echo ""
echo "🧪 Testing services..."
echo ""

# Проверка CoreDNS - используем простой curl с таймаутом
echo "Testing CoreDNS health..."
if timeout 5 curl -s http://localhost:8080/health 2>/dev/null | grep -q "OK"; then
    echo "  ✅ CoreDNS health: OK"
else
    echo "  ⚠️  CoreDNS health: FAILED"
    echo "  CoreDNS logs:"
    docker compose logs coredns --tail=5 2>/dev/null || true
fi

# Проверка DNS Proxy health
echo "Testing DNS Proxy health..."
if timeout 5 curl -s http://localhost:8054/health 2>/dev/null | grep -q "healthy"; then
    echo "  ✅ DNS Proxy health: OK"
else
    echo "  ⚠️  DNS Proxy health: FAILED"
    echo "  DNS Proxy logs (last 10 lines):"
    docker compose logs dns-proxy --tail=10 2>/dev/null || true
fi

# Проверка Valkey - используем redis-cli с echo (не интерактивно)
echo "Testing Valkey connection..."
if timeout 5 docker compose exec -T valkey sh -c "echo 'ping' | valkey-cli -a '$VALKEY_PASSWORD' --no-auth-warning" 2>/dev/null | grep -q "PONG"; then
    echo "  ✅ Valkey: OK"
else
    echo "  ⚠️  Valkey: FAILED or not ready"
fi

echo ""
echo "🧪 Testing DNS service..."
if command -v dig &> /dev/null; then
    echo "Testing with dig..."
    
    # Ждем еще немного
    sleep 5
    
    if timeout 10 dig @127.0.0.1 google.com +short +tcp 2>&1 | head -1 | grep -q -E "^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$|^[a-f0-9:]+$"; then
        echo "  ✅ DNS over TCP: OK"
    else
        echo "  ⚠️  DNS over TCP: FAILED"
    fi
    
    if timeout 10 dig @127.0.0.1 google.com +short 2>&1 | head -1 | grep -q -E "^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$|^[a-f0-9:]+$"; then
        echo "  ✅ DNS over UDP: OK"
    else
        echo "  ⚠️  DNS over UDP: FAILED"
    fi
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
echo "🔧 Quick tests:"
echo "  docker compose logs -f dns-proxy      # View logs"
echo "  dig @127.0.0.1 google.com             # Test DNS"
echo "  curl http://localhost:8054/health     # Check health"
echo ""
echo "⏹️  To stop: docker compose down"
echo "========================================"
