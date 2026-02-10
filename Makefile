.PHONY: build up down logs test clean tidy

build:
        docker compose build

up:
        docker compose up -d

down:
        docker compose down

logs:
        docker compose logs -f

logs-proxy:
        docker compose logs -f dns-proxy

logs-coredns:
        docker compose logs -f coredns

test-dns:
        dig @127.0.0.1 example.com
        dig @127.0.0.1 example.com +tcp

test-health:
        curl http://localhost:8080/health
        curl http://localhost:8054/health

clean:
        docker compose down -v

network:
        docker network inspect dnscloud-go_dns-net

restart:
        docker compose restart

# Новая команда для обновления зависимостей
tidy:
        go mod tidy
