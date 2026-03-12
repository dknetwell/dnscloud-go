# DNS Security Proxy

Production-ready DNS inspection proxy written in Go. Intercepts DNS queries, enriches domain lookups via pluggable sources (CloudAPI и др.), кэширует результаты на двух уровнях и возвращает sinkhole-ответ для заблокированных доменов. TLS/DoT/DoH обеспечивается через CoreDNS как фронтенд.

```
Client
  │
  ▼
CoreDNS  ←── DoT (853) / DoH (443) / plain DNS (53)
  │            offload TLS, normalize all query types
  ▼
dns-proxy (Go)   ←── L1 cache (Ristretto in-memory)
  │                   L2 cache (Valkey/Redis, персистентный)
  │                   Enrichment workers pool
  │                   → CloudAPI enricher (PAN-OS XML/JSON API)
  │                   → (your enricher N)
  ▼
Upstream DNS (8.8.8.8 / 1.1.1.1)
```

---

## Быстрый старт

```bash
git clone -b clau https://github.com/dknetwell/dnscloud-go.git
cd dnscloud-go

# Обязательно: скопируй и заполни .env
cp .env.example .env
vim .env   # CLOUDAPI_ENDPOINT=... CLOUDAPI_APIKEY=...

chmod +x setup.sh check.sh dns-monitor.sh
./setup.sh
```

После старта:

| Эндпоинт | Описание |
|---|---|
| `:53 udp/tcp` | Plain DNS |
| `:853 tcp` | DNS-over-TLS (DoT) |
| `:443 tcp` | DNS-over-HTTPS (DoH) — `/dns-query` |
| `http://localhost:8080/health` | Healthcheck (только localhost) |
| `http://localhost:8080/stats` | JSON-статистика движка (только localhost) |
| `http://localhost:8080/metrics` | Prometheus метрики (только localhost) |

> **Важно:** порт `8080` публикуется только на `127.0.0.1` — наружу не торчит. Это намеренно.

---

## Проверка после запуска

```bash
./check.sh
```

Скрипт проверяет все протоколы (UDP/TCP/DoT/DoH), кэш L1/L2, Stats API, Prometheus метрики, логи контейнеров. Все проверки должны быть ✔.

```bash
# Ручная проверка DNS
dig @127.0.0.1 google.com A
dig @127.0.0.1 fraud.ru A     # должен вернуть 0.0.0.0 (blocked, category=1)
```

---

## Мониторинг в реальном времени

```bash
./dns-monitor.sh                    # localhost, обновление каждые 5с
./dns-monitor.sh 127.0.0.1 3        # обновление каждые 3с
./dns-monitor.sh 192.168.1.10 5     # удалённый хост
```

Показывает: RPS (запросов в секунду с дельтой), процент блокировок, cache hit rate по слоям, статус CloudAPI enricher (ok/error), размер очереди обогащения, топ запрашиваемых доменов, заблокированные домены с категорией, состояние контейнеров.

---

## Публикация сервиса

### Порты наружу

| Порт | Протокол | Публиковать | Причина |
|---|---|---|---|
| 53 | UDP + TCP | ✅ да | Plain DNS |
| 853 | TCP | ✅ да | DNS-over-TLS |
| 443 | TCP | ✅ да | DNS-over-HTTPS |
| **8080** | TCP | ❌ **нет** | stats/metrics — только localhost |

### Настройка firewalld (Rocky Linux / RHEL)

Rate limiting — защита от amplification-атак и злоупотребления как open resolver:

```bash
# DNS UDP :53
firewall-cmd --permanent --add-rich-rule='
  rule family="ipv4"
  port port="53" protocol="udp"
  limit value="20/s"
  accept'

# DNS TCP :53
firewall-cmd --permanent --add-rich-rule='
  rule family="ipv4"
  port port="53" protocol="tcp"
  limit value="20/s"
  accept'

# DoT :853
firewall-cmd --permanent --add-rich-rule='
  rule family="ipv4"
  port port="853" protocol="tcp"
  limit value="10/s"
  accept'

# DoH :443
firewall-cmd --permanent --add-rich-rule='
  rule family="ipv4"
  port port="443" protocol="tcp"
  limit value="10/s"
  accept'

# Применить
firewall-cmd --reload
```

