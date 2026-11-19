# Interpretation records separate from approved corrections

The [approved correction layer](corrected-reading.md) applies published XSD
1.0 errata. The records here explain source questions that arise during
corpus review. A closed issue is not, by itself, a new normative XSD 1.0
amendment. Selected corrections below follow the
[selected-correction policy](SOURCES.md#selected-corrections). Their adoption
does not claim publication as Second Edition errata.

<a id="processor-tie-break"></a>
**Processor tie-break.** Precedence is: explicit specification text, then
adopted corrections, then accepted W3C test-suite expectations, then measured
processor behaviour. Only when the earlier sources leave a reading doubtful does
a record follow Xerces-J, then .NET, then libxml2, taking the first that has a
result. libxml2 is permissive, so its agreement alone is weak evidence.
xmlschema results are diagnostic only.

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
missing declarations are distinct from skip: their attributes hold unknown
field values (I-022).
The four intrinsic XSI attributes on an assessed element remain typed even
with a skip attribute wildcard; attributes under a skipped element do not
contribute identity values. Treating absent type information as an empty
string remains forbidden.

The adopted correction outranks processor behaviour. Observed with Xerces-J
2.12.2 and xmlschema 4.3.2: Xerces-J gives an attribute admitted by a skip
wildcard the field value `null`, so a key whose field selects only such
attributes is satisfied and a keyref row holding one reports a missing key, and
it accepts two assessed attributes under one field
([I-022](#i-022--undeclared-lax-attributes-are-unknown-field-values)).
xmlschema counts skipped children and attributes as field nodes, reporting
multiple values, and takes an `xsi:nil` under a skipped element as a field
value. Cases: `project/corrections-skip-field-assessment`.

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

Observed with .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13: xmlschema and
libxml2 reject two nilled fields under `unique`, xmlschema comparing them as
empty strings, and .NET does not count a nilled node toward field cardinality
when it comes first, accepting it followed by a value or by a second nilled
node. Xerces-J 2.12.2 gives the
project result. Cases: `project/corrections-unique-nilled-as-missing`.

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
Validator acceptance does not waive independent recursive wildcard restrictions.

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
implementation and work accounting are documented at `ValidateContentRestriction`
in [`internal/schema`](../../internal/schema/content_restriction.go), not in the
historical proposal.

The waiver is an adopted correction, which outranks processor behaviour.
Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2
2.9.13: .NET and xmlschema keep the order of generated choices and reject the
waived reorderings. Xerces-J accepts reorderings of a generated choice in the
base but rejects a reversed generated choice in the derived type and a generated
choice mapped onto a wildcard (`rcase-RecurseLax.2`). libxml2 checks no particle
restriction ([I-028](#i-028--content-model-checks)) and accepts the opponents
that cross an explicit boundary. Cases: `project/corrections-span-*`.

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

The library therefore knows no `xml:*` attribute declarations unless an
explicit source supplies them, and an undeclared `xml:lang` is assessed like any
other attribute
([cvc-complex-type](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-complex-type)
clause 3, [cvc-type](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-type)
clause 3.1.1). .NET 10.0.14 admits `xml:*` attributes without declarations and
resolves an `xml:lang` reference without an import, only warning; .NET and
xmlschema 4.3.2 resolve `xml:lang` from a built-in `xml.xsd` that the explicit
sources do not supply. Xerces-J 2.12.2 and libxml2 2.9.13 agree with the
project. Cases: `project/namespace-declarations-xml-missing-ref`,
`project/namespace-declarations-xml-missing-wildcard-strict`,
`project/namespace-declarations-xml-reference-without-import`,
`project/runtime-rejects-undeclared-xml-namespace-attributes` and
`project/structures-simple-four-xsi-attributes`.

[Built-in Attribute Declarations §3.2.7](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#d0e3067)
states that "there are four attribute declarations present in every schema by
definition", so a reference such as `ref="xsi:type"` resolves without an XSI
schema document; the adopted correction removes only the import. Accepted W3C
`msMeta/Additional_w3c.xml` addB182, a schema document whose target namespace
is the XML Schema namespace, shows that explicit sources can supply further
components in that namespace. Text and correction decide these cases, so
processor results do not. Observed with Xerces-J 2.12.2, .NET 10.0.14 and
libxml2 2.9.13: none resolves a reference to an intrinsic XSI declaration.
Xerces-J and libxml2 report it unresolved, or the missing import when there is
none, and .NET reports the attribute undeclared. Xerces-J also requires an
import for a reference to a supplied XSD-namespace component. Cases:
`project/builtin-xsi-known-reference-without-import`,
`project/builtin-xsi-required-*` and `project/xsd-components-*-without-import`.

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

A physical `xsi:schemaLocation` is assessed only as a list of anyURI, and
`xsi:noNamespaceSchemaLocation` as one anyURI, on assessed elements. The
attribute's type makes any item count valid and no constraint records an
even-count rule; Xerces-J, .NET, xmlschema and libxml2 accept an odd count
(corpus case `xsi-schema-location-hint-syntax`). Hints on skip-processed content
are not assessed, consistent with `xsi:nil` and `xsi:type` being ignored there.
An element whose start fails fatally before attribute assessment gets no hint
diagnostic.

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

The [XSI declaration restriction](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#no-xsi)
applies to a declaration whose {target namespace} is the XSI namespace, so an
unqualified local declaration in a schema with that target namespace is
admitted. These rules, §3.2.7 and the adopted correction decide the cases
below, so processor results do not. Observed with Xerces-J 2.12.2, .NET
10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- Xerces-J, .NET and libxml2 reject every schema that references an intrinsic
  XSI declaration ([I-006](#i-006--xsdxsi-schema-references)), so they assess
  none of its instances.
- xmlschema synthesizes an absent XSI attribute from a use's default or fixed
  value and checks a physical one against a use's fixed value. It accepts
  supplied declarations in the XSI namespace, including a qualified local one,
  and an invalid default or fixed `xsi:nil` value. It compares `@xsi:type` and
  `@xsi:nil` fields by spelling, compares an `@xsi:schemaLocation` field as a
  string that equals no anyURI list, and does not reject an invalid anyURI item
  in `xsi:schemaLocation`.
- .NET and libxml2 select no node for an `@xsi:*` field, so a key on one reports
  a missing field and a unique constraint on one never collides.
- .NET rejects an unqualified local attribute named `nil` in a schema whose
  target namespace is the XSI namespace, and an XSI attribute other than the
  four intrinsic ones that a skip wildcard admits.

Cases: `project/builtin-xsi-absent-defaults-no-intrinsic-replay`,
`project/builtin-xsi-absent-fixed-boolean-key`,
`project/builtin-xsi-constraint-qname-not-type-reference`,
`project/builtin-xsi-constraint-uri-list-not-hint-pairs`,
`project/builtin-xsi-default-qname-key-values`,
`project/builtin-xsi-default-schema-location-list-key`,
`project/builtin-xsi-forbidden-declaration-*`,
`project/builtin-xsi-invalid-nil-default`, `project/builtin-xsi-invalid-nil-fixed`,
`project/builtin-xsi-local-nil-qualified`, `project/builtin-xsi-local-nil-unqualified`,
`project/builtin-xsi-physical-hint-list-key`, `project/builtin-xsi-physical-nil-key`,
`project/builtin-xsi-physical-type-unique`,
`project/builtin-xsi-present-fixed-value-exemption` and
`project/builtin-xsi-unknown-instance-skip`.

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

The element, model group reference and compositor mappings (§3.3.2, §3.7.2,
§3.8.2) give an `xs:element`, `xs:group`, `xs:all`, `xs:choice` or `xs:sequence`
with `minOccurs` and `maxOccurs` 0 no component in the same words, and the
library reads them the same way: the item's own
references and value constraints are not resolved or checked, while its source
syntax, the rules decided at its start tag, and the definitions nested in it
(an inline type definition, identity constraints) keep their own mapping.
[I-023](#i-023--unresolved-references) lists the cases.

The mapping decides the prohibited-attribute cases below, so processor results
do not. Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and
libxml2 2.9.13:

- Xerces-J and xmlschema resolve a prohibited item's type and check its fixed
  value against that type and against a referenced declaration's fixed value.
  Xerces-J, .NET and libxml2 apply the XSI declaration restriction to a
  prohibited qualified local item.
- xmlschema keeps the prohibited item as a declaration. It reports a duplicate
  beside a local or group use of the same name, compares it with an inherited
  use under extension, supplies its fixed value to an absent attribute, checks
  an attribute that a skip wildcard or an intrinsic XSI declaration admits
  against that fixed value, and rejects a physical XSI attribute that a
  prohibited reference names.
- .NET reports a duplicate for a prohibited and a required local item of one
  name in a restriction, and accepts an invalid inline simple type under a
  prohibited item.
- libxml2 rejects an inline simple type under a prohibited item, which the
  schema for schemas admits.
- Xerces-J and libxml2 reject the prohibited XSI references
  ([I-006](#i-006--xsdxsi-schema-references)), and .NET selects no node for an
  `@xsi:*` field ([I-007](#i-007--xsi-attribute-uses-and-psvi-contributions)).

Cases: `project/builtin-xsi-prohibited-fixed-no-use`,
`project/builtin-xsi-prohibited-maps-no-use`, `project/prohibited-fixed-*`,
`project/prohibited-no-component-invalid-boolean-fixed-prohibited`,
`project/prohibited-no-component-invalid-inline-type`,
`project/prohibited-no-component-missing-xsd-type-prohibited`,
`project/prohibited-no-component-qualified-xsi-local`,
`project/prohibited-no-component-reference-fixed-mismatch-prohibited` and
`project/prohibited-no-component-valid-inline-type`.

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
uses, which [DCS-Q001](#dcs-q001) keeps qualified. Type validity of the
constraint's literal belongs to DCS-Q001's Simple Default Valid correction.

Observed: xmlschema 4.3.2 compares a fixed QName by spelling, so an instance
value whose prefix binds the same namespace fails; Xerces-J 2.12.2 gives the
project result. Cases: `project/corrections-fixed-qname`.

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

The adopted correction outranks processor behaviour. Observed with Xerces-J
2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13: Xerces-J accepts an
`xsi:type` member type that bypasses a restricted union's enumeration or
pattern, and a substitution-group member whose type is a member of the head's
restricted union; Xerces-J and libxml2 do the same for a restricted union
nested in another union. xmlschema loses the constraint under a facet-free restriction of
a restricted union. .NET rejects a union whose member is a restricted union.
Cases: `project/corrections-restricted-union-nested`,
`project/corrections-restricted-union-xsi` and
`project/corrections-union-substitution-bad-member`.

## I-011 — regex metacharacter grammar

**Status:** adopt the brace/bar consistency repair. The
[F&O 3.1 correction note](https://www.w3.org/TR/2017/REC-xpath-functions-31-20170321/#regex-syntax)
explicitly identifies missing braces as an XSD 1.0 error.

Bare braces are metacharacters, escaped braces are literals, and unescaped bar
introduces alternation. The existing parser already implements this reading.
This does not import XPath regex anchors: `^` and `$` remain literal XSD
characters. Unicode versions and other XPath regex extensions are independent.

Unescaped dash is a literal only as the first term of a positive group,
immediately before `]`, or immediately before the `-[` that starts subtraction.
It never starts a range (`[--a]` is rejected) and never ends one at `[`, `]` or
end of input. `[a--[b]]` is `{a, -}` minus `{b}`, `[a-z--[b-z]]` is `{a, -}`,
`[--[a]]` is a first-term dash minus `{a}`, and an escaped `\-` follows ordinary
range rules.

Datatypes F.1 allows a literal dash "only at the beginning or end of a positive
character group"; this record treats the end of the `posCharGroup` operand of a
subtraction as such an end. The accepted W3C test `msMeta/Regex_w3c.xml` reF56
(`[a-z--[b-z]]` valid) decides it, and reF57 (`[a--b]` invalid) bounds it. The
[processor tie-break](#processor-tie-break) was considered and does not apply:
an accepted W3C expectation outranks processor behaviour. Xerces rejects both
dash-before-subtraction shapes; following it would require recording reF56 as
a project correction without specification grounding. Fixtures:
`project/datatypes/regex-dash-subtraction-after-range-end` and
`project/datatypes/regex-dash-range-leading`.

Observed with .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13: .NET reads `^`
and `$` as anchors. .NET, xmlschema and libxml2 accept a bare `}`, and .NET and
libxml2 a bare `{`. .NET and libxml2 start a range at a dash, accepting
`[--a]` and rejecting `[a--[b]]` as a reversed range. The correction and the
W3C expectations decide these cases, so processor results do not. Cases:
`project/corrections-regex-bare-close-brace`,
`project/corrections-regex-bare-open-brace` and
`project/corrections-regex-literals-and-alternation`.

## Remaining source-integration boundaries

These are qualifications, not alternative permitted policies or approved
repairs. Success/failure alone cannot observe disputed PSVI/component properties.
Fixture outcomes remain in the corpus manifest.

| Record | Source and retained boundary |
| --- | --- |
| Complex-type annotations | Structures [type annotations](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#ct-annotations), [XML mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-type) and [absence convention](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#key-null): required annotation set versus absent no-annotation mapping. Do not invent an annotation, equate absence with empty or reject an otherwise admitted constructor to conceal it. |
| Element validity outcome connective | [Element validity outcome](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-validity) lacks the connective between nested declaration/type-route clauses. Ordinary applicable-route readings remain conditional; no approved AND/OR repair or blanket rejection follows. Descendant and assessment-attempted conditions remain independent. |
| IBS-Q001 — root ID eligibility and successful declaration validation | [Eligibility](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#key-eas) depends on successful declaration validation; [bindings](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#iib-binding) use ID-valued attributes/children, while [cvc-id](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-id) forbids empty bindings. A directly ID-typed root's own value can create an ownerless key conditional on that same success. Preserve the success/contribution question; invent no phase, root-self owner or blanket ID-element rejection. An independently eligible IDREF can create an unambiguously empty binding. |
| <a id="ids-q001"></a>IDS-Q001 — identity-component resolution | The later `src-resolve` list repairs the omitted identity-component kind. Adopt that clarification, already implemented: keyrefs resolve to supplied key/unique definitions; wrong category, missing target, arity and tuple checks remain. Existing `idH035` protects keyref-to-keyref rejection. No new indexing algorithm follows. |
| Selector/field XPath spelling | [Selector](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-selector-xpath)/[field](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-fields-xpaths) components allow whitespace/expanded axes more broadly than normative Appendix A patterns. Distinct component and XML-representation phases must both be reconciled; component permission alone does not prove XML spelling admission. |
| Empty `xml:lang` in documentation | Dated [documentation mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#element-documentation) and XML Second Edition support nonempty xml:lang; Appendix A imports an XML declaration. The 2009 revision of xml.xsd allows empty but does not establish the 2004 imported body. Preserve source selection, not generic XML rejection or automatic later-version acceptance. |
| CMP-Q001 — empty import namespace | An [import](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-import) with present-empty namespace has anyURI syntax, and no Structures constraint names it, so the text alone is doubtful. Accepted W3C `msMeta/Schema_w3c.xml` schZ014_a, a schema without targetNamespace whose only child is `<xsd:import namespace=""/>`, expects invalid (accepted 2006-07-16), and that expectation decides it, as its sibling schZ014_b decides the empty targetNamespace in [I-041](#i-041--empty-target-namespace): a present `namespace` whose collapsed value is empty is a schema error, used or not. Empty is not absent; ordinary obtained-referent matching still applies. Observed on the same document (`project/namespace-schema-import-namespace-cannot-be-empty`): Xerces-J 2.12.2 and libxml2 2.9.13 accept, .NET 10.0.14 rejects the empty value and xmlschema 4.3.2 rejects it under its absent-namespace rule. With a targetNamespace present, Xerces-J, xmlschema and libxml2 accept and only .NET rejects. |
| Attribute-group redefine contributions | Attribute-group [redefine clause 7.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-agrere) uses ordinary mapping, including nonself group contributions; its note describes explicit-only children. No inheritance from the original group is settled. The note alone does not justify rejecting otherwise matching nonself references. |
| <a id="dcs-q001"></a>DCS-Q001 — value-constraint type validity | Local [attribute mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-attribute) puts a constraint on the use, while declaration rules ([a-props-correct](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#a-props-correct) and [e-props-correct](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-props-correct) clause 2) require "the canonical lexical representation of its value" to be valid, and [Use correctness](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#au-props-correct) has no type-validity clause. The 1.0 canonical rule is undefined for types without a canonical representation (duration, the g* types, anyURI, QName, NOTATION, the string family). **Type validity:** the selected [Simple Default Valid](SOURCES.md#selected-corrections) correction decides it at element declarations, attribute declarations and attribute uses alike: "V.{lexical form} is ·valid· with respect to T" and "V.{lexical form} maps to V.{value}"; the canonical form is not checked. This is an adopted correction, so the processor tie-break is not reached. Present fixed values use the selected typed equality in I-009. Case: `project/corrections-fixed-pattern-type-validity` (local attribute `fixed="1"` on a boolean restricted by pattern `1`: valid; Xerces-J rejects it, checking the canonical `true` against the pattern). Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13 on 56 controls (`project/attributes/value-constraint-type-validity/*`) covering local, global and referenced attributes, global and local elements, default and fixed, and boolean, decimal, float, integer, dateTime, hexBinary, duration, gYear, list and union types: Xerces-J checks its canonical string, so it rejects every control whose literal satisfies the pattern while that string does not, including duration (`PT60S` as `P0Y0M0DT0H0M60S`) and gYear (`2000+00:00` as `2000Z`), which have no 1.0 canonical representation; .NET, xmlschema and libxml2 accept them. When the literal fails the pattern and the canonical form satisfies it, every provider rejects, except libxml2, which does not check a use-level `fixed` on an attribute reference. **ID defaults:** whether declaration-only ID-default prohibitions transfer to attribute uses remains source-qualified; preserve the ID-default probes as qualified, not universal acceptance or rejection. DTD ID-default rules and E1-56 do not bridge these properties. |
| PSVI-Q001 — union-member PSVI scope | E1-56 full/lightweight union-member scope; owned by [corrected reading](corrected-reading.md#e1-56-applied-reading). |
| Same-type affiliation under head exclusions | Under both head exclusions, formal [element/type derivation](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-props-correct) admits type identity while §3.3.1 nomination prose forbids any affiliation. Same-type affiliation remains source-qualified; a prohibited nonempty derivation is a separate negative. |
| Anonymous identity / wildcard algebra | The [component identity note](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#no-identity) leaves some anonymous identity choices incomplete. [Multi-value wildcard algebra](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-aw-union) requires expressible pairwise intermediates without a complete operand order; a mathematically expressible final set does not prove a chosen fold valid. |

## Datatype and observation qualifications

The [published Datatypes](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html)
and [source editions](SOURCES.md) supply the following boundaries. Unresolved
does not mean arbitrary processor choice.

| Boundary | Oracle treatment |
| --- | --- |
| Regex Unicode version | Record Unicode version and baseline versus encouraged later support for changed categories/later blocks. A later block is not automatically a mandatory rejection. |
| Canonical zero and union overlap | Generic one-to-one canonical representation needs integration with decimal/integer minus-zero wording and overlapping integer/decimal union members. Conventional outputs are not disproved; two policies are not thereby permitted. Explicit nonPositiveInteger and float/double zero rules remain determined. Acceptance/member metadata cannot prove an ambiguous canonical output. |
| Unsigned aliases / single-member union / gMonth | Complete Appendix A unsigned definitions supply signed integer aliases with bounds; preserve the prose seam. Concrete component/representation rules allow one union member despite informal “greater than 1”. Published gMonth form is `--MM`; historical deleted `--MM--` is excluded. |
| <a id="temporal-boundaries"></a>Temporal boundaries | No lexical form has a second 60 (Second 60 in the [Time midnight](#time-midnight) row), so Appendix E's arithmetic for a leap second has no input. **BCE leap years.** A year's leap status is that of [Appendix E](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#adding-durations-to-dateTimes)'s `maximumDayInMonthFor`: the signed year as written is leap when divisible by 400, or by 4 and not by 100, so `-0004` and `-0400` are leap and `-0001`, `-0005`, `-0100` and `-0401` are not. The text is doubtful: §3.2.7 computes values "using the Gregorian algorithm" from 0001-01-01 without settling BCE years, and its note, which calls `-0001` "the year 1 BCE" and 1 BCE "a leap year", supports a proleptic, astronomical count instead. No adopted correction applies and no accepted W3C test has a BCE month-day, so the processor tie-break settles it by Xerces-J, which applies Appendix E. One leap status governs lexical admission of February 29, timezone normalization across a month end, order, bounds, enumeration and equality. There is still no year 0: the day after `-0001-12-31` is `0001-01-01`. Cases: `project/temporal-year-boundaries` (its `bce-*` phases) and `project/datatypes/temporal/bce-leap-year/*`. Xerces-J and libxml2 give the project result in each; xmlschema does too except across the year boundary, where it counts a year 0 and rejects `-0001-12-31T23:00:00-02:00` as `0001-01-01T01:00:00Z`. .NET rejects every negative year, in instances and facet literals, so it gives no evidence either way. Cross-reference time/g* rollover identity needs a source interpretation, not clock-only equality or an invented canonical g* algorithm. Mixed-zone dateTime separation exactly 14 hours is incomparable under the explicit opening rule. Date example sign/direction discrepancies do not override operative offset/interval definitions. |
| <a id="time-midnight"></a>Time midnight — [R-127](https://www.w3.org/2001/05/xmlschema-rec-comments.html#pfiTimeMidnight) | [Issue 2116](https://www.w3.org/Bugs/Public/show_bug.cgi?id=2116) identifies the conflict between §3.2.8's arbitrary-date ordering of `00:00:00` and `24:00:00` and §3.2.8.2's midnight canonical form. R-127 was classified a clarification with erratum, and the selected baseline carries no published correction. The W3C text and tests decide the midnight, second 60 and same-side questions below; where they leave what a `time` value is doubtful, the processor tie-break settles it by Xerces-J. **Value.** A `time` value is its timezone-normalized time of day. The day a timezone offset moves a time to is not part of the value: `00:30:00+14:00` is `10:30:00Z`, `23:00:00-02:00` is `01:00:00Z`, and `23:00:00-01:00` is `00:00:00Z`. One order and one equality relate two `time` values wherever the specification relates them: an instance value against a bound (`project/datatypes/bounds/instance-maxInclusive-time-next-day-bound`, `project/datatypes/bounds/instance-maxExclusive-time-next-day-bound`, `project/datatypes/bounds/instance-minInclusive-time-previous-day-bound`, `project/datatypes/bounds/instance-minExclusive-time-previous-day-bound`, `project/datatypes/bounds/instance-maxInclusive-time-previous-day-bound`, `project/datatypes/bounds/instance-maxExclusive-time-previous-day-bound`, `project/datatypes/bounds/instance-minInclusive-time-next-day-bound`, `project/datatypes/bounds/instance-minExclusive-time-next-day-bound`), enumeration (`project/datatypes/bounds/enumeration-time-equal-across-day`, `project/time-facet-comparison-ignores-date-shift`), element and attribute fixed values (`project/datatypes/temporal/time-element-fixed-equal-across-day`, `project/datatypes/temporal/time-attribute-fixed-equal-across-day`), identity constraints (`project/datatypes/temporal/time-unique-equal-across-day`), a value constraint applied under an `xsi:type` that bounds the time (`project/datatypes/temporal/time-default-under-xsi-type-bound`, `project/datatypes/temporal/time-fixed-under-xsi-type-bound`), base value-space membership of a bound, enumeration or default literal (§4.3.7–§4.3.10; `project/datatypes/bounds/restrict-maxExclusive-time-earlier-day-later-clock`, `project/datatypes/bounds/restrict-maxInclusive-time-earlier-day-later-clock`, `project/datatypes/bounds/restrict-minInclusive-time-later-day-earlier-clock`, `project/datatypes/bounds/restrict-minExclusive-time-later-day-earlier-clock`, `project/datatypes/bounds/enumeration-time-under-next-day-max`, `project/datatypes/bounds/default-time-under-next-day-max` invalid, `project/datatypes/bounds/enumeration-time-over-previous-day-max`, `project/datatypes/bounds/default-time-at-next-day-max` valid), with the base's other facets still applying to the literal (`project/datatypes/bounds/restrict-maxInclusive-time-earlier-day-base-pattern`, `project/datatypes/bounds/restrict-maxInclusive-time-earlier-day-outside-base-pattern`), the equality that admits an exclusive restatement (Bounds below), the equality that keeps a fixed base bound (`project/datatypes/bounds/fixed-maxExclusive-time-earlier-day-same-clock`, `project/datatypes/bounds/fixed-maxInclusive-time-earlier-day-same-clock`, `project/datatypes/bounds/fixed-minExclusive-time-later-day-same-clock`, `project/datatypes/bounds/fixed-minInclusive-time-later-day-same-clock`, `project/datatypes/bounds/fixed-maxExclusive-time-same-day-other-offset`, `project/datatypes/bounds/fixed-minInclusive-time-same-day-other-offset` valid), the opposite-side clauses of the `*-valid-restriction` constraints (`project/datatypes/bounds/opposite-maxInclusive-time-earlier-day-later-clock`, `project/datatypes/bounds/opposite-minInclusive-time-later-day-earlier-clock` valid, `project/datatypes/bounds/opposite-maxInclusive-time-later-day-earlier-clock`, `project/datatypes/bounds/opposite-minInclusive-time-earlier-day-later-clock` invalid), and `minInclusive <= maxInclusive` and `minExclusive <= maxExclusive` (`project/datatypes/bounds/same-step-minInclusive-maxInclusive-time-earlier-day-later-clock`, `project/datatypes/bounds/same-step-minExclusive-maxExclusive-time-earlier-day-later-clock` valid, `project/datatypes/bounds/same-step-minInclusive-maxInclusive-time-later-day-earlier-clock`, `project/datatypes/bounds/same-step-minExclusive-maxExclusive-time-later-day-earlier-clock` invalid). Xerces-J gives the project's result in every one of these cases. .NET and xmlschema order and equate times on the day the offset moves them to, and libxml2 carries a negative-offset time into the next day but wraps a positive-offset time within its day, so neither reading explains its results. In these cases libxml2 orders `23:00:00-02:00` and `22:00:00-03:00` after each `Z` time from `01:00:00Z` to `12:00:00Z` that they are compared with: it accepts such a time as an instance value, an enumeration or default literal, or a bound below a maximum `23:00:00-02:00`, and rejects it as an instance value or a maximum against a minimum `22:00:00-03:00` or `23:00:00-02:00`. libxml2 rejects `23:00:00-01:00` against a maximum `23:59:59Z` and an enumerated `00:00:00Z`, and rejects a default or fixed `23:00:00-02:00` under an `xsi:type` maximum `02:00:00Z` but accepts it under a minimum `02:00:00Z`. libxml2 rejects `01:00:00Z` against a fixed value or fixed base bound `23:00:00-02:00`, also under `xsi:type`, rejects `20:30:00-14:00` against an enumerated `10:30:00Z`, and accepts `23:00:00-02:00` with `01:00:00Z` and `01:00:00Z` with `03:00:00+02:00` as distinct `xs:unique` values. libxml2 rejects `21:00:00-04:00` against an element `fixed="23:00:00-02:00"` but accepts it against the same attribute fixed value, and under a fixed base bound rejects a derived `maxExclusive` `22:00:00-03:00` under `23:00:00-02:00` as not less than it and a derived `minExclusive` `23:00:00Z` over `01:00:00+02:00` as not greater than it. .NET accepts a derived `maxInclusive` `00:30:00Z` that the base pattern `\d\d:00:00Z` does not match, and accepts the text `01:30:00Z` under an `xsi:type` against a fixed `23:00:00-02:00`. No W3C instance test distinguishes the readings: the zoned msMeta instances (time_maxExclusive001_1155 through time_minInclusive006_1163) do not cross midnight after normalization, and the NIST time bounds are unzoned. **Timezone presence.** A time with a timezone and one without are ordered by the §3.2.7.4 partial order, in which a time without a timezone may be that clock time at any offset from `-14:00` to `+14:00` and an indeterminate comparison satisfies no bound. They are never equal. The comparison places both times on one arbitrary date (§3.2.8), a time with a timezone at its normalized time of day, and the fourteen-hour interval around a time without one does not wrap past midnight: `00:00:00` is before `23:00:00Z`, and `23:00:00` is after `00:00:00Z`. Xerces-J agrees; .NET, xmlschema and libxml2 order the pair without the indeterminacy (`project/datatypes/bounds/instance-maxInclusive-time-next-day-bound`, `project/datatypes/bounds/instance-maxInclusive-time-unzoned-bound`, `project/datatypes/bounds/instance-minInclusive-time-unzoned-bound`, `project/datatypes/bounds/instance-maxInclusive-time-unzoned-bound-determinate`). **Midnight.** `24:00:00` is midnight, the value of `00:00:00`; no separate value ends the day. The W3C text decides this, so the processor tie-break does not apply. §3.2.8.2 states that “the canonical representation for midnight is 00:00:00”, and §2.3.1 requires a “one-to-one mapping between literals in the canonical lexical representation and values”, so `24:00:00` cannot denote a value that `00:00:00` does not. Structures applies an element value constraint through “the canonical lexical representation of the {value constraint} value” (Element Locally Valid (Element) 5.1.1 and 5.1.2), so a separate end-of-day value needs either a canonical representation that §3.2.8.2 excludes or a `fixed="24:00:00"` that rejects `24:00:00`. XSD 1.1 Datatypes §3.3.8.2 states the reading outright, “The lexical mapping maps '00:00:00' and '24:00:00' to the same value, namely midnight”, and XPath Functions and Operators 3.1 `op:time-equal` gives `xs:time("24:00:00+01:00") eq xs:time("00:00:00+01:00")` as true, the two being “two different lexical representations of the same value”. The hour 24 is therefore normalized like every other time: `24:00:00Z`, `24:00:00+00:00` and `24:00:00-00:00` are `00:00:00Z`, `24:00:00-01:00` is `01:00:00Z`, and `24:00:00+01:00` is `23:00:00Z`. The canonical representation is `00:00:00`, with `Z` when the time has a timezone. The same order and equality apply to it in every relation: bounds (`project/datatypes/bounds/instance-maxInclusive-time-midnight`, `project/datatypes/bounds/instance-maxInclusive-time-leap-second`, `project/datatypes/bounds/instance-minInclusive-time-midnight`, `project/datatypes/bounds/instance-maxInclusive-time-midnight-utc`, `project/datatypes/bounds/instance-minInclusive-time-midnight-utc`, `project/datatypes/bounds/instance-maxExclusive-time-midnight-bound`, `project/datatypes/bounds/same-step-minInclusive-maxInclusive-time-hour-24-upper`), enumeration (`project/datatypes/temporal/time-enumeration-midnight`, `project/datatypes/temporal/time-enumeration-end-of-day`, `project/datatypes/temporal/time-enumeration-start-of-day-utc`, `project/datatypes/temporal/time-enumeration-hour-24-offset`), fixed and default values (`project/datatypes/temporal/time-element-fixed-midnight`, `project/datatypes/temporal/time-element-fixed-end-of-day`, `project/datatypes/temporal/time-attribute-fixed-midnight`, `project/datatypes/temporal/time-attribute-fixed-end-of-day`, `project/datatypes/temporal/time-default-end-of-day-under-xsi-type-bound`, `project/datatypes/temporal/time-fixed-end-of-day-under-xsi-type-bound`), identity constraints (`project/datatypes/temporal/time-unique-midnight`), and the equality that keeps a fixed base bound (`project/datatypes/bounds/fixed-minInclusive-time-midnight-next-day`, `project/datatypes/bounds/fixed-maxInclusive-time-midnight-zero-offset`). Xerces-J differs: it keeps `24:00:00` without a timezone or with `Z` distinct from and after `00:00:00`, maps `24:00:00+00:00` and `24:00:00-00:00` to `00:00:00Z`, and applies a value constraint through the canonical `00:00:00`, so its `fixed="24:00:00"` accepts `00:00:00` and rejects `24:00:00`. libxml2 keeps `24:00:00` distinct from and after `00:00:00` with or without a zero offset, and places the hour 24 with a non-zero offset after every time of the day. xmlschema maps `24:00:00` to `00:00:00`, and .NET rejects the lexical form. **Second 60.** The seconds field never admits 60, in `time`, in `dateTime` and in every other place their clock appears: an instance value, a value constraint and a facet literal. [Appendix D.3.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#isoformats) is the doubtful text: it says “A value of 60 or more is allowed only in the case of leap seconds” and counts a leap second used with an inappropriate month and day as added to the following minute. Accepted W3C `msMeta/DataTypes_w3c.xml/time016_2024` decides it: it rejects the instance `13:20:60`, which that sentence would carry into the next minute, and no accepted W3C test admits a second 60, so the processor tie-break does not apply. XSD 1.1 agrees: it states that “Leap-seconds are not permitted”, limits the seconds to `[0-5] digit`, and lists that the treatment of leap seconds “is no longer ·implementation-defined·”. Cases: instance values (`project/datatypes/temporal/second-60`, `project/datatypes/bounds/instance-maxInclusive-time-leap-second`), value constraints (`project/attribute-fixed-time-leap-second-offset-equivalent-lexical`, `project/attribute-fixed-time-leap-second-offset-equivalent-utc`, `project/element-fixed-time-leap-second-offset-equivalent-lexical`, `project/element-fixed-time-leap-second-offset-equivalent-utc`) and facet literals (`project/datatypes/bounds/restrict-maxInclusive-time-second-60`, `project/datatypes/bounds/restrict-maxInclusive-dateTime-second-60`). Xerces-J, .NET, xmlschema and libxml2 reject the second 60 in each. **Same-side restriction clauses.** W3C `msMeta/DataTypes_w3c.xml/time_maxInclusive007_119` (base maxInclusive `12:00:00-10:00`, derived `12:00:00-14:00`, invalid) is valid by time of day, `02:00:00Z` against `22:00:00Z`, and Xerces-J accepts it. The clauses of the `*-valid-restriction` constraints that forbid a derived bound from loosening the base bound of its own side therefore order the two bounds by un-wrapped normalized time, in which a bound keeps the day that its timezone offset or its hour 24 moves it to: §3.2.8 states that “the order relation on time values is the Order relation on dateTime (§3.2.7.4) using an arbitrary date”. A derived bound inside the base by value is rejected when its day loosens the base (`project/datatypes/bounds/restrict-maxInclusive-time-later-day-earlier-clock`, `project/datatypes/bounds/restrict-minInclusive-time-earlier-day-later-clock`), also when it has the value of the base bound (`project/datatypes/bounds/fixed-maxExclusive-time-later-day-same-clock`, `project/datatypes/bounds/fixed-maxInclusive-time-later-day-same-clock`, `project/datatypes/bounds/fixed-minExclusive-time-earlier-day-same-clock`, `project/datatypes/bounds/fixed-minInclusive-time-earlier-day-same-clock`, and the restatements under Bounds below); Xerces-J accepts every one. libxml2 accepts a derived `maxInclusive` `22:00:00Z` under a base `00:00:00+01:00` and a derived `minInclusive` `01:00:00+02:00` under a fixed base `23:00:00Z`. A bound `24:00:00Z` lies one day after `00:00:00Z` in these clauses only: a derived upper bound `24:00:00Z` loosens a base `00:00:00Z` and a derived lower bound `00:00:00Z` loosens a base `24:00:00Z` (`project/datatypes/bounds/restrict-maxInclusive-time-hour-24-later-day`, `project/datatypes/bounds/restrict-minInclusive-time-hour-24-earlier-day` invalid), while a derived upper bound `00:00:00Z` does not loosen a base `24:00:00Z` (`project/datatypes/bounds/restrict-maxInclusive-time-hour-24-earlier-day` valid). Xerces-J and libxml2 give those three results because they order `24:00:00Z` after `00:00:00Z` in every relation; xmlschema accepts all three schemas. No other relation reads that day: no W3C test requires it elsewhere, and Xerces-J, apart from its distinct `24:00:00`, orders every other relation by time of day. The explicit next-day mapping of `dateTime` in §3.2.7.1 remains determined. |
| <a id="duration-equality-and-order"></a>Duration equality and order | Duration equality is value identity, as for every value space: §4.2.1 requires that “if a = b, then a and b cannot be distinguished (i.e., equality is identity)”, while §3.2.6.2 defines only the strict order, x < y iff s+x < s+y for each of four reference dateTimes. Identity is taken between (months, seconds) pairs, the value space of [XSD 1.1 §I.4](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/datatypes.html#changes), which lets “clearly equivalent durations like P2Y and P24M” have the same value; read literally, the six coordinates of §3.2.6 would also distinguish `P1Y` from `P12M`, which no processor does for bound facets and no W3C test requires. [XSD 1.1 §3.3.6.1](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/datatypes.html#duration) confirms that “two duration values are equal if and only if they are identical”; its preceding sentence, which orders a pair “equal” when all four sums are equal, contradicts that for pairs such as `P400Y` and `P146097D`, whose sums coincide at every reference dateTime, and is not selected. With §4.2.2, under which “a <> b is used to indicate the case when a != b and neither a < b nor b < a”, such a pair is unequal and incomparable. Every schema-level equality between duration bounds is therefore identity: the equality that admits an exclusive restatement (“or be equal to {value} in {base type definition}”, §4.3.8, §4.3.9) and the equality under a fixed base bound (“cannot specify a value for maxInclusive other than {value}”, §4.3.7.1 and its siblings), as for enumeration. `P400Y` neither restates nor keeps a fixed `P146097D`, nor the reverse: a derived exclusive bound equal to the base's only in order is an ordinary literal outside the base value space (`project/datatypes/bounds/exclusive-equal-maxExclusive-duration-years-over-days`, `project/datatypes/bounds/exclusive-equal-maxExclusive-duration-days-over-years`, `project/datatypes/bounds/exclusive-equal-minExclusive-duration-years-over-days`, `project/datatypes/bounds/exclusive-equal-minExclusive-duration-days-over-years`) and changes a fixed bound (`project/datatypes/bounds/fixed-maxInclusive-duration-years-over-days`, `project/datatypes/bounds/fixed-minInclusive-duration-days-over-years`, `project/datatypes/bounds/fixed-maxExclusive-duration-days-over-years`, `project/datatypes/bounds/fixed-minExclusive-duration-years-over-days`), while `P12M` restates `P1Y` and `PT24H` restates `P1D` (`project/datatypes/bounds/exclusive-equal-maxExclusive-duration-months-for-years`, `project/datatypes/bounds/exclusive-equal-minExclusive-duration-hours-for-day`, `project/datatypes/bounds/fixed-maxInclusive-duration-months-for-years`, `project/datatypes/bounds/fixed-minInclusive-duration-hours-for-day`, `project/datatypes/bounds/fixed-maxExclusive-duration-months-for-years`). An order-equal pair is not less, so it fails an exclusive bound. Whether it satisfies an inclusive bound is doubtful: §4.3.7.3 asks for a value “chronologically less than or equal to {value}”, which the equal sums satisfy, while §4.2.2 makes the pair incomparable and §3.2.6.3 counts an indeterminate comparison as false. The processor tie-break settles it: Xerces-J admits it, as does xmlschema, so it is admitted as a facet literal and as an instance value (`project/datatypes/bounds/restrict-maxInclusive-duration-years-at-days` valid, `project/datatypes/bounds/restrict-maxInclusive-duration-years-under-exclusive-days` invalid, `project/datatypes/bounds/instance-maxExclusive-duration-order-equal`); .NET and libxml2 order the pair strictly and do not observe the question. The asymmetry with equality is deliberate: §4.2.1 settles equality without doubt, so the tie-break is never reached there. A pair incomparable in order, such as `P1M` and `P30D`, satisfies no bound and restates nothing (§3.2.6.3, “indeterminate comparisons should be considered as "false"”; `project/datatypes/bounds/exclusive-equal-maxExclusive-duration-incomparable`, `project/datatypes/bounds/fixed-maxInclusive-duration-incomparable`). xmlschema agrees except on the incomparable restatement, where it lets an indeterminate comparison pass. Xerces-J takes order-equal durations for equal values in both relations, which §4.2.1 rules out. .NET orders durations as day counts with 365-day years and 30-day months, and libxml2 by a day range, so neither observes this pair's equality; libxml2 also rejects every restated exclusive bound. libxml2 orders `P400Y` above `P146097D`: it accepts a derived `maxExclusive` `P146097D` under `P400Y` and a derived `minExclusive` `P400Y` under `P146097D`, and rejects a derived `maxInclusive` `P400Y` under `P146097D`. |
| Duration / truncation prose | Duration's total-order paragraph refers to a timezone that its grammar lacks; invent no feasible timezone branch. Appendix D's right-truncation list omits gYearMonth despite its explicit type definition; specific grammars still govern. |
| Fundamental facets | Preserve printed property formulas separately from mathematical value-set cardinality: fixed-length integer lists classify finite; enumeration/pattern alone do not trigger atomic finite classification; Appendix C labels gMonth countably infinite. Union ordered explicitly excludes anySimpleType as common ancestor while bounded does not; add no unprinted common-primitive exclusion. Boolean validity cannot expose these component properties. |
| Bounds | Repeated equal exclusive endpoints have explicit allowance; do not substitute generic instance validity. A `minExclusive` or `maxExclusive` equal to the effective same facet of the base is admitted without base value-space membership, so the base's digit and enumeration facets and its bounds of that same facet do not apply to it. The valid-restriction clauses still compare it with the base's other bounds (`project/datatypes/bounds/exclusive-equal-maxExclusive-empty-base`), and the base pattern still applies to its spelling, as for every facet literal (`project/datatypes/bounds/exclusive-equal-maxExclusive-outside-base-pattern`). Restatement equality is value equality: for `time` the equality of the Time midnight boundary above, and for `duration` value identity (Duration equality and order above). A `time` literal with the value of the base facet restates it on whatever day its spelling places it, so the base's enumeration does not apply to it (`project/datatypes/bounds/exclusive-equal-maxExclusive-time-earlier-day-outside-base-enumeration`, `project/datatypes/bounds/exclusive-equal-minExclusive-time-later-day-outside-base-enumeration` valid, as Xerces-J and .NET find; xmlschema and libxml2 reject them). The same-side clause of the Time midnight boundary still orders the restatement against the base bound by un-wrapped time: one spelled on the day that does not loosen the base is admitted (`project/datatypes/bounds/exclusive-equal-maxExclusive-time-same-spelling`, `project/datatypes/bounds/exclusive-equal-maxExclusive-time-earlier-day`, `project/datatypes/bounds/exclusive-equal-minExclusive-time-later-day`), and one spelled on the day that loosens it is rejected (`project/datatypes/bounds/exclusive-equal-maxExclusive-time-later-day`, `project/datatypes/bounds/exclusive-equal-maxExclusive-time-later-day-negative-offset`, `project/datatypes/bounds/exclusive-equal-minExclusive-time-earlier-day`), where Xerces-J accepts it. Incomparable restriction endpoints need their actual prohibited-comparison wording. Inherited opposite lower-bound kinds retain the same-datatype versus same-step scope question; no blanket rewrite/rejection follows. |
| URI / language / precision | No implicit percent decoding, case folding, normalization or absolutization establishes anyURI equality. No modern language registry rejection follows from its historical pattern. [Double extrema, overflow and documented precision](SOURCES.md#floating-point-source-versions) remain separately qualified. |
| <a id="entity-member-selection"></a>ENTITY member selection / [10662](https://www.w3.org/Bugs/Public/show_bug.cgi?id=10662) | Full later ENTITY-context assessment is not adopted. In an `ENTITY|string` union a lexical NCName such as `missing` cannot prove normative string fallback: the later [Datatype Valid](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/datatypes.html#cvc-datatype-valid) and [String Valid](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#cvc-simple-type) rules separate datatype-valid member selection from ENTITY assessment against document declarations. The current unsupported ENTITY result occurs before facets, so making it immediately terminal could suppress legitimate fallback after an enclosing list or restricted union fails its datatype rules. A complete phase change would need nested value propagation and explicit facet-literal construction semantics; it is not selected under the DTD-unsupported contract. Conformance fallback witnesses use a non-NCName; NCName observations remain qualified evidence without conformance credit. |
| <a id="observation-product-modality"></a>Observation / product / modality | Missing public canonical/PSVI/component output is unobservable, not covered. Product exclusions and resource/precision refusals are not datatype-invalid oracles. Compatibility recommendations, discouraged constructs and generic error/recovery permissions do not waive explicit must/conformance requirements or create rejection rules. A finite `minOccurs` or `maxOccurs` above 4294967295 is a library resource limit, the runtime `uint32` cap of [resource limits](../RESOURCE_LIMITS.md), not a validity rule: nonNegativeInteger admits it. `project/architecture-maxoccurs-above-uint32-is-schema-limit` (`maxOccurs="4294967296"`) is accepted by .NET 10.0.14 and xmlschema 4.3.2; `project/architecture-minoccurs-above-uint32-is-schema-limit` (`minOccurs="4294967296"`) is accepted by .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13. Xerces-J 2.12.2 rejects both at its own `int` limit, and libxml2 rejects the `maxOccurs` case. |

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

Observed: Xerces-J 2.12.2 and .NET 10.0.14 accept both constructors, xmlschema
4.3.2 rejects the list and libxml2 2.9.13 rejects both. stE053 decides the
union. For the list item the sources stay doubtful, and the
[processor tie-break](#processor-tie-break) follows Xerces-J, which accepts it.
Cases: `project/datatypes/type-derivation/list-anysimpletype-constructor` and
`project/datatypes/type-derivation/union-anysimpletype-constructor`.

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

The same holds wherever the order is partial. A bound facet admits a value only
when the order relates it to the bound as the facet requires, and an
incomparable pair satisfies no bound. NaN therefore satisfies only an inclusive
NaN bound, through equality, and no other value satisfies a NaN bound. A zoned
and an unzoned `dateTime` within fourteen hours of each other, the endpoints
included, are incomparable, since §3.2.7.4 orders them only when one is less
or greater than the other at every offset from `+14:00` to `-14:00`
([Temporal boundaries](#temporal-boundaries)), and `P1Y` is incomparable with
`P365D` ([Duration equality and order](#duration-equality-and-order)). A NaN
bound declared under a base `minInclusive` of 0 lies outside the base value
space. Xerces-J 2.12.2 gives the project result in every case listed. Observed
with .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- .NET and xmlschema admit NaN under every finite bound and every value under a
  NaN bound, and order the mixed-timezone and duration pairs.
- libxml2 admits NaN under a finite `minInclusive`, non-NaN values under a NaN
  `maxInclusive` or `maxExclusive`, and the mixed-timezone values at and inside
  the boundary.
- .NET accepts an equal `minExclusive` and `maxInclusive`, or `minInclusive`
  and `maxExclusive`, in one step.
- xmlschema and libxml2 accept the NaN bound outside the base value space.

Cases: `project/datatypes/bounds/conformance-minexclusive-maxinclusive-equal`,
`project/datatypes/bounds/conformance-mininclusive-maxexclusive-equal`,
`project/datatypes/bounds/inherited-opposing-unordered-nan`,
`project/datatypes/numeric/nan-ordered-bounds/*`,
`project/datatypes/temporal/mixed-zone-boundary/*`,
`project/double-bounds-nan-equal` and
`project/runtime-duration-partial-order-strict`.

## I-016 — recursive ID-ness

**Status:** selected reading from [W3C bug 10662](https://www.w3.org/Bugs/Public/show_bug.cgi?id=10662)
and the XSD 1.1 [ID-related types](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#ch_id)
clarification (§G.1.7; [§3.17.5.2](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#sic-id));
the XSD 1.0 ID prohibitions are retained, and attribute defaults bind IDREFs as
accepted W3C `idZ012` expects (below).

XSD 1.0 states its ID rules for types "derived from ID", which literally
excludes lists and unions while [cvc-id](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-id)
still requires their values to be tracked. One predicate now decides ID-ness at
schema and run time. A simple type carries ID when it is `xs:ID`, is derived from
a type that carries ID, is a list whose item type carries ID, or is a union with
a member that carries ID. IDREF carriage follows the same recursion.

The type-level predicate drives every schema-time and static rule:

- no default or fixed value on an ID-carrying element declaration
  ([e-props-correct](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-props-correct)),
  attribute declaration or attribute use
  ([a-props-correct](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#a-props-correct));
- at most one ID-carrying attribute use per attribute set
  ([ct-props-correct](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#ct-props-correct));
- wildcard attributes are counted by their declaration's type
  ([cvc-complex-type](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-complex-type)
  5.1 and 5.2), so two `union(int, ID)` wildcard attributes conflict even when
  both values select `int`.

The ID/IDREF table stays value-level: a union contributes the projections of its
selected member, and a list contributes each item's. A `union(int, ID)` value `1`
adds no ID; a `list(ID)` value `a b` adds two, so duplicates within one list are
rejected and an IDREF may target a list item.

Xerces-J and xmlschema mostly agree; .NET and libxml2 apply the literal
"derived from ID" reading and accept defaults on ID unions and duplicate list
items. The literal 1.0 reading was rejected because it leaves a default on an
ID-carrying union able to create duplicate IDs and splits ID-ness between
schema checks and the cvc-id table. Fixtures:
`tests/corpus/project/identity/id-carriers/` and the corrected
`w3c/msMeta/SimpleType_w3c.xml/stE072`.

[cvc-id](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-id)
clauses 1 and 2 require every IDREF value assessed under an IDREF-carrying type
to name an ID in the document, and an ID value to occur once, whether it comes
from an attribute, element simple content, a type selected by `xsi:type` or a
lax wildcard's declaration. An element's default or fixed value counts: under
[cvc-elt](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-elt)
clause 5.1.2 the element is validated with that value as its content. An
attribute's default or fixed value counts too. The text alone is doubtful: the
[ID/IDREF table](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-id)
takes its eligible item set from items "successfully validated with respect to
an attribute declaration as per Attribute Locally Valid", which an attribute
supplied by an attribute use's default or fixed value never undergoes, and XSD
1.1 §G.1.7 presents "The validation rules for values of type xs:IDREF,
xs:ENTITY, or xs:ENTITIES are now enforced on default values" as a change. The
accepted W3C expectation `w3c/msMeta/IdentityConstraint_w3c.xml/idZ012` (an
`xs:IDREFS` attribute with `default="abc"` on an empty `root`, expected
invalid) decides it, and accepted W3C expectations rank above the
[processor tie-break](#processor-tie-break), which is therefore not reached: an
attribute absent from the instance and supplied by a default or fixed value
contributes to identity-constraint key-sequences and binds its IDREF values,
which must name an ID. This holds for local, global and referenced attribute
declarations, `fixed` as well as `default`, and IDREF restrictions, `xs:IDREFS`,
IDREF lists and `union(int, IDREF)`. Observed with Xerces-J 2.12.2, .NET
10.0.14, xmlschema 4.3.2 and libxml2 2.9.13: Xerces-J adds no ID/IDREF binding
for an attribute supplied by a default or fixed value, so it accepts each
missing target such an attribute names, while it resolves element defaults and
physical attributes; .NET and xmlschema count attribute defaults, and .NET
throws `InvalidCastException` on a `union(int, IDREF)` attribute default.
Cases: `project/corrections-default-list-union-idref` (instances
`attribute-missing-target` and `element-missing-target` are invalid),
`project/identity/default-idref-binding/defaults`,
`project/identity/default-idref-binding/identity-fields` and
`project/identity/default-idref-binding/id-attribute-default` (an `xs:ID`
attribute default is a schema error, so a default never supplies an ID). libxml2
2.9.13 never resolves IDREF values in schema validation (a single `xs:IDREF`
attribute naming no ID is accepted), tracks only attribute IDs (two `xs:ID`
elements with equal content are accepted), and treats only `xs:ID` itself, not
its restrictions, as ID for the value-constraint and one-ID-per-attribute-set
rules. On the wildcard rule of cvc-complex-type clause 5.2 it reports
"Unimplemented block". xmlschema 4.3.2 applies clause 5.2 only to ID attributes
that are present, while the rule counts every ID-carrying use in {attribute
uses}. Xerces-J 2.12.2 reports each. .NET 10.0.14 and xmlschema report the
unresolved IDREF and the element ID duplicates. Their list and union
differences are the carriage differences described above. .NET adds no ID for
an item of a list of `union(int, ID)`, so it accepts a repeated item and
rejects an IDREF naming one, and it resolves no item of a list of
`union(int, IDREF)`, so it accepts an attribute or element default of that type
naming no ID. xmlschema reports distinct ID items of a list of `union(int, ID)`
as duplicated, and counts each item of an `xs:ID` list attribute as an ID
attribute, so it rejects a two-item value as more than one ID attribute. Cases:
`project/identity/conformance-idrefs-repeated-builtin`,
`project/identity/conformance-idrefs-repeated-two`,
`project/identity/isolated-activation/dynamic`,
`project/identity/isolated-activation/lax`,
`project/identity/selected-default-idref/default`,
`project/identity/selected-default-idref/fixed`,
`project/identity/id-carriers/element-content`,
`project/identity/id-carriers/list-id`, `project/identity/id-carriers/union-id`,
`project/attributes/conformance-global-id-derived-default`,
`project/attributes/conformance-global-id-derived-fixed`,
`project/attributes/conformance-group-distinct-uses-two-id-types`,
`project/attributes/wildcard-id-declared-lax`,
`project/attributes/wildcard-id-declared-strict` and
`project/attributes/wildcard-id-use-order`.

On the carriage cases Xerces-J 2.12.2 gives the project result, and the
[processor tie-break](#processor-tie-break) supports the selected reading.
.NET 10.0.14 and libxml2 2.9.13 accept a default on an ID-carrying union,
including one whose ID member is declared later in the document. On wildcard
attributes .NET, xmlschema 4.3.2 and libxml2 accept two union attributes whose
values select the `int` member, and a list and a union carrier together, and
libxml2 accepts a list carrier beside a declared ID. libxml2 also accepts a
dangling IDREF under an enumeration of `xs:IDREF`. Cases:
`project/identity/id-carriers/forward-member-default-id`,
`project/identity/id-carriers/union-id-default`,
`project/identity/id-carriers/wildcard-carriers` and
`project/datatypes/enumeration/large-group-name-family-types`.

## I-017 — list identity and equality

**Status:** adopted [selected correction](SOURCES.md#selected-corrections):
XSD 1.0 identity-constraint
[value equality](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-identity-constraint)
of lists uses the XSD 1.1 [list identity](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/#identity)
of Datatypes §2.2.1 and the list rule of
[§2.2.2](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/#equality),
without its length-one identification.

XSD 1.0 Datatypes §2.5.1.2 and §4.2.1 and cvc-identity-constraint do not say
whether lists of different item types are equal. XSD 1.1 states the rule: two
lists are the same "if they are the same sequence of atomic values", and "there
is only one empty list".

Two lists are equal when they have the same length and their items are
pairwise equal. Empty lists are therefore equal whatever their item types. A
list never equals an atomic value: the empty list differs from the empty string,
and the one-item list `a` differs from the string `a`. The XSD 1.1 rule that
treats an atomic value as a length-one list was rejected because XSD 1.0 has
no such identification and it would make keys depend on whether a field's type
is atomic or a list.

Value equality and the comparable identity key are one contract: lists carry a
reserved tag outside the primitive-kind range, and each item carries its own
primitive kind. The correction is adopted rather than the processor tie-break
because Xerces-J's observed rule makes keys depend on item-type shape, which
the paragraph above rejects: it keeps lists unequal when their item types have
different primitive categories, and when one has a union item type and the
other an atomic one, even with equal items.

Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2
2.9.13:

- Xerces-J and .NET keep empty lists of `xs:int` and `xs:string` items
  distinct, and one-item and empty lists of a `union(int, string)` item type
  distinct from lists of `xs:int` or `xs:string` items; both equate empty
  `xs:int` and `xs:integer` lists, two lists of `xs:string` items under
  different list types, and union-item lists whose members are reordered.
  Xerces-J rejects an atomic `xs:int` keyref to a one-item `xs:int` list key.
- .NET, xmlschema and libxml2 equate a one-item list with an atomic value.
- xmlschema equates empty lists whatever their item types, and lists with
  equal items whatever their item types.
- libxml2 fails on every empty-list attribute ("No precomputed value
  available").

Cases: `project/identity/list-identity-keys` and
`project/identity/list-identity-item-types/*` (key, unique and keyref over
empty and one-item lists of `xs:int`, `xs:integer`, `xs:string` and union item
types, against each other and against atomic values).

## I-018 — nilled requires a governing declaration

**Status:** literal reading of XSD 1.0
[cvc-elt](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-elt)
clause 3 and
[cvc-assess-elt](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-assess-elt).

Clause 3 of Element Locally Valid (Element) applies only to an element with a
governing element declaration. An element assessed by type alone (a lax
wildcard match without a declaration, an `xs:anyType` child, or an undeclared
root selected by `xsi:type`) is never nilled: `xsi:nil` is still checked as an
`xs:boolean`, and the element's content is validated against its type. So
`xsi:nil="true"` with content is valid there, and `xsi:nil="true"` on an empty
`xs:int` element is invalid. A declared non-nillable element still rejects
`xsi:nil="true"`.

Xerces-J agrees on every fixture. .NET and libxml2 do not check `xsi:nil` as
`xs:boolean` without a declaration; xmlschema treats such an element as a
non-nillable declaration, except that it treats an element under a lax
wildcard with `xsi:type` as nillable, accepting it empty and rejecting it with
content. libxml2 2.9.13 also does not assess an undeclared
validation root through `xsi:type` and rejects it. Cases:
`project/elements/xsi-nil-without-declaration`.

## I-019 — schema document identity is the resolved location

**Status:** include identity is the adopted XSD 1.1 Structures
[§4.2.3](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/structures.html#compound-schema)
correction ([selected corrections](SOURCES.md#selected-corrections)): "If two
`<include>` elements specify the same schema location (after resolving relative
URI references) then they refer to the same schema document. If they specify
different schema locations, then they refer to different schema documents,
unless the implementation is able to determine that the two URIs are references
to the same resource".
XSD 1.0 has no identity rule. Applying the same identity to imports and explicit
roots is a library choice: 1.0 §4.2.3 and 1.1 §4.2.6 both leave applications free to
"ignore all but the first `<import>` for a given namespace".

A schema document is identified by its resolved source key. References that
resolve to one key are one document, contributing its components once. Distinct
keys are distinct documents even when their bytes are identical, so two copies
declaring the same global component violate
[sch-props-correct](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#sch-props-correct)
clause 2 (`schema.duplicate`). Callers that need copies at several locations
(for example two hints to `xml.xsd`) treated as one document map them to one key
in their resolver.

Rejected: content-fingerprint deduplication, because identical bytes do not
imply an identical include/import closure (relative references resolve against
each location); and keeping only the first document per namespace, because the
result depends on load order and hides genuine conflicts.

The library therefore applies resolved-location identity to includes, imports
and explicit roots alike. Imports of one namespace from distinct locations are
distinct documents; when they declare the same global component they conflict
under sch-props-correct clause 2. The sources permit keeping only the first
import, and Xerces-J, libxml2 and xmlschema do so; processing every import is
the other choice that latitude permits, not a doubtful reading, so the
[processor tie-break](#processor-tie-break) is not reached, and the
first-import choice is rejected for the reasons above. Fixtures: the `project/namespace-duplicate-*`
and `namespace-shared-include-*`/`namespace-repeated-import-*` cases in
`tests/corpus/project/namespaces/`. Observed with Xerces-J 2.12.2, .NET 10.0.14,
xmlschema 4.3.2 and libxml2 2.9.13:

- `project/namespace-duplicate-include-locations-conflict` (invalid): all four
  reject.
- `project/namespace-duplicate-identical-target-namespace-sources-conflict`
  (invalid; byte-identical documents as two explicit roots): Xerces-J accepts,
  keeping one grammar per target namespace; .NET and xmlschema reject; libxml2
  takes one root and reports blocked.
- `project/namespace-duplicate-import-locations-conflict` (invalid; one
  namespace imported from two locations): Xerces-J, xmlschema and libxml2
  accept, libxml2 warning that it skips the second import; .NET rejects.
- `project/schema-id-document-local` (valid; two no-namespace explicit roots):
  Xerces-J keeps only the first root, in either order and whether or not their
  `schema/@id` values collide, so it rejects the instance of the second root's
  element (`second`); .NET and xmlschema accept; libxml2 takes one root and
  reports blocked.

The explicit roots of one compilation supply one component set, independent of
their order. An `<import>` without `schemaLocation` is satisfied by any root
that supplies the namespace: [§4.2.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#composition-schemaImport)
says that without it "the schema author is leaving the identification of that
schema to the instance, application or user", and the explicit sources are the
caller's identification. The text delegates identification rather than
leaving a reading doubtful, so the [processor tie-break](#processor-tie-break)
is not reached. The library's identification is the whole root set, so the
verdict does not depend on root order; `TestNamespaceAttributeSourceResolution`
in `schema_namespace_attributes_test.go` compiles the same sources through a
resolver, as explicit roots and as reversed roots, and load-order dependence is
a rejected alternative in [ARCHITECTURE.md](../../ARCHITECTURE.md). Xerces-J
2.12.2 (`XMLSchemaFactory.newSchema(Source[])`) loads roots in order and
reports `src-resolve` when the referencing root precedes the supplier, so it
gives both verdicts for one source set: it rejects importer-then-supplier and
accepts supplier-then-importer, and it accepts when the import names an
explicit `schemaLocation`. .NET 10.0.14 and xmlschema 4.3.2 accept in either
order, and libxml2 2.9.13 takes one root and reports blocked. Cases:
`project/attribute-group-wildcard-expressible`,
`project/local-attribute-wildcard-expressible`,
`project/namespace-declarations-xlink-locationless-roots`,
`project/namespace-declarations-xml-locationless-roots`, their supplier-first
variants `project/attribute-group-wildcard-expressible-reversed`,
`project/local-attribute-wildcard-expressible-reversed`,
`project/namespace-declarations-xlink-locationless-roots-reversed` and
`project/namespace-declarations-xml-locationless-roots-reversed`, which Xerces-J
accepts,
`project/namespaces/locationless-import-root-order/importer-then-supplier`
(Xerces-J rejects),
`project/namespaces/locationless-import-root-order/supplier-then-importer`,
`project/xsd-components-attribute-group-imported`,
`project/xsd-components-attribute-imported`,
`project/xsd-components-element-imported`,
`project/xsd-components-group-imported` and
`project/xsd-components-type-imported`. In the five `xsd-components-*-imported`
cases the supplier of the XML Schema namespace components is the second root:
Xerces-J reports `src-resolve` in manifest order and accepts with the supplier
first, so the cause is root order, not the namespace
([I-006](#i-006--xsdxsi-schema-references) owns the import exemption); .NET and
xmlschema accept and libxml2 reports blocked.

## I-020 — XML name characters follow the Fifth Edition

**Status:** selected edition for name characters. XSD 1.0 cites
[XML 1.0 Second Edition](https://www.w3.org/TR/2000/REC-xml-20001006/); the
library uses the
[Fifth Edition](https://www.w3.org/TR/2008/REC-xml-20081126/#NT-NameStartChar)
`NameStartChar` and `NameChar` productions instead.

One name grammar, owned by `internal/lex`, serves XML parsing, `xs:Name`,
`xs:NCName`, `xs:NMTOKEN`, QName parts, schema component names and the regex
`\i` and `\c` classes. Mixing editions would accept documents whose element
names fail as `xs:Name` values. The Fifth Edition is the edition current XML
parsers accept, and it accepts every Second Edition name used in practice.
Only name characters change: the XML declaration grammar and every other
dependency stay as described in [SOURCES.md](SOURCES.md). In particular
`VersionNum` keeps the Second Edition production [26], so a well-formed version
other than `1.0` (such as `1.1`, `1.5` or `1.0.0`) is `unsupported.xml_version`,
while a malformed one (`""`, `1,0`) and a malformed `EncName` are XML
well-formedness errors. The Fifth Edition `1.x`-as-1.0 rule is not adopted.

In their name datatypes Xerces-J, .NET and libxml2 use Second Edition classes
and reject characters such as U+0132, U+2070 and U+10000, and xmlschema uses
Fifth Edition classes except for supplementary characters. U+00D7 is excluded
in every edition. Fixture: `tests/corpus/project/datatypes/name-fifth-edition-chars`.
In element and attribute names the XML parsers of Xerces-J 2.12.2, .NET 10.0.14
and xmlschema 4.3.2 reject U+02FF and U+037F as well as supplementary
characters, and the parser of libxml2 2.9.13 admits all of them
(`project/xml/qname-admission-fifth-edition-ranges`).

Accepted W3C expectations decide the edition, so the
[processor tie-break](#processor-tie-break) does not apply.
`msMeta/Regex_w3c.xml` reZ006i expects its `\c` instance valid for XML 1.0,
citing bug 13606 and the Fifth Edition name characters. Saxon
`XmlVersions.testSet` xv100c and xv100i (whose metadata references the XSD 1.1
drafts) support the same classes.

## I-021 — chameleon reference checks use the effective target namespace

**Status:** source-qualified reading of
[src-include](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-include)
clause 2.1 and
[src-import](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-import)
clauses 1.1–1.2, chosen by the [processor tie-break](#processor-tie-break).

Both clauses name the `targetNamespace` [attribute] of the enclosing `<schema>`
and predate the chameleon transformation, so the sources leave a chameleon's
own references doubtful. A chameleon is checked as the document it becomes in
each target context: in a view adopting `urn:t` it may include `urn:t` or
no-namespace documents, may import without `namespace`, and may not import
`urn:t`. In its no-namespace view the ordinary rules apply. A document included
into several namespaces must satisfy the checks in every view.

Rejected: checking the declared (absent) attribute literally. It makes the
reference validity of a document independent of its includer, but every
measured processor rejects it on at least one shape; libxml2 applies the
effective namespace to include only, so, directly or through a transitive
include, it admits a chameleon's import of the adopted namespace and rejects its import without
`namespace`. Xerces-J 2.12.2 gives the project result on the transitive shapes.
Fixtures: `project/composition/chameleon-includes-same-tns`,
`project/composition/chameleon-imports-effective-ns`,
`project/composition/chameleon-import-without-namespace`,
`project/composition/chameleon-transitive-import-effective-ns` and
`project/composition/chameleon-transitive-import-without-namespace`.

## I-022 — undeclared lax attributes are unknown field values

**Status:** source-qualified reading of
[cvc-identity-constraint](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-identity-constraint)
clause 3 for attributes that
[lax assessment](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-assess-attr)
leaves without a declaration, chosen by the
[processor tie-break](#processor-tie-break).

Clause 3 requires a selected field node to have a simple type. An attribute
admitted by a lax wildcard with no matching declaration has no type at all, so
the sources leave its field value doubtful: an error, a string, or nothing.
Such a field holds an *unknown* value:

- it is a selected node, so `key` field presence holds and a key's nillable
  rule still applies;
- it equals no value, so a row holding one never takes part in `unique` or
  `key` duplicates and a key row holding one can be referenced by nothing;
- a `keyref` row holding one matches no key and reports the keyref failure;
- it adds no field cardinality: a typed or nilled node selected by the same
  field supersedes it, two unknown nodes stay unknown, and two typed nodes
  remain multiple values. Clause 3's one-member limit is stated together with
  its simple-type requirement, so it shares that doubt. The measured rows
  decide it: Xerces, .NET and libxml2 accept a typed and an unknown node under
  one field (`v-typed-and-unknown-same-node`), and all four providers accept two
  unknown nodes (`v-two-unknown-one-field`);
- an absent field outranks it, so the row is incomplete.

This applies only to lax assessment without a declaration. Skip-admitted
attributes follow I-002 and supply nothing. Strict wildcards without a
declaration and disallowed attributes are invalid attributes, and the fields
selecting them are invalid.

Rejected:

- the literal no-simple-value error. All four providers accept equal undeclared
  attributes of an element that a lax element wildcard admits under `unique`
  (`v-lax-equal`).
- string comparison. Xerces, .NET and libxml2 accept equal undeclared values
  under `unique`.
- treating the field as absent. Xerces and .NET accept a key whose only field is
  an undeclared lax attribute.

The project departs from Xerces twice:

- Xerces accepts two typed attributes selected by one field on one node. The
  clause 3 limit of one node is explicit, so the project reports multiple
  values, as .NET, libxml2 and xmlschema do.
- Xerces matches an unknown keyref value to an unknown key value (both `null`).
  Under this reading an unknown value equals nothing. .NET and libxml2 also
  reject that instance, though on the key.

Other providers depart from the project as follows:

- xmlschema compares an undeclared attribute that an attribute wildcard admits
  by its string. It reports equal ones as duplicates under `unique` and `key`
  (`v-attribute-wildcard-equal`, `v-two-field-unknown-equal`,
  `v-unknown-key-rows-equal`), matches keyref and key values through them
  (`i-unknown-key-and-unknown-ref`, `i-unknown-key-row-not-referable`,
  `i-unknown-ref-equals-key`, `i-two-field-ref-one-unknown`), and reports
  multiple values when one is selected beside a typed node
  (`v-typed-and-unknown-same-node`, `v-typed-and-unknown-different-nodes`). It
  also checks a keyref row whose other field is absent, reporting the value
  `(None, '2')` not found (`v-two-field-ref-unknown-and-absent`).
- libxml2 treats an undeclared lax attribute as absent: a key field selecting
  one fails because not all fields evaluate to a node (`v-lax-present`,
  `v-lax-equal`, `v-two-field-unknown-equal`, `v-unknown-key-rows-equal`), and a
  keyref row holding one raises no keyref error (`i-unknown-ref-equals-key`,
  `i-two-field-ref-one-unknown`).
- .NET reports a key field selecting an undeclared attribute that an attribute
  wildcard admits as an empty node (`v-two-field-unknown-equal`,
  `v-unknown-key-rows-equal`), and a keyref row holding one raises no keyref
  error (`i-unknown-ref-equals-key`, `i-two-field-ref-one-unknown`).
- .NET and xmlschema accept a key row with no field node on an element that a
  lax element wildcard admits without a declaration (`i-lax-missing`).
- Xerces accepts a key whose only field selects an attribute that a skip
  wildcard admits (`i-skip-attribute-present`).

Fixtures: `project/identity/undeclared-attribute-field-unique`,
`project/identity/undeclared-attribute-field-key`,
`project/identity/undeclared-attribute-field-keyref` and, for two typed
attributes under one field, `project/structures-field-distinct-attribute-union`.
Per-instance dissent is recorded as manifest `oracle` metadata.

## I-023 — unresolved references

**Status:** source-qualified reading of
[QName resolution](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-resolve)
and [Missing Sub-components §5.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#conformance-missing),
following the accepted W3C expectations of `w3c/saxonMeta/Missing.testSet`.
The record orders its sources as the [processor tie-break](#processor-tie-break)
does and defines no precedence of its own.

`src-resolve` requires every QName reference to resolve. §5.3 describes how
"the resolution of such QNames may fail, resulting in one or more values of or
containing absent where a component is mandated", and modifies assessment of
an item validated against a component "any of whose properties has or contains
such an absent value": an attribute fails as if clause 1 of Attribute Locally
Valid had failed, an element as if clause 1 of Element Locally Valid (Element)
had failed. The schema therefore loads with the property absent, as the
accepted W3C expectations confirm: `missing001` (element `type`), `missing002`
(`substitutionGroup`), `missing003` (element `type` with `substitutionGroup`)
and `missing006` (list `itemType`) are valid. They also decide the reference
kinds the schema cannot carry absent: `missing004` and `missing005` (simple and
complex `base`) are invalid, and `w3c/msMeta` `elemL002`, `groupB008`, `attE002`, `attgC008` and
`idC038` reject an unresolved element, model group, attribute and attribute
group reference and keyref `refer`. Xerces-J, .NET, xmlschema and libxml2
reject all six Missing schemas; an accepted W3C expectation outranks processor
behaviour, so their results are manifest `oracle` metadata on those cases.

| Reference | Unresolved | Source |
| --- | --- | --- |
| `type` on an element declaration, list `itemType` | Loads; an item assessed against the declaration or type is invalid | W3C `missing001`, `missing003`, `missing006` |
| `substitutionGroup` | Loads; the member has no substitution group affiliation and, without its own type, validates as `anyType` (below) | W3C `missing002`, both of whose instances are valid |
| `type` on an attribute declaration; union `memberTypes`; `default` or `fixed` on a declaration whose type is missing | Loads; an item assessed against the declaration or union is invalid, and the value constraint is not checked (`project/composition/unresolved-references/attribute-type-missing`, `project/composition/unresolved-references/union-member-missing`, `project/composition/unresolved-references/element-type-missing-default`, `project/composition/unresolved-references/attribute-type-missing-value-constraint`, and a list of such a union, `project/unavailable-scalar-union-list`) | §5.3: a failed resolution leaves the property “absent”, and only an item “validated with respect to a component [...] any of whose properties has or contains such an absent value” fails. The text decides it, as W3C `missing001` and `missing006` confirm for element `type` and list `itemType`, so processor results do not; Xerces-J, .NET, xmlschema and libxml2 reject each schema, which the cases record as `oracle` metadata |
| `base` | Rejected | W3C `missing004`, `missing005` |
| `ref` of an `xs:element`, `xs:group`, `xs:attribute` (use other than prohibited) or `xs:attributeGroup` that maps to a component; keyref `refer` | Rejected (`project/composition/unresolved-references/element-ref-missing`, `group-ref-missing`, `attribute-ref-missing`, `attribute-group-ref-missing`, `keyref-refer-missing`) | W3C `elemL002`, `groupB008`, `attE002`, `attgC008`, `idC038`; Xerces-J, .NET, xmlschema and libxml2 reject each project case |
| `xs:group ref` with `minOccurs` and `maxOccurs` 0; `xs:attribute ref` with `use="prohibited"` | Not resolved and not import-authorized, and a group that refers to itself only through such a reference is not circular (`project/composition/unresolved-references/group-ref-zero-unimported`, `project/group-zero-occurrence-absent-content`, `project/group-zero-occurrence-choice`, `project/group-zero-occurrence-mutual`, `project/group-zero-occurrence-self`, `project/prohibited-no-component-missing-imported-attribute-prohibited`) | §3.7.2: such a group reference "corresponds to no component at all", and [I-008](#i-008--prohibited-attribute-use-with-fixed), so there is no reference to resolve and no particle for Model Group Correct clause 2 to follow. The text decides it, so processor results do not; .NET accepts the group reference, Xerces-J, xmlschema and libxml2 reject it, and Xerces-J and xmlschema report a circular group through it. Xerces-J, xmlschema and libxml2 reject a prohibited attribute `ref` that names no declaration, which .NET ignores with a warning |
| A name in the XML Schema namespace | Rejected (`project/composition/unresolved-references/xsd-namespace-type-missing`, `xsd-namespace-member-missing`) | The name needs no import ([I-006](#i-006--xsdxsi-schema-references)), and an explicit source may supply further components in the namespace (W3C addB182), but here neither a builtin nor an explicit source supplies it. §5.3 does not distinguish namespaces, so the text alone leaves the tolerance doubtful; an accepted W3C expectation decides it for a union member: `msMeta/SimpleType_w3c.xml` stK002 (member `xsd:timeDuration`) is invalid, and with that name replaced by a builtin the library compiles the schema, so the name is its only defect. No accepted witness has an element or attribute `type` as its only defect, so for `xsd-namespace-type-missing` the processor tie-break settles it by Xerces-J, which rejects, although it reports the missing import (`src-resolve.4.2`) that I-006 removes; .NET, xmlschema and libxml2 also reject both project cases. Library owner: `typeQNameMayBeUnavailable` |
| A name in a namespace the document does not import | Rejected (`project/composition/unresolved-references/type-unimported-namespace`) | `src-resolve` clause 4 applies before any of the tolerances above |
| `ref`, `type`, `default` and `fixed` of an `xs:element` with `minOccurs` and `maxOccurs` 0 | Not resolved, not import-authorized and not checked. The rules decided at its start tag and at every start tag inside it still apply: its QName syntax and prefix bindings, the occurrence relation and the `xs:all` range, XPath, pattern, a repeated facet, size and `xs:whiteSpace` values, and namespace declaration names. The definitions nested in it keep their own mapping and checks: its inline type definition, as for a prohibited attribute, and its identity constraints, which map to identity-constraint definitions and take part in name uniqueness, are evaluated for no instance element, and resolve a keyref `refer` that names them as any other identity constraint does; a keyref value never matches one. As no particle, it takes part in no Element Declarations Consistent, Unique Particle Attribution or particle restriction check (`project/content-models/element-zero-occurrence/*`) | §3.3.2 maps the item to “no component at all”, as §3.7.2 does for the group reference above, and §3.11.2 maps every `xs:key`, `xs:unique` and `xs:keyref` to an identity-constraint definition. The text decides it, so processor results do not. Of the schemas that a skipped `ref`, `type`, `default` or `fixed` would otherwise make invalid, Xerces-J rejects every one, .NET accepts all, xmlschema only the NOTATION type, and libxml2 only the missing reference and the NOTATION type. All four agree on the identity-constraint schemas; on the keyref instances .NET rejects the absent value and xmlschema fails with an internal error. On the particle checks Xerces-J rejects the zero-count restriction particle, xmlschema the Element Declarations Consistent case and libxml2 the wildcard case, and libxml2 accepts the base whose only particle is zero-count |
| `xs:all`, `xs:choice` or `xs:sequence` with `minOccurs` and `maxOccurs` 0 | Maps to no particle, so its enclosing model group has none there. Element Declarations Consistent, Unique Particle Attribution and the circular-group rule of Model Group Correct clause 2 do not reach its children, so a group reference inside it creates no circularity. Its children still map by their own occurrence and keep their own checks: value constraints, facets, `src-resolve`, and identity constraints, which a keyref may still name (`project/compositor-zero-*`) | §3.8.2 maps each compositor to a particle “unless minOccurs=maxOccurs=0, in which case the item corresponds to no component at all”, and Model Group Correct clause 2 forbids “within the {particles} of a group” a particle at any depth “whose {term} is the group itself”. The text decides it, so processor results do not. Xerces-J 2.12.2 and xmlschema 4.3.2 report circularity through the compositor, and xmlschema applies Element Declarations Consistent and Unique Particle Attribution across it. .NET 10.0.14 skips every check below it and cannot resolve a keyref to a key declared there. libxml2 2.9.13 skips a repeated facet. Xerces-J agrees with the project on every descendant check |

A tolerated missing `substitutionGroup` head leaves `{substitution group
affiliation}`, an optional property, absent. The §3.3.2 `{type definition}`
mapping gives a declaration without its own type the type of its head,
otherwise the ur-type, and does not say which branch an unresolved head takes;
without an explicit `type` or inline type, the member validates with `anyType`.
With a resolved head, an omitted member type is inherited through the
affiliation chain before value constraints are checked; it does not fall back
to `anyType` while the head's type is still being completed. Evidence:
`TestSubstitutionMemberWithMissingHeadUsesDefaultType` in
`internal/schema/schema_compile_external_test.go`.

## I-024 — printed primitive-type summary

**Status:** published-text discrepancy; Part 2 definitions govern. Not an
approved erratum or a new primitive inventory.

[Structures §3.14.7](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#d0e16395)
prints `number` in its primitive list and omits `decimal`, `QName` and
`NOTATION`; its earlier
[simple-type example](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#Simple_Type_Definitions)
also uses `xs:number`. The same summary says primitive facets are empty. Its
own reference points to the actual
[Part 2 primitive definitions](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#built-in-primitive-datatypes),
which include decimal, QName and NOTATION. Part 2's
[whiteSpace rules](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#rf-whiteSpace)
specify preserve for string and fixed collapse for the other atomic primitives.
The [Appendix A primitive sketches](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#schema)
are informative displays of those definitions.

The library follows the Part 2 definitions: the printed summary supplies no
`xs:number` builtin and does not remove required whitespace processing. No
approved amendment replaces that paragraph.

## I-025 — identity values of unrelated restrictions

**Status:** published formal rule governs over introductory prose; not a new
equality policy or approved patch.

The [§3.11.1 introduction](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#Identity-constraint_Definition_details)
says differing-type values can be equal only when one type derives from the
other. The formal
[Identity-constraint Satisfied](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-identity-constraint)
rule compares tuples through Part 2
[Equal](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#equal)
on type-determined values.

Two independently defined restrictions of integer that both supply `1` hold the
same integer value, so no declared-field-type ancestry veto applies. Two
distinct simple children `v` and `w`, selected by `v|w` with field `.`, supply
equal complete tuples for one `unique` constraint. Different declared type
names do not leave this result unspecified.

## I-026 — element declarations consistent counts declarations

**Status:** source-qualified reading of
[Element Declarations Consistent](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-element-consistent),
resolved by the normative
[schema for schemas](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#normative-schemaSchema)
and corroborated by the final
[XSD 1.1 wording](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#cos-element-consistent).

The 1.0 text names "two or more element declaration particles with the same
{name} and {target namespace}", but particles have neither property, and its
implicit-containment definition speaks of containing an element declaration.
Read as a particle count, the rule would reject the schema for schemas itself:
`xs:schema` references `xs:annotation`, whose type is anonymous, from two
particles. XSD 1.1 states the rule over "two or more element declarations". The
library compares distinct element declaration components: one declaration
reached through several `ref` particles, repeated or nested group references,
base and extension particles, or implicit substitution-group containment
(a head referenced twice, or a head and its member) counts once. Distinct
same-named declarations, global or local, still need one named top-level type.
No other XSD 1.1 EDC clause is adopted: the wildcard clause, type tables and
open content stay out of scope.

Rejected: counting particles literally. It invalidates the normative schema for
schemas and a common authoring pattern, and no measured processor applies it.
Fixtures: `project/structures-edc-reused-anonymous` and
`project/content-models/edc-same-declaration/*`; the distinct-declaration
opponents are `project/structures-edc-two-anonymous`,
`project/content-models/edc-same-declaration/global-ref-and-local-anonymous`
and `project/content-models/edc-same-declaration/two-groups-distinct-local-anonymous`.
libxml2 2.9.13 accepts every distinct-declaration opponent, same-named
declarations whose types are distinct, anonymous or named, including
`project/content-models/edc-same-declaration/local-different-named-types`;
Xerces-J 2.12.2 rejects each.

## I-027 — derivation control and actual substitution groups

**Status:** published XSD 1.0 rules; the text decides each case except union
membership under clause 2.3, which the
[processor tie-break](#processor-tie-break) settles below.

[Substitution Group OK (Transitive)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-equiv-derived-ok-rec)
clause 2.3 blocks a member D for a head C when any derivation method involved in
deriving D's type from C's type is in the union of the blocking constraint, C's
type's {prohibited substitutions} and the {prohibited substitutions} "of any
intermediate {type definition}s in the derivation". The method set is the whole
chain's: an intermediate type that blocks extension blocks D even when the
extension step lies above that intermediate. The blocking constraint is the
{disallowed substitutions} of the head being substituted. The `block` of an
intermediate element applies only when that element is itself the head.

[Element Locally Valid (Element)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-elt)
clause 4.3 derives an `xsi:type` from the declared type "given the union of the
{disallowed substitutions} and the {type definition}'s {prohibited
substitutions}", through every step down to the declared type and no further,
including when the declared type is `xs:anyType` or `xs:anySimpleType`. An
atomic type derives from `xs:anyType` by restriction.
[Type Derivation OK (Simple)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-st-derived-ok)
clause 2.1 makes a set containing restriction block every distinct simple type,
including a member type selected by `xsi:type` under a union.
[Element Declaration Properties Correct](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-props-correct)
clause 4 checks a member's type against the head's {substitution group
exclusions}, and cvc-elt clause 2 rejects an abstract declaration, nilled or not.

**Union membership.** Whether membership in a union contributes a derivation
method to clause 2.3 is doubtful. Type Derivation OK (Simple) clause 2.1 makes
a block set containing restriction block every distinct simple type, while its
clause 2.2.4 derives a member type from its union by membership, which is no
{base type definition} step. No adopted correction decides it, and no accepted
W3C test does: the union-typed heads of `particlesZ012`, `particlesZ015` and
`particlesZ020` have no block, `elemT071` and `elemT072` are unblocked, and
`addB114` exercises only `xsi:type`. The processor tie-break settles it by
Xerces-J: for clause 2.3 the method set is the base-chain steps from D's type to
C's type or, when C's type is a union, to the member that D's type derives
from, through nested unions; membership itself contributes no method. A head
with `block="restriction"` therefore accepts a member typed by a member of its
union, and blocks a member whose type restricts that member type.
Element Declaration Properties Correct clause 4 (`final`) and cvc-elt clause 4.3
(`xsi:type`) still treat every distinct simple type as restriction. Cases:
`project/elements/substitution-union-member-restriction-block` (instance
`member-blocked` is valid) and `project/elements/substitution-union-member/*`.
Xerces-J gives the project result on each, except that it accepts
`restricted-union-head-member-is-union-member`, whose head type is a union
restricted by a facet that [I-010](#i-010--restricted-unions-retain-their-constraints)
gives no members. .NET treats membership as restriction under a block, and also
blocks a complex extension of a member type under `block="restriction"`.
xmlschema treats membership as restriction under a block and as extension under
`block="extension"` and `final="extension"`, and accepts a member typed by an
inner union of the head's union type. libxml2 times out on the union-headed
schemas whose member it accepts.
A facet-free simple-content restriction defines no new simple type, so only the
base complex type's `final` applies to it
([Derivation Valid (Restriction, Complex)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#derivation-ok-restriction)
clause 1).

The [Substitution Group](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-equiv-class)
definition makes a head's ·substitution group· the members of its affiliation
closure whose {abstract} is false and that are validly substitutable under the
head's {disallowed substitutions}. Every rule that reads the term links to that
definition: implicit containment for
[Element Declarations Consistent](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-element-consistent)
and Unique Particle Attribution, and the generated choice of
[Particle Valid (Restriction)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-particle-restrict)
clause 2.1, which clause 2.2 may then find pointless. An abstract member
therefore conflicts with nothing under Element Declarations Consistent, and an
abstract head with one concrete member becomes a choice of that member alone.
Xerces-J counts abstract declarations in both places, and the definition
decides against it.

Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- Xerces-J keeps abstract declarations in implicit containment and in the
  clause 2.1 choice, and agrees with the project otherwise.
- .NET ignores type {prohibited substitutions} for substitution, checks element
  blocks per affiliation link instead of against the head being substituted,
  ignores the element block set under `xsi:type` when the declared type is a
  ur-type, ignores the head's {substitution group exclusions}, accepts a nilled
  abstract declaration and omits implicit containment from Element
  Declarations Consistent. It rejects a restriction that replaces an element
  by a reference to an abstract head whose substitution group holds only that
  element.
- xmlschema ignores intermediate type blocks, extends a restriction block or
  `final` past the declared type, drops members reached through an abstract or
  blocking intermediate element, keeps abstract declarations in particle
  restriction and rejects a complex simple-content type under a union. It
  omits implicit containment from Element Declarations Consistent, and accepts
  a restriction that replaces a head particle by a member particle with
  `maxOccurs="2"`.
- libxml2 checks only the last derivation step against the head type's block,
  omits implicit containment and particle restriction, applies a simple base's
  `final` to a facet-free simple-content restriction, and cannot construct a
  simple-content restriction of an extension of `xs:anySimpleType`.

Cases: `project/elements/chains-block-owners-*`, `project/elements/chains-depth3-*`,
`project/elements/continuation-type-block-*`, `project/elements/handoffs-xsi-*`,
`project/elements/handoffs-affiliation-s-restriction`,
`project/content-models/chains-affiliation-middle-*`,
`project/content-models/implicit-substitution-consistency/abstract-member`, `project/content-models/implicit-substitution-consistency/conflicting-type`, `project/content-models/implicit-substitution-consistency/transitive-conflicting-type`,
`project/content-models/substitution-intermediate-type-*`,
`project/content-models/substitution-boundaries/abstract-blocked-intermediate`,
`project/content-models/substitution-normalize-both-ranges-d-literal-member`,
`project/content-models/substitution-normalize-mapandsum-dab-repeated-first`,
`project/content-models/substitution-normalize-mapandsum-dba-repeated-first`,
`project/content-models/oracle-ordering-both-singleton-ranges`,
`project/content-models/oracle-ordering-both-singleton-unit`,
`project/content-models/oracle-ordering-derived-singleton-control`,
`project/namespace-substitution-group-declared-type-blocks-restriction`,
`project/namespace-substitution-group-simple-content-any-type-head-final-restriction`,
`project/namespace-substitution-group-simple-content-complex-type-derives-from-simple-head`,
`project/namespace-substitution-group-transitive-intermediate-block`,
`project/namespace-xsi-type-allows-simple-type-for-any-type`,
`project/simple-content-final-base-without-facets` and
`project/structures-abstract-declaration-and-actual-type`.

## I-028 — content-model checks

**Status:** published XSD 1.0 rules with no interpretive question, except the
cases the [processor tie-break](#processor-tie-break) settles below.

The library applies
[Unique Particle Attribution](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-nonambig),
[Element Declarations Consistent](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-element-consistent)
(counted as in [I-026](#i-026--element-declarations-consistent-counts-declarations)),
[Particle Valid (Restriction)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-particle-restrict)
with its pointless-group clause 2.2 and every case it dispatches to
([NameAndTypeOK](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-NameAndTypeOK),
[Recurse](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-Recurse),
RecurseLax, RecurseUnordered,
[MapAndSum](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-MapAndSum),
NSCompat, NSSubset with
[Wildcard Subset](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-ns-subset),
and [NSRecurseCheckCardinality](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-NSRecurseCheckCardinality)
as [I-004](#i-004--published-member-minimum-rule) repairs it), the empty-content
clause 5.3 of
[Derivation Valid (Restriction, Complex)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#derivation-ok-restriction)
and the content-type clause 1.4 of
[Derivation Valid (Extension)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-ct-extends).
A particle with `minOccurs` and `maxOccurs` 0 maps to no component
(§3.3.2), so it admits no element, and a named model group needs a model group
(§3.7.2). Where a particle shared through two group references makes Unique
Particle Attribution doubtful (`project/structures-group-two-ref-choice`), the
processor tie-break settles it by Xerces-J, which agrees with the project.

**Single-member all groups (rule P).** Clause 2.2 of Particle Valid
(Restriction) calls a group with one member pointless, and its prose names the
particle's range for `sequence` and `choice` but not for `all`, so whether an
`all` with one member and `minOccurs="0"` is pointless is doubtful. The
processor tie-break settles it by Xerces-J, with which .NET agrees on every
control: a single-member `all` is pointless only where the particle containing
it has {min occurs} and {max occurs} 1, as for `sequence` and `choice`. An
`all` with `minOccurs="0"` keeps its group and range and goes through the case
table; its range is not folded into its member. So a sequence of one optional
element does not restrict an optional single-member `all`
(`project/content-models/pointless-groups/singleton-all-outer-optional`,
invalid), while an optional sequence restricts it through RecurseUnordered, and
a required `all`, sequence or choice of one member reduces to that member.
Cases: `project/content-models/pointless-groups/optional-singleton-all-occurrence`
and `project/content-models/pointless-groups/singleton-all-occurrence/*`.
libxml2 accepts every one of these schemas.

**Declared pointless groups.** Clause 2.2 of Particle Valid (Restriction)
ignores pointless groups, and its text does not say whether a group is tested
as declared or again once its own pointless children are ignored: a choice of
a wildcard and an empty choice has two members as declared and one after the
empty choice is ignored. No adopted correction or accepted W3C test decides
it, so the processor tie-break settles it by Xerces-J
(`XSConstraints.getNonUnaryGroup` and `gatherChildren`). A group is a pointless
singleton only when it is declared with one member and its particle is 1..1,
and that test is not repeated after its children are reduced. Within its
parent, a 1..1 child group of the same compositor is replaced by its members,
and a 1..1 child group that contains no element or wildcard is ignored. A
required choice declared with no members matches nothing
([I-042](#i-042--empty-choice)), so it is ignored only inside a choice. The
choice that clause 2.1 generates for a substitution group is not declared and
still reduces to its only member
([I-027](#i-027--derivation-control-and-actual-substitution-groups)). Cases:
`project/content-models/pointless-groups/declared/*`. Xerces-J gives the
project result in every case except the two I-042 owns; .NET and libxml2
accept every invalid restriction in the set; xmlschema accepts the invalid
restrictions of a base with an ignored group beside a wildcard and keeps
singleton groups in a derived type (`derived-*-nested-elt`, invalid there).

**Inherited ur-type wildcard.** NSSubset clause 3 waives the processing
strength "unless B is the content model wildcard of the ·ur-type
definition·". A type that extends xs:anyType without explicit content takes
xs:anyType's {content type} (§3.4.2 clause 3.2.1), and a mixed one with empty
explicit content holds xs:anyType's particle (clause 3.2.3), so whether a
restriction of such a type compares against the ur-type wildcard is doubtful.
The processor tie-break settles it by Xerces-J, which checks particle
restriction against every base except xs:anyType itself: only a restriction
whose {base type definition} is xs:anyType satisfies clause 5.1 of
[Derivation Valid (Restriction, Complex)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#derivation-ok-restriction),
and any other restriction compares the inherited wildcard as an ordinary lax
`##any` wildcard 0..unbounded. Attribute wildcards follow clause 4.3, whose text
names the immediate base. Cases: `project/content-models/particle-restriction/inherited-urtype-wildcard`
and the restrictions under `project/content-models/type-derivation/urtype-content/*`.
.NET and xmlschema accept a lax or strict wildcard restricting a mixed empty
extension and otherwise agree; libxml2 accepts every one of these
restrictions.

[Derivation Valid (Extension)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-ct-extends)
clause 1.4.3.2.2.1 has no exemption for xs:anyType: an unmixed extension of
xs:anyType that adds particles is invalid, and a mixed one is the sequence of
xs:anyType's particle and the added particles, which fails Unique Particle
Attribution against the lax `##any` wildcard. All four providers agree
(`project/content-models/type-derivation/urtype-content/anytype-extension-sequence-*`).

A mixed extension with empty explicit content has an ·effective content· of a
synthetic empty sequence (§3.4.2 complex content clause 2.1.4), which is not
empty, so its {content type} is the sequence of the base particle and that
sequence (clause 3.2.3), not the base {content type}. When the base particle is
an `all` group, the sequence violates
[All Group Limited](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-all-limited)
clause 1.2. Xerces-J and libxml2 reject it; .NET and xmlschema accept it
(`project/content-models/type-derivation/extension-content/mixed-empty-extension-all-base`).
An `all` group with no members is empty explicit content (clause 2.1.2), so
in a mixed type its particle is that synthetic empty sequence, not an `all`
group: a mixed extension with an empty `all` extends a mixed sequence type,
and a mixed type whose content is an empty `all` can be extended by a
sequence. Xerces-J, .NET and libxml2 accept both; xmlschema rejects both as
extensions involving `xs:all`
(`project/content-models/type-derivation/extension-content/mixed-empty-all-extension`
and `project/content-models/type-derivation/extension-content/mixed-empty-all-base`).

NameAndTypeOK clause 3.2.3 requires that "R's declaration's {identity-constraint
definitions} is a subset of B's declaration's {identity-constraint
definitions}, if any". The text decides it as written: a local declaration in a
restriction may not add an identity constraint, and "if any" does not waive the
subset when the base declaration has none, since the empty set has no nonempty
subset. [SOURCES.md](SOURCES.md#xsd-11-change-list-dispositions) keeps the 1.0
structural algorithm for G.1.5, and the 1.1
[G.1.5](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#changes) note
"The rules for complex type restriction now allow identity constraints on
local elements" records the 1.0 rule as a prohibition. The text decides, so
the processor tie-break is not applicable. Xerces-J 2.12.2, .NET 10.0.14 and
libxml2 2.9.13 never enforce clause 3.2.3: they accept a derived declaration
that adds a key or unique to a base with none, adds two, or replaces the
base's unique with a different unique or with a key, and enforce an added
unique at run time. xmlschema 4.3.2 rejects each such addition. Cases:
`project/structures-local-idc-addition` (invalid) and
`project/content-models/particle-restriction/local-idc-*` (an added key or
unique, or a base unique replaced by another unique or a key, is invalid; a
dropped constraint and a declaration shared through a global element or named
group are valid).

Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- libxml2 checks no particle restriction. It accepts a reversed sequence, a
  widened element range, forbidden particle pairs, unmapped or reordered choice
  members, wildcard namespace, range and strength restrictions, group-to-wildcard
  aggregates and lost fixed values. Minimal reproduction: base `sequence(a, b)`
  restricted to `sequence(b, a)`, and base `a maxOccurs="2"` restricted to
  `a maxOccurs="3"`; the other three reject both. It also accepts duplicate or
  overlapping choice particles and wildcards, same-named declarations with
  distinct types, a named group without a model group and an element for a
  zero-count particle. Its acceptance is no evidence for any rule in this
  record.
- .NET does not treat a substitution-group member as competing with its head,
  accepts `##local` under `##other`, and admits an empty restriction only of an
  empty or mixed base, omitting clause 5.3.2.2's emptiable element-only base.
- xmlschema rejects a valid MapAndSum restriction, omits the MapAndSum clause 2
  range check, ignores extension steps in a restricted child's type
  (NameAndTypeOK clause 3.2.5) and the per-member namespace check, keeps
  pointless singleton groups in the comparison, admits an empty restriction of a
  non-emptiable base and a mixed complex-content extension of a simple-content
  base.
- .NET and xmlschema miss ambiguity reached through group references.
- Xerces-J gives the project result in every case listed except
  `project/structures-local-idc-addition`, the invalid
  `project/content-models/particle-restriction/local-idc-*` cases (above) and
  the required empty choices [I-042](#i-042--empty-choice) owns.

Cases: `project/content-models/contrasts-particle-*`,
`project/content-models/dispatch-all-sequence-forbidden`, `project/content-models/dispatch-choice-normalize-optional`, `project/content-models/dispatch-choice-normalize-repeated`, `project/content-models/dispatch-lax-missing-map`, `project/content-models/dispatch-lax-reverse`, `project/content-models/dispatch-mapandsum-count-three-minimum`, `project/content-models/dispatch-range-finite-unbounded`, `project/content-models/dispatch-sequence-required-gap`, `project/content-models/dispatch-sequence-reverse`, `project/content-models/dispatch-source-zero-elimination`, `project/content-models/dispatch-wrapper-all-required`, `project/content-models/dispatch-wrapper-choice-member-range`, `project/content-models/dispatch-wrapper-sequence-required`, `project/content-models/occurrence-boundaries/*`,
`project/content-models/oracle-minimum-*`,
`project/content-models/oracle-ordering-base-mapandsum-unmapped`,
`project/content-models/oracle-ordering-both-singleton-range-invalid`,
`project/content-models/oracle-ordering-derived-singleton-concrete-invalid`,
`project/content-models/particle-restriction/lax-recursive-choice-unmapped`,
`project/content-models/particle-restriction/lax-unbounded-branch-reverse`,
`project/content-models/particle-restriction/mapandsum-maximum-product-only`,
`project/content-models/particle-restriction/inherited-urtype-wildcard`,
`project/content-models/pointless-groups/empty-restriction-epsilon-base`,
`project/content-models/pointless-groups/empty-restriction-required-base`,
`project/content-models/pointless-groups/nonempty-restriction-epsilon-base`,
`project/content-models/pointless-groups/singleton-all-choice-required`,
`project/content-models/pointless-groups/singleton-all-excess-maximum`,
`project/content-models/pointless-groups/singleton-all-outer-optional`,
`project/content-models/pointless-groups/optional-singleton-all-occurrence`,
`project/content-models/pointless-groups/singleton-all-occurrence/*`,
`project/content-models/properties-all-wildcard-*`,
`project/content-models/properties-child-chain-er`,
`project/content-models/properties-child-simple-bridge-opponent`,
`project/content-models/properties-choice-wildcard-*`,
`project/content-models/properties-name-scope-*`,
`project/content-models/properties-wildcard-*`,
`project/content-models/substitution-normalize-blocked-concrete-head-member-invalid`,
`project/content-models/substitution-normalize-both-ranges-b-literal-member`,
`project/content-models/type-derivation/extension-content/mixed-empty-extension-all-base`,
`project/content-models/type-derivation/extension-content/mixed-empty-all-extension`,
`project/content-models/type-derivation/extension-content/mixed-empty-all-base`,
`project/content-models/type-derivation/simple-base-mixed-extension-*`,
`project/content-models/type-derivation/urtype-content/*`,
`project/content-models/wildcard-restriction-*`,
`project/architecture-repeated-choice-duplicate-optional-upa`,
`project/architecture-repeated-equivalent-wildcards-upa`,
`project/architecture-substitution-head-member-upa`,
`project/builtin-hierarchy-complex-restriction-invalid`,
`project/namespace-repeating-sequence-wildcard-overlap-is-upa-compile-error`,
`project/namespace-restriction-wildcard-namespace-rejects-invalid-range`,
`project/namespace-substitution-group-upa-is-checked-after-substitutions`,
`project/structures-all-outer-widen`, `project/structures-edc-type-expanded-name`,
`project/structures-group-empty-separator`, `project/structures-group-missing-model`,
`project/structures-group-two-ref-choice`, `project/structures-local-fixed-default`,
`project/structures-local-fixed-lost`, `project/structures-mapandsum-*`,
`project/structures-retained-nested-sequence-order` and
`project/structures-sequence-all-injective`.

## I-029 — extension after restriction

**Status:** published XSD 1.0 rule; the text decides, so the
[processor tie-break](#processor-tie-break) is not reached.

[Derivation Valid (Extension)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-ct-extends)
clause 1.5 requires that "it must in principle be possible to derive the
complex type definition in two steps, the first an extension and the second a
restriction (possibly vacuous), from that type definition among its ancestors
whose {base type definition} is the ·ur-type definition·". An extension of that
ancestor keeps each of its attribute uses (clause 1.2), and a restriction of the
extension must keep a use's type derived from the inherited type and its fixed
value
([Derivation Valid (Restriction, Complex)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#derivation-ok-restriction)
clauses 2.1.2 and 2.1.3). An attribute that an intermediate restriction
prohibited may therefore return through a later extension only with the
ancestor's type or a type derived from it and with the ancestor's fixed value.
A later extension that changes the fixed value, drops it or gives the attribute
an unrelated type fails clause 1.5. The intermediate restriction's own narrowing
is not part of the obligation.

The clause's 1.0 Note reads "This requirement ensures that nothing removed by a
restriction is subsequently added back by an extension". XSD 1.1 keeps the
clause
([cos-ct-extends](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#cos-ct-extends)
clause 1.5) and qualifies the Note as this record reads it: "nothing removed by
a restriction is subsequently added back by an extension in an incompatible way
(for example, with a conflicting type assignment or value constraint)". The 1.1
Note is corroboration, not an adopted correction. Accepted W3C `msMeta/Attribute_w3c.xml` attZ007i and attZ007v
witness the admitted side: an attribute that a restriction prohibits returns
through a later extension with the ancestor's type and `use="required"`, and
the schema is valid.

Observed: Xerces-J 2.12.2, .NET 10.0.14 and libxml2 2.9.13 do not apply clause
1.5 and accept every reintroduction. xmlschema 4.3.2 checks the extension's new
use against the intermediate restriction and rejects valid reintroductions.
When the ancestor's use is fixed, xmlschema rejects the restriction that
prohibits it, reporting a different fixed value.

Cases: `project/content-models/type-derivation/prohibited-ancestor-fixed-changed`,
`project/content-models/type-derivation/prohibited-ancestor-fixed-equal`,
`project/content-models/type-derivation/prohibited-ancestor-fixed-lost`,
`project/content-models/type-derivation/prohibited-ancestor-restored-type`,
`project/content-models/type-derivation/prohibited-ancestor-unrelated-type`,
`project/content-models/type-derivation/prohibited-intermediate-narrowing-discarded`
and `project/content-models/type-derivation/prohibited-ancestor-readd-*` (an
attribute added back with a fixed value the ancestor lacks, another default, a
derived type, `use="required"` or an equal fixed value spelled differently is
valid; with a type not derived from the ancestor's type it is invalid).

## I-030 — value constraints compare values

**Status:** published XSD 1.0 rules, with one doubtful clause settled by the
[processor tie-break](#processor-tie-break).

[Attribute Use Correct](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#au-props-correct)
clause 2 requires a use's fixed value to "match that of the {attribute
declaration}'s {value constraint}", NameAndTypeOK clause 3.2.2 requires a
restricting element's fixed constraint "with the same value", and
[Attribute Locally Valid](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-attribute)
clause 4 requires an attribute to match its declaration's fixed value. These
compare datatype values under Part 2
[equality](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#equal):
decimal `1.0` equals `1`, QNames compare as expanded names whatever their
prefixes, `P1Y` equals `P12M` and differs from `P365D`
([Duration equality and order](#duration-equality-and-order)), and `INF` differs from `-INF`. Use-owned fixed values at
instance time follow [I-009](#i-009--fixed-attribute-use-values); element fixed
values follow [E1-56](corrected-reading.md#e1-56-applied-reading).

A value constraint on `<attribute ref>` belongs to the use (§3.2.2), and
[Attribute Default Value](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#sic-attrDefault)
takes the use's constraint over the declaration's, including an empty default or
fixed value. An element value constraint keeps the declared type's value when
`xsi:type` selects another type (cvc-elt clause 5.2.2.2.2; compare
[I-013](#i-013--qname-element-constraints-under-xsitype)).

[Derivation Valid (Restriction, Complex)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#derivation-ok-restriction)
clause 2.1.3.2 asks for R's effective value constraint to be "fixed with the
same string as B's". A {value constraint} holds a value, so the string meant
(source literal or canonical form) is undefined and the reading is doubtful. The
tie-break settles it by Xerces-J, which compares values there as everywhere
else. .NET agrees. The same comparison applies inside the reconstruction of
[I-029](#i-029--extension-after-restriction).

Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- Xerces-J compares values and gives the project result in every case listed.
- .NET equates `P1Y` and `P365D`, and supplies the declaration's default
  instead of an empty use default.
- xmlschema compares spellings, both between schema constraints and for a QName
  instance value against a declaration's fixed QName.
- .NET, xmlschema and libxml2 compare a string fixed value after the `xsi:type`
  token normalization.
- libxml2 ignores `fixed` on an attribute reference at instance time (a
  reference with `fixed="b"` accepts `a="other"`, the equivalent local
  declaration rejects it), ignores an empty fixed use and a mismatched fixed
  use, compares no fixed values between declarations, uses and restrictions,
  and compares element fixed values by spelling.

Cases: `project/attributes/conformance-global-fixed-qname`,
`project/attributes/continuation-independent-fixed-*`,
`project/attributes/continuation-independent-default-fixed-*`,
`project/attributes/continuation-matrix24-*-bf1-*`,
`project/attributes/continuation-use-fixed-*`,
`project/attributes/fixed-value-space-attribute-reference/*`,
`project/attributes/fixed-value-space-attribute-restriction/*`,
`project/content-models/fixed-value-space-element-restriction/*`,
`project/content-models/properties-fixed-value-controls`,
`project/content-models/properties-fixed-value-opponent`,
`project/empty-attribute-reference-default`, `project/empty-attribute-reference-fixed`,
`project/fixed-constraint-actual-whitespace` and
`project/prohibited-no-component-reference-fixed-mismatch-optional`.

## I-031 — identity-constraint tables and keyref resolution

**Status:** published XSD 1.0 rules; the text decides, so the
[processor tie-break](#processor-tie-break) is not reached.

The [Identity-constraint Table](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#sic-key)
of an element holds a node table for each eligible constraint: its own
qualified rows (clause 2) and every entry of its children's tables (clause 1),
"provided no two entries have the same ·key-sequence· but distinct nodes".
"Potential conflicts are resolved by not including any conflicting entries
which would have owed their inclusion to clause 1", so a local row keeps its
key-sequence, conflicting child entries disappear, and unrelated entries of the
same children survive.
[Identity-constraint Satisfied](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-identity-constraint)
clause 4.3 treats that table as "logically prior" and asks for a node table only
"for each member of the ·qualified node set·": an element with no keyref rows
needs no binding, and an incomplete keyref row is not checked. Clause 4.2.3
forbids key fields assessed through a nillable declaration. A chameleon include
transforms a keyref `refer` like every other QName
([src-include](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-include)
clause 2.3).

Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- Xerces-J keeps a key-sequence that two child entries contribute, so a keyref
  resolves to it, and loses the unrelated entries of a conflicting child.
- .NET never consults child tables: keyrefs fail against keys bound on
  descendants, and a keyref on an element without its own referenced constraint
  reports no key in scope even with no rows. It ignores a keyref transformed by
  a chameleon include. The same schema in its own namespace makes .NET reject.
- libxml2 counts a local and an inherited entry as two matches, so conflicting
  child entries defeat a local entry. When two conflicting descendant entries
  precede another child's equal entry, it keeps them and reports more than one
  match for a keyref; in the reverse order it accepts.
- xmlschema misses table conflicts, checks incomplete keyref rows and fails with
  `KeyError` on an element without a binding. It accepts a keyref value that
  matches no entry on an element with no complete row of its own whose table
  takes entries from a descendant.
- .NET, xmlschema and libxml2 do not apply clause 4.2.3.

Cases: `project/identity/joins-key-tables`, `project/identity/joins-unique-tables`,
`project/chameleon-defaults-and-identity`, `project/structures-field-cardinality-keyref`,
`project/structures-key-nillable-field-true`, `project/structures-recursive-key-tables-*`
and, for xmlschema's checks of incomplete keyref rows, `project/identity/joins-keyref-three-fields`.

## I-032 — identity XPath node-sets

**Status:** published XSD 1.0 rules; the text decides, so the
[processor tie-break](#processor-tie-break) is not reached.

[Selector Value OK](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-selector-xpath)
and Fields Value OK require each expression to be "a valid XPath expression, as
defined in [XPath]", and
[Identity-constraint Satisfied](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-identity-constraint)
clause 1 evaluates the selector to "a node-set (as defined in [XPath])". In XPath
1.0 `.//.` abbreviates `self::node()/descendant-or-self::node()/self::node()`,
which selects the context element and every descendant, and a union is a
node-set in which each node occurs once. Clause 3 asks each field for "a
node-set with exactly one member", so one node reached through several union
branches is one member.

Observed: Xerces-J 2.12.2 evaluates selector `.//.` as the context element
alone, missing every descendant row, and counts a node reached through several
field branches as several matches. libxml2 2.9.13 omits the context element
from `.//.`. .NET 10.0.14 and xmlschema 4.3.2 follow the XPath meaning.

Cases: `project/identity/field-union-nodes/overlap`,
`project/identity/selector-self-descendant-scopes` and
`project/identity/selector-self-descendant-union-single-selection`.

## I-033 — whitespace normalization

**Status:** published XSD 1.0 rules; the text decides, so the
[processor tie-break](#processor-tie-break) is not reached.

The [normalized value](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#key-nv)
replaces and collapses only `#x20`, `#x9`, `#xA` and `#xD`, and element-only
content admits only XML white space
([Element Locally Valid (Complex Type)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-complex-type)
clause 2.3). NEL, LINE SEPARATOR and NO-BREAK SPACE therefore survive
normalization, stay inside one list item, are not stripped from an integer and
are character content in element-only content. A list's whiteSpace is collapse,
and its pattern applies to the collapsed literal.

For a union, [whiteSpace](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#rf-whiteSpace)
"does not apply directly; however, the normalization behavior of ·union· types
is controlled by the value of whiteSpace on that one of the ·memberTypes·
against which the ·union· is successfully validated". Members are tried in order
([§2.5.1.3](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#union-datatypes)),
so the union's pattern sees the literal as the first validating member
normalizes it: a `token` member collapses it and a `string` member preserves it.

Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- Xerces-J collapses every union value before the union's pattern. Minimal
  reproduction: a union whose only member is `xs:string`, restricted by pattern
  `a  b` (two spaces), rejects `a  b` only in Xerces-J, and the same union with
  pattern `a b` accepts `a  b` only in Xerces-J.
- .NET rejects a list literal whose collapsed form matches the list pattern,
  applies a union's pattern before normalization and does not normalize a union
  value through its validating member.
- xmlschema splits and collapses at Unicode white space and treats U+00A0 as
  white space in integers and in element-only content.
- xmlschema and libxml2 preserve a `string` member's value, as the project does.

Cases: `project/datatypes/lists/conformance-whole-list-pattern-multiple-items`,
`project/datatypes/lists/xml10-whitespace-boundaries/*`,
`project/datatypes/unions/outer-pattern-member-order/*`,
`project/datatypes/whitespace/conformance-collapse-value-consumer`,
`project/union-pattern-after-collapse-rejects`, `project/union-pattern-after-collapse-rejects-string-member`, `project/union-pattern-applies-after-normalization-newline`, `project/union-pattern-whitespace-normalization`, `project/union-whitespace-collapse-runtime`,
`project/integer-non-xml-whitespace`,
`project/structures-empty-and-element-only-character-items` and
`project/content-models/contrasts-mixed-priority-outer-absent-inner-absent`.

## I-034 — regular expressions follow the XSD grammar over characters

**Status:** published XSD 1.0 grammar; the text decides, so the
[processor tie-break](#processor-tie-break) is not reached.
[I-011](#i-011--regex-metacharacter-grammar) owns the dash and brace questions.

A [regular expression](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#regexs)
matches the whole value, character by character, a supplementary character
being one character. Its own productions apply: `.` is `[^\n\r]` ([37a]); `\s`
is `[#x20\t\n\r]` and `\w` is `[#x0000-#x10FFFF]-[\p{P}\p{Z}\p{C}]` ([37]); `]`
is not a normal character ([10]) and `[` and `]` are not class characters
([22]); character class subtraction is part of the grammar ([16]); and a
quantified nullable atom matches the empty string ([4]). Only the listed
escapes exist: [24] `SingleCharEsc ::= '\' [nrt\|.?*+(){}#x2D#x5B#x5D#x5E]`
admits no `\q`, and no other escape production does. The category names of
[35] are `C` and `Cc`, `Cf`, `Co`, `Cn`, and the grammar notes that "the
properties mentioned above exclude the Cs property", so `\p{Cs}` is not a
category escape. Block names come from the printed table ([36]).

Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- Xerces-J admits `\q`; Xerces-J, .NET and xmlschema admit `\p{Cs}`. Both
  contradict the productions quoted above.
- .NET translates a pattern to `^(...)$`, whose `$` also matches before a final
  newline (pattern `a` admits `a` followed by LF only in .NET). It matches UTF-16
  code units, so `.` and `[^a]` reject a supplementary character and a
  supplementary range is rejected as reversed. Its `.` matches CR, its `\s` is
  Unicode, and it admits `[a]]` and `[a[b]]`.
- xmlschema uses Python `\s` and `\w`.
- libxml2 reads a single character followed by `-[` as a range start, so it
  rejects subtraction from a group that ends with a single character
  (`[a-z-[aeiou]]` works). It rejects empty repetitions of a nullable atom and
  admits unknown block names.

Cases: `project/datatypes/regex/conformance-escape-01-atom`, `project/datatypes/regex/conformance-escape-01-class`, `project/datatypes/regex/conformance-grammar-dot`, `project/datatypes/regex/conformance-grammar-negated-class`, `project/datatypes/regex/conformance-grammar-range-supplementary`, `project/datatypes/regex/conformance-invalid-extra-class-close`, `project/datatypes/regex/conformance-invalid-raw-nested-class`, `project/datatypes/regex/conformance-invalid-surrogate-category`, `project/datatypes/regex/conformance-invalid-unknown-block`, `project/datatypes/regex/conformance-invalid-unknown-escape`, `project/datatypes/regex/conformance-repeat-closed-nullable`, `project/datatypes/regex/conformance-repeat-exact-nullable`, `project/datatypes/regex/conformance-repeat-open-nullable`, `project/datatypes/regex/conformance-shorthand-nonspace`, `project/datatypes/regex/conformance-shorthand-nonword`, `project/datatypes/regex/conformance-shorthand-space`, `project/datatypes/regex/conformance-shorthand-word`, `project/datatypes/regex/conformance-subtraction-empty-result`, `project/datatypes/regex/conformance-subtraction-empty-result-star`, `project/datatypes/regex/conformance-subtraction-negative-subtrahend` and `project/datatypes/regex/conformance-subtraction-nested`.

## I-035 — length facets measure by primitive

**Status:** published XSD 1.0 rule; the text decides, so the
[processor tie-break](#processor-tie-break) is not reached.

[Length Valid](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#cvc-length-valid)
and its minLength and maxLength siblings measure a string or anyURI value "in
characters" (clause 1.1), which §4.3.1.1 defines as "characters as defined in
[XML 1.0 (Second Edition)]": one `Char` per code point, so a supplementary
character counts once. Clause 1.3 makes any value facet-valid for QName and
NOTATION.

Observed: Xerces-J 2.12.2 and .NET 10.0.14 count UTF-16 code units, so a
supplementary character counts twice (Xerces-J reports length 3 for `😀a`).
.NET also measures QName and NOTATION values by their lexical length.
xmlschema 4.3.2 and libxml2 2.9.13 count characters.

Cases: `project/datatypes/length/conformance-string-supplementary-*`,
`project/datatypes/length/conformance-qname-vacuous-*` and
`project/datatypes/length/conformance-notation-vacuous-*`.

## I-036 — builtin lexical and value spaces

**Status:** published XSD 1.0 Datatypes text with no interpretive question.

Each builtin's lexical grammar decides admission.
[Timezones](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#dateTime-timezones)
have an hour magnitude of at most 14, minutes 0 at 14, minutes at most 59, and
only uppercase `Z`.
[dateTime](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#dateTime-lexical-representation)
and the g* types admit years of more than four digits and hour 24 with zero
minutes and seconds, and so does
[time](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#time),
whose value the [Time midnight](#time-midnight) boundary owns.
[base64Binary](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#base64Binary)
restricts the symbol before padding to the B04 and B16 sets.
[hexBinary](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#hexBinary),
[decimal](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#decimal)
and [integer](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#integer)
admit no interior space and only ASCII digits, and
[nonNegativeInteger](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#nonNegativeInteger)
admits `-0`, also in `minOccurs` and `maxOccurs`.
[float](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#float)
and [double](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#double)
need exponent digits, spell the specials exactly `INF`, `-INF` and `NaN`, and
have one zero, so `0` equals `-0` in fixed values and identity constraints.
Decimal values compare by value, so `1` matches fixed `01.0`.
[anyURI](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#anyURI)
admits a space, since its lexical space is defined after the escaping
procedure. [NMTOKENS](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#NMTOKENS)
has `minLength` 1. The value space of
[ENTITY](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#ENTITY)
holds only names declared as unparsed entities, so in a document that declares
none every ENTITY value is invalid, while a list type without a length facet
still admits an empty list of them. The
[ENTITY member selection](#entity-member-selection) boundary owns ENTITY
assessment beyond this.

Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- .NET admits out-of-range and lowercase timezones, `Infinity`, `nan`, spaced
  hexBinary and noncanonical base64 final symbols, and rejects years above 9999,
  hour 24 and an anyURI with a space. Minimal reproduction: `gYear` `2001+14:30`
  and `2001z` are valid and `dateTime` `2001-02-03T24:00:00Z` and
  `10000-01-01T00:00:00Z` invalid only in .NET. It also rejects the `time`
  values `24:00:00Z`, `24:00:00.000Z` and `24:00:00.000`.
- xmlschema accepts a nonempty `ENTITY` value, also through a restriction or a
  list, in a document that declares no unparsed entity.
- libxml2 admits `1E+`, `1E` and empty NMTOKENS, rejects `-0` in occurrence
  attributes, keeps `0` and `-0` distinct, and rejects a decimal that differs
  from its fixed value only in spelling.
- xmlschema admits `1 2` as a decimal and Arabic-Indic digits as an integer.
- Xerces-J, .NET and libxml2 reject a 30-digit year in `date`, `gYear` and
  `gYearMonth`, and xmlschema fails with `OverflowError` (year overflow). The
  [dateTime lexical representation](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#dateTime-lexical-representation)
  admits a "four-or-more digit optionally negative-signed numeral", and
  Appendix D.3.3 allows "more than four digits". The
  [§3.2.6](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#duration)
  note, to which §3.2.7 refers, lets minimally conforming processors "set an
  application-defined limit on the maximum number of digits", which "must be
  clearly documented"; the library sets no year-digit limit beyond its token
  limits in [resource limits](../RESOURCE_LIMITS.md) and admits the years, so
  the text decides and the processor tie-break is not reached.
- Xerces-J gives the project result in every case listed except the wide years
  of `project/temporal-year-boundaries`.

Cases: `project/datatypes/lexical/conformance-*`,
`project/datatypes/temporal/conformance-time-lexical`, `project/time-midnight-fraction`,
`project/corrections-derived-entity-components`,
`project/datatypes/temporal/conformance-*-zone`,
`project/datatypes/temporal/conformance-datetime-lexical`,
`project/datatypes/temporal/conformance-gyear-lexical`,
`project/any-uri-allows-spaces`, `project/runtime-any-uri-allows-space`,
`project/architecture-fixed-decimal-canonical-value`,
`project/runtime-decimal-negative-canonical-fixed`,
`project/float-fixed-zero-value-space`, `project/identity-float-double-zero-value-space`,
`project/negative-zero-max-occurrence`, `project/negative-zero-occurrence` and
the `wide-date`, `wide-year` and `wide-month` phases of
`project/temporal-year-boundaries`.

## I-037 — enumeration compares values

**Status:** published XSD 1.0 rules; the text decides, so the
[processor tie-break](#processor-tie-break) is not reached.

[Enumeration valid](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#cvc-enumeration-valid)
asks whether the value is one of the enumerated values, and equality is
identity (§2.2.1 and the note under
[float](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#float)).
A list value is the whole ordered sequence of item values
([list datatypes](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#list-datatypes)),
so `01.0 +2` equals `1 2`, `a b a` equals itself and `a` does not equal `a b`.
NaN equals itself, and a float literal denotes the nearest float value
(`0.100000001` and `0.1` are the same float). NOTATION and QName values are
expanded names: schema literals resolve in schema scope and instance values in
instance scope, including the default namespace for an unprefixed NOTATION
value.

A timezoned temporal value is its normalized instant, so
`2000-01-01T13:00:00+01:00` equals `2000-01-01T12:00:00Z`. A value without a
timezone lies on the other timeline of
[§3.2.7.4](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#dateTime-order)
and never equals a timezoned one (`2000-01-01` is not `2000-01-01Z`). A dateTime
hour 24 is the first instant of the next day
([lexical representation](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#dateTime-lexical-representation)),
so `2000-12-31T24:00:00Z` equals `2001-01-01T00:00:00Z`. For `time` the
[Time midnight](#time-midnight) boundary governs.

Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- .NET does not compare list enumerations as whole list values. Minimal
  reproduction: enumeration `{a b, c d}` over a string list admits `a` and `c`
  and rejects `a b` and ` a b `. It does not equate NaN with NaN, rejects
  hour 24 and equates `P1Y` with `P365D`.
- .NET and xmlschema equate an untimezoned value with the timezoned value of
  the same clock (enumeration `2000-01-01Z` admits `2000-01-01` in both).
- xmlschema does not round float literals to float precision before comparing
  and compares NOTATION enumerations without resolving prefixes.
- libxml2 ignores the default namespace for an unprefixed NOTATION value, keeps
  a dateTime hour 24 distinct from the next day's midnight and does not
  normalize a zoned `time` for equality.
- Xerces-J gives the project result in every case listed.

Cases: `project/datatypes/enumeration/large-group-decimal-list-value-space`,
`project/datatypes/enumeration/large-group-duration-value-space`,
`project/datatypes/enumeration/large-group-float-zero-nan`,
`project/datatypes/enumeration/large-group-list-literals`,
`project/datatypes/enumeration/large-group-temporal-union-members`,
`project/datatypes/enumeration/large-group-temporal-zones`,
`project/datatypes/lists/conformance-multiplicity-and-order-enumeration`,
`project/datatypes/notation/envelopes-namespace-enumeration-consumers` and
`project/datatypes/temporal/midnight-year-rollover`.

## I-038 — restriction steps read the base's effective facets

**Status:** published XSD 1.0 rules, with one garbled clause repaired by the
selected [minExclusive correction](SOURCES.md#selected-corrections) and one
doubtful clause settled by the [processor tie-break](#processor-tie-break).

Each `*-valid-restriction` constraint compares a facet of a derivation step with
the facets in {facets} of its {base type definition}, which include every facet
inherited from earlier steps. The comparisons are a derived bound against the
inherited opposite bound
([maxInclusive](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#maxInclusive-valid-restriction)
clause 4,
[maxExclusive](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#maxExclusive-valid-restriction)
clauses 3 and 4,
[minInclusive](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#minInclusive-valid-restriction)
clause 4,
[minExclusive](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#minExclusive-valid-restriction)
clause 2 as repaired below); a derived `length` against the inherited `length`
([length valid restriction](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#length-valid-restriction));
`length` with a `minLength` or `maxLength` that an earlier length-free step
supplies, and the inequalities between them
([length and minLength or maxLength](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#length-minLength-maxLength));
and `whiteSpace` against a base `collapse`
([whiteSpace valid restriction](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#whiteSpace-valid-restriction)
clause 1). maxExclusive clause 3 makes it an error when "minInclusive is among
the members of {facets} of {base type definition} and {value} is less than or
equal to the {value} of the parent minInclusive", so a derived `maxExclusive`
10 under an inherited `minInclusive` 10 is rejected although 10 is in the base
value space.

Its mirror, minExclusive clause 2, reads "{value} is greater the {value} of the
parent maxInclusive", with the relation's operator garbled. Every other 1.0
clause comparing a derived bound with an opposite parent bound where either is
exclusive makes equality an error, and the later
[minExclusive valid restriction](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/#minExclusive-valid-restriction)
clause 3 reads "greater than or equal to the {value} of that maxInclusive". The
library adopts that clause as a
[selected correction](SOURCES.md#selected-corrections), so a derived
`minExclusive` 10 under an inherited `maxInclusive` 10 is rejected, directly or
through a facet-only middle step, and the processor tie-break is not reached.
Equal inclusive opposite bounds remain allowed.

The facet {fixed} rule
([maxLength](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#maxLength-fixed)
and its siblings) says that if {fixed} is true, "types for which the current
type is the {base type definition} cannot specify a value … other than
{value}". Read literally it binds only the immediate restriction, so a middle
step restating the value with `fixed="false"` would release every later step,
while the `length` example states the purpose transitively ("types derived from
productCode … cannot change the length"). The reading is doubtful, and the
tie-break settles it by Xerces-J, which rejects (`FixedFacetValue`): once a facet
is fixed, no later restriction in the chain may specify another value, whatever
`fixed` a restating step spells. No W3C test restates a fixed facet with
`fixed="false"`.

Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- .NET skips the inherited-bound and inherited-length comparisons.
- Xerces-J skips maxExclusive clause 3 and the repaired minExclusive clause 2
  when the literal is in the base value space
  (`project/datatypes/bounds/continuation-b23-equal`,
  `project/datatypes/bounds/continuation-b26-equal`), the b26 shape also
  through a facet-only middle step, while it rejects the same pairs within one
  step. The clause and the correction decide against it.
- libxml2 rejects the b23 shape and accepts the b26 shape.
- xmlschema skips the `length`, `minLength` and `maxLength` coexistence rule.
- libxml2 admits `whiteSpace` replace under a builtin non-string type.
- .NET, xmlschema and libxml2 apply the immediate-base reading of {fixed}.

Cases: `project/datatypes/bounds/continuation-b20-equal`,
`project/datatypes/bounds/continuation-b23-equal`,
`project/datatypes/bounds/continuation-b24-equal`,
`project/datatypes/bounds/continuation-b26-equal`,
`project/datatypes/bounds/continuation-b31-equal`,
`project/datatypes/bounds/inherited-opposing-exclusive-equality`,
`project/datatypes/bounds/opposite-bound-equal/*` (each derived bound equal to an
inherited opposite bound: invalid when either bound is exclusive, valid for two
inclusive bounds, also through a facet-only middle step),
`project/datatypes/facets/conformance-inherit-length-less`,
`project/datatypes/length/continuation-both-maximum-numeric-false`, `project/datatypes/length/continuation-both-minimum-numeric-false`, `project/datatypes/length/continuation-fixed-length-equal-false-middle-change`, `project/datatypes/length/continuation-fixed-maxlength-equal-false-middle-change`, `project/datatypes/length/continuation-fixed-minlength-equal-false-middle-change`, `project/datatypes/length/continuation-inherit-length-empty-middle-less`, `project/datatypes/length/continuation-maximum-changed-in-length-step`, `project/datatypes/length/continuation-maximum-numeric-false`, `project/datatypes/length/continuation-minimum-changed-in-length-step`, `project/datatypes/length/continuation-minimum-numeric-false` and
`project/datatypes/whitespace/conformance-nonstring-*-replace`.

## I-039 — schema-document syntax

**Status:** published XSD 1.0 schema documents with no interpretive question;
the text decides, so the [processor tie-break](#processor-tie-break) is not
reached.

The normative [schema for datatype definitions](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#schema)
and [schema for schemas](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#normative-schemaSchema)
decide these shapes:

- `enumeration` and `pattern` are `noFixedFacet`s, which prohibit `fixed`
  whatever its value.
- `localSimpleType` declares `<xs:attribute name="final" use="prohibited"/>`,
  and a local complex type has no `block` or `final` attribute, so even an empty
  value is an error.
- `simpleType/@final` is a `simpleDerivationSet` (`#all` or a list of `list`,
  `union` and `restriction`).
- The restriction content model puts an inline `simpleType` before the facets.
- Notation `public` is `xs:public` (a token) and `system` is `xs:anyURI`, so a
  whitespace-only value that collapses to the empty string is admitted.
- `schema/@id` is an `xs:ID`, checked after whitespace collapse.
- `minOccurs` and `maxOccurs` are `xs:nonNegativeInteger`, so `+01` is admitted.

[Single Facet Value](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#src-single-facet-value)
forbids repeating a facet other than `pattern` and `enumeration` in one step,
even with an equal value.

Observed with Xerces-J 2.12.2, .NET 10.0.14, xmlschema 4.3.2 and libxml2 2.9.13:

- Xerces-J and .NET admit `final=""` on a local simpleType, which the
  `localSimpleType` declaration decides against.
- .NET and libxml2 admit `fixed` on enumeration and pattern.
- .NET admits `final="extension"`, an inline simpleType after a facet and an
  empty `block` or `final` on a local complexType, validates `schema/@id` before
  collapsing it, and rejects the whitespace-only notation identifiers as invalid
  URIs.
- libxml2 admits duplicate facets and rejects `minOccurs="+01"`.

Cases: `project/datatypes/facets/conformance-*-prohibited-fixed-*`,
`project/datatypes/facets/conformance-duplicate-*`,
`project/datatypes/construction/conformance-inline-after-facet`,
`project/datatypes/construction/conformance-inline-final-empty`,
`project/datatypes/construction/continuation-local-simpletype-final-empty-forbidden`,
`project/datatypes/construction/continuation-simple-final-invalid-local-extension`,
`project/datatypes/notation/envelopes-identifier-public-space-only`,
`project/datatypes/notation/envelopes-identifier-system-space-only`,
`project/content-models/continuation-local-complextype-block-empty-forbidden`,
`project/content-models/continuation-local-complextype-final-empty-forbidden`,
`project/content-models/dispatch-source-default-and-alias` and
`project/composition/conformance-schema-header-id-collapsed`.

## I-040 — direct NOTATION use

**Status:** published XSD 1.0 text over accepted W3C expectations; the text
decides, so the [processor tie-break](#processor-tie-break) is not reached.

The [NOTATION constraint](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#enumeration-required-notation)
says "It is an ·error· for NOTATION to be used directly in a schema. Only
datatypes that are ·derived· from NOTATION by specifying a value for
·enumeration· can be used in a schema." A reference to `xs:NOTATION` as an
element or attribute type, as a list `itemType` or as a union member uses
NOTATION in a schema without that derivation. A restriction of NOTATION
without an enumeration facet (facet-free or pattern-only) is not derived by
specifying an enumeration, so its use is also an error.

The accepted W3C tests `msMeta/SimpleType_w3c.xml` stE056 through stE065 expect a
schema with `xs:NOTATION` as a union member to be valid. Explicit text outranks
them, and the corpus expects those schemas invalid with the W3C result kept as
`oracle` metadata.
[XSD 1.1 §3.3.19](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/#enumeration-required-notation)
narrows the error to validating a literal directly against NOTATION, which would
admit the list and union constructors; that is a later-version change, not a
published 1.0 erratum, and is not adopted.

Observed: Xerces-J 2.12.2 and .NET 10.0.14 reject a facet-free or pattern-only
restriction and a raw element type but accept NOTATION as a list item or union
member. xmlschema 4.3.2 and libxml2 2.9.13 accept all five shapes.

Cases: `project/datatypes/notation/envelopes-constructor-alias-no-enum`, `project/datatypes/notation/envelopes-constructor-list-raw-item`, `project/datatypes/notation/envelopes-constructor-pattern-no-enum`, `project/datatypes/notation/envelopes-constructor-raw-element` and `project/datatypes/notation/envelopes-constructor-union-raw-member`.

## I-041 — empty target namespace

**Status:** doubtful published XSD 1.0 text decided by an accepted W3C
expectation, which outranks processor behaviour.

[§3.15.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-schema)
states: "Since the empty string is not a legal namespace name, supplying an empty
string for targetNamespace is incoherent, and is not the same as not specifying
it at all." No constraint names the consequence, and the schema for schemas
admits `""` as anyURI, so the text alone is doubtful. Accepted W3C
`msMeta/Schema_w3c.xml` schZ014_b, an otherwise empty schema with
`targetNamespace=""`, expects invalid (accepted 2006-07-16). That expectation decides
the question. `targetNamespace` is an anyURI, whose whitespace is collapse, so
`targetNamespace="   "` is the same case.

Observed: Xerces-J 2.12.2 warns `EmptyTargetNamespace` and accepts, libxml2
2.9.13 accepts, and .NET 10.0.14 and xmlschema 4.3.2 reject.

Its sibling schZ014_a, an `<xsd:import namespace=""/>` in a schema without
targetNamespace, expects invalid under the same acceptance and decides the
empty import namespace in [CMP-Q001](#remaining-source-integration-boundaries):
Xerces-J and libxml2 accept that document, .NET and xmlschema reject it
(`project/namespace-schema-import-namespace-cannot-be-empty`). The same rule
rejects `namespace=""` and `namespace="   "` in a schema with a
targetNamespace, and `namespace=""` with a schemaLocation whose no-namespace
element the importer references; an import without a `namespace` attribute in
a schema with a targetNamespace imports no-namespace components and is valid
(`project/namespaces/import-namespace-empty/*`). Xerces-J, libxml2 and
xmlschema accept the empty and whitespace imports in a schema with a
targetNamespace, and .NET rejects them. xmlschema also rejects the located
import without a `namespace` attribute, resolving the reference `ref="n"`
under `xmlns=""` in the importer's default namespace.

Cases: `project/composition/conformance-schema-header-target-empty`,
`project/composition/conformance-schema-header-target-whitespace` and
`project/namespace-schema-target-namespace-cannot-be-empty`.

## I-042 — empty choice

**Status:** published XSD 1.0 rule, confirmed by an accepted W3C expectation;
the [processor tie-break](#processor-tie-break) is not reached.

[Element Sequence Valid](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-model-group)
clause 2 requires "a particle among the {particles} such that the sequence is
·valid· with respect to that particle", and the paragraph after it states that
"no sequence can be ·valid· with respect to such a group whose {compositor} is
choice". A required empty choice therefore rejects every sequence, including the
empty one. Accepted W3C `saxonMeta/Complex.testSet` complex022 ("Empty choice
should accept no instances", accepted 2015-06-02, no version restriction) expects
both instances invalid. An empty choice with `minOccurs="0"` remains satisfiable
by absence.

A required empty choice is not empty content: §3.4.2 complex content clause
2.1.3 treats a choice with no members as empty only with `minOccurs="0"`. An
extension of a type whose content is a required empty choice therefore has the
sequence of that choice and the added particles (clause 3.2.3) as its {content
type}, and admits no instance.

Observed: Xerces-J 2.12.2, .NET 10.0.14 and xmlschema 4.3.2 accept empty content
and a child for a required empty choice, and accept the added particles in an
extension of it. libxml2 2.9.13 rejects all of them.

Under Particle Valid (Restriction) clause 2.2 a required empty choice is
therefore not an ignorable empty group except inside a choice, where it adds
nothing ([I-028](#i-028--content-model-checks)). A sequence keeps it, so a
restriction compares it like any other member: dropping it from a base
sequence admits content the base does not, and keeping it in a derived
sequence the base lacks leaves it unmapped. Xerces-J ignores it as empty
everywhere and accepts both restrictions.

Cases: `project/content-models/nested-empty-choice-required`,
`project/content-models/type-derivation/extension-content/required-empty-choice-base`,
`project/content-models/pointless-groups/declared/base-seq-emptychoice1-elt` and
`project/content-models/pointless-groups/declared/derived-seq-a-emptychoice1-b`.

## I-043 — libxml2 continues after well-formedness errors

**Status:** provider behaviour under published
[XML 1.0](https://www.w3.org/TR/2008/REC-xml-20081126/) and
[Namespaces in XML 1.0](https://www.w3.org/TR/2009/REC-xml-names-20091208/)
constraints; no reading is in doubt.

Element and attribute names must match `QName`, prefixes must be declared, `xml`
keeps its namespace and `xmlns` is not bound
([§3](https://www.w3.org/TR/2009/REC-xml-names-20091208/#xmlReserved)),
expanded attribute names are unique
([Attributes Unique](https://www.w3.org/TR/2009/REC-xml-names-20091208/#nsc-AttrsUnique)),
a prefixed namespace declaration is well formed, and no processing-instruction
target contains a colon. `<!DOCTYPE` needs the white space that
[production 28](https://www.w3.org/TR/2008/REC-xml-20081126/#NT-doctypedecl)
requires. A document that breaks one of these is not namespace-well-formed and
is rejected as XML input, in an instance or in a schema document, annotation
payloads included.

Observed: libxml2 2.9.13 (`xmllint`) reports each namespace constraint as a recoverable
error or `namespace error`, validates the document and exits 0, so the adapter
(`tools/oracles/libxml2/oracle.sh`) records valid with the diagnostic as a
warning. It accepts a `<!DOCTYPE` without the required white space and reports
nothing (`project/schema-unknown-markup`, `invalid-doctype-prefix` of
`project/streaming-unknown-markup`). Xerces-J 2.12.2, .NET 10.0.14 and
xmlschema 4.3.2 reject.

Cases: `project/namespaces/exact-uri-identity/attributes`,
`project/schema-syntax/envelopes-annotation-duplicate-expanded-attribute`,
`project/schema-syntax/envelopes-annotation-reserved-namespace-binding`,
`project/schema-syntax/envelopes-annotation-unbound-payload-prefix`,
`project/xml/pi-target-colon-in-schema`, `project/xml/pi-target-ncname`,
`project/xml/qname-admission-ascii-name-classes`,
`project/xml/qname-admission-colon-forms`, `project/xml/qname-admission-non-ascii`,
`project/schema-unknown-markup`, `project/streaming-duplicate-attributes`,
`project/streaming-namespace-declarations` and `project/streaming-unknown-markup`.

## I-044 — other Structures rules some providers do not enforce

**Status:** published XSD 1.0 rules with no interpretive question; Xerces-J
2.12.2 gives the project result in every case listed.

| Rule | Provider behaviour | Cases |
| --- | --- | --- |
| [src-resolve](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-resolve) clause 4.2: the referencing document imports the namespace itself | .NET 10.0.14 only warns | `project/namespace-qname-references-require-direct-import-invalid` |
| [Wildcard mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-openness): `namespace=""` is "a set whose members are namespace names corresponding to the space-delimited substrings of the string", here no members, so it matches nothing and overlaps no particle | .NET 10.0.14 treats it as `##any`; xmlschema 4.3.2 matches nothing at instance time but finds it overlapping `##any` for Unique Particle Attribution | `project/attributes/conformance-namespace-empty`, `project/content-models/wildcard-overlap/empty-finite-any-disjoint` |
| [§3.6.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-attributeGroup): an attribute group's {attribute uses} is "the union of the set of attribute uses" of its children and referenced groups, so one use reached through several references is one member and [ag-props-correct](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#ag-props-correct) clause 2 ("two distinct members") does not apply | .NET 10.0.14 and libxml2 2.9.13 report a duplicate attribute use | `project/attributes/conformance-group-shared-use-diamond` |
| [Derivation Valid (Restriction, Complex)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#derivation-ok-restriction) clause 4.3: the strength check applies "unless the {base type definition} is the ·ur-type definition·", so a direct restriction of `xs:anyType` may use a `skip` attribute wildcard | xmlschema 4.3.2 applies the check against `xs:anyType` | `project/attributes/continuation-wildcard-anytype6-skip-one`, `project/attributes/continuation-wildcard-anytype6-skip-two` |
| [Attribute Wildcard Intersection](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-aw-intersect) clauses 5 and 6, [src-attribute_group](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-attribute_group) clause 2, and the local wildcard's {process contents} for a complete wildcard (§3.6.2) | xmlschema 4.3.2 keeps not(absent) when intersecting it with not(urn:a), accepts an inexpressible intersection and applies strict to a lax complete wildcard | `project/attributes/conformance-group-negation-absent-reversed`, `project/attributes/conformance-group-negation-named`, `project/attributes/conformance-group-wildcard-local-lax` |
| [Complex type mapping §3.4.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-type): complexContent `mixed` overrides complexType `mixed`, and the facets of a simple-content restriction of a mixed base restrict its content type, also beside an inline simpleType | libxml2 2.9.13 uses complexType `mixed` and does not apply those facets; xmlschema 4.3.2 accepts U+00A0 as white space in element-only content | `project/content-models/contrasts-mixed-priority-outer-true-inner-false`, `project/content-models/contrasts-mixed-nested-simple-integer`, `project/simplecontent-restriction-mixed-emptiable-base` |
| [e-props-correct](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-props-correct) clause 2 against an inherited type; cvc-elt clause 5.1.2 with a substitution member as governing declaration | xmlschema 4.3.2 skips a default's check against an inherited type and does not supply a substitution member's own default to identity fields | `project/content-models/substitution-boundaries/inherited-type-invalid-default`, `project/content-models/substitution-member-constraints/default` |
| [cvc-elt](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-elt) clause 5.1: a value constraint applies when "the item has neither element nor character [children]"; comments, processing instructions and an empty CDATA section supply no character, so the default applies | libxml2 2.9.13 treats an empty CDATA section as content | `project/elements/default-ignored-markup` |
| [cvc-identity-constraint](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-identity-constraint) clause 4.2.1: for a key "every member of the ·target node set· is also a member of the ·qualified node set·", in every scope, including a selector `.` on a recursive element | xmlschema 4.3.2 accepts a missing key field on an outer or middle element whose descendant repeats the key | `project/identity/selector-self-key-recursive-scopes` |
