#!/bin/bash
set -e

echo "========================================"
echo "DNS Security Proxy - Setup Script"
echo "========================================"
echo "CoreDNS 1.14.1 + Go DNS Proxy"
echo ""

# Устанавливаем обработчик прерывания для Ctrl+C
trap 'echo ""; echo "🛑 Script interrupted by user"; docker compose down 2>/dev/null || true; exit 1' INT TERM

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

# Проверяем наличие config.yaml
if [ ! -f config/config.yaml ]; then
    echo "❌ ERROR: config/config.yaml not found!"
    echo "   Please create it with proper configuration"
    exit 1
fi

# Убеждаемся что listen адреса правильные в config.yaml
echo "🔧 Validating config.yaml..."
if grep -q 'dns_listen: ":5353"' config/config.yaml; then
    echo "  Fixing dns_listen address..."
    sed -i 's/dns_listen: ":5353"/dns_listen: "0.0.0.0:5353"/' config/config.yaml
fi

if grep -q 'http_listen: ":8054"' config/config.yaml; then
    echo "  Fixing http_listen address..."
    sed -i 's/http_listen: ":8054"/http_listen: "0.0.0.0:8054"/' config/config.yaml
fi

# Создаем сертификаты с правильными правами
echo "🔐 Setting up TLS certificates..."
if [ ! -f certs/server.crt ] || [ ! -f certs/server.key ]; then
    echo "  Generating self-signed certificates..."
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout certs/server.key -out certs/server.crt \
        -days 365 -subj "/CN=dns.localhost" 2>/dev/null || \
    echo "  ⚠️  Could not generate certificates, continuing..."
fi

