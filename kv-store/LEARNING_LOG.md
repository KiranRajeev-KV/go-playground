# KV Store Learning Log

## Environment

- Go version: go1.26.5-X:nodwarf5 linux/amd64 (installed); module declares Go 1.26.4.
- Module: `kvstore` (`go.mod`).
- Last verified commit or working-tree state: Git root is `/home/kr/dev/go-playground` at `31326d6`; this `kv-store` directory is reported as untracked (`?? ./`).

## Project Rules

- One active task at a time.
- The learner implements every change.
- The learner owns semantic Go code, APIs, tests, benchmarks, concurrency, error behavior, dependencies, and bug fixes.
- Codex owns Markdown documentation, Go comments and doc comments, and standard formatting-only maintenance.
- Codex never implements semantic Go behavior or tests for the learner.
- Idiomatic Go is preferred.
- Verification is run by Codex.

## Workflow Policy Changes

- 2026-07-18: Ownership model revised. Codex now maintains `AGENTS.md`, `README.md`, `LEARNING_LOG.md`, relevant Markdown, Go comments, and formatting-only changes; the learner remains solely responsible for all semantic Go implementation and tests. Completed learner tasks receive path-scoped Conventional Commit handoff commands before the next task. This policy change is repository maintenance, not a learner task.
- Codex-maintained migration changes: created `AGENTS.md` with durable repository workflow, ownership, verification, and handoff rules; created `README.md` describing the verified public API and development commands; updated `LEARNING_LOG.md` project rules and this policy record. No executable Go syntax was changed.

## Imported History

Historically completed and present in the current implementation: one `sync.RWMutex` protects the map; goroutine coordination is outside the store and uses `sync.WaitGroup`; construction is pointer based; the missing-value sentinel is inspectable; `Set` has no return value; `Delete`, generic `LoadOrStore`, callback `Update`, and a shallow-copy `Snapshot` are implemented; the zero value is usable; the reusable package is tested externally; and sequential get/set, parallel get, and multi-size snapshot benchmarks exist. These were confirmed by source and test inspection in this session. The current session has also run the baseline tests, race detector, vet, formatting, and benchmark command recorded below.

## Completed Tasks

### Task 17: Validate benchmark measurement boundaries

- Objective: Ensure each benchmark measures only the KV-store operation it claims to measure.
- Problem addressed: The parallel-read benchmark measured store construction and initial population because it did not establish a timing boundary after setup.
- Contract or design decision: Setup belongs outside the measurement interval. `B.Loop` establishes that boundary automatically; a benchmark using `RunParallel` must do so outside the parallel worker body.
- Acceptance criteria: Sequential get/set and all three snapshot sizes retain their intended coverage; parallel get reads an initialized shared store with setup excluded; benchmark output reports allocations; baseline checks stay clean.
- Learner implementation summary: Added an outer timer reset after the shared store is constructed and seeded, immediately before `RunParallel`.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -run='^$' -bench=. -benchmem -count=5`.
- Verification results: On 2026-07-18 all baseline commands passed: formatting produced no file names; regular and race test runs passed 28 tests; vet reported no issues. The repeated benchmark command completed successfully for sequential get, sequential set, parallel get, and snapshots of 10, 1000, and 100000 entries, reporting allocation metrics for every group.
- Important lesson: A passing benchmark can still be misleading. Know where the `testing` package starts timing, and never manipulate a global benchmark timer from within `RunParallel` workers.
- References: https://pkg.go.dev/testing#B.Loop ; https://pkg.go.dev/testing#B.RunParallel
- Completed: 2026-07-18

### Task 18: Document the public API's concurrency contracts

- Objective: Make the package safe to use correctly without requiring callers to read its implementation.
- Problem addressed: The exported API omitted its no-copy rule, snapshot ownership semantics, and the lock held while `Update` runs caller code.
- Contract or design decision: Documentation describes current behavior without changing it. A `Store` must not be copied after first use; `Snapshot` returns an independent map but shallow-copies its keys and values; `Update` callbacks must not re-enter the same store or wait on work that needs it.
- Acceptance criteria: All exported declarations have accurate identifier-leading comments; the required concurrency and ownership constraints are explicit; behavior remains unchanged; baseline checks pass.
- Learner implementation summary: Added concise doc comments for the public error, type, constructor, and methods, including the explicit callback re-entry and waiting restriction.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`.
- Verification results: On 2026-07-18, `git diff --check` passed and formatting produced no file names. `go test ./...` and `go test -race ./...` passed; vet completed successfully with no reported issues. The sandboxed test process could not initially read a shared build-cache entry, but the same commands passed when granted shared-cache access; no code failure was observed.
- Important lesson: Locking behavior is part of an API contract whenever callers supply a callback. A short, explicit warning can prevent a subtle self-deadlock.
- References: https://go.dev/doc/comment ; https://pkg.go.dev/sync#RWMutex
- Completed: 2026-07-18

