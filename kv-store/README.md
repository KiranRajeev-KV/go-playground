# kvstore

`kvstore` is a small in-memory, generic key-value store built as a Go learning project.

## Requirements

The module declares Go 1.26.4. Check the installed toolchain with:

```text
go version
```

## Usage

The store accepts any comparable key type and any value type.

```go
store := kvstore.New[string, int]()
store.Set("visits", 1)

value, err := store.Get("visits")
```

The zero value is also ready to use:

```go
var store kvstore.Store[string, int]
store.Set("visits", 1)
```

## Operations and contracts

- `Get` returns `ErrValueNotFound` for a missing key.
- `Set` inserts or replaces a value.
- `Delete` removes a key and returns `ErrValueNotFound` when it is absent.
- `LoadOrStore` atomically returns an existing value or stores the supplied value.
- `Update` atomically replaces an existing value through a callback. A nil callback returns `ErrNilUpdateFunc`; callbacks run under the store's exclusive lock and must not re-enter that store or wait for work that needs it.
- `Snapshot` returns a new map containing a shallow copy of the current entries.
- `Len` returns a concurrency-safe, point-in-time entry count.
- `Clear` atomically removes every entry.

`Store` synchronizes access to its internal map. It does not deep-copy values or synchronize mutable data referenced by values; callers own that synchronization. Do not copy a `Store` after first use.

## Development

Run the validation suite from the repository root:

```text
gofmt -l .
go test ./...
go test -race ./...
go vet ./...
```

Run benchmarks with allocation reporting:

```text
go test -run='^$' -bench=. -benchmem -count=5
```

## Repository structure

- `store.go` — reusable `kvstore` package
- `store_test.go` — external-package tests
- `store_benchmark_test.go` — sequential, parallel, and snapshot benchmarks
- `LEARNING_LOG.md` — learning-task history and active task
- `AGENTS.md` — repository workflow guide
