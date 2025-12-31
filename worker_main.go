package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/zerverless/orchestrator/internal/worker"
)

func main() {
	url := "ws://localhost:8000/ws/volunteer"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}

	w := worker.New(url)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	log.Printf("Starting worker, connecting to %s", url)
	if err := w.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
