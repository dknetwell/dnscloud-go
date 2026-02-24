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
  │                   L2 cache (Valkey/Redis)
  │                   Enrichment workers pool
  │                   → CloudAPI enricher
  │                   → (your enricher N)
  ▼
Upstream DNS (8.8.8.8 / 1.1.1.1)
```

---

## Быстрый старт

```bash
git clone https://github.com/your-org/dns-security-proxy.git
cd dns-security-proxy

# Обязательно: задай API-ключ
cp .env.example .env
vim .env   # CLOUDAPI_APIKEY=...

bash setup.sh
```

После старта:

| Эндпоинт | Описание |
|---|---|
| `:53 udp/tcp` | Plain DNS |
| `:853 tcp` | DNS-over-TLS (DoT) |
| `:443 tcp` | DNS-over-HTTPS (DoH) — `/dns-query` |
| `http://localhost:8080/health` | Healthcheck |
| `http://localhost:8080/stats` | JSON-статистика движка |
| `http://localhost:8080/metrics` | Prometheus метрики |

---

## Конфигурация

Приоритет загрузки: **переменные окружения → config.yaml → встроенные дефолты**.

### Переменные окружения (`.env`)

#### DNS

| Переменная | Дефолт | Описание |
|---|---|---|
| `DNS_LISTEN_UDP` | `:53` | Адрес UDP-сокета |
| `DNS_LISTEN_TCP` | `:53` | Адрес TCP-сокета |
| `DNS_UPSTREAMS` | `8.8.8.8:53,1.1.1.1:53` | Upstream серверы через запятую |
| `DNS_SINKHOLE_IPV4` | `0.0.0.0` | IP для заблокированных A-запросов |
| `DNS_SINKHOLE_IPV6` | `::` | IP для заблокированных AAAA-запросов |
| `DNS_MAX_PACKET` | `1232` | Максимальный размер UDP-пакета (байт) |

#### CloudAPI / Enricher

| Переменная | Дефолт | Описание |
|---|---|---|
| `CLOUDAPI_ENDPOINT` | — | URL API (`https://api.example.com/check`) |
| `CLOUDAPI_APIKEY` | — | Bearer-токен авторизации |
| `CLOUDAPI_TIMEOUT` | `2` | Таймаут HTTP-запроса (секунды) — **SLA ответа клиенту** |
| `CLOUDAPI_RPS` | `50` | Rate limit — запросов в секунду к API |
| `CLOUDAPI_BURST` | `100` | Burst лимит (накопленный кредит токенов) |
| `CLOUDAPI_INSECURE` | `false` | Отключить TLS-верификацию (dev only) |

> **SLA:** `CLOUDAPI_TIMEOUT` контролирует максимальное время ожидания ответа от enricher. Обогащение происходит асинхронно в worker-pool — клиент DNS получает ответ немедленно (allow по умолчанию), пока результат кэшируется для следующего запроса. Это архитектурный выбор в пользу latency: первый запрос к домену всегда быстрый.

#### Engine / Worker Pool

| Переменная | Дефолт | Описание |
|---|---|---|
| `ENGINE_WORKERS` | `100` | Количество горутин-воркеров обогащения |
| `ENGINE_QUEUE` | `1000` | Размер очереди задач обогащения |

> При переполнении очереди домен получает `negative=true`, TTL=10s и пропускается без обогащения. В логах появится `warn` с полем `"msg":"enrichment queue full"`.

#### TTL

| Переменная | Дефолт | Описание |
|---|---|---|
| `TTL_DEFAULT` | `300` | TTL по умолчанию для новых записей (секунды) |
| `TTL_MIN` | `60` | Минимальный TTL (clamp снизу) |
| `TTL_MAX` | `86400` | Максимальный TTL (clamp сверху) |

#### Кэш

| Переменная | Дефолт | Описание |
|---|---|---|
| `CACHE_MAX_COST` | `1073741824` (1 GB) | Максимальный размер L1-кэша в байтах |

#### Valkey (L2 кэш)

| Переменная | Дефолт | Описание |
|---|---|---|
| `VALKEY_ADDR` | `valkey:6379` | Адрес Valkey/Redis |
| `VALKEY_PASSWORD` | — | Пароль |
| `VALKEY_DB` | `0` | Номер базы данных |

