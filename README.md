# Go Playground

A collection of standalone Go experiments and projects in a single repository.

## Why?

Having separate GitHub repos for every small experiment gets messy. This repo serves as a sandbox for trying out different Go concepts, patterns, and projects - all in one place.

Each subdirectory is an independent project with its own `go.mod`.

## Projects

- [distributed-log-analyzer](./distributed-log-analyzer/README.md) - Distributed log analysis with MapReduce and real-time dashboard
- [distributed-transactions](./distributed-transactions/README.md) - Distributed transaction system comparing 2PC and 3PC protocols with chaos engineering
- [lamport-vector-clocks](./lamport-vector-clocks/README.md) - Lamport and Vector clock implementations

## Adding New Projects

1. Create a new directory
2. Run `go mod init <project-name>` inside it
3. Build and run independently
