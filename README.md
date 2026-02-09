markdown
# DNS Security Proxy

Проект DNS прокси с проверкой безопасности на базе CoreDNS и Go.

## Архитектура
┌─────────────────────────────────────────────────────┐
│ CoreDNS 1.14.1 │
│ (DoH/DoT/DNS фронтенд, кеширование, метрики) │
│ │
│ DoH (:443) ───┐ │
│ DoT (:853) ───┼──► DNS-over-TCP ──► DNS Proxy │
│ DNS (:53) ───┘ (:5353) │
└──────────────────────────────────┬──────────────────┘
│
┌───────────────────────────────────▼──────────────────┐
│ DNS Proxy (Go приложение) │
│ │
│ Логика проверок: │
│ 1. Кеш (memory + Valkey) │
│ 2. Cloud API проверка │
│ 3. SLA контроль (95ms) │
│ │
│ Метрики: :8054/metrics │
│ Health: :8054/health │
└──────────────────────────────────┬──────────────────┘
│
┌───────────────────────────────────▼──────────────────┐
│ Valkey (распределенный кеш) │
└─────────────────────────────────────────────────────┘

text

## Быстрый старт

### Требования
- Docker 20.10+
- Docker Compose 2.0+
- 2GB RAM
- 5GB свободного места

### Установка

```bash
# 1. Клонируйте репозиторий
git clone https://github.com/dknetwell/dnscloud-go.git
cd dnscloud-go

# 2. Настройте окружение
cp .env.example .env
# Отредактируйте .env, установите CLOUD_API_KEY

# 3. Запустите
chmod +x setup.sh
./setup.sh
```

###Проверка

# Проверка DNS
dig @127.0.0.1 google.com

# Проверка health
curl http://localhost:8080/health
curl http://localhost:8054/health

# Метрики
curl http://localhost:9091/metrics
curl http://localhost:8054/metrics


Конфигурация
Основные настройки (.env)

CLOUD_API_KEY=ваш_ключ
VALKEY_PASSWORD=SecurePass123!
LOG_LEVEL=info
RATE_LIMIT_RPS=5
Детальные настройки (config/config.yaml)
Таймауты и SLA

Кеширование

Sinkhole IP

TTL настройки

Логирование

Мониторинг
Endpoints
CoreDNS: :8080/health, :9091/metrics

DNS Proxy: :8054/health, :8054/metrics

Valkey: :6379 (redis-cli)

Логи

# Логи DNS Proxy
docker compose logs -f dns-proxy

# Логи CoreDNS
docker compose logs -f coredns

# Логи Valkey
docker compose logs -f dns-valkey
