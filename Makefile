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



docker-up: ## Запустить все сервисы через Docker Compose
	docker-compose up --build -d

docker-down: ## Остановить все сервисы Docker Compose
	docker-compose down -v

docker-logs: ## Показать логи сервиса
	docker-compose logs -f trending

docker-restart: ## Перезапустить сервис
	docker-compose restart trending


produce: 
	go run scripts/produce_events.go -brokers=localhost:9092 -topic=search-queries -rps=1000 -duration=30

load: ## Нагрузочное тестирование (требуется hey)
	@if command -v hey > /dev/null; then \
		./scripts/load_test.sh; \
	else \
		echo "Error: 'hey' is not installed. Install it with: go install github.com/rakyll/hey@latest"; \
		exit 1; \
	fi


lint: ## Запустить линтер
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install it from https://golangci-lint.run/usage/install/"; \
	fi


tidy: ## Обновить зависимости
	go mod tidy
	go mod verify

install-tools: ## Установить инструменты для разработки
	go install github.com/rakyll/hey@latest
	@echo "Tools installed successfully"


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