### Task 19: Define safe nil-callback behavior for Update

- Objective: Give `Update` a deterministic, non-panicking error contract when the caller supplies a nil callback.
- Problem addressed: A nil callback previously panicked only for existing keys, making an invalid-input failure depend on store state.
- Contract or design decision: A nil callback is invalid input and takes precedence over key lookup. `Update` returns the exported, inspectable `ErrNilUpdateFunc` for it, does not panic, and does not mutate the store.
- Acceptance criteria: Nil callbacks are identifiable with `errors.Is` for present and absent keys; existing values are preserved; ordinary updates and missing-key behavior are unchanged; external-package tests and baseline checks pass.
- Learner implementation summary: Added `ErrNilUpdateFunc`, validates the callback before taking the lock or reading the map, documents the precedence, and adds external tests for both key states and value preservation.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully with no reported issues. The test commands used shared build-cache access because the restricted sandbox cannot read an existing cache entry.
- Important lesson: Treat invalid public API inputs consistently. A sentinel error gives callers a branch they can test, while a state-dependent panic gives them a surprise.
- References: https://go.dev/ref/spec#Calls ; https://pkg.go.dev/errors#Is
- Completed: 2026-07-18

### Task 20: Clarify reference-value ownership

- Objective: Make the generic store's synchronization boundary clear when values contain pointers, maps, slices, or other mutable references.
- Problem addressed: “Concurrency-safe” could be read as protecting mutable objects referenced by values, although the mutex only protects the internal map and generic values use ordinary assignment.
- Contract or design decision: The store synchronizes its map only and never deep-copies values. Callers own synchronization of mutable data reachable through stored values.
- Acceptance criteria: Public docs distinguish map synchronization from value ownership, describe assignment behavior for `Set`, `Get`, and `LoadOrStore`, an external test demonstrates direct reference aliasing, and baseline checks pass.
- Learner implementation summary: Clarified ownership in the type and relevant method comments; added `TestReferenceValueAliasing` covering `Set`/`Get` and a newly stored `LoadOrStore` slice value.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully with no reported issues. The test commands used shared build-cache access because the restricted sandbox cannot read an existing cache entry.
- Important lesson: A mutex protects only the state it guards. Generic containers cannot silently make referenced objects safe or independent without a specified copying strategy.
- References: https://go.dev/ref/spec#Assignments ; https://go.dev/ref/mem
- Completed: 2026-07-18

### Task 21: Test Delete's concurrent atomicity

- Objective: Verify the claimed atomic delete contract under contention.
- Problem addressed: `Delete` used an exclusive lock but had only sequential tests, leaving its key concurrency claim unverified.
- Contract or design decision: Concurrent deletion of one present key yields exactly one successful deletion; every other completed call observes the absent-key error, with no specified winner.
- Acceptance criteria: An external test uses `sync.WaitGroup`, avoids timing and ordering assumptions, asserts one success and all other errors with `errors.Is`, verifies the final absence, and baseline checks pass.
- Learner implementation summary: Added `TestConcurrentDeleteSameKey`, which gathers one result per worker through a buffered channel after a `WaitGroup` completes and validates the permitted outcome set.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully with no reported issues. The test commands used shared build-cache access because the restricted sandbox cannot read an existing cache entry.
- Important lesson: Correct concurrent tests specify invariants across all valid schedules. They should count outcomes, not predict a winner.
- References: https://go.dev/ref/mem ; https://pkg.go.dev/sync#WaitGroup ; https://pkg.go.dev/errors#Is
- Completed: 2026-07-18

### Task 22: Verify zero-value LoadOrStore