Проверить применённые правила:

```bash
firewall-cmd --list-rich-rules
# Ожидаемый вывод:
# rule family="ipv4" port port="443" protocol="tcp" accept limit value="10/s"
# rule family="ipv4" port port="53"  protocol="tcp" accept limit value="20/s"
# rule family="ipv4" port port="53"  protocol="udp" accept limit value="20/s"
# rule family="ipv4" port port="853" protocol="tcp" accept limit value="10/s"

firewall-cmd --list-services
# Должно быть пусто — сервисы dns/https без rate limit нам не нужны.
# Если есть — убрать:
firewall-cmd --permanent --remove-service=dns
firewall-cmd --permanent --remove-service=https
firewall-cmd --reload
```

Проверить что `8080` не торчит наружу:

```bash
netstat -tunelp | grep 8080
# Должно быть: 127.0.0.1:8080 — НЕ 0.0.0.0:8080
```

### Ограничение рекурсии по IP (опционально, рекомендуется)

Если сервис предназначен только для определённых подсетей — добавить ACL в `Corefile`:

```
.:53 {
    acl {
        allow net 10.0.0.0/8
        allow net 192.168.0.0/16
        block
    }
    forward . 172.28.0.20:53 {
        prefer_udp
    }
    errors
    log
}
```

### Проверка снаружи (после публикации)

```bash
# С внешней машины:
dig @<PUBLIC_IP> google.com A          # должен работать
dig @<PUBLIC_IP> -p 853 google.com +tls   # DoT должен работать
curl http://<PUBLIC_IP>:8080/health    # должен зависнуть — порт закрыт
```

---

## Конфигурация

Приоритет загрузки: **переменные окружения (.env) → config/config.yaml → встроенные дефолты**.

### Переменные окружения (`.env`)

#### DNS

| Переменная | Дефолт | Описание |
|---|---|---|
| `DNS_LISTEN_UDP` | `:53` | Адрес UDP-сокета |
| `DNS_LISTEN_TCP` | `:53` | Адрес TCP-сокета |
| `DNS_UPSTREAMS` | `8.8.8.8:53,1.1.1.1:53` | Upstream серверы через запятую |
| `DNS_SINKHOLE_IPV4` | `0.0.0.0` | IP для заблокированных A-запросов |
| `DNS_SINKHOLE_IPV6` | `::` | IP для заблокированных AAAA-запросов |

#### CloudAPI / Enricher

| Переменная | Дефолт | Описание |
|---|---|---|
| `CLOUDAPI_ENDPOINT` | — | URL API (`https://host/api/`) |
| `CLOUDAPI_APIKEY` | — | Ключ авторизации (X-PAN-KEY) |
| `CLOUDAPI_TIMEOUT` | `5` | Таймаут HTTP-запроса (секунды) |
| `CLOUDAPI_RPS` | `50` | Rate limit — запросов в секунду к API |
| `CLOUDAPI_BURST` | `100` | Burst лимит |
| `CLOUDAPI_INSECURE` | `false` | Отключить TLS-верификацию (dev only) |

#### Engine / Worker Pool

| Переменная | Дефолт | Описание |
|---|---|---|
| `ENGINE_WORKERS` | `100` | Количество горутин-воркеров обогащения |
| `ENGINE_QUEUE` | `1000` | Размер очереди задач обогащения |

#### Прочее

| Переменная | Дефолт | Описание |
|---|---|---|
| `VALKEY_ADDR` | `valkey:6379` | Адрес Valkey/Redis |
| `HTTP_LISTEN` | `:8080` | Адрес HTTP-сервера |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

> **SLA:** DNS-ответ клиент получает немедленно — обогащение асинхронно. Первый запрос к домену всегда быстрый (`allow` по умолчанию), второй — уже с результатом CloudAPI.

---

## Логирование

Все логи в **stdout** в формате **JSON** (logfmt-совместимо с Promtail).

