# Interpretation records separate from approved corrections

The [approved correction layer](corrected-reading.md) applies published XSD
1.0 errata. The records here explain source questions that arise during
corpus review. Historical acquisitions have role
`interpretation-history-not-erratum` in [the provenance manifest](source-manifest.json);
the snapshots are not bundled.
A closed issue is not, by itself, a new normative XSD 1.0 amendment.

## I-001 — complex types with simple content as identity fields

The published [identity rule](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-identity-constraint)
says the selected field node has a simple type; nearby prose speaks of its
content/value being simple. [W3C issue 10236](https://www.w3.org/Bugs/Public/show_bug.cgi?id=10236)
records the resulting ambiguity and the WG's earlier conclusion that simple
content of a complex type was intended to participate. The issue is marked
for both 1.0 and 1.1; its 2010 resolution adds a clarification entry to the
1.1 change list, rather than publishing an additional Second Edition erratum.

Concrete pair: an integer `v` field and a `v` with complex simple content
extending integer, both containing `1`. The documented WG interpretation
admits both as value-bearing fields. An element-only complex `v` containing
an `x` child is a distinct nonsimple negative. Do not score a literal
1.0-text rejection of the simple-content case as a demonstrated normative
defect without preserving this interpretation boundary. Record the intended
valid result as **WG clarification**, separate from an unambiguous published
1.0 requirement.

## I-002 — fields inside a skipped subtree

[Issue 1937](https://www.w3.org/Bugs/Public/show_bug.cgi?id=1937) is explicitly version **1.1 only**.
Its adopted resolution treats the skipped subtree as absent for identity
processing. That version tag matters: it cannot silently add an XSD 1.0
exception to the published selector, node-cardinality, and simple-value
conditions.

Concrete pair: a selector or field reaches `v` beneath a `processContents=skip`
element, versus the same `v` assessed through a declaration. Preserve the
actual typed-value/assessment facts for each. A skipped node is not a
type-valid empty string. Keep the XSD 1.0 consequence source-qualified where
the necessary value is unavailable; do not use the 1.1 skip exemption as
proof of a mandatory 1.0 acceptance result.

## I-003 — nilled field values

[Issue 2219](https://www.w3.org/Bugs/Public/show_bug.cgi?id=2219) asks whether nilled selected elements
count as missing values. Its disposition explicitly describes the accepted
**1.1** behavior: nil acts as a missing field for `unique`, whereas a key
still needs field values. [Issue 10236](https://www.w3.org/Bugs/Public/show_bug.cgi?id=10236) relates
that history to the later identity-rule revisions. Neither record is listed
as a new approved 1.0 Second Edition erratum.

Concrete distinctions: absent `v`; nonnil string `<v/>`; and string `v`
with `xsi:nil=true`. XPath can select the nilled **node**, even though E1-56
gives it no schema-normalized value. Calling that selection an empty node
set conflates two different predicates. Keep the 1.0 qualified-value interpretation visible. Independently, a key
field assessed through a nillable declaration fails the published key
condition even if its instance is not nilled.

## I-004 — published member-minimum rule

**Status:** published XSD 1.0 oracle resolved; acknowledged bug and proposed
repair remain interpretation history.

[NSRecurseCheckCardinality](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-NSRecurseCheckCardinality)
requires both recursive restriction of every immediate member against the
original wildcard particle and restriction of the group's effective total
range. For an element member, [NSCompat](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-NSCompat)
invokes [Occurrence Range OK](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#range-ok), including
the original base minimum. Passing the aggregate check does not waive that
member check. The rule does not replace the base minimum with zero.

[R-240](https://www.w3.org/2001/05/xmlschema-rec-comments.html#pfiNSRecurseCheckCardinality) identifies
this consequence and proposes such a replacement. Thompson's
[6 January 2004 reply](https://lists.w3.org/Archives/Public/www-xml-schema-comments/2004JanMar/0000.html) confirms the
particle reading and acknowledges a bug. This is evidence of intended repair,
not published replacement text. The published Second Edition still prints both
checks; the [approved errata](https://www.w3.org/2004/03/xmlschema-errata.html) do not
amend this rule.

For this repository's published-1.0 oracle, a member minimum below the base
minimum fails even when the aggregate fits. An all group with required a and
optional b against wildcard1..2 is a minimal distinction: total1..2 fits,
but b0..1 does not. Sequence and repeated-choice consumers must likewise
separate member and aggregate bounds. Ha070, Ha080 and R013 retain their
upstream valid provenance in `schema.oracle.w3c`; their effective schema
expectation is invalid under this baseline. R013's retained instance supplies
no validation credit under the invalid-schema expectation.

Report acceptance as divergence from the published rule, qualified by the
acknowledged bug and interoperability history. Do not present acceptance as
proof of an approved repair, or literal rejection as contrary to published
XSD 1.0. This interpretation record adds no erratum.

## I-005 — implicit substitution-choice order

**Status:** underspecified in published XSD 1.0; proposed order waiver is not
an approved amendment.

[Particle restriction 2.1](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-particle-restrict)
constructs a choice from an actual substitution-group set without assigning
its list order. [RecurseLax](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-RecurseLax)
requires an ordered mapping. [R-122](https://www.w3.org/2001/05/xmlschema-rec-comments.html#pfiSubGrpRestrict)
records this problem and an instruction to draft an erratum; Thompson's
[14 February 2002 reply](https://lists.w3.org/Archives/Public/www-xml-schema-comments/2002JanMar/0512.html)
proposes waiving order for implicit choices. The register's drafting status
is distinct from an approved correction. Neither the published Second Edition
nor its approved errata supplies that waiver or a canonical ordering.

Do not infer declaration order, QName sorting, a shared generated order, or
existential reordering from processor behavior. Order-sensitive implicit-choice
cases remain without a unique published-1.0 expectation. Explicit source
choices still have their printed order and ordinary RecurseLax requirements.

Source-supported executable witnesses can avoid the missing premise.
[MapAndSum](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-MapAndSum) does not require an
ordered target mapping, so either explicit sequence order can restrict a
matching repeated generated base choice. A unit generated singleton also
normalizes without an ordering decision. For such witnesses, prove the same
outcome for every relevant enumeration. This is a method of choosing safe
examples, not a new universal-order validity rule or a required processor
algorithm. Actual membership, filtering, ranges and other derivation rules
still apply.

## Remaining source-integration boundaries

These are qualifications, not alternative permitted policies or approved
repairs. Success/failure alone cannot observe disputed PSVI/component properties.
Source IDs are retained for traceability; fixture outcomes remain in the corpus manifest.

| Record | Source and retained boundary |
| --- | --- |
| CTS-Q001 | Structures [type annotations](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#ct-annotations), [XML mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-type) and [absence convention](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#key-null): required annotation set versus absent no-annotation mapping. Do not invent an annotation, equate absence with empty or reject an otherwise admitted constructor to conceal it. |
| ASQ001 | [Element validity outcome](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-validity) lacks the connective between nested declaration/type-route clauses. Ordinary applicable-route readings remain conditional; no approved AND/OR repair or blanket rejection follows. Descendant and assessment-attempted conditions remain independent. |
| IBS-Q001 — root ID eligibility and successful declaration validation | [Eligibility](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#key-eas) depends on successful declaration validation; [bindings](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#iib-binding) use ID-valued attributes/children, while [cvc-id](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-id) forbids empty bindings. A directly ID-typed root's own value can create an ownerless key conditional on that same success. Preserve the success/contribution question; invent no phase, root-self owner or blanket ID-element rejection. An independently eligible IDREF can create an unambiguously empty binding. |
| IDS-Q001 | Keyref [refer mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-key) invokes generic [QName resolution](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-resolve), whose kind list omits identity constraints. Keep the intended referenced component explicit; category, arity, identity and tuple rules apply once supplied. No reject-all-keyrefs rule or unpublished indexing algorithm follows. |
| XPS-Q001 / XPQ001 | [Selector](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-selector-xpath)/[field](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-fields-xpaths) components allow whitespace/expanded axes more broadly than normative Appendix A patterns. Distinct component and XML-representation phases must both be reconciled; component permission alone does not prove XML spelling admission. |
| ANQ001 | Dated [documentation mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#element-documentation) and XML Second Edition support nonempty xml:lang; Appendix A imports an XML declaration. The acquired 2009 xml.xsd allows empty but does not establish the 2004 imported body. Preserve source selection, not generic XML rejection or automatic later-version acceptance. |
| CMP-Q001 / ICP-Q001 | An unused [import](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-import) with present-empty namespace has anyURI syntax but unresolved semantic-domain scope relative to the empty targetNamespace prohibition. Empty is not absent; lexical admission and existing corpus expectations do not settle acceptance. Ordinary obtained-referent matching still applies. |
| CMP-Q002 | Attribute-group [redefine clause 7.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-agrere) uses ordinary mapping, including nonself group contributions; its note describes explicit-only children. No inheritance from the original group is settled. The note alone does not justify rejecting otherwise matching nonself references. |
| DCS-Q001 | Local [attribute mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-attribute) puts a constraint on the use, while declaration [canonical/ID rules](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#a-props-correct) name a different property. [Use correctness](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#au-props-correct) does not expressly transfer both rules. Preserve the ID-default and boolean-pattern/canonical-literal probes as source-qualified, not universal acceptance/rejection. Literal/type admission and present-attribute fixed canonical-string checks still apply. DTD ID-default rules and E1-56 do not bridge these properties. |
| PSVI-Q001 | E1-56 patches lightweight union-member paragraphs, leaving earlier full-member wording and contributor entry premises intact. A member can match while a governing union facet fails; that attribute boundary has no unique full-member oracle here. Invalid complex attributes defeat the element contributor's type-validity premise; declaration-only fixed/identity failures need not. See [corrected reading](corrected-reading.md). |
| B-044 | [Attribute synthesis](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#sic-attrDefault) predicates contribution on a use constraint but refers to an effective constraint allowing declaration fallback. Preserve absent-use/global-default integration separately from explicit-use synthesis. Added type/member metadata is a recommendation at this entry; independently applicable type-validation duties retain their own premises. |
| A-106 | Under both head exclusions, formal [element/type derivation](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-props-correct) admits type identity while §3.3.1 nomination prose forbids any affiliation. Same-type affiliation remains source-qualified; a prohibited nonempty derivation is a separate negative. |
| Anonymous identity / wildcard algebra | The [component identity note](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#no-identity) leaves some anonymous identity choices incomplete. [Multi-value wildcard algebra](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-aw-union) requires expressible pairwise intermediates without a complete operand order; a mathematically expressible final set does not prove a chosen fold valid. |

## Datatype and observation qualifications

The [published Datatypes](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html)
and [source editions](SOURCES.md) supply the following boundaries retained from
the research notes. Unresolved does not mean arbitrary processor choice.

| Boundary | Oracle treatment |
| --- | --- |
| RXQ001 / RGQ001 | The Char production omits braces from exclusions while normal-character prose calls them metacharacters; prose omits alternation bar while grammar excludes it. Bare-brace regex oracles remain unresolved. Escaped literals and legal quantifiers are distinct determined cases. |
| RXQ002 | Record Unicode version and baseline versus encouraged later support for changed categories/later blocks. A later block is not automatically a mandatory rejection. |
| Canonical zero and union overlap | Generic one-to-one canonical representation needs integration with decimal/integer minus-zero wording and overlapping integer/decimal union members. Conventional outputs are not disproved; two policies are not thereby permitted. Explicit nonPositiveInteger and float/double zero rules remain determined. Acceptance/member metadata cannot prove an ambiguous canonical output. |
| Unsigned aliases / single-member union / gMonth | Complete Appendix A unsigned definitions supply signed integer aliases with bounds; preserve the prose seam. Concrete component/representation rules allow one union member despite informal “greater than 1”. Published gMonth form is `--MM`; historical deleted `--MM--` is excluded. |
| Temporal boundaries | Preserve leap-second lexical/body/Appendix D premises; Appendix E treats an admitted leap second as arithmetic overflow but does not settle all lexical admission. BCE/CE arithmetic needs explicit historical-year mapping. Cross-reference time/g* rollover identity needs a source interpretation, not clock-only equality or an invented canonical g* algorithm. Mixed-zone dateTime separation exactly 14 hours is incomparable under the explicit opening rule. Date example sign/direction discrepancies do not override operative offset/interval definitions. |
| Duration / truncation prose | Duration's total-order paragraph refers to a timezone that its grammar lacks; invent no feasible timezone branch. Appendix D's right-truncation list omits gYearMonth despite its explicit type definition; specific grammars still govern. |
| Fundamental facets — FFP004/006/008, FM13 | Preserve printed property formulas separately from mathematical value-set cardinality: fixed-length integer lists classify finite; enumeration/pattern alone do not trigger atomic finite classification; Appendix C labels gMonth countably infinite. Union ordered explicitly excludes anySimpleType as common ancestor while bounded does not; add no unprinted common-primitive exclusion. Boolean validity cannot expose these component properties. |
| Bounds | Repeated equal exclusive endpoints have explicit allowance; do not substitute generic instance validity. Incomparable restriction endpoints need their actual prohibited-comparison wording. Inherited opposite lower-bound kinds retain the same-datatype versus same-step scope question; no blanket rewrite/rejection follows. |
| URI / language / precision | No implicit percent decoding, case folding, normalization or absolutization establishes anyURI equality. No modern language registry rejection follows from its historical pattern. [Double extrema, overflow and documented precision](SOURCES.md#floating-point-source-versions) remain separately qualified. |
| Observation / product / modality | Missing public canonical/PSVI/component output is unobservable, not covered. Product exclusions and resource/precision refusals are not datatype-invalid oracles. Compatibility recommendations, discouraged constructs and generic error/recovery permissions do not waive explicit must/conformance requirements or create rejection rules. |

Required XSD 1.0 result, WG interpretation, XSD 1.1 change, processor choice,
unspecified behavior and unavailable observation remain distinct evidence
classes. Corpus deviations at these seams must cite the published rule and
its qualification; neither the implementation nor another validator is the oracle.
