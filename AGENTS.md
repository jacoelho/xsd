# Repository Instructions

`ARCHITECTURE.md` is the sole architecture source of truth.

## Workflow

1. **Scope.** Read the request, `git status`, and relevant history. Derive
   discoverable facts from the repository before asking questions. Preserve
   unrelated worktree changes.
2. **Route.** For every design, implementation, diagnosis, or review, open
   `ARCHITECTURE.md` and follow **Authority And Navigation** to the module and
   flow sections the task reaches, plus their relevant rejected alternatives.
   Trace from the entrypoint to the state owner, including callers, writers,
   failure exits, and cleanup.
3. **Design.** Shape the change as the dependency-complete packet defined in
   **Change Protocol**: contract, owner, invariant, path, bounds, proof, and
   writeback. Apply its design rules and the test rules in
   **Tests And Enforcement**.
4. **Write back.** Put each durable fact in the owner assigned by
   **Documentation Ownership** and remove stale copies.

A change is complete when current behavior, `ARCHITECTURE.md`, tests, and public
documentation agree; the verification contract passes; no partially migrated
path remains; and every remaining follow-up is reported to the user or owned by
a `.codex` plan.

## Project Contract

- Supported inputs are XSD 1.0, XML 1.0, and UTF-8. Other XML versions are rejected.
- Schema sources are explicit. The library never dynamically loads schemas over
  HTTP, another network transport, or instance `xsi:schemaLocation` hints.
- Compilation, source resolution/opening, and validation are synchronous and
  context-free. Library interfaces do not accept or propagate `context.Context`;
  callers own interruption of blocking resolvers, openers, files, and readers.
- This root file holds all agent instructions; subdirectories carry none.

## Verification Contract

- When a change or conformance diagnosis concerns schema or instance validity
  that the corpus harness can express (default options, explicit file roots,
  independent instances), add project-owned corpus cases until every edge case
  of the affected rule is covered, as
  [Project Fixture Organization](tests/README.md#project-fixture-organization)
  defines. Cover other behavior at the matching row of the `tests/README.md`
  evidence table.
- For XSD/XML conformance diagnosis or new/changed corpus cases, follow the
  mandatory [cross-validator comparison](tests/README.md#cross-validator-diagnosis)
  procedure.
- Format changed Go files with `gofmt` and run `git diff --check`.
- Before completion run `make test`, `make race`, `make wasm-test`,
  `make web-test`, `make browser-test`, `make fuzz-smoke`, `make bench-smoke`,
  `make staticcheck`, and `make lint`.
- For changed validation, namespace, stream, derivation, or content-model hot
  paths, compare `-benchmem` results against the pre-change tree with
  `benchstat` (`-count` of at least 10, runs serialized); use the
  [commit comparison](BENCHMARKS.md#commit-comparisons) workloads where they
  apply.
