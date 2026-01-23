# Attribute use="prohibited" still accepts fixed values

- Severity: Major
- Area: Correctness
- Location: internal/parser/attribute.go:84-206, internal/schemacheck/attributes.go:48-70

## Summary
Attribute declarations with `use="prohibited"` reject `default` but still accept `fixed`. XSD 1.0 mapping rules state that `use="prohibited"` means the attribute use does not exist, so value constraints must be absent. The current parser and schemacheck only block defaults, allowing invalid schemas through.

## Consequence
Schemas can declare prohibited attributes with fixed values without error, which should be invalid. This can later surface as inconsistent validation behavior or incorrect acceptance of invalid schemas.

## Fix
- In `parseAttribute`, reject `use="prohibited"` when `default` is present (both the ref and local branches).
- In `validateAttributeDeclStructure`, add a guard for `decl.Use == types.Prohibited && decl.HasDefault` to catch programmatically constructed schemas.
- Follow W3C XSD 1.0 tests for `fixed` + `prohibited` (allowed, fixed is effectively inert).

## Test
- Add a parser/loader test with a local attribute `use="prohibited" default="X"` and assert schema validation fails.
- Add a similar test for an attribute reference with `use="prohibited" default="X"` to ensure both branches are covered.

## Resolution Notes (Project Behavior)

This project follows the W3C XSD 1.0 test suite behavior:
- `use="prohibited"` + `default` → **invalid**
- `use="prohibited"` + `fixed` → **valid** (fixed is effectively inert)

Check W3C tests:
- attKb009
- attKc009
- attP029
- attP031
