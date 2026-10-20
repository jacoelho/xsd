# Specification-derived scenarios

These catalogues derive questions and expected behavior from the
[corrected XSD 1.0 reading](../corrected-reading.md). They do not use existing
implementation behavior, test names, or corpus size as the oracle.

| Catalogue | Scope |
| --- | --- |
| [Structures](structures.md) | Components, XML representation, derivation, content, composition, assessment, PSVI, and appendices A–J. |
| [Datatypes](datatypes.md) | All builtins, construction, facets, equality, ordering, time/duration algorithms, regex grammar, and appendices A–I. |
| [Schema grammar](grammar.md) | Parameterized syntax obligations for every applicable path in both dated Appendix A grammars. |
| [Concrete grammar contexts](grammar-contexts.md) | Resolved helper/inheritance contracts, context-specific witnesses, and ownership of every dated Appendix A inventory node. |
| [Dependent interactions](interactions.md) | Finite products of interacting decisions, admissibility exclusions, outcome rules and observing witnesses. |
| [External rules](external-rules.md) | Concrete XML, namespace, XPath, URI, Unicode, language and numerical dependency boundaries using the retained editions. |
| [Primer and boundaries](primer-and-boundaries.md) | Primer features/appendices and the XML, namespace, URI, and processor boundaries on which XSD depends. |
| [Approved correction](../corrected-reading.md) | E1-56 changes and their distinguishing PSVI scenarios. |
| [Source inventory](../inventory.json) | Published sections, named constraints, numbered branches, property definitions, representation summaries, regex productions, and appendix grammar nodes. |
| [Concrete branch scenarios](branches/README.md) | Explicit component/XML/value examples and dispositions for every numbered baseline branch, with source questions retained. |
| [Source questions](source-questions.md) | Known unresolved source premises and links to the local catalogues that own existing questions. |

Every named constraint has a family mapping, and every numbered baseline
branch has a concrete-case disposition. Those checks establish description
and reference closure. The context and interaction catalogues expand the
remaining parameterized obligations; their described witnesses are not
executed fixtures or semantic completeness proofs. Executable fixtures and
the corpus comparison are separate evidence. Do not describe a family or
case link as established corpus coverage.

## Finite descriptions of unbounded inputs

XML documents, arbitrary-precision values, nesting depths, and repetition
counts have infinitely many spellings and sizes. “Comprehensive” therefore
means an explicit rule and finite equivalence/boundary partitions, with
parameterized families where counts or values are unbounded. It does not
mean enumerating every possible document.

For each source decision, record:

1. **Authority:** part, exact clause/property/production, and applicable erratum.
2. **Preconditions:** schema context, available components, input infoset,
   namespace bindings, and any processor choice relevant to the decision.
3. **Partitions:** each alternative, each premise independently false,
   absent/present/empty distinctions, and below/at/above every bound.
4. **Expected result:** schema correctness, local validity, assessment result,
   typed value/equality/order, normalization, or specific PSVI contribution.
5. **Witness:** a minimal satisfying case plus distinguishing counterexamples;
   isolate the premise so an earlier failure does not mask the rule.
6. **Interactions:** the complete set of dependent axes necessary to decide
   this rule; remove infeasible combinations with a reason.
7. **Observation:** the boundary that exposes the result, including cases a
   validity-only API cannot distinguish.

An AND rule needs the all-true case and each relevant failing conjunct. An OR
rule needs each satisfying alternative and the all-false case. An ordered
case distinction needs overlapping premises to prove priority. A recursive
rule needs the base case, a nested case, and the same rule reached through
each recursive alternative. Do not assume those simple witnesses prove an
interaction between multiple rules.

For a finite enumeration, instantiate every member. For an integer bound,
use below/at/above, zero, omitted/defaulted, and unbounded where admitted.
For a Unicode class, use members/nonmembers and each defined range edge,
including supplementary-plane characters admitted by XML. For decimal or
temporal ordering, include equality, both ordering directions, and
indeterminacy where the value space is partially ordered.

## Keep different outcomes distinct

| Outcome | Meaning for the later audit |
| --- | --- |
| Required valid / required invalid | A binding rule determines schema or instance validity under stated premises. |
| Required contribution | A binding PSVI/type/value fact; success/failure alone may not observe it. |
| Permitted choice | The specification permits multiple behaviors; record the selected behavior and check the complete permitted set. |
| Unspecified | No unique oracle follows from XSD 1.0. Record the exact source boundary instead of inventing a pass/fail rule. |
| External precondition | XML/Namespaces/Infoset or another referenced standard owns admission before XSD assessment. |
| Outside the product contract | A specification scenario remains in the catalogue even when an explicit repository capability does not expose it. |
| Resource/operational failure | A bounded implementation or failed source access did not establish semantic invalidity. |

An empty derived value space, an unresolved component, an unknown governing
declaration, a rejected schema, and an invalid instance are different facts.
Likewise, nil, missing, empty lexical text, whitespace-only text, empty list,
absent PSVI property, and defaulted value are not interchangeable.

## Required interaction strategy

Expand a rule's **dependent** axes fully. Pairwise sampling does not replace
the branch structure when, for example, nil/default/type/content, union-member
selection/facets/namespace context, or restriction/compositor/cardinality/
substitution jointly determine the result. The Structures and Datatypes
catalogues name the required semantic products.

Also use metamorphic companions where the specification justifies them:
rename prefixes consistently; replace lexical spellings with equal values;
reorder `all` children; split or join XML character delivery without changing
the infoset; permute independent declarations; and replace a source reference
by an equivalent explicit component set. Each transformation needs its own
premises. For example, reordering union members or QName namespace bindings
is not generally semantics-preserving.

## Later corpus comparison contract

Keep the specification catalogue independent. A subsequent comparison will
map each concrete scenario to actual schema/instance content and effective
expectations from [the corpus manifest](../../../tests/manifest.json), using
[the harness contract](../../../tests/README.md). Filename or description
similarity is a search lead, not coverage evidence.

The evidence record needs scenario ID/parameters, fixture paths and manifest
case IDs, admitted roots/dependencies, expected schema outcome, expected
instance outcome, observable seam, and any masking error. Classify coverage
as demonstrated, partial, missing, not observable, outside the chosen product
contract, or unresolved oracle. Keep invalid-schema cases separate from
invalid-instance cases and preserve upstream/oracle provenance.

`unsupported.txt` describes deliberate harness skips, not specification
exceptions. Schema-less cases and instance-directed discovery must remain
visible even though this repository requires explicit schema sources. A
test that passes for the wrong earlier error does not establish the later
scenario. Do not infer a corpus gap solely from a missing diagnostic code:
one valid fixture can exercise several rules, and one invalid fixture can
mask several apparent targets.

## Maintaining traceability

Verify retained source hashes and lengths against the source manifest. Check
the inventory against the publications and their independent Appendix C
constraint tables. Keep named-constraint mappings, branch dispositions, case
definitions and local citations consistent, including nested catalogues.
Check grammar-context ownership against the dated inventory, bounded selector
ranges and unique GC/IP/XR definitions.

A source change requires a reviewed provenance update, renewed errata check,
and renewed semantic analysis. Updating the inventory alone cannot approve a
new oracle.
