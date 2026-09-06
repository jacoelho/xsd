# Local XSD 1.0 Reference

Use this directory to locate the relevant standards rule before changing schema
or validation semantics. The Markdown files are searchable topic guides; the
W3C XML snapshots under `xml/` preserve clause IDs cited by those guides.

| Question | Start here | Typical implementation owner |
| --- | --- | --- |
| Schema shape and basic terminology | `01_primer.md` | `internal/schema` |
| Element and attribute declarations, defaults, fixed values, qualification | `02_elements_and_attributes.md` | `internal/schema`, `internal/validate` |
| Complex types, particles, derivation, wildcards, abstract types | `03_complex_types.md` | `internal/schema` |
| `unique`, `key`, `keyref`, selectors, and fields | `04_identity_constraints.md` | `internal/schema`, `internal/validate` |
| Assessment order, `xsi:type`, `xsi:nil`, and validity rules | `05_validation.md` | `internal/validate` |
| Simple types, lexical/value spaces, facets, and whitespace | `06_data_types.md` | `internal/value`, `internal/schema` |
| Namespace declarations, form defaults, include/import, and composition | `07_namespaces.md` | `internal/xmlstream`, `internal/source`, `internal/schema` |
| Schema components, groups, annotations, notations, and uniqueness | `08_schema_components.md` | `internal/schema` |

Search by the specification term or constraint code before translating it into
repository vocabulary. Record the exact rule in the focused regression test.
Use `ARCHITECTURE.md` to locate the authoritative module and `tests/README.md`
to select the evidence surface.

These guides do not define supported behavior. XSD 1.0 and XML 1.0 requirements,
the repository's explicit public contract, and deliberate unsupported cases are
authoritative. If a guide conflicts with them, correct or remove the guide; do
not create a second implementation interpretation.
