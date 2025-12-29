# Embedded Database Design for Zerverless

## Overview

Each orchestrator hosts an embedded NoSQL database that workers can easily access during job execution. This enables stateful workloads, caching, and data persistence.

## Database Choice: BadgerDB

**Why BadgerDB?**
- ✅ Pure Go, no CGO dependencies
- ✅ Fast key-value store
- ✅ Embedded (no separate process)
- ✅ ACID transactions
- ✅ Simple API
- ✅ Low memory footprint

**Alternatives considered:**
- BoltDB: Simpler but slower, less active
- Pebble: More complex, RocksDB-inspired
- buntdb: In-memory only

## Architecture

### Database Structure

```
Orchestrator
├── Embedded BadgerDB
│   ├── Namespace: user/{user_id}/
│   │   ├── Key: "key1" → Value: "value1"
│   │   ├── Key: "key2" → Value: {"json": "object"}
│   │   └── Collections: "users", "sessions", etc.
│   └── Global namespace: "_global/"
│       └── System keys
└── API Endpoints
    ├── GET /api/db/{namespace}/{key}
    ├── PUT /api/db/{namespace}/{key}
    ├── DELETE /api/db/{namespace}/{key}
    └── POST /api/db/{namespace}/query
```

### Access Methods

1. **HTTP API** (Primary)
   - Workers make HTTP requests during job execution
   - Simple REST interface
   - JSON responses

2. **WebSocket Messages** (Optional)
   - Workers can request database operations via WebSocket
   - Useful for high-frequency operations
   - Lower latency

3. **Direct Access** (Future)
   - Workers connect directly to database
   - Requires network access to orchestrator

## API Design

### Key-Value Operations

#### Get Value

```http
GET /api/db/{namespace}/{key}
```

**Response:**
```json
{
  "key": "user:123",
  "value": {"name": "Alice", "age": 30},
  "exists": true
}
```

#### Set Value

```http
PUT /api/db/{namespace}/{key}
Content-Type: application/json

{
  "value": {"name": "Alice", "age": 30},
  "ttl": 3600
}
```

#### Delete Key

```http
DELETE /api/db/{namespace}/{key}
```

#### List Keys (Prefix Scan)

```http
GET /api/db/{namespace}?prefix=user:&limit=100
```

**Response:**
```json
{
  "keys": ["user:1", "user:2", "user:3"],
  "count": 3
}
```

### Query Operations

#### Batch Get

```http
POST /api/db/{namespace}/batch
Content-Type: application/json

{
  "keys": ["key1", "key2", "key3"]
}
```

**Response:**
```json
{
  "results": {
    "key1": {"value": "value1", "exists": true},
    "key2": {"value": null, "exists": false},
    "key3": {"value": "value3", "exists": true}
  }
}
```

#### Batch Set

```http
PUT /api/db/{namespace}/batch
Content-Type: application/json

{
  "items": [
    {"key": "key1", "value": "value1"},
    {"key": "key2", "value": "value2"}
  ]
}
```

### Namespace Isolation

- **User Namespaces**: `user/{user_id}/` - Isolated per user
- **Job Namespaces**: `job/{job_id}/` - Temporary, auto-cleanup
- **Global Namespace**: `_global/` - Shared across users (admin only)

## Usage Examples

### From Worker Code (Lua)

```lua
-- During job execution, make HTTP request to orchestrator
local http = require("http")
local db_url = os.getenv("ZERVERLESS_DB_URL") or "http://localhost:8000"

-- Get value
local response = http.get(db_url .. "/api/db/user/alice/key1")
local data = json.decode(response.body)

-- Set value
http.put(db_url .. "/api/db/user/alice/key1", {
  headers = {["Content-Type"] = "application/json"},
  body = json.encode({value = "new value"})
})
```

### From Worker Code (JavaScript)

```javascript
const dbUrl = process.env.ZERVERLESS_DB_URL || 'http://localhost:8000';

// Get value
const response = await fetch(`${dbUrl}/api/db/user/alice/key1`);
const data = await response.json();

// Set value
await fetch(`${dbUrl}/api/db/user/alice/key1`, {
  method: 'PUT',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({value: {name: 'Alice', count: 42}})
});
```

