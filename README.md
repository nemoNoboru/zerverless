# Zerverless

A **volunteer-powered distributed compute platform** — like torrents, but for computation.

Volunteers donate CPU cycles via browser or native client; developers submit jobs or deploy serverless functions that execute on the volunteer network.

## Core Principles

- **Easy volunteering**: Open browser tab → donate cycles (zero install)
- **Multi-runtime**: Execute WebAssembly, Python, Lua, or JavaScript
- **Serverless deployments**: Deploy functions at custom paths with HTTP invocation
- **Secure**: Sandboxed execution for untrusted code
- **Federated**: Multiple orchestrators can run and mesh together

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

## Code Execution

Workers support multiple runtimes:

- **WebAssembly** - Via [wazero](https://github.com/tetratelabs/wazero) (pure Go, no CGO)
- **Python** - Via CPython WASM with full stdlib support ([wasmtime-go](https://github.com/bytecodealliance/wasmtime-go))
- **Lua** - Pure Go runtime ([gopher-lua](https://github.com/yuin/gopher-lua))
- **JavaScript** - Pure Go runtime ([goja](https://github.com/dop251/goja))

See [examples/](examples/) for samples.

```bash
# Build Wasm example (requires TinyGo)
make wasm-examples

# Enable Python support (requires python.wasm + stdlib)
./bin/zerverless --worker ws://localhost:8000/ws/volunteer --python ./python.wasm --python-lib ./lib
```

## Usage Workflow

### One-Time Job Execution
1. Start orchestrator: `make dev`
2. Connect worker: `./bin/zerverless --worker ws://localhost:8000/ws/volunteer`
3. Submit job via API (see examples below)
4. Poll for job completion
5. Retrieve results

### Serverless Function Deployment
1. Deploy function to path: `POST /api/deploy/{user}/{path}`
2. Invoke via HTTP: `GET|POST /{user}/{path}`
3. Function executes synchronously on available worker
4. Response returned immediately

## API Examples

> 💡 **Quick Start**: Import the [Postman collection](Zerverless.postman_collection.json) for ready-to-use API requests!

### Health & Stats

```bash
# Check orchestrator health
curl http://localhost:8000/health

# Get node info
curl http://localhost:8000/info

# Get stats (volunteers, jobs)
curl http://localhost:8000/stats
```

### Submit Jobs

#### Run Python Code

```bash
# Submit Python job
curl -X POST http://localhost:8000/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "job_type": "python",
    "code": "print(\"Hello from Python!\")\nprint(2 + 2)",
    "timeout_seconds": 30
  }'

# Returns: {"id":"job-uuid-here",...}
```

#### Run JavaScript Code

```bash
# Submit JavaScript job
curl -X POST http://localhost:8000/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "job_type": "js",
    "code": "console.log(\"Hello from JS!\"); 42",
    "timeout_seconds": 30
  }'
```

#### Run Lua Code

```bash
# Submit Lua job with input data
curl -X POST http://localhost:8000/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "job_type": "lua",
    "code": "print(\"Hello \" .. INPUT.name); return INPUT.x + INPUT.y",
    "input_data": {"name": "World", "x": 10, "y": 32},
    "timeout_seconds": 30
  }'
```

### Check Job Status

```bash
# Get specific job
curl http://localhost:8000/api/jobs/JOB_ID_HERE

# List all jobs
curl http://localhost:8000/api/jobs

# List only completed jobs
curl "http://localhost:8000/api/jobs?status=completed&limit=10"

# List only pending jobs
curl "http://localhost:8000/api/jobs?status=pending"
```

### Serverless Deployments

#### Deploy a Function

```bash
# Deploy a Lua function at /alice/hello
curl -X POST http://localhost:8000/api/deploy/alice/hello \
  -H "Content-Type: application/json" \
  -d '{
    "runtime": "lua",
    "code": "function handle(req) return {status=200, body=\"Hello, \" .. req.query.name .. \"!\"} end"
  }'

# Deploy a JavaScript function at /bob/api/greet
curl -X POST http://localhost:8000/api/deploy/bob/api/greet \
  -H "Content-Type: application/json" \
  -d '{
    "runtime": "js",
    "code": "function handle(req) { return {status: 200, headers: {\"Content-Type\": \"application/json\"}, body: JSON.stringify({message: \"Hello \" + req.query.name})} }"
  }'

# Deploy a Python function at /charlie/python-hello
curl -X POST http://localhost:8000/api/deploy/charlie/python-hello \
  -H "Content-Type: application/json" \
  -d '{
    "runtime": "python",
    "code": "import json\ndef handle(req):\n    name = req.get(\"query\", {}).get(\"name\", \"World\")\n    return {\"status\": 200, \"headers\": {\"Content-Type\": \"text/plain\"}, \"body\": f\"Hello from Python, {name}!\"}\nresult = handle(INPUT)\nprint(json.dumps(result))"
  }'
```

#### Invoke Deployed Functions

```bash
# Call the deployed function
curl "http://localhost:8000/alice/hello?name=World"
# Returns: Hello, World!

curl "http://localhost:8000/bob/api/greet?name=Alice"
# Returns: {"message":"Hello Alice"}

# POST with body
curl -X POST http://localhost:8000/alice/hello \
  -H "Content-Type: application/json" \
  -d '{"data": "test"}'
```

#### List and Delete Deployments

```bash
# List all deployments
curl http://localhost:8000/api/deploy

# Delete a deployment
curl -X DELETE http://localhost:8000/api/deploy/alice/hello
```

### Function Handler Interface

Deployed functions must export a `handle(request)` function:

**Lua:**
```lua
function handle(req)
  -- req.method, req.path, req.query, req.headers, req.body
  return {
    status = 200,
    headers = {["Content-Type"] = "application/json"},
    body = "response body"
  }
end
```

**JavaScript:**
```javascript
function handle(req) {
  // req.method, req.path, req.query, req.headers, req.body
  return {
    status: 200,
    headers: {"Content-Type": "application/json"},
    body: "response body"
  };
}
```

**Python:**
```python
import json

def handle(req):
  # req is a dict with: method, path, query, headers, body
  return {
    "status": 200,
    "headers": {"Content-Type": "application/json"},
    "body": "response body"
  }

# Required: Call handle and print result as JSON
result = handle(INPUT)
print(json.dumps(result))
```

## URLs When Running

| Service | URL |
|---------|-----|
| Orchestrator API | http://localhost:8000 |
| Volunteer UI | http://localhost:8000/volunteer/ |

## Architecture

```
                    ┌─────────────────────────┐
                    │    Orchestrator Node    │
                    │   ┌─────────────────┐   │
                    │   │   HTTP API      │   │  ← Job submission
                    │   │   /api/jobs     │   │  ← Deployments
                    │   │   /{user}/*     │   │  ← Function invocation
                    │   └─────────────────┘   │
                    │   ┌─────────────────┐   │
                    │   │ WebSocket Server│   │
                    │   │ /ws/volunteer   │   │
                    │   └─────────────────┘   │
                    │   ┌─────────────────┐   │
                    │   │ Job Dispatcher  │   │
                    │   │ Volunteer Mgr   │   │
                    │   └─────────────────┘   │
                    └───────────┬─────────────┘
                                │ WebSocket
                ┌───────────────┼───────────────┐
                │               │               │
         ┌──────▼─────┐  ┌──────▼─────┐  ┌──────▼─────┐
         │  Worker    │  │  Worker    │  │  Browser   │
         │  (Native)  │  │  (Docker)  │  │ Volunteer  │
         │            │  │            │  │            │
         │ Wasm │Lua │  │ Python│JS  │  │  JS │Wasm  │
         └────────────┘  └────────────┘  └────────────┘
```

Future: Multiple orchestrators will mesh via libp2p for decentralized operation.

## Development Status

### ✅ Completed
1. **Volunteer Management** - WebSocket connections with capability matching
2. **Job Execution** - Async job submission with status tracking
3. **Multi-Runtime Support** - WebAssembly, Python, Lua, JavaScript
4. **Serverless Deployments** - Deploy functions at custom HTTP paths
5. **Browser Volunteers** - HTML/JS volunteer client included

### 🚧 In Progress
6. **IPFS Storage** - Store Wasm modules and results using Boxo
7. **P2P Network** - Multiple orchestrators communicate via libp2p

### 📋 Planned
8. **Production Hardening** - Auth, rate limiting, monitoring
9. **DHT-based Discovery** - Decentralized orchestrator discovery

## Documentation

- [spec/v1.md](spec/v1.md) - Full specification
- [spec/contracts.md](spec/contracts.md) - API contracts (HTTP, WebSocket, P2P)
- [Zerverless.postman_collection.json](Zerverless.postman_collection.json) - Postman API collection

### Using the Postman Collection

1. Open Postman
2. Click **Import** → **Upload Files**
3. Select `Zerverless.postman_collection.json`
4. The collection includes:
   - All API endpoints pre-configured
   - Example requests for each runtime (Python, JS, Lua)
   - Auto-saves job IDs from submissions
   - Deployment and invocation examples
5. Start your orchestrator and worker, then execute requests!

## License

MIT

