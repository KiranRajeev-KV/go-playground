# Distributed Transaction System: 2PC vs 3PC

A Go-based distributed transaction system for evaluating and comparing Two-Phase Commit (2PC) and Three-Phase Commit (3PC) protocols with built-in chaos engineering capabilities.

## Overview

This project implements a realistic e-commerce order fulfillment scenario with three services:

- **Coordinator** (Order Service) - Orchestrates the distributed transaction
- **Inventory Service** - Manages product inventory
- **Payment Service** - Processes payments

### Key Features

- **Protocol Switching**: Run with either 2PC or 3PC via `-protocol` flag
- **Chaos Engineering**: Inject failures (drop, delay, phantom) at any protocol phase
- **Crash Recovery**: WAL-based persistence with automatic recovery on restart
- **Idempotency**: Prevents double-charges from network retries
- **Load Testing**: Built-in benchmark client with metrics

## Quick Start

```bash
# Build
just build

# Run all services (2PC by default)
just run-all

# Run benchmark
./dist-tx -mode=client -tps=50 -duration=30s -warmup=5s

# Or run with 3PC
./dist-tx -mode=coordinator -db=coordinator.db -protocol=3pc &
./dist-tx -mode=inventory -db=inventory.db -seed &
./dist-tx -mode=payment -db=payment.db -seed &
```

## Architecture
<img width="1440" height="1736" alt="image" src="https://github.com/user-attachments/assets/46a49d34-7d31-4b22-a7fd-c1921da44930" />


## Protocols

### Two-Phase Commit (2PC)

<img width="1440" height="1398" alt="image" src="https://github.com/user-attachments/assets/9d6ea13d-d70a-4bce-bcc1-3b778e8ae7e5" />

### Three-Phase Commit (3PC)

<img width="1440" height="1652" alt="image" src="https://github.com/user-attachments/assets/0da1664d-4ef2-4e75-98e3-5bbb8cd81bf8" />

**3PC Advantage**: Once a participant reaches `PreCommit`, it will commit even if `DoCommit` is never received (amnesia-free recovery).

## Command-Line Flags

### Service Modes

| Flag | Description | Default |
|------|-------------|---------|
| `-mode` | Service mode: `coordinator`, `inventory`, `payment`, `client` | (required) |

### Common Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-db` | Path to SQLite database | `data.db` |
| `-port` | Port to listen on | Mode-specific |
| `-seed` | Seed database with sample data | `false` |
| `-recover` | Run crash recovery on startup | `false` |
| `-protocol` | Protocol: `2pc` or `3pc` | `2pc` |
| `-chaos` | Chaos simulation config | (none) |

### Client Mode Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-tps` | Target transactions per second | `10` |
| `-duration` | Test duration | `30s` |
| `-warmup` | Warmup period | `5s` |
| `-coordinator-addr` | Coordinator address | `localhost:50051` |

## Chaos Engineering

Chaos injection allows testing failure scenarios by modifying gRPC metadata sent from coordinator to participants.

### Chaos Format

```
<target>-<action>-<phase>[:probability]
```

| Component | Options |
|-----------|---------|
| `target` | `inventory`, `payment`, `all` |
| `action` | `drop`, `delay`, `phantom`, `log` |
| `phase` | `prepare`, `commit`, `abort` (2PC) or `cancommit`, `precommit`, `docommit` (3PC) |
| `probability` | 1-100 (optional, default: 100) |

### Chaos Actions

| Action | Behavior |
|--------|----------|
| `drop` | Silently drop request, return unavailable error |
| `delay` | Add configured delay (default: 15s) before processing |
| `phantom` | Process request but drop response |
| `log` | Log the chaos instruction, process normally |

### Phase Aliases

For convenience, phases can be referenced by aliases:

| Phase | Aliases |
|-------|---------|
| `prepare` | `prepare`, `voting` |
| `commit` | `commit`, `commit-phase` |
| `cancommit` | `cancheck`, `cancommit`, `can-commit` |
| `precommit` | `precommit`, `pre-commit` |
| `docommit` | `docommit`, `do-commit`, `commit` |

### Examples

```bash
# Drop 20% of Prepare requests to inventory
-chaos="inventory-drop-prepare:20"

# Delay all commit phases on payment
-chaos="payment-delay-commit"

# Phantom failures on all participants at cancommit (3PC)
-chaos="all-phantom-cancommit:30"

# Log (but don't inject) all aborts
-chaos="all-log-abort"
```

## Benchmarking Results

Tests run with: `TPS=50, Duration=10s, Warmup=2s`

### 2PC Results

