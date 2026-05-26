#!/bin/bash

# Load Testing Script for Trend Service
# This script performs various load tests on the trend service API

set -e

# Configuration from environment or defaults
API_URL="${API_URL:-http://localhost:8080}"
KAFKA_BROKERS="${BROKERS:-localhost:9092}"
KAFKA_TOPIC="${TOPIC:-search-queries}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Trend Service Load Testing ===${NC}"
echo "API URL: $API_URL"
echo "Kafka Brokers: $KAFKA_BROKERS"
echo "Kafka Topic: $KAFKA_TOPIC"
echo ""

# Check if hey is installed
if ! command -v hey &> /dev/null; then
    echo -e "${RED}Error: 'hey' tool is not installed${NC}"
    echo "Install it with: go install github.com/rakyll/hey@latest"
    echo "Or on macOS: brew install hey"
    exit 1
fi

# Check if service is running
echo -e "${YELLOW}Checking if service is running...${NC}"
if ! curl -s "${API_URL}/healthz" > /dev/null; then
    echo -e "${RED}Error: Service is not responding at ${API_URL}${NC}"
    echo "Start the service with: make up"
    exit 1
fi
echo -e "${GREEN}✓ Service is running${NC}"
echo ""

# Function to run load test
run_load_test() {
    local name=$1
    local requests=$2
    local concurrency=$3
    local endpoint=$4
    
    echo -e "${YELLOW}Running: $name${NC}"
    echo "Requests: $requests, Concurrency: $concurrency"
    hey -n "$requests" -c "$concurrency" -m GET "${API_URL}${endpoint}"
    echo ""
}

# Test 1: Basic Load Test
echo -e "${GREEN}=== Test 1: Basic Load (1000 requests, 10 concurrent) ===${NC}"
run_load_test "Basic Load" 1000 10 "/top?n=10"

# Test 2: Medium Load Test
echo -e "${GREEN}=== Test 2: Medium Load (5000 requests, 50 concurrent) ===${NC}"
run_load_test "Medium Load" 5000 50 "/top?n=20"

# Test 3: High Load Test
echo -e "${GREEN}=== Test 3: High Load (10000 requests, 100 concurrent) ===${NC}"
run_load_test "High Load" 10000 100 "/top?n=10"

# Test 4: Stress Test
echo -e "${GREEN}=== Test 4: Stress Test (20000 requests, 200 concurrent) ===${NC}"
run_load_test "Stress Test" 20000 200 "/top?n=50"

# Test 5: Stoplist API Test
echo -e "${GREEN}=== Test 5: Stoplist API (1000 requests, 20 concurrent) ===${NC}"
run_load_test "Stoplist GET" 1000 20 "/stoplist"

echo -e "${GREEN}=== Load Testing Complete ===${NC}"
echo ""
echo -e "${YELLOW}To view metrics in Prometheus:${NC}"
echo "1. Open http://localhost:9090"
echo "2. Try these queries:"
echo "   - rate(trend_top_requests_total[1m])  # Requests per second"
echo "   - histogram_quantile(0.95, rate(trend_top_latency_seconds_bucket[1m]))  # 95th percentile latency"
echo "   - trend_aggregator_events_total  # Total events processed"
echo "   - trend_aggregator_unique_queries  # Unique queries in window"
echo ""
echo -e "${YELLOW}To produce Kafka events:${NC}"
echo "go run scripts/produce_events.go -brokers=${KAFKA_BROKERS} -topic=${KAFKA_TOPIC} -rps=1000 -duration=60"
echo ""
