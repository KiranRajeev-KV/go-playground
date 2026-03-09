# Distributed Clock Implementation

A demonstration of Lamport and Vector clocks in a distributed master-worker system.

## Building

```bash
go build -o clock-demo .
```

## Usage

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-mode` | `master` or `worker` | (required) |
| `-clock` | `lamport` or `vector` | `lamport` |
| `-port` | Port number | `8080` |
| `-id` | Worker ID (for vector clock) | `1` |
| `-nodes` | Total nodes (for vector clock) | `2` |

### Running

**Terminal 1 - Start Worker:**
```bash
go run . -mode=worker -port=8080 -id=1 -clock=lamport
```

**Terminal 2 - Start Master:**
```bash
go run . -mode=master -port=8080 -clock=lamport
```

For vector clock:
```bash
# Terminal 1
go run . -mode=worker -port=8080 -id=1 -nodes=2 -clock=vector

# Terminal 2
go run . -mode=master -port=8080 -nodes=2 -clock=vector
```

## How It Works

### Lamport Clock

A logical clock that provides a total ordering of events. Each process maintains a counter that increments on each internal event and max(received, local) + 1 on message receipt.

### Vector Clock

A logical clock that provides partial ordering of events. Each process maintains a vector of counters, one per node. On message receipt, each element is max(sent[i], received[i]).

## Example Output

**Lamport:**
```
Worker time: 2 | result: 3
Master time: 4 | Response: Result: 3
Worker time: 6 | result: 30
Master time: 8 | Response: Result: 30
```

**Vector:**
```
Worker time: [1 2]
Master time: [2 1] | Response: Result: 3
Master time: [4 2] | Response: Result: 30
```
