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

### Task 27: Fuzz sequential store operations against a reference model

- Objective: Learn property-based testing by checking sequential store behavior against a plain Go map model.
- Why it was selected: Deterministic tests cover known cases; fuzzing generates combinations of operations and inputs that are difficult to enumerate without expanding production behavior.
- Required behavior: One external fuzz target interprets bounded bytes as `Set`, `Get`, `Delete`, and `Clear` operations on `Store[byte, byte]`, comparing values, missing-key errors, and `Len` with a `map[byte]byte` model after each step.
- Constraints: Execution is sequential, bounded, deterministic except for fuzz input, and free of sleeps and map-order-dependent assertions.
- Acceptance criteria: The target has at least two deterministic seeds, compares every operation with the model, uses `errors.Is` for missing keys, passes ordinary tests and a fixed fuzz run, and keeps baseline checks clean.
- Learner implementation summary: Added `store_fuzz_test.go` with the correctly named `FuzzStoreSequentialOperations`, two operation-sequence seeds, a 64-operation bound, and full byte-keyspace model checks after each step.
- Codex-maintained changes: `README.md` and `AGENTS.md` now record the exact fixed-duration fuzz command; `store_fuzz_test.go` received formatting-only changes from `go fmt ./...`; `LEARNING_LOG.md` records the completed task.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -run='^$' -fuzz=FuzzStoreSequentialOperations -fuzztime=5s`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully. The five-second fuzz run executed 836,167 inputs with no failures and discovered six additional interesting coverage inputs in the fuzz cache.
- Important lesson: A fuzz target is useful when it has a simple oracle, bounded work per input, and failure messages that identify the diverging operation.
- References: https://go.dev/doc/security/fuzz ; https://pkg.go.dev/testing#F ; https://pkg.go.dev/errors#Is
- Associated commit message recommendation: `test(store): fuzz sequential operations`
- Completed: 2026-07-18

### Task 28: Add an atomic LoadAndDelete operation

- Objective: Add a single-operation read-and-remove capability that avoids check-then-delete races.
- Why it was selected: It complements atomic `LoadOrStore`, `Delete`, and `Clear` for callers who need the removed value without splitting work across operations.
- Required behavior: `LoadAndDelete` atomically removes and returns an existing value with `loaded == true`; absent keys return `V`'s zero value and `false`, including on a zero-value store.
- Constraints: It uses the exclusive lock, preserves shallow ownership, adds no errors or goroutines, and does not compose public `Get` and `Delete` calls.
- Acceptance criteria: External tests cover present, missing, zero-value, and concurrent same-key calls; only one concurrent caller loads the value; existing behavior and baseline checks remain clean.
- Learner implementation summary: Added the exclusive-lock `LoadAndDelete` method and external tests for present, missing, zero-value, and 100-way same-key contention behavior.
- Codex-maintained changes: `store.go` gained the comment-only method contract; `README.md` lists `LoadAndDelete`; `go fmt ./...` made no reported formatting changes.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully with no reported issues.
- Important lesson: Presence must be reported independently of the returned value because a stored value can itself be the type's zero value.
- References: https://pkg.go.dev/sync#Map.LoadAndDelete ; https://pkg.go.dev/sync#RWMutex.Lock ; https://go.dev/ref/mem
- Associated commit message recommendation: `feat(store): add atomic load and delete`
- Completed: 2026-07-18

### Task 29: Extend the fuzz model with LoadAndDelete

- Objective: Keep the model-based fuzz test aligned with the public mutation API.
- Why it was selected: `LoadAndDelete` needed sequence coverage alongside `Set`, `Get`, `Delete`, and `Clear`, beyond its deterministic operation tests.
- Required behavior: The fuzz target interprets one operation encoding as `LoadAndDelete`, compares its value and presence boolean to the map model, updates the model, and runs the full state check.
- Constraints: Production code and prior operation meanings remain unchanged; execution stays sequential and bounded; presence is independent of the returned value.
- Acceptance criteria: The operation set and a seed cover present and absent paths; each step checks value, boolean, and state; existing fuzz behavior remains intact; and baseline plus five-second fuzz checks pass.
- Learner implementation summary: Added the operation encoding and present/absent seed, compared `LoadAndDelete` results to model presence and value, and restored the required fuzz target name.
- Codex-maintained changes: `LEARNING_LOG.md` records the completed task; no additional documentation or formatting maintenance was needed.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -run='^$' -fuzz=FuzzStoreSequentialOperations -fuzztime=5s`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully. The exact five-second fuzz target ran 468,623 inputs with no failures and found two additional interesting coverage inputs.
- Important lesson: Model-based fuzzing must compare both a returned value and its presence indicator when the value type has a meaningful zero value.
- References: https://go.dev/doc/security/fuzz ; https://pkg.go.dev/sync#Map.LoadAndDelete ; https://pkg.go.dev/testing#F
- Associated commit message recommendation: `test(store): fuzz load and delete`
- Completed: 2026-07-18

