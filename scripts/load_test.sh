#!/bin/bash

# Скрипт для нагрузочного тестирования Trend Service

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Проверка наличия hey
if ! command -v hey &> /dev/null; then
    echo -e "${RED}Error: 'hey' is not installed${NC}"
    echo "Install it with: go install github.com/rakyll/hey@latest"
    echo "Or on macOS: brew install hey"
    exit 1
fi

# Проверка доступности сервиса
echo -e "${YELLOW}Checking service availability...${NC}"
if ! curl -s http://localhost:8080/healthz > /dev/null; then
    echo -e "${RED}Error: Service is not available at http://localhost:8080${NC}"
    echo "Start the service with: docker-compose up -d"
    exit 1
fi
echo -e "${GREEN}Service is available${NC}"
echo ""

# Тест 1: Базовая нагрузка
echo -e "${YELLOW}=== Test 1: Basic Load (10k requests, 100 concurrent) ===${NC}"
hey -n 10000 -c 100 http://localhost:8080/top
echo ""

# Тест 2: Средняя нагрузка
echo -e "${YELLOW}=== Test 2: Medium Load (30s, 200 concurrent) ===${NC}"
hey -z 30s -c 200 http://localhost:8080/top?n=10
echo ""

# Тест 3: Высокая нагрузка
echo -e "${YELLOW}=== Test 3: High Load (30s, 500 concurrent) ===${NC}"
hey -z 30s -c 500 http://localhost:8080/top?n=20
echo ""

# Тест 4: Стресс-тест
echo -e "${YELLOW}=== Test 4: Stress Test (20s, 1000 concurrent) ===${NC}"
hey -z 20s -c 1000 http://localhost:8080/top
echo ""

# Тест 5: Stoplist API
echo -e "${YELLOW}=== Test 5: Stoplist API (5k requests, 50 concurrent) ===${NC}"
hey -n 5000 -c 50 http://localhost:8080/stoplist
echo ""

echo -e "${GREEN}All tests completed!${NC}"
echo ""
echo -e "${YELLOW}Check Prometheus metrics at: http://localhost:9090${NC}"
echo "Useful queries:"
echo "  - RPS: rate(trending_top_requests_total[1m])"
echo "  - P99 Latency: histogram_quantile(0.99, rate(trending_top_latency_seconds_bucket[5m]))"
echo "  - Drop Rate: rate(trending_events_dropped_total[1m]) / rate(trending_events_consumed_total[1m])"
