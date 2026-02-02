#!/bin/bash
set -e

cd /home/proxy/dnscloud-go

echo "=== Deploying DNS Security Proxy ==="

# 1. Проверяем переменные окружения
if [ -z "$CLOUD_API_KEY" ]; then
    echo "ERROR: CLOUD_API_KEY is not set"
    exit 1
fi

# 2. Останавливаем текущую версию
docker compose down || true

# 3. Собираем новый образ
echo "Building new image..."
docker compose build --no-cache

# 4. Запускаем
echo "Starting services..."
docker compose up -d

# 5. Ждем запуска
echo "Waiting for services to start..."
sleep 10

# 6. Проверяем здоровье
if curl -s http://localhost:8080/health | grep -q "healthy"; then
    echo "✅ Services are healthy"
else
    echo "❌ Health check failed"
    docker compose logs coredns
    exit 1
fi

# 7. Тестируем DNS
echo "Testing DNS..."
if dig @172.16.10.16 google.com +short > /dev/null; then
    echo "✅ DNS is working"
else
    echo "❌ DNS test failed"
    exit 1
fi

echo "=== Deployment completed successfully ==="