### Task 30: Add an atomic Swap operation

- Objective: Add an atomic replacement primitive that returns the previous value without a caller-side read-then-write race.
- Why it was selected: `Swap` is the natural write-side complement to the existing atomic insertion, removal, and read-removal primitives.
- Required behavior: `Swap` stores its supplied value and returns the previous value plus prior key presence; missing keys return zero and false while being inserted.
- Constraints: It uses the exclusive lock and lazy initialization, adds no errors or goroutines, and preserves shallow ownership and zero-value usability.
- Acceptance criteria: External tests cover missing, existing, zero-value, zero stored value, and concurrent same-key cases; only one concurrent call is the initial insertion; baseline checks pass.
- Learner implementation summary: Added the exclusive-lock `Swap` method and external tests for all contract paths, including a concurrent value-chain invariant.
- Codex-maintained changes: `store.go` gained the comment-only `Swap` contract; `README.md` lists `Swap`; `go fmt ./...` made no reported formatting changes.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Regular tests and race tests passed, and vet completed successfully with no reported issues.
- Important lesson: The value returned by an atomic swap must be paired with a presence boolean; zero values alone cannot tell callers whether a key existed.
- References: https://pkg.go.dev/sync#Map.Swap ; https://pkg.go.dev/sync#RWMutex.Lock ; https://go.dev/ref/mem
- Associated commit message recommendation: `feat(store): add atomic swap`
- Completed: 2026-07-18

### Task 31: Extend the fuzz model with Swap

- Objective: Keep the model-based fuzz oracle aligned with the latest atomic mutation operation.
- Why it was selected: `Swap` required sequence coverage alongside the existing operations.
- Required behavior: The fuzz target compares `Swap`'s previous value and presence boolean to the map model, then updates the model and checks full state.
- Constraints: Production code, existing encodings, bounds, and sequential execution remain unchanged; model presence is evaluated before assignment.
- Acceptance criteria: The operation and a missing/existing seed are present; every step checks return values and state; baseline and five-second fuzz checks pass.
- Learner implementation summary: Added the `Swap` operation, seed, model comparison, state update, and target coverage.
- Codex-maintained changes: `LEARNING_LOG.md` records the completed task; no other maintenance was needed.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -run='^$' -fuzz=FuzzStoreSequentialOperations -fuzztime=5s`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Tests, race detection, and vet passed. The fuzz target ran 1,299,182 inputs in five seconds without failures.
- Important lesson: An atomic write operation's return values describe state before its write, so model comparison must precede model assignment.
- References: https://go.dev/doc/security/fuzz ; https://pkg.go.dev/sync#Map.Swap
- Associated commit message recommendation: `test(store): fuzz swap`
- Completed: 2026-07-18

### Task 32: Extend the fuzz model with LoadOrStore

- Objective: Keep the model-based fuzz oracle aligned with the atomic first-writer-wins operation.
- Why it was selected: `LoadOrStore` needed sequence coverage as a public atomic mutation primitive.
- Required behavior: The target compares value and loaded boolean to model state, inserts only when absent, and checks full state.
- Constraints: Production code, prior encodings, bounds, and sequential execution remain unchanged; model presence is evaluated before the call.
- Acceptance criteria: Operation and seed cover missing/existing paths; every step checks result and state; baseline and five-second fuzz checks pass.
- Learner implementation summary: Added the operation encoding, seed, model comparison, conditional insertion, and full-state check.
- Codex-maintained changes: `LEARNING_LOG.md` records the completed task; no other maintenance was required.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -run='^$' -fuzz=FuzzStoreSequentialOperations -fuzztime=5s`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Tests, race detection, and vet passed. The fuzz target ran 916,619 inputs in five seconds without failures.
- Important lesson: First-writer-wins operations require asymmetric model updates: only a missing key receives the supplied value.
- References: https://go.dev/doc/security/fuzz ; https://pkg.go.dev/sync#Map.LoadOrStore
- Associated commit message recommendation: `test(store): fuzz load or store`
- Completed: 2026-07-18

### Task 33: Extend the fuzz model with Update

