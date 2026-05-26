package main

import (
	"log"
	"trendservice/internal/app"
)

func main() {
	if err := app.Start(); err != nil {
		log.Fatalf("application error: %v", err)
	}
}

