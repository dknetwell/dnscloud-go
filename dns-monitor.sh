#!/bin/bash
# =============================================================
# dns-monitor.sh — живой мониторинг DNS Security Proxy
# Использование: bash dns-monitor.sh [HOST] [INTERVAL_SEC]
# По умолчанию: HOST=127.0.0.1, INTERVAL=5
# =============================================================

HOST="${1:-127.0.0.1}"
INTERVAL="${2:-5}"
STATS_URL="http://${HOST}:8080"

# Цвета
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; DIM='\033[2m'; NC='\033[0m'
BLUE='\033[0;34m'; MAGENTA='\033[0;35m'

# Сохраняем предыдущие значения счётчиков для вычисления дельты (RPS)
prev_total=0; prev_blocked=0; prev_ok=0; prev_err=0
prev_l1=0; prev_l2=0; prev_ts=0

clear_screen() { printf '\033[H\033[2J'; }

fetch_metric() {
    # $1 — строка grep
    echo "$METRICS" | grep -v '^#' | grep "^$1" | awk '{print $NF}' | head -1
}

fetch_metric_label() {
    # $1 — полная строка с labels для точного матча
    echo "$METRICS" | grep -v '^#' | grep "$1" | awk '{print $NF}' | head -1
}

format_number() {
    # Форматирует число с разделителями тысяч
    printf "%'.0f" "${1:-0}" 2>/dev/null || echo "${1:-0}"
}

bar() {
    # Рисует прогресс-бар: bar CURRENT MAX WIDTH
    local val="${1:-0}"; local max="${2:-100}"; local width="${3:-20}"
    local filled=0
    if [ "$max" -gt 0 ] 2>/dev/null; then
        filled=$(python3 -c "print(int(${val}/${max}*${width}))" 2>/dev/null || echo 0)
    fi
    local empty=$((width - filled))
    printf "${GREEN}"
    printf '%0.s█' $(seq 1 $filled 2>/dev/null) 2>/dev/null
    printf "${DIM}"
    printf '%0.s░' $(seq 1 $empty 2>/dev/null) 2>/dev/null
    printf "${NC}"
}

print_header() {
    local now
    now=$(date '+%Y-%m-%d %H:%M:%S')
    echo -e "${BOLD}${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BOLD}${CYAN}║          DNS Security Proxy — Live Monitor                   ║${NC}"
    echo -e "${BOLD}${CYAN}║  Host: ${HOST}   Refresh: ${INTERVAL}s   ${now}  ║${NC}"
    echo -e "${BOLD}${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
}

print_section() {
    echo -e "\n${BOLD}${YELLOW}▶ $1${NC}"
    echo -e "${DIM}──────────────────────────────────────────────────────────────${NC}"
}