- Objective: Exercise callback-based mutation in generated sequential operation sequences.
- Why it was selected: `Update` was the remaining public state-transition operation absent from the model fuzzer.
- Required behavior: The target uses a pure callback returning generated value, checks nil or `ErrValueNotFound` against model presence, updates the model only when present, and checks full state.
- Constraints: Production code, encodings, bounds, and sequential execution remain unchanged; callbacks never re-enter the store.
- Acceptance criteria: Operation and missing/existing seed are present; every step checks error and state; baseline and five-second fuzz checks pass.
- Learner implementation summary: Added update encoding, seed, pure callback, error comparison, conditional model update, and full-state check.
- Codex-maintained changes: `LEARNING_LOG.md` records the completed task; no other maintenance was needed.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -run='^$' -fuzz=FuzzStoreSequentialOperations -fuzztime=5s`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Tests, race detection, and vet passed. The fuzz target ran 1,199,786 inputs in five seconds without failures.
- Important lesson: Fuzzing callback APIs should use pure callbacks and verify visible state, avoiding re-entrancy hazards as part of the test itself.
- References: https://go.dev/doc/security/fuzz ; https://pkg.go.dev/testing#F
- Associated commit message recommendation: `test(store): fuzz update`
- Completed: 2026-07-18

### Task 34: Extend the fuzz model with Snapshot

- Objective: Exercise shallow map snapshots in generated sequential operation sequences.
- Why it was selected: `Snapshot` was not explicitly invoked by the model fuzzer.
- Required behavior: The target compares snapshot length and all byte-key/value entries to the model.
- Constraints: Production code, bounds, and sequential execution remain unchanged; comparison is order independent and does not mutate returned snapshots.
- Acceptance criteria: Snapshot operation and non-empty seed are present; complete map equivalence is checked; baseline and fuzz validation pass.
- Learner implementation summary: Added Snapshot and Len operation encodings, non-empty snapshot seed, and byte-keyspace snapshot equivalence checks.
- Codex-maintained changes: `LEARNING_LOG.md` records the completed task; no other maintenance was required.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -run='^$' -fuzz=FuzzStoreSequentialOperations -fuzztime=5s`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Tests, race detection, and vet passed. The fuzz target ran 740,966 inputs in five seconds without failures.
- Important lesson: Map equivalence tests must compare by key rather than range order.
- References: https://go.dev/doc/security/fuzz ; https://pkg.go.dev/maps#Equal
- Associated commit message recommendation: `test(store): fuzz snapshot`
- Completed: 2026-07-18

### Task 35: Benchmark atomic mutation operations

- Objective: Measure the steady-state cost of atomic mutation operations on existing keys.
- Why this is next: The learner expanded the related benchmark work to `Swap`, `LoadOrStore`, `Update`, `LoadAndDelete`, and `Delete`; these form one coherent performance group.
- Required behavior: Each benchmark measures exactly one existing-key operation per timed iteration, with construction, insertion, re-population, and map allocation excluded as applicable. Every benchmark reports allocations.
- Constraints: Do not measure missing-key behavior, map growth, setup, or unrelated work. Keep stable keys and deterministic setup.
- Acceptance criteria:
  - Swap, LoadOrStore, Update, LoadAndDelete, and Delete each measure their existing-key path.
  - Setup is excluded and allocation metrics are reported by every benchmark.
  - Existing benchmarks remain unchanged.
  - Baseline and both fixed-duration and repeated benchmark commands pass.
- Verification plan:
  - `git diff --check`
  - `gofmt -l .`
  - `go test ./...`
  - `go test -race ./...`
  - `go vet ./...`
  - `go test -run='^$' -bench='^BenchmarkSwap$' -benchmem -benchtime=1s`
  - `go test -run='^$' -bench=. -benchmem -count=5`
- References:
  - https://pkg.go.dev/testing#B.Loop
  - https://go.dev/doc/diagnostics#profiling
- Learner implementation summary: Added existing-key benchmarks for Swap, LoadOrStore, Update, LoadAndDelete, and Delete, with stopped-timer re-population for destructive operations.
- Verification results: On 2026-07-18, tests, race detection, and vet passed. Fixed-duration results reported 0 B/op and 0 allocs/op for every benchmark; the repeated full suite completed successfully.
- Associated commit message recommendation: `bench(store): benchmark atomic mutations`
- Completed: 2026-07-18

### Task 36: Benchmark Len under sequential and parallel reads

