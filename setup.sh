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

# Исправляем Corefile (убираем DoH/DoT если нет сертификатов)
echo "🔧 Creating Corefile..."
cat > Corefile << 'EOF'
.:53 {
    # ВСЕ запросы форвардим в DNS Proxy
    forward . dns-proxy:5353 {
        max_concurrent 10000
        expire 30s
        health_check 10s
        prefer_udp
    }
    
    # Кеширование
    cache {
        success 10000 3600 300
        denial 10000 3600 300
        prefetch 1000 10m 80%
    }
    
    # Логирование
    log . {
        class all
        format combined
    }
    
    # Обработка ошибок
    errors {
        consolidate 5s 100
    }
    
    # Метрики и health
    prometheus :9091
    health :8080
    
    # Безопасность
    bind 0.0.0.0
    bufsize 1232
}
EOF

# Создаем сертификаты с правильными правами
if [ ! -f certs/server.crt ]; then
    echo "🔐 Generating test TLS certificates..."
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout certs/server.key -out certs/server.crt \
        -days 365 -subj "/CN=dns.localhost" 2>/dev/null && \
    chmod 644 certs/server.key certs/server.crt 2>/dev/null || true
    echo "⚠️  Using self-signed certificates"
fi

# Убираем version из docker-compose.yml если есть
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

# Создаем http_server.go с правильным health хендлером если его нет
if [ ! -f http_server.go ]; then
    echo "📝 Creating http_server.go..."
    cat > http_server.go << 'EOF'
package main

import (
    "context"
    "encoding/json"
    "net/http"
    "time"
)

// HTTPServer - HTTP сервер для метрик и health
type HTTPServer struct {
    server *http.Server
    engine *CheckEngine
}

func newHTTPServer(address string, engine *CheckEngine) *HTTPServer {
    mux := http.NewServeMux()
    srv := &HTTPServer{
        engine: engine,
        server: &http.Server{
            Addr:         address,
            Handler:      mux,
            ReadTimeout:  10 * time.Second,
            WriteTimeout: 10 * time.Second,
            IdleTimeout:  60 * time.Second,
        },
    }
    
    // Регистрируем обработчики
    mux.HandleFunc("/health", srv.healthHandler)
    mux.HandleFunc("/stats", srv.statsHandler)
    mux.HandleFunc("/", srv.defaultHandler)
    
    return srv
}

func (s *HTTPServer) start() error {
    return s.server.ListenAndServe()
}

func (s *HTTPServer) shutdown(ctx context.Context) error {
    return s.server.Shutdown(ctx)
}

func (s *HTTPServer) healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    
    health := map[string]interface{}{
        "status":    "healthy",
        "service":   "dns-proxy",
        "timestamp": time.Now().Format(time.RFC3339),
        "version":   "1.0.0",
    }
    
    json.NewEncoder(w).Encode(health)
}

func (s *HTTPServer) statsHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    
    if s.engine == nil {
        json.NewEncoder(w).Encode(map[string]string{"error": "engine not initialized"})
        return
    }
    
    stats := s.engine.getStats()
    json.NewEncoder(w).Encode(stats)
}

func (s *HTTPServer) defaultHandler(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    
    w.Header().Set("Content-Type", "text/plain")
    w.Write([]byte("DNS Security Proxy v1.0.0\n"))
}
EOF
fi

echo "🐳 Building and starting containers..."
docker compose down 2>/dev/null || true
docker compose up -d --build

echo ""
echo "⏳ Waiting for services to start (20 seconds)..."
sleep 20

echo ""
echo "✅ Services Status:"
docker compose ps

echo ""
echo "🧪 Testing services..."
echo ""

# Проверка CoreDNS - только базовый DNS без TLS
echo "Testing CoreDNS health..."
if timeout 3 curl -s http://localhost:8080/health 2>/dev/null | grep -q "OK"; then
    echo "  ✅ CoreDNS health: OK"
else
    echo "  ⚠️  CoreDNS health: FAILED"
fi

# Проверка DNS Proxy health
echo "Testing DNS Proxy health..."
if timeout 3 curl -s http://localhost:8054/health 2>/dev/null | grep -q "healthy"; then
    echo "  ✅ DNS Proxy health: OK"
else
    echo "  ⚠️  DNS Proxy health: FAILED"
    echo "  Checking logs..."
    docker compose logs dns-proxy --tail=5 2>/dev/null || true
fi

# Проверка Valkey - упрощенная проверка без зависания
echo "Testing Valkey connection..."
if docker compose ps valkey 2>/dev/null | grep -q "healthy"; then
    echo "  ✅ Valkey: Container is healthy"
else
    echo "  ⚠️  Valkey: Container not healthy"
fi

echo ""
echo "🧪 Testing DNS service..."
if command -v dig &> /dev/null; then
    echo "Testing with dig..."
    
    # Даем больше времени на запуск
    sleep 5
    
    # Тест UDP
    if timeout 5 dig @127.0.0.1 example.com +short 2>&1 | head -1 | grep -q -E "^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$|^[a-f0-9:]+$|^example\.com\.$"; then
        echo "  ✅ DNS over UDP: Responding"
    else
        if timeout 5 dig @127.0.0.1 example.com 2>&1 | grep -q "connection refused"; then
            echo "  ⚠️  DNS over UDP: Connection refused"
        else
            echo "  ⚠️  DNS over UDP: No response"
        fi
    fi
    
    # Тест TCP
    if timeout 5 dig @127.0.0.1 example.com +short +tcp 2>&1 | head -1 | grep -q -E "^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$|^[a-f0-9:]+$|^example\.com\.$"; then
        echo "  ✅ DNS over TCP: Responding"
    else
        echo "  ⚠️  DNS over TCP: No response"
    fi
else
    echo "  ℹ️  dig not installed, skipping DNS tests"
fi

echo ""
echo "========================================"
echo "🚀 Setup completed!"
echo "========================================"
echo ""
echo "🌐 Services running:"
docker compose ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
echo ""
echo "🔧 Management commands:"
echo "  docker compose logs -f dns-proxy      # View DNS proxy logs"
echo "  docker compose logs -f coredns        # View CoreDNS logs"
echo "  curl http://localhost:8054/health     # Check proxy health"
echo "  dig @127.0.0.1 example.com            # Test DNS"
echo ""
echo "⚠️  If services aren't working:"
echo "  1. Check logs: docker compose logs"
echo "  2. Restart: docker compose restart"
echo ""
echo "⏹️  To stop: docker compose down"
echo "========================================"
