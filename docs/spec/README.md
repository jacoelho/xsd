# Local XSD 1.0 Reference

Start with the [corrected reading](corrected-reading.md): the English
**28 October 2004 Second Edition** plus all approved later errata. The reading
layer applies E1-56 alongside unchanged official publications. It is
independently authored commentary, not a replacement W3C specification.

| Reference | Purpose |
| --- | --- |
| [Sources and authority](SOURCES.md) | Editions, approved corrections, normative appendix labels, related Notes, dependency versions, and source caveats. |
| [Source attribution](NOTICE.md) | Original copyright/status and the notice accompanying implementation-support extractions. |
| [Primer](upstream/primer.html), [Structures](upstream/structures.html), [Datatypes](upstream/datatypes.html) | Complete published HTML, including generated tables and appendix bodies; Primer is explanatory. |
| [Approved Second Edition errata](upstream/errata-second-edition.html) | E1-56; no Part 0 or Part 2 entries in the captured list. |
| [Scenario catalogues](scenarios/README.md) | Specification-derived decisions, equivalence partitions, boundary cases, interactions, and unresolved questions for later corpus comparison. |
| [Concrete branch scenarios](scenarios/branches/README.md) | Explicit witnesses and dispositions for numbered baseline branches; description closure is distinct from executed coverage. |
| [Grammar contexts](scenarios/grammar-contexts.md), [interactions](scenarios/interactions.md), [external rules](scenarios/external-rules.md) | Concrete expansions beyond the numbered branches, with effective contexts, dependent axes and version-qualified external premises. |
| [Interpretation records](INTERPRETATIONS.md) | W3C clarification history and later-version fixes kept separate from approved 1.0 errata. |
| [Source questions](scenarios/source-questions.md) | Unresolved text-integration and source-version premises, kept separate from approved corrections and corpus expectations. |
| [Source manifest](source-manifest.json) | Original URLs, retrieval date, byte lengths, roles, and SHA-256 fingerprints. |
| [Derived inventory](inventory.json) | Every published section, named constraint, numbered branch, property definition, representation summary, regex production, and dated Appendix A grammar node. |

Named-constraint mappings, numbered-branch descriptions, grammar contexts,
dependent products and external-reference partitions are described. Independent
semantic review and comparison against actual fixtures remain distinct work.
The catalogues do not assert corpus coverage.

`upstream/` contains unchanged W3C publications, appendix fragments, historical
change views, and independent schema distributions. `dependencies/` preserves
available direct references. The retained IEEE technical text and corrected
Clinger report have explicit [edition qualifications](SOURCES.md#floating-point-source-versions).
`xml/` preserves the original XML sources and their DTDs. Those XML files
retain historical deleted text and external/generated placeholders: use the
published HTML for semantic reading. The `*-with-errata.html` files show
2001–2004 changes and do not incorporate E1-56.

The topic guides below remain navigation aids. Search by the standards term
or constraint ID before translating a rule into repository vocabulary.

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

Record the exact source rule when adding a focused regression test.
Use `ARCHITECTURE.md` to locate the authoritative module and `tests/README.md`
to select the evidence surface.

These guides do not define supported behavior. XSD 1.0 and XML 1.0 requirements,
the repository's explicit public contract, and deliberate unsupported cases are
authoritative. If a guide conflicts with them, correct or remove the guide; do
not create a second implementation interpretation.

When updating this reference, verify retained source hashes and byte lengths
against the source manifest. Keep the inventory, named-constraint mappings,
scenario IDs, branch dispositions, Appendix A ownership and local citations
aligned with the retained publications. Appendix C supplies an independent
constraint enumeration. These checks establish traceability, not exhaustive
semantic or corpus coverage. Source changes require a separate provenance
and errata review.
