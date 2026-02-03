#!/bin/bash
set -e

echo "========================================"
echo "DNS Security Proxy Setup"
echo "========================================"

# Проверка Docker
if ! command -v docker &> /dev/null; then
    echo "ERROR: Docker is not installed"
    echo "Please install Docker first:"
    echo "  sudo dnf install -y docker docker-compose-plugin"
    exit 1
fi

# Запуск Docker
if ! systemctl is-active --quiet docker; then
    echo "Starting Docker service..."
    sudo systemctl enable --now docker
    sleep 2
fi

# Добавляем пользователя в группу docker
if ! groups $USER | grep -q docker; then
    echo "Adding $USER to docker group..."
    sudo usermod -aG docker $USER
    echo "Please logout and login again or run: newgrp docker"
fi

# Создаем необходимые директории
echo "Creating directories..."
mkdir -p config data/blocklists logs

# Создаем .env если его нет
if [ ! -f .env ]; then
    echo "Creating .env file from template..."
    cp .env.example .env
    echo ""
    echo "⚠️  Please edit .env file and set your CLOUD_API_KEY"
    echo "   Then run this script again."
    echo ""
    exit 0
fi

# Загружаем переменные
source .env

# Проверка обязательных переменных
if [ -z "$CLOUD_API_KEY" ] || [ "$CLOUD_API_KEY" = "your_api_key_here" ]; then
    echo "ERROR: CLOUD_API_KEY is not set in .env file"
    echo "Please edit .env file and set your API key"
    exit 1
fi

# Создаем тестовые блок-листы если их нет
if [ ! -f data/blocklists/malware.txt ]; then
    echo "Creating example blocklists..."
    cat > data/blocklists/malware.txt << 'MALWARE'
# Example malware domains
example-malware.com
test-bad-site.net
evil-domain.org
MALWARE

    cat > data/blocklists/phishing.txt << 'PHISHING'
# Example phishing domains
phishing-example.com
fake-bank-site.net
PHISHING
fi

echo "Starting services with Docker Compose..."
docker compose up -d --build

echo ""
echo "Waiting for services to start..."
sleep 10

# Проверка
echo ""
echo "========================================"
echo "Services Status:"
echo "========================================"
docker compose ps

echo ""
echo "Testing DNS service..."
if command -v dig &> /dev/null; then
    if timeout 2 dig @127.0.0.1 google.com +short > /dev/null 2>&1; then
        echo "✅ DNS is working"
    else
        echo "⚠️  DNS test failed (might need more time to start)"
    fi
else
    echo "ℹ️  Install 'dig' to test DNS: sudo dnf install bind-utils"
fi

echo ""
echo "========================================"
echo "Setup Complete!"
echo "========================================"
echo ""
echo "Services:"
echo "  DNS Proxy:      udp://127.0.0.1:53"
echo "  Health check:   http://localhost:8080/health"
echo "  Metrics:        http://localhost:9091/metrics"
echo "  Valkey:         localhost:6379"
echo ""
echo "Test commands:"
echo "  dig @127.0.0.1 google.com"
echo "  curl http://localhost:8080/health"
echo ""
echo "View logs:"
echo "  docker compose logs -f"
echo ""
echo "Stop services:"
echo "  docker compose down"
echo ""
echo "========================================"
