# Declaration branch witnesses

This is an independent, specification-derived description of concrete witnesses
for Structures §§1–3.3. Read the [unchanged publication](../../upstream/structures.html)
with [E1-56](../../upstream/errata-second-edition.html#e1-56) and the
[corrected reading](../../corrected-reading.md). The
[scenario method](../README.md) distinguishes a required result from an
implementation choice, unavailable observation, or unresolved source tension.
These witnesses refine families S-001, S-003–S-007, S-009–S-017, S-032, and
the assessment/PSVI families in [the Structures catalogue](../structures.md).
No implementation or corpus behavior supplied an expectation.

**S** observes schema construction/component properties, **V** local validity,
**P** PSVI, and **O** invocation or processor choice. A success/failure-only API
cannot prove P. “Invalid” below means the named schema or assessed item, not an
I/O failure. A component-level missing-reference witness uses the partial-schema
boundary of §5.3; it is not a claim that an arbitrary malformed component is a
valid schema. Each semicolon-separated variant is a separate witness.

## Shared exact sketches

Use `xs=http://www.w3.org/2001/XMLSchema`,
`xsi=http://www.w3.org/2001/XMLSchema-instance`, `t=urn:t`, and `p=urn:p`.
Unless changed, the schema has `targetNamespace="urn:t"`,
`elementFormDefault="qualified"`, `attributeFormDefault="unqualified"`.
Declarations quoted below are inserted into that schema; instance roots are
`<t:r xmlns:t="urn:t" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">`.
No unmentioned defaults, fixed constraints, blocking, abstractness, identity
constraints, attributes, or content are present. Assess the entire root.

```xml
<xs:simpleType name="NonNeg">
  <xs:restriction base="xs:integer"><xs:minInclusive value="0"/></xs:restriction>
</xs:simpleType>
<xs:simpleType name="U">
  <xs:union memberTypes="xs:integer xs:boolean"/>
</xs:simpleType>
<xs:complexType name="B">
  <xs:sequence><xs:element name="x" type="xs:string" minOccurs="0"/></xs:sequence>
  <xs:attribute name="a" type="xs:integer"/>
</xs:complexType>
<xs:complexType name="E">
  <xs:complexContent><xs:extension base="t:B">
    <xs:sequence><xs:element name="y" type="xs:string" minOccurs="0"/></xs:sequence>
  </xs:extension></xs:complexContent>
</xs:complexType>
<xs:complexType name="R">
  <xs:complexContent><xs:restriction base="t:B">
    <xs:attribute name="a" type="xs:integer"/>
  </xs:restriction></xs:complexContent>
</xs:complexType>
<xs:complexType name="ER">
  <xs:complexContent><xs:restriction base="t:E">
    <xs:sequence><xs:element name="x" type="xs:string" minOccurs="0"/></xs:sequence>
    <xs:attribute name="a" type="xs:integer"/>
  </xs:restriction></xs:complexContent>
</xs:complexType>
<xs:complexType name="SC">
  <xs:simpleContent><xs:extension base="xs:integer">
    <xs:attribute name="a" type="xs:integer"/>
  </xs:extension></xs:simpleContent>
</xs:complexType>
<xs:complexType name="ReqSC">
  <xs:simpleContent><xs:restriction base="t:SC">
    <xs:attribute name="a" type="xs:integer" use="required"/>
  </xs:restriction></xs:simpleContent>
</xs:complexType>
<xs:complexType name="NonNegSC">
  <xs:simpleContent><xs:restriction base="t:SC">
    <xs:minInclusive value="0"/>
    <xs:attribute name="a" type="xs:integer"/>
  </xs:restriction></xs:simpleContent>
</xs:complexType>
<xs:complexType name="M" mixed="true">
  <xs:sequence><xs:element name="x" type="xs:string" minOccurs="0"/></xs:sequence>
</xs:complexType>
<xs:complexType name="NeedX" mixed="true">
  <xs:sequence><xs:element name="x" type="xs:string"/></xs:sequence>
</xs:complexType>
<xs:complexType name="Empty"/>
```

`r:T` means `<xs:element name="r" type="T"/>`; specified attributes such as
`nillable="true"` are added to that declaration. `body="1"` means exactly one
character child `1`. “Empty” has no character or element children; comments
and PIs are separately stated. Use the named type's existing namespace bindings
for schema QNames; add bindings explicitly stated for instance QNames.

For substitution witnesses, add global `h:B`, `m:E substitutionGroup="t:h"`,
`n:E substitutionGroup="t:m"`, and a root whose sole required particle is
`<xs:element ref="t:h"/>`. Only the stated variant changes the head/member
properties. An instance child `<t:m/>` is distinguished from `<t:h/>`.

## Foundations and unnumbered representation conditions

Authority: [§2.1](../../upstream/structures.html#d0e504),
[§2.4](../../upstream/structures.html#concepts-conformance),
[§2.5](../../upstream/structures.html#concepts-nameSymbolSpaces),
[§2.6](../../upstream/structures.html#Instance_Document_Constructions), and
[§3.1](../../upstream/structures.html#scIntro).

| ID | Exact setup/change | Expected result and observation |
| --- | --- | --- |
| A-001 | Root has a required local child `v:xs:integer`; compare `<t:v>1</t:v>` with `<t:v>oops</t:v>`, keeping the root's own particle match unchanged. | Child local validity changes valid→invalid; root assessment incorporates that result. Distinguishes local validity from aggregate assessment. V/P. |
| A-002 | Supply the same `r:xs:integer` component directly and as XML, then body `1`/`oops`; advertise minimum, XML-representation, or full conformance. | All levels owe semantic constraints/validation/PSVI; XML level additionally owes XML mappings; full level additionally owes the specified Web access capability. A minimum processor need not accept the XML input form. O. |
| A-003 | Declare `r` as both element and type; then two distinct types named `r`, one simple and one complex. Put local `a`/`x` in each of two complex types. | Element/type name coexistence and independent local spaces are permitted; the two types conflict because simple/complex types share one space. Reuse the name also in attribute/group/attribute-group/notation/identity spaces without creating an intra-space duplicate. S. |
| A-004 | Rename all schema `xs` prefixes to `s` with the same binding; then bind `s` to `urn:not-xsd`. For `r` compare namespace `urn:t`, no namespace, and `urn:other`. | Prefix renaming preserves components; wrong schema namespace does not represent an XSD declaration. Only the declared expanded instance name matches `r`. Namespace admission precedes assessment. S/V. |
| A-005 | `r:B` omits nillable, abstract, value constraint, identity children and annotation; compare adding `nillable="false"`, an empty annotation, and `default=""` on `r:xs:string`. | Omitted booleans are false; omitted optional properties are absent; identity set is empty. Empty annotation and an empty-string constraint are present values, not absence. Required properties cannot be discarded except the separately admitted missing-reference boundary. S. |
| A-006 | `Node` contains optional local `next` of type `t:Node`; place the type after `r:Node`. Compare a two-level instance with an empty root. | Both valid. Forward references and re-entrant component identity are permitted; the schema component graph need not be acyclic merely because a type refers to itself through content. This does not permit cyclic derivation or substitution affiliation. S/V. |
| A-007 | Construct the same conformant schema element Infoset inside `<wrapper>` and as a document root, with its namespace context retained. | The schema document is the schema element Infoset; being the document element is not intrinsically required. A processor may choose which input form its interface accepts. O/S. |
| A-008 | Add `p:note="v"` and a first `xs:annotation` to an attribute declaration; compare unqualified `note`, `xs:note`, and an annotation after its inline `simpleType`. | Foreign attributes and first annotation are admitted by the relevant grammar and map to annotation information; unqualified/XSD unknown attributes and misplaced annotation fail grammar. Where the foreign wildcard is lax and a declaration is available, an invalid declared foreign value is not an unconditional bypass. S. |
| A-009 | Element text has code points `SPACE a TAB SPACE b LF SPACE`; use `xs:string`, `xs:normalizedString`, `xs:token`, and omitted attribute type anySimpleType. Insert a comment and PI between `a` and TAB. | Preserve keeps the sequence; replace changes TAB/LF to spaces; collapse yields `a b`; anySimpleType preserves. Comments/PIs add no characters. For attribute witnesses use character references or a synthetic Infoset so XML attribute normalization does not erase the intended input first. V/P. |
| A-010 | Existing DTD-normalized attribute initial value is `a b`; compare its XSD token validation with initial value ` SPACE a TAB b SPACE `. | Both XSD-normalized values are `a b`. XSD normalization applies to the received initial value independently of prior DTD processing; no reconstruction of original serialization is required. V/P. |
| A-011 | On a simple `r:xs:string` with nillable=true, independently supply `xsi:type="xs:string"`, `xsi:nil="false"`, `xsi:schemaLocation="urn:a a.xsd urn:b b.xsd"`, or `xsi:noNamespaceSchemaLocation="s.xsd"`, with `xs` bound in the instance. | The four builtin attributes need no user declarations; their types are QName, boolean, list(anyURI), and anyURI. List typing alone does not establish schemaLocation pair semantics; location hints do not force retrieval. Adding `xsi:other` is outside this exception. V/P/O. |
| A-012 | Declare global `ga:xs:integer`; use `ref="t:ga"` in B. Compare instance attribute `t:ga="1"` with unprefixed `ga="1"` while setting the instance default namespace to `urn:t`. | Only the prefixed attribute has the required namespace. Default namespaces do not qualify attribute names; prefixes spelling the same namespace are interchangeable. S/V. |

## Attribute declarations, validation, and PSVI

Authority: [§3.2.2](../../upstream/structures.html#declare-attribute),
[`src-attribute`](../../upstream/structures.html#src-attribute),
[`cvc-attribute`](../../upstream/structures.html#cvc-attribute),
[`cvc-assess-attr`](../../upstream/structures.html#cvc-assess-attr),
[§3.2.5](../../upstream/structures.html#Attribute_Declaration_details), and
[§3.2.6](../../upstream/structures.html#a-props-correct), with E1-56 E-01/E-02.

The canonical-form and ID-constraint conditions in A-021/A-022 govern a
declaration's value constraint. Local named and reference attributes put their
own constraint on an attribute use instead; retain
[DCS-Q001](../source-questions.md#dcs-q001--attribute-use-constraints-and-declaration-scoped-rules)
before transferring those conditions. Typed initial-value construction still
applies to the use.

| ID | Exact setup/change | Expected result and observation |
| --- | --- | --- |
| A-013 | Global `<xs:attribute name="ga" type="xs:integer"/>`; replace type with inline restriction `xs:integer minInclusive=0`; then omit both. | Global declaration scope; type respectively builtin integer, anonymous simple restriction, and anySimpleType. Values `-1` distinguish the first two; `oops` distinguishes integer from anySimpleType. S/V. |
| A-014 | Add local `<xs:attribute name="a" type="xs:integer"/>` to an empty type; compare `use="required"`, `use="optional"`, and `use="prohibited"`. | Absent/optional creates a nonrequired use plus local declaration; required creates a required use; prohibited creates no use/declaration from this occurrence. Missing `a` fails only required; present `a` is not admitted by a prohibited use without another admitting path. S/V. |
| A-015 | Local `a:xs:integer default="1"`; add `fixed="1"`. | Default alone correct; default plus fixed is invalid schema even though the values agree. Repeat with different values to show that equality cannot repair syntax. S. |
| A-016 | Local `a default="1"`; independently use absent, optional, required, prohibited. | Absent/optional are correct; required/prohibited violate the default/use condition. In contrast, `fixed="1" use="required"` is not forbidden by that condition. S. |
| A-017 | Local attribute with only name, only ref to global ga, neither, or both. | Exactly-one variants correct; neither/both invalid. A top-level `ref` is additionally forbidden by global grammar, independent of this local-only condition. S. |
| A-018 | Valid local `ref="t:ga"`; independently add `type="xs:integer"`, `form="qualified"`, or inline `simpleType`. | Each added item separately makes the schema invalid. Occurrence annotation/id and use/value-constraint attributes have their own admitted grammar; they are not banned by the ref-specific condition. S. |
| A-019 | Local named `a type="xs:integer"`; add inline restriction `xs:integer minInclusive=0`. | Either type source alone correct; both together invalid. This isolates type/inline exclusivity without a ref. S. |
| A-020 | Change global ga's name to `a:b`, then to empty; alternatively resolve its type to complex B. At component level make {name} an invalid NCName or {type definition} a complex component. | Invalid schema/component properties. A simple type and legal name are required; a missing referenced component follows §5.3 instead of being silently replaced with a wrong-kind component. S. |
| A-021 | Global ga integer default `+001` versus `oops`; then use integer restriction `pattern="[+].*"` with default `+1`. | `+001` is an admitted integer constraint and canonical `1` validates; `oops` fails value construction. The pattern case has admitted initial spelling but canonical `1` violates the type, so a-props-correct clause 2 rejects it. Do not use that rejected schema as an E1-56 fixture. S. |
| A-022 | Global attribute of xs:ID; then an ID restriction with minLength=1; add default `id1` or fixed `id1`. | All four constraint-bearing variants invalid schema; the same types without constraints are permitted. S. |
| A-023 | Declare attribute name `xmlns`; name `xmlns:p`; name `xml`. | First fails no-xmlns; second is not an NCName; ordinary local name `xml` is not forbidden by those rules. Namespace declarations are not ordinary attributes and cannot acquire defaults through XSD declarations. S. |
| A-024 | A schema targeting the xsi namespace declares `extra`; compare redeclaring `nil`, and using the pre-existing builtin nil declaration without user declaration. | Both user declarations are forbidden; only the four specified builtin declarations are exceptions. Prefix spelling is irrelevant. S. |
| A-025 | In target namespace urn:t, local a uses form absent/unqualified/qualified under each attributeFormDefault value; repeat with no target namespace. | Qualified/qualified-default produces urn:t; unqualified/override produces absent namespace. With no target namespace all these admitted forms yield absent namespace. Global ga ignores local-form default and has its schema target namespace. S/V. |
| A-026 | Two types independently declare local a, one integer and one boolean; separately replace the locals in two compatible types with a reference to an attributeGroup containing a. | Local type scopes are separate. The group declaration initially has absent scope, resolved in each type construction. `a="true"` distinguishes boolean from integer; moving ga to a global declaration gives global scope instead. S/V. |
| A-027 | Global ga integer fixed `1`, referenced with no use-level value constraint; compare `t:ga="1"`, `+001`, `2`, `oops`. | First two are declaration-valid by typed equality; `2` is type-valid but fixed-invalid; `oops` is type-invalid. Do not add a use-level fixed constraint, whose separate §3.5 lexical rule would mask this distinction. V/P. |
| A-028 | Assess attribute `{urn:t}missing="1"` without a declaration; then provide a global integer declaration and evaluate it. | First cannot satisfy local-valid clause 1 or strict assessment; second can. Missing declaration is distinct from an available declaration whose value is invalid. V/P. |
| A-029 | Under the admitted partial-schema boundary, known ga has an unresolved simple-type reference; assess its value `1`. | cvc-attribute clause 2 is unsatisfied; strict attribute assessment cannot be established. Do not invent integer or anySimpleType as its missing declared type. V/P/O. |
| A-030 | Known integer ga receives `1`, `1.5`, and empty string, without fixed constraints. | Only `1` type-valid. Strict assessment can be full and invalid for the other two; full does not mean valid. V/P. |
| A-031 | Change ga from integer default `1` to fixed `1`; provide explicit `2`. For an absence companion, give the optional ga ref an explicit matching use-level default/fixed `1`, respectively. | Default does not constrain explicit `2`; fixed does. The explicit optional use constraint supplies the absent attribute under the stated synthesis predicate. Without that use constraint, keep declaration-only fallback as B-044's source question rather than inferring synthesis. Element defaults are a separate mechanism. V/P. |
| A-032 | Same ga: explicit attribute use; then wildcard lax with ga globally available; then wildcard skip with ga still available. | First uses the context-determined declaration, second may resolve by expanded name and becomes strictly assessed when evaluated, third must not gain strict assessment through that lookup. Unknown lax attribute has no known declaration. V/P. |
| A-033 | Provide a known declaration/type and value `oops` but stipulate that no local validation was performed; compare completed validation. | Metadata availability alone does not establish assessment. Completed evaluation establishes strict assessment even when invalid; absent declaration/type still prevents strict assessment. O/P. |
| A-034 | Compare strictly assessed ga=`1`, strictly assessed ga=`oops`, and unassessed skip attribute. | Attribute validity/attempted pairs are `(valid,full)`, `(invalid,full)`, `(notKnown,none)` respectively. No attribute partial-assessment outcome exists. P. |
| A-035 | The valid and invalid assessed ga cases of A-034. | Valid item schema-error-code absent; invalid item has a list, possibly empty. Populating explanatory codes is encouraged, not a required nonempty/ordered diagnostic sequence. P. |
| A-036 | Valid ga with processors choosing to expose or omit full declaration information; then fixed-invalid but integer-valid ga=`2`. | The optional attribute-declaration contribution is permitted for a valid attribute; the latter does not acquire that requirement merely because its simple value is valid. Type-derived facts are separately governed. P/O. |
| A-037 | A-027 ga=`2` versus `oops`; then type U with `1`, `true`, and `oops`. | E1-56: integer-valid `2` has normalized value `2` despite fixed invalidity; `oops` has absent normalized value. U selects integer for `1`, boolean for `true`, no actual member for `oops`; unsuccessful union validation cannot supply a member. P. |
| A-038 | Named U and an anonymous inline union with anonymous integer restriction followed by xs:boolean; assess `1`/`true`. Compare full-component and lightweight type representations. | Report selected member identity consistently with member order. Lightweight names/namespaces/anonymous flags describe actual types; an anonymous name may be absent or processor-generated unique name. Named U is not anonymous. Both permitted representation alternatives require consistent facts. P/O. |
| A-039 | Skip attribute initial value ` SPACE a TAB b SPACE `; compare strictly assessed xs:token attribute with same initial value. | Unassessed attribute retains initial value and anySimpleType-based type facts; assessed token yields `a b`. AnySimpleType preservation is distinct from string lexical canonicalization. P. |
| A-040 | Put an assessed attribute beneath a root with schema-information; then add schema-information to a nearer ancestor. | Validation-context identifies the nearest ancestor with that property; attributes do not use themselves as the element context. P. |
| A-041 | Schema ga has QName fixed `p:n`, with schema p=urn:A; instance uses `q:n`, q=urn:A, then q=urn:B. | First declaration-valid, second fixed-invalid; schema QName value is not reinterpreted using the instance's spelling/bindings. Prefix-only changes preserving the expanded value do not change equality. V/P. |
| A-042 | Global ga integer default `+001`, with an optional ref carrying its own explicit default `+001`; compare present instance `2` and absent instance. | Schema-default property is canonical `1`; present attribute normalized value is `2`, schema-specified infoset. The absent attribute is contributed because the optional use constraint is explicitly present. Dropping that explicit use constraint reaches B-044's separate declaration-fallback question. P. |

## Element representation and properties

Authority: [§3.3.1](../../upstream/structures.html#Element_Declaration_details),
[§3.3.2](../../upstream/structures.html#declare-element),
[`src-element`](../../upstream/structures.html#src-element), and
[`e-props-correct`](../../upstream/structures.html#e-props-correct).

| ID | Exact setup/change | Expected result and observation |
| --- | --- | --- |
| A-043 | `r` has type t:B, then inline anonymous empty complexType, then no type or inline child or affiliation. | Type respectively B, anonymous complex type, anyType. The last admits mixed text/undeclared child/attribute subject to lax recursive assessment where declarations exist; absence is not the empty type. S/V/P. |
| A-044 | Local element x or local ref t:h in a sequence, occurrences omitted; then 0..0, 0..1, 1..1, 1..unbounded; compare zero, one, and two children. Replace a zero occurrence spelling by -0 or -00. | Default interval 1..1. Zero/zero yields no component from that occurrence; other forms yield a particle with the local/global declaration term and stated bounds. Negative-zero spellings denote the admitted nonNegativeInteger value zero; they do not violate the occurrence lexical type. Zero/one/two distinguish ranges; 1..0 is invalid schema, not an empty particle. S/V. |
| A-045 | Local element with name only, ref only, neither, or both; add global element ref instead of name. | Local exactly-one alternatives correct; neither/both invalid. Global ref violates the global grammar. S. |
| A-046 | Local ref t:h; independently add simpleType, complexType, key, keyref, unique, nillable, default, fixed, form, block, or type. | Every added child/attribute is independently forbidden; for key/keyref/unique use otherwise syntactically complete definitions so the ref prohibition is isolated. minOccurs/maxOccurs/id/annotation remain admitted. S. |
| A-047 | Named `r type="xs:integer"`; add inline simpleType, then separately inline complexType. Independently combine default `1` and fixed `1`. | Both type/inline variants invalid, as is default+fixed even with equal values. A sole inline type is admitted; simpleType plus complexType is rejected by grammar. S. |
| A-048 | r:B with effective block `""`, `#all`, and each subset of `{extension,restriction,substitution}` written as its space-separated names; compare legal blockDefault inheritance, explicit empty override, and illegal blockDefault="list union". | Legal values map respectively to the empty set, all three, or exactly the named members. All eight subsets are witnesses. Explicit empty overrides a legal default. list/union are outside the schema blockDefault grammar, so that root value makes the schema invalid before mapping; a local override cannot rescue it. S. |
| A-049 | Effective final `""`, `extension`, `restriction`, `extension restriction`, `#all`; compare inheritance from finalDefault and explicit empty override. | Map to the four subsets of extension/restriction; #all is the full two-member set. list/union tokens of finalDefault do not become element exclusions. Exclusions govern affiliations, not the element's block property. S. |
| A-050 | Local named x under a complexType or named group; vary form and elementFormDefault as in A-025. Global x omits nillable/abstract/annotation/identity children. | Namespace mapping follows form/default; direct local scope is its type, group-origin scope initially absent until use, global scope global. Omitted nillable/abstract=false, annotation absent, identity set empty. Named group reuse does not turn a local declaration into a global. S. |
| A-051 | Global m has affiliation t:h and no explicit type; then give type t:E; then inline simpleType incompatible with B. | Omitted type inherits B from h; explicit E overrides the fallback and is derivationally admissible; incompatible inline type fails affiliation constraints rather than being ignored. Removing affiliation as well selects anyType. S. |
| A-052 | Element name `a:b`, nonboolean nillable `yes`, or type reference to attribute ga; component alternative has identity-constraint property a scalar instead of a set. | Each violates grammar/property ranges independently. An unresolved valid type QName is a missing-component issue, not permission to substitute a wrong-kind component. S. |
| A-053 | r:xs:ID with default/fixed `id1`; repeat for restriction of ID and complex simple-content extension of ID. | All constraint-bearing cases invalid schema; without value constraints the types can be used. The prohibition inspects complex simple content as well as a directly simple element type. S. |

## Element local validity and independent failure exits

Authority: [`cvc-elt`](../../upstream/structures.html#cvc-elt),
[`cvc-type`](../../upstream/structures.html#cvc-type),
[`cvc-id`](../../upstream/structures.html#cvc-id). Unless stated, the component
schema is correct, and all unrelated premises satisfy their constraints.

| ID | Exact setup/change | Expected result and observation |
| --- | --- | --- |
| A-054 | Ask declaration-based local validation of root r with no declaration; compare known r:xs:string with empty content. | First lacks a required declaration, second valid. Type-only/lax assessment may remain possible via different invocation rules; missing declaration is not equivalent to an invalid lexical value. V/O. |
| A-055 | r:B has abstract=false then true; assess its own name, with/without nillable=true and xsi:nil=true, or xsi:type=t:E. | Direct declaration validation cannot pass when the declaration is abstract; nil and a concrete xsi:type do not make its own declaration concrete. A concrete substitution member is a distinct declaration. V. |
| A-056 | r:xs:string nillable=false; nil absent, false, 0, true, 1. | Absence is allowed; every present xsi:nil variant violates the formal nonnillable presence condition, including false/0. Malformed boolean `yes` also fails builtin attribute typing. V. |
| A-057 | r:xs:integer nillable=true, empty content; nil absent,false,0,true,1,` SPACE true SPACE `,yes. | Only true/1/whitespace-valid-true bypass the empty integer value failure; false/0/absent require ordinary content validation and fail; yes is invalid, not truthy. With body `1`, false/0/absent are valid. V. |
| A-058 | Nillable r:xs:integer nil=true; content none, comment only, PI only, SPACE, `1`, or `<t:x/>`. | First three satisfy the empty-child condition; character/element cases fail. A SPACE still is character content even if its type would collapse it away. V. |
| A-059 | A-058 empty r with no constraint, default `1`, or fixed `1`; repeat nil=false with fixed. Add required integer attribute a on a complex type. | True nil permits no/default constraint but forbids fixed; false does not nil the element and permits the fixed/default empty-content path. True nil still requires permitted/required attributes; absence or `a="oops"` invalidates that complex case. V/P. |
| A-060 | r:B with instance xsi:type `p:E` where p unbound; `t:NoSuch`; `t:E:bad`; and bound existing `t:E`. | Respectively QName/binding failure, unresolved type, malformed QName, permitted E. Invalid type attributes do not silently become valid by falling back to B; B is only the defined actual-type fallback for subsequent evaluation. V. |
| A-061 | r:B xsi:type B/E/R/ER; set element block empty, extension, restriction, or both. | Same B identity passes derivation blocking; E fails exactly when extension blocked, R when restriction blocked, ER when either blocked. Unrelated xs:string fails derivation. Exercise a two-step path so blocking checks all derivation methods. V. |
| A-062 | Set B block=extension, then leave it empty; assess r:B xsi:type E. Independently set B abstract=true and use absent xsi:type or concrete E. | Type block and element block are unioned for complex derivation: E fails under B's extension prohibition. Abstract B cannot validate as actual type, but concrete E can when derivation/block conditions permit. Schema type final would instead constrain construction of E. V/S. |
| A-063 | r:xs:decimal uses xsi:type xs:integer, xs:decimal, or xs:string; element block empty versus restriction. | Integer is a permitted restriction only when restriction unblocked; same decimal identity passes; string is unrelated. The simple-type route does not read a complex-type prohibited-substitutions property. V. |
| A-064 | r:B with `<t:x/>` versus `<t:z/>`; separately r:xs:integer body `1`/`oops`, no constraint/nil/type overrides. | Correct content/value valid; wrong child or lexical value invalid through actual-type validation. Keeping declaration/type selection correct isolates clause 5.2.1. V. |
| A-065 | r:xs:integer default `+001`, empty; add only a comment/PI; then SPACE or body `1`. | Empty/comment/PI cases use clause 5.1; SPACE/body use ordinary content branch because character children exist. SPACE fails integer validation; `1` validates without schema-supplied content. V/P. |
| A-066 | r:xs:integer default `-1`, empty, xsi:type xs:nonNegativeInteger; compare default `0`. Separately r:SC default `1`, xsi:type=t:ReqSC, absent/present a=`2`. | `-1` is valid for declared integer but not a valid default for the local nonnegative type; `0` passes. In SC case literal default validity succeeds, yet missing required a fails actual-type validation; present a passes. Isolates 5.1.1 from 5.1.2. V. |
| A-067 | r:SC fixed `1`, body `1`, `+001`, or `2`; r:M fixed `a`, body `a` or `a SPACE`; then body `a<t:x/>`. | SC compares typed integer equality, accepting aliases and rejecting 2; mixed content compares initial characters, so trailing SPACE fails. Any element child violates the fixed-content no-element condition even when the text equals the fixed string. See source tension below for the published typed/canonical wording. V. |
| A-068 | No constraint: r:xs:string empty and r:xs:integer empty; add true nil only to a nillable integer. | Empty string valid; empty integer invalid; true nil uses 5.2 but suppresses the simple content value check. It does not suppress other type constraints. V. |
| A-069 | Nillable r:xs:string with body `a`; independently add one of the four builtin xsi attributes with valid values, ordinary `a="1"`, xml:lang="en", or xsi:other="v". | Only the four named xsi attributes are exempt from the simple-type no-attributes rule. Ensure nil=false and xsi:type=xs:string to avoid masking by incompatible selections. V. |
| A-070 | r:xs:integer body `1<!--c--><?p v?>2`; compare `<t:x/>` child. Separately select an absent actual type via partial schema, abstract complex type, or concrete B. | First initial value `12` valid; element child invalid for simple type. Missing type and abstract complex type each independently prevent local type validity; concrete B with empty content passes. V. |
| A-071 | r:SC, nillable=true, key selector `.` field `@a`, optional a integer. Assess nil=true with a=`1`, and without a. Compare nonnilled body `1` with a=`1`/missing. | Key must hold in all cases; missing key field invalidates declaration assessment despite nil/type content success. Nil bypass is not an identity-constraint bypass. V/P. |
| A-072 | Root has repeated n elements with optional attrs id:xs:ID, ref:xs:IDREF, refs:xs:IDREFS. Compare ref=a then id=a; only ref=a; id=a twice; refs=`a b` with only id=a. Assess a subtree containing ref=a while its matching id lies outside that validation root. | Forward reference valid; missing target and list's missing b fail zero-binding root condition; duplicate IDs fail multiple-binding condition. An outside-root ID does not satisfy a subtree root's table. Distinguish invalid root closure from lexical validity of each IDREF. V/P. |

## Assessment routes and outcome aggregation

Authority: [`cvc-assess-elt`](../../upstream/structures.html#cvc-assess-elt),
[`sic-e-outcome`](../../upstream/structures.html#sic-e-outcome), and
[element error/declaration contributions](../../upstream/structures.html#sic-elt-error-code).

| ID | Exact setup/change | Expected result and observation |
| --- | --- | --- |
| A-073 | Known r:B is selected by processor-stipulated declaration; as a child by an explicit particle; through a non-skip wildcard with global r available. Repeat wildcard skip. | The first three supply the respective declaration routes when local evaluation occurs; skip prevents the expanded-name lookup route. Declaration availability does not itself establish completed validity evaluation. O/V/P. |
| A-074 | Known root declaration has unresolved type reference under §5.3, and actual-type validation is reached; compare known r:B. | Missing actual type defeats the strict declaration-assessment prerequisite; known B with completed evaluation satisfies it. In neither case should mere availability of a declaration manufacture a type. O/P. |
| A-075 | Unknown root name u; stipulate xs:integer type and body `1`/`oops`. Then omit stipulation but supply xsi:type="xs:integer" with proper binding. | Both known-type routes permit strict type-based assessment without an element declaration; lexical validity is still evaluated. No stipulated type, no usable xsi:type, and no declaration leaves only the non-skip lax possibility. O/V/P. |
| A-076 | Unknown root with stipulated B; xsi:type E, R, unrelated xs:string, malformed QName, or absent. Toggle B's extension prohibition. | E/R admitted only when derived under the stipulated type's prohibited substitutions; unrelated/malformed cannot supply a valid local type. Absent local type uses stipulated B. Where both usable xsi:type and declaration routes exist, retain the specified xsi:type precedence without deleting separately applicable declaration constraints. O/V/P. |
| A-077 | A known declaration or known stipulated type is available, but assessment stops before local validity evaluation; compare completed evaluation with invalid content. | First has not met the evaluation premise; second can be strictly assessed and invalid. Test the two availability routes independently. An early operational failure is not semantic evidence that evaluation completed. O/P. |
| A-078 | Strict known root contains known valid child and known valid attribute; change each independently to invalid; then use lax/skip unknown child and attribute. | Assess every child and attribute according to its own route. Child and attribute outcomes remain separate inputs to parent aggregation; neither missing assessment nor an invalid descendant may be silently promoted to valid/full. P. |
| A-079 | Unknown non-skip root with no usable type: empty; with known valid child r:xs:integer body 1; with only unknown children. Repeat under skip. | Non-skip root may be laxly assessed against anyType. Its validity remains notKnown; known strictly assessed descendant changes attempted from none to partial. All-none descendants permit none. Skip does not authorize lax fallback or opportunistic global lookup. P/O. |
| A-080 | Strict root: all descendants full-valid; local-invalid only; one full-invalid child; one lax unknown child; one mustFind unknown child. Also unassessed parent with one full child. | Respectively `(valid,full)`, `(invalid,full)`, `(invalid,full)`, `(valid,partial)` if otherwise locally valid, `(invalid,partial)`, and `(notKnown,partial)`. Unknown lax/skip descendants do not themselves equal invalid; mustFind unknown vetoes parent validity. P. |
| A-081 | Locally valid and locally invalid assessed roots, including a root invalid only by fixed comparison. | Valid root schema-error-code absent; invalid local item has a list, allowed to be empty. No mandated diagnostic order, nonempty text, or repository-specific error name follows from this contribution. P. |
| A-082 | Schema-information exists on assessed root, then additionally on a nested element. | Element validation-context uses nearest ancestor with schema-information or itself when it has that property; descendants switch to nearer context. Distinct from attribute's ancestor-only rule. P. |
| A-083 | Valid nillable r:SC nil=true with permitted a=`1`; compare ordinary nonnilled body1. Choose full element-declaration contribution or lightweight nil alternative. | Declaration component alternative identifies r; nil alternative is true/false respectively. The processor choice does not change validity or add character content. An invalid declaration assessment does not gain a required contribution from this valid-only rule. P/O. |

## Corrected element PSVI and higher-order witnesses

Authority: [§3.3.5](../../upstream/structures.html#sic-eltType),
[E1-56](../../upstream/errata-second-edition.html#e1-56), and
[§3.3.1](../../upstream/structures.html#Element_Declaration_details).

| ID | Exact setup/change | Expected result and observation |
| --- | --- | --- |
| A-084 | r:xs:token body ` SPACE a SPACE SPACE b SPACE `; r:xs:integer body oops; r:SC body ` SPACE +001 SPACE `; r:M body a; r:Empty empty. | For the type-valid token/SC/M/Empty cases, corrected schema-normalized value is respectively `a b`, `+001`, absent, absent. Integer oops fails sic-eltType's outer type-validity entry; its inner cases alone require neither presence nor absence there. Complex simple content remains distinct from mixed/empty complex content. P. |
| A-085 | r:xs:integer default or fixed `+001`, empty/comment-only/PI-only; compare explicit +001 and explicit1. | Applied constraint contributes lexical form `+001`, not canonical1. Explicit +001 normalizes to +001, explicit1 to1. Original child lists are preserved, including no character children in defaulted cases. P. |
| A-086 | r:SC fixed1 body2, otherwise type-valid; compare bodyoops. Separately r:SC default1 with optional a and key selector `.` field `@a`, empty and a absent. | First retains normalized value2 despite fixed-invalid declaration. Type-invalid oops fails the contributor's outer entry, so no mandatory normalized-value presence/absence follows from its inner cases alone. In the keyed default case type validation succeeds but identity fails: default applies, normalized value1 and schema-specified schema remain. Do not infer contribution absence from overall declaration invalidity. P. |
| A-087 | r:xs:integer default1 empty, fixed1 empty, fixed1 explicit1, and no constraint explicit1. Repeat default1 with nil=true on nillable declaration. | Schema-specified values schema, schema, infoset, infoset, infoset respectively. Equality with a fixed value is not evidence the schema supplied content. Nilled case normalized value absent; no default text children. P. |
| A-088 | Nillable r:U with default1: nil=true empty; nil=false empty; nil absent empty; nil=true with fixed1. | True nil suppresses normalized/member values; false/absent applies default and selects integer. True nil plus fixed is invalid, still cannot be repaired by default/union selection. E1-56's nil and normalized-value premises take priority over union membership. P/V. |
| A-089 | r:U body1, true, oops, default1 empty; repeat using complex simple-content union and anonymous union member. | Type-valid 1/true/default1 select integer/boolean/integer. Union-invalid oops fails the contributor's outer entry; no mandatory member presence/absence follows from the inner cases alone. For the amended lightweight paragraph, normalized value must exist before member properties; retain PSVI-Q001's exact full-member wording qualification. Anonymous member names are optional/generated; applicable namespace/anonymous flags remain truthful. P. |
| A-090 | U1 union xs:string then xs:token; U2 reverses order. Body ` SPACE a SPACE `; then default ` SPACE a SPACE ` on empty element of each union. | Explicit content chooses string/` SPACE a SPACE ` versus token/`a`. Defaulted normalized value follows constraint lexical-form branch; actual-member reporting validates that resulting value using union order. No canonical rewrite or inserted child text. P. |
| A-091 | QName r default p:n with schema p=urn:A; empty instance rebinds p=urn:B. Compare fixed p:n and explicit q:n with q=urn:A/urn:B. Repeat union string/QName in both orders. | Schema constraint value keeps schema namespace context; explicit QName uses instance context. Equal expanded QName passes fixed, different one fails. String-first union selects string and has string equality; QName-first can select QName. The schema context must not be replaced by the instance's rebinding. V/P. |
| A-092 | Named B, named xs:integer, anonymous simple restriction, anonymous complex simple-content type; each valid. Choose full type component or lightweight representation. | Type kind, namespace, name/anonymous status match actual selected type; anonymous generated names permitted but not required. Type identity does not become declared B when a permitted E xsi:type actually validated. P/O. |
| A-093 | r:xs:integer default+001 empty versus explicit2; then r:M default a empty. | Schema-default canonical1 versus normalized+001/2 remain distinct. Mixed default contributes schema-default a and schema-specified schema but no normalized-value property or text child. P. |
| A-094 | Unknown root laxly assessed against anyType; unknown attribute skipped/not strictly assessed. | Element type facts are anyType-based; attribute facts anySimpleType-based. Do not confuse their varieties, infer a named declaration, or invent a default. P. |
| A-095 | Apply A-085/A-088 after consistently renaming xsi prefix to i; add only comments/PIs, then add a single SPACE character. | Prefix change preserves all results. Comments/PIs preserve default/nil emptiness; SPACE changes the clause-5.1/3.2 child premises even if lexical whitespace processing would erase it. P/V. |
| A-096 | Full dependent product: r:SC or r:ReqSC; nillable false/true; nil absent/false/0/true/1/yes; constraint absent/default-1/fixed-1; content empty/comment/SPACE/+001/-1/child x; xsi:type absent/same declared type/t:NonNegSC/unrelated B; a absent/7/oops. In the ReqSC variant use a copy of NonNegSC restricting ReqSC with required a. | For every tuple, retain all independent errors: forbidden/malformed nil, true-nil children/fixed, incompatible xsi:type, and required/invalid a. True nil bypasses content value but none of those errors. Empty nonnil with a constraint checks the constraint against actual type, so -1 fails only the nonnegative type; other nonnil inputs validate actual content, then fixed equality. SPACE/child cannot trigger default. For failed xsi:type selection the defined actual-type fallback still matters to later contributions; invalidity alone does not choose it. PSVI follows A-084–A-093. This explicit finite product is an expansion contract; its tuples are not claimed as executed coverage. S/V/P. |

## Default legality and substitution groups

Authority: [`cos-valid-default`](../../upstream/structures.html#cos-valid-default),
[`e-props-correct`](../../upstream/structures.html#e-props-correct),
[`cos-equiv-derived-ok-rec`](../../upstream/structures.html#cos-equiv-derived-ok-rec),
and [`cos-equiv-class`](../../upstream/structures.html#cos-equiv-class).

| ID | Exact setup/change | Expected result and observation |
| --- | --- | --- |
| A-097 | Add default1 to r:xs:integer, r:SC, r:M, r:NeedX, r:B, r:Empty; for the first two change value to oops. | Integer/SC numeric1 valid; their oops invalid. M permits a default because mixed particle emptiable. NeedX invalid because not emptiable; B invalid because element-only even though optional x; Empty invalid because neither mixed nor simple content. Isolates content kind from emptiability. S. |
| A-098 | Same substitution packet with member type E, R, B, or unrelated xs:string, head B final empty. | E/R/B derivationally admissible; unrelated type invalid schema. Global affiliation requirement is separate: assigning affiliation to a local-scope component fails, and local substitutionGroup syntax is forbidden. Missing head reference remains the missing-component boundary. S. |
| A-099 | h affiliation m and m affiliation h; self-affiliated h; normal h←m←n chain. | Both cycles invalid schema, even with identical types or abstract declarations; acyclic chain admissible. Following affiliation and following type derivation are different graphs. S. |
| A-100 | Compute substitutability D=C=h under block substitution/extension/restriction/#all. Then ask whether distinct unrelated n of the same B type substitutes for h with no affiliation chain. | Identity case succeeds regardless blocking; unrelated same-type declaration fails chain requirement. This does not promise abstract h appears in its actual group. S/V. |
| A-101 | Existing admissible m→h affiliation, head block absent versus substitution. | Schema affiliation remains admissible in both; instance m matches h particle only when substitution not blocked. h itself, if concrete, remains usable under identity. Element block does not delete the affiliation component. S/V. |
| A-102 | h←m←n with types B,E,E; use m or n as child at h particle; reverse and use h at a particle referencing n; remove n's affiliation. | Forward direct/transitive substitution passes; reverse and disconnected declaration fail. Same type names alone do not replace the required directed chain. S/V. |
| A-103 | m:E under h:B; compare h block extension/restriction. Then member type ER and intermediate E block restriction; compare B block extension. | Extension blocks m:E, restriction alone does not. ER's extension+restriction path is barred by either corresponding blocking method; intermediate type prohibition participates. Repeat a simple-type chain with empty complex-only prohibition sets to isolate that alternative. S/V. |
| A-104 | Set head abstract=true; m abstract=true; n concrete. Enumerate potential and actual groups for h. Then make n abstract. | Potential group includes h,m,n; actual excludes abstract h/m but includes admissible concrete n. Making n abstract removes the last concrete member; potential closure is unchanged. A blocked concrete member is separately removed by substitutability, not abstractness. S/V. |
| A-105 | Head final extension with m:E versus m:R; head final restriction reverses; head block extension with m:E. | Final rejects the prohibited affiliation at construction; block permits construction but prevents instance substitution. finalDefault/blockDefault flow through their own mappings. Validity of a type declaration E itself is distinct from its use as an affiliated element type. S/V. |
| A-106 | Head final=#all, member type exactly B, member affiliated h; compare member E. | Formal e-props-correct/type-derivation identity permits same-type B while E is prohibited; §3.3.1 prose says both exclusions forbid any nomination. Preserve this source tension rather than claiming a unique oracle for B. E rejection is unambiguous. S. |
| A-107 | No occurrence of element r in an optional parent particle; compare a present empty r default1. | The absent element creates neither element nor default text; the present empty element gets the corrected default contributions. Defaults do not satisfy a required missing particle or synthesize absent elements. V/P. |

## Source tensions and limits

1. E1-56 replaces the schema-normalized-value definition and schema-specified
   default branch, but does not replace the canonical-form validation premises
   in cvc-elt 5.1 or component default legality. A-021/A-065/A-085 distinguish
   schema admissibility, validation of a constraint, and recorded lexical PSVI.
   The 2004 component tableau stores a value/kind pair whereas the erratum
   refers to the constraint's lexical form; a reading/fixture must retain the
   supplied lexical spelling rather than reconstruct it from the value.
2. cvc-elt 5.2.2.2.2 phrases matching between an actual value and a canonical
   lexical representation. A-067 uses typed equality supported by §3.3.1;
   keep the cross-representation wording visible when deriving a lower-level
   oracle. The printed branch guards name mixed and complex-simple content;
   they do not expressly state the direct-simple-type case. Preserve that
   wording limit without replacing the §3.3.1-supported typed reading or
   inventing an acceptance loophole. A validity-only fixture cannot settle a
   PSVI-string interpretation.
3. A-106 records an actual prose/formal conflict. It is not resolved by a
   processor's existing acceptance. No later approved erratum on the retained
   list settles it.
4. The outer scope of sic-eltType and the corrected per-property validity
   premises must both be preserved. A-086 isolates declaration-level fixed or
   identity failures while type validation succeeds. It deliberately does not
   infer mandatory type contributions from a complex type with invalid
   attributes or an invalid lexical/content value. A-084/A-086/A-089's failed
   type cells and A-096's dependent reuse inherit this qualification: the inner
   absence cases do not require universal property absence outside the entry.
5. [PSVI-Q001](../source-questions.md#psvi-q001--union-member-amendment-sites-and-contributor-scope)
   retains E1-56's explicit lightweight member patch sites and the unchanged
   earlier full-member wording. For attributes, a member accepting while the
   governing union facet fails is a separate boundary from A-037's ordinary
   successful/no-member examples. Do not extend the replacement text silently
   to choose a unique full-member oracle. Element contributor entry and
   nil/default/value premises remain separate; no contradiction or fabricated
   member follows from the presentation asymmetry.

This file audits the owned branch decisions and supplies prose/property
witnesses; it does not instantiate every Appendix A grammar path, execute the
finite interaction products, or prove corpus coverage. Grammar-level exhaustive
instantiation, detailed §3.4/§3.9/§3.11/§3.14 derivation/content/identity
algorithms, and the later corpus comparison have separate owners. Cross-rule
examples here expose their effects at the declaration boundary.

## Named contributions without numbered branches

| Source constraint | Concrete cases | Decision |
| --- | --- | --- |
| `sic-attr-decl` | A-036 | Valid-only optional attribute-declaration contribution. |
| `no-xmlns` | A-023 | Namespace declarations cannot be supplied as ordinary declared attributes. |
| `no-xsi` | A-024 | User declarations in the XSI namespace versus its four predefined exceptions. |
| `sic-elt-decl` | A-083 | Valid-only declaration/nil exposure alternatives. |

## Numbered-branch disposition

Keys below are source-order identifiers from `inventory.json`, **not** normative
clause numbers. `reviewed` maps a source decision to distinguishing witnesses;
`composite` groups conjunction/case children without asserting a separate
leaf oracle. `superseded` refers to a baseline rule replaced by E1-56 and maps
to witnesses of the current reading. A mapping is traceability, not observed
behavioral coverage.

| Inventory key | Concrete scenarios | Disposition | Reason |
| --- | --- | --- | --- |
| `structures/d0e504/branch/001` | A-001 | reviewed | Local validity aspect. |
| `structures/d0e504/branch/002` | A-001 | reviewed | Aggregation/augmentation aspect. |
| `structures/src-attribute/branch/001` | A-015 | reviewed | Exclusive default/fixed. |
| `structures/src-attribute/branch/002` | A-016 | reviewed | Default requires optional use. |
| `structures/src-attribute/branch/003` | A-014, A-017, A-018 | composite | Local-only conjunction. |
| `structures/src-attribute/branch/004` | A-017 | reviewed | Exactly one name/ref. |
| `structures/src-attribute/branch/005` | A-018 | reviewed | Every ref-forbidden item independently. |
| `structures/src-attribute/branch/006` | A-019 | reviewed | Exclusive type sources. |
| `structures/src-attribute/branch/007` | A-020, A-021, A-022, A-023, A-024 | composite | Component constraints apply as well as grammar. |
| `structures/cvc-attribute/branch/001` | A-028 | reviewed | Absent/known declaration. |
| `structures/cvc-attribute/branch/002` | A-029 | reviewed | Missing type differs from bad value. |
| `structures/cvc-attribute/branch/003` | A-030 | reviewed | Valid/invalid simple lexical value. |
| `structures/cvc-attribute/branch/004` | A-027, A-031, A-041 | reviewed | Typed fixed equality, default independence, QName context. |
| `structures/cvc-assess-attr/branch/001` | A-028, A-032 | composite | Known-declaration alternatives. |
| `structures/cvc-assess-attr/branch/002` | A-032 | reviewed | Context-determined declaration. |
| `structures/cvc-assess-attr/branch/003` | A-032 | reviewed | Lookup unless skip. |
| `structures/cvc-assess-attr/branch/004` | A-033 | reviewed | Completed evaluation is required. |
| `structures/cvc-assess-attr/branch/005` | A-029, A-034 | reviewed | Declaration/type availability gates strict assessment. |
| `structures/sic-a-outcome/branch/001` | A-034 | composite | Strict assessment parent case. |
| `structures/sic-a-outcome/branch/002` | A-034 | reviewed | Strict-valid result. |
| `structures/sic-a-outcome/branch/003` | A-034 | reviewed | Strict-invalid result. |
| `structures/sic-a-outcome/branch/004` | A-034 | reviewed | Not-strict result. |
| `structures/sic-a-outcome/branch/005` | A-034 | reviewed | Attempted full. |
| `structures/sic-a-outcome/branch/006` | A-034 | reviewed | Attempted none. |
| `structures/sic-attr-error-code/branch/001` | A-035 | reviewed | Invalid gives a list, not necessarily nonempty. |
| `structures/sic-attr-error-code/branch/002` | A-035 | reviewed | Valid gives absent. |
| `structures/sic-attrType/branch/001` | A-039 | reviewed | Retained unassessed initial-value branch; E1-56 validity gate is separately A-037. |
| `structures/sic-attrType/branch/002` | A-039 | reviewed | Retained unassessed anySimpleType facts. |
| `structures/a-props-correct/branch/001` | A-020, A-025, A-026 | reviewed | Property ranges, namespace and scope. |
| `structures/a-props-correct/branch/002` | A-021 | reviewed | Canonical constraint validity, including an isolated counterexample. |
| `structures/a-props-correct/branch/003` | A-022 | reviewed | ID-derived value-constraint prohibition. |
| `structures/declare-element/branch/001` | A-048 | reviewed | Empty effective block. |
| `structures/declare-element/branch/002` | A-048 | reviewed | Effective #all. |
| `structures/declare-element/branch/003` | A-048 | reviewed | Every subset and inherited/explicit override. |
| `structures/src-element/branch/001` | A-047 | reviewed | Exclusive default/fixed. |
| `structures/src-element/branch/002` | A-045, A-046 | composite | Local-only conjunction. |
| `structures/src-element/branch/003` | A-045 | reviewed | Exactly one name/ref. |
| `structures/src-element/branch/004` | A-046 | reviewed | Every ref-forbidden child/attribute independently. |
| `structures/src-element/branch/005` | A-047 | reviewed | Exclusive explicit/inline type. |
| `structures/src-element/branch/006` | A-044, A-052, A-053, A-097, A-098, A-099 | composite | Particle and component constraints remain required. |
| `structures/cvc-elt/branch/001` | A-054 | reviewed | Declaration availability. |
| `structures/cvc-elt/branch/002` | A-055 | reviewed | Abstract declaration is not rescued by nil/xsi:type. |
| `structures/cvc-elt/branch/003` | A-056, A-057, A-058, A-059 | composite | Nil cases and their independent premises. |
| `structures/cvc-elt/branch/004` | A-056 | reviewed | Nonnillable forbids presence, even false. |
| `structures/cvc-elt/branch/005` | A-057, A-058, A-059 | composite | True-nil conjuncts. |
| `structures/cvc-elt/branch/006` | A-058 | reviewed | Character/element children versus comment/PI. |
| `structures/cvc-elt/branch/007` | A-059 | reviewed | True nil cannot carry fixed constraint. |
| `structures/cvc-elt/branch/008` | A-060, A-061, A-062, A-063 | composite | xsi:type conjunction. |
| `structures/cvc-elt/branch/009` | A-060 | reviewed | QName lexical/binding failure. |
| `structures/cvc-elt/branch/010` | A-060 | reviewed | Existing versus missing type. |
| `structures/cvc-elt/branch/011` | A-061, A-062, A-063 | reviewed | Simple/complex derivation and complete block union. |
| `structures/cvc-elt/branch/012` | A-064, A-065, A-066, A-067, A-068 | composite | Defaulted versus ordinary-content cases. |
| `structures/cvc-elt/branch/013` | A-065, A-066 | composite | Constraint present, no character/element children, not nilled. |
| `structures/cvc-elt/branch/014` | A-066, A-097 | reviewed | Default must be valid for actual local type. |
| `structures/cvc-elt/branch/015` | A-059, A-066 | reviewed | Type validity independently includes attribute requirements. |
| `structures/cvc-elt/branch/016` | A-064, A-067, A-068 | composite | No constraint, explicit children, or true-nil case. |
| `structures/cvc-elt/branch/017` | A-064, A-068 | reviewed | Actual-type validation. |
| `structures/cvc-elt/branch/018` | A-059, A-067 | composite | Nonnilled fixed-content conjunction. |
| `structures/cvc-elt/branch/019` | A-067 | reviewed | Fixed content excludes all element children. |
| `structures/cvc-elt/branch/020` | A-067 | composite | Mixed versus simple-content fixed equality. |
| `structures/cvc-elt/branch/021` | A-067 | reviewed | Mixed compares initial characters. |
| `structures/cvc-elt/branch/022` | A-067 | reviewed | Simple-content typed equality; published wording tension recorded. |
| `structures/cvc-elt/branch/023` | A-071 | reviewed | Identity constraints still apply under nil. |
| `structures/cvc-elt/branch/024` | A-072 | reviewed | ID/IDREF root closure, including subtree boundary. |
| `structures/cvc-type/branch/001` | A-070, A-074 | reviewed | Actual type must be available. |
| `structures/cvc-type/branch/002` | A-062, A-070 | reviewed | Actual complex type must be concrete. |
| `structures/cvc-type/branch/003` | A-064, A-069, A-070 | composite | Simple versus complex case distinction. |
| `structures/cvc-type/branch/004` | A-068, A-069, A-070 | composite | Simple-type conjuncts. |
| `structures/cvc-type/branch/005` | A-069 | reviewed | Four and only four special xsi attributes. |
| `structures/cvc-type/branch/006` | A-070 | reviewed | No element children; comments/PIs ignored. |
| `structures/cvc-type/branch/007` | A-057, A-068, A-070 | reviewed | Simple value check unless true-nil branch applied. |
| `structures/cvc-type/branch/008` | A-059, A-064 | reviewed | Complex type delegation includes attributes and content. |
| `structures/cvc-id/branch/001` | A-072 | reviewed | Zero binding: forward/missing/list/subtree witnesses. |
| `structures/cvc-id/branch/002` | A-072 | reviewed | Multiple bindings: duplicate IDs. |
| `structures/cvc-assess-elt/branch/001` | A-073, A-074, A-075, A-076, A-077 | composite | Declaration/type route alternatives. |
| `structures/cvc-assess-elt/branch/002` | A-073, A-074, A-077 | composite | Declaration route conjunction. |
| `structures/cvc-assess-elt/branch/003` | A-073 | composite | Known-declaration alternatives. |
| `structures/cvc-assess-elt/branch/004` | A-073 | reviewed | Processor-stipulated declaration. |
| `structures/cvc-assess-elt/branch/005` | A-073 | reviewed | Context-determined declaration. |
| `structures/cvc-assess-elt/branch/006` | A-073 | composite | Lookup conjunction. |
| `structures/cvc-assess-elt/branch/007` | A-073, A-079 | reviewed | Skip blocks lookup. |
| `structures/cvc-assess-elt/branch/008` | A-073 | reviewed | Expanded-name declaration resolution. |
| `structures/cvc-assess-elt/branch/009` | A-077 | reviewed | Declaration validity actually evaluated. |
| `structures/cvc-assess-elt/branch/010` | A-074 | reviewed | Missing actual type prevents strict assessment. |
| `structures/cvc-assess-elt/branch/011` | A-075, A-076, A-077 | composite | Type route conjunction. |
| `structures/cvc-assess-elt/branch/012` | A-075, A-076 | composite | Known-type alternatives. |
| `structures/cvc-assess-elt/branch/013` | A-075 | reviewed | Processor-stipulated type. |
| `structures/cvc-assess-elt/branch/014` | A-060, A-075, A-076 | composite | Usable xsi:type conjunction. |
| `structures/cvc-assess-elt/branch/015` | A-075 | reviewed | Presence of xsi:type. |
| `structures/cvc-assess-elt/branch/016` | A-060, A-076 | reviewed | QName validity is required, not truthiness. |
| `structures/cvc-assess-elt/branch/017` | A-060, A-075 | reviewed | Type definition resolution. |
| `structures/cvc-assess-elt/branch/018` | A-076 | reviewed | Derivation relative to stipulated type and its blocks. |
| `structures/cvc-assess-elt/branch/019` | A-075, A-076, A-077 | reviewed | Evaluate selected local or stipulated type. |
| `structures/cvc-assess-elt/branch/020` | A-078, A-079, A-080 | reviewed | Every child/attribute assessment and skip/lax distinctions. |
| `structures/sic-e-outcome/branch/001` | A-080 | composite | Strict-assessment parent case. |
| `structures/sic-e-outcome/branch/002` | A-078, A-080 | composite | All validity requirements conjunctively. |
| `structures/sic-e-outcome/branch/003` | A-075, A-080 | composite | Declaration/type local-valid alternatives. |
| `structures/sic-e-outcome/branch/004` | A-064, A-080 | reviewed | Declaration-based local validity. |
| `structures/sic-e-outcome/branch/005` | A-075, A-080 | reviewed | Type-based local validity. |
| `structures/sic-e-outcome/branch/006` | A-078, A-080 | reviewed | Invalid descendant veto. |
| `structures/sic-e-outcome/branch/007` | A-080 | reviewed | mustFind+notKnown veto, unlike lax unknown. |
| `structures/sic-e-outcome/branch/008` | A-080 | reviewed | Strict but failed conjunction gives invalid. |
| `structures/sic-e-outcome/branch/009` | A-079, A-080 | reviewed | Not strictly assessed gives notKnown. |
| `structures/sic-e-outcome/branch/010` | A-080 | reviewed | Strict plus all descendant attempts full. |
| `structures/sic-e-outcome/branch/011` | A-079 | reviewed | Not strict plus all descendant attempts none. |
| `structures/sic-e-outcome/branch/012` | A-079, A-080 | reviewed | Mixed assessment gives partial. |
| `structures/sic-elt-error-code/branch/001` | A-081 | reviewed | Invalid gives a list, possibly empty. |
| `structures/sic-elt-error-code/branch/002` | A-081 | reviewed | Valid gives absent. |
| `structures/sic-eltType/branch/001` | A-084, A-085, A-086, A-088, A-089, A-093 | superseded | E1-56 replaces old normalized-value case 1, including default exclusion; current ordered cases are reviewed. |
| `structures/sic-eltType/branch/002` | A-084, A-088, A-093 | superseded | E1-56 replaces full case distinction; current absent outcomes include nil, mixed/empty, and failed value. |
| `structures/sic-eltDefault/branch/001` | A-085, A-086, A-087, A-093 | superseded | E1-56 changes schema-specified gate to clause-5.1 application and deletes canonical-value assertion. |
| `structures/sic-eltDefault/branch/002` | A-087, A-088 | reviewed | Infoset fallthrough retained after corrected gate. |
| `structures/e-props-correct/branch/001` | A-050, A-052 | reviewed | Property ranges, defaults, scope and namespace. |
| `structures/e-props-correct/branch/002` | A-097 | reviewed | Default validity for every admitted content kind. |
| `structures/e-props-correct/branch/003` | A-098 | reviewed | Affiliation requires global scope. |
| `structures/e-props-correct/branch/004` | A-098, A-105, A-106 | reviewed | Affiliation type derivation/final; same-type #all tension explicit. |
| `structures/e-props-correct/branch/005` | A-053 | reviewed | Direct and simple-content ID-derived prohibition. |
| `structures/e-props-correct/branch/006` | A-099 | reviewed | Self/two-node cycles versus acyclic affiliation. |
| `structures/cos-valid-default/branch/001` | A-021, A-097 | reviewed | Simple type validation; canonical spelling distinction. |
| `structures/cos-valid-default/branch/002` | A-097 | composite | Complex-type conjunction. |
| `structures/cos-valid-default/branch/003` | A-097 | reviewed | Only simple-content or mixed, not merely emptiable. |
| `structures/cos-valid-default/branch/004` | A-097 | composite | Simple-content versus mixed cases. |
| `structures/cos-valid-default/branch/005` | A-097 | reviewed | Simple-content value validity. |
| `structures/cos-valid-default/branch/006` | A-097 | reviewed | Mixed particle emptiability. |
| `structures/cos-equiv-derived-ok-rec/branch/001` | A-100 | reviewed | Identity before blocking. |
| `structures/cos-equiv-derived-ok-rec/branch/002` | A-101, A-102, A-103 | composite | Distinct-declaration conjunction. |
| `structures/cos-equiv-derived-ok-rec/branch/003` | A-101 | reviewed | Substitution block. |
| `structures/cos-equiv-derived-ok-rec/branch/004` | A-100, A-102 | reviewed | Directed transitive affiliation chain. |
| `structures/cos-equiv-derived-ok-rec/branch/005` | A-061, A-103 | reviewed | All derivation steps and complex intermediate prohibitions. |
| `structures/cos-equiv-class/branch/001` | A-104 | reviewed | Head belongs to potential group. |
| `structures/cos-equiv-class/branch/002` | A-102, A-104 | reviewed | Potential group transitive closure. |
| `structures/cos-equiv-class/branch/003` | A-104 | reviewed | Actual group removes abstract declarations. |
| `structures/cos-equiv-class/branch/004` | A-101, A-103, A-104 | reviewed | Actual group additionally requires substitutability. |
