#!/bin/bash

# ============================================================
# DNS Security Proxy — проверочный скрипт
# Использование: bash check.sh [HOST]
# По умолчанию HOST=127.0.0.1
# ============================================================

HOST="${1:-127.0.0.1}"
STATS_URL="http://${HOST}:8080"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

pass() { echo -e "  ${GREEN}✔${NC} $1"; }
fail() { echo -e "  ${RED}✖${NC} $1"; }
info() { echo -e "  ${CYAN}→${NC} $1"; }
section() { echo -e "\n${BOLD}${YELLOW}══ $1 ══${NC}"; }

ERRORS=0

check_contains() {
    local desc="$1"
    local result="$2"
    local expected="$3"
    if echo "$result" | grep -qE "$expected"; then
        pass "$desc"
    else
        fail "$desc (ожидали: '$expected', получили: '$(echo $result | head -c 80)')"
        ERRORS=$((ERRORS+1))
    fi
}

check_eq() {
    local desc="$1"
    local result="$2"
    local expected="$3"
    if [ "$result" = "$expected" ]; then
        pass "$desc"
    else
        fail "$desc (ожидали: '$expected', получили: '$result')"
        ERRORS=$((ERRORS+1))
    fi
}

# ============================================================
section "1. Healthcheck"
# ============================================================

resp=$(curl -s -o /dev/null -w "%{http_code}" "${STATS_URL}/health" 2>/dev/null)
check_eq "HTTP /health → 200" "$resp" "200"

# ============================================================
section "2. DNS — plain UDP :53"
# ============================================================

r=$(dig +short @${HOST} google.com A 2>/dev/null | head -1)
check_contains "google.com A (UDP)" "$r" "\."

r=$(dig +short @${HOST} google.com AAAA 2>/dev/null | head -1)
check_contains "google.com AAAA (UDP)" "$r" ":"

r=$(dig +short @${HOST} yan.ru A 2>/dev/null | head -1)
check_contains "yan.ru A (UDP)" "$r" "\."

# MX — проверяем наличие ответа (не +short, он может быть пустым для некоторых зон)
r=$(dig @${HOST} example.com MX +noall +answer 2>/dev/null)
check_contains "example.com MX (UDP)" "$r" "MX"

# Проверяем через +tcp явно
r=$(dig +short +tcp @${HOST} google.com TXT 2>/dev/null | head -1)
check_contains "google.com TXT (TCP)" "$r" "."

# ============================================================
section "3. DNS — plain TCP :53"
# ============================================================

r=$(dig +short +tcp @${HOST} google.com A 2>/dev/null | head -1)
check_contains "google.com A (TCP)" "$r" "\."

r=$(dig +short +tcp @${HOST} ya.ru A 2>/dev/null | head -1)
check_contains "ya.ru A (TCP)" "$r" "\."

# ============================================================
section "4. DNS-over-TLS (DoT) :853"
# ============================================================

r=$(dig +short +tls @${HOST} -p 853 google.com A 2>/dev/null | head -1)
check_contains "google.com A (DoT)" "$r" "\."

r=$(dig +short +tls @${HOST} -p 853 github.com A 2>/dev/null | head -1)
check_contains "github.com A (DoT)" "$r" "\."

# Порт 853 без TLS — dig должен упасть с ошибкой (exit code != 0)
dig +short @${HOST} -p 853 google.com A 2>/dev/null
if [ $? -ne 0 ]; then
    pass "порт 853 без TLS — соединение отклонено (ожидаемо)"
else
    fail "порт 853 без TLS неожиданно ответил"
    ERRORS=$((ERRORS+1))
fi

# ============================================================
section "5. DNS-over-HTTPS (DoH) :443"
# ============================================================

tls_cn=$(curl -sk -v "https://${HOST}/dns-query" 2>&1 | grep "subject:" | head -1)
check_contains "DoH TLS handshake" "$tls_cn" "CN="

# HTTP 400 на заведомо кривой запрос
code=$(curl -sk -o /dev/null -w "%{http_code}" "https://${HOST}/dns-query?dns=AA" 2>/dev/null)
check_eq "DoH /dns-query?dns=AA → 400 (некорректный запрос)" "$code" "400"

# Валидный DoH запрос — google.com A в wire format, base64url без padding
DOH_QUERY="AAABAAABAAAAAAAABmdvb2dsZQNjb20AAAEAAQ"
code=$(curl -sk -o /dev/null -w "%{http_code}" \
    -H "Accept: application/dns-message" \
    "https://${HOST}/dns-query?dns=${DOH_QUERY}" 2>/dev/null)
check_eq "DoH валидный запрос google.com A → 200" "$code" "200"

# ============================================================
section "6. Кэш — L1 (in-memory) и L2 (Valkey)"
# ============================================================

# Прогреваем домен
dig +short @${HOST} warmup-cache.example.com A &>/dev/null
sleep 0.5

