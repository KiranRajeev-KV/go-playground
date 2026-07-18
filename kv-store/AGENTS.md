# Repository Guide

## Purpose and layout

This is a learning project for an in-memory, generic Go key-value store. The reusable `kvstore` package is implemented in `store.go`; external-package tests live in `store_test.go`; benchmarks live in `store_benchmark_test.go`. `LEARNING_LOG.md` is the chronological task record.

## Go policy

`go.mod` declares Go 1.26.4. Verify the installed toolchain with `go version` and treat it as the source of truth when running commands.

## Ownership

The learner implements every semantic Go change: production behavior, APIs, data structures, locking, errors, tests, benchmarks, fuzzing, dependencies, and bug fixes. Codex must not implement or supply pasteable solutions for those changes.

Codex maintains repository guidance and routine documentation: `AGENTS.md`, `README.md`, `LEARNING_LOG.md`, relevant Markdown files, Go comments and doc comments, and standard formatting-only changes. Before editing a Go file for comments, verify the change affects only comments or formatting.

Only one learner implementation task may be active. Do not assign documentation, comment, or formatting maintenance as a learner task.

## Engineering conventions

Prefer idiomatic Go and official Go documentation as the primary source. Preserve the store's documented contracts: zero-value usability, no copying after first use, map-level synchronization only, shallow value ownership, inspectable sentinel errors, and point-in-time operation semantics.

## Verification

Run the checks relevant to the active task. The normal baseline is:

```text
git diff --check
gofmt -l .
go test ./...
go test -race ./...
go vet ./...
```

For benchmark work, also run:

```text
go test -run='^$' -bench=. -benchmem -count=5
```

## Completion and handoff

When a learner task is complete, inspect the diff, run verification, apply directly related documentation or formatting maintenance, rerun verification, and record the work in `LEARNING_LOG.md`. Update `README.md` for externally visible behavior. Provide exact, path-scoped Conventional Commit commands for the learner to run; never commit, reset, clean, or discard work yourself. Keep unrelated or pre-existing changes out of commit recommendations.
