# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /zerverless ./cmd/orchestrator

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /zerverless /app/zerverless
COPY volunteer /app/volunteer

EXPOSE 8000

CMD ["/app/zerverless"]