- Objective: Measure the store's point-in-time size query in single-threaded and parallel read scenarios.
- Why it was selected: `Len` is an established concurrent read operation whose sequential and contended costs should be measured independently.
- Required behavior: Add sequential and parallel Len benchmarks against a pre-populated store, with setup excluded and allocations reported.
- Constraints: Each timed iteration performs only `Len`; the parallel benchmark must leave setup outside `RunParallel` and must not manipulate benchmark timing from workers.
- Acceptance criteria: Both benchmarks measure only Len; parallel setup remains outside `RunParallel`; allocation metrics are reported; fixed-duration and repeated suites pass.
- Learner implementation summary: Added `BenchmarkSequentialLen` and `BenchmarkParallelLen`, each against 1,000 seeded entries; the parallel benchmark resets timing immediately before `RunParallel`.
- Codex-maintained changes: `LEARNING_LOG.md` records this completed task; `go fmt ./...` made no formatting changes.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -run='^$' -bench='Benchmark.*Len' -benchmem -benchtime=1s`; `go test -run='^$' -bench=. -benchmem -count=5`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Tests and race tests passed, and vet completed with no findings. The one-second Len run completed with `BenchmarkSequentialLen` at 11.84 ns/op and `BenchmarkParallelLen` at 34.99 ns/op; both reported 0 B/op and 0 allocs/op. The five-run full benchmark suite completed successfully.
- Important lesson: Parallel benchmark setup requires an explicit timing boundary before `RunParallel`; `RunParallel` workers should perform only the operation being measured.
- References: https://pkg.go.dev/testing#B.Loop ; https://pkg.go.dev/testing#B.RunParallel ; https://pkg.go.dev/testing#B.ReportAllocs
- Associated commit message recommendation: `bench(store): benchmark len reads`
- Completed: 2026-07-18

### Task 37: Benchmark Snapshot under parallel readers

- Objective: Measure snapshot behavior when multiple goroutines concurrently take independent snapshots of the same stable store.
- Why it was selected: `Snapshot` already had sequential size benchmarks; this focused benchmark reveals its concurrent-read and allocation profile without changing the API.
- Required behavior: Add one parallel `Snapshot` benchmark using a shared, pre-populated store of 1,000 entries. Every timed worker iteration must take exactly one snapshot, and setup must be excluded from timing.
- Constraints: Do not mutate the store in the benchmark, do not reuse or share returned snapshot maps, and do not control timers inside `RunParallel` workers. Report allocations.
- Acceptance criteria:
  - The benchmark uses `RunParallel` with setup completed before timing starts.
  - Each measured iteration calls only `Snapshot` on the shared store.
  - Allocation metrics are reported.
  - Existing benchmarks remain unchanged.
  - Baseline, one-second targeted, and repeated full benchmark suites pass.
- Learner implementation summary: Added `BenchmarkParallelSnapshot`, which shares a pre-populated 1,000-entry store and resets timing before its workers call `Snapshot`.
- Codex-maintained changes: `LEARNING_LOG.md` records this completed task; `go fmt ./...` made no formatting changes.
- Verification commands: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -run='^$' -bench='^BenchmarkParallelSnapshot$' -benchmem -benchtime=1s`; `go test -run='^$' -bench=. -benchmem -count=5`.
- Verification results: On 2026-07-18, diff and formatting checks passed. Tests and race tests passed, and vet completed with no findings. The one-second benchmark completed at 17,907 ns/op with 36,993 B/op and 6 allocs/op. The five-run full benchmark suite completed successfully.
- Important lesson: A parallel read benchmark can still allocate heavily when each operation intentionally materializes a new result; allocation metrics explain that contract cost.
- References: https://pkg.go.dev/testing#B.RunParallel ; https://pkg.go.dev/testing#B.ReportAllocs ; https://go.dev/ref/mem
- Associated commit message recommendation: `bench(store): benchmark parallel snapshots`
- Completed: 2026-07-18

## Active Task

### Task 38: Benchmark Set under parallel write contention

- Objective: Measure the existing-key `Set` path when several goroutines contend for the store's exclusive lock.
- Why this is next: The suite now measures parallel read operations and sequential writes, but does not measure the lock-contention profile of the core write operation.
- Required behavior: Add one parallel `Set` benchmark against a shared store containing the benchmark key. Setup must be excluded, and every worker iteration must perform exactly one existing-key `Set`.
- Constraints: Do not measure map growth, absent-key insertion, or per-worker setup. Do not control timers inside workers. Use a deterministic value and report allocations.
- Acceptance criteria:
  - The benchmark uses `RunParallel` with a timing boundary after setup.
  - All workers write the same existing key.
  - The timed worker body contains only one `Set` operation.
  - Allocation metrics are reported.
  - Baseline, one-second targeted, and repeated full benchmark suites pass.
- Verification plan: `git diff --check`; `gofmt -l .`; `go test ./...`; `go test -race ./...`; `go vet ./...`; `go test -run='^$' -bench='^BenchmarkParallelSet$' -benchmem -benchtime=1s`; `go test -run='^$' -bench=. -benchmem -count=5`.
- References: https://pkg.go.dev/testing#B.RunParallel ; https://pkg.go.dev/sync#RWMutex.Lock ; https://pkg.go.dev/testing#B.ReportAllocs
- Status: Active.
