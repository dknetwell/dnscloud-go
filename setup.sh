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
    export $(grep -v '^#'