| Test | Committed | Aborted | Errors | Actual TPS | Chaos Stats (D/Dly/P) |
|------|-----------|---------|--------|------------|----------------------|
| **No Chaos** | 522 (100%) | 0 (0%) | 0 | 52.20 | 0/0/0 |
| **DROP 10%** | 478 (91.4%) | 44 (8.4%) | 1 (0.2%) | 52.30 | 44/0/0 |
| **DELAY 10%** | 461 (99.4%) | 0 (0%) | 3 (0.6%) | 46.40 | 0/61/0 |
| **PHANTOM 10%** | 471 (90.1%) | 52 (9.9%) | 0 | 52.30 | 0/0/52 |

### 3PC Results (cancommit phase)

| Test | Committed | Aborted | Errors | Actual TPS | Chaos Stats (D/Dly/P) |
|------|-----------|---------|--------|------------|----------------------|
| **No Chaos** | 523 (100%) | 0 (0%) | 0 | 52.30 | 0/0/0 |
| **DROP 10%** | 472 (90.2%) | 51 (9.8%) | 0 | 52.30 | 51/0/0 |
| **DELAY 10%** | 462 (99.6%) | 0 (0%) | 2 (0.4%) | 46.40 | 0/60/0 |
| **PHANTOM 10%** | 466 (89.1%) | 57 (10.9%) | 0 | 52.30 | 0/0/57 |

### 2PC vs 3PC Comparison (DROP 10%)

| Protocol | Chaos Phase | Committed | Aborted | Chaos Drops |
|----------|-------------|-----------|---------|-------------|
| **2PC** | prepare | 478 (91.4%) | 44 (8.4%) | 44 |
| **3PC** | cancommit | 472 (90.2%) | 51 (9.8%) | 51 |

## Key Inferences

### 1. DROP/PHANTOM Behavior
- Both actions cause aborts proportional to the chaos probability (~10%)
- DROP is more efficient (silently drops) while PHANTOM processes before dropping
- No significant TPS difference when drops don't cause cascading issues

### 2. DELAY Impact
- DELAY causes **timeout-based failures** rather than immediate aborts
- Visible in the "In-Flight" counter building up during test
- Results in **reduced throughput** (46.40 vs 52.30 TPS) as requests pile up
- Most delayed requests eventually complete if they finish before client timeout

### 3. 2PC vs 3PC at First Phase
- **Similar failure rates** at the first phase (~10% abort rate with 10% chaos)
- Both protocols are equally vulnerable to failures before commit
- 3PC's advantage is in **later phases**, not the first phase

### 4. 3PC's Resilience Advantage
```
3PC DoCommit Drop Scenario:
1. Coordinator sends DoCommit to participants
2. Network drops the DoCommit message
3. Coordinator returns "COMMITTED" to client
4. Participant (having received PreCommit) eventually commits anyway

Result: Client gets committed, participant commits - NO DATA LOSS
```

### 5. Protocol Selection Guidance

| Scenario | Recommended Protocol |
|----------|---------------------|
| Low latency requirement | **2PC** (fewer round trips) |
| High failure rate network | **3PC** (better recovery) |
| Coordinator crash risk | **3PC** (participants can self-recover) |
| Simple, well-connected systems | **2PC** (simpler, less overhead) |

## Justfile Commands

```bash
just build           # Build the binary
just proto           # Generate gRPC stubs
just run-coordinator # Run coordinator only
just run-inventory  # Run inventory only
just run-payment     # Run payment only
just run-participants # Run inventory + payment
just run-all         # Run all services
just run-client      # Run client (TPS=50, 60s)
just run-benchmark   # Run benchmark (TPS=100, 30s, warmup=5s)
just clean-data      # Remove all .db files
just stop-all        # Kill all processes
just test            # Run tests
just lint            # Lint code
just fmt             # Format code
```

## Project Structure

```
distributed-transactions/
├── main.go                    # Entry point with mode routing
├── Justfile                   # Task runner
├── proto/
│   └── transaction.proto      # gRPC service definitions
├── pb/                        # Generated Go code
├── internal/
│   ├── app/
│   │   └── client.go          # Load testing client
│   ├── coordinator/
│   │   ├── service.go         # gRPC server implementation
│   │   └── state.go           # 2PC/3PC state machines
│   ├── participant/
│   │   └── handlers.go        # Participant RPC handlers
│   ├── db/
│   │   └── schema.go          # SQLite schema & queries
│   └── middleware/
│       ├── chaos.go           # Chaos injection interceptor
│       └── idempotency.go    # Idempotency middleware
└── README.md
```

## Technical Highlights

- **WAL Mode**: SQLite in WAL mode for better concurrency
- **Dedicated WAL Pattern**: Business state and transaction log separated
- **Fan-Out/Fan-In**: Parallel requests to participants with errgroup
- **Atomic Operations**: Sync-free coordination with atomic counters
- **Embedded SQLite**: No external database dependencies
