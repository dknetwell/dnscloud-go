#!/bin/bash
echo "🔧 Fixing permissions and common issues..."

# Исправляем права на сертификаты
echo "1. Fixing certificate permissions..."
chmod 644 certs/* 2>/dev/null || true

# Исправляем Go зависимости
echo "2. Updating Go dependencies..."
rm -f go.sum 2>/dev/null
go mod tidy 2>/dev/null || true

# Исправляем cache.go
echo "3. Fixing cache.go SetEx error..."
sed -i 's/\.SetEx(/\.Set(/g' cache.go 2>/dev/null || true

# Исправляем cloud_api.go указатель
echo "4. Fixing cloud_api.go pointer..."
sed -i 's/func newCloudAPIClient(config CloudAPIConfig) \*CloudAPIClient/func newCloudAPIClient(config \*CloudAPIConfig) \*CloudAPIClient/g' cloud_api.go 2>/dev/null || true

# Убираем version из docker-compose
echo "5. Removing version from docker-compose.yml..."
sed -i '/^version:/d' docker-compose.yml 2>/dev/null || true

echo "✅ Fixes applied!"
echo "Run: docker compose up -d --build"
