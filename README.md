markdown
# DNS Security Proxy

Проект DNS прокси с проверкой безопасности на базе CoreDNS и Go.

## Архитектура
```
┌─────────────────────────────────────────────────────┐
│ CoreDNS 1.14.1 │
│ (DoH/DoT/DNS фронтенд, кеширование, метрики) │
│ │
│ DoH (:443) ───┐ │
│ DoT (:853) ───┼──► DNS-over-TCP ──► DNS Proxy │
│ DNS (:53) ───┘ (:5353) │
└──────────────────────────────────┬─────────────────┘
│
┌───────────────────────────────────▼─────────────────┐
│ DNS Proxy (Go приложение) │
│ │
│ Логика проверок: │
│ 1. Кеш (memory + Valkey) │
│ 2. Cloud API проверка │
│ 3. SLA контроль (95ms) │
│ │
│ Метрики: :8054/metrics │
│ Health: :8054/health │
└──────────────────────────────────┬─────────────────┘
│
┌───────────────────────────────────▼─────────────────┐
│ Valkey (распределенный кеш) │
└─────────────────────────────────────────────────────┘
```

## Быстрый старт

### Требования
- Docker 20.10+
- Docker Compose 2.0+
- 2GB RAM
- 5GB свободного места

### Установка

```
# 1. Клонируйте репозиторий
git clone https://github.com/yourusername/dnscloud-go.git
cd dnscloud-go

# 2. Настройте окружение
cp .env.example .env
# Отредактируйте .env, установите CLOUD_API_KEY

# 3. Запустите
chmod +x setup.sh
./setup.sh
```
Проверка
```
# Проверка DNS
dig @127.0.0.1 google.com
dig @127.0.0.1 example.com +tcp

# Проверка health
curl http://localhost:8080/health
curl http://localhost:8054/health
```
# Метрики
```
curl http://localhost:9091/metrics
curl http://localhost:8054/metrics
Конфигурация
```
Основные настройки (.env)
```
CLOUD_API_KEY=ваш_ключ
VALKEY_PASSWORD=SecurePass123!
LOG_LEVEL=info
RATE_LIMIT_RPS=5
Детальные настройки (config/config.yaml)
Таймауты и SLA - контроль времени ответа

Кеширование - настройки memory и Valkey кеша

Sinkhole IP - адреса для блокировки

TTL настройки - время жизни записей по категориям

Endpoints
CoreDNS: :8080/health, :9091/metrics

DNS Proxy: :8054/health, :8054/metrics

Valkey: :6379 (redis-cli)
```
Логи
```
# Логи DNS Proxy
docker compose logs -f dns-proxy

# Логи CoreDNS
docker compose logs -f coredns

# Логи Valkey
docker compose logs -f valkey
Производительность
SLA: 100ms (95ms приложение + 5ms CoreDNS)
```
Throughput: до 10k RPS

Поддержка: DNS, DoH, DoT

Кеширование: memory + Valkey

Управление
```
# Статус
docker compose ps

# Запуск
docker compose up -d

# Остановка
docker compose down

# Перезапуск
docker compose restart

# Обновление
docker compose pull
docker compose up -d --build
```
Мониторинг
```
Prometheus метрики: http://localhost:8054/metrics

Статистика: http://localhost:8054/stats

Health checks: автоматические проверки
```
Расширение
Добавление новой проверки
Добавьте новый клиент в cloud_api.go

Обновите логику в engine.go

Добавьте конфигурацию в config.yaml

Изменение SLA
```
yaml
# config/config.yaml
timeouts:
  total: 95ms  # Общий таймаут SLA
  cloud_api: 50ms
```

## Процесс развертывания:

### 1. Подготовка сервера (Rocky Linux 10):
```
# Установка Docker
sudo dnf update -y
sudo dnf config-manager --add-repo=https://download.docker.com/linux/rhel/docker-ce.repo
sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable --now docker
sudo usermod -aG docker $USER
newgrp docker

# Установка утилит
sudo dnf install -y git curl wget bind-utils jq openssl
2. Клонирование репозитория:

# С токеном GitHub (для приватного репо)
git clone https://TOKEN@github.com/yourusername/dnscloud-go.git
cd dnscloud-go
3. Настройка и запуск:

# Даем права на выполнение
chmod +x setup.sh

# Запускаем скрипт (он все сделает сам)
./setup.sh

# Скрипт выполнит:
# 1. Создание необходимых директорий
# 2. Генерацию конфигурационных файлов
# 3. Генерацию TLS сертификатов
# 4. Сборку Docker образов
# 5. Запуск всех сервисов
# 6. Проверку работоспособности
4. Проверка работы:

# Проверка статуса
docker compose ps

# Тестирование DNS
dig @127.0.0.1 google.com
dig @127.0.0.1 example.com +tcp

# Проверка метрик
curl http://localhost:8054/metrics
curl http://localhost:9091/metrics

# Просмотр логов
docker compose logs -f dns-proxy
5. Настройка firewall (если нужно):
bash
# Разрешаем порты
sudo firewall-cmd --permanent --add-port=53/tcp
sudo firewall-cmd --permanent --add-port=53/udp
sudo firewall-cmd --permanent --add-port=853/tcp
sudo firewall-cmd --permanent --add-port=443/tcp
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --permanent --add-port=9091/tcp
sudo firewall-cmd --permanent --add-port=8054/tcp
sudo firewall-cmd --reload
```
