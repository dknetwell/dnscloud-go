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
    exit 1
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
docker compose up -d --build

echo ""
echo "⏳ Waiting for services to start (40 seconds)..."
sleep 40

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

# Проверка CoreDNS
echo "Testing CoreDNS health..."
if timeout 10 curl -s http://localhost:8080/health 2>/dev/null | grep -q "OK"; then
    echo "  ✅ CoreDNS health: OK"
else
    echo "  ⚠️  CoreDNS health: FAILED"
fi

# Проверка Valkey
echo "Testing Valkey connection..."
if docker compose ps valkey 2>/dev/null | grep -q "healthy"; then
    echo "  ✅ Valkey: Healthy"
else
    echo "  ⚠️  Valkey: Not healthy"
fi

echo ""
echo "🧪 Testing DNS..."
if command -v dig &> /dev/null; then
    echo "Testing DNS with dig..."
    
    # Проверяем что DNS Proxy слушает
    if docker compose exec -T dns-proxy netstat -tln 2>/dev/null | grep -q ":5353"; then
        echo "  ✅ DNS Proxy listening on 5353"
    else
        echo "  ❌ DNS Proxy NOT listening on 5353"
    fi
    
    # Тестируем DNS
    echo -n "  DNS query: "
    if result=$(timeout 10 dig @127.0.0.1 example.com +short 2>&1); then
        if echo "$result" | grep -q -E "^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$"; then
            echo "✅ Got response"
        else
            echo "⚠️  Got: $(echo "$result" | head -1)"
        fi
    else
        echo "❌ No response"
    fi
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
echo "🔧 Debug commands:"
echo "  docker compose logs -f"
echo "  docker network inspect dnscloud-go_dns-net"
echo "  docker compose exec coredns nslookup dns-proxy"
echo ""
echo "⏹️  To stop: docker compose down"
echo "========================================"
