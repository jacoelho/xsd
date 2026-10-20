# XSD 1.0 reference notes

The oracle baseline is the English **28 October 2004 Second Edition**, read
with approved later errata. These notes record source authority and decisions;
they do not replace the specifications or assert exhaustive corpus coverage.

| Reference | Purpose |
| --- | --- |
| [Sources](SOURCES.md) | Authoritative editions, appendix scope and dependency-version caveats. |
| [Corrected reading](corrected-reading.md) | The seven E1-56 changes and their PSVI scope. |
| [Interpretations](INTERPRETATIONS.md) | Published-rule decisions, WG clarifications and unresolved boundaries. |
| [Provenance](source-manifest.json) | Historical source URLs, retrieval metadata, byte lengths and SHA-256 hashes. Paths identify audit download locations, not a bundled-file inventory. |
| [Attribution](NOTICE.md) | Source notices and reuse terms. |

Read the published [Primer](https://www.w3.org/TR/2004/REC-xmlschema-0-20041028/primer.html),
[Structures](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html)
and [Datatypes](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html)
online. Bulk publication/dependency archives and the research scenario catalogue
are not bundled. The preexisting `xml/` sources remain as historical reference;
they contain deleted text and generated/external placeholders, so use published
HTML for semantic reading. Historical `*-with-errata.html` views compare 2001
with 2004 and do not incorporate E1-56.

The topic guides remain navigation aids:

| Topic | Guide |
| --- | --- |
| Basic schema vocabulary | [Primer](01_primer.md) |
| Declarations, defaults, fixed values and form | [Elements and attributes](02_elements_and_attributes.md) |
| Particles, derivation and wildcards | [Complex types](03_complex_types.md) |
| Keys, unique constraints and references | [Identity constraints](04_identity_constraints.md) |
| Assessment, xsi:type and xsi:nil | [Validation](05_validation.md) |
| Lexical/value spaces and facets | [Datatypes](06_data_types.md) |
| Namespace admission and composition | [Namespaces](07_namespaces.md) |
| Groups, annotations and notations | [Schema components](08_schema_components.md) |

[ARCHITECTURE.md](../../ARCHITECTURE.md) owns implementation boundaries;
[tests/README.md](../../tests/README.md) owns corpus operation.
`tests/manifest.json` remains the only inventory and expectation authority.
Cite the exact source rule for new cases. Preserve unresolved oracles,
unobservable outputs and deliberate product limits separately from executed
coverage; processor agreement is diagnostic evidence, not an oracle.
