# Zerverless

A **non-profit, volunteer-powered distributed compute platform** — like torrents, but for computation. Zero servers needed.

Volunteers donate CPU cycles via browser or native client; developers submit jobs (Wasm modules) that execute on the volunteer network.

## Core Principles

- **Non-profit**: Community-owned, no crypto/tokens, donation-based
- **Easy volunteering**: Open browser tab → donate cycles (zero install)
- **Decentralized**: No single point of failure, DHT-based coordination
- **Secure**: Wasm sandboxing for untrusted code execution
- **Federated**: Multiple organizations can run nodes and mesh together

## Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.22+ |
| IPFS/P2P | [Boxo](https://github.com/ipfs/boxo) + libp2p |
| Wasm Runtime | [wazero](https://github.com/tetratelabs/wazero) |
| HTTP Router | [chi](https://github.com/go-chi/chi) |
| WebSockets | [nhooyr/websocket](https://github.com/nhooyr/websocket) |

## Prerequisites

- Go 1.22+ ([install](https://go.dev/doc/install))
- Docker & Docker Compose (optional, for containerized development)

## Quick Start

```bash
# Clone and setup
git clone <repo>
cd zerverless

# Download dependencies
go mod tidy

# Run as orchestrator (default)
make dev

# Or build and run
make build
./bin/zerverless
```

## Run as Worker

Connect to an existing orchestrator as a worker node:

```bash
./bin/zerverless --worker ws://localhost:8000/ws/volunteer
```

## Docker Development

```bash
# Start with Docker Compose
make docker-dev

# Or production mode
make docker-up
```

## Testing

```bash
make test
```

## URLs When Running

| Service | URL |
|---------|-----|
| Orchestrator A API | http://localhost:8000 |
| Orchestrator B API | http://localhost:8001 |
| Volunteer UI | http://localhost:3000 |

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ Orchestrator A  │◄───►│ Orchestrator B  │◄───►│ Orchestrator C  │
│    (libp2p)     │     │    (libp2p)     │     │    (libp2p)     │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
    ┌────┴────┐             ┌────┴────┐             ┌────┴────┐
    │Volunteers│             │Volunteers│             │Volunteers│
    │(Browser) │             │(Browser) │             │(Browser) │
    └──────────┘             └──────────┘             └──────────┘
```

## Development Phases

1. **Hello Volunteers** - WebSocket server accepts volunteer connections
2. **Run Some Code** - Send jobs to volunteers, get results
3. **Wasm Sandbox** - Execute Wasm instead of raw code
4. **IPFS Storage** - Store Wasm and results using Boxo
5. **P2P Network** - Multiple orchestrators communicate via libp2p
6. **Production Hardening** - Auth, rate limiting, monitoring

## Documentation

- [spec/v1.md](spec/v1.md) - Full specification
- [spec/contracts.md](spec/contracts.md) - API contracts (HTTP, WebSocket, P2P)

## License

MIT

