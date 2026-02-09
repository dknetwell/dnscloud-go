.PHONY: build run stop logs test clean deploy monitor

# Основные команды
build:
	docker compose build

run:
	docker compose up -d

stop:
	docker compose down

logs:
	docker compose logs -f dns-proxy

logs-coredns:
	docker compose logs -f coredns

logs-valkey:
	docker compose logs -f valkey

# Тестирование
test-dns:
	dig @127.0.0.1 google.com
	dig @127.0.0.1 example.com +tcp

test-health:
	curl -s http://localhost:8080/health | jq .
	curl -s http://localhost:8054/health | jq .

test-metrics:
	curl -s http://localhost:9091/metrics | head -5
	curl -s http://localhost:8054/metrics | head -5

# Мониторинг
monitor:
	watch -n 2 'echo "=== Containers ===" && docker compose ps && echo "" && \
		echo "=== Stats ===" && curl -s http://localhost:8054/stats | jq'

# Развертывание
deploy: build run
	@echo "✅ Deployment completed"

# Очистка
clean:
	docker compose down -v
	rm -rf certs/*.key certs/*.crt

# Перезагрузка конфигурации
reload:
	docker compose restart dns-proxy

# Бэкап конфигурации
backup:
	tar -czf backup-$(date +%Y%m%d-%H%M%S).tar.gz config/ .env

# Обновление зависимостей
deps:
	go mod tidy
	go mod download

# Проверка кода
lint:
	gofmt -d *.go

# Бенчмарк
bench:
	ab -c 100 -n 10000 http://localhost:8054/health

# Сборка без Docker
build-local:
	go build -o dns-proxy .

# Запуск локально
run-local: build-local
	./dns-proxy
