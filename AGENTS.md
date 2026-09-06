# Repository Instructions

## Operating Loop

1. **Scope.** Read the request, `git status`, and relevant history. Preserve
   unrelated worktree changes. Derive discoverable facts from the repository
   before asking questions.
2. **Route.** For every design, implementation, diagnosis, or review, open
   `ARCHITECTURE.md`, select the matching row in **Authority And Navigation**,
   then read only that module contract, flow, and relevant rejected alternatives.
3. **Trace.** Follow the complete path from entrypoint to authoritative state
   owner, including callers, readers, writers, representations, transitions,
   limits, failure exits, cleanup, tests, and measurements.
4. **Design.** Define the smallest dependency-complete change packet from
   `ARCHITECTURE.md`: contract, owner, invariant, path, bounds, proof, and
   writeback. Treat explicit requirements and external contracts as binding;
   treat existing internal structure as replaceable.
5. **Implement.** Replace the path atomically. Keep one canonical representation
   and one execution path. Delete superseded code instead of adding shims,
   feature flags, deprecated paths, or parallel state models unless an external
   migration contract requires them.
6. **Prove.** Run focused tests first, then the full verification contract.
   Check the complete affected seam and opposing/adversarial shapes, not only the
   edited file or motivating example.
7. **Accrete.** Put each durable fact in its owner: architecture in
   `ARCHITECTURE.md` with only a routing pointer here, public usage in
   `README.md`, executable behavior in tests, non-obvious local intent beside
   code, future work in a plan, and revision-scoped evidence in a ledger. Remove
   stale copies.

The work is complete when current behavior, architecture, tests, and public
documentation agree; required checks pass; and no partially migrated path or
unowned follow-up remains.

## Communication

- Use the fewest words that preserve meaning, precision, and necessary context.
- State evidence, uncertainty, risks, and trade-offs directly. Avoid filler,
  repetition, praise, and reflexive agreement.
- Report material findings only. Consolidate symptoms under their root cause.
- Comments explain non-obvious intent, constraints, invariants, or consequences.

## Design Rules

- Organize by vertical business capability. A module owns its vocabulary,
  state, invariants, behavior, interface, and tests.
- Give each fact, policy, state transition, mutable resource, and lifecycle one
  authoritative owner. Make data flow, mutation, start, stop, rollback,
  cancellation, and failure visible.
- Make illegal states unrepresentable where practical. Validate and normalize
  external data once at entry; enforce invariants at construction and transition.
- Keep domain decisions deterministic and side-effect-free where practical.
  Isolate I/O, clocks, randomness, concurrency, storage, transport, frameworks,
  and vendor formats at their owning edge.
- Keep dependencies explicit and acyclic. Pass them directly. Add a narrow,
  consumer-owned interface only at a present substitution seam; use concrete
  types otherwise. Avoid globals, registries, hidden initialization, reflection,
  callbacks, service locators, and cross-module reach-through.
- Choose the smallest design satisfying current constraints. An abstraction
  must delete more complexity than it adds. Prefer small syntax duplication to
  false coupling; centralize duplicated policy and invariants.
- Use one term per concept. Name modules for owned capabilities, not vague roles
  such as `manager`, `core`, `common`, `engine`, or `utils`.
- Bound work, retained memory, queues, batches, recursion, retries, concurrency,
  and I/O at the admitting owner. Caches and projections remain reproducible
  from an explicit source and have bounded retention.
- Make ordering, delivery, acknowledgement, retry, idempotency, deduplication,
  consistency, backpressure, timeout, and partial-failure semantics explicit
  where distributed or asynchronous behavior exists.
- Presentation and transport translate or render; they do not own domain policy.
- Fix defect classes. When a defect reveals split ownership, invalid state,
  duplicated policy, or unsafe sequencing, repair the model that permits it.
- Prefer explicit control flow and actionable contextual errors. Never depend on
  logs to reconstruct failure or collapse errors into ambiguous booleans.
- Test observable contracts at the owning interface. Use deterministic fakes;
  exercise invariants, boundaries, failure, rollback, reuse, concurrency, and
  resource limits. Delete change-detector tests that mirror implementation.

## Project Contract

- `ARCHITECTURE.md` is the sole architecture source of truth. Plans and ledgers
  may reference it but cannot redefine current ownership or lifecycle.
- Supported inputs are XSD 1.0, XML 1.0, and UTF-8. XML 1.1 is rejected.
- Schema sources are explicit. The library never dynamically loads schemas over
  HTTP, another network transport, or instance `xsi:schemaLocation` hints.
- Compilation, source resolution/opening, and validation are synchronous and
  context-free. Library interfaces do not accept or propagate `context.Context`;
  callers own interruption of blocking resolvers, openers, files, and readers.
- Preserve unrelated changes. Do not create nested `AGENTS.md` files.

## Verification Contract

- Format changed Go files with `gofmt` and run `git diff --check`.
- Before completion run `make test`, `make race`, `make wasm-test`,
  `make web-test`, `make browser-test`, `make fuzz-smoke`, `make bench-smoke`,
  `make staticcheck`, and `make lint`.
- For changed validation, namespace, stream, derivation, or content-model hot
  paths, also run focused allocation and benchmark comparisons.
