.PHONY: dev run build test lint clean docker-dev docker-up docker-down worker

# Development
dev:
	go run ./cmd/orchestrator

run: build
	./bin/zerverless

# Run as worker
worker:
	@if [ -z "$(URL)" ]; then echo "Usage: make worker URL=ws://localhost:8000/ws/volunteer"; exit 1; fi
	go run ./cmd/orchestrator --worker $(URL)

# Build
build:
	go build -o bin/zerverless ./cmd/orchestrator

# Test
test:
	go test ./... -v -race -cover

test-short:
	go test ./... -short

# Lint
lint:
	golangci-lint run

fmt:
	go fmt ./...

# Docker
docker-dev:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

# Clean
clean:
	rm -rf bin/ tmp/
	docker compose down -v --rmi local 2>/dev/null || true

# Dependencies
deps:
	go mod download
	go mod tidy

