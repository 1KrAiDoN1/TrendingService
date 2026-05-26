.PHONY: help build run test clean docker-up docker-down produce load lint fmt tidy install-tools

help: ## Показать справку
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Собрать бинарник
	go build -o bin/server cmd/server/main.go

run: ## Запустить сервис локально
	go run cmd/server/main.go

test: ## Запустить тесты
	go test ./... -race -count=1 -v

test-coverage: ## Запустить тесты с покрытием
	go test ./... -race -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

bench: ## Запустить бенчмарки
	go test -bench=. -benchmem ./internal/aggregator

clean: ## Очистить артефакты сборки
	rm -rf bin/
	rm -f coverage.out coverage.html

docker-up: ## Запустить все сервисы через Docker Compose
	docker-compose up --build -d

docker-down: ## Остановить все сервисы Docker Compose
	docker-compose down -v

docker-logs: ## Показать логи сервиса
	docker-compose logs -f trending

docker-restart: ## Перезапустить сервис
	docker-compose restart trending

produce: ## Генерировать тестовые события в Kafka (1000 RPS, 60 секунд)
	go run scripts/produce_events.go -rps=1000 -duration=60

produce-high: ## Генерировать события с высокой нагрузкой (5000 RPS, 30 секунд)
	go run scripts/produce_events.go -rps=5000 -duration=30

load: ## Нагрузочное тестирование (требуется hey)
	@if command -v hey > /dev/null; then \
		./scripts/load_test.sh; \
	else \
		echo "Error: 'hey' is not installed. Install it with: go install github.com/rakyll/hey@latest"; \
		exit 1; \
	fi

load-simple: ## Простое нагрузочное тестирование (60s, 200 concurrent)
	hey -z 60s -c 200 'http://localhost:8080/top?n=10'

lint: ## Запустить линтер
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install it from https://golangci-lint.run/usage/install/"; \
	fi

fmt: ## Форматировать код
	go fmt ./...
	gofmt -s -w .

tidy: ## Обновить зависимости
	go mod tidy
	go mod verify

install-tools: ## Установить инструменты для разработки
	go install github.com/rakyll/hey@latest
	@echo "Tools installed successfully"

check: ## Проверить готовность к запуску
	@echo "Checking dependencies..."
	@go mod verify
	@echo "Checking code formatting..."
	@test -z "$$(gofmt -l .)" || (echo "Code is not formatted. Run 'make fmt'" && exit 1)
	@echo "Running tests..."
	@go test ./... -short
	@echo "All checks passed!"

prometheus: ## Открыть Prometheus в браузере
	@echo "Opening Prometheus at http://localhost:9090"
	@open http://localhost:9090 || xdg-open http://localhost:9090 || echo "Please open http://localhost:9090 in your browser"

api-test: ## Тестовые запросы к API
	@echo "Testing /healthz..."
	@curl -s http://localhost:8080/healthz
	@echo "\n\nTesting /top..."
	@curl -s http://localhost:8080/top?n=5 | jq
	@echo "\n\nTesting /stoplist..."
	@curl -s http://localhost:8080/stoplist | jq