#### Прочее

| Переменная | Дефолт | Описание |
|---|---|---|
| `HTTP_LISTEN` | `:8080` | Адрес HTTP-сервера (stats/metrics/health) |
| `LOG_LEVEL` | `info` | Уровень логирования: `debug`, `info`, `warn`, `error` |
| `CONFIG_PATH` | — | Путь к YAML-конфигу (если не задан — YAML не читается) |

### config.yaml (опционально)

```yaml
logging:
  level: info

dns:
  listen_udp: ":53"
  listen_tcp: ":53"
  upstream:
    - "8.8.8.8:53"
    - "1.1.1.1:53"
  sinkhole_ipv4: "0.0.0.0"
  sinkhole_ipv6: "::"
  max_packet_size: 1232

cloud_api:
  endpoint: "https://api.example.com/check"
  api_key: "CHANGE_ME"
  insecure_skip_verify: false
  rate_limit: 50      # RPS к CloudAPI
  burst: 100          # burst бюджет токенов
  timeout_seconds: 2  # SLA таймаут

ttl:
  default: 300
  min: 60
  max: 86400

cache:
  max_cost: 1073741824

valkey:
  address: "valkey:6379"
  password: ""
  db: 0

engine:
  worker_count: 100
  worker_queue_size: 1000

http:
  listen: ":8080"
```

---

## Логирование

Все логи пишутся в **stdout** в формате **JSON**, по одной записи на строку (logfmt-совместимо с Promtail).

### Структура лог-записи

```json
{
  "ts":         "2026-02-24T10:01:02.345678Z",
  "level":      "info",
  "component":  "dns",
  "msg":        "dns_request",

  "domain":     "evil-malware.com",
  "client_ip":  "192.168.1.42",
  "qtype":      "A",
  "latency_ms": 0.38,
  "blocked":    true,
  "category":   14,
  "action":     "block",
  "source":     "cloud_api",
  "ttl":        60
}
```

### Поля

| Поле | Тип | Описание |
|---|---|---|
| `ts` | string (RFC3339Nano) | Timestamp в UTC |
| `level` | string | `debug` / `info` / `warn` / `error` / `fatal` |
| `component` | string | `dns`, `engine`, `cloud_api`, `http`, `system` |
| `msg` | string | Тип события |
| `domain` | string | Запрошенный домен |
| `client_ip` | string | IP-адрес клиента DNS |
| `qtype` | string | Тип DNS-запроса (`A`, `AAAA`, `MX`, …) |
| `latency_ms` | float | Время обработки запроса в мс |
| `blocked` | bool | Заблокирован ли домен |
| `category` | int | Категория угрозы (из enricher, 0 = чисто) |
| `action` | string | `allow` / `block` |
| `source` | string | Источник решения (`cloud_api`, `engine`, …) |
| `ttl` | int | TTL записи в секундах |
| `error` | string | Текст ошибки (только при `level=error/warn`) |
| `fields` | object | Произвольные доп. поля (используется в `LogInfoFields`) |

### Типы событий (`msg`)

| msg | level | Описание |
|---|---|---|
| `dns_request` | info | Каждый DNS-запрос с полным контекстом |
| `enrich` | debug | Вызов каждого enricher (включается при `LOG_LEVEL=debug`) |
| `enrichment queue full` | warn | Очередь воркеров переполнена |
| `upstream failed` | error | Все upstream DNS недоступны |
| `DNS server started` | info | Сервер поднялся |
| `HTTP server started` | info | HTTP сервер поднялся |
| `shutdown signal received` | info | Получен SIGTERM/SIGINT |

---

## Сбор логов: Promtail + Loki + Grafana

### 1. Promtail — сбор из Docker

```yaml
# promtail-config.yaml
scrape_configs:
  - job_name: dns-proxy
    docker_sd_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
    relabel_configs:
      - source_labels: [__meta_docker_container_name]
        regex: dns-proxy
        action: keep
      - source_labels: [__meta_docker_container_name]
        target_label: container
    pipeline_stages:
      - json:
          expressions:
            level:      level
            component:  component
            msg:        msg
            domain:     domain
            client_ip:  client_ip
            blocked:    blocked
            category:   category
            latency_ms: latency_ms
            source:     source
      - labels:
          level:
          component:
          msg:
          blocked:
          source:
      - timestamp:
          source: ts
          format: RFC3339Nano
```

