#!/bin/bash

# Start a worker that connects to the orchestrator
# This worker will have Docker capability if Docker is available

ORCHESTRATOR_URL="ws://localhost:8000/ws/volunteer"

echo "Starting worker connecting to $ORCHESTRATOR_URL"
echo "Press Ctrl+C to stop"
echo ""

# Use the worker package directly
go run -exec ./orchestrator --worker "$ORCHESTRATOR_URL" 2>&1 || {
  # If that doesn't work, create a simple worker
  cat > /tmp/worker.go << 'EOF'
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
EOF
  go run /tmp/worker.go "$ORCHESTRATOR_URL"
}




