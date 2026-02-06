# Internal Layer Boundaries

This directory documents the intended one-way dependency flow inside
`internal/`.

## Target Pipeline Direction

1. `xml` + `parser` + `loader` (source and parse)
2. `schemacheck` + `typeops` + `typegraph` (schema constraints and type semantics)
3. `schema` + `resolver` (IDs, resolved links, semantic graph integrity)
4. `runtimebuild` + `models` (runtime compilation)
5. `runtime` + `validator` + `ic` (runtime validation)

## Reusable Utility Packages

- `typeops`: shared type-resolution/facet logic.
- `typegraph`: shared base-chain and content-particle navigation.
- `traversal`: shared particle/content traversal helpers.
- `value`, `valuekey`, `num`: shared lexical/value-space primitives.

These utility packages must stay free of orchestration-layer imports.
