# Selected later corrections to the XSD 1.0 baseline

The library supports XSD 1.0 with the corrections enumerated in
[SOURCES.md](SOURCES.md#selected-corrections). A later specification can repair
an earlier defect without making the whole later language applicable. Each
adoption needs a source-backed rule, a bounded implementation and an opposing
witness. Validator agreement alone supplies none of those.

This audit used the local Second Edition references, approved E1-56 reading,
interpretation records, and the change lists in the final
[Structures](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#changes) and
[Datatypes](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/datatypes.html#changes)
Recommendations. The family census below screens candidates; it does not claim
exhaustive implementation verification of every subordinate change.

## Adopted boundaries and proof

The interpretation records own the precise selected semantics. The manifest
owns executable expectations and provider observations.

| Boundary | Result of audit | Observable proof |
| --- | --- | --- |
| Fixed attribute uses | Replace canonical-spelling comparison with typed equality, [I-009](INTERPRETATIONS.md#i-009--fixed-attribute-use-values). | Integer/boolean aliases across local, referenced and grouped uses; numeric/list/QName equality; unequal values, list order, string whitespace and source-pattern opponents. |
| Restricted union integrity | Preserve effective constrained members and block member-type derivation escapes, [I-010](INTERPRETATIONS.md#i-010--restricted-unions-retain-their-constraints). | Direct/inherited enumeration and pattern, nested unions, `xsi:type` and substitution affiliation; legitimate restrictions and unconstrained alternatives remain valid. |
| Physical XSI type resolution | Already implemented; adopt WG correction 4299 explicitly, [I-014](INTERPRETATIONS.md#i-014--xsitype-under-element-wildcards). | Known types under strict/lax wildcards validate their constraints; unknown, malformed, unbound and abstract selections reject; skip ignores schema assessment throughout its subtree. |
| Nilled identity fields | Selected simple/simple-content nodes lack tuple values, [I-003](INTERPRETATIONS.md#i-003--nilled-field-values). | Unique/keyref omit incomplete tuples; key requires values; nil/value and nil/nil in one field still violate selected-node cardinality; nonnil empty strings collide. |
| Skipped identity nodes | Treat skip as an assessment boundary, [I-002](INTERPRETATIONS.md#i-002--fields-inside-a-skipped-subtree). | Ignore skipped selector rows and skipped element/attribute fields; an assessed key row with only skipped fields still fails. Lax and recovery remain separate. |
| Group-to-wildcard restriction | Select R-240's temporary zero member minimum, [I-004](INTERPRETATIONS.md#i-004--published-member-minimum-rule). | All/sequence/repeated-choice positives; original aggregate, namespace and processing-strength negatives. |
| Substitution-choice order | Select R-122's generated-choice waiver on either operand, [I-005](INTERPRETATIONS.md#i-005--implicit-substitution-choice-order). | Reversed generated members pass; explicit reversal, span crossing, range and membership opponents fail. Adjacent generated spans remain distinct. |
| Complex simple-content fields | Already implemented; make the WG clarification explicit, [I-001](INTERPRETATIONS.md#i-001--complex-types-with-simple-content-as-identity-fields). | Equal/distinct integer values across simple and complex simple-content declarations; element-only complex content remains invalid. |
| Regex braces/bar | Already implemented; adopt the consistency repair, [I-011](INTERPRETATIONS.md#i-011--regex-metacharacter-grammar). | Bare braces fail, escaped braces/bar match literally, ordinary alternation works, `^`/`$` remain XSD literals. |
| Effective declaration constraints | Already implemented; clarify global constraint fallback for an absent optional attribute reference. | No local override applies the global default; explicit empty override and required presence are separate controls. XSI still supplies no absent values. |
| Identity-component resolution | Already implemented; later `src-resolve` recognizes the omitted component kind. | Existing key/unique references and `idH035` keyref-to-keyref rejection. No new lookup algorithm or missing-target acceptance. |
| Binary64 extrema | Already implemented; select corrected integer-exponent limits −1074…971. | Smallest subnormal and largest finite double, signed zero and rounding/overflow controls; no blanket new lexical forms. |
| Composite document identity | Existing recursive propagation remains correct; close default-replay coverage. | User lists/unions retain IDREF projections through physical and defaulted values; resolved/missing targets and physical overrides distinguish the paths. Scalar/nonempty ENTITY assessment is unsupported; empty lists and lexical non-NCName string fallback are separate controls. |

The existing XSD/XSI import and XSI no-synthesis adoptions remain in I-006 and
I-007. None of these corrections selects declarations by namespace URL or
permits schema retrieval. Explicit source closure, builtin component authority,
source limits and instance-hint nonloading remain binding.

## Source-family dispositions

The references below are screening categories, not a blanket adoption of their
contents. Editorial clarifications may confirm existing behavior; new language
facilities and deliberate semantic changes need independent authorization and
implementation.

| Change-list family | Disposition in this audit |
| --- | --- |
| Structures G.1.1 dependencies | Keep XML 1.0. No automatic XPath tokenization, Unicode, URI or referenced-edition upgrade. |
| G.1.2 schema versions | Exclude conditional inclusion and version-selection facilities. |
| G.1.3 content models | Exclude relaxed UPA, expanded all groups and new wildcard facilities. |
| G.1.4 assertions | Exclude assertions and XPath expression facilities. |
| G.1.5 complex derivation | Keep the 1.0 structural algorithm; select only bounded R-122/R-240 repairs. |
| G.1.6 complex types | No blanket mixed-content or consistency-rule relaxation. |
| G.1.7 ID-related types | Confirm recursive IDREF/ENTITY handling; retain existing ID defaults, ID-attribute and DTD boundaries. |
| G.1.8 simple types | Select union integrity and typed constraints; exclude new primitive/universal types and assertions. |
| G.1.9 elements | No type alternatives, abstract-type or other blanket element-policy adoption. |
| G.1.10 attributes | Select effective constraint fallback and existing XSI corrections; exclude inheritable attributes. |
| G.1.11 component model | No requirement to expose new component/property APIs. |
| G.1.12 assessment | Select skip identity processing and physical `xsi:type` resolution (4299); no broader declaration/type fallback policy. |
| G.1.13 PSVI | Preserve existing observation limits; add no PSVI export surface. |
| G.1.14 conformance | No XSD 1.1 conformance claim or automatic error-classification replacement. |
| G.1.15 composition | Retain selected import exception; exclude override and dynamic network acquisition. |
| G.1.16 miscellaneous | Require individual evidence; no general adoption. |
| G.1.17 clarifications | Select simple-content identity fields and the individually recorded corrections. |
| Datatypes I.1 facets/types | Keep applicable clarifications separate from new assertions/types. |
| I.2 numeric | Correct binary64 bounds; retain unsigned signed aliases and existing 1.0 NaN equality. Exclude `+INF` and changed value models. |
| I.3 temporal | Do not import year zero, timezone-retention or changed temporal ordering. Existing lexical/arithmetic qualifications remain in the interpretation records. |
| I.4 other | Select union/regex repairs; preserve NOTATION checks. No new datatypes or automatic dependency upgrade. |

For example, [9196](https://www.w3.org/Bugs/Public/show_bug.cgi?id=9196)
addresses a NaN regression in 1.1; it does not justify replacing this library's
existing 1.0 equality. [10662](https://www.w3.org/Bugs/Public/show_bug.cgi?id=10662)
supports consistent handling through derived/composite types, but does not
justify ID defaults, multiple ID attributes or DTD support. An unobservable
PSVI property is not proven by an instance-validity result.

### ENTITY member-selection qualification

Do not claim full ENTITY adoption of 10662. In an `ENTITY|string` union, a
lexical NCName such as `missing` does not provide an unambiguous string-fallback
oracle. The later rules separate datatype-valid member selection from ENTITY
assessment against document declarations. The current unsupported ENTITY result
occurs before facets: making it immediately terminal could suppress legitimate
fallback after an enclosing list or restricted union fails its datatype rules.
A complete phase change would need nested value propagation and explicit
facet-literal construction semantics. It is not selected here under the existing
DTD-unsupported contract. The conformance fallback witness uses a non-NCName;
older NCName observations remain qualified evidence without conformance credit.
See [Datatype Valid](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/datatypes.html#cvc-datatype-valid)
and [String Valid](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#cvc-simple-type).

## Four-validator diagnosis

Compare each changed semantic boundary with Java Apache Xerces, .NET XSD,
Python `xmlschema` in XSD 1.0 mode, and libxml2, using the
[corpus comparison contract](../../tests/README.md#cross-validator-diagnosis).
Pin source bytes, versions and source closure; disable external acquisition,
DTDs and instance hints. Separate schema construction, instance assessment,
unsupported results and provider failures.

Disagreements found in this audit include canonical/facet treatment of fixed
literals, inherited/nested restricted unions, selected nilled-field cardinality,
generated-choice mapping and literal regex anchors. They remain observations
against the selected source rule, not majority votes. A libxml2 substitution
construction timeout is unassessed, never evidence of schema invalidity.
Exact case-level results belong in `tests/manifest.json`; temporary runner logs
are revision evidence, not a second expected-outcome registry.
