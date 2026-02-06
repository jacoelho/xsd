# Internal Layer Boundaries

This directory documents the intended one-way dependency flow inside
`internal/`.

## Target Pipeline Direction

1. `xsdxml` + `parser` + `source` (source and parse)
2. `semanticcheck` + `semanticresolve` + `semantic` + `pipeline` (semantic preparation)
3. `runtimecompile` + `contentmodel` (runtime compilation)
4. `runtime` + `validator` + `identity` (runtime validation)

## Reusable Utility Packages

- `typeops`: shared type-resolution/facet logic.
- `typegraph`: shared base-chain and content-particle navigation.
- `traversal`: shared particle/content traversal helpers.
- `value`, `valuekey`, `num`: shared lexical/value-space primitives.
- `whitespace`, `xpath`, `xmlnames`, `ids`: shared low-level helpers.

These utility packages must stay free of orchestration-layer imports.