# Сравниваем latency первого и повторного запроса
t1=$(dig +stats @${HOST} google.com A 2>/dev/null | grep "Query time" | awk '{print $4}')
t2=$(dig +stats @${HOST} google.com A 2>/dev/null | grep "Query time" | awk '{print $4}')
info "google.com: 1-й запрос ${t1}ms, повторный ${t2}ms (из кэша должен быть быстрее)"
if [ "${t2:-999}" -le "${t1:-0}" ] 2>/dev/null; then
    pass "L1 кэш работает (повторный запрос не медленнее)"
else
    info "Время повторного запроса (${t2}ms) > первого (${t1}ms) — возможно оба из кэша"
fi

# Valkey — список ключей
info "Ключи в Valkey:"
keys=$(docker exec valkey valkey-cli keys "*" 2>/dev/null)
if [ -n "$keys" ]; then
    echo "$keys" | head -10 | while read k; do echo "    $k"; done
    count=$(echo "$keys" | wc -l)
    pass "В Valkey ${count} ключей"
else
    fail "Valkey пуст"
    ERRORS=$((ERRORS+1))
fi

# Valkey — содержимое конкретного ключа
info "Valkey: google.com →"
val=$(docker exec valkey valkey-cli get "google.com" 2>/dev/null)
if [ -n "$val" ]; then
    echo "$val" | python3 -m json.tool 2>/dev/null | sed 's/^/    /'
    pass "google.com есть в L2 кэше"

    # Проверяем что source заполнен после enrichment
    src=$(echo "$val" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('source',''))" 2>/dev/null)
    if [ "$src" = "cloud_api" ]; then
        pass "source=cloud_api (enrichment отработал и обновил кэш)"
    else
        info "source=${src} (enrichment ещё не обновил запись или CloudAPI не настроен)"
    fi
else
    info "google.com не в L2 кэше (enrichment ещё не завершился)"
fi

# ============================================================
section "7. Stats API"
# ============================================================

stats=$(curl -s "${STATS_URL}/stats" 2>/dev/null)
if [ -z "$stats" ]; then
    fail "Stats API недоступен"
    ERRORS=$((ERRORS+1))
else
    pass "Stats API отвечает"
    echo "$stats" | python3 -m json.tool 2>/dev/null | sed 's/^/    /'

    total=$(echo "$stats"  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('total_requests',0))" 2>/dev/null)
    hits=$(echo "$stats"   | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('cache_hits',0))"     2>/dev/null)
    misses=$(echo "$stats" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('cache_misses',0))"   2>/dev/null)
    api=$(echo "$stats"    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('api_calls',0))"      2>/dev/null)
    lat_ns=$(echo "$stats" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('avg_latency_ns',0))" 2>/dev/null)
    lat_ms=$(python3 -c "print(round(${lat_ns:-0}/1000000, 3))" 2>/dev/null)

    info "total_requests : ${total}"
    info "cache_hits     : ${hits}"
    info "cache_misses   : ${misses}"
    info "api_calls      : ${api}"
    info "avg_latency_ns : ${lat_ns} (${lat_ms} ms)"

    [ "${total:-0}" -gt 0 ] && pass "Запросы обрабатываются (total=${total})" \
                             || fail "Нет ни одного запроса"

    # Соотношение hits/misses
    if [ "${total:-0}" -gt 0 ] && [ "${hits:-0}" -gt 0 ]; then
        ratio=$(python3 -c "print(round(${hits}/${total}*100, 1))" 2>/dev/null)
        info "Cache hit rate: ${ratio}%"
    fi
fi

# ============================================================
section "8. Prometheus метрики"
# ============================================================

metrics=$(curl -s "${STATS_URL}/metrics" 2>/dev/null)
if [ -z "$metrics" ]; then
    fail "Metrics endpoint недоступен"
    ERRORS=$((ERRORS+1))
else
    pass "/metrics отвечает"

    for metric in \
        "dns_requests_total" \
        "dns_requests_blocked_total" \
        "dns_cache_hits_total" \
        "dns_enricher_calls_total" \
        "dns_request_duration_ms" \
        "dns_enricher_duration_ms" \
        "dns_enricher_queue_size"
    do
        if echo "$metrics" | grep -q "^${metric}"; then
            val=$(echo "$metrics" | grep "^${metric}" | grep -v "^#" | head -1 | awk '{print $NF}')
            pass "метрика ${metric} = ${val}"
        else
            fail "метрика ${metric} отсутствует"
            ERRORS=$((ERRORS+1))
        fi
    done

    echo ""
    info "Ключевые счётчики:"
    echo "$metrics" | grep -v "^#" | grep -E \
        "^(dns_requests_total|dns_requests_blocked_total|dns_enricher_queue_size) " \
        | awk '{printf "    %-50s %s\n", $1, $2}'

    info "Cache hits по слоям:"
    echo "$metrics" | grep -v "^#" | grep "dns_cache_hits_total" \
        | awk '{printf "    %-55s %s\n", $1, $2}'

    info "Enricher calls (ok/error — здесь виден статус CloudAPI):"
    echo "$metrics" | grep -v "^#" | grep "dns_enricher_calls_total" \
        | awk '{printf "    %-60s %s\n", $1, $2}'

    # Явная проверка: были ли успешные вызовы enricher
    ok_calls=$(echo "$metrics" | grep 'dns_enricher_calls_total{.*status="ok"' | grep -v "^#" | awk '{print $NF}' | head -1)
    err_calls=$(echo "$metrics" | grep 'dns_enricher_calls_total{.*status="error"' | grep -v "^#" | awk '{print $NF}' | head -1)
    ok_calls=${ok_calls:-0}
    err_calls=${err_calls:-0}

    if python3 -c "exit(0 if float('${ok_calls}') > 0 else 1)" 2>/dev/null; then
        pass "CloudAPI enricher: ${ok_calls} успешных вызовов"
    else
        info "CloudAPI enricher: 0 успешных вызовов (endpoint не настроен или ошибка)"
    fi
    if python3 -c "exit(0 if float('${err_calls}') > 0 else 1)" 2>/dev/null; then
        fail "CloudAPI enricher: ${err_calls} ошибок — проверь CLOUDAPI_ENDPOINT и CLOUDAPI_APIKEY"
        ERRORS=$((ERRORS+1))
    fi

    info "p95 latency DNS запросов (из histogram):"
    echo "$metrics" | grep "dns_request_duration_ms_bucket" | grep -v "^#" \
        | awk '{printf "    %s\n", $0}' | tail -5
