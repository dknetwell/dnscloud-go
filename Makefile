.PHONY: build run test clean deploy

build:
	docker compose build

run:
	docker compose up -d

stop:
	docker compose down

test:
	go test ./... -v

test-dns:
	dig @172.16.10.16 google.com
	dig @172.16.10.16 test-malware.com

bench:
	go test -bench=. ./checker/...

lint:
	golangci-lint run

clean:
	docker compose down -v
	rm -f dns-security-plugin.so

deploy: build run
	@echo "Deployment completed"

logs:
	docker compose logs -f coredns

health:
	curl http://localhost:8080/health

metrics:
	curl http://localhost:9091/metrics

update-config:
	docker compose restart coredns
