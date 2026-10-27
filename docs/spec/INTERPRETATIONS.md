# Interpretation records separate from approved corrections

The [approved correction layer](corrected-reading.md) applies published XSD
1.0 errata. The records here explain source questions that arise during
corpus review. Historical acquisitions have role
`interpretation-history-not-erratum` in [the provenance manifest](source-manifest.json);
the snapshots are not bundled.
A closed issue is not, by itself, a new normative XSD 1.0 amendment.
Selected corrections below follow the
[selected-correction policy](SOURCES.md#selected-corrections). Their adoption
does not claim publication as Second Edition errata. The
[correction audit](XSD11_CORRECTIONS.md) records scope and opposing witnesses.

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
an `x` child is a distinct nonsimple negative. **Status:** adopted WG clarification, already implemented. Both kinds of
simple content supply typed field values. The corpus pairs equal and distinct
integers across these declarations and retains the nonsimple negative. This is
an explicit correction to the literal baseline, not an additional published
Second Edition erratum.

## I-002 — fields inside a skipped subtree

**Status:** explicitly adopt [issue 1937](https://www.w3.org/Bugs/Public/show_bug.cgi?id=1937)
and the final [identity rule](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#cvc-identity-constraint)
as a selected semantic correction. The issue is tagged 1.1 only; this is not a
claim that the published 1.0 cardinality rule already contains the exemption.

Skipped selector nodes are absent from the target set. Skipped field nodes
supply neither values nor cardinality contributions. This includes attributes
admitted by a skip wildcard and all descendants/attributes under an element
admitted by skip. An assessed selected row still needs complete field values
for `key`; unique/keyref omit incomplete tuples. Any number of skipped fields
may accompany one assessed typed field, but two assessed nodes remain invalid.

Skip already excludes descendant assessment, so the correction removes no
valid typed descendants. XML syntax and namespace checks still apply. Lax
missing declarations and recovery are distinct states and retain their rules.
The four intrinsic XSI attributes on an assessed element remain typed even
with a skip attribute wildcard; attributes under a skipped element do not
contribute identity values. Treating absent type information as an empty
string remains forbidden.

## I-003 — nilled field values

**Status:** adopted value-absence correction from
[issue 2219](https://www.w3.org/Bugs/Public/show_bug.cgi?id=2219), qualified
against the final [identity rule](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#cvc-identity-constraint).

A selected nilled simple/simple-content element has no field value. Its tuple
is incomplete: `unique` and `keyref` omit it, while `key` requires a value.
A nonnilled empty string remains a value and can collide with another empty
string. Selecting a nilled node still counts for field cardinality; selecting
it and another node for one field is invalid. Nilled element-only complex
content does not gain a simple type through this correction.

The final 2012 key rule still excludes fields assessed through nillable element
declarations, even for nonnilled instances. That declaration rule remains in
force. Physical attribute fields such as `@xsi:nil` retain their typed values.
The separate skipped-subtree adoption is in I-002. Neither is a new approved
XSD 1.0 erratum.

## I-004 — published member-minimum rule

**Status:** adopt the bounded repair in
[R-240](https://www.w3.org/2001/05/xmlschema-rec-comments.html#pfiNSRecurseCheckCardinality),
not the XSD 1.1 replacement restriction algorithm.

Published [NSRecurseCheckCardinality](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-NSRecurseCheckCardinality)
compares every immediate member with the original wildcard, as well as checking
the aggregate range. The WG acknowledged that the original member minimum can
reject a group whose aggregate satisfies the wildcard. The Second Edition and
its approved errata do not publish the proposed replacement.

The library compares immediate members with a temporary copy of the wildcard
whose minimum is zero. Its maximum, namespace constraint and processing
strength remain intact. The group aggregate must still restrict the original
wildcard range. This admits an all group with required `a` and optional `b`
against wildcard 1..2, while retaining aggregate and namespace opponents.
The selected correction supersedes earlier literal-baseline invalid overrides;
previous observations remain provenance only for unchanged inputs. Validator
acceptance does not waive independent recursive wildcard restrictions.

## I-005 — implicit substitution-choice order

**Status:** adopt the generated-choice order waiver proposed in
[R-122](https://www.w3.org/2001/05/xmlschema-rec-comments.html#pfiSubGrpRestrict)
and Thompson's [2002 reply](https://lists.w3.org/Archives/Public/www-xml-schema-comments/2002JanMar/0512.html).
This is an explicit library interpretation, not published Second Edition errata.

Published substitution expansion builds a choice from an unordered set, while
RecurseLax asks for ordered mapping. The waiver applies to a generated choice
on either operand. Explicit source siblings retain their order, including when
pointless-group removal flattens a generated choice beside them. A waiver must
not spread to an enclosing explicit choice or permit crossing its boundaries.

The comparison view retains contiguous generated-member spans. Functional
RecurseLax matching can share a target within a waived comparison; it does not
acquire RecurseUnordered's separate injectivity rule. Membership, occurrence,
block/final, type and wildcard restrictions remain independent. Source-order
and declaration-order opponents protect this boundary. The bounded matching
implementation and work accounting belong to [the architecture](../../ARCHITECTURE.md),
not to the historical proposal.

## I-006 — XSD/XSI schema references

**Status:** adopted XSD 1.0 correction from W3C issues 1890 and 4363.

Printed Second Edition [QName resolution](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-resolve)
requires the containing target namespace or an import, without an express
built-in exception. W3C [issue 1890](https://www.w3.org/Bugs/Public/show_bug.cgi?id=1890)
and [issue 4363](https://www.w3.org/Bugs/Public/show_bug.cgi?id=4363) explicitly
apply to both 1.0 and 1.1. The Working Group adopted import-free access for the
entire XSD and XSI namespaces in May 2007. The rule appears in
[XSD 1.1 QName resolution](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#src-resolve),
but not in the published Second Edition errata. The library deliberately adopts
this correction under the policy in [SOURCES.md](SOURCES.md#selected-corrections).

A schema reference into either namespace needs no import. Ordinary component
lookup still enforces the requested kind and expanded name; missing components
retain their ordinary error or unavailability policy.
A supplied custom component in the XSD namespace therefore needs an explicit
source but no import; matching a built-in type's local name in another symbol
space cannot select that type. Canonical builtins remain immutable, and supplied
XSI attribute declarations remain forbidden. The four intrinsic XSI declarations
are available without obtaining an XSI schema document. Unknown XSI attribute
references still fail lookup.

XML, XLink and other foreign namespaces retain ordinary import authorization
and explicit-source assembly. An explicit XSD/XSI import retains ordinary
source-resolution, namespace-matching, failure and budget rules. Neither the
reference exemption nor intrinsic availability authorizes HTTP acquisition or
instance-directed component loading.

## I-007 — XSI attribute uses and PSVI contributions

**Status:** adopted XSD 1.0 defaulting correction from W3C issue 4363;
physical assessment and required presence retain their existing rules.

The [XSI declaration restriction](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#no-xsi)
forbids supplied XSI attribute declarations. The intrinsic declarations have no
value constraints. Their types are QName (`type`), boolean (`nil`), anyURI
(`noNamespaceSchemaLocation`) and an anonymous list of anyURI (`schemaLocation`).
A reference can carry a default/fixed constraint; construction still validates
that constraint against the declared datatype and source representation rules.
For example, `xsi:nil` with `default="bogus"` is a schema error, and a required
use cannot specify a default. QName literals need only be valid bound QNames;
ignored `xsi:type` constraints do not cause type lookup or element-start processing.

Printed [Attribute Default Value](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#sic-attrDefault)
has no XSI exclusion. [Issue 4363, comment 4](https://www.w3.org/Bugs/Public/show_bug.cgi?id=4363#c4)
identifies that PSVI consequence; comment 7 records the June 2007 decision that
XSI default/fixed values are neither applied nor checked. The library adopts its
defaulting exclusion, as expressed in
[XSD 1.1](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#sic-attrDefault),
without adopting unrelated 1.1 features. Absent XSI attributes contribute no
synthesized identity value, type selection, nil state or location hint.

[Complex-type validity clause 3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-aam)
excludes exactly these four names from ordinary attribute-use checking, so a
physical value does not acquire a use's fixed-value check. Its intrinsic lexical
and element-start rules still apply. Clause 4 independently requires physical
presence for every required use. A physically present invalid value still counts
as present. Names outside the four intrinsic declarations follow ordinary
attribute and wildcard assessment, including skip wildcards.

Physical intrinsic attributes participate in typed identity checks.
[Element assessment](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-assess-elt)
assesses associated attributes; [attribute assessment](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-assess-attr)
can resolve their intrinsic declarations without a local use. Thus `@xsi:nil`
compares boolean values, `@xsi:type` compares expanded QNames, and
`@xsi:schemaLocation` compares ordered URI lists. The key rule excluding nillable
element fields does not exclude attribute fields on a nilled element.

## I-008 — prohibited attribute use with fixed

**Status:** published XSD 1.0 mapping; conflicting upstream expectation retained
as provenance.

Both local-declaration and reference branches of
[attribute mapping §3.2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-attribute)
say that `use="prohibited"` corresponds to no component. Neither branch makes an
exception for `fixed`. The [representation constraint](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-attribute)
separately forbids `default` unless use is optional; it does not turn a
prohibited fixed source item into an optional use.

The conditional mapping also supplies no declaration type, fixed value or
reference target to validate for that item. The compiler therefore skips
target lookup, declaration-name restrictions and datatype/fixed compatibility
checks. Source syntax and representation constraints remain mandatory, including
QName lexical/binding admission, `name`/`ref` exclusivity, type-source exclusivity
and default placement. An inline `simpleType` independently maps to a component
and retains its own constructor, reference and facet checks. Global declarations
remain independently validated even when referenced only by prohibited items.

The library therefore creates no use or synthesized value for a prohibited
fixed item. Under complex-type restriction its name filters only inherited base
uses; it cannot erase an explicit local or group use. Under extension it cannot
remove an inherited use. Attribute groups export actual uses, not prohibition
directives. These effects are independent of source order. An actual ordinary
attribute is assessed through any other applicable use or wildcard; without either it fails. A skip wildcard can admit
it without checking the discarded fixed value or datatype. Intrinsic XSI
attributes retain their independent instance rules and also receive no
synthesized value from the discarded item.

W3C `msMeta/Attribute_w3c.xml/attP031` marks its instance valid even though the
actual attribute has only a prohibited fixed source item and no wildcard.
The corpus expects invalid under the published mapping and preserves the
original valid result in `instance.oracle.w3c`, with its reason and source.
The upstream result is interoperability evidence, not replacement normative
text. An absent attribute remains a distinct valid witness.

## I-009 — fixed attribute-use values

**Status:** adopt typed-value equality from the later
[Attribute Use Locally Valid rule](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#cvc-au).

The published 1.0 use rule compares normalized spelling with a canonical lexical
form; the declaration rule already compares values. Ordinary use-owned fixed
constraints now use the same typed equality. Integer `+001` equals `1`, boolean
`1` equals `true`, and ordered lists compare item values. Type facets still
validate the supplied spelling first. String whitespace, list order, unequal
numbers and QName namespace differences remain observable negatives. XSI's
separate intrinsic assessment and no-synthesis policy remain in I-007.

This does not transfer declaration-only ID-default prohibitions to attribute
uses or solve every canonical-projection question in DCS-Q001.

## I-010 — restricted unions retain their constraints

**Status:** adopt the restricted-union correction identified in Structures
[G.1.8](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#changes)
and [simple derivation](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#cos-st-derived-ok).
The [pure-union definition](https://www.w3.org/TR/2014/REC-xquery-30-20140408/#dt-pure-union-type)
also distinguishes constrained intermediate unions.

An effective pattern or enumeration restriction cannot be bypassed by using a
member type as `xsi:type` or as a substitution-group member's type. Ordinary
identity and restriction through the declared base chain remain valid. A
facet-free restriction of a constrained union still inherits its constraints.
Nested union construction preserves such a constrained node instead of
flattening away its facets. Facet-free unions retain ordered, deduplicated
flattening, and an independent unconstrained alternative remains usable.

## I-011 — regex metacharacter grammar

**Status:** adopt the brace/bar consistency repair. The
[F&O 3.1 correction note](https://www.w3.org/TR/2017/REC-xpath-functions-31-20170321/#regex-syntax)
explicitly identifies missing braces as an XSD 1.0 error.

Bare braces are metacharacters, escaped braces are literals, and unescaped bar
introduces alternation. The existing parser already implements this reading.
This does not import XPath regex anchors: `^` and `$` remain literal XSD
characters. Unicode versions and other XPath regex extensions are independent.

## Remaining source-integration boundaries

These are qualifications, not alternative permitted policies or approved
repairs. Success/failure alone cannot observe disputed PSVI/component properties.
Source IDs are retained for traceability; fixture outcomes remain in the corpus manifest.

| Record | Source and retained boundary |
| --- | --- |
| CTS-Q001 | Structures [type annotations](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#ct-annotations), [XML mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-type) and [absence convention](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#key-null): required annotation set versus absent no-annotation mapping. Do not invent an annotation, equate absence with empty or reject an otherwise admitted constructor to conceal it. |
| ASQ001 | [Element validity outcome](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-validity) lacks the connective between nested declaration/type-route clauses. Ordinary applicable-route readings remain conditional; no approved AND/OR repair or blanket rejection follows. Descendant and assessment-attempted conditions remain independent. |
| IBS-Q001 — root ID eligibility and successful declaration validation | [Eligibility](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#key-eas) depends on successful declaration validation; [bindings](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#iib-binding) use ID-valued attributes/children, while [cvc-id](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-id) forbids empty bindings. A directly ID-typed root's own value can create an ownerless key conditional on that same success. Preserve the success/contribution question; invent no phase, root-self owner or blanket ID-element rejection. An independently eligible IDREF can create an unambiguously empty binding. |
| IDS-Q001 | The later `src-resolve` list repairs the omitted identity-component kind. Adopt that clarification, already implemented: keyrefs resolve to supplied key/unique definitions; wrong category, missing target, arity and tuple checks remain. Existing `idH035` protects keyref-to-keyref rejection. No new indexing algorithm follows. |
| XPS-Q001 / XPQ001 | [Selector](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-selector-xpath)/[field](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-fields-xpaths) components allow whitespace/expanded axes more broadly than normative Appendix A patterns. Distinct component and XML-representation phases must both be reconciled; component permission alone does not prove XML spelling admission. |
| ANQ001 | Dated [documentation mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#element-documentation) and XML Second Edition support nonempty xml:lang; Appendix A imports an XML declaration. The acquired 2009 xml.xsd allows empty but does not establish the 2004 imported body. Preserve source selection, not generic XML rejection or automatic later-version acceptance. |
| CMP-Q001 / ICP-Q001 | An unused [import](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-import) with present-empty namespace has anyURI syntax but unresolved semantic-domain scope relative to the empty targetNamespace prohibition. Empty is not absent; lexical admission and existing corpus expectations do not settle acceptance. Ordinary obtained-referent matching still applies. |
| CMP-Q002 | Attribute-group [redefine clause 7.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-agrere) uses ordinary mapping, including nonself group contributions; its note describes explicit-only children. No inheritance from the original group is settled. The note alone does not justify rejecting otherwise matching nonself references. |
| DCS-Q001 | Local [attribute mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-attribute) puts a constraint on the use, while declaration [canonical/ID rules](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#a-props-correct) name a different property. [Use correctness](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#au-props-correct) does not expressly transfer both rules. Preserve the ID-default and boolean-pattern/canonical-literal probes as source-qualified, not universal acceptance/rejection. Literal/type admission still applies; present fixed values use the selected typed equality in I-009. DTD ID-default rules and E1-56 do not bridge these properties. |
| PSVI-Q001 | E1-56 patches lightweight union-member paragraphs, leaving earlier full-member wording and contributor entry premises intact. A member can match while a governing union facet fails; that attribute boundary has no unique full-member oracle here. Invalid complex attributes defeat the element contributor's type-validity premise; declaration-only fixed/identity failures need not. See [corrected reading](corrected-reading.md). |
| B-044 | Adopt effective declaration fallback as clarified in [G.1.10](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#changes): an absent optional reference uses its global declaration constraint when there is no local override. Already implemented; explicit empty overrides and XSI no-synthesis remain distinct. Added PSVI metadata has its own observation limits. |
| A-106 | Under both head exclusions, formal [element/type derivation](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-props-correct) admits type identity while §3.3.1 nomination prose forbids any affiliation. Same-type affiliation remains source-qualified; a prohibited nonempty derivation is a separate negative. |
| Anonymous identity / wildcard algebra | The [component identity note](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#no-identity) leaves some anonymous identity choices incomplete. [Multi-value wildcard algebra](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-aw-union) requires expressible pairwise intermediates without a complete operand order; a mathematically expressible final set does not prove a chosen fold valid. |

## Datatype and observation qualifications

The [published Datatypes](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html)
and [source editions](SOURCES.md) supply the following boundaries retained from
the research notes. Unresolved does not mean arbitrary processor choice.

| Boundary | Oracle treatment |
| --- | --- |
| RXQ001 / RGQ001 | Resolved by the selected grammar correction in [I-011](#i-011--regex-metacharacter-grammar). XPath-only regex features remain excluded. |
| RXQ002 | Record Unicode version and baseline versus encouraged later support for changed categories/later blocks. A later block is not automatically a mandatory rejection. |
| Canonical zero and union overlap | Generic one-to-one canonical representation needs integration with decimal/integer minus-zero wording and overlapping integer/decimal union members. Conventional outputs are not disproved; two policies are not thereby permitted. Explicit nonPositiveInteger and float/double zero rules remain determined. Acceptance/member metadata cannot prove an ambiguous canonical output. |
| Unsigned aliases / single-member union / gMonth | Complete Appendix A unsigned definitions supply signed integer aliases with bounds; preserve the prose seam. Concrete component/representation rules allow one union member despite informal “greater than 1”. Published gMonth form is `--MM`; historical deleted `--MM--` is excluded. |
| Temporal boundaries | Preserve leap-second lexical/body/Appendix D premises; Appendix E treats an admitted leap second as arithmetic overflow but does not settle all lexical admission. BCE/CE arithmetic needs explicit historical-year mapping. Cross-reference time/g* rollover identity needs a source interpretation, not clock-only equality or an invented canonical g* algorithm. Mixed-zone dateTime separation exactly 14 hours is incomparable under the explicit opening rule. Date example sign/direction discrepancies do not override operative offset/interval definitions. |
| Time midnight — R-127 | [Issue 2116](https://www.w3.org/Bugs/Public/show_bug.cgi?id=2116) identifies the conflict between §3.2.8's arbitrary-date ordering of `00:00:00` and `24:00:00` and §3.2.8.2's midnight canonical form. Its recorded clarification resolution is not a published correction in the selected baseline. Keep time-bound derivation probes source-qualified; neither day-preserving ordering nor recurring-clock equality alone settles this oracle. The explicit next-day mapping of `dateTime` in §3.2.7.1 remains determined. |
| Duration equality and order | §3.2.6 describes six coordinates, while §3.2.6.2 defines strict order using four reference dates. Equal endpoints for `P400Y` and `P146097D` do not independently establish duration identity. [XSD 1.1 §I.4](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/datatypes.html#changes) explicitly changes the value space to two coordinates; no such correction is selected here. Integrate identity, equality and partial order before assigning this pair an unqualified XSD 1.0 enumeration or bound expectation. |
| Duration / truncation prose | Duration's total-order paragraph refers to a timezone that its grammar lacks; invent no feasible timezone branch. Appendix D's right-truncation list omits gYearMonth despite its explicit type definition; specific grammars still govern. |
| Fundamental facets — FFP004/006/008, FM13 | Preserve printed property formulas separately from mathematical value-set cardinality: fixed-length integer lists classify finite; enumeration/pattern alone do not trigger atomic finite classification; Appendix C labels gMonth countably infinite. Union ordered explicitly excludes anySimpleType as common ancestor while bounded does not; add no unprinted common-primitive exclusion. Boolean validity cannot expose these component properties. |
| Bounds | Repeated equal exclusive endpoints have explicit allowance; do not substitute generic instance validity. Incomparable restriction endpoints need their actual prohibited-comparison wording. Inherited opposite lower-bound kinds retain the same-datatype versus same-step scope question; no blanket rewrite/rejection follows. |
| URI / language / precision | No implicit percent decoding, case folding, normalization or absolutization establishes anyURI equality. No modern language registry rejection follows from its historical pattern. [Double extrema, overflow and documented precision](SOURCES.md#floating-point-source-versions) remain separately qualified. |
| ENTITY member selection / 10662 | Full later ENTITY-context assessment is not adopted. A lexical NCName in `ENTITY|string` cannot prove normative string fallback; early unsupported results precede facets, so making them terminal can also suppress legitimate datatype fallback. Keep the [phase qualification](XSD11_CORRECTIONS.md#entity-member-selection-qualification) and use non-NCName fallback witnesses for conformance. |
| Observation / product / modality | Missing public canonical/PSVI/component output is unobservable, not covered. Product exclusions and resource/precision refusals are not datatype-invalid oracles. Compatibility recommendations, discouraged constructs and generic error/recovery permissions do not waive explicit must/conformance requirements or create rejection rules. |

Required XSD 1.0 result, WG interpretation, XSD 1.1 change, processor choice,
unspecified behavior and unavailable observation remain distinct evidence
classes. Corpus deviations at these seams must cite the published rule and
its qualification; neither the implementation nor another validator is the oracle.

## I-012 — anySimpleType constructor membership

**Status:** retained, source-qualified XSD 1.0 library interpretation; not an
approved erratum or an adoption of the XSD 1.1 constructor exclusions.

The 2004 [simple ur-type](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#Simple_Type_Definitions)
has absent variety, while [cos-st-restricts 2.1 and 3.1](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-st-restricts)
require suitable varieties for list items and union members. Literal combination
of these rules excludes `anySimpleType` from both constructors.

The [W3C stE053 metadata](https://github.com/w3c/xsdtests/blob/3d686a51975b4136fc957e1f006aefbe85a8a410/msMeta/SimpleType_w3c.xml) expressly expects its union containing
`anySimpleType` to be valid in 1.0 and invalid in 1.1. The library retains that
1.0 union interpretation. It also retains list-item acceptance, whose historical
support is weaker and remains disputed. These are explicit qualifications,
not consequences of the internal scalar representation or processor voting.

The final [1.1 construction rules](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/#atomic-vs-list)
do not establish a published 1.0 amendment. Separate constructor witnesses and
ordinary atomic controls record the list and union differences. A future change
requires resolving the versioned source conflict; it must not follow merely
from the operational variety assigned to builtin `anySimpleType`.

## I-013 — QName element constraints under xsi:type

**Status:** selected library interpretation of a lexical/context integration gap;
the three corpus witnesses are not settled processor-conformance judgments.

The [element representation](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-element)
resolves a declaration constraint to its actual value in schema scope.
[cvc-elt 5.1.1–5.1.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-elt)
then asks for its canonical lexical representation when applying it under
`xsi:type`. [QName](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#QName)
has contextual lexical mapping but no specified canonical representation.
Those clauses do not specify the resolver for this synthetic revalidation.
Approved E1-56 changes default/PSVI lexical wording without settling that resolver.

The library preserves the constraint's schema namespace context while checking
its spelling against the actual type, including facets. Missing or rebound
instance prefixes therefore cannot change the declaration's QName value.
Explicit instance text continues to resolve in instance scope. Replaying a
schema spelling in instance scope is an interoperability alternative, not a
reason to classify either reading as a confirmed implementation defect.

`qname-constraint-schema-context` records missing/rebound-prefix defaults and a
missing-prefix fixed value under a QName-derived actual type. All four comparison
processors reject those three instances; the selected library interpretation
accepts them. A WG resolution or a provenance-backed test for this exact
`xsi:type` seam would be needed to settle the source gap.

## I-014 — xsi:type under element wildcards

**Status:** selected WG correction 4299 to the XSD 1.0 baseline; existing
library behavior, separately from published Second Edition errata.

The printed [element-assessment rule](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-assess-elt)
permits assessment through a known `xsi:type` without an element declaration.
If neither declaration nor type assessment succeeds, it permits lax assessment
with `anyType` unless the subtree is skipped. This leaves the outcome of an
unresolved type name unclear: `cvc-elt` clause 4 applies to a declaration that
this element lacks.

[Issue 4299](https://www.w3.org/Bugs/Public/show_bug.cgi?id=4299#c2), classified
1.0/1.1, records the WG's 23 March 2007 decision to require physical `xsi:type`
values to resolve to type definitions. The rule appears in
[2012 `cvc-attribute`, clause 5](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#cvc-attribute).
The library deliberately adopts that bounded correction. Assessed physical
`xsi:type` must be a valid QName naming an available type, including beneath a
lax wildcard. Its selected type must be nonabstract and its constraints still
apply. QName-valued schema defaults and fixed constraints do not trigger this
physical-instance lookup.

A strict wildcard can obtain strict assessment through the known type route;
it does not always need an element declaration. Lax fallback still assesses
declared descendants. Skip leaves its subtree unassessed, including physical
XSI values; XML and namespace well-formedness remain required. None of these
routes waives wildcard namespace matching or loads schemas from instance hints
or network URLs. The library reports assessment failures without exposing a
PSVI validity property; failure is not a claim that every enclosing laxly
assessed item has the PSVI value `invalid`.

## I-015 — ordered-bound consistency and unordered values

**Status:** selected reading of the operative XSD 1.0 facet-consistency prose;
processor disagreements remain corpus provenance.

The same-step consistency rules for [minInclusive/maxInclusive](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#minInclusive-less-than-equal-to-maxInclusive)
and [minExclusive/maxExclusive](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#minExclusive-less-than-equal-to-maxExclusive)
prohibit a lower value greater than the upper value. Equal endpoints therefore
permit both-inclusive and both-exclusive pairs; the latter describes an empty
value space. Mixed inclusive/exclusive pairs prohibit equality as well.

The rule captions use less-than-or-equal notation, but the operative prose
prohibits the opposite comparison. For unordered values these formulations are
not equivalent. The library follows the prose: incomparable endpoints do not
violate the pair-consistency rule, including finite/NaN pairs for float and
double, just as with partial temporal orders. This does not make unordered
instance comparisons succeed. Each bound still tests its own value-space
predicate, so a mixed finite/NaN pair admits no instance, while a NaN/NaN
inclusive pair admits NaN through equality.

Facet-literal admission against the base type, fixed facets, and restriction
narrowing remain separate obligations. In particular, a bound declared in a
later restriction cannot bypass a base bound merely because the resulting pair
would be allowed when declared together.