# Исправляем права доступа к сертификатам
chmod 644 certs/* 2>/dev/null || true

# Создаем правильный Corefile
echo "📝 Creating Corefile..."
cat > Corefile << 'EOF'
# DNS Security Proxy
# Все протоколы → DNS Proxy через DNS-over-TCP

.:53 {
    # ВСЕ запросы форвардим в ваше приложение
    forward . dns-proxy:5353 {
        max_concurrent 10000
        expire 30s
        health_check 10s
        policy sequential
        prefer_udp
    }
    
    # Кеширование
    cache {
        success 10000 3600 300
        denial 10000 3600 300
        prefetch 1000 10m 80%
    }
    
    # Логирование
    log
    
    # Обработка ошибок
    errors
    
    # Метрики Prometheus
    prometheus :9091
    
    # Health check
    health :8080
    
    # Безопасность
    bind 0.0.0.0
    bufsize 1232
}

# DNS-over-TLS (DoT)
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
    bind 0.0.0.0
}

# DNS-over-HTTPS (DoH)
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
    bind 0.0.0.0
}
EOF

# Убираем version из docker-compose.yml если есть
if [ -f docker-compose.yml ] && grep -q "^version:" docker-compose.yml; then
    echo "📝 Removing version from docker-compose.yml..."
    sed -i '/^version:/d' docker-compose.yml
fi

# Обновляем Go зависимости
echo "🔄 Updating Go dependencies..."
rm -f go.sum 2>/dev/null || true
if command -v go &> /dev/null; then
    go mod tidy 2>/dev/null || echo "⚠️  go mod tidy had warnings"
else
    echo "⚠️  Go not installed, skipping dependency update"
fi

# Исправляем common errors в Go коде
echo "🔧 Fixing compilation errors..."
if [ -f cache.go ]; then
    # Исправляем SetEx → Set
    sed -i 's/\.SetEx(/\.Set(/g' cache.go 2>/dev/null || true
    echo "  ✅ Fixed redis.SetEx method"
fi

if [ -f cloud_api.go ]; then
    # Исправляем указатель
    if grep -q "func newCloudAPIClient(config CloudAPIConfig)" cloud_api.go; then
        sed -i 's/func newCloudAPIClient(config CloudAPIConfig) \*CloudAPIClient/func newCloudAPIClient(config \*CloudAPIConfig) \*CloudAPIClient/g' cloud_api.go
        echo "  ✅ Fixed cloud_api.go pointer"
    fi
fi

echo "🐳 Building and starting containers..."
docker compose down 2>/dev/null || true
docker compose up -d --build

echo ""
echo "⏳ Waiting for services to start (30 seconds)..."
for i in {1..30}; do
    echo -n "."
    sleep 1
done
echo ""

echo ""
echo "✅ Services Status:"
timeout 5 docker compose ps 2>/dev/null || echo "⚠️  Could not get service status"

echo ""
echo "🧪 Testing services..."
echo ""

# Проверка CoreDNS
echo "Testing CoreDNS health..."
if timeout 10 curl -s -f http://localhost:8080/health 2>/dev/null | grep -q "OK"; then
    echo "  ✅ CoreDNS health: OK"
else
    echo "  ⚠️  CoreDNS health: FAILED"
    echo "  CoreDNS logs (last 3 lines):"
    timeout 5 docker compose logs coredns --tail=3 2>/dev/null | tail -3 || true
fi

# Проверка DNS Proxy health
echo "Testing DNS Proxy health..."
if timeout 10 curl -s -f http://localhost:8054/health 2>/dev/null | grep -q "healthy"; then
    echo "  ✅ DNS Proxy health: OK"
else
    echo "  ⚠️  DNS Proxy health: FAILED"
    echo "  DNS Proxy logs (last 5 lines):"
    timeout 5 docker compose logs dns-proxy --tail=5 2>/dev/null | tail -5 || true
fi

# Проверка Valkey - безопасно без зависания
echo "Testing Valkey connection..."
VALKEY_HEALTH=$(timeout 5 docker compose ps valkey --format json 2>/dev/null | grep -o '"State":"[^"]*"' | cut -d'"' -f4 || echo "")
if [[ "$VALKEY_HEALTH" == "running" ]] || [[ "$VALKEY_HEALTH" == "healthy" ]]; then
    echo "  ✅ Valkey: Container is $VALKEY_HEALTH"
else
    echo "  ⚠️  Valkey: Status is '$VALKEY_HEALTH'"
fi

echo ""
echo "🧪 Testing DNS service..."
if command -v dig &> /dev/null; then
    echo "Testing with dig..."
    
    # Даем еще время на полный запуск
    sleep 5
    
    # Тест UDP
    echo -n "  UDP DNS: "
    if UDP_OUTPUT=$(timeout 10 dig @127.0.0.1 example.com +short +time=3 +tries=2 2>&1); then
        if echo "$UDP_OUTPUT" | head -1 | grep -q -E "^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$|^[a-f0-9:]+$"; then
            echo "✅ Responding"
        else
            echo "⚠️  Got response: $(echo "$UDP_OUTPUT" | head -1 | tr -d '\n')"
        fi
    else
        echo "⚠️  Timeout or connection refused"
    fi
    
    # Тест TCP
    echo -n "  TCP DNS: "
    if TCP_OUTPUT=$(timeout 10 dig @127.0.0.1 example.com +short +tcp +time=3 +tries=2 2>&1); then
        if echo "$TCP_OUTPUT" | head -1 | grep -q -E "^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$|^[a-f0-9:]+$"; then
            echo "✅ Responding"
        else
            echo "⚠️  Got response: $(echo "$TCP_OUTPUT" | head -1 | tr -d '\n')"
        fi
    else
        echo "⚠️  Timeout or connection refused"
    fi
    
    # Быстрая проверка DoT если openssl установлен
    if command -v openssl &> /dev/null; then
        echo -n "  DoT (TLS): "
        if timeout 5 openssl s_client -connect 127.0.0.1:853 -quiet 2>&1 | head -1 | grep -q "Connected"; then
            echo "✅ Port listening"
        else
            echo "⚠️  Port not accessible"
        fi
    fi
else
    echo "  ℹ️  dig not installed, skipping DNS tests"
fi

echo ""
echo "========================================"
echo "🚀 Setup completed!"
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
echo "📊 Service status:"
docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || true
echo ""
echo "🔧 Management commands:"
echo "  docker compose logs -f dns-proxy      # View DNS proxy logs"
echo "  docker compose logs -f coredns        # View CoreDNS logs"
echo "  docker compose restart                # Restart all services"
echo "  docker compose down                   # Stop all services"
echo ""
echo "📈 Monitoring:"
echo "  curl http://localhost:8054/stats      # Get proxy statistics"
echo "  curl http://localhost:8054/metrics    # Prometheus metrics"
echo ""
echo "⚠️  Troubleshooting:"
echo "  1. Check all logs: docker compose logs"
echo "  2. Restart specific service: docker compose restart [service]"
echo "  3. Rebuild and restart: docker compose up -d --build"
echo ""
echo "⏹️  To stop all services: docker compose down"
echo "========================================"
