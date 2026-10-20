# Structures scenario catalogue

This catalogue derives testable partitions from **XML Schema 1.0 Part 1:
Structures, Second Edition**, its normative appendices, and the subsequent
[E1-56 correction](../upstream/errata-second-edition.html#e1-56). It is an
independent specification oracle: no implementation behavior or corpus result
is evidence for the expectations below. The [published Recommendation](../upstream/structures.html)
is the base text. Its XML source and the W3C `structures-with-errata.html` change
rendering contain first-edition editorial history; neither alone incorporates
E1-56. Apply the [source audit](../SOURCES.md) when selecting effective text.

A family is a finite set of semantic equivalence partitions, not a claim to
enumerate every XML document. Each listed decision requires a witness for every
branch; each boundary requires witnesses on both sides and at equality where
meaningful. A negative witness violates only the named rule unless the family
explicitly asks for interacting failures. A passing schema compilation alone
cannot prove instance validity, PSVI contributions, or processor conformance.

Observation codes are **S** (schema construction, including XML representation
admission), **V** (instance assessment), **P** (PSVI), and **O** (processor option
or invocation contract). These codes identify an oracle surface, not a required
repository API. An unavailable PSVI surface is an evidence gap, not an automatic
validation defect. `absent` means the specification's absent value, distinct
from an empty string. `R` and `B` mean restricted and base particles/types.

## Reading and fixture expansion rules

For every family, preserve expanded names, component kind, scope, derivation
method, and value identity independently. Prefix spelling and XML serialization
are additional witnesses only where namespace context or XML normalization can
change a result. Use both direct components and components reached through
named groups, derivation, include, import, or redefine when those paths affect
the rule. Run required interactions listed below after single-rule witnesses.

All XML representation tests apply the normative schema-for-schemas grammar
**and** the prose representation/component constraints. Absence of a restriction
in an illustration does not override Appendix A. Do not replace a specification
option, recommendation, or unspecified case with an invented required rejection.
Part 2 owns datatype value/facet/regex details; S-060–S-063 cover the Structures
interface and link to that independent catalogue when available.

## Foundations, schema representation, and names

### S-001 — input and conformance layers

Source: ([§§1.2, 2.4](../upstream/structures.html#concepts-conformance), [§4.1](../upstream/structures.html#layer1)).

Supply an XML Infoset with required properties; a component schema supplied without XML; and the same schema represented as XML. Distinguish minimum conformance, XML-representation conformance, and full Web conformance. Exercise XML 1.0 documents with and without namespace use.

Core constraints/assessment/PSVI apply to all processors; XML grammar/mappings apply to processors claiming XML-representation support; Web retrieval obligations attach to the full level. A non-network library is not thereby defective under the first two levels. **S/V/P/O**.

### S-002 — schema-document grammar

Source: ([§3.15.2](../upstream/structures.html#declare-schema), [A](../upstream/structures.html#normative-schemaSchema)).

Empty schema; initial `include/import/redefine/annotation` sequence; any order of later global definitions with interleaved annotations. Move a composition directive after a global definition; omit required names, add forbidden attributes, reorder children, repeat singleton children, add an unknown XSD element, or put text in element-only grammar. Exercise every production in the representation inventory below.

Appendix A admits/rejects syntax; prose adds constraints it cannot express. Forward component references are not invalid merely because definitions appear later. Schema element may be an embedded Infoset element, not necessarily a document root. **S/O**.

### S-003 — lexical attributes and extensibility

Source: ([A](../upstream/structures.html#normative-schemaSchema), [§3.13.2](../upstream/structures.html#declare-annotation)).

For each grammar attribute test absent/default, a legal lexical form, another legal spelling of the same value, and malformed/out-of-domain values. Test foreign namespaced attributes with a declaration, without one, and with an available declaration they violate; compare an unqualified unknown attribute and an unknown attribute in the XSD namespace. Test foreign child markup inside and outside annotation content.

Follow the actual attribute wildcard and its `lax` processing: a foreign attribute is not an unconditional bypass when a declaration is available. Open attributes do not permit arbitrary foreign child elements in ordinary XSD grammar. Annotation content has its own open grammar. `id` is XML ID, unique in its schema document; `version` has no assessment semantics. **S/P**.

### S-004 — symbol spaces and scope

Source: ([§2.5](../upstream/structures.html#concepts-nameSymbolSpaces), [§3.15.6](../upstream/structures.html#sch-props-correct)).

Same expanded name in element/type/attribute/group/attribute-group/notation/identity-constraint spaces; duplicate names within each one; simple versus complex type with the same expanded name; same local name in different namespaces; global versus local and locals in distinct complex types. Include/import the same component twice versus two distinct conflicting definitions.

Different symbol spaces coexist; simple and complex types share one type space. Identity constraints have their own global identity space. Local element and attribute spaces belong to each complex type and remain distinct from globals. Repeated inclusion of the same components does not create distinct duplicates. **S**.

### S-005 — QName interpretation

Source: ([§3.15.3](../upstream/structures.html#src-qname)).

Prefixed QName with bound, rebound, shadowed, or undeclared prefix; unprefixed QName with default namespace present/absent; lexical whitespace; malformed QName/NCName. Compare the default namespace for QName **values** with the absence of a namespace on unprefixed XML attribute **names**. Repeat with reconstructed in-scope namespaces.

Resolve QName values in the containing element's namespace context; an unprefixed value uses its default namespace if present, otherwise absent. An unbound prefix fails. Prefix aliases with the same binding denote the same expanded name. A component's `name` is an NCName, not a reference QName. **S/V**.

### S-006 — QName lookup and import permission

Source: ([`src-resolve`](../upstream/structures.html#src-resolve), [`cvc-resolve-instance`](../upstream/structures.html#cvc-resolve-instance)).

Cross each reference kind with matching component, wrong kind, wrong namespace, missing component, and same name in another symbol space. In schema documents cross own namespace/absent/foreign namespace with a matching `import`, no `import`, or an import for another namespace. Test component already available through another document while this document lacks required import permission.

Schema lookup requires the right kind/name/namespace **and** this document's permitted namespace relationship. A global component in the assembled schema is insufficient permission for a foreign schema reference. Instance lookup uses schema component tables, without imposing schema-document import syntax on the instance. Missing usable components feed S-071, not a fabricated lexical QName error. **S/V/O**.

### S-007 — defaults and namespace mappings

Source: ([§§3.2.2](../upstream/structures.html#declare-attribute), [3.3.2](../upstream/structures.html#declare-element), [3.4.2](../upstream/structures.html#declare-type), [3.15.2](../upstream/structures.html#declare-schema)).

Cross target namespace absent/nonempty with global/local declaration, form default qualified/unqualified, explicit `form` absent/qualified/unqualified. For `block`/`final`, cross explicit absent/empty/one method/all methods/`#all` with schema defaults containing applicable and inapplicable methods. Test empty `targetNamespace` separately from absence.

Globals take schema target namespace; local declaration qualification follows explicit form then its form default. Apply only methods relevant to the owning property; explicit empty overrides a nonempty default. Finality is not inherited. Empty target namespace is incoherent, not the no-namespace case. **S/V**.

### S-008 — component properties and identity

Source: ([§3.1](../upstream/structures.html#scIntro), [identity note](../upstream/structures.html#no-identity)).

For every component in the inventory below test required, optional-absent, empty-set/sequence, and wrong-kind property shapes at its construction boundary. Compare named identity, identity by construction/inheritance, and separately built anonymous but structurally equal types.

Property-tableau domains and all applicable constraints apply recursively, subject to §5.3 missing-subcomponent rules. Named components of the same kind/name/namespace and shared components by construction have defined identity. The Recommendation explicitly leaves other identity cases incomplete; record allowed processor disagreement instead of demanding structural interning or pointer inequality. **S/V/O**.

The source models a schema as a labelled property graph rooted at the schema,
with component/literal vertices and property edges; reentrant references can
occur. This is an idealization: embedding and indirection are implementation
choices. A subset or superset property permits equality unless **proper**
inclusion is explicitly required. The XML/component correspondences also
determine a reverse correspondence in the abstract. These conventions impose
no object layout, unique source spelling, serializer API or byte round trip.


### Component and representation inventory

The 13 component kinds in [§2.2](../upstream/structures.html#concepts-data-model)
are all included below. The schema container in §3.15 is additional. For every
property, S-008 supplies the shape/absence partition; the named families supply
semantic and XML-to-property decisions.

| Component | Properties and representation paths requiring independent witnesses | Families |
| --- | --- | --- |
| Attribute declaration | name, target namespace, type, scope, value constraint, annotation; global declaration, local declaration, reference to global; implicit `anySimpleType` | S-007, S-009–S-012 |
| Element declaration | name, target namespace, type, scope, value constraint, nillable, identity constraints, substitution affiliation, exclusions, block, abstract, annotation; global/local/ref; explicit type, inline type, head-inherited type, implicit `anyType` | S-007, S-013–S-017, S-055–S-057, S-079 |
| Complex type | name, namespace, base, derivation method, final, abstract, attribute uses/wildcard, content, block, annotations; named/anonymous; shorthand, simpleContent and complexContent extension/restriction | S-018–S-028 |
| Attribute use | required, declaration, value constraint; named local attribute and global attribute reference; prohibited declaration yields no use | S-009–S-011, S-020, S-026 |
| Attribute-group definition | name, namespace, uses, wildcard, annotation; named definition versus reference expanding uses/wildcard | S-023, S-029, S-065 |
| Model-group definition | name, namespace, group, annotation; named definition versus reference yielding a particle | S-030, S-065 |
| Model group | compositor, ordered particle list, annotation; all/choice/sequence and nested or named reuse | S-030–S-035, S-038–S-047 |
| Particle | min, max, term (element/wildcard/group); local/ref element, group reference, all/choice/sequence, any; `0..0` representations yield no particle | S-035–S-047 |
| Wildcard | namespace constraint, process contents, annotation; any/anyAttribute, local, inherited, intersected and unioned forms | S-023, S-048–S-052 |
| Identity-constraint definition | name, namespace, category, selector, ordered fields, referenced key, annotations; unique/key/keyref and selector/field annotations | S-053–S-057 |
| Notation declaration | name, namespace, system/public identifier, annotation; notation representation | S-058 |
| Annotation | ordered appinfo, ordered documentation, foreign attributes; explicit annotation and enclosing component attributes | S-003, S-059 |
| Simple type | name, namespace, base, constraining/fundamental facets, final, variety, annotation; primitive/item/member according to variety; named/anonymous restriction/list/union | S-060–S-063; Part 2 catalogue |
| Schema container | sets of global type/attribute/element/group/attribute-group/notation components and annotations; collected through composition; built-in types and four `xsi` attributes | S-002, S-004, S-064–S-069, S-080 |

For S-002/S-003, expand each row of this representation checklist. In each
context, test every allowed attribute, missing required attributes, every
child alternative and multiplicity, order boundaries, and disallowed attributes
from the related contexts. Attribute lexical spaces come from Appendix A and
Part 2; `id` and foreign-attribute handling are shared S-003 partitions.

| XML representation/context | Grammar and context distinctions |
| --- | --- |
| schema | Composition/annotation prefix, then global definitions with annotation interleaving; targetNamespace, form defaults, derivation defaults, id/version/xml:lang |
| attribute: global/local/ref | Global name required; local name/ref alternative; type/inline simpleType alternative; form/use/default/fixed contextual restrictions; annotation before simpleType |
| element: global/local/ref | Global name, abstract/final/substitutionGroup; local occurrence/form; ref excludes declaration-only content/attributes; annotation, optional inline type, then identity constraints |
| complexType: global/anonymous | Global name required, anonymous name forbidden; annotation then exactly one content alternative; attribute uses before terminal anyAttribute; contextual abstract/block/final attributes |
| simpleContent and complexContent | annotation then exactly one extension/restriction; only complexContent has meaningful mixed; every derivation `base` required |
| simpleContent restriction/extension | Restriction can add inline simpleType/facets before attributes; extension cannot contain those; one terminal anyAttribute |
| complexContent restriction/extension | Optional single content group before attribute/group uses and final anyAttribute; no facets or inline simpleType |
| attributeGroup definition/ref | Definition has name and ordered attributes/group references before anyAttribute; ref has required ref and only optional annotation |
| group definition/ref | Definition names exactly one all/choice/sequence; ref has required ref, occurrences, optional annotation but no model body; definition's child group cannot carry occurrences |
| all | min 0/1 and max 1; annotation then local/ref elements only; child occurrences 0/1 under prose constraints |
| choice/sequence | annotation then zero or more element/group/choice/sequence/any; occurrences; all is not a nested alternative |
| any and anyAttribute | namespace tokens/list and processContents; optional annotation; only any has occurrence attributes |
| unique/key/keyref | Required name; keyref additionally required refer; annotation then exactly one selector and one or more fields |
| selector/field | Required restricted XPath and optional annotation; attribute axis only in a field's last step |
| notation | Required name; optional public/system in Appendix A, at least one required by the component tableau; public is token, system anyURI |
| annotation | zero or more appinfo/documentation, ordered; id and foreign attributes |
| appinfo/documentation | Mixed arbitrary content; source anyURI; documentation xml:lang; foreign attributes; no ordinary annotated-type id permission |
| simpleType global/anonymous | Named global versus unnamed local; annotation then exactly one restriction/list/union; final contextual grammar |
| simpleType restriction | Exactly one base reference/inline simpleType by prose; annotation, optional inline simpleType, facet children |
| list | Exactly one itemType/inline simpleType by prose; optional annotation and inline type |
| union | memberTypes followed by inline simpleType members; zero members fails component rules; nested union flattening preserves order |
| facet elements | Each admitted Part 2 facet's value/fixed/id grammar, annotation position, repeated-facet rules; enumeration and pattern have distinct grammars |
| include/import/redefine | include and redefine require schemaLocation; import namespace/location optional; only redefine admits definitions, restricted to simpleType/complexType/group/attributeGroup |

## Declarations, defaults, and instance type selection

### S-009 — attribute representation

Source: ([`src-attribute`](../upstream/structures.html#src-attribute), [§3.2.2](../upstream/structures.html#declare-attribute)).

Global/local/ref; neither/both name and ref; named/inline/omitted type; both type and inline; ref plus type/form/inline; `use` optional/required/prohibited; default/fixed alone/together. Cross prohibited with inherited use in a restriction.

Local named syntax creates declaration plus use; ref creates only a use of the global declaration; prohibited yields neither use nor new declaration. Omitted type is anySimpleType. default and fixed exclude each other; default requires optional use. Global grammar remains distinct. **S**.

### S-010 — attribute constraints and reserved names

Source: ([`a-props-correct`](../upstream/structures.html#a-props-correct), [`au-props-correct`](../upstream/structures.html#au-props-correct), [`no-xmlns`](../upstream/structures.html#no-xmlns), [`no-xsi`](../upstream/structures.html#no-xsi)).

Valid/invalid default and fixed literals; ID and ID-derived type with a value constraint; local use absent/default/fixed when referenced declaration has fixed value; same/different typed values and lexical aliases. Declare `xmlns`, `xmlns:p`, or any attribute in the xsi namespace.

The mapping requires a typed actual value for a constraint literal. A declaration's value constraint additionally requires canonical-form validity and is forbidden for an ID-derived declaration type. Local named and reference constraints belong to the separate attribute use; preserve [DCS-Q001](source-questions.md#dcs-q001--attribute-use-constraints-and-declaration-scoped-rules) rather than automatically transferring those declaration-only conditions. A use overriding a declaration's fixed constraint must retain the same fixed value. User declarations of xmlns and xsi attributes are forbidden; the four built-in xsi declarations are the exception. **S/V**.

For an anySimpleType value constraint, retain [D-004's unspecified mapping boundary](datatypes.md#d-004--component-identity-and-the-simple-ur-type)
before assigning a unique typed/canonical value; independent construction
conditions still apply.

### S-011 — attribute validation and use

Source: ([`cvc-attribute`](../upstream/structures.html#cvc-attribute), [`cvc-au`](../upstream/structures.html#cvc-au), [`cvc-assess-attr`](../upstream/structures.html#cvc-assess-attr)).

Present/missing declaration and type; value valid/invalid; declaration fixed equal/unequal by typed value; use fixed compared with its canonical lexical representation; required missing/present; undeclared wildcard-matched or skip attribute. Use numeric/boolean lexical aliases to distinguish value equality from string conditions.

Declaration validates simple value and declaration-level fixed value. Use-level fixed has the lexical comparison stated by `cvc-au`; retain this separately rather than silently replacing all fixed checks with one policy. Attribute strict assessment needs a usable declaration/type and completed evaluation. Explicit matching use wins over wildcard. **V/P**.

### S-012 — attribute PSVI and E1-56

Source: ([§3.2.5](../upstream/structures.html#sic-attrType), [E1-56](../upstream/errata-second-edition.html#e1-56)).

Valid/invalid normalized value with fixed comparison succeeding/failing independently; atomic/list/union; union value matching first/later/no members; named/anonymous members; strictly assessed versus unassessed; present/defaulted attribute.

E1-56 makes normalized value absent when it is invalid against the governing simple type, and gates union member identity on successful value validation. A value can pass the simple type yet fail a separate fixed condition; preserve those separate outcomes. Record full type-component versus lightweight type/member properties according to processor choice. Defaulted attributes use S-020; ordinary attributes are schema specified `infoset`. **P/O**.

For a member accepting while a governing union facet fails, retain
[PSVI-Q001's exact full/lightweight amendment-site qualification](source-questions.md#psvi-q001--union-member-amendment-sites-and-contributor-scope)
before treating that summary as a unique full-member oracle.

### S-013 — element representation

Source: ([`src-element`](../upstream/structures.html#src-element), [§3.3.2](../upstream/structures.html#declare-element)).

Global/local/ref; named/inline/omitted type; default/fixed conflict; ref combined with each forbidden child/attribute; zero occurrence; substitution head present/absent with explicit or omitted type.

A local/ref element yields particle plus declaration/reference unless 0..0 yields nothing. ref allows only its contextual occurrence/id/annotation syntax; declaration-only properties cannot override the global. Omitted type is the head's type when affiliated, otherwise anyType. Identity constraints belong to declaring syntax, not ref syntax. **S**.

### S-014 — declaration versus actual type

Source: ([`cvc-elt`](../upstream/structures.html#cvc-elt), [`cvc-type`](../upstream/structures.html#cvc-type)).

Present/missing declaration/type; abstract element with concrete type; concrete element with abstract declared type; concrete derived xsi:type; wrong local name/namespace at a particle versus processor-stipulated root declaration.

Abstract element declarations cannot directly validate their own name. An abstract declared type is usable only when a permitted concrete actual type is selected. Simple type content admits only the four special xsi attributes, no element children, and a valid simple value unless nil rules suppress content validation. Name matching belongs to particle selection or the stipulated-root invocation check. **V/O**.

### S-015 — xsi:type

Source: ([`xsi_type`](../upstream/structures.html#xsi_type), [`cvc-elt`](../upstream/structures.html#cvc-elt), [`cvc-assess-elt`](../upstream/structures.html#cvc-assess-elt)).

QName lexical validity/binding/lookup; same type, immediate/multistep derived type, unrelated type, abstract type; extension/restriction at each step; element block and type block individually/jointly; simple/complex actual types; xsi:type on undeclared element and with a processor-stipulated type.

Select the resolved permitted actual type only after all QName/resolution/derivation checks. Block unions apply as specified. A same-type identity case is distinct from a nontrivial derivation. A usable xsi:type can permit strict type-based assessment even without an element declaration; it does not bypass attribute/content validation. **V/P/O**.

### S-016 — xsi:nil

Source: ([`xsi_nil`](../upstream/structures.html#xsi_nil), [`cvc-elt`](../upstream/structures.html#cvc-elt)).

nillable false/true × nil absent/false/0/true/1/malformed; empty, whitespace-only, text, element, comment, or PI content; default/fixed/no constraint; required/invalid/extra attributes; abstract/wrong xsi:type.

In the published `cvc-elt` rule, nillable=false forbids the **presence** of xsi:nil, including false/0. True nil on nillable requires no character or element children and no fixed value constraint. Comments/PIs are not character/element children. Nil bypasses content-value/model checks only; attributes, type derivation, identity constraints, and abstract rules still apply. False nil does not bypass ordinary content checks. **V/P**.

### S-017 — element default/fixed and E1-56

Source: ([`cos-valid-default`](../upstream/structures.html#cos-valid-default), [`cvc-elt`](../upstream/structures.html#cvc-elt), [`sic-eltDefault`](../upstream/structures.html#sic-eltDefault), [E1-56](../upstream/errata-second-edition.html#e1-56)).

Missing element versus present empty element; empty with comments/PIs versus whitespace/text/child; simple, simple-content complex, mixed emptiable/nonemptiable, empty and element-only type; ID-derived types; xsi:type permitting/rejecting default; default versus fixed; union values whose original lexical form differs from canonical form; nil true/false.

Defaults never synthesize a missing element. Value constraints require simple/simple-content or mixed emptiable content, and exclude ID-derived values. Empty nonnil uses the constraint for assessment; fixed additionally constrains nonempty content, with mixed character comparison distinct from typed simple comparison. E1-56 supplies the constraint's lexical form for defaulted simple-content PSVI; mixed defaults do **not** create schema normalized value or character children. Schema specified is `schema` exactly when the default-use clause applied. **S/V/P**.

An anySimpleType default whose value mapping is implicated retains
[D-004's qualification](datatypes.md#d-004--component-identity-and-the-simple-ur-type);
lexical admission alone supplies no unique typed default.


## Complex types and attribute assembly

### S-018 — four content categories

Source: ([`cvc-complex-type`](../upstream/structures.html#cvc-complex-type)).

Empty/simple/element-only/mixed × no content/characters/XML whitespace/non-XML whitespace/element/comment/PI. For models, cross legal/illegal child order and cardinality with text before/between/after children. Repeat nil true where permitted.

Empty rejects character and element children; simple rejects element children and validates concatenated character value; element-only permits only XML whitespace character items and must satisfy its model; mixed permits arbitrary text but still requires its element sequence. Comments/PIs do not become character content. Nil affects content checks per S-016. **V**.

### S-019 — complex attributes and ID wildcard rules

Source: ([`cvc-complex-type`](../upstream/structures.html#cvc-complex-type)).

Explicit use absent/present/invalid/required; unknown attribute with absent/disallowing/allowing wildcard; matching declared use plus permissive wildcard; the four special xsi attributes versus another xsi name. Cross zero/one/two ID-derived wildcard attributes, wildcard strict/lax/skip, and a declared ID use present/absent.

Explicit uses take precedence. Required uses must appear; default synthesis does not excuse a required absent use. The four built-ins are handled specially, not every xsi attribute. At most one validated wildcard ID is allowed, and none is allowed when the type has an ID-derived use, even if that declared optional ID is absent. Skip does not confer validated ID identity. **V/P**.

### S-020 — attribute default PSVI

Source: ([`sic-attrDefault`](../upstream/structures.html#sic-attrDefault)).

Optional/required × missing/present × default/fixed/no constraint; local versus referenced declaration and effective use constraint; qualification; atomic/list/union; lexical normalization.

Missing nonrequired uses with a value constraint add an attribute information item with name/namespace, canonical normalized/default value, validation context and valid/full/schema state. Section 3.4.5 separately says the added items **should also** have full type properties, with member properties where appropriate, or their lightweight alternatives. Preserve that recommendation separately from the listed added fields and from any independently applicable Attribute Validated by Type/E1-56 duties. It supplies no blanket omission policy or absence oracle. A present attribute keeps its own validated value and infoset origin; required missing is not repaired by this contribution. No namespace declaration is synthesized. Check validation context and default participation in identity constraints. **P/V**.

Where an item's type is anySimpleType and constructing its value implicates
the unspecified lexical-to-value mapping, retain [D-004's qualification](datatypes.md#d-004--component-identity-and-the-simple-ur-type)
before assigning a unique value/default oracle. Ordinary constrained-type
default witnesses keep their stated rules.

### S-021 — simpleContent mappings

Source: ([§3.4.2](../upstream/structures.html#declare-type), [`src-ct`](../upstream/structures.html#src-ct)).

Base simple type versus complex simple/empty/mixed/element-only; extension/restriction; mixed base emptiable/not; inline simpleType present/absent and facet children; outer complexType mixed true/false.

Extension may use a simple base or complex simple-content base and retains the simple content type. Restriction uses complex simple-content base, or mixed emptiable base with a required inline simpleType. Other base/content combinations fail. Restriction facets construct the resulting simple content. `mixed` on complexType with simpleContent has no effect in XSD 1.0; do not reject it merely as undesirable. **S/V**.

### S-022 — complexContent mappings

Source: ([§3.4.2](../upstream/structures.html#declare-type), [`src-ct`](../upstream/structures.html#src-ct)).

Explicit complexContent versus shorthand; extension/restriction; simple/complex base; outer/inner mixed absent/true/false; no group, empty all/sequence, empty optional/nonoptional choice, nonempty group; empty/nonempty base.

complexContent requires complex base. Inner mixed overrides outer; then false default. Shorthand is restriction of anyType. Empty effective content, a zero-element mixed sequence, and a nonempty model are distinct mappings. Extension uses inherited content when local content is empty, local content when base is empty, otherwise a 1..1 sequence of base then extension content under the mapping. **S/V**.

### S-023 — attribute-set and wildcard construction

Source: ([§3.4.2](../upstream/structures.html#declare-type), [§3.6.2](../upstream/structures.html#declare-attributeGroup)).

Local attributes/groups/base uses; same-name explicit replacement; prohibited inherited use; extension/restriction/shorthand. Cross no/one/multiple group wildcards with local anyAttribute absent/present; base wildcard absent/present. Vary processContents order in group references and the local wildcard independently of namespace sets.

Group use sets combine; restriction replaces/removes same-name inherited uses, while extension retains base uses. Complete wildcard is local/group intersection; its processContents comes from local wildcard if present, otherwise the **first** contributing group wildcard. Restriction uses complete wildcard; extension unions it with base wildcard and uses complete wildcard's processContents when both exist. Unexpressible required union/intersection is a schema error; empty namespace set is expressible. **S/V**.

### S-024 — complex property invariants

Source: ([`ct-props-correct`](../upstream/structures.html#ct-props-correct)).

Named/anonymous, simple/complex base, extension/restriction; duplicate expanded-name uses from local/group/base; two ID-derived uses; direct/indirect type-base cycle; legal recursive element content versus illegal base cycle.

A simple base requires extension. Attribute names are unique within the type; at most one ID-derived declaration is included. Base chains must reach anyType; only anyType has its specified self-base. Recursive content referring to its containing type is not itself a derivation cycle. **S**.

### S-025 — complex extension

Source: ([`cos-ct-extends`](../upstream/structures.html#cos-ct-extends), [`cos-particle-extend`](../upstream/structures.html#cos-particle-extend)).

Simple/complex base; final permits/blocks; retained/removed/changed base attribute type; wildcard absent/narrowed/widened; content empty/simple/element-only/mixed; base prefix preserved/changed/reordered; extend–restrict–extend chain with a removed feature restored/not restored.

Retain base attributes and permit its wildcard namespace set; simple content type stays identical; compatible model categories preserve base as the initial particle. The full ancestry must permit equivalent extension followed by restriction: an extension cannot restore something removed by an earlier restriction. This is stronger than checking the immediate base alone. **S/V**.

### S-026 — complex restriction of attributes

Source: ([`derivation-ok-restriction`](../upstream/structures.html#derivation-ok-restriction)).

Base final restriction; same expanded name with required/optional/prohibited; R type same/derived/unrelated; effective fixed absent/default/fixed and equal/different; new R attribute allowed/disallowed by B wildcard; B required use retained/omitted/relaxed; R wildcard absent/present/subset/superset and process strength.

Required base uses remain required; optional can become required or be removed. Matching R types derive from B types; a fixed B effective constraint stays fixed with the prescribed same value/string condition. New uses need namespace permission from B wildcard. R wildcard must narrow B and cannot weaken processing, except the specified anyType exception. **S/V**.

### S-027 — complex restriction of content

Source: ([`derivation-ok-restriction`](../upstream/structures.html#derivation-ok-restriction)).

Base anyType; simple-content B/R; mixed emptiable B→simple content; empty B→empty; emptiable/nonemptiable model B→empty; mixed→element-only; element-only→mixed; group→group restriction; pointless empty model against empty base.

Accept only the stated content cases: validly derived simple content, permitted empty restriction, or permitted model category plus valid particle restriction. Mixed cannot be introduced while restricting element-only. Empty-base/pointless-model cases follow normalization, not a blanket ban on element-only syntax. A nonemptiable required model cannot restrict to empty. **S/V**.

### S-028 — complex derivation relation

Source: ([`cos-ct-derived-ok`](../upstream/structures.html#cos-ct-derived-ok)).

D=B, immediate base, multistep chain, unrelated types, chain reaching anyType, transition to simple base; block set empty/extension/restriction/both, blocked method at first/intermediate step.

Identity succeeds independently of a prohibited nontrivial method. Every required derivation step must be permitted and reach B; recursion switches to the simple relation where specified. Stop at the ur-type boundary rather than following its self-base indefinitely. Use in xsi:type, substitution, and restriction scenarios. **S/V**.


## Groups, particles, and derivation

### S-029 — attribute groups

Source: ([`src-attribute_group`](../upstream/structures.html#src-attribute_group), [`ag-props-correct`](../upstream/structures.html#ag-props-correct)).

Named group/ref; nested groups; direct/indirect circular references; same-name distinct uses versus reusing the same use; two ID-derived uses; absent/one/multiple wildcard contributors; redefine self-reference.

References contribute use members and a computed wildcard without a runtime wrapper. Set union includes the same use only once. Two distinct use members with the same declaration name/namespace fail even when their types or constraints agree; two distinct ID-derived use members also fail, even if they share a declaration. Groups without wildcards contribute no operand; required intersections must be expressible and retain the specified processing/annotation provenance. Ordinary direct/indirect cycles fail; redefine self-reference resolves the original under S-065. Definition/ref grammar, including the optional ref annotation, remains [GC-007](grammar-contexts.md#gc-007--attrdecls-and-attributegroup-definitionreference). **S/V**.

### S-030 — model-group definitions and cycles

Source: ([`src-model_group_defn`](../upstream/structures.html#src-model_group_defn), [`mgd-props-correct`](../upstream/structures.html#mgd-props-correct), [`src-model_group`](../upstream/structures.html#src-model_group), [`mg-props-correct`](../upstream/structures.html#mg-props-correct)).

Definition/ref context; missing/multiple model groups; occurrence on definition child versus reference; referenced group reused at distinct model positions; direct/indirect cycles; redefine self-reference; recursion through an element's type.

A named definition owns one model group; a ref creates a particle with the reference's range. Model-group containment cannot cycle at any depth. Legal element/type recursion is not group containment recursion. Distinct uses of one named group create distinct particle positions for UPA; they do not collapse into one position. **S/V**.

### S-031 — model-group validation

Source: ([`cvc-model-group`](../upstream/structures.html#cvc-model-group)).

For sequence, choice, and all use empty/one/multiple children, reordered items, optional/required branches, unexpected items, group repetition and empty subpartitions. Compare groups with no particles.

Sequence needs an ordered partition across all particles; choice needs one branch validating the entire selected sequence; all admits its members in any order with their individual ranges. Empty sequence/all validate empty sequence; empty choice validates no sequence, including empty, unless an outer zero occurrence supplies the empty match. **V**.

### S-032 — all limits

Source: ([`cos-all-limited`](../upstream/structures.html#cos-all-limited), [A](../upstream/structures.html#normative-schemaSchema)).

All as whole type model, named-group definition, reference as whole model, nested in sequence/choice, or repeated; outer min 0/1/>1 and max 0/1/>1/unbounded; member element local/ref with max 0/1/>1 and min 0/1.

Only allowed whole-model/definition placement is accepted; surviving all particle max is 1. Members are element particles with max 0 or 1 before zero-particle elimination. A valid instance may reorder members; that does not imply a restriction may reorder an all definition under S-044. **S/V**.

### S-033 — Element Declarations Consistent

Source: ([`cos-element-consistent`](../upstream/structures.html#cos-element-consistent)).

Same expanded name directly, in nested groups, across repeated named-group references, and implicitly through substitution groups. Same named type versus different named types versus repeated anonymous types; different namespaces; deterministic versus ambiguous positions.

Repeated same-name element particles require the same **named top-level** type name and namespace under the stated rule. Distinct but structurally identical anonymous types do not satisfy that condition. Scope does not excuse inconsistent names. Test independently from UPA: a deterministic sequence can violate consistency and an inconsistent-free choice can violate UPA. **S**.

### S-034 — Unique Particle Attribution

Source: ([`cos-nonambig`](../upstream/structures.html#cos-nonambig), [H](../upstream/structures.html#non-ambig)).

Element/element, element/substitution-member, wildcard/wildcard, wildcard/element or substitution-member overlap. Place overlapping positions in choice/all, adjacent sequence, across nullable groups, at repetition boundaries, after required distinct separators, and in distinct named-group expansions. Compare fixed occurrence with variable occurrence and use same-name alternatives differing only in type/content/attributes.

Select the particle without child content/attributes or future siblings. Overlap is acceptable when current position/state already distinguishes particles; a later discriminator or child value cannot resolve ambiguity. XSD 1.0 has no automatic explicit-element-over-wildcard priority. Apply to compiled component positions, including implicit substitutions. Appendix H guides witnesses but is not a replacement normative algorithm. **S**.

### S-035 — effective ranges and emptiability

Source: ([`cos-seq-range`](../upstream/structures.html#cos-seq-range), [`cos-choice-range`](../upstream/structures.html#cos-choice-range), [`cos-group-emptiable`](../upstream/structures.html#cos-group-emptiable)).

Empty group; leaf min/max 0/1/N/unbounded; nested all/sequence versus choice; optional outer group; zero-total group with unbounded outer range; nonzero child with unbounded range; min zero but nonempty possible. Use deep alternating group kinds.

Sequence/all use sums, choice uses min/max across alternatives, then multiply by the enclosing range, with the stated unbounded cases. An outer unbounded repetition of a zero-total group remains zero; do not turn every `0 × unbounded` shape into infinity. Emptiable is outer min=0 or group effective minimum=0. Keep emptiability distinct from empty language (nonoptional empty choice). **S/V**.

### S-036 — occurrence and particle assessment

Source: ([`p-props-correct`](../upstream/structures.html#p-props-correct), [`cvc-particle`](../upstream/structures.html#cvc-particle)).

Occurrence lexicals, min greater/equal/less than max, finite/unbounded, omitted range, 0..0. For each surviving term element/wildcard/group exercise below/at/above min and finite max; empty groups and repeated nullable models; local/global/abstract/substituting declaration.

Absent occurrences default to 1. 0..0 representation creates no component; a surviving numeric maximum is at least 1 and at least min. Leaf particles constrain item count; group particles constrain number of valid subsequences, which may be empty. Element selection matches expanded name and scope/substitution conditions. Wildcards use the **actual item's** namespace, never a substitution head's namespace. **S/V**.

### S-037 — particle extension

Source: ([`cos-particle-extend`](../upstream/structures.html#cos-particle-extend)).

Same particle; 1..1 sequence whose first member recursively matches B; changed first range/term/type; base in later position; annotation-only difference; outer sequence range different from 1..1.

Identity or the specified base-prefix structure succeeds; similar accepted languages alone do not establish this structural relation. Annotation properties may differ in the required recursive base match. **S**.

### S-038 — restriction dispatch and normalization

Source: ([`cos-particle-restrict`](../upstream/structures.html#cos-particle-restrict)).

Every R/B term-kind pair in the matrix below; identity short circuit; substitution head expansion on R/B/both; empty/singleton/redundant groups with eligible/ineligible outer ranges; nested same-kind sequence/choice.

Normalize **only** the specified pointless cases and expand applicable global substitution heads to choice over their actual substitution groups before selecting a rule. Forbidden dispatches remain forbidden even when a general language-containment argument would succeed. Valid restriction is the specification's structural relation. **S**.

### S-039 — Elt:Elt NameAndTypeOK

Source: ([`rcase-NameAndTypeOK`](../upstream/structures.html#rcase-NameAndTypeOK)).

Equal/different local name/namespace and contained/widened range; both global versus at least one local. For the latter branch vary R/B nillable, fixed constraint, identity-constraint set, block set, and derived type. Include B union with R member, a complex-extension negative, and simple list/union R versus B=anySimpleType.

Names/namespaces and contained occurrence range always apply. When both declarations are global, the rule's alternate global branch applies; do not impose the local-property subclauses there. Otherwise R cannot add nillability, lose a required equal fixed value, add identity constraints beyond B's set, weaken block, or use disallowed type derivation. Supply {extension,list,union} to the appropriate complex/simple predicate. In the simple predicate only restriction is relevant; retain its clause 2.1 gate and list/union-to-anySimpleType alternative (S-062, B-071). Preserve the stated **R identity-constraint subset of B** direction. **S**.

### S-040 — Elt:Any NSCompat

Source: ([`rcase-NSCompat`](../upstream/structures.html#rcase-NSCompat)).

Element namespace allowed/excluded/absent by B namespace constraint; equal/narrower/wider occurrence range; every B processContents value; substitution-expanded head.

Namespace admission and occurrence containment are required. Do not invent a comparison between a declaration and wildcard processContents. Expanded heads use dispatch appropriate to the resulting group. **S**.

### S-041 — Elt:Group RecurseAsIfGroup

Source: ([`rcase-RecurseAsIfGroup`](../upstream/structures.html#rcase-RecurseAsIfGroup)).

B all/choice/sequence with matching element, optional unmatched particles, required unmatched particles, changed order or blocked range.

Wrap R in B's group kind with outer 1..1 and the original R particle as sole member; use Recurse for all/sequence and RecurseLax for choice. The element's own range is not lost during wrapping. **S**.

### S-042 — Any:Any NSSubset

Source: ([`rcase-NSSubset`](../upstream/structures.html#rcase-NSSubset)).

Namespace subset/equal/not subset; contained/widened range; all nine processing-strength pairs; B wildcard is/is not anyType's content wildcard.

R namespaces/range narrow B; R processing is same or stronger (`strict > lax > skip`) except the specific anyType wildcard exception. Equal namespace languages alone do not excuse weakened processing. **S**.

### S-043 — Group:Any NSRecurseCheckCardinality

Source: ([`rcase-NSRecurseCheckCardinality`](../upstream/structures.html#rcase-NSRecurseCheckCardinality)).

R all/choice/sequence; nested member admitted/rejected by B wildcard; group effective minimum/maximum just inside/equal/outside B; unbounded and empty group.

Every member must recursively restrict B **and** effective total range must restrict B's range. A total-count success cannot compensate for one invalid namespace/member relation. A member-by-member success cannot compensate for excessive total range. **S**.

### S-044 — All:All and Sequence:Sequence Recurse

Source: ([`rcase-Recurse`](../upstream/structures.html#rcase-Recurse)).

Equal/narrower/wider outer range; complete ordered mapping; unmapped B optional/required; unmapped R; attempted many-R-to-one-B mapping; reorder R particles; nested recursive successes/failures.

All R particles need the specified order-preserving mapping, recursive restriction, and emptiable unmapped B particles. The all:all relation still preserves the listed order even though all instance matching is unordered. A language subset unsupported by that mapping is not enough. **S**.

### S-045 — Choice:Choice RecurseLax

Source: ([`rcase-RecurseLax`](../upstream/structures.html#rcase-RecurseLax)).

Remove B alternatives; retain matching ordered alternatives; reorder or duplicate mapped alternatives; add unmapped R alternative; widen range; nested branches.

Complete order-preserving mapping from R is required, but unmapped B choices need not be emptiable. Reordering equivalent alternatives can fail this structural rule. **S**.

### S-046 — Sequence:All RecurseUnordered

Source: ([`rcase-RecurseUnordered`](../upstream/structures.html#rcase-RecurseUnordered)).

R selects B members in original/reversed order; B unmatched optional/required; two R particles mapping to one B; R unmapped member; nested restriction; range containment.

The mapping can reorder but must be complete and injective, each mapped particle must restrict, and unmatched B must be emptiable. **S**.

### S-047 — Sequence:Choice MapAndSum

Source: ([`rcase-MapAndSum`](../upstream/structures.html#rcase-MapAndSum)).

R members map in any order, repeatedly to one B alternative, or fail to map; R outer range and particle-list length yield product below/equal/above B bounds; unbounded R; nullable/variable-range member.

All R members need recursive mappings; compare `(R.min × number of R particles, R.max × number of R particles)` to B's range using the exact stated rule. Do not substitute a more permissive computed-language/effective-range test for that deliberately restrictive product. **S**.


All 25 ordered particle-kind combinations have a dispatch obligation:

| R \\ B | Element | Wildcard | all | choice | sequence |
| --- | --- | --- | --- | --- | --- |
| Element | S-039 | S-040 | S-041 | S-041 | S-041 |
| Wildcard | forbidden | S-042 | forbidden | forbidden | forbidden |
| all | forbidden | S-043 | S-044 | forbidden | forbidden |
| choice | forbidden | S-043 | forbidden | S-045 | forbidden |
| sequence | forbidden | S-043 | S-046 | S-047 | S-044 |

The forbidden cells are S-038 negative partitions **after normalization**.
Each allowed cell needs direct and recursive witnesses and an independently
failing cardinality or member branch where that cell has one. `range-ok` uses
R.min ≥ B.min and either B.max=unbounded or finite R.max ≤ finite B.max;
exercise finite/unbounded in both directions and equal/lower/upper bounds.

S-038 pointless-group cases must be expanded literally: empty sequence; sequence
with outer 1..1 and either one member or a sequence parent; empty all or singleton
all; empty choice with outer min=0; choice with outer 1..1 and either one member
or a choice parent. Include the opposing outer-range/context for each case so a
normalization broader than the specification cannot pass unnoticed.

## Wildcard algebra and assessment

Let `A` mean any namespace including absent; `N(x)` mean the wildcard's negation
of x, excluding **both x and absent**; and `S` mean an explicit finite set of
namespace names and/or absent. `N(absent)` therefore means all present namespace
names. A representable empty set is distinct from an absent wildcard.

### S-048 — namespace syntax and admission

Source: ([§3.10.2](../upstream/structures.html#declare-openness), [`src-wildcard`](../upstream/structures.html#src-wildcard), [`w-props-correct`](../upstream/structures.html#w-props-correct), [`cvc-wildcard-namespace`](../upstream/structures.html#cvc-wildcard-namespace)).

namespace absent/##any/##other/list; URI, ##local, ##targetNamespace, duplicate/list order; target namespace present/absent; empty list versus illegal keyword combination; each actual namespace equal/different/absent.

Default namespace constraint is A and processing strict. ##other maps to N(targetNamespace or absent) and never admits unqualified items. ##targetNamespace maps to absent when targetNamespace is absent; ##local maps to absent. Finite set membership uses exact namespace identity. Grammar forbids mixing ##any/##other with list members. **S/V**.

### S-049 — strict/lax/skip

Source: ([`cvc-wildcard`](../upstream/structures.html#cvc-wildcard), [`cvc-assess-elt`](../upstream/structures.html#cvc-assess-elt), [`cvc-assess-attr`](../upstream/structures.html#cvc-assess-attr)).

Element/attribute × namespace admitted/denied × processing strict/lax/skip × declaration available/missing/incomplete × content/value valid/invalid. For elements add usable/invalid/missing xsi:type, declared descendants under undeclared ancestor, and malformed XML even under skip.

Namespace denial always fails. Strict marks mustFind and requires successful strict assessment (a usable xsi:type can supply element type assessment). Lax validates when possible; missing information differs from invalid known content. Skip suppresses schema assessment but never XML well-formedness. Do not use global declarations to override an explicit skip context. **V/P**.

### S-050 — wildcard subset

Source: ([`cos-ns-subset`](../upstream/structures.html#cos-ns-subset)).

All ordered pairs among A, N(x), N(absent), finite S including empty/absent/x/other. Include identical/different negations and set inclusion/equality/disjointness.

Any sub is a subset of A. N(x) is below the identical N(x), not an arbitrary different negation. Finite S is below a containing finite set or N(x) only if it excludes x and absent. Finite empty set is a subset in the stated set branches. **S**.

### S-051 — attribute wildcard union

Source: ([`cos-aw-union`](../upstream/structures.html#cos-aw-union)).

Every row of the union table below, swapped operands, identical operands, empty sets, 3+ inputs, and an unexpressible intermediate. Combine with the processContents provenance of S-023.

Compute the exact intensional result; do not approximate an unexpressible result by A. Namespace union and processContents selection are separate decisions. **S/V**.

### S-052 — attribute wildcard intersection

Source: ([`cos-aw-intersect`](../upstream/structures.html#cos-aw-intersect)).

Every row of the intersection table below, swapped operands, identical operands, disjoint/empty sets, 3+ inputs and an unexpressible intermediate. Test an all-disallowing yet present wildcard.

Compute the exact intensional result; distinct N(namespace) intersections are unexpressible. Follow the specified iterative operation and do not silently replace unexpressibility with empty/any. **S/V**.


| Union operands/condition | Required namespace result |
| --- | --- |
| Same value | That value |
| Either A | A |
| S and T | S ∪ T |
| Different negations | N(absent) |
| N(x), x present namespace, S includes x and absent | A |
| N(x), S includes x but not absent | N(absent) |
| N(x), S includes absent but not x | Unexpressible |
| N(x), S includes neither | N(x) |
| N(absent), S includes absent | A |
| N(absent), S excludes absent | N(absent) |

| Intersection operands/condition | Required namespace result |
| --- | --- |
| Same value | That value |
| A and other | Other |
| N(x) and S | S minus x minus absent |
| S and T | S ∩ T |
| N(x) and N(y), distinct present namespace names | Unexpressible |
| N(x), x present, and N(absent) | N(x) |

## Identity constraints, annotations, notations, and simple-type interface

### S-053 — identity representation and XPath grammar

Source: ([`src-identity-constraint`](../upstream/structures.html#src-identity-constraint), [`c-props-correct`](../upstream/structures.html#c-props-correct), [`c-selector-xpath`](../upstream/structures.html#c-selector-xpath), [`c-fields-xpaths`](../upstream/structures.html#c-fields-xpaths)).

key/unique/keyref; refer missing/present and target key/unique/keyref/missing; equal/different ordered field count. Selector/field path: `.`, child QName, `*`, `p:*`, slash steps, optional leading `.//`, union alternatives, whitespace around tokens, bound/unbound prefixes; final attribute step in field; attribute selector, middle attribute step, predicates, functions, parent/ancestor axes, absolute paths, `//` elsewhere, malformed tokens.

Selector and field are the restricted XPath 1.0 subsets, not arbitrary XPath and not XML QName resolution with a default element namespace. Name tests obey XPath 1.0 namespace rules, including unprefixed element tests selecting no-namespace names. Keyref references key/unique with equal arity; key/unique do not carry referenced key. Tokenization takes the longest token, with whitespace only at allowed token boundaries. **S/V**.

Component syntax and evaluation are conditional on XML representation admission;
preserve [XPS-Q001](source-questions.md#xps-q001--xpath-component-syntax-and-xml-representation)
for internal whitespace and equivalent axis spellings. The intended keyref
target is an explicit premise under [IDS-Q001](source-questions.md#ids-q001--identity-constraint-refer-and-generic-qname-resolution).

### S-054 — selector and field evaluation

Source: ([`cvc-identity-constraint`](../upstream/structures.html#cvc-identity-constraint)).

No/one/multiple selector matches; context node selected by `.`; descendant scope; overlapping union branches selecting the same node; field selects zero/one/multiple nodes; attribute versus simple-content element versus complex-content element; field order and duplicate field expressions; defaulted value, nilled node, and failed assessment.

XPath node sets deduplicate nodes. Each field yields zero or one node with a simple value; multiple matches or nonsimple values fail the shared rule. Ordered fields form a typed tuple from schema normalized values, including defaults. A qualified tuple requires every field to yield one suitable node. Missing tuples and invalid multiple-node fields are different branches. **V/P**.

For anySimpleType items whose value mapping is implicated, apply
[D-004's explicit unspecified boundary](datatypes.md#d-004--component-identity-and-the-simple-ur-type)
before inferring typed tuple equality or an identity-check result. This does
not waive independent selector, field-cardinality or schema-admission rules.

For complex simple-content fields, nilled values, and fields under skipped
subtrees, preserve the [W3C interpretation history](../INTERPRETATIONS.md).
The published 1.0 text, documented WG clarification, and later 1.1 changes
are separate oracle classifications.

### S-055 — unique and key

Source: ([`cvc-identity-constraint`](../upstream/structures.html#cvc-identity-constraint)).

Keep the [introductory type-wording qualification](../corrected-reading.md#identity-constraint-type-wording) separate from the formal typed-value comparison; the stipulated sibling-integer case has no extra declared-type ancestry veto.

Empty/one/two selected rows; complete/incomplete tuples; duplicate full tuple, duplicate one field only, lexical aliases of same typed value, distinct typed values with same spelling, cross-derived comparable types, list/union field values. For key, element field declared nillable true/false with nil absent/false/true.

unique compares only qualified rows; key requires all selected rows qualified. Equal full typed tuples fail uniqueness; lexical difference alone does not distinguish equal values. For key, an element field assessed through a declaration whose nillable property is true is forbidden **even when the instance is not nilled**. Use Part 2 equality, not untyped string equality or application collation. **V**.

### S-056 — keyref

Source: ([`cvc-identity-constraint`](../upstream/structures.html#cvc-identity-constraint)).

Qualified/incomplete reference tuple; matching/nonmatching target; forward/backward order; key and unique target; multiple fields with swapped order; zero matching target table; target in own subtree, descendant scope, sibling scope, ancestor-only scope, and repeated nested instances of the declaring element.

Each qualified reference needs an equal tuple in the referenced constraint's node table at the constraint element. Incomplete reference tuples are not checked like key rows. Declaration order does not create a forward-reference restriction. Visibility follows recursively assembled subtree tables, not a document-global registry or arbitrary ancestor lookup. **V/P**.

These rules assume the intended referenced component; retain
[IDS-Q001](source-questions.md#ids-q001--identity-constraint-refer-and-generic-qname-resolution)
for the generic QName-resolution integration step.

### S-057 — identity tables and conflicts

Source: ([`sic-key`](../upstream/structures.html#sic-key)).

Constraint eligible locally/from children/both; same tuple same node; same tuple distinct nodes across child tables; local tuple conflicting with propagated tuple; conflicts only among inherited entries; valid/invalid unique/key.

First determine eligibility: the appropriate unique/key clause 4.1/4.2 holds for this item and definition, or an element child has a table entry for the same definition. This contributor supplies one binding per eligible definition; that binding may have an empty node table. Do not conflate no eligible definition with an eligible empty binding. Build its node table from qualifying local rows and recursively propagated child tables. Conflicting entries owing their inclusion to child propagation are excluded; if all conflicting entries are inherited, none of them survives. One local entry can therefore survive against inherited conflicts. Cross own-key failure with child-entry eligibility as in C-012. Table exposure is optional, but keyref behavior must be as if the prescribed tables existed. **V/P/O**.

### S-058 — notation

Source: ([§3.12](../upstream/structures.html#cNotation_Declarations), [`sic-notation-used`](../upstream/structures.html#sic-notation-used)).

Name/namespace/public/system mappings; public-only, system-only, both, or neither in the XML representation and abstract component; NOTATION value with matching/missing declaration, prefix context, enumeration restriction and invalid lexical form. One versus multiple NOTATION attributes.

Notation is a named component requiring public or system. The published 2004 representation summary and Appendix A make both attributes individually optional: system-only is admitted; neither passes that syntax but fails the component requirement. No direct independent instance validation rule is added by §3.12. Its use is constrained through Part 2 NOTATION and QName lookup. On a valid NOTATION attribute the parent receives the notation component or system/public pair. With multiple such attributes, the supplying attribute is unspecified; the compatibility recommendation is not a mandatory schema rejection. **S/V/P/O**.

### S-059 — annotations

Source: ([§3.13](../upstream/structures.html#cAnnotations), [`src-annotation`](../upstream/structures.html#src-annotation), [`an-props-correct`](../upstream/structures.html#an-props-correct)).

No/empty/multiple appinfo/documentation; alternating order; arbitrary nested markup/text/foreign attributes; source/xml:lang; annotation at each permitted source location including selector/field/derivation; foreign attributes on annotation and enclosing owner, including no explicit annotation.

Preserve ordered application/user information sequences and admitted attributes through the specified component mapping. Annotation information does not itself impose instance validity constraints. A source URI is supplementary information and is not dereferenced during validation. Annotation grammar is still checked; application markup inside its open content is not schema component syntax. Test multi-location annotation collection on complex types and identity constraints separately from a single annotation. **S/P**.

The complex-type zero-annotation endpoint retains
[CTS-Q001](source-questions.md#cts-q001--complex-type-annotations-with-no-source-annotation).
For documentation `xml:lang`, preserve the dated language rules and imported-body
version premise in [ANQ001](source-questions.md#anq001--annotation-xmllang-and-the-imported-source-version).

### S-060 — simple-type representation and normalization

Source: ([§3.14.2](../upstream/structures.html#declare-datatype), [`src-simple-type`](../upstream/structures.html#src-simple-type), [Part 2](../upstream/datatypes.html)).

Atomic restriction/list/union; base vs inline and itemType vs inline each neither/one/both; memberTypes and inline union members alone/together/empty; nested unions, duplicate members, order; direct/indirect base/item/member cycles; named/global versus anonymous/local.

Choose exactly one restriction base or list item source. Explicit union members are referenced members followed by inline members; flatten nested unions in order. Nonempty valid membership and cycle constraints apply. §3.14's non-normative repeated descriptions do not supersede Part 2; its explicitly normative integration rules remain obligations. **S/V/P**.

### S-061 — simple restriction and facet composition

Source: ([`st-props-correct`](../upstream/structures.html#st-props-correct), [`cos-st-restricts`](../upstream/structures.html#cos-st-restricts), [`st-restrict-facets`](../upstream/structures.html#st-restrict-facets)).

Atomic/list/union B/R; permitted/forbidden facets and base final; atomic primitive retained; list item atomic/union-of-atomic/list/union-containing-list; item final list; union members atomic/list/unflattened union and final union; corresponding ordered members under restriction; inherited versus new scalar/pattern/enumeration facets.

Keep variety and atomic primitive identity; apply allowed facet sets and valid narrowing. New list/union construction and restriction of existing list/union have different permitted facets and final checks. Scalar facets replace same-kind inherited facets as specified; pattern/enumeration composition retains the relevant distinct values under Part 2 semantics. Lists cannot acquire list-valued items. User atomic restriction directly from anySimpleType is not admitted. **S/V**.

### S-062 — simple derivation relation

Source: ([`cos-st-derived-ok`](../upstream/structures.html#cos-st-derived-ok)).

D=B; direct/recursive base; list/union versus anySimpleType; D derives from a member of union B; same lexical/value spaces but unrelated types; prohibited restriction at own/intermediate step; transition from complex simple content through simple relation.

Follow exactly the identity and alternative derivation paths, including derivation from a union member. Matching accepted values alone does not establish derivation. Apply the supplied blocking subset and base final conditions as written; do not conflate list/union construction methods with restriction-chain checks. **S/V**.

### S-063 — string validity and entity context

Source: ([`cvc-simple-type`](../upstream/structures.html#cvc-simple-type), [§3.1.4](../upstream/structures.html#key-nv), [D](../upstream/structures.html#infoset)).

Atomic/list/union with whitespace preserve/replace/collapse and XML attribute normalization; literal versus character-reference tab/newline, with/without DTD tokenized attribute normalization; element character/CDATA/entity-boundary concatenation, comments and PIs between text chunks. ENTITY/ENTITIES and restrictions thereof with unparsed entity declared/not declared, parsed entity only, multiple list tokens, or no unparsedEntities Infoset property.

Apply Part 2 lexical/value/facet validity after the specified normalization; text chunk boundaries do not change the element value. ENTITY requires a matching **unparsed** entity declaration; ENTITIES requires all tokens to match. Missing unparsedEntities support makes all such values fail; it does not make the extra constraint optional. **V/P/O**.


## Composition, assessment, and processor responsibilities

### S-064 — include and chameleon adoption

Source: ([`src-include`](../upstream/structures.html#src-include), [§4.2.1](../upstream/structures.html#compound-schema)).

Same target namespaces; both absent; included absent into present; included present into absent; different present namespaces. Resolve to valid schema, wrong root, invalid schema, or unavailable location; repeated/diamond/cyclic includes; forward references through included documents. Adopt one no-namespace document into two namespaces and examine nested qualified/unqualified declarations, wildcard constraints and unresolved reference QNames.

Same namespace and chameleon cases assemble components; mismatches fail. No resolution is not itself an include error; a resolved invalid referent is. Chameleon replaces absent target namespaces in the specified named/nested qualified components, wildcard constraints and pending reference QNames; it does not qualify deliberately unqualified local declarations. Same source can contribute separate adopted contexts; repeated same components are not duplicate definitions. **S/V/O**.

### S-065 — redefine

Source: ([`src-redefine`](../upstream/structures.html#src-redefine), [`src-expredef`](../upstream/structures.html#src-expredef), [§4.2.2](../upstream/structures.html#modify-schema)).

Empty/annotation-only versus substantive redefine with unavailable location; same/chameleon/mismatched namespace. Types: self base versus another/missing base, extension/restriction, original missing. Model group: zero/one/two self refs; self ref range 1..1 or other; no self ref with valid/invalid restriction. Attribute group: zero/one/two self refs; no self ref with required/fixed/wildcard restriction. Nested/multiple redefines, references from unredefined components and derivations that become invalid after replacement.

Substantive redefine requires successful resolution. Redefined types create unnamed original plus newly named type whose self-base resolves to that original. Group self refs refer to original group; no-self cases must restrict the original. Restrictive attribute-group redefinition has no implicit inheritance from the original group; retain [CMP-Q002](source-questions.md#cmp-q002--restrictive-attribute-group-redefine-and-nonself-references) for its nonself-reference mapping question. Replacement is pervasive for other component references; ordinary copied aliases or document-local overrides are insufficient. Revalidate affected components. **S/V**.

Compare repeated equivalent redefinitions of the same document in the same way
under the same adopted namespace with inequivalent replacements and distinct
component identities. Equivalent repetition does not itself violate component
uniqueness, but individual redefinition identity still needs comparison. The
source permits avoiding repeated work; it imposes no interning or cache API.
Use [IP-014's repeat product](interactions.md#ip-014--redefinition--original-identity--ancestry--chameleon-references)
rather than inferring equivalence from a repeated URI alone.

### S-066 — import

Source: ([`src-import`](../upstream/structures.html#src-import), [§4.2.3](../upstream/structures.html#composition-schemaImport)).

Import different namespace, same namespace, namespace absent from present/absent parent; schemaLocation absent/present; target referent matching/mismatching/absent namespace; location unavailable/invalid/valid; multiple documents for one namespace, duplicate identical component, contradictory definitions; namespace permission versus actual component availability.

Cannot import the enclosing target namespace; namespace-absent import requires an enclosing target namespace and imports no-namespace components. A referent, if supplied, must match the requested namespace and be a valid schema. Failure to find it is not itself an import error. Components combine under identity/uniqueness rules; location is a hint and processing every hint is not mandated. Import permission alone does not manufacture a component. **S/V/O**.

An unused present-empty `namespace=""` with no referent is a separate semantic
scope question, [CMP-Q001](source-questions.md#cmp-q001--unused-present-empty-import-namespace).
Do not collapse it into absent namespace or infer its outcome from anyURI syntax.

### S-067 — document locations and Web conventions

Source: ([§2.7](../upstream/structures.html#web-representation), [§4.3](../upstream/structures.html#composition-instances), [`schema_reference`](../upstream/structures.html#schema_reference)).

schemaLocation pair list empty/even/odd; URI lexicals, XML base/relative reference/fragment, schema root versus embedded schema; hint for current/another/no namespace; hints before/after first use, nested hint with global effect; multiple hints, ignored/used/overridden hints; local repository/namespace dereference/location dereference/no lookup.

Distinguish the built-in anyURI-list datatype from namespace/location pairing semantics. Hints do not redefine target namespaces or element names. Layer-3 general-purpose Web processors have their stated defaults/override duties; schema acquisition choices are processor/application dependent. When using dynamic acquisition, declarations never change after establishment and results must match bulk acquisition of the same components. Late addressed hints have the specified error condition. Explicit no-network operation is a permitted lower-layer policy. **S/V/O**.

Within the applicable Web schema-location conventions, absent contrary
application/command-line direction, processors **should** attempt each hinted
schema-document location URI. This recommendation differs from namespace-URI
dereference defaults and supplies no mandatory network request count. Timing
is relative to the initially validated element: a hint strictly after the
first addressed-namespace **element or attribute** within that root is an error.
Use in another namespace or outside the chosen root does not establish that
premise. Successfully acquired definitions remain available beyond the hint
element's lexical scope. Expand [IP-018's hint subproduct](interactions.md#ip-018--invocationconformance--acquisition--root--optional-observations)
with actual source admission, ordering and observation premises.

### S-068 — assessment and outcome lattice

Source: ([`cvc-assess-elt`](../upstream/structures.html#cvc-assess-elt), [`sic-e-outcome`](../upstream/structures.html#sic-e-outcome), [`sic-a-outcome`](../upstream/structures.html#sic-a-outcome), [§5.2](../upstream/structures.html#validation_outcome)).

Invoke with stipulated complex type, stipulated matching/mismatching declaration, or no stipulation. Cross strict/lax/skip with local valid/invalid/unassessable and child/attribute validity valid/invalid/notKnown, validation attempted full/partial/none, and mustFind versus non-mustFind. Include a locally invalid parent with otherwise assessable children.

Validity is valid/invalid/notKnown and attempted is full/partial/none, not one boolean. Strict success also depends on assessed children/attributes and mustFind requirements. A lax parent can have assessed descendants without becoming strictly assessed itself. Attempted becomes partial in the mixed cases specified by the outcome table. Attributes have full/none, not the element partial case. Successful-outcome policy belongs to the application. **V/P/O**.

### S-069 — declaration/type/schema PSVI

Source: ([`sic-elt-decl`](../upstream/structures.html#sic-elt-decl), [`sic-eltType`](../upstream/structures.html#sic-eltType), [`sic-attr-decl`](../upstream/structures.html#sic-attr-decl), [`sic-schema`](../upstream/structures.html#sic-schema), [E1-56](../upstream/errata-second-edition.html#e1-56)).

Valid/invalid/nilled/defaulted/lax values; named/anonymous type and union member; full component versus lightweight identity projection; no/one/multiple namespaces including absent; schema components/documents exposed/omitted; imported/chameleon documents; nested validation root.

Emit the required property set for the applicable case and chosen representation; do not demand optional declaration/table exposure. E1-56 gates union metadata on available valid normalized values; mixed/nilled content has no simple normalized value. Lax element fallback uses anyType; unassessed attribute fallback uses anySimpleType/initial value. Root schema-information groups namespaces, can leave component sets empty, but if component exposure is provided it includes all named components actually involved directly/indirectly. Validation context and document URI/Infoset absence follow their own rules. **P/O**.

### S-070 — ID/IDREF(S)

Source: ([`cvc-id`](../upstream/structures.html#cvc-id), [`sic-id`](../upstream/structures.html#sic-id)).

Attribute/element ID, IDREF, IDREFS and derived types; zero/one/multiple bindings; forward/backward/missing reference; repeated reference token; duplicate IDs on distinct binding elements; eligible success versus invalid/skipped/unassessed values; validation subtree versus full document; reuse in separate assessment episodes.

Build bindings from the items that satisfy the literal successful-declaration-validation
and same-context eligibility conditions. Both ID and IDREF values can introduce
keys. Root validity checks every empty binding and every binding with multiple
elements separately; an empty binding need not originate from an IDREF. Tokens
compare by their type-normalized strings. The binding's element set uses ID
attribute owners and parents of ID-valued child elements, rather than a count
of value occurrences. Include same-parent versus different-parent carriers and
the direct ID-valued validation-root setup in
[IBS-Q001](source-questions.md#ibs-q001--root-id-eligibility-and-successful-declaration-validation),
whose success/contribution dependency remains unresolved. Validation-root and
same-context premises do not define a complete repeated-session contract;
retain S-073's episode boundary. Table exposure is optional. Keep S-019's
declared/wildcard ID-per-type checks separate from root bindings. **V/P/O**.

### S-071 — missing components

Source: ([§5.3](../upstream/structures.html#conformance-missing), [§4.1](../upstream/structures.html#layer1)).

Missing declaration, base type, member/item type, group, attribute declaration, or referenced key; missing directly versus through a property/set at depth; missing needed versus unused component; QName discharged before actual need versus never discharged; strict/lax/skip context.

Missing mandatory subcomponents make the validating component unusable; attribute behaves as missing declaration, element as missing declaration with permitted continued lax assessment. Assessment affected by missing components cannot become full. Retain unresolved QName information for later discharge during construction; do not reinterpret absence as an empty valid component. Acquisition failure and a structurally invalid schema are different conditions. **S/V/P/O**.

### S-072 — schema errors and validation diagnostics

Source: ([§5.1](../upstream/structures.html#conformance-schemaValidity), [`sic-attr-error-code`](../upstream/structures.html#sic-attr-error-code), [`sic-elt-error-code`](../upstream/structures.html#sic-elt-error-code)).

Schema-for-schemas failure, representation-constraint failure, and component-constraint failure; multiple errors in each; valid schema with invalid instance; invalid local value with otherwise valid type versus type-invalid value; processor stop/continue behavior after detection.

For the schema-XML condition, distinguish the would-be schema-for-schemas
results `(valid, full)` and `(valid, partial)`: §5.1 admits both. An invalid
result does not satisfy that condition. An absent or unobserved assessment
does not establish the would-be result, and does not by itself prove a schema
error. No separate schema-for-schemas validation pass is prescribed. Passing
this condition does not waive representation or component constraints. The
concrete boundary is [C-081](branches/composition-and-identity.md#c-081--schema-errors-are-not-instance-invalid-witnesses)
and the dependent product is [IP-018](interactions.md#ip-018--invocationconformance--acquisition--root--optional-observations).

The three schema-error classes are distinguished; a schema-like object failing schema requirements cannot be used for coherent assessment. Error recovery/order after schema construction error is not prescribed. For locally assessed invalid instances, the schema-error-code property is a list; recording particular reason codes is encouraged, not universally required. For valid local assessment the property is absent. Do not require diagnostic wording/order unsupported by the standard. **S/V/P/O**.

### S-073 — processor state and options

Source: ([§§2.4](../upstream/structures.html#concepts-conformance), [4.1](../upstream/structures.html#layer1), [5.2–5.4](../upstream/structures.html#conformance-processorResponsibilities)).

Precompiled/bulk versus permitted lazy acquisition of identical components; component already established versus later hint/redefinition; type/declaration/root invocation choices; lightweight/full PSVI; optional exposure; repeated assessment of original Infoset versus prior PSVI; user-directed location overrides.

Same final component schema gives the same assessment despite permitted assembly timing; established declarations cannot mutate mid-assessment. Choice of validation root and supported invocation modes is outside the universal core predicate. Multiple assessment episodes have no complete normative contract here; do not invent required preservation/overwriting rules. Record exact supported conformance layer and processor choices before using fixtures as a universal oracle. **V/P/O**.


## Appendices and built-ins

### S-074 — Appendix A, normative schema for schemas

Source: ([A](../upstream/structures.html#normative-schemaSchema), [dated schema text](../upstream/structures.xsd.txt)).

Apply every schema grammar context in the checklist; test utility simple types, occurrence values, form/derivation controls, wildcard token lists, selector/field regex-shaped admission, schema-document identity constraints and id uniqueness. Pair cases accepted by Appendix A but forbidden by prose, and cases accepted by the non-normative DTD but forbidden by A.

Appendix A is normative only for the syntactic constraints its schema expresses, with Part 2's included schema vocabulary. It cannot prove semantic constraints it does not express. Prose plus A is the oracle; do not use a later live namespace schema as a substitute for the dated edition. Documentation inside the schema is explanatory, not a separate normative layer. **S**.

### S-075 — Appendix B, normative dependencies

Source: ([B](../upstream/structures.html#normative-references)).

Map each cited dependency to the relevant semantic seam: XML 1.0/Infoset, namespaces/QName, XPath 1.0, Part 2 datatypes, URI references/XML base/XPointer where invoked. Use a cross-boundary witness for each invoked rule rather than treating the bibliography as a list of unconditional features.

A reference's cited scope controls the requirement; do not import unrelated features, later editions, full XPath capability, or XSD 1.1. Dependency-level unresolved facts must stay visible in the dependency catalogue/source audit. **S/V/P/O**.

### S-076 — Appendix C, normative outcome tabulations

Source: ([C](../upstream/structures.html#outcomes)).

For every C.1/C.3/C.4 rule link, map to its family and each numbered condition. For C.2, distinguish required, conditional, alternate, optional and explicitly absent PSVI properties; E1-56 overlay; error lists versus codes.

Reports of numbered constraint violations should use the table's name plus the
applicable part number, separated by periods. Distinguish a whole-constraint
name from a numbered or nested-part report: `cos-ct-extends.1.2` identifies
clause 1.2 of Derivation Valid (Extension). This recommendation does not require
an API exposing codes, exhaustive reporting, or particular message wording or
order; S-072 retains the separate encouragement to record reason codes.

Appendix C indexes obligations rather than adding a new acceptance algorithm. Every named rule and every PSVI contribution must have an owner below; indexed presence alone does not prove branch witnesses. Corrected clause reading prevails over uncorrected outcome wording. **S/V/P**.

### S-077 — Appendix D, normative Infoset preconditions

Source: ([D](../upstream/structures.html#infoset)).

Missing/supplied attribute local name/namespace/normalized value; character code; element children/attributes/name/namespace/in-scope namespaces or reconstructible namespace attributes; namespace prefix/name; unparsedEntities present/absent. Compare input and output child/attribute information sets.

Required Infoset support is a precondition, not an arbitrary instance invalidity code. Namespace context may be reconstructed as allowed. UnparsedEntities is a recommended property with the specified ENTITY validity consequence. PSVI contributions are additive; defaulting does not destructively rewrite input children. **V/P/O**.

### S-078 — Appendices E–J, non-normative material

Source: ([E](../upstream/structures.html#component-diagram), [F](../upstream/structures.html#normative-glossary), [G](../upstream/structures.html#nonnormative-schemaDTD), [H](../upstream/structures.html#non-ambig), [I](../upstream/structures.html#nonnormative-references), [J](../upstream/structures.html#acknowledgments)).

E: verify relationships against the 13-component inventory; F: link vocabulary back to its normative definition; G: schema documents with optional XML/DOCTYPE declarations, DTD normalization/defaulting and notation/entity declarations; H: all four particle-overlap shapes and nullable/repetition adjacency; I: tutorial/historical proposals; J: attribution.

E diagram and F glossary aid navigation without new rejection rules. G's DTD is illustrative and cannot override A/prose (notably context restrictions or open annotation content). H explicitly disclaims completeness; S-034 remains normative. I references and J acknowledgements add no conformance conditions or fixture requirements. Preserve provenance, but do not invent behavioral tests for names or historical proposals. **S/V/O** where connected to normative rules.

### S-079 — substitution affiliation and actual group

Source: ([`e-props-correct`](../upstream/structures.html#e-props-correct), [`cos-equiv-derived-ok-rec`](../upstream/structures.html#cos-equiv-derived-ok-rec), [`cos-equiv-class`](../upstream/structures.html#cos-equiv-class)).

Global/local member/head; omitted/explicit head-derived type; direct/transitive affiliation; self/indirect affiliation cycle; same/unrelated type; head final restriction/extension; member/head abstract; head block substitution/extension/restriction; intermediate type block; same-type and mixed-method ancestry; members in other namespaces.

Only globals affiliate and affiliation cannot cycle. Member type derives from head given head final, independently of whether an instance substitution is blocked. Potential group closes transitively over affiliation and includes head; actual group filters abstract members and invalid substitutions. Identity substitution is distinct from replacing a head with another declaration. Intermediate type block contributes to substitution validity. Local element particles do not gain global substitution behavior from a same-name global. Use the actual group in UPA/restriction expansion and actual member namespace for wildcard assessment. **S/V**.

### S-080 — built-in schema components

Source: ([§3.2.7](../upstream/structures.html#cAttribute_Declarations), [anyType](../upstream/structures.html#ur-type-itself), [anySimpleType](../upstream/structures.html#simple-ur-type-itself), [Part 2](../upstream/datatypes.html)).

Retain the [printed primitive-summary qualification](../corrected-reading.md#printed-primitive-type-summary) when reading §3.14.7.

Refer to all built-in primitive/derived simple types, anySimpleType and anyType without importing a network resource. Four xsi built-ins: type QName, nil boolean, schemaLocation anyURI list, noNamespaceSchemaLocation anyURI; malformed/valid values. AnyType arbitrary mixed content/attributes with known/unknown declarations; anySimpleType arbitrary legal XML character values.

Built-ins exist by definition; user syntax need not retrieve the XSD namespace URI. anyType has mixed lax element and attribute wildcards, not skip, so known declarations can make descendants/attributes invalid. anyType's self-base and anySimpleType's bridge to anyType are specified exceptions, not user-cycle precedents. User xsi declarations remain forbidden. **S/V/P**.

The four [builtin attribute tables](../upstream/structures.html#xsi.type)
give global declarations in `http://www.w3.org/2001/XMLSchema-instance`, each
with absent value constraint and annotation. Their names are `type`, `nil`,
`schemaLocation` and `noNamespaceSchemaLocation`; types are respectively
builtin QName, boolean, the particular anonymous type below, and builtin anyURI.

The `schemaLocation` type has absent name, XSI target namespace, simple ur-type
base, list variety, builtin anyURI item type, and printed absent facets and
annotation. Retain those table facts without silently equating absent with an
empty set or replacing this component by named anyURI. For an actually assessed
attribute where the chosen lightweight type contribution applies, it reports
simple/XSI/anonymous=true; a type name is absent unless the processor supplies
a unique identifier. Contrast `noNamespaceSchemaLocation`'s named anyURI type
in the XSD namespace. The full alternative observes an isomorphic component
with the stated properties. Cross all four builtins with actual assessment
and full/lightweight representation; metadata availability alone is not
assessment. No generated anonymous spelling, duplicate full-and-light output,
network lookup or new inspection API is required. List typing remains
separate from hint pairing, timing and acquisition policy.


## Required interactions

These products are part of the catalogue, not optional pairwise sampling.
For each row enumerate the **reachable full product** of listed partitions,
then remove a combination only with a documented reason it is impossible or
has the same rule path and observable result as a retained combination.
One-family witnesses remain necessary to isolate failures. For large numeric or
recursive domains, use the family's finite boundary/shape classes; no fixed
small depth proves the general recursive algorithm.

| Product | Dimensions and expected distinguishing effects |
| --- | --- |
| Names and composition | S-004–S-007 × S-064–S-066: component kind, namespace permission, prefix/default binding, scope, normal/chameleon context, repeated source identity. Same-source reuse must not collapse different adopted namespaces; availability does not bypass import permission. |
| Defaults, type, nil and PSVI | S-012/S-017/S-020 × S-015/S-016 × S-018 × S-060–S-062: lexical/value validity, actual versus declared type, empty versus whitespace content, union member, value constraint, nil, simple/mixed category. Check validity and corrected normalized/default/schema-specified/member properties independently. |
| Attributes and derivation | S-009–S-011 × S-023/S-025/S-026 × S-029: local/reference/group/inherited source, required/prohibited/default/fixed, matching explicit name, new wildcard-permitted name, ID-derived type. Explicit use priority and required inherited use must survive assembly. |
| Wildcard algebra and assessment | S-023 × S-048–S-052 × S-019/S-049: namespace algebra operand class, processing provenance/strength, referent availability, known-invalid content, type/attribute ID rules. An equal namespace set does not imply equal validity outcomes under different processing. |
| Substitution, model consistency and UPA | S-079 × S-033/S-034/S-036/S-038: direct/transitive affiliation, abstract filtering, type derivation/block, same-name type identity, actual member namespace, repeated/nullable model position. Check both schemas where membership creates ambiguity and opposing blocked/abstract cases where it does not. |
| Particle restriction recursion | S-035/S-038 × each S-039–S-047 applicable cell × S-079: term-kind normalization, inherited ranges, nested mapping, substitution expansion and emptiability. Put a failure at root, immediate child and deeper descendant; isolate member validity from total-cardinality validity. |
| Extension/restriction ancestry | S-025–S-028 × S-062 × S-065: methods at each ancestry step, final/block at base/intermediate, attributes/models removed/restored, original/redefined component identity. Immediate-base success must not conceal invalid full ancestry or pervasive redefine effects. |
| Identity values | S-053–S-057 × S-012/S-017/S-020/S-060–S-063: XPath namespace tests, field count/order, value versus lexical equality, defaulting, union/list types, nillable declaration. The expected key sequence comes from independently derived typed values. |
| Identity scope and propagation | S-055–S-057 × S-068/S-071: local/child/ancestor/sibling/nested constraint domains, missing or invalid component/value, inherited table conflict, forward reference. Do not replace the prescribed subtree table with global string membership. |
| ID context | S-019 × S-063/S-070 × S-049/S-068: explicit/wildcard ID declaration, optional declared ID absent, skipped versus validly assessed value, multiple IDs, subtree root. Root ID/IDREF rules and per-type ID restrictions must both hold. |
| Missing information and outcomes | S-006/S-049/S-071 × S-068/S-069: each missing component kind, skip/lax/strict, type/declaration-based assessment, known-invalid descendant and mustFind. Distinguish no assessment from completed invalid assessment. |
| Processor policy versus core | S-001/S-067/S-073 × S-068/S-069/S-077: conformance layer, root/invocation mode, schema acquisition strategy, optional exposure and required Infoset support. State policy preconditions before judging the observable result. |

## Named-constraint coverage index

This index covers all **95** named `constraintnote` entries in the second-edition
Part 1 source: every `cvc`, `src`, `cos` and `sic`, including constraint IDs whose
spelling lacks that prefix. It identifies owners; it does not claim that fixtures
already exist or that a broad family proves every numbered subcondition. The
main source inventory also indexes numbered branches in property mappings and
other prose outside these named constraints; use that inventory when assigning
individual fixture IDs.

| Constraint ID | Scenario families |
| --- | --- |
| `src-attribute` | S-009, S-010 |
| `cvc-attribute` | S-011, S-012 |
| `cvc-assess-attr` | S-011, S-049, S-068, S-071 |
| `sic-a-outcome` | S-012, S-068 |
| `sic-attr-error-code` | S-072 |
| `sic-attr-decl` | S-069 |
| `sic-attrType` | S-012, S-069 |
| `a-props-correct` | S-008, S-010 |
| `no-xmlns` | S-010 |
| `no-xsi` | S-010, S-080 |
| `src-element` | S-013 |
| `cvc-elt` | S-014, S-015, S-016, S-017, S-055, S-056, S-070 |
| `cvc-type` | S-014, S-018 |
| `cvc-id` | S-070 |
| `cvc-assess-elt` | S-015, S-049, S-068, S-071 |
| `sic-e-outcome` | S-068 |
| `sic-elt-error-code` | S-072 |
| `sic-elt-decl` | S-016, S-069 |
| `sic-eltType` | S-017, S-069 |
| `sic-eltDefault` | S-017 |
| `e-props-correct` | S-008, S-017, S-079 |
| `cos-valid-default` | S-017 |
| `cos-equiv-derived-ok-rec` | S-079 |
| `cos-equiv-class` | S-079 |
| `src-ct` | S-021, S-022, S-023 |
| `cvc-complex-type` | S-018, S-019 |
| `sic-attrDefault` | S-020 |
| `ct-props-correct` | S-008, S-024 |
| `cos-ct-extends` | S-025 |
| `derivation-ok-restriction` | S-026, S-027 |
| `cos-ct-derived-ok` | S-015, S-028, S-079 |
| `cvc-au` | S-011 |
| `au-props-correct` | S-008, S-010 |
| `src-attribute_group` | S-023, S-029 |
| `ag-props-correct` | S-008, S-029 |
| `src-model_group_defn` | S-030 |
| `mgd-props-correct` | S-008, S-030 |
| `src-model_group` | S-030, S-032 |
| `cvc-model-group` | S-031 |
| `mg-props-correct` | S-008, S-030 |
| `cos-all-limited` | S-032 |
| `cos-element-consistent` | S-033 |
| `cos-nonambig` | S-034 |
| `cos-seq-range` | S-035, S-043 |
| `cos-choice-range` | S-035, S-043 |
| `cvc-particle` | S-031, S-036, S-049, S-079 |
| `p-props-correct` | S-008, S-036 |
| `cos-particle-extend` | S-025, S-037 |
| `cos-particle-restrict` | S-027, S-038, S-039, S-040, S-041, S-042, S-043, S-044, S-045, S-046, S-047 |
| `range-ok` | S-035, S-038, S-039, S-040, S-041, S-042, S-043, S-044, S-045, S-046, S-047 |
| `rcase-NameAndTypeOK` | S-039 |
| `rcase-NSCompat` | S-040 |
| `rcase-RecurseAsIfGroup` | S-041 |
| `rcase-NSSubset` | S-042 |
| `rcase-NSRecurseCheckCardinality` | S-043 |
| `rcase-Recurse` | S-044 |
| `rcase-RecurseLax` | S-045 |
| `rcase-RecurseUnordered` | S-046 |
| `rcase-MapAndSum` | S-047 |
| `cos-group-emptiable` | S-017, S-027, S-035, S-044, S-046 |
| `src-wildcard` | S-048 |
| `cvc-wildcard` | S-049 |
| `cvc-wildcard-namespace` | S-040, S-048 |
| `w-props-correct` | S-008, S-048 |
| `cos-ns-subset` | S-025, S-026, S-042, S-050 |
| `cos-aw-union` | S-023, S-051 |
| `cos-aw-intersect` | S-023, S-052 |
| `src-identity-constraint` | S-053 |
| `cvc-identity-constraint` | S-054, S-055, S-056 |
| `sic-key` | S-057 |
| `c-props-correct` | S-008, S-053 |
| `c-selector-xpath` | S-053 |
| `c-fields-xpaths` | S-053 |
| `src-notation` | S-058 |
| `sic-notation-used` | S-058 |
| `n-props-correct` | S-008, S-058 |
| `src-annotation` | S-059 |
| `an-props-correct` | S-008, S-059 |
| `src-simple-type` | S-060 |
| `cvc-simple-type` | S-063 |
| `st-props-correct` | S-008, S-061 |
| `cos-st-restricts` | S-061 |
| `cos-st-derived-ok` | S-062 |
| `st-restrict-facets` | S-061 |
| `src-qname` | S-005 |
| `src-resolve` | S-006 |
| `cvc-resolve-instance` | S-006, S-015 |
| `sic-schema` | S-069 |
| `sic-id` | S-070 |
| `sch-props-correct` | S-004, S-008 |
| `src-include` | S-064 |
| `src-redefine` | S-065 |
| `src-expredef` | S-065 |
| `src-import` | S-066 |
| `schema_reference` | S-067, S-073 |

## Questions and unproven coverage

These are oracle boundaries to retain during later fixture design, not reasons
to reject otherwise valid inputs:

- The specification leaves some anonymous component identity cases incomplete
  (§3.4.6 identity note). Which identity choices does a processor expose, and
  does a fixture require a result the standard actually leaves open? S-008.
- Which conformance level and invocation modes are under assessment? Which
  schema-location strategies, user overrides, optional PSVI projections, and
  identity/ID table exposures are supported? S-001/S-067/S-069/S-073.
- What outcome does the calling application regard as successful when validity
  is notKnown or attempted is partial? The specification leaves that policy to
  the application. S-068.
- Which NOTATION attribute supplies parent PSVI when several qualify is
  unspecified. S-058. Multi-episode assessment behavior is likewise outside
  the complete normative contract. S-073.
- E1-56's published replacement language contains vocabulary/editing defects
  (including lexical-form terminology) that must be read through the explicit
  correction record, without silently inventing further errata. Cite
  [corrected reading](../corrected-reading.md) when constructing those oracles.
- This catalogue does not establish tests for every numbered branch merely by
  mapping its enclosing rule. Per-branch fixture identifiers, independent
  expected values, and reachable product witnesses remain to be generated;
  the main source/grammar inventory is the denominator for that later work.
- Full datatype/facet/regex partitions, XML/namespace lexical boundaries and URI
  resolution details belong to their companion catalogues. Cross-boundary
  scenario links are obligations, not evidence that the corresponding fixtures
  or oracle outputs already exist.
- No corpus comparison or implementation conformance conclusion has been made.
  Later coverage must distinguish valid-schema, invalid-schema, valid-instance,
  invalid-instance, PSVI and processor-policy evidence; one cannot substitute
  for another.
