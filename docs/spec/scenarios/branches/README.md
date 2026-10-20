# Concrete branch scenarios

This layer instantiates the [scenario families](../README.md) with explicit
types, component sketches, lexical values, XML shapes, mutations, and expected
outcomes. Source-authority and [attribution](../../NOTICE.md) remain those of
the [corrected reading](../../corrected-reading.md). No implementation or
corpus behavior defines an oracle here.

| File | Owned source decisions | Case prefix |
| --- | --- | --- |
| [Declarations](declarations.md) | Structures foundations and §§3.1–3.3: attributes, elements, assessment, defaults, nil, substitution, and PSVI. | A |
| [Content and derivation](content-and-derivation.md) | Structures §§3.4–3.10: complex types, uses/groups, content, particle restriction, and wildcard algebra. | B |
| [Composition and identity](composition-and-identity.md) | Structures §§3.11–5: identity constraints, notations, annotations, simple-type contracts, resolution, include/redefine/import, and assessment. | C |
| [Datatypes](datatypes.md) | Part 2 construction, builtins, facets, value relations, temporal algorithms, and regex productions. | D2 |

Each case ID names a group of explicitly stated variants. It does not count
as one executed fixture. Shared sketches define unchanged preconditions;
variants must preserve them so an earlier unrelated failure does not mask
the intended predicate. Abstract component relations, local validity,
whole-schema construction, PSVI, and processor choices are separate observing
boundaries. A failed component-relation predicate is not automatically a
reachable invalid XML schema at that exact boundary.

## Source dispositions

The tables account for every baseline `numbered_branches` key in
[inventory.json](../../inventory.json). A suffix such as `/branch/003` is
a source-order identifier; the actual normative number remains in the source.
PSVI properties can restart their numbering within one named contribution.

| Disposition | Meaning |
| --- | --- |
| `reviewed` | A concrete scenario description and expected consequence are supplied. This does not prove semantic completeness or execution. |
| `composite` | A parent conjunction, alternative, or case selection uses the listed distinguishing child witnesses. |
| `superseded` | An obsolete baseline numbered case is replaced by E1-56; use the corrected cases, not the old assertion. |
| `informative` | The source is explicitly non-normative, or the current source retains an empty numbering slot with no operative condition. |
| `question` | The listed scenario retains an unresolved source interpretation. It is indexed, not a settled pass/fail oracle. |

Other source questions also occur inside case groups: the number of rows
marked `question` is not an inventory of all possible ambiguities. In
particular, preserve [the identity interpretation records](../../INTERPRETATIONS.md)
and the [source-question records](../source-questions.md), alongside the local
question sections in each file. WG clarifications and fixes
expressly made for XSD 1.1 are not silently additional approved 1.0 errata.

## Verify structural closure

Check exact branch-key closure and file ownership, unique case definitions,
references to defined local cases, valid dispositions and reasons, the
superseded E1-56 cases, source classification for informative rows and local
Markdown citations. Verify source hashes and compare the inventory against
the retained publications and their independent Appendix C constraint tables.
An unrelated but existing case can still have the wrong semantic expectation;
structural closure cannot replace source review.

## Remaining comparison boundaries

The concrete layer includes unnumbered conditions and property/grammar
examples, but complete expansion of all Appendix A contexts and dependent
interaction products remains an additional audit obligation. External XML,
URI, Unicode/version, and precision rules need their own source-qualified
partitions where they affect XSD. No corpus case is marked covered merely
because one of these descriptions or mappings exists. Use the
[later comparison contract](../README.md#later-corpus-comparison-contract)
to distinguish demonstrated, partial, masked, missing, unobservable,
product-excluded, and unresolved-oracle evidence.