fi

# ============================================================
section "9. Логи dns-proxy"
# ============================================================

info "Последние dns_request события:"
docker compose logs --no-log-prefix dns-proxy 2>/dev/null \
    | grep '"msg":"dns_request"' \
    | tail -5 \
    | while read line; do
        domain=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('domain','?'))" 2>/dev/null)
        latency=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('latency_ms','?'))" 2>/dev/null)
        blocked=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('blocked','?'))" 2>/dev/null)
        source=$(echo "$line"  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('source','?'))"  2>/dev/null)
        qtype=$(echo "$line"   | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('qtype','?'))"   2>/dev/null)
        echo "    ${qtype} ${domain} → blocked=${blocked}, source=${source}, latency=${latency}ms"
    done

echo ""
info "Последние enrich_ok события (latency к CloudAPI, категория):"
docker compose logs --no-log-prefix dns-proxy 2>/dev/null \
    | grep '"msg":"enrich_ok"' \
    | tail -5 \
    | while read line; do
        domain=$(echo "$line"   | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('domain','?'))"    2>/dev/null)
        latency=$(echo "$line"  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('latency_ms','?'))" 2>/dev/null)
        category=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('category','?'))"  2>/dev/null)
        action=$(echo "$line"   | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('action','?'))"    2>/dev/null)
        blocked=$(echo "$line"  | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('blocked','?'))"   2>/dev/null)
        echo "    ${domain} → category=${category}, action=${action}, blocked=${blocked}, api_latency=${latency}ms"
    done

echo ""
info "Последние enrich_error события:"
errs=$(docker compose logs --no-log-prefix dns-proxy 2>/dev/null | grep '"msg":"enrich_error"' | tail -5)
if [ -z "$errs" ]; then
    pass "Нет enrich_error в логах"
else
    echo "$errs" | while read line; do
        domain=$(echo "$line" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('domain','?'))" 2>/dev/null)
        err=$(echo "$line"    | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('error','?'))"  2>/dev/null)
        echo -e "    ${RED}${domain}: ${err}${NC}"
    done
    ERRORS=$((ERRORS+1))
fi

echo ""
info "Warn/Error уровня в логах:"
warns=$(docker compose logs --no-log-prefix dns-proxy 2>/dev/null \
    | grep -E '"level":"(warn|error|fatal)"')
if [ -z "$warns" ]; then
    pass "Нет warn/error/fatal в логах"
else
    echo "$warns" | tail -10 | while read line; do
        echo -e "    ${YELLOW}$line${NC}"
    done
fi

# ============================================================
section "10. Контейнеры"
# ============================================================

for svc in dns-proxy valkey coredns; do
    status=$(docker inspect --format='{{.State.Health.Status}}' "$svc" 2>/dev/null)
    running=$(docker inspect --format='{{.State.Running}}' "$svc" 2>/dev/null)
    uptime=$(docker inspect --format='{{.State.StartedAt}}' "$svc" 2>/dev/null | cut -dT -f1)
    if [ "$running" = "true" ]; then
        if [ "$status" = "healthy" ]; then
            pass "$svc — running, healthy (since ${uptime})"
        elif [ -z "$status" ]; then
            pass "$svc — running (нет healthcheck, since ${uptime})"
        else
            fail "$svc — running, статус: $status"
            ERRORS=$((ERRORS+1))
        fi
    else
        fail "$svc — не запущен"
        ERRORS=$((ERRORS+1))
    fi
done

# ============================================================
echo ""
echo -e "${BOLD}══════════════════════════════════════${NC}"
if [ "$ERRORS" -eq 0 ]; then
    echo -e "${GREEN}${BOLD}  Все проверки пройдены успешно ✔${NC}"
else
    echo -e "${RED}${BOLD}  Провалено проверок: ${ERRORS} ✖${NC}"
fi
echo -e "${BOLD}══════════════════════════════════════${NC}"
exit $ERRORS