### From Worker Code (Python)

```python
import os
import json
import urllib.request

db_url = os.getenv("ZERVERLESS_DB_URL", "http://localhost:8000")

# Get value
with urllib.request.urlopen(f"{db_url}/api/db/user/alice/key1") as f:
    data = json.loads(f.read())

# Set value
req = urllib.request.Request(
    f"{db_url}/api/db/user/alice/key1",
    data=json.dumps({"value": {"name": "Alice", "count": 42}}).encode(),
    headers={"Content-Type": "application/json"},
    method="PUT"
)
urllib.request.urlopen(req)
```

## WebSocket Integration

### Database Request Message

```json
{
  "type": "db_request",
  "request_id": "req-123",
  "operation": "get",
  "namespace": "user/alice",
  "key": "key1"
}
```

### Database Response Message

```json
{
  "type": "db_response",
  "request_id": "req-123",
  "value": {"name": "Alice"},
  "exists": true
}
```

## Security Considerations

1. **Namespace Isolation**
   - Users can only access their own namespace
   - Job namespaces are temporary
   - Global namespace requires admin privileges

2. **Authentication**
   - Workers authenticated via WebSocket
   - Job context includes user ID
   - Validate namespace matches user

3. **Rate Limiting**
   - Limit requests per worker
   - Prevent abuse
   - Timeout long-running queries

4. **Data Validation**
   - Validate key formats
   - Size limits on values
   - Prevent path traversal

## Implementation

### File Structure

```
internal/db/
  ├── store.go          # BadgerDB wrapper
  ├── store_test.go     # Tests
  ├── namespace.go      # Namespace management
  └── handlers.go       # HTTP handlers
```

### Database Store

```go
type Store struct {
    db *badger.DB
}

func NewStore(dataDir string) (*Store, error) {
    opts := badger.DefaultOptions(dataDir)
    db, err := badger.Open(opts)
    if err != nil {
        return nil, err
    }
    return &Store{db: db}, nil
}

func (s *Store) Get(namespace, key string) ([]byte, error) {
    fullKey := namespace + key
    var value []byte
    err := s.db.View(func(txn *badger.Txn) error {
        item, err := txn.Get([]byte(fullKey))
        if err != nil {
            return err
        }
        return item.Value(func(val []byte) error {
            value = append([]byte{}, val...)
            return nil
        })
    })
    return value, err
}

func (s *Store) Set(namespace, key string, value []byte) error {
    fullKey := namespace + key
    return s.db.Update(func(txn *badger.Txn) error {
        return txn.Set([]byte(fullKey), value)
    })
}
```

## Configuration

```go
type DBConfig struct {
    DataDir      string        // Database directory
    MaxValueSize int64         // Max value size (default: 10MB)
    TTL          time.Duration // Default TTL (optional)
}
```

## Performance Considerations

1. **Caching**
   - BadgerDB has built-in caching
   - Consider additional in-memory cache for hot keys

2. **Batch Operations**
   - Use transactions for multiple operations
   - Batch API for efficiency

3. **Compaction**
   - BadgerDB handles compaction automatically
   - Monitor disk usage

4. **Backup**
   - Regular backups to external storage
   - Snapshot API for consistent backups

## Use Cases

1. **Session Storage**
   - Store user sessions
   - TTL-based expiration

2. **Caching**
   - Cache computation results
   - Reduce redundant work

3. **State Management**
   - Maintain state across job executions
   - Shared state between jobs

4. **Configuration Storage**
   - Store application configs
   - Feature flags

5. **Metrics/Analytics**
   - Store metrics
   - Aggregate data

## Future Enhancements

- **Indexes**: Secondary indexes for querying
- **Collections**: Logical grouping of keys
- **Replication**: Sync between orchestrators
- **Query Language**: SQL-like queries
- **Streaming**: Real-time updates via WebSocket


