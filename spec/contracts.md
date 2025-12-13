# API Contracts

**Version:** 0.2.0-alpha  
**Last Updated:** 2024-12

This document defines all API contracts for the Zerverless Platform.

---

## Table of Contents

1. [HTTP API](#1-http-api)
2. [WebSocket Protocol](#2-websocket-protocol)
3. [Peer-to-Peer Protocol](#3-peer-to-peer-protocol)

---

## 1. HTTP API

### 1.1 Endpoints Overview

#### Health & Info

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Health check |
| GET | `/info` | Node info (id, version, uptime) |
| GET | `/stats` | Statistics (volunteers, jobs, peers) |

#### Jobs

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/jobs` | Submit a new job |
| GET | `/api/jobs/{job_id}` | Get job status and result |
| GET | `/api/jobs` | List jobs (with pagination) |
| DELETE | `/api/jobs/{job_id}` | Cancel a job |

#### Admin

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/peers` | List connected peers |
| POST | `/api/peers` | Manually connect to peer |
| GET | `/api/volunteers` | List connected volunteers |

---

### 1.2 Endpoint Details

#### GET /health

Health check endpoint.

**Response (200 OK):**

```json
{
    "status": "healthy"
}
```

---

#### GET /info

Returns node information.

**Response (200 OK):**

```json
{
    "node_id": "12D3KooWxxxxxx",
    "version": "0.2.0",
    "uptime_seconds": 3600
}
```

---

#### GET /stats

Returns platform statistics.

**Response (200 OK):**

```json
{
    "node_id": "12D3KooW...",
    "uptime_seconds": 3600,
    "volunteers": {
        "connected": 5,
        "idle": 3,
        "busy": 2
    },
    "jobs": {
        "pending": 10,
        "running": 2,
        "completed_total": 150,
        "failed_total": 3
    },
    "peers": {
        "connected": 2
    }
}
```

---

#### POST /api/jobs

Submit a new job for execution.

**Request:**

```json
{
    "wasm_cid": "QmXxx...",
    "input_data": {
        "x": 6,
        "y": 7
    },
    "timeout_seconds": 30
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `wasm_cid` | string | Yes | IPFS CID of the Wasm module |
| `input_data` | object | No | Input data passed to Wasm |
| `timeout_seconds` | int | No | Execution timeout (default: 30) |

**Response (201 Created):**

```json
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "result": null,
    "error": null,
    "created_at": "2024-12-01T10:00:00Z",
    "completed_at": null
}
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 400 | Invalid request body |
| 503 | Queue full |

---

#### GET /api/jobs/{job_id}

Get job status and result.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `job_id` | string | UUID of the job |

**Response (200 OK) - Pending:**

```json
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending",
    "result": null,
    "error": null,
    "created_at": "2024-12-01T10:00:00Z",
    "completed_at": null
}
```

**Response (200 OK) - Completed:**

```json
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "completed",
    "result": 42,
    "error": null,
    "created_at": "2024-12-01T10:00:00Z",
    "completed_at": "2024-12-01T10:00:05Z"
}
```

**Response (200 OK) - Failed:**

```json
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "failed",
    "result": null,
    "error": "Wasm execution error: out of memory",
    "created_at": "2024-12-01T10:00:00Z",
    "completed_at": "2024-12-01T10:00:03Z"
}
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 404 | Job not found |

---

#### GET /api/jobs

List jobs with pagination.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 20 | Number of jobs to return |
| `offset` | int | 0 | Offset for pagination |
| `status` | string | - | Filter by status |

**Response (200 OK):**

```json
{
    "jobs": [
        {
            "id": "550e8400-e29b-41d4-a716-446655440000",
            "status": "completed",
            "created_at": "2024-12-01T10:00:00Z"
        }
    ],
    "total": 150,
    "limit": 20,
    "offset": 0
}
```

---

#### DELETE /api/jobs/{job_id}

Cancel a pending or running job.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `job_id` | string | UUID of the job |

**Response (204 No Content)**

**Error Responses:**

| Status | Description |
|--------|-------------|
| 404 | Job not found |
| 409 | Job already completed |

---

#### GET /api/peers

List connected peers.

**Response (200 OK):**

```json
{
    "peers": [
        {
            "id": "12D3KooWxxxxx",
            "addrs": ["/ip4/192.168.1.1/tcp/4001"],
            "status": "connected",
            "volunteer_count": 5,
            "load": 0.3
        }
    ]
}
```

---

#### GET /api/volunteers

List connected volunteers.

**Response (200 OK):**

```json
{
    "volunteers": [
        {
            "id": "550e8400-e29b-41d4-a716-446655440000",
            "status": "idle",
            "jobs_completed": 42,
            "connected_at": "2024-12-01T09:00:00Z"
        }
    ],
    "total": 5
}
```

---

## 2. WebSocket Protocol

### 2.1 Volunteer Connection

**Endpoint:** `ws://host:port/ws/volunteer`

All messages are JSON with a `type` field.

---

### 2.2 Message Types

#### Volunteer → Orchestrator

| Type | Description | Payload |
|------|-------------|---------|
| `ready` | Volunteer is ready for work | `{ capabilities }` |
| `result` | Job completed successfully | `{ job_id, result }` |
| `error` | Job failed | `{ job_id, error }` |
| `heartbeat` | Keep-alive ping | `{}` |

#### Orchestrator → Volunteer

| Type | Description | Payload |
|------|-------------|---------|
| `ack` | Connection acknowledged | `{ volunteer_id }` |
| `job` | New job to execute | `{ job_id, wasm_cid, input_data, timeout }` |
| `cancel` | Cancel current job | `{ job_id }` |
| `heartbeat` | Keep-alive pong | `{}` |

---

### 2.3 Message Schemas

#### `ready` (Volunteer → Orchestrator)

Sent after connection to indicate volunteer is ready.

```json
{
    "type": "ready",
    "capabilities": {
        "wasm": true,
        "max_memory_mb": 128
    }
}
```

---

#### `ack` (Orchestrator → Volunteer)

Sent immediately upon WebSocket connection.

```json
{
    "type": "ack",
    "volunteer_id": "550e8400-e29b-41d4-a716-446655440000",
    "message": "Welcome!"
}
```

---

#### `job` (Orchestrator → Volunteer)

Dispatches a job to the volunteer.

```json
{
    "type": "job",
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "wasm_cid": "QmXxx...",
    "wasm_url": "http://localhost:8080/ipfs/QmXxx...",
    "input_data": {
        "x": 6,
        "y": 7
    },
    "timeout_seconds": 30
}
```

---

#### `result` (Volunteer → Orchestrator)

Sent when job completes successfully.

```json
{
    "type": "result",
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "result": 42
}
```

---

#### `error` (Volunteer → Orchestrator)

Sent when job fails.

```json
{
    "type": "error",
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "error": "Wasm execution failed: out of memory"
}
```

---

#### `cancel` (Orchestrator → Volunteer)

Requests cancellation of current job.

```json
{
    "type": "cancel",
    "job_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

---

#### `heartbeat` (Both Directions)

Keep-alive message.

```json
{
    "type": "heartbeat"
}
```

---

### 2.4 Connection Flow

```
Volunteer                          Orchestrator
    │                                   │
    │──────── Connect WebSocket ───────►│
    │                                   │
    │◄─────────── ack ─────────────────│
    │         {volunteer_id}            │
    │                                   │
    │─────────── ready ────────────────►│
    │       {capabilities}              │
    │                                   │
    │           ... wait ...            │
    │                                   │
    │◄──────────── job ────────────────│
    │    {job_id, wasm_cid, input}      │
    │                                   │
    │         ... execute ...           │
    │                                   │
    │──────────── result ──────────────►│
    │       {job_id, result}            │
    │                                   │
    │─────────── ready ────────────────►│
    │         (ready for next)          │
    │                                   │
```

---

### 2.5 Error Handling

| Scenario | Behavior |
|----------|----------|
| Connection lost | Volunteer marked disconnected, job reassigned |
| Heartbeat timeout (60s) | Connection terminated |
| Invalid message | Connection terminated |
| Job timeout | `cancel` sent, job reassigned |

---

## 3. Peer-to-Peer Protocol

### 3.1 Protocol IDs

| Protocol | ID | Description |
|----------|-----|-------------|
| Job Forward | `/zerverless/job/1.0.0` | Forward jobs to peer |
| Stats Sync | `/zerverless/stats/1.0.0` | Share load information |

---

### 3.2 Job Forward Protocol

Used to forward jobs to peers with available volunteers.

**Request:**

```json
{
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "wasm_cid": "QmXxx...",
    "input_data": {"x": 6, "y": 7},
    "timeout_seconds": 30,
    "origin_peer": "12D3KooWxxxxx"
}
```

**Response:**

```json
{
    "accepted": true,
    "job_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

---

### 3.3 Stats Sync Protocol

Periodic exchange of load information.

**Message:**

```json
{
    "peer_id": "12D3KooWxxxxx",
    "volunteer_count": 5,
    "idle_count": 3,
    "pending_jobs": 10,
    "load": 0.4
}
```

---

### 3.4 Peer Discovery

Handled automatically by libp2p's Kademlia DHT. Nodes:

1. Bootstrap to known peers
2. Participate in DHT
3. Discover new peers automatically
4. Exchange stats periodically

---

## Go Type Definitions

### Job Types

```go
// Request/Response types for HTTP API

type JobCreateRequest struct {
    WasmCID        string         `json:"wasm_cid"`
    InputData      map[string]any `json:"input_data"`
    TimeoutSeconds int            `json:"timeout_seconds"`
}

type JobResponse struct {
    ID          string     `json:"id"`
    Status      string     `json:"status"`
    Result      any        `json:"result,omitempty"`
    Error       string     `json:"error,omitempty"`
    CreatedAt   time.Time  `json:"created_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type JobListResponse struct {
    Jobs   []JobResponse `json:"jobs"`
    Total  int           `json:"total"`
    Limit  int           `json:"limit"`
    Offset int           `json:"offset"`
}
```

### WebSocket Message Types

```go
// Volunteer sends
type WSReadyMessage struct {
    Type         string        `json:"type"` // "ready"
    Capabilities *Capabilities `json:"capabilities,omitempty"`
}

type WSResultMessage struct {
    Type   string `json:"type"` // "result"
    JobID  string `json:"job_id"`
    Result any    `json:"result"`
}

type WSErrorMessage struct {
    Type  string `json:"type"` // "error"
    JobID string `json:"job_id"`
    Error string `json:"error"`
}

// Orchestrator sends
type WSAckMessage struct {
    Type        string `json:"type"` // "ack"
    VolunteerID string `json:"volunteer_id"`
    Message     string `json:"message"`
}

type WSJobMessage struct {
    Type           string         `json:"type"` // "job"
    JobID          string         `json:"job_id"`
    WasmCID        string         `json:"wasm_cid"`
    WasmURL        string         `json:"wasm_url"`
    InputData      map[string]any `json:"input_data"`
    TimeoutSeconds int            `json:"timeout_seconds"`
}

type WSCancelMessage struct {
    Type  string `json:"type"` // "cancel"
    JobID string `json:"job_id"`
}

type WSHeartbeatMessage struct {
    Type string `json:"type"` // "heartbeat"
}
```

### Stats Types

```go
type StatsResponse struct {
    NodeID        string         `json:"node_id"`
    UptimeSeconds int64          `json:"uptime_seconds"`
    Volunteers    VolunteerStats `json:"volunteers"`
    Jobs          JobStats       `json:"jobs"`
    Peers         PeerStats      `json:"peers"`
}

type VolunteerStats struct {
    Connected int `json:"connected"`
    Idle      int `json:"idle"`
    Busy      int `json:"busy"`
}

type JobStats struct {
    Pending        int `json:"pending"`
    Running        int `json:"running"`
    CompletedTotal int `json:"completed_total"`
    FailedTotal    int `json:"failed_total"`
}

type PeerStats struct {
    Connected int `json:"connected"`
}
```

---

_This contract is versioned with the main specification._

