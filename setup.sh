#!/bin/bash
set -e

echo "========================================"
echo "DNS Security Proxy - Setup"
echo "========================================"

trap 'echo ""; echo "Interrupted"; exit 1' INT TERM

if ! command -v docker &> /dev/null; then
    echo "Docker not installed"
    exit 1
fi

mkdir -p config certs

if [ ! -f config/config.yaml ]; then
cat > config/config.yaml <<'YAML'
logging:
  level: info
  syslog: false

dns:
  listen_udp: ":53"
  listen_tcp: ":53"
  upstream:
    - "8.8.8.8:53"
  # Глобальный fallback sinkhole (если категория не задана или sinkhole_ipv4 пуст)
  sinkhole_ipv4: "0.0.0.0"
  sinkhole_ipv6: "::"
  max_packet_size: 1232

cloud_api:
  endpoint: "https://example.com/api"
  api_key: "CHANGE_ME"
  insecure_skip_verify: false
  rate_limit: 50
  burst: 100
  timeout_seconds: 2

# ─────────────────────────────────────────────────────────────
# Словарь категорий CloudAPI → действие + sinkhole
#
# Категории:
#   0 - benign/unknown
#   1 - malware
#   2 - command and control
#   3 - phishing
#   4 - dynamicDNS
#   5 - newly registered domain
#   6 - grayware
#   7 - parked
#   8 - proxy
#   9 - allowlist
#
# action: block — заблокировать, вернуть sinkhole
# action: allow — пропустить
#
# sinkhole_ipv4/ipv6:
#   "" или не указан → использовать глобальный dns.sinkhole_ipv4
#   "0.0.0.0"        → hard drop (NXDOMAIN-аналог)
#   "192.168.1.100"  → страница-заглушка (block page)
# ─────────────────────────────────────────────────────────────
categories:
  1:
    name: malware
    action: block
    sinkhole_ipv4: "0.0.0.0"
    sinkhole_ipv6: "::"
  2:
    name: command_and_control
    action: block
    sinkhole_ipv4: "0.0.0.0"
    sinkhole_ipv6: "::"
  3:
    name: phishing
    action: block
    sinkhole_ipv4: "0.0.0.0"
    sinkhole_ipv6: "::"
  4:
    name: dynamic_dns
    action: block
    sinkhole_ipv4: "0.0.0.0"
    sinkhole_ipv6: "::"
  5:
    name: newly_registered
    action: block
    sinkhole_ipv4: "0.0.0.0"
    sinkhole_ipv6: "::"
  6:
    name: grayware
    action: block
    sinkhole_ipv4: ""          # пусто → fallback на dns.sinkhole_ipv4
    sinkhole_ipv6: ""
  7:
    name: parked
    action: allow              # парковка — не блокируем
  8:
    name: proxy
    action: block
    sinkhole_ipv4: "0.0.0.0"
    sinkhole_ipv6: "::"
  9:
    name: allowlist
    action: allow

ttl:
  default: 60
  min: 30
  max: 3600

cache:
  max_cost: 10000000

valkey:
  address: "valkey:6379"
  password: ""
  db: 0

engine:
  worker_count: 50
  worker_queue_size: 10000

http:
  listen: ":8080"
YAML
echo "Created config/config.yaml"
fi

if [ ! -f certs/server.crt ]; then
    echo "Generating self-signed certificates..."
    openssl req -x509 -newkey rsa:2048 -nodes \
        -keyout certs/server.key \
        -out certs/server.crt \
        -days 365 \
        -subj "/CN=dns.local"
fi

chmod 644 certs/* || true

docker compose down || true
docker compose build --no-cache
docker compose up -d

echo "Waiting for services..."
sleep 15

docker compose ps

echo ""
echo "DNS: udp/tcp 53"
echo "DoT: 853"
echo "DoH: https://localhost/dns-query"
echo "Stats: http://localhost:8080/stats"
echo "Metrics: http://localhost:8080/metrics"
