package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/segmentio/kafka-go"
)

type SearchEvent struct {
	Query     string `json:"query"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	RequestID string `json:"request_id"`
	Timestamp int64  `json:"timestamp"`
}

func main() {
	brokers := flag.String("brokers", "localhost:9092", "Kafka brokers")
	topic := flag.String("topic", "search-queries", "Kafka topic")
	rps := flag.Int("rps", 5000, "Events per second")
	duration := flag.Int("duration", 120, "Duration in seconds (0 = infinite)")
	flag.Parse()

	log.Printf("Starting event producer: brokers=%s, topic=%s, rps=%d", *brokers, *topic, *rps)

	w := &kafka.Writer{
		Addr:         kafka.TCP(*brokers),
		Topic:        *topic,
		Balancer:     &kafka.Hash{},
		BatchSize:    100,
		BatchTimeout: 10 * time.Millisecond,
		Async:        true,
	}
	defer func() {
		if err := w.Close(); err != nil {
			log.Printf("Failed to close writer: %v", err)
		}
	}()

	// Популярные запросы с разным весом (Zipf distribution)
	queries := []struct {
		query  string
		weight int
	}{
		{"iphone 15 pro", 100},
		{"samsung galaxy s24", 80},
		{"macbook air m3", 60},
		{"airpods pro 2", 50},
		{"playstation 5", 45},
		{"xbox series x", 40},
		{"nintendo switch", 35},
		{"apple watch", 30},
		{"ipad pro", 25},
		{"dyson пылесос", 20},
		{"lego star wars", 18},
		{"кроссовки nike", 15},
		{"куртка зимняя", 12},
		{"термобельё", 10},
		{"велосипед детский", 8},
		{"наушники беспроводные", 7},
		{"чехол для телефона", 6},
		{"powerbank", 5},
		{"умные часы", 4},
		{"фитнес браслет", 3},
	}

	// Создаем взвешенный список
	var weightedQueries []string
	for _, q := range queries {
		for i := 0; i < q.weight; i++ {
			weightedQueries = append(weightedQueries, q.query)
		}
	}

	rand.NewSource(time.Now().UnixNano())
	// Интервал теперь рассчитывается корректно
	interval := time.Second / time.Duration(*rps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	n := 0
	start := time.Now()
	ctx := context.Background()

	log.Printf("Producing events at %d RPS...", *rps)

	for range ticker.C {
		// Генерируем событие
		event := SearchEvent{
			Query:     weightedQueries[rand.Intn(len(weightedQueries))],
			UserID:    fmt.Sprintf("user_%d", rand.Intn(10000)),
			SessionID: fmt.Sprintf("sess_%d", rand.Intn(50000)),
			RequestID: fmt.Sprintf("req_%d_%d", time.Now().UnixNano(), rand.Intn(1000)),
			Timestamp: time.Now().Unix(),
		}

		b, err := json.Marshal(event)
		if err != nil {
			log.Printf("Failed to marshal event: %v", err)
			continue
		}

		err = w.WriteMessages(ctx, kafka.Message{
			Key:   []byte(event.UserID),
			Value: b,
		})
		if err != nil {
			log.Printf("Failed to write message: %v", err)
			continue
		}

		n++
		if n%25000 == 0 {
			elapsed := time.Since(start)
			actualRPS := float64(n) / elapsed.Seconds()
			log.Printf("Sent: %d events, Elapsed: %s, Actual RPS: %.2f", n, elapsed.Round(time.Second), actualRPS)
		}

		// Проверяем длительность
		if *duration > 0 && time.Since(start) >= time.Duration(*duration)*time.Second {
			log.Printf("Duration reached. Total events sent: %d", n)
			break
		}
	}

	log.Printf("Producer finished. Total events: %d, Duration: %s", n, time.Since(start).Round(time.Second))
}