- Objective: Complete the zero-value usability contract for every public map-writing operation.
- Problem addressed: `LoadOrStore` lazily initializes a nil map but had no direct zero-value test.
- Contract or design decision: A zero-value store has the same first-writer-wins `LoadOrStore` semantics as a store returned by `New`.
- Acceptance criteria: An external test uses a zero-value store, verifies the first result is newly stored and persisted, verifies a second call loads the original rather than overwriting it, and baseline checks pass.
- Learner implementation summary: Added `TestLoadOrStoreZeroValueStore`, covering both calls and the final `Get` value.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully with no reported issues. The test commands used shared build-cache access because the restricted sandbox cannot read an existing cache entry.
- Important lesson: Zero-value usability is a contract for every method, not merely a property inferred from one successful write.
- References: https://go.dev/doc/effective_go#allocation_new ; https://pkg.go.dev/sync#Map.LoadOrStore
- Completed: 2026-07-18

### Task 23: Document zero-value usability

- Objective: Make the allocation behavior of the public store type clear to callers.
- Problem addressed: The zero value was intentionally usable and tested, but callers could not learn that from the type documentation.
- Contract or design decision: A zero-value `Store` is ready to use; this does not alter the rule against copying a store after first use.
- Acceptance criteria: The type comment explicitly states zero-value usability, other public contracts remain accurate, no behavior changes, and baseline checks pass.
- Learner implementation summary: Added a focused zero-value readiness sentence to the `Store` comment.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully with no reported issues. The test commands used shared build-cache access because the restricted sandbox cannot read an existing cache entry.
- Important lesson: Zero-value usability is valuable only when callers can discover it in the API contract.
- References: https://go.dev/doc/effective_go#allocation_new
- Completed: 2026-07-18

### Task 24: Add an atomic Len operation

- Objective: Expose the number of entries in the store as a concurrency-safe read operation.
- Why it was selected: A size query is a natural bounded capability after establishing the store's ownership, error, zero-value, and concurrency contracts.
- Required behavior: `Len` returns the number of entries present at the time of its call for zero-value and constructed stores, without mutation or internal-state exposure.
- Constraints: `Len` is a point-in-time observation, not a transaction. It uses the existing synchronization strategy without fields, goroutines, dependencies, or a counter.
- Acceptance criteria: It returns zero for zero-value and new stores; grows for distinct inserts; is stable on overwrite; shrinks after deletion; has external tests for sequential and concurrent-mutation usage; and passes the baseline checks.
- Learner implementation summary: Added the read-locked `Len` method and external tests for empty, zero-value, insertion, overwrite, deletion, and concurrent reads alongside mutation.
- Codex-maintained changes: `store.go` gained the comment-only `Len` contract; `README.md` lists `Len` as a point-in-time, concurrency-safe operation; `go fmt ./...` made no reported formatting changes.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully with no reported issues. The concurrent test executes `Len` while distinct-key mutators call `Set` and `Delete`.
- Important lesson: A read lock makes one size observation safe, but the result is immediately eligible to become stale; concurrent tests should validate allowed ranges rather than scheduler-dependent exact values.
- References: https://pkg.go.dev/sync#RWMutex.RLock ; https://go.dev/ref/mem ; https://go.dev/doc/articles/race_detector
- Associated commit message recommendation: `feat(store): add atomic len`
- Completed: 2026-07-18

### Task 25: Add an atomic Clear operation

- Objective: Add a bounded bulk-mutation operation while preserving the store's locking and zero-value contracts.
- Why it was selected: With `Len` available, clearing all entries is a useful complementary operation that teaches atomic bulk mutation without adding data-structure complexity.
- Required behavior: `Clear` removes every current entry, succeeds for zero-value and empty stores without error, and is atomic with respect to other store operations.
- Constraints: It uses the existing exclusive-lock strategy, clears in place, adds no counters or goroutines, and preserves ownership semantics.
- Acceptance criteria: Populated stores become empty and former keys are missing; empty and zero-value stores remain usable; external tests exercise concurrent readers; existing contracts remain unchanged; and baseline checks pass.
- Learner implementation summary: Added the exclusive-lock `Clear` method and external tests for populated, empty, zero-value, and concurrent-reader behavior.
- Codex-maintained changes: `store.go` gained the comment-only atomic `Clear` contract; `README.md` lists `Clear`; `go fmt ./...` made no reported formatting changes.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully with no reported issues. The concurrent test observes only complete pre-clear or post-clear states from `Len` and `Snapshot`.
- Important lesson: A bulk operation is atomic when callers cannot observe a partial internal transition, not when it prevents later operations from changing the store.
- References: https://pkg.go.dev/builtin#clear ; https://pkg.go.dev/sync#RWMutex.Lock ; https://go.dev/ref/mem
- Associated commit message recommendation: `feat(store): add atomic clear`
- Completed: 2026-07-18