monitor_loop() {
    while true; do
        # Получаем данные
        STATS=$(curl -s --connect-timeout 2 "${STATS_URL}/stats" 2>/dev/null)
        METRICS=$(curl -s --connect-timeout 2 "${STATS_URL}/metrics" 2>/dev/null)
        LOGS=$(docker compose logs --no-log-prefix dns-proxy --since "${INTERVAL}s" 2>/dev/null)
        now_ts=$(date +%s)

        clear_screen
        print_header

        # ── Статус подключения ──────────────────────────────────────────
        if [ -z "$STATS" ]; then
            echo -e "\n  ${RED}✖ Прокси недоступен (${STATS_URL})${NC}"
            sleep "$INTERVAL"
            continue
        fi

        # Парсим stats
        total=$(echo "$STATS"   | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('total_requests',0))" 2>/dev/null || echo 0)
        hits=$(echo "$STATS"    | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('cache_hits',0))"     2>/dev/null || echo 0)
        misses=$(echo "$STATS"  | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('cache_misses',0))"   2>/dev/null || echo 0)
        api=$(echo "$STATS"     | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('api_calls',0))"      2>/dev/null || echo 0)
        queue=$(echo "$STATS"   | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('enrichment_queue',0))" 2>/dev/null || echo 0)
        lat_ns=$(echo "$STATS"  | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('avg_latency_ns',0))" 2>/dev/null || echo 0)
        lat_ms=$(python3 -c "print(round(${lat_ns}/1000000,3))" 2>/dev/null || echo 0)

        # Парсим Prometheus метрики
        blocked_total=$(fetch_metric "dns_requests_blocked_total ")
        blocked_total=${blocked_total:-0}
        l1=$(fetch_metric_label 'dns_cache_hits_total{layer="l1"}')
        l1=${l1:-0}; l1=${l1%.*}
        l2=$(fetch_metric_label 'dns_cache_hits_total{layer="l2"}')
        l2=${l2:-0}; l2=${l2%.*}
        enrich_ok=$(fetch_metric_label 'dns_enricher_calls_total{enricher="cloud_api",status="ok"}')
        enrich_ok=${enrich_ok:-0}; enrich_ok=${enrich_ok%.*}
        enrich_err=$(fetch_metric_label 'dns_enricher_calls_total{enricher="cloud_api",status="error"}')
        enrich_err=${enrich_err:-0}; enrich_err=${enrich_err%.*}

        # Вычисляем дельты (RPS за последний интервал)
        dt=$((now_ts - prev_ts))
        rps=0; rps_blocked=0; rps_ok=0; rps_err=0
        if [ "$dt" -gt 0 ] && [ "$prev_ts" -gt 0 ]; then
            rps=$(python3 -c "print(round((${total}-${prev_total})/${dt},1))" 2>/dev/null || echo 0)
            rps_blocked=$(python3 -c "print(round((${blocked_total}-${prev_blocked})/${dt},1))" 2>/dev/null || echo 0)
            rps_ok=$(python3 -c "print(round((${enrich_ok}-${prev_ok})/${dt},1))" 2>/dev/null || echo 0)
            rps_err=$(python3 -c "print(round((${enrich_err}-${prev_err})/${dt},1))" 2>/dev/null || echo 0)
        fi
        prev_total=$total; prev_blocked=$blocked_total; prev_ok=$enrich_ok
        prev_err=$enrich_err; prev_l1=$l1; prev_l2=$l2; prev_ts=$now_ts

        # ── Секция: DNS трафик ──────────────────────────────────────────
        print_section "DNS Трафик"

        block_pct=0
        [ "${total:-0}" -gt 0 ] && block_pct=$(python3 -c "print(round(${blocked_total}/${total}*100,1))" 2>/dev/null || echo 0)

        printf "  %-22s ${BOLD}%s${NC} запросов   ${DIM}(+%.1f/сек)${NC}\n" \
            "Всего запросов:" "$(format_number $total)" "$rps"
        printf "  %-22s ${RED}${BOLD}%s${NC} заблокировано  ${DIM}(+%.1f/сек)${NC}  [%.1f%%]\n" \
            "Заблокировано:" "$(format_number $blocked_total)" "$rps_blocked" "$block_pct"
        printf "  %-22s ${GREEN}%s${NC} мс\n" "Avg latency:" "$lat_ms"
        printf "  %-22s " "Блокировок:"
        bar "$blocked_total" "$total" 30
        echo ""

        # ── Секция: Кэш ────────────────────────────────────────────────
        print_section "Кэш"

        hit_rate=0
        [ "${total:-0}" -gt 0 ] && hit_rate=$(python3 -c "print(round(${hits}/${total}*100,1))" 2>/dev/null || echo 0)

        printf "  %-22s ${GREEN}%s${NC}  ${DIM}(miss: %s)${NC}\n" \
            "L1 hits (Ristretto):" "$(format_number $l1)" "$(format_number $misses)"
        printf "  %-22s ${GREEN}%s${NC}\n" "L2 hits (Valkey):" "$(format_number $l2)"
        printf "  %-22s ${BOLD}%.1f%%${NC}\n" "Cache hit rate:" "$hit_rate"
        printf "  %-22s " "Hit rate:"
        bar "$hits" "$total" 30
        echo ""

        # ── Секция: CloudAPI Enricher ───────────────────────────────────
        print_section "CloudAPI Enricher"

        enrich_total=$((enrich_ok + enrich_err))
        err_rate=0
        [ "$enrich_total" -gt 0 ] && err_rate=$(python3 -c "print(round(${enrich_err}/${enrich_total}*100,1))" 2>/dev/null || echo 0)

        # Статус enricher
        if python3 -c "exit(0 if float('${rps_err}') > 0 else 1)" 2>/dev/null; then
            enrich_status="${RED}⚠ ОШИБКИ${NC}"
        else
            enrich_status="${GREEN}✔ OK${NC}"
        fi

        printf "  %-22s %s\n" "Статус:" "$(echo -e $enrich_status)"
        printf "  %-22s ${GREEN}%s${NC}  ${DIM}(+%.1f/сек)${NC}\n" \
            "Успешно (ok):" "$(format_number $enrich_ok)" "$rps_ok"
        printf "  %-22s ${RED}%s${NC}  ${DIM}(+%.1f/сек)${NC}  [err rate: %.1f%%]\n" \
            "Ошибки:" "$(format_number $enrich_err)" "$rps_err" "$err_rate"
        printf "  %-22s ${YELLOW}%s${NC} задач\n" "Очередь:" "$queue"

        # Алерт если очередь растёт
        if [ "${queue:-0}" -gt 100 ] 2>/dev/null; then
            echo -e "  ${RED}${BOLD}⚠ ВНИМАНИЕ: очередь обогащения > 100 — возможна перегрузка!${NC}"
        fi

        # ── Секция: Последние события за интервал ──────────────────────
        print_section "Последние события (за ${INTERVAL}с)"

        # Заблокированные домены
        blocked_domains=$(echo "$LOGS" | grep '"blocked":true' | \
            python3 -c "
import sys, json
domains = []
for line in sys.stdin:
    try:
        d = json.loads(line)
        if d.get('blocked'):
            domains.append('{} [cat:{}]'.format(d.get('domain','?'), d.get('category','?')))
    except: pass
print('\n'.join(domains[-5:]) if domains else '')
" 2>/dev/null)

        if [ -n "$blocked_domains" ]; then
            echo -e "  ${RED}${BOLD}🚫 Заблокированные домены:${NC}"
            echo "$blocked_domains" | while read d; do
                echo -e "     ${RED}✖${NC} $d"
            done
        else
            echo -e "  ${GREEN}✔ Заблокированных доменов нет${NC}"
        fi

        # Ошибки обогащения за последний интервал
        recent_errors=$(echo "$LOGS" | grep '"msg":"enrich_error"' | \
            python3 -c "
import sys, json
errs = []
for line in sys.stdin:
    try:
        d = json.loads(line)
        errs.append('{}: {}'.format(d.get('domain','?'), d.get('error','?')[:60]))
    except: pass
print('\n'.join(errs[-3:]) if errs else '')
" 2>/dev/null)

        if [ -n "$recent_errors" ]; then
            echo -e "\n  ${YELLOW}⚠ Ошибки enricher:${NC}"
            echo "$recent_errors" | while read e; do
                echo -e "     ${YELLOW}→${NC} $e"
            done
        fi

        # Upstream ошибки
        upstream_errs=$(echo "$LOGS" | grep '"msg":"upstream failed"' | wc -l)
        if [ "${upstream_errs:-0}" -gt 0 ] 2>/dev/null; then
            echo -e "\n  ${RED}⚠ upstream failed: ${upstream_errs} раз за последние ${INTERVAL}с${NC}"
        fi

        # Топ запрашиваемых доменов за интервал
        top_domains=$(echo "$LOGS" | grep '"msg":"dns_request"' | \
            python3 -c "
import sys, json
from collections import Counter
domains = []
for line in sys.stdin:
    try:
        d = json.loads(line)
        if d.get('msg') == 'dns_request':
            domains.append(d.get('domain','?'))
    except: pass
top = Counter(domains).most_common(5)
for domain, count in top:
    print(f'  {count:4d}x  {domain}')
" 2>/dev/null)

        if [ -n "$top_domains" ]; then
            echo -e "\n  ${CYAN}Топ доменов:${NC}"
            echo "$top_domains" | while read line; do
                echo -e "     ${DIM}$line${NC}"
            done
        fi

        # ── Секция: Контейнеры ──────────────────────────────────────────
        print_section "Контейнеры"
        for svc in dns-proxy valkey coredns; do
            running=$(docker inspect --format='{{.State.Running}}' "$svc" 2>/dev/null)
            health=$(docker inspect --format='{{.State.Health.Status}}' "$svc" 2>/dev/null)
            if [ "$running" = "true" ]; then
                if [ "$health" = "healthy" ] || [ -z "$health" ]; then
                    echo -e "  ${GREEN}✔${NC} $svc"
                else
                    echo -e "  ${YELLOW}⚠${NC} $svc (${health})"
                fi
            else
                echo -e "  ${RED}✖${NC} $svc — не запущен"
            fi
        done

        echo -e "\n${DIM}  Обновление каждые ${INTERVAL}с. Ctrl+C для выхода.${NC}"
        sleep "$INTERVAL"
    done
}

# Проверка зависимостей
for cmd in curl python3 docker; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "Ошибка: '$cmd' не найден"
        exit 1
    fi
done

monitor_loop
