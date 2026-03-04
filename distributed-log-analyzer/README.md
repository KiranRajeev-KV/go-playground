# DLA (Distributed Log Analyzer)

A distributed log analysis system in Go using MapReduce-style distributed algorithm with real-time dashboard.

<img width="1880" height="1061" alt="image" src="https://github.com/user-attachments/assets/8c7a0c17-c970-41ba-b472-4c3ad7f54b24" />



## Architecture

```
┌─────────────┐      ┌─────────────┐
│   Master    │      │   Server    │
│  (Coord)    │────▶│  (HTTP/SSE) │
└─────────────┘      └─────────────┘
       │            
       │ Polls every 2s    
       ▼                  
┌───────────────────────────────────────┐
│              Workers                  │
│  ┌──────────┐ ┌──────────┐ ┌────────┐ │
│  │Worker 1  │ │Worker 2  │ │Worker3 │ │
│  │:9091     │ │:9092     │ │:9093   │ │
│  └──────────┘ └──────────┘ └────────┘ │
└───────────────────────────────────────┘
```

## Log Format

Each log entry contains:

```
[timestamp] [worker_id] [request_id] [component] [client_ip] [http_method] [endpoint] [status_code] [response_time_ms] [latency_ms] [response_size] [message]
```

Example:
```
2026-03-04T15:22:11Z worker2 req1a2b3.168.0.15 GET /login 200 c auth 192152ms 148ms 2048 User login success
```

Fields:
- `timestamp`: ISO 8601 format (UTC)
- `worker_id`: unique worker identifier
- `request_id`: unique correlation ID
- `component`: service/module name (auth, api, gateway, payment, user, notification, search)
- `client_ip`: simulated client IP
- `http_method`: GET/POST/PUT/DELETE
- `endpoint`: API path
- `status_code`: HTTP response (200, 201, 204, 301, 400, 401, 403, 404, 500, 503)
- `response_time_ms`: total response time
- `latency_ms`: latency before response
- `response_size`: size in bytes
- `message`: description

## MapReduce Workflow

1. **Map Phase**: Each worker continuously generates log entries and stores them in a buffered channel. When the master requests, workers dequeue a batch and run the Map function to extract:
   - Log level counts (INFO/WARN/ERROR)
   - Endpoint counts
   - Status code counts
   - Latency buckets

2. **Reduce Phase**: Master periodically polls all workers every 2 seconds, collects intermediate results, and aggregates them into global metrics.

3. **SSE**: The server streams aggregated metrics to connected clients via Server-Sent Events.

## API Specification

### Worker Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/map` | Execute Map function on batch of logs |
| GET | `/health` | Worker health check |

### Master/Server Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Serve dashboard.html |
| GET | `/events` | SSE endpoint for real-time metrics |
| GET | `/metrics` | Current aggregated metrics (JSON) |
| GET | `/health` | Server health check |

## Running Locally

### Prerequisites

- Go 1.21+

### Build

```bash
go build -o distributed-log-analyzer .
```

### Run All Components

```bash
./distributed-log-analyzer -mode=all
```

This starts:
- Master coordinator
- HTTP/SSE server on port 9090
- 3 workers on ports 9091, 9092, 9093

### Run Components Separately

Start workers:
```bash
./distributed-log-analyzer -mode=start-workers
```

Start master/server:
```bash
./distributed-log-analyzer -mode=master -master-port=9090
```

Start individual worker:
```bash
./distributed-log-analyzer -mode=worker -worker-id=worker1 -worker-port=9091
```

## Viewing Dashboard

Open your browser and navigate to:

```
http://localhost:9090
```

### Dashboard Features

**Main Stats (Cumulative - since start):**
- Total Requests
- Avg Latency (overall average)
- Error Rate (overall percentage)

**2-Second Window Stats:**
- Requests in last 2 seconds
- Avg Latency (last 2 seconds)
- Error Rate (last 2 seconds)

**Charts:**
- Log Level Distribution (donut chart - INFO/WARN/ERROR)
- Latency Over Time (line chart)

**Live Logs:**
- Real-time log entries with color-coded levels
- Full log format display

## Metrics

### Cumulative (since start)
- **Total Requests**: All requests processed
- **Avg Latency**: Overall average response time
- **Error Rate**: Overall percentage of 5xx errors
- **Log Level Counts**: Total INFO, WARN, ERROR counts
- **Top Endpoints**: Most frequently accessed endpoints

### Window (last 2 seconds)
- **Requests**: Number of requests in the window
- **Avg Latency**: Average response time in the window
- **Error Rate**: Error percentage in the window

## Implementation Details

- **Log Generation**: Goroutine generates realistic logs every 100ms
- **Queue**: Buffered channel (capacity 1000) per worker
- **Polling Interval**: Master polls workers every 2 seconds
- **Communication**: REST JSON over HTTP
- **Real-time Updates**: Server-Sent Events (SSE)
- **Frontend**: HTML + Chart.js + Tailwind CSS

## Project Structure

```
/distributed-log-analyzer
├── master.go          # Master coordinator
├── worker.go         # Worker logic (generator + map RPC)
├── server.go         # HTTP/SSE server
├── common.go        # Shared types and functions
├── main.go          # Entry point
├── dashboard.html   # Frontend dashboard
├── README.md        # This file
└── go.mod           # Go module
```