### Task 26: Benchmark Clear with excluded re-population setup

- Objective: Measure the cost of clearing populated stores without measuring the work needed to re-populate them.
- Why it was selected: `Clear` leaves the store empty, so its benchmark teaches careful per-iteration timer boundaries beyond one-time setup.
- Required behavior: Sequential `Clear` subtests cover populated sizes 10, 1000, and 100000; each timed iteration clears one populated store, and re-population/allocation setup remains outside the measured interval.
- Constraints: Store behavior remains unchanged. Setup is deterministic, sleeps are forbidden, and the benchmark does not measure `Set` or map allocation as part of `Clear`.
- Acceptance criteria: All three sizes are exact; every timed operation clears once; setup is excluded per iteration; allocations are reported; benchmark output and baseline checks complete.
- Learner implementation summary: Added `BenchmarkClear` subtests for the three sizes, with one-time setup before `B.Loop` and stopped-timer re-population between timed clears.
- Codex-maintained changes: No Go documentation or formatting changes were required; `LEARNING_LOG.md` records the completed benchmark task.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -run='^$' -bench='^BenchmarkClear$' -benchmem -benchtime=1s`; `go test -run='^$' -bench=. -benchmem -count=5`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully. Both the fixed-duration `BenchmarkClear` run and the repeated full benchmark suite completed successfully with allocation reporting.
- Important lesson: A benchmark needs a timing boundary for every setup phase, not just the setup before its loop. Labels must state the actual workload or the numbers are not interpretable.
- References: https://pkg.go.dev/testing#B.StopTimer ; https://pkg.go.dev/testing#B.StartTimer ; https://pkg.go.dev/testing#B.Loop
- Associated commit message recommendation: `bench(store): add clear benchmarks`
- Completed: 2026-07-18

## Active Task

### Task 27: Fuzz sequential store operations against a reference model

- Objective: Learn property-based testing by checking the store's sequential behavior against a plain Go map model.
- Why this is next: Deterministic tests cover known cases; fuzzing can generate combinations of operations and inputs that are difficult to enumerate, without expanding production behavior.
- Required behavior: Add one external-package fuzz target that interprets a byte slice as a bounded sequence of `Set`, `Get`, `Delete`, and `Clear` operations on `Store[byte, byte]`. Maintain a plain `map[byte]byte` model and after each operation assert equivalent values, missing-key errors through `errors.Is`, and `Len` results. Add at least two deterministic seed inputs with `f.Add`.
- Constraints: This task is sequential only; do not introduce goroutines, sleeps, randomness outside the fuzz input, or map-order-dependent assertions. Bound processing so a large fuzz input cannot make one execution unreasonably slow. Do not mutate the generated corpus manually.
- Acceptance criteria:
  - The fuzz target is in external package `kvstore_test` and has deterministic seeds.
  - Each supported operation is compared with the reference model at every step.
  - Missing-key behavior uses `errors.Is` with `ErrValueNotFound`.
  - The target passes ordinary tests and a fixed fuzz run.
  - Existing behavior remains unchanged.
  - Formatting, tests, race detection, and vet remain clean.
- Verification plan:
  - `git diff --check`
  - `gofmt -l .`
  - `go test ./...`
  - `go test -race ./...`
  - `go vet ./...`
  - `go test -run='^$' -fuzz=FuzzStoreSequentialOperations -fuzztime=5s`
- References:
  - https://go.dev/doc/security/fuzz
  - https://pkg.go.dev/testing#F
  - https://pkg.go.dev/errors#Is
- Status: Active.
- Review notes: The model is the oracle, not a second implementation of locking. Keep the operation encoding simple and the comparison local to each step so a failure points to the operation that diverged.