```bash
# Живые логи
docker compose logs -f dns-proxy | jq .

# Только заблокированные домены
docker compose logs -f dns-proxy | jq 'select(.blocked == true)'

# Только ошибки enricher
docker compose logs -f dns-proxy | jq 'select(.msg == "enrich_error")'

# Медленные запросы к CloudAPI (>1000ms)
docker compose logs -f dns-proxy | jq 'select(.msg == "enrich_ok" and .latency_ms > 1000)'
```

---

## Метрики Prometheus

Эндпоинт: `http://localhost:8080/metrics`

| Метрика | Labels | Описание |
|---|---|---|
| `dns_requests_total` | — | Всего DNS-запросов |
| `dns_requests_blocked_total` | — | Заблокировано |
| `dns_cache_hits_total` | `layer={l1,l2}` | Попадания в кэш |
| `dns_enricher_calls_total` | `enricher`, `status={ok,error}` | Вызовы enricher |
| `dns_request_duration_ms` | `blocked={true,false}` | Latency запроса |
| `dns_enricher_duration_ms` | `enricher` | Latency enricher |
| `dns_enricher_queue_size` | — | Размер очереди обогащения |

```promql
# RPS
rate(dns_requests_total[1m])

# Процент блокировок
rate(dns_requests_blocked_total[1m]) / rate(dns_requests_total[1m]) * 100

# Cache hit rate L1
rate(dns_cache_hits_total{layer="l1"}[1m]) / rate(dns_requests_total[1m]) * 100

# p95 latency
histogram_quantile(0.95, rate(dns_request_duration_ms_bucket[5m]))

# Ошибки CloudAPI
rate(dns_enricher_calls_total{enricher="cloud_api",status="error"}[5m])
```

---

## Добавление нового источника (Enricher)

Реализовать интерфейс:

```go
type Enricher interface {
    Name() string
    Enrich(ctx context.Context, domain string, result *DomainResult) error
}
```

Зарегистрировать в `main.go`:

```go
enrichers := []Enricher{
    NewCloudAPIEnricher(cfg),
    NewMyEnricher(cfg),    // добавить сюда
}
```

Метрики `dns_enricher_calls_total{enricher="my_enricher"}` подхватятся автоматически.

---

## Структура проекта

```
.
├── main.go                  # Точка входа
├── config.go                # Config struct + LoadConfig
├── env.go                   # Helpers: getEnv, getEnvInt, ...
├── models.go                # DomainResult, Stats
├── logger.go                # JSON-логгер
├── metrics.go               # Prometheus метрики
├── dns_handler.go           # DNS сервер, sinkhole, upstream
├── engine.go                # CheckEngine, worker pool, singleflight
├── cache.go                 # L1 кэш (Ristretto)
├── valkey.go                # L2 кэш (Valkey/Redis, персистентный)
├── enricher.go              # Интерфейс Enricher
├── cloud_api_enricher.go    # Реализация CloudAPI (PAN-OS)
├── http_server.go           # /health /stats /metrics
├── Dockerfile               # Multi-stage build
├── docker-compose.yml       # dns-proxy + valkey + coredns
├── Corefile                 # CoreDNS: plain + DoT + DoH → dns-proxy
├── setup.sh                 # Генерация сертов + docker compose up
├── check.sh                 # Проверочный скрипт (все протоколы + кэш + метрики)
├── dns-monitor.sh           # Живой мониторинг в терминале
└── config/config.yaml       # Основной конфиг (категории, TTL, воркеры)
```

---

## Разработка

```bash
# Локальный запуск без Docker
go run .

# Тест DNS
dig @127.0.0.1 google.com A
dig @127.0.0.1 fraud.ru A        # заблокированный домен

# DoT
dig @127.0.0.1 -p 853 google.com A +tls

# Логи
docker compose logs -f dns-proxy | jq .

# Ручной просмотр L2 кэша
docker exec -it valkey valkey-cli keys "*"
docker exec -it valkey valkey-cli get "google.com"

# Проверить персистентность Valkey
docker exec valkey ls /data
# appendonlydir  dump.rdb  ← оба файла должны быть
```

---

## Требования

- Docker + Docker Compose plugin
- Rocky Linux 10 / любой Linux с Docker
- Порты `53`, `853`, `443` свободны
- `8080` открыт только локально (`127.0.0.1`)
