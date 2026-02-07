# Internal Architecture Refactor Baseline

This document tracks the baseline and phased plan for reducing internal package
complexity without changing schema validation behavior.

## Baseline (measured on 2026-02-07)

- Internal packages: 26
- Internal Go files: 419
- Internal LOC: 84,728
- Largest files:
  - `internal/runtimecompile/schema.go`: 1563 lines
  - `internal/types/simple_type.go`: 1080 lines
  - `internal/types/comparable_values.go`: 1060 lines
  - `internal/semanticcheck/particles.go`: 1038 lines

Top internal import fan-out (direct internal deps):

- `internal/runtimecompile`: 15
- `internal/semanticcheck`: 10
- `internal/validator`: 9
- `internal/semanticresolve`: 9

## Refactor goals

- Remove low-value wrapper indirection in traversal helpers.
- Consolidate safe traversal and attribute QName helper logic.
- Decompose oversized files into focused modules.
- Reduce unnecessary coupling while preserving deterministic behavior.
- Add CI guardrails to prevent architecture regressions.

## Non-goals for this cycle

- No XSD semantic behavior changes.
- No external/public API changes.
- No `internal/types` package split yet.
- No interface-first rewrites without concrete substitution needs.

## Guardrails

The architecture guardrail script (`scripts/check_arch.sh`) enforces:

- Package graph integrity (`go list ./...`)
- Fan-out visibility report for internal packages
- Validator production boundary:
  non-test files in `internal/validator` must not import semantic layers
