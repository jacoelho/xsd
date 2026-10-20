# Source questions and interpretation boundaries

These records preserve questions about the retained XSD 1.0 text and its
referenced sources. They are independently authored reading notes, not approved
errata. Use the [corrected reading](../corrected-reading.md) for published
corrections and [interpretation records](../INTERPRETATIONS.md) for WG
clarifications and changes explicitly made for XSD 1.1.

Each question identifies the setup and source premises that remain unsettled.
A corpus expectation, implementation result or unavailable observation cannot
choose an answer. The records do not add a rejection rule, a component API or
an assessment phase. Unrelated requirements still apply to each setup.

The source IDs remain stable across the research records. Aliases identify the
same question; they are not additional ambiguities or independent coverage.
Existing local question sections retain ownership where linked below. This is
a reconciliation of known questions, not a claim that no further questions can
arise.

## CTS-Q001 — complex-type annotations with no source annotation

**Status:** unresolved component-property/XML-mapping integration.

Consider an otherwise admitted complex type with no annotation on the type,
its simpleContent/complexContent wrapper, or its derivation child. The bare
named form `<xs:complexType name="T"/>` is one such source shape.

The [component tableau](../upstream/structures.html#ct-annotations) requires an
annotation set. The [XML correspondence](../upstream/structures.html#declare-type)
collects annotations from those source levels, then describes the no-annotation
case as absent. The specification's [absence convention](../upstream/structures.html#key-null)
requires nonoptional properties and distinguishes an empty set from absence.
[Properties Correct](../upstream/structures.html#ct-props-correct) imports the
tableau without expressly reconciling these formulations.

Preserve both formulations. Do not invent an annotation, equate absence with an
empty set, or reject an otherwise admitted constructor to remove this tension.
Exact component-property exposure would observe the disputed result;
success/failure alone does not. [ANQ001](#anq001--annotation-xmllang-and-the-imported-source-version)
concerns a separate source-version premise for annotation attributes.

## ASQ001 — element assessment outcome route connective

**Status:** unresolved integration of the printed outcome-rule structure.

In [Assessment Outcome (Element)](../upstream/structures.html#e-validity),
the nested block 1.1.1 lists two subclauses: declaration-route assessment
applied and declaration-local validity succeeded; type-route assessment
applied and type-local validity succeeded. No connective introduces that
nested pair. The nearby [assessment entry](../upstream/structures.html#c-xd)
explicitly selects one of its declaration and type alternatives, and notes
that they generally apply separately, with the stated xsi:type precedence.

Preserve this missing-connective integration question before treating a
route-based validity calculation as universal. The ordinary applicable-route
reading used by [IP-015](interactions.md#ip-015--missing-declarationtype--assessment-route--strictness--descendants)
is conditional on that reading. This establishes neither a wrong ordinary
example nor an approved AND/OR repair, two permitted processor policies, or a
blanket rejection rule. Other explicit outcome conditions, including invalid
descendants, mustFind/notKnown descendants and validation-attempted branches,
remain separate. Record the reading and observing PSVI boundary when comparing
the affected validity results.

## IBS-Q001 — root ID eligibility and successful declaration validation

**Status:** unresolved success/contribution consistency question, not a
permitted choice of validity outcomes.

Consider a validation root declared directly ID, or a simple restriction of
ID, with valid text `a`, no attributes or element children, and no independent
failure. [Eligibility](../upstream/structures.html#key-eas) requires successful
validation against the declaration in the same context. Eligible ID values
introduce table keys even when there are no IDREF values. The
[owner predicate](../upstream/structures.html#iib-binding) uses an element's
ID-valued attributes or children; the root's own content supplies no such
same-context parent owner. [cvc-id](../upstream/structures.html#cvc-id) forbids
every empty binding, and [root declaration clause 7](../upstream/structures.html#cvc-elt)
requires that condition.

Successful root validation would therefore introduce an ownerless key and
violate a condition of that same success. If the listed success and contribution
conditions are read as simultaneous, exhaustive definitions, this minimal setup
has no consistent assignment. Preserve the question of how those definitions
are reconciled. This supplies no provisional phase, root-self owner rule,
blanket ID-element rejection or processor pass/fail oracle.

A separately eligible IDREF attribute can instead create the key independently;
with no ID-valued attribute or child owner, the ordinary empty-binding failure
then applies regardless of the root's eligibility. General
[type derivation](../upstream/structures.html#Type_Derivation), including complex
extension from a simple base, must be checked before excluding a carrier type.
Optional table exposure does not settle the success/contribution question.

## IDS-Q001 — identity-constraint refer and generic QName resolution

**Status:** unresolved integration of a normative reference.

An element-owned keyref is intended to refer to a key or unique component.
The [refer mapping](../upstream/structures.html#declare-key) invokes
[QName resolution](../upstream/structures.html#key-resolve), whose
[kind/property cases](../upstream/structures.html#src-resolve) enumerate six
component kinds without an identity-constraint entry. The
[schema tableau](../upstream/structures.html#Schema_details) has no root
identity-constraint set. Element ownership and the
[identity-constraint symbol space](../upstream/structures.html#identity-constraint_definitions)
do not expressly supply the omitted generic indexing case.

Keep the intended referenced component as an explicit premise. The clear
target-category, arity, component-identity and tuple rules still apply once
that premise is supplied. Do not infer rejection of every keyref or add an
unpublished indexing algorithm to conceal the missing integration step.

## XPS-Q001 — XPath component syntax and XML representation

**Alias:** XPQ001. **Status:** unresolved intended relationship between distinct
representation and component phases.

Consider internal XML whitespace in `r / s` or `@ id`, or an equivalent
expanded axis spelling. The [selector](../upstream/structures.html#c-selector-xpath)
and [field](../upstream/structures.html#c-fields-xpaths) component rules permit
token-boundary whitespace and specified equivalent axis forms. The normative
[Appendix A attribute patterns](../upstream/structures.html#normative-schemaSchema)
are narrower for these spellings. Their [token base](../upstream/datatypes.html#token)
collapses whitespace but retains a single internal space.

These are different phases, so a narrower XML representation is not itself a
same-phase contradiction. [Schema representation](../upstream/structures.html#src-identity-constraint)
requires the syntax and corresponding component constraints together. The
[precedence note](../upstream/structures.html#intro-terminology) for Representation
Summary illustrations does not make Appendix A merely illustrative. Preserve
both layers and the reconciliation question. Component-permitted syntax alone
does not establish mandatory admission of the XML spelling.

## ANQ001 — annotation xml:lang and the imported source version

**Status:** unresolved historical import-body and source-selection premise.

For annotation documentation, compare empty `xml:lang` with a valid language
tag. The dated [representation summary](../upstream/structures.html#element-documentation)
uses [language](../upstream/datatypes.html#language), which excludes empty.
The cited [XML Second Edition](../dependencies/xml-1.0-second-edition.html#sec-lang-tag)
does not provide a later empty-language reset exception. Normative
[Appendix A](../upstream/structures.html#normative-schemaSchema), however,
references an imported XML-namespace declaration.

The independently distributed [retained xml.xsd](../upstream/xml.xsd) was
modified in 2009 and admits empty through its lang union. It does not prove
which declaration body the 2004 import selected and is not an approved XSD
amendment. The dated clauses support empty rejection under their premises;
keep the import-body/version qualification visible. Do not turn that result
into generic XML well-formedness rejection or automatic empty acceptance from
the later distribution. Source selection also precedes any complete whitespace
oracle for that later union.

## CMP-Q001 — unused present-empty import namespace

**Alias:** ICP-Q001. **Status:** unresolved semantic-domain scope.

Consider `<xs:import namespace=""/>` with no resolved referent and no needed
imported component. Keep it distinct from an omitted namespace attribute and
from an empty schema targetNamespace. The import's
[namespace attribute](../upstream/structures.html#element-import) has
[anyURI](../upstream/datatypes.html#anyURI) syntax, which admits empty.
The enumerated [import constraints](../upstream/structures.html#src-import)
give no explicit present-empty prohibition, and matching a referent's namespace
is conditional on obtaining that referent. The broader
[schema namespace discussion](../upstream/structures.html#declare-schema)
calls an empty targetNamespace incoherent; its application to this unused
import is the scope question.

Lexical anyURI admission does not prove semantic acceptance, and a corpus's
invalid expectation does not supply the missing scope premise. Retain the
ordinary absent/same/different namespace and obtained-referent rules. Do not
equate empty with absent or invent a resolution strategy to settle this case.

## CMP-Q002 — restrictive attribute-group redefine and nonself references

**Status:** unresolved intended scope of the explicit-only explanation.

For a no-self-reference attribute-group redefinition,
[clause 7.2](../upstream/structures.html#c-agrere) uses the ordinary
[attribute-group mapping](../upstream/structures.html#declare-attributeGroup)
and requires its uses/wildcard to restrict the original. That mapping includes
contributions from nonself group references. The adjacent explanatory note
describes all and only explicit attribute children and an explicit anyAttribute,
while expressly excluding inheritance from the original group.

Hold an otherwise admitted original A with required x and a separate H
contributing the matching required x. A no-self replacement of A containing
only a reference to H distinguishes referenced contributions from implicit
original inheritance. The ordinary mapping supplies x; a literal explicit-only
reading of the note describes no use, which would fail required-use preservation.
Retain matching expanded names/types and independent restriction/grammar rules.

The absence of inheritance from original A remains settled. Preserve the
normative mapping versus explanatory-note authority distinction; the note
alone supplies no blanket rejection of nonself references. Record this
integration question before assigning a unique note-based component or schema
oracle, and do not declare two permitted processor policies. No existing
ordinary no-reference example is disproved by this setup.

## DCS-Q001 — attribute-use constraints and declaration-scoped rules

**Status:** unresolved intended transfer between distinct component properties.

For a named local attribute, the [XML mapping](../upstream/structures.html#declare-attribute)
places default/fixed on the attribute use; its newly constructed declaration's
value constraint is absent. A reference can likewise supply a separate use
constraint. [src-attribute clause 5](../upstream/structures.html#src-attribute)
invokes declaration correctness, whose [clauses 2–3](../upstream/structures.html#a-props-correct)
require canonical-form validity and prohibit a declaration constraint for an
ID-derived type. [Attribute Use Correct](../upstream/structures.html#au-props-correct)
requires property domains and consistency with a declaration's fixed value,
without expressly transferring those two conditions to the use's own property.

The mapping still requires an [actual value](../upstream/structures.html#key-vv)
with respect to the declaration's type. Literal/type admission is not missing.
Keep it separate from revalidating that value's canonical spelling.

Two minimal construction probes expose the distinction: an otherwise legal
optional local attribute with `type="xs:ID" default="a"`; and a local attribute
whose [boolean](../upstream/datatypes.html#boolean) restriction has
[pattern](../upstream/datatypes.html#rf-pattern) `1` and `fixed="1"`. The latter
literal supplies true, while its canonical spelling `true` fails the pattern.
In the corresponding global declarations, the declaration-level conditions
directly apply. They do not name the local use's separate constraint.

Preserve the intended-transfer question without inventing universal acceptance
or rejection. Present attributes still undergo [declaration type validation](../upstream/structures.html#cvc-attribute)
and the [use's fixed canonical-string check](../upstream/structures.html#cvc-au).
The [XML ID-default validity constraint](../dependencies/xml-1.0-second-edition.html#id-default)
concerns DTD attribute declarations and does not expressly bridge these XSD
properties. [E1-56](../upstream/errata-second-edition.html#e1-56) changes PSVI
contributions without supplying that construction bridge. Later-version rules
and corpus expectations cannot be imported as an approved 1.0 correction.
The declaration-fallback question at [B-044](branches/content-and-derivation.md#b-044--synthesized-attribute-psvi)
remains separate.

## PSVI-Q001 — union-member amendment sites and contributor scope

**Status:** unresolved attribute full/lightweight integration; exact element
patch-site and entry qualifications retained separately.

E1-56 amends the paragraphs preceding the lightweight member namespace,
anonymous and name fields for [attributes](../upstream/structures.html#a-key-amt)
and [elements](../upstream/structures.html#key-amt). The earlier full
[attribute member component](../upstream/structures.html#a-member_type_definition)
and [element member component](../upstream/structures.html#e-member_type_definition)
wording is not replaced by those instructions. Each still identifies a member
that actually validated the value. The attribute amendment adds governing-type
validity; the element amendment requires a nonabsent schema-normalized value
and validates that resulting value. Preserve these exact sites and their
respective contributor entries rather than treating the amendment as a textual
replacement of both alternatives.

An attribute boundary probe uses an admitted string-first/boolean union
restricted by enumeration `a`: value `b` matches the string member but fails
the governing union facet. Member acceptance and governing-type validity are
distinct under [Datatype Valid](../upstream/datatypes.html#cvc-datatype-valid).
Before assigning a unique full-member oracle, retain what “actually validated”
and [Attribute Validated by Type](../upstream/structures.html#sic-attrType)'s
entry establish, alongside the explicitly patched lightweight paragraph.
Ordinary first/later/no-member-match examples do not resolve that boundary.
This record requires neither member presence nor absence there and does not
classify two processor policies as permitted.

For elements, [Element Validated by Type](../upstream/structures.html#sic-eltType)
retains its successful type-validity outer premise. A lexical/type-invalid
value, including invalid complex attributes, does not establish that entry.
An inner absent-value case therefore supplies no universal absence requirement
outside it. Declaration-only fixed or identity failure can coexist with
successful type validity and remains a different control. Preserve the
unchanged full-member wording and the amended lightweight paragraph in
nil/default/value contexts without manufacturing a member or a contradiction.

These are reading qualifications, not another approved correction or a new
validity rule. No generated member, mandatory metadata API or general output
policy follows. [Corrected reading](../corrected-reading.md) owns the seven
actual amendments; A-084/A-086/A-089/A-096 retain the qualified element cells.

## Adjudicated particle boundaries

[I-004](../INTERPRETATIONS.md#i-004--published-member-minimum-rule) resolves
the member-minimum corpus oracle under the published-1.0 baseline: both member
and aggregate checks apply. The acknowledged historical bug remains an intent
qualification, not an approved repair or an open choice of baseline rules.

[I-005](../INTERPRETATIONS.md#i-005--implicit-substitution-choice-order)
classifies generated substitution-choice order as underspecified. Its history
supports no canonical sort or adopted order waiver. Order-sensitive outcomes
remain unscored; order-independent consequences can receive hard expectations.

## Existing question and choice owners

The linked catalogues retain the detailed scenarios. Their classifications
remain distinct from the unresolved integration records above.

| Owner | Boundary |
| --- | --- |
| [D-072](datatypes.md#d-072--questions-and-permissible-choices-that-must-remain-visible), [D2-048](branches/datatypes.md#d2-048--source-tensions-and-exact-unresolved-or-permitted-oracles) | RXQ001/RGQ001 is one bare-brace grammar/prose question. RXQ002 records Unicode-version premises and permitted future support. The one-member union construction is settled by the concrete rules. |
| [D-013](datatypes.md#d-013--fundamental-facet-derivation), [D-072](datatypes.md#d-072--questions-and-permissible-choices-that-must-remain-visible) | FFP004/006/008 retain formula-versus-value-space examples. FM13 preserves the union bounded/ordered common-ancestor wording asymmetry; these row IDs are not new question IDs. |
| [XR-115/116](external-rules.md) | Printed double parameters versus the cited IEEE parameters, and the conditional external overflow result versus an unqualified XSD oracle. These already have source-qualified records. |
| [A-106](branches/declarations.md#source-tensions-and-limits) | Same-type affiliation under both head exclusions: formal type-derivation identity versus §3.3.1 nomination prose. Corpus expectations do not settle this question. |
| [Structures identity, order, default and algebra notes](branches/content-and-derivation.md#unresolved-specification-tensions-no-invented-rejection) | Anonymous identity choices, substitution-set ordering, attribute-use/declaration fallback and wildcard operand selection retain their local owners. |
| [I-001–I-003](../INTERPRETATIONS.md) | WG clarification and later-version identity-field changes remain separate from published 1.0 errata. |

A source/corpus expectation discrepancy is comparison evidence, not a second
published permission. Unavailable PSVI or component output is an observation
limit. Neither class creates another normative source conflict.