### 2. Полезные запросы в Loki (LogQL)

```logql
# Все заблокированные запросы
{container="dns-proxy"} | json | blocked = "true"

# Топ заблокированных доменов за час
{container="dns-proxy"} | json | blocked = "true"
  | line_format "{{.domain}}"
  | count_over_time[1h]

# Медленные запросы (>10ms)
{container="dns-proxy"} | json
  | latency_ms > 10

# Запросы по конкретному клиенту
{container="dns-proxy"} | json | client_ip = "192.168.1.42"

# Ошибки enricher
{container="dns-proxy"} | json | level = "warn" | component = "cloud_api"

# Топ категорий угроз
{container="dns-proxy"} | json | blocked = "true"
  | line_format "{{.category}}"
```

---

## Метрики Prometheus

Эндпоинт: `http://localhost:8080/metrics`

### Счётчики

| Метрика | Labels | Описание |
|---|---|---|
| `dns_requests_total` | — | Всего DNS-запросов |
| `dns_requests_blocked_total` | — | Заблокировано (sinkhole) |
| `dns_cache_hits_total` | `layer={l1,l2}` | Попадания в кэш по уровням |
| `dns_enricher_calls_total` | `enricher`, `status={ok,error}` | Вызовы enricher-ов |

### Гистограммы

| Метрика | Labels | Описание |
|---|---|---|
| `dns_request_duration_ms` | `blocked={true,false}` | Latency обработки запроса в мс |
| `dns_enricher_duration_ms` | `enricher` | Latency каждого enricher в мс |

### Gauge

| Метрика | Описание |
|---|---|
| `dns_enricher_queue_size` | Текущий размер очереди обогащения |

### Пример Grafana dashboard (PromQL)

```promql
# RPS
rate(dns_requests_total[1m])

# Процент блокировок
rate(dns_requests_blocked_total[1m]) / rate(dns_requests_total[1m]) * 100

# Cache hit rate (L1)
rate(dns_cache_hits_total{layer="l1"}[1m]) / rate(dns_requests_total[1m]) * 100

# p95 latency
histogram_quantile(0.95, rate(dns_request_duration_ms_bucket[5m]))

# Ошибки CloudAPI
rate(dns_enricher_calls_total{enricher="cloud_api",status="error"}[5m])

# Заполненность очереди
dns_enricher_queue_size
```

### Prometheus scrape config

```yaml
scrape_configs:
  - job_name: dns-proxy
    static_configs:
      - targets: ["localhost:8080"]
    metrics_path: /metrics
    scrape_interval: 15s
```

---

## Добавление нового источника (Enricher)

Архитектура намеренно открыта для расширения. Достаточно реализовать интерфейс:

```go
type Enricher interface {
    Name() string
    Enrich(ctx context.Context, domain string, result *DomainResult) error
}
```

### Шаг 1 — создать файл `my_enricher.go`

```go
package main

import (
    "context"
    "fmt"
)

type MyEnricher struct {
    cfg *Config
    // добавь свои поля: http.Client, rate.Limiter, etc.
}

func NewMyEnricher(cfg *Config) *MyEnricher {
    return &MyEnricher{cfg: cfg}
}

// Name — используется как label в метриках Prometheus
// и как component в логах. Должен быть уникальным.
func (e *MyEnricher) Name() string {
    return "my_enricher"
}

func (e *MyEnricher) Enrich(ctx context.Context, domain string, result *DomainResult) error {
    // ctx уже содержит timeout из CLOUDAPI_TIMEOUT
    // result можно изменять: result.Blocked, result.Category, result.Action, result.TTL

    score, err := callMyAPI(ctx, domain)
    if err != nil {
        return fmt.Errorf("my_enricher: %w", err)
    }

    if score > 80 {
        result.Blocked = true
        result.Category = 99   // твоя категория
        result.Action = "block"
        result.Source = e.Name()
    }

    return nil
}
```

### Шаг 2 — зарегистрировать в `main.go`

```go
cloudEnricher := NewCloudAPIEnricher(cfg)
myEnricher    := NewMyEnricher(cfg)

enrichers := []Enricher{
    cloudEnricher,  // выполняются по порядку
    myEnricher,
}

engine := NewCheckEngine(cfg, cache, valkeyClient, enrichers)
```

