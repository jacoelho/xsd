# Repository Instructions

## Communication

- Use the fewest words that preserve meaning, precision, and necessary context. Remove filler, repetition, praise, and process narration. Do not sacrifice completeness for brevity.
- Do not flatter, reassure, or agree reflexively. Give the strongest evidence-based assessment. State disagreement, uncertainty, risks, and trade-offs directly.
- Comments explain non-obvious intent, constraints, invariants, or consequences. Do not restate the code.

## Design Stance

- Treat explicit requirements, external contracts, persisted data, and operational constraints as binding. Treat existing internal structure as replaceable.
- Design the clean target state first. Unless compatibility or migration is explicitly required, prefer breaking changes and deletion over compatibility wrappers, shims, deprecated paths, or parallel implementations.
- Fix classes of defects, not symptoms. When a defect exposes ambiguous ownership, invalid state, duplicated policy, or unsafe sequencing, change the model or boundary so recurrence becomes difficult or impossible. Do not over-engineer isolated mistakes.
- Correctness and explicit requirements outrank compatibility with accidental internal design.

## Architecture

- Organise code by vertical business capability, not horizontal technical layer. Each module owns its vocabulary, state, invariants, behaviour, contracts, and tests.
- Give every fact, invariant, state transition, and capability one authoritative owner. Use one canonical internal representation and one execution path for the same operation. Avoid mirrored state, competing writers, duplicate state machines, and split ownership.
- Make illegal states unrepresentable where practical. Enforce invariants when values are created and when state transitions occur.
- Validate and normalise external data once at the boundary. Do not pass partially valid, vendor-specific, transport-shaped, or storage-shaped data into domain logic.
- Keep domain decisions deterministic and side-effect-free where practical. Isolate I/O, clocks, randomness, concurrency, storage, transport, frameworks, and vendor integrations at the edges.
- Keep dependencies explicit and acyclic. Pass dependencies directly. Use narrow, consumer-owned interfaces only at real boundaries. Avoid globals, service locators, hidden registries, implicit initialisation, and cross-module reach-through.
- Choose the smallest design that satisfies the stated constraints. Every abstraction must remove more complexity than it introduces. Avoid speculative generality and frameworks designed for hypothetical future requirements.
- Use domain language consistently: one term per concept and one meaning per term. Name components after the capability they own, not vague roles such as `manager`, `core`, `common`, `engine`, or `utils`.
- Centralise duplicated policy, knowledge, and invariants, not merely similar syntax. Small code duplication is preferable to false coupling. Duplicated business rules are not.
- Make data flow, mutation, ownership, and lifecycle visible. Prefer immutable values and explicit transitions. Every mutable resource, task, goroutine, queue, cache, and connection has one owner and a defined start, stop, cancellation, and failure path.
- Bound all work. Define limits for concurrency, memory, queues, batches, retries, and execution time. Avoid unbounded accumulation and hidden background work.
- Make distributed-system semantics explicit: ordering, delivery guarantees, acknowledgement points, retries, idempotency, deduplication, consistency, backpressure, timeout behaviour, and partial failure.
- Prefer partitioned ownership and local decisions over global coordination, shared mutable state, hot rows, central locks, or singleton coordinators.
- Presentation and transport layers translate or render. They do not own business rules, state transitions, or validation policy.
- Derived state must have a clearly identified source and be reproducible from that source. A cache or projection must not silently become authoritative.
- Delete obsolete paths when replacing behaviour. Do not leave two implementations, state models, or control flows for the same responsibility.

## Architecture Documentation

- Maintain one canonical architecture document. It is the source of truth for boundaries, ownership, contracts, data flows, invariants, state transitions, failure modes, constraints, and trade-offs.
- Resolve conflicting designs rather than documenting all alternatives indefinitely. Remove or clearly mark superseded decisions.
- A design is incomplete while ownership, lifecycle, failure behaviour, performance bounds, or cross-boundary contracts remain ambiguous.
- Record material rejected alternatives and explain the constraint or trade-off that rejected them.

## Implementation and Review

- Inspect the code, tests, history, and documentation before asking questions. Do not ask the user for information the repository can answer.
- Review complete execution paths and subsystem boundaries, not isolated files. Trace callers, dependencies, state ownership, concurrency, persistence, errors, and tests.
- Prefer concrete types until an interface is required by a real boundary or consumer. Keep interfaces narrow and behavioural.
- Prefer explicit control flow over callbacks, reflection, registration magic, hidden mutation, and framework-driven behaviour.
- Errors must preserve context and remain actionable. Do not swallow errors, convert them into ambiguous booleans, or depend on logs to reconstruct what failed.
- Tests must exercise invariants, contracts, failure behaviour, cancellation, retries, and concurrency semantics. Prefer deterministic tests and explicit fakes.
- Report material findings only. Consolidate duplicates, distinguish root causes from symptoms, and avoid producing endless low-value observations after coverage is complete.

## Project Contract

- `ARCHITECTURE.md` is the sole source of truth for repository architecture. Other documentation may link to it but must not restate a competing design.
- The supported standards are XSD 1.0, XML 1.0, and UTF-8. XML 1.1 remains unsupported and rejected.
- Schema sources are explicit. The library does not dynamically load schemas over HTTP or another network transport.
- `internal/compile` owns schema semantics and compiler orchestration. `internal/runtime` owns immutable published schema data and reusable runtime algorithms. `internal/validate` owns document-local validation state. `internal/stream` owns borrowed token lifetimes. `internal/xmlns` owns namespace policy and retained namespace contexts.
- Compilation, schema resolution/opening, and validation are synchronous and context-free. Public and internal library APIs must not accept or propagate `context.Context`. Callers own cancellation or interruption of blocking resolvers, openers, files, and readers.
- Use concrete types unless a present production boundary requires substitution. Do not add an interface solely to make mocking easier.
- Preserve unrelated dirty-worktree changes. Replace internal paths atomically and delete obsolete implementations; do not add compatibility layers or feature-flagged parallel paths.
- Bound work, retained memory, queues, recursion, batches, and I/O at their owning boundary. Every reader, goroutine, timer, worker, cache, and scratch buffer has one lifecycle owner and a bounded retention policy.

## Verification

- Format changed Go files with `gofmt`.
- Run `git diff --check`.
- Run `make test`, `make race`, `make wasm-test`, `make web-test`, `make browser-test`, `make fuzz-smoke`, `make bench-smoke`, `make staticcheck`, and `make lint` before declaring the work complete.
- Run focused allocation and benchmark checks for any changed validation, namespace, stream, derivation, or content-model hot path.
- Do not create nested `AGENTS.md` files.