**Готово.** Метрики `dns_enricher_calls_total{enricher="my_enricher"}` и `dns_enricher_duration_ms{enricher="my_enricher"}` подхватятся автоматически. Логи при `LOG_LEVEL=debug` тоже появятся без дополнительных правок.

### Шаг 3 (опционально) — добавить конфиг

Расширь `Config` в `config.go`:

```go
MyEnricher struct {
    Endpoint string `yaml:"endpoint"`
    APIKey   string `yaml:"api_key"`
} `yaml:"my_enricher"`
```

Добавь загрузку в `LoadConfig()`:

```go
cfg.MyEnricher.Endpoint = getEnv("MY_ENRICHER_ENDPOINT", cfg.MyEnricher.Endpoint)
cfg.MyEnricher.APIKey   = getEnv("MY_ENRICHER_APIKEY",   cfg.MyEnricher.APIKey)
```

---

## Rate Limiting и SLA

### Rate limit к CloudAPI

Реализован через [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate) (token bucket):

```
CLOUDAPI_RPS=50    → 50 запросов в секунду (скорость пополнения токенов)
CLOUDAPI_BURST=100 → до 100 запросов мгновенно при накопленном бюджете
```

При превышении лимита `Enrich()` возвращает ошибку `"rate limit exceeded"` — домен обрабатывается без обогащения (allow by default), метрика `dns_enricher_calls_total{status="error"}` инкрементируется.

### SLA ответа клиенту

DNS-ответ клиент получает **немедленно** — обогащение происходит асинхронно:

```
Client запрос
     │
     ▼
CheckDomain() → L1 hit?  ──yes──→ ответ ~0.01ms
     │
     no
     ▼
             → L2 hit?  ──yes──→ ответ ~1-2ms
     │
     no
     ▼
     Создать DomainResult{action:"allow"}
     Отправить в jobs channel (async)  ──→ воркер → CloudAPI → cache
     │
     ▼
     Ответ клиенту СРАЗУ ~0.1ms
     (следующий запрос к тому же домену вернёт обогащённый результат)
```

Таймаут `CLOUDAPI_TIMEOUT` ограничивает время ожидания воркера внутри enricher, **не влияя на latency ответа клиенту**.

---

## Структура проекта

```
.
├── main.go                  # Точка входа, сборка зависимостей
├── config.go                # Config struct + LoadConfig (YAML + ENV)
├── env.go                   # Helpers: getEnv, getEnvInt, ...
├── models.go                # DomainResult, Stats
├── logger.go                # JSON-логгер (LogEntry, LogDNSRequest, ...)
├── metrics.go               # Prometheus метрики
├── dns_handler.go           # DNS сервер, handleDNS, sinkhole
├── engine.go                # CheckEngine, worker pool, singleflight
├── cache.go                 # L1 кэш (Ristretto)
├── valkey.go                # L2 кэш (Valkey/Redis)
├── enricher.go              # Интерфейс Enricher
├── cloud_api_enricher.go    # Реализация CloudAPI enricher
├── http_server.go           # /health /stats /metrics
├── Dockerfile               # Multi-stage build
├── docker-compose.yml       # dns-proxy + valkey + coredns
├── Corefile                 # CoreDNS: plain + DoT + DoH → dns-proxy
├── setup.sh                 # Генерация сертов + docker compose up
└── config/config.yaml       # (создаётся setup.sh, опционально)
```

---

## Разработка

```bash
# Локальный запуск без Docker
go run . 

# Тест DNS
dig @127.0.0.1 google.com

# Тест DoT
dig @127.0.0.1 -p 853 google.com +tls

# Посмотреть логи
docker compose logs -f dns-proxy | jq .

# Фильтрация только заблокированных
docker compose logs -f dns-proxy | jq 'select(.blocked == true)'

# Ручной просмотр L2 кэша
docker exec -it valkey valkey-cli keys "*"
docker exec -it valkey valkey-cli get "google.com"
```

---

## Требования

- Docker + Docker Compose plugin
- Rocky Linux 10 / любой Linux с Docker
- Порты `53`, `853`, `443`, `8080` свободны
