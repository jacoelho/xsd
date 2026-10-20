# Bounded products of dependent XSD 1.0 decisions

This catalogue expands the mandatory products in
[Structures](structures.md#required-interactions), [Datatypes](datatypes.md),
and the [concrete branch layer](branches/README.md). Its expected results come
from the [dated specifications](../corrected-reading.md), with E1-56 applied;
[interpretation records](../INTERPRETATIONS.md) remain distinct from errata.
No implementation or corpus behavior supplies an oracle.

An IP case is a **finite product description**, not one executed fixture.
Every listed axis takes every value in its domain, jointly, subject only to
its explicit exclusions. The outcome rules define each retained tuple; the
concrete witness and mutations make those rules reviewable. This is stronger
than a pairwise sample and smaller than printing every serial spelling of the
same tuple. It does not assert that finitely many examples exhaust unbounded
schemas, values, or recursive depths.

## Shared sketches and product evaluation

Use `xs=http://www.w3.org/2001/XMLSchema`,
`xsi=http://www.w3.org/2001/XMLSchema-instance`, `p=urn:a`, and `q=urn:a`.
Unless a product states otherwise, schema and instance element names have
**no namespace** and schema unprefixed QNames have no default namespace.
Every stated source document/component is supplied explicitly. No unmentioned
constraint, wildcard, attribute, identity constraint, block/final, or abstract
property exists. Root assessment covers the complete shown document.

```xml
<xs:simpleType name="NN"><xs:restriction base="xs:integer">
  <xs:minInclusive value="0"/>
</xs:restriction></xs:simpleType>
<xs:simpleType name="IS"><xs:union memberTypes="xs:integer xs:string"/></xs:simpleType>
<xs:simpleType name="SI"><xs:union memberTypes="xs:string xs:integer"/></xs:simpleType>
<xs:simpleType name="IB"><xs:union memberTypes="xs:integer xs:boolean"/></xs:simpleType>
<xs:simpleType name="IL"><xs:list itemType="xs:integer"/></xs:simpleType>
<xs:complexType name="SC"><xs:simpleContent><xs:extension base="xs:integer">
  <xs:attribute name="a" type="xs:integer"/>
</xs:extension></xs:simpleContent></xs:complexType>
<xs:complexType name="ReqSC"><xs:simpleContent><xs:restriction base="SC">
  <xs:attribute name="a" type="xs:integer" use="required"/>
</xs:restriction></xs:simpleContent></xs:complexType>
<xs:complexType name="M" mixed="true"><xs:sequence>
  <xs:element name="x" type="xs:string" minOccurs="0"/>
</xs:sequence></xs:complexType>
<xs:complexType name="EO"><xs:sequence>
  <xs:element name="x" type="xs:string" minOccurs="0"/>
</xs:sequence></xs:complexType>
<xs:complexType name="Empty"/>
```

`R(T,B; f=v,...)` is one named simpleType restriction of B containing the exact
listed facet elements `<xs:f value="v"/>`. `fixed` on a facet is written
explicitly when intended. `L(T,I)` is a list type with itemType I; `U(T,A B)`
is a union with that ordered memberTypes sequence. Names refer to the exact
components above or to components defined by the particular product.

**SC** observes schema correctness; **V** local/aggregate instance validity;
**P** component or PSVI properties; **E** typed identity/order; **K** canonical
representation where exposed; **O** processor/invocation policy. **Q** is an
unresolved or qualified source oracle. `+`/`−` attach to SC/V when determined.
An invalid schema has no subsequent V claim. An XML-invalid document has no
XSD-validity claim. A missing P/K observation does not mean a pass.

For every retained tuple record the result as a vector, not one boolean:

1. XML/namespace prerequisites and schema correctness.
2. Declaration, actual type, particle/use, and assessment route.
3. Normalized literal, typed value/member, and independent validity predicates.
4. Aggregate outcome and each applicable E1-56 contribution.
5. Masking, unavailable observations, and source qualification.

A false conjunct does not excuse checking the other defined conjuncts. Where
an earlier predicate makes a later value undefined, record **unreached/undefined**
instead of inventing its value. Diagnostic ordering is not implied. A tuple
excluded because its XML representation cannot express a component change
remains an abstract relation case only when that boundary is explicitly named.

## IP-001 — Element default/fixed × actual type × nil × content × corrected PSVI

Authority: [Element Locally Valid](../upstream/structures.html#cvc-elt),
[Element Default Valid](../upstream/structures.html#e-props-correct),
[E1-56](../corrected-reading.md), and [A-051–A-071](branches/declarations.md).

Every schema-admitted tuple that reaches instance assessment has a known
element declaration and its local validity under `cvc-elt` assessed, whether
valid or invalid. This supplies
[Element Default Value's entry premise](../upstream/structures.html#sic-eltDefault).
The opposing type-only route without declaration-local assessment belongs to
[E-110](../corrected-reading.md#scenarios-specific-to-the-correction), outside
this product's axes; its `infoset` rule must not be extrapolated to that route.

| Axis | Finite domain |
| --- | --- |
| Declared category | xs:integer; IB; SC; M; EO; Empty |
| Actual type | Declared type; a legal restriction as below |
| Declaration constraint | None; default `+001`; default `-1`; fixed `+001` |
| nillable / nil attribute | false/true × absent/`false`/`true` |
| Character/element children | Empty; comment+PI only; one space; `+001`; `1`; `2`; `oops`; one `<x/>` |
| Required attribute on actual type | For SC actual ReqSC: `a="2"`; absent; `a="oops"`. For all other cells: no attribute. |

The legal restriction is NN for integer; R(IBdigits,IB; pattern=`[+]?[0-9]+`)
for IB; ReqSC for SC. For M use mixed restriction with an empty sequence
(removing optional x); for EO use restriction with empty content, whose actual
category is **Empty**, not element-only. For Empty use a vacuous empty
restriction. Evaluate the resulting actual category rather than copying the
declared category. The product does not presume unrelated xsi:type is legal;
that separate full selection product is IP-008.

**Schema exclusions.** EO/Empty with default or fixed is SC−: neither is a
simple/simple-content type or an emptiable mixed type. That is a classified
schema outcome, not an instance test. No ordinary `a` attribute is supplied
on simple-typed elements; introducing it would test a different rule.
Every constraint literal above is valid for its declared integer/IB/SC/M type.

**Complete outcome rules.**

| Predicate/contribution | Rule for every retained tuple |
| --- | --- |
| Nil admission | With nillable=false, **any present xsi:nil**, including false, violates the declaration rule. With nillable=true and nil=true, character or element children violate nil emptiness; comment/PI alone do not. Any fixed constraint also forbids true nil. |
| Applied value constraint | It applies exactly when constraint is present, no character/element children exist, and true-nil clause 3.2 did not apply. Empty and comment/PI use this route; a space does not. |
| Type/default validity | On the applied route, retain cvc-elt 5.1's **canonical-representation** validation premises and enforce the actual type's attributes; E1-56 changes the P contribution to lexical form, not those premises. Thus default -1 fails NN; default +001 has canonical integer1, admitted by IBdigits. Missing/invalid a fails ReqSC independently of valid default text. |
| Ordinary content validity | Otherwise use the actual type and shown content. Integer/SC accept +001,1,2 but not space/oops/element children. IB also rejects oops. M admits text and its declared optional x; its restricted form rejects x. Unrestricted EO admits XML whitespace around permitted x; EO's Empty restriction and declared Empty admit neither characters nor elements, including whitespace. True nil bypasses content-value checking, **not attributes/identity**. |
| Explicit fixed comparison | When nonnil fixed content is explicit, no element child is allowed. Integer/SC/IB's integer member compare typed value to 1; +001 and 1 agree, 2 does not. Mixed content uses the prescribed initial-character comparison; `1` does not equal lexical +001. Preserve the fixed-value wording qualification in the branch catalogue for ambiguous typed/canonical readings. |
| Corrected normalized value | Retain sic-eltType's unchanged outer type-validity premise. Where applicable, its ordered property cases are: true nil absent; non-simple/non-simple-content actual category absent; applied constraint lexical form; otherwise successfully validated simple value as normalized; else absent. A declaration-only fixed/identity failure need not defeat successful type validation. Invalid lexical/content values or complex-type attributes do defeat it: the inner cases alone require neither presence nor absence outside that entry; retain the branch catalogue's qualified P oracle for those cells. |
| Corrected schema-specified | Within the declaration-local-assessment entry stated above, `schema` exactly when clause 5.1 applied; otherwise `infoset`. Explicit value equal to fixed does not mean the schema supplied it. |
| Member P | Within the contributor's entry, the amended lightweight paragraph requires successful validation of the resulting nonabsent schema-normalized value. Preserve the unchanged full-member wording and [PSVI-Q001's scope qualification](source-questions.md#psvi-q001--union-member-amendment-sites-and-contributor-scope); type-invalid cells do not gain a universal presence/absence oracle from the inner cases. |
| Children | Applying a default/fixed element value creates no character children. |

**Concrete witness.** v declared xs:integer, default=+001, nillable=true,
actual declared, empty body: V+, normalized P `+001`, specified=`schema`, no
character child. Mutate only body to `1`: V+, specified=`infoset`. Mutate only
the empty witness to xsi:nil=true: V+, normalized value absent, specified=
`infoset`. Add a space to that nilled element: V− nil emptiness. Use NN actual
and default=-1: V− actual-default validity. Use SC→ReqSC, default=+001, omit a:
V− attributes although the simple content value is valid; its mandatory P
contribution is not settled by that content fact. Contrast unrestricted EO
with its Empty restriction using one space: only the former admits it.
These distinguish independent conjuncts and actual content categories.

**Masking/observation.** Capture local type/value validity and P independently
of aggregate V. For EO/Empty constraint tuples stop at SC−. For nil=true with
fixed stop short of claiming a valid nilled value. Empty strings, absent
properties, default text, and whitespace text are different axis values.

### Parsed entity versus character-element companion

This bounded companion fixes the relevant parent/child declarations and XML
processing premises. It connects [Primer Appendix C](../upstream/primer.html#usingEntities),
[XML entity replacement](../dependencies/xml-1.0-second-edition.html), mixed
content and the corrected child value contribution; it does not expand the
numeric/type axes above.

First declare `city` as `xs:string`. Literal `Montréal`, numeric
`Montr&#xE9;al`, and the following internal parsed-entity form yield the same
character sequence before XSD when the declaration is read:

```xml
<!DOCTYPE city [<!ENTITY eacute "&#xE9;">]>
<city>Montr&eacute;al</city>
```

An undeclared reference is an XML failure. Failure or lack of support at the
XML/DTD-processing boundary does not establish XSD string invalidity. No
external entity retrieval is needed.

For the character-element alternative, supply `characters.xsd` explicitly:

```xml
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:char">
  <xs:element name="eacute" type="xs:token" fixed="é"/>
</xs:schema>
```

Supply a second schema with no target namespace:

```xml
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:c="urn:char">
  <xs:import namespace="urn:char" schemaLocation="characters.xsd"/>
  <xs:element name="city">
    <xs:complexType mixed="true">
      <xs:sequence>
        <xs:element ref="c:eacute" minOccurs="0" maxOccurs="1"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>
```

| Instance under the mixed declaration | Validity and distinct facts |
| --- | --- |
| `<city>Montréal</city>` | Valid; parent character children contain the literal word and there is no character element. |
| `<city xmlns:c="urn:char">Montr<c:eacute/>al</city>` | Valid; the empty child uses fixed value é under the applicable corrected contribution but receives no character child. Parent retains character text, child element, character text. |
| `<city xmlns:c="urn:char">Montr<c:eacute>é</c:eacute>al</city>` | Valid; explicit child content equals fixed é. Its character child exists. |
| `<city xmlns:c="urn:char">Montr<c:eacute>x</c:eacute>al</city>` | Invalid from the child's unequal fixed value. |

The mixed parent has no schema-normalized-value contribution. Child fixed
PSVI is not entity replacement and is not inserted or concatenated into the
parent's character children. Replace only the `city` declaration with
`<xs:element name="city" type="xs:string"/>`, keeping the import and child
declaration available: the same character-element form fails parent
element-child admission. Declaring/importing that child does not make its
simple-typed parent mixed. Observe XML text, parent admission, child fixed/type
validity, applicable child PSVI and actual child lists separately. A validity-only
interface cannot establish every tree/PSVI fact; no string API or serializer is
prescribed. These are source-derived descriptions, not executed corpus cases.

## IP-002 — Attribute declaration/use × presence × normalization × constraints

Authority: [Attribute Locally Valid](../upstream/structures.html#cvc-attribute),
[Attribute Use](../upstream/structures.html#cvc-au),
[attribute PSVI](../upstream/structures.html#sic-attrType),
[E1-56](../corrected-reading.md), [B-043–B-044](branches/content-and-derivation.md).
Use root v with empty complex content and one attribute use a.

| Axis | Domain |
| --- | --- |
| Source | Local declaration; reference to one global declaration; referenced attribute group; inherited unchanged use |
| Use | Optional; required; prohibited |
| Effective constraint | None; default `+001`; fixed `+001` |
| Constraint owner | Explicit use only; global declaration only with absent use constraint; both equal fixed values (where permitted) |
| Type | xs:integer; xs:string; xs:token; IS; SI; IL |
| Presence/value | Absent; empty; `+001`; `1`; `  1  `; `2`; `oops`; `1 2` |
| Independent parent error | None; unrelated required b missing (b is declared integer) |

**Exclusions and preconditions.** An explicit local/use `default` attribute
with required/prohibited `use` is SC−, not a defaulted required attribute.
A global declaration default with an otherwise required reference use has no
local default attribute and is a different source cell: it is not rejected
by that representation predicate; an absent required attribute still fails.
Prohibited local
uses are a restriction mechanism and contribute no admitted runtime use;
otherwise-forbidden declaration attributes are SC− before V. IL constraint
+001 denotes a one-item list and is legal. A referenced global declaration's
constraint contributes when the use has no overriding constraint; a fixed
declaration cannot be overridden by a different constraint/value. That
source-by-constraint construction cross-check is IP-011.

**Oracle for the full product.** A present attribute under prohibited use is
V− unless a separately admitted wildcard route owns it, which this product
excludes. Absent required is V−. Absent optional with a nonabsent **use** constraint contributes
a schema-specified attribute using the specified canonical normalized/default
string; absent optional without an effective constraint contributes none.
A declaration-only default with absent use constraint is the text-integration
Q recorded in B-044: the synthesis predicate names the use constraint while
the effective-constraint definition permits declaration fallback. Do not
silently add a mapping step to settle that cell. A present attribute validates using its
type's whitespace/member/list rules, then each applicable fixed comparison **at its own boundary**. Declaration
fixed uses typed equality; published cvc-au additionally requires the normalized
spelling to match the use fixed value's canonical lexical representation.
Default does not replace a present empty/invalid value. Present integer +001/1/whitespace-1
are E=1; string spellings differ; token whitespace-1 becomes `1`; IS selects
integer for numeric forms; SI selects string first; IL uses item sequences.
For declaration-fixed +001, SI/string require selected string +001 while
integer/IS compare equal values. For **use-fixed** integer +001, explicit
normalized `1` matches canonical1, but explicit `+001` fails cvc-au despite
declaration equality; IS's integer and IL's one-integer canonical form have
the same spelling distinction. SI/string canonical +001 requires that exact
string. IL's two-item `1 2` also differs in typed sequence value.

The independent parent-b error makes aggregate V− while a's successful
normalized-value/member P remains present under E1-56. A type-invalid a has
no successful member/normalized P even if another member was tentatively tried.
Synthesized attribute presence, source=`schema`, schema-default, and normalized
value are separate observations; there is no element-style insertion of text.

**Witness and discriminators.** Optional integer a default=+001, `<v/>`:
V+, added a with value 1 and schema source. `<v a=""/>`: V−, no fallback to
default. Use-fixed integer a=+001 accepts `<v a="1"/>`; explicit
`<v a="+001"/>` has equal typed value yet fails its use-level canonical-string
comparison. Use-fixed SI a=+001 rejects `a="1"` because selected string1
differs from +001. Add missing required b:
aggregate V− while a's type-valid P survives. These mutations require no change
to declared namespaces or content.

## IP-003 — Union/list selection × lexical facets × value facets × typed tuples

Authority: [union](../upstream/datatypes.html#union-datatypes),
[list](../upstream/datatypes.html#list-datatypes),
[pattern](../upstream/datatypes.html#rf-pattern),
[enumeration](../upstream/datatypes.html#rf-enumeration),
[identity](../upstream/structures.html#cvc-identity-constraint).

| Axis | Domain |
| --- | --- |
| Construction | IS; SI; IL; list of union(integer,boolean); union(IL,xs:string); union(xs:string,IL) |
| Restriction step | None; one local enumeration of literal `1`; one local pattern `[0-9 ]+`; both |
| Candidate | `1`; `01`; ` 1 `; `-1`; `true`; `1 2`; `1 true`; empty; `x` |
| Context | Ordinary v content; attribute field @a in unique with another row @a=`1` |
| Extra item restriction | None; item NN, only where integer/list construction permits it |

**Exclusions.** No length/numeric/whiteSpace facet is added directly to a
union. List facets see the whole normalized sequence; item facets see each
item. A list whose item union includes IL is SC−, not another valid row.
An enumeration literal must be type-valid for the chosen base in its own
context; failed construction is reported SC−. These exact bases all admit 1,
so their stated enumeration is legal. The optional NN substitution is a
separate listed constructor mutation, not a change to IL's inherited item
property inside XML restriction.

**Complete oracle.** First derive ordered member selection, member-specific
normalization, and all item values. Apply local pattern to the normalized
whole literal and enumeration to the selected typed value/sequence. Both
conditions must hold. Same-step repeated patterns, added by IP-016, are OR;
separate restriction steps are AND. Ordinary content V reflects those rules.
In identity context, only successfully typed values supply qualified tuples;
two complete equal tuples violate unique, while unequal primitive values do
not. A string-selected `01` is unequal to string `1`; integer-selected `01`
equals integer 1; IL `01` equals one-item IL `1`; IL `1 2` is not equal.
A no-value outcome never becomes the empty string/empty list by convenience.

**Witness.** IS field values `01` and `1` violate unique. Change only governing
type to SI: they are distinct strings and unique succeeds. Restrict SI by
enumeration literal 1: `01` now fails the type, so the resulting aggregate
failure proves no identity duplicate. IL ` 1 ` and `1` form equal sequences;
pattern `[0-9 ]+` rejects boolean token `true` even in a list-of-union whose
item type accepts it. IL admits -1; replacing its constructor item with NN
rejects -1 before list-level facets. These are four dependent effects, not interchangeable
failure fixtures.

**Observation.** Record selected member/item type and tuple value; merely
observing V− cannot distinguish failed type restriction from duplicate identity.

**Canonical overlap subproduct.** For canonical observations, expand the
following dependent axes under admitted constructors and Part 1 assessment:

| Axis | Domain |
| --- | --- |
| Member values | Disjoint primitive spaces; overlapping members sharing an atomic ancestor |
| Member canonical grammars | Same; different |
| Equal-value lexical inputs | One spelling accepted by overlapping members; distinct equal-value spellings with different member matches |
| Member order | Each order of the compared pair |
| Type override | None; legal member or derived-member override where the governing element assessment permits it |
| Carrier | Direct union; admitted list whose item union has only atomic members |
| Canonical observation | Exposed; unavailable |

The equal-value axis applies only to overlapping spaces; disjoint primitive
controls cannot share a datatype value. Exclude combinations without an
admitted equal-value pair or an applicable type override.

As a question witness, use union(integer,decimal) with `1` and `1.0`: retain
their equal value and actual selected member separately, then reverse member
order and apply only legal overrides. Their differing member canonical grammars
must be joined with the generic one-to-one canonical definition under
[D-072's unresolved qualification](datatypes.md#d-072--questions-and-permissible-choices-that-must-remain-visible).
Assign no new canonical output policy. Preserve the existing disjoint
integer/string controls. For list carriers, examine the item canonical
dependency; `xsi:type` is not a per-token member override. Exclude inadmissible
constructor/type combinations. Unavailable K remains unobserved, and exposed
member metadata alone does not resolve this question.

## IP-004 — Schema literal QName context × instance context × actual member

Authority: [QName](../upstream/datatypes.html#QName),
[fixed/default values](../upstream/structures.html#Element_Declaration_details),
[E1-56](../corrected-reading.md), [D2-013/D2-029](branches/datatypes.md).

Use schema prefixes p and q bound to urn:a. Define Q=xs:QName,
QS=union(xs:QName,xs:string), SQ=union(xs:string,xs:QName). The owning
declaration/use has fixed/default literal `p:n` under the schema's bindings.

| Axis | Domain |
| --- | --- |
| Type/member order | Q; QS; SQ |
| Constraint owner and assessment | Element declaration; global attribute assessed through a non-skip wildcard with no use constraint; local attribute use with explicit use constraint for fixed/default |
| Constraint | Enumeration p:n; fixed p:n; default p:n |
| Candidate context | p=urn:a,q=urn:a; p=urn:b,q=urn:a; p unbound,q=urn:a |
| Present candidate | p:n; q:n; n with default namespace urn:a; p:m; empty |
| Presence | Explicit; absent attribute/empty element (for fixed/default only) |

Enumeration is a type restriction; it does not itself create default content.
QName uses the literal's context: schema p:n has value (urn:a,n). Explicit
Q/QS q:n equals it, p:n rebound urn:b does not. Those are typed-value results,
not sufficient conditions for every use-level fixed check. An unbound p fails QName,
but QS can select string if the QName member fails, producing string `p:n`
which is distinct from the schema QName value. SQ selects string first, so
fixed/enumeration p:n compares strings: q:n differs even under equal namespace
bindings. Unprefixed QName n uses the instance default namespace when present;
an unprefixed string n is simply n.

For the element's empty-content constraint application, retain the schema
literal's value/context and E1-56's lexical-form PSVI boundary; do not re-resolve
schema p in an instance that lacks p or binds it differently. Element children
remain absent. Attribute absence uses IP-002's separate synthesis predicate,
canonical contribution and declaration-only fallback qualification.
An absent wildcard-only attribute has no use to synthesize it. An explicit
optional use can supply an attribute; a missing required use still fails.

For a **use-level fixed attribute**, additionally apply cvc-au's normalized-
string/canonical constraint comparison from IP-002/B-043. Part 2 supplies no
context-free canonical QName prefix spelling, so Q/QS selected-QName use cells
retain that canonical-projection source question; typed equality alone is not
an unconditional V+ oracle. SQ selected-string cells use string canonical
spelling and still require the use-level comparison. A changed QName prefix
preserves its value but need not preserve a separately required string check.
The same unavailable canonical QName spelling qualifies the canonical string
contribution of an added QName attribute; it is not E1-56's element lexical
contribution.
Changing union order can change normalization, identity and the applicable
projection boundary.

**Witness.** An **element declaration** with QName fixed p:n accepts instance q:n with q=urn:a and rejects
p:n with p=urn:b. SQ fixed p:n does the opposite spelling discrimination:
p:n is equal as a string even when p=urn:b; q:n is unequal. Remove element
content under default p:n and unbind p only in the instance: the schema-supplied
QName value remains (urn:a,n). A P-only lexical property is not a proof that
instance namespace bindings were used for that value.

**Masking.** If q itself is unbound, a failed QName precondition cannot isolate
fixed inequality. For SQ, namespace binding is irrelevant to the selected
string; do not count those binding permutations as independent QName coverage.

## IP-005 — Identity category × cardinality × defaults × typed value × nil

Authority: [Identity-constraint Satisfied](../upstream/structures.html#cvc-identity-constraint),
[selector/field XPath](../upstream/structures.html#c-fields-xpaths),
[E1-56](../corrected-reading.md), [I-001–I-003](../INTERPRETATIONS.md).
Use root doc permitting repeated r, each with optional repeated v children and
optional a/b attributes. Selector is `r`; fields are either `@a` or ordered
`v,@b`. The schema content model admits every stated node count before the
identity predicate is considered.

| Axis | Domain |
| --- | --- |
| Category | unique; key; keyref to a separately valid key containing tuple 1 or (1,2) |
| Selector population | Zero rows; one row; two distinct row nodes |
| Field cardinality per row | Zero/one/two nodes **independently for each field** |
| Typed field form | xs:integer; xs:string; IL; IS; SI; complex simple content SC; element-only complex |
| Supply | Explicit; defaulted attribute; defaulted empty element; no supply |
| Value pair | `1`/`01`; `1`/`2`; list `1 2`/` 1  2 `; tuple `(1,2)`/`(2,1)` |
| Element field declaration/state | nonnillable+nonnil; nillable+nonnil; nillable+nil=true |
| Assessment | Declared typed; invalid typed literal; field under skip |

**Exclusions.** Attribute XPath cannot select two same-expanded-name attributes
because XML forbids duplicates; use `@*` with two distinct declared attributes
for that cardinality cell, or use repeated v. Nil applies only to element
fields. Defaulted attributes are present after schema contribution, while
absent unconstrained attributes select zero nodes. Empty defaulted v selects
one element, not newly added text nodes. Type/value pairs that fail lexical
admission are retained as type-invalid vectors but cannot prove E equality.
Element-only complex fields are a separate nonsimple-node failure, not a
zero-node selection. Two-field labels use compatible two-field key arity;
wrong keyref arity is SC− and excluded from runtime tuple comparison.

**Full-product oracle.**

| Step | Outcome rule |
| --- | --- |
| XPath/cardinality | Evaluate names using the schema expression's prefix bindings. More than one selected node for **any** field fails the identity rule, even if another field is empty; do not filter incomplete rows first. |
| Value qualification | Zero selected node means missing. One successfully simple-valued node supplies its type-determined value. Derive defaults/member/list values as IP-001–IP-004. SC fields carry the I-001 WG-clarification qualification; skipped/nilled/invalid-value cells carry I-002/I-003 or the lack of a valid value, not fabricated empty strings. |
| unique | Compare only complete qualified tuples. Zero selected rows and rows with genuine missing fields do not by themselves violate unique. Two distinct complete equal tuples violate it; unequal ordered tuples do not. |
| key | Every selected row must have a complete tuple, tuples must be distinct, and no element field may have been assessed through a nillable declaration. Nillable=true therefore fails the last predicate even with nil absent/false and a perfectly good lexical value. |
| keyref | Every complete qualified tuple must occur in the referenced table, member for member and in order. Genuine incomplete rows create no referent obligation. More-than-one-node field failure remains independent. |
| Type failures/source questions | Aggregate invalidity of a field is not proof of a particular qualified-set result. Keep the exact cardinality and unavailable type/value facts; Q where the captured 1.0 interpretation does not decide the intended qualified-value result. |

**Witness.** unique @a over `<doc><r a="01"/><r a="1"/></doc>` with integer
a fails; change a to string and it succeeds. Default integer a=1 with both
a absent creates two equal tuples and fails. Removing the default creates
missing fields: unique succeeds, key fails. With fields v,@b, first row
`<r><v>1</v><v>2</v></r>` fails field cardinality despite missing b.
A single `<v>1</v>` declared nillable=true fails key's declaration condition
without needing an actual nilled value. Nil=true unique/keyref cases stay
source-qualified; the later 1.1 interpretation is not an approved 1.0 erratum.

**Observation.** Log target nodes, per-field node count, successful type/value,
qualified tuple, and category result separately. A V− fixture with both
invalid content and duplicate values does not establish duplicate comparison.

## IP-006 — Identity scope × table propagation × local conflict × keyref order

Authority: [identity tables](../upstream/structures.html#sic-key),
[identity validation](../upstream/structures.html#cvc-identity-constraint),
[C-010–C-014](branches/composition-and-identity.md).
Use one global recursive scope declaration. K on scope selects its **direct**
r children with integer field @id; R on scope selects direct use with field
@rid and refer=K. Recursive child scope uses that same declaration and K
**component**, not another same-named constraint.

| Axis | Domain |
| --- | --- |
| Local K rows in the assessed scope | None; one id=1; two distinct id=1 nodes; one id=2 |
| Child scopes' K table candidates | None; one child id=1; two children each id=1; two children id=1/id=2 |
| Reference placement | Current scope; one child; sibling scope; outside validation root |
| Reference value | 1; 2; missing rid; invalid integer rid=bad |
| Document order | use before relevant rows; use after rows |
| Selector reach | Direct r; `.//r` including descendants |

**Complete oracle.** First compute each scope's own constraint satisfaction
using its selector. Distinct local equal rows violate K; separate child keys
can each be locally valid. Assemble the current table from local entries and
propagated tables of the **same K component**, preserving selected-node
identity. Conflicting inherited entries for distinct nodes are removed;
a local entry wins over inherited conflicting candidates; duplicate routes to
one identical pair are not distinct-node conflicts. Then each complete R tuple
must find an entry in its own scope's assembled table. Propagation is upward,
not a global or downward membership set. Document order does not change the
logically assembled table. A sibling's key does not become visible inside
the other child merely because their parent can see both. `.//r` can create
local duplicates where direct r did not; apply that change before propagation.

**Witness.** `<scope><scope><r id="1"/></scope><use rid="1"/></scope>`
succeeds through child-table propagation. Add a second child scope with its
own r id=1: each child K succeeds but parent use fails because both inherited
candidates conflict and are removed. Add one **local** parent r id=1:
parent use succeeds through the local row. Add a second local r id=1:
parent K now fails independently. Move use inside an empty sibling scope:
it cannot see the other sibling's row. A forward use at the parent succeeds
exactly as a backward use when the final assembled table is the same.

**Masking/exclusions.** Invalid rid does not supply a successfully typed
reference value; it cannot prove missing-reference behavior. Replacing K
with a distinct component of the same spelling changes the setup and is a
resolution/component-identity failure, not table propagation. Restricting
assessment to a child excludes outside ancestor/sibling rows from that episode.

## IP-007 — ID declaration eligibility × wildcard processing × multiplicity × root

Authority: [ID context](../upstream/structures.html#sic-id),
[root ID/IDREF](../upstream/structures.html#cvc-id),
[complex wildcard ID limits](../upstream/structures.html#cvc-complex-type),
[B-121/C-054–C-056](branches/composition-and-identity.md).
Use declared item/use elements, optional declared local x:ID, and optional
wildcard attributes `{urn:u}i` and `{urn:u}j` with global ID declarations.
IDREF/IDREFS references occur on a separate declared use element.

| Axis | Domain |
| --- | --- |
| Declared local ID use | Absent; optional absent in instance; optional x=a present |
| Wildcard processing | strict; lax; skip |
| Wildcard referent | Global ID declaration; ID restriction; xs:string; missing declaration |
| Wild attributes present | None; i=a; i=a and j=b |
| Reference | None; IDREF a; IDREF b; IDREFS `a b a` |
| Assessment root | Entire doc; subtree containing only use |
| Candidate lexical quality | Valid NCName a/b; invalid `p:a` |

**Oracle.** Namespace admission comes first. Strict/lax with available ID
referent assess and can contribute a wild ID; skip does not do so merely
because declarations exist. Missing strict referent fails assessment; missing
lax referent supplies no successful ID-typed value. More than one assessed
wild ID fails the per-type rule; one assessed wild ID also conflicts with
**any declared ID use**, even if that optional use is absent. At most one
ID-derived attribute use is a separate schema constraint.

The root ID table only uses eligible successfully validated items within
its validation episode, with declaration-type eligibility as specified.
A governing union that selects an ID member does not automatically satisfy
a predicate naming an ID/derived-ID **declaration type**. IDREFS expands to
its individual keys a and b, deduplicating its table keys, not one literal
`a b a` key. References need exactly one owner element in the root table;
zero/multiple owners fail. Values outside a subtree root do not repair it.
Skipped/invalid lexical items are not successful eligible values.

**Witness.** No declared x, strict wildcard, global i:ID, only i=a and use
ref=a: success. Add global j:ID and j=b: V− wild-ID count despite distinct
values. Instead add optional declared x:ID but leave x absent: i=a now fails
the declared-use conflict. Under skip with no declared x, i=a is not an ID
binding; use ref=a is unresolved at root. Under whole-doc assessment a separate
declared item id=a repairs the reference; a subtree containing only use does
not see that owner. This separates local ID-use constraints from root closure.

**Observation.** Per-type wild-ID set, declaration-type eligibility, root-table
key and owner set are distinct. Two eligible ID-valued child nodes under one
owner are not automatically two distinct owner elements; preserve the source's
set-of-owners definition rather than counting lexical occurrences.

## IP-008 — xsi:type ancestry × block/final × abstractness × nil/attributes

Authority: [actual type](../upstream/structures.html#cvc-elt),
[complex derivation](../upstream/structures.html#cos-ct-derived-ok),
[simple derivation](../upstream/structures.html#cos-st-derived-ok),
[extension ancestry](../upstream/structures.html#cos-ct-extends).
Let B have optional x:string child and required a:integer attribute. E extends
B by optional y; R restricts B by removing optional x while retaining required
a; ER restricts E to optional x and required a. All are named concrete types
unless the abstract axis changes the selected type.

| Axis | Domain |
| --- | --- |
| Requested actual type | Absent xsi:type; B identity; E extension; R restriction; ER extension+restriction; unrelated xs:string; unbound/malformed QName; missing type |
| Element block | Empty; extension; restriction; both |
| B prohibited substitutions | Empty; extension; restriction; both |
| Construction final at traversed base | Empty; matching next method prohibited |
| Actual abstract | false; true (complex types only) |
| nil | Absent; true on a nillable declaration with no fixed constraint |
| a/content | a=1/absent/a=bad × empty/x/y/x-then-y/y-then-x |

**SC/representation exclusions.** If final blocks construction of a needed
edge, the schema is SC− before xsi:type. Abstract B can legally be a base;
abstract simple types do not exist. Malformed/unbound xsi:type is not an
unrelated-but-resolved type. Four builtin xsi attributes do not require ordinary
attribute uses; unrelated ordinary attributes do.

**Full oracle.** Resolve xsi:type and require valid derivation to B given
element block union B's applicable prohibited substitutions. Identity B→B
traverses no nontrivial edge and succeeds despite blocked methods. E uses
extension, R restriction, ER both; **every** traversed edge matters. Unrelated
or unresolved requests fail actual-type selection. If successful, actual type
must be concrete; validate its required a and content. Nil=true does not waive
abstract-type, derivation, required-attribute, or identity conditions, though
it suppresses ordinary content-value checking and requires no character/element
children. A y-only body is not valid for B/R but can be valid for E. A y-then-x
body violates E's extension ordering. P records the actual selected type only
under the source's successful selection route; retain the declared-type fallback
and overall failed clause separately when xsi:type fails.

**Witness.** v:B with a=1, xsi:type=E, `<y/>` succeeds under empty controls.
Set only element block=extension: selection fails. Set block=restriction
instead: E remains permitted but ER fails. Request B identity under block=#all:
identity still succeeds. Make E abstract: xsi:type=E fails; use concrete E2
extending E when legal and supply its required a to restore a concrete actual
type. True nil with absent a fails despite no content. Those changes isolate
construction, derivation, abstractness, attributes, and content.

## IP-009 — Substitution affiliation × controls × abstract filter × UPA/consistency

Authority: [substitution group](../upstream/structures.html#cos-equiv-class),
[actual substitution](../upstream/structures.html#cos-equiv-derived-ok-rec),
[UPA](../upstream/structures.html#cos-nonambig),
[element consistency](../upstream/structures.html#cos-element-consistent).
Use global head h:B, member m:E affiliated with h, and n:E affiliated with m.
A root content model references h. Member and head may be in separate imported
namespaces with references expanded accordingly.

| Axis | Domain |
| --- | --- |
| Affiliation | Direct m→h; transitive n→m→h; none; cycle |
| Member/head properties | Each abstract false/true independently |
| Type path | Same named type; extension; restriction; unrelated |
| Head final / block | Empty/extension/restriction/both × empty/substitution/extension/restriction/all |
| Model position | Sole h; choice(h,m); sequence(h?,m); sequence(h,separator,m); repeated choice with h/m |
| Second same-expanded-name particle type | Same named type identity; different named type; distinct anonymous types |

**Dependent oracle.** First check affiliation/type construction and final;
cycles/unrelated member type fail SC. Same-type affiliation under head final
containing both methods retains A-106's formal/prose tension; do not turn that
cell into a unique SC oracle. Compute transitive potential group, then
actual permissible substitutions using abstractness and block/derivation
controls. A head can be abstract while concrete members remain usable;
an abstract member is not an actual usable element. Check UPA on names of
**actual** members in each reachable position: choice(h,m) is ambiguous when
m can stand for h; blocking/filtering m can remove that overlap. Optional h
before m also overlaps on first m; a required distinct separator can make
the sequence deterministic. Repetition/nullable boundaries must be checked,
not merely the first branch. Independently apply same-name type consistency;
UPA success does not excuse different named/anonymous type identities under
the rule. Global and local declarations are not interchangeable just because
their expanded names match.

**Witness.** choice(ref h,ref m) with concrete m validly substituting h is
SC− UPA. Make h.block=substitution: m no longer expands h, so those distinct
named alternatives can be unambiguous, while an instance m matched directly
to the explicit m alternative remains permitted. Making m abstract removes
that usable declaration rather than turning it into a valid m instance.
Sequence of two local a declarations with different named types is a distinct
consistency failure even with deterministic positions. Sequence h,separator,m
can be deterministic with the original affiliation and demonstrates why one
overlap at a nonconcurrent position is not automatically UPA failure.

**Masking.** A final-prohibited member construction cannot prove UPA filtering;
use a validly constructed but blocked/abstract member for that opposing case.
Record namespace-qualified actual member names before wildcard interaction.

## IP-010 — Wildcard algebra × processing provenance × known referent × explicit use

Authority: [attribute wildcard union](../upstream/structures.html#cos-aw-union),
[intersection](../upstream/structures.html#cos-aw-intersect),
[wildcard assessment](../upstream/structures.html#cvc-wildcard),
[attribute-set mapping](../upstream/structures.html#declare-type),
[B-006–B-013/B-081–B-090](branches/content-and-derivation.md).

Here a/b denote urn:a/urn:b, `∅` the absent namespace, `{}` the empty namespace
**set**. A admits all; N(a) excludes a and ∅; N(∅) excludes only ∅.

| Axis | Domain |
| --- | --- |
| Every namespace operand | A; N(a); N(b); N(∅); each of the eight finite subsets of {a,b,∅} |
| Assembly | Direct wildcard; two attribute-group intersection; local+group intersection; extension union of base and complete wildcard |
| Processing of each contributor | strict; lax; skip, independently |
| Contributor order | First/second swapped; third contributor A appended |
| Probe namespace | a; b; fresh urn:c; absent |
| Referent | Known valid integer value=1; known invalid integer value=bad; declaration missing |
| Attribute ownership | No explicit use; explicit matching name of xs:integer; explicit matching name xs:string; different explicit name |

**Namespace evaluator for the entire finite operand product.** Equal operands
return themselves. Union with A returns A; intersection with A returns the
other. Finite/finite uses mathematical union/intersection, including {}.
N(a)∩finite S removes a and ∅. N(a)∩N(b), a≠b both present, is unexpressible;
N(a)∩N(∅)=N(a). Distinct negations union to N(∅). N(a)∪S is A if S includes
a and ∅, N(∅) if it includes a only, unexpressible if it includes ∅ only,
and N(a) if neither. N(∅)∪S is A exactly when S includes ∅, otherwise N(∅).
These mutually distinguishing cases decide all listed pairs; do not widen an
unexpressible set to A or shrink it to {}. For a given pairwise expression, an unexpressible intermediate is a
construction failure. For three-or-more operands the source does not completely
specify operand selection: retain all permitted folding outcomes or mark Q
when different folds differ in intermediate expressibility. A mathematically
expressible final set alone does not prove a selected intermediate was legal.

**Processing evaluator.** Group/local intersections take processContents from
the local wildcard when present, otherwise from the **first contributing group**.
They do not choose the strongest operand. Extension unions with the base
namespace set but, when complete wildcard is present, takes complete-wildcard
processing; absent complete wildcard retains the base wildcard. Reversing group
order can therefore preserve the namespace set and change processing.

**Assessment evaluator.** Explicit matching attribute use owns the value and
its type before wildcard admission; an integer explicit use is not bypassed
by a skip wildcard, and a different explicit name does not claim it. Under
wildcard ownership, denied probe namespace is V− regardless of processing.
Admitted strict requires successful referent assessment; admitted lax assesses
a known declaration and rejects known-invalid data but can leave missing
referents unassessed; skip does not assess the known declaration. Multiple
wild ID and declared-ID-use restrictions add IP-007, independently of namespace
admission. Element probes additionally use IP-015's xsi:type/type-route cases.

**Witness.** G1 {a,b}/strict, G2 {a}/skip, no local wildcard yields {a}/strict;
unknown a:x fails. Swap groups: {a}/skip, same namespace set, unknown a:x is
accepted by the wildcard. Add local {a}/lax: processing becomes lax in both
orders; known a:x integer=bad fails while missing declaration can be unassessed.
Add explicit local a:x integer: bad fails even with skip. Change intersection
to disjoint {a}∩{b}: resulting {} denies every attribute, distinguishable
from absent wildcard by its component observation even with no attributes.
Use N(a)∪{∅}: SC− unexpressible; replacing {∅} by {a,∅} yields A and SC+.

**Exclusions/observation.** Duplicate explicit uses with the same expanded name
are SC− before runtime ownership. An all-disallowing present wildcard is not
absence. A no-attribute instance cannot alone distinguish their namespace sets.
SC errors are observed during construction, process provenance through P and
known/missing referent mutations, not a fabricated “strongest mode” rule.

## IP-011 — Attribute derivation × inherited uses × group source × fixed × ID

Authority: [Derivation Valid (Restriction)](../upstream/structures.html#derivation-ok-restriction),
[extension](../upstream/structures.html#cos-ct-extends),
[attribute-use mapping](../upstream/structures.html#declare-type),
[B-028–B-044](branches/content-and-derivation.md).
Use an empty-content B with x:xs:integer and optional y:xs:string. R/D is the
proposed derived type; the same-name source can be local, a global reference,
or a referenced group with the stated declaration type/constraint.

| Axis | Domain |
| --- | --- |
| Method | Restriction; extension |
| B x use | Optional; required |
| Derived x | Omitted; same type optional; same type required; NN optional; NN required; prohibited |
| B effective x constraint | None; fixed 1 |
| Derived effective constraint | None; default 1; fixed 1; fixed 2 |
| New z use | None; z in namespace admitted by B wildcard; z outside it |
| B wildcard | Absent; {a}/strict; A/lax |
| Derived wildcard | Absent; equal; narrower {a}; wider A, each strict/lax/skip |
| ID status | Ordinary x/y; x ID-derived only; x and y ID-derived |

**SC evaluator.** Attribute-use names remain unique and at most one declared
ID-derived use is allowed. Extension must retain each base use's declaration
name, namespace, and type identity; it does not become a way to replace x's
type with NN or remove x. Restriction inherits an omitted ordinary use under
the XML mapping, replaces an explicitly matching use, and removes a prohibited
use where legal. Required base x cannot be removed or weakened to optional;
optional can become required/remove. Matching type must derive correctly.
A fixed base effective constraint requires derived fixed with the prescribed
same value; default/none/different fixed cannot loosen it. Required+default
is independently SC−. For new restriction z, B wildcard must admit its
namespace. Restriction wildcard must be a namespace subset and processing
no weaker than B, except the source's anyType exception; it is not implicitly
inherited when the complete derived wildcard is absent. Extension wildcard
retention/union follows IP-010 rather than this restriction-subset rule.

Changing an ordinary field to ID makes a nonabsent declaration value constraint
illegal and can violate the one-ID-use count; keep those SC gates before ID
episode behavior. A use constraint must also obey its declaration/use rules,
including typed actual-value construction and declaration-fixed consistency.
The declaration's absent constraint and the use's own constraint are different
properties: preserve [DCS-Q001](source-questions.md#dcs-q001--attribute-use-constraints-and-declaration-scoped-rules)
before transferring the declaration-level ID prohibition or canonical-form
revalidation to the use.
An XML `use=prohibited` cannot stand as an admitted runtime use for extension.
Abstract component mutations testing omission/removal independently are labeled
relation tests, not assumed to arise from that XML mapping.

**V evaluator after SC+.** Explicit required attributes must be present and
type-valid. Optional absent constrained attributes and fixed values follow
IP-002, preserving declaration typed equality versus cvc-au's canonical-string
use predicate and the declaration-only-default synthesis question. A prohibited/removed name with no admitting wildcard
fails when present. A name admitted through the remaining wildcard is assessed
by that wildcard, not by the removed declaration. Required inherited uses must
survive local/group assembly before assessment. ID/wild-ID interactions use
IP-007 after these construction gates.

**Witness.** B has required integer x fixed1; R explicitly requires NN x
fixed1, retaining y: SC+; `<v x="1"/>` succeeds. Input `01` is type-valid
and declaration-equal but fails the published use-level canonical-string check. Change only R x to optional:
SC−. Restore required but fixed2: SC− independent fixed mismatch. Omit x in the
restriction XML: inherited required x remains; `<v/>` fails. Optional base x
can instead be explicitly prohibited in R; x absent succeeds, present fails
when no wildcard remains. An extension replacing x's type NN is SC− even if
all sample values happen to be nonnegative. A new a:z restriction use succeeds
with B {a}/strict but not with an absent base wildcard.

**Masking.** A constructed type with two declared IDs cannot prove root ID
reference behavior. A new name denied by the base wildcard cannot prove
process-strength restriction. Keep the SC conjunction vector and V outcomes
separate, and test group/local source permutations without changing the final
expanded name/type/constraint facts.

## IP-012 — Particle restriction dispatch × normalization × ranges × recursion

Authority: [Particle Valid (Restriction)](../upstream/structures.html#cos-particle-restrict),
[occurrence range](../upstream/structures.html#range-ok),
[effective totals](../upstream/structures.html#cos-seq-range),
[B-059–B-118](branches/content-and-derivation.md).
This product tests **component relations** first; arbitrary XML nesting may
violate all-placement/UPA before that relation can be observed.

| Axis | Domain |
| --- | --- |
| Restricted/base term kind | Element; wildcard; all; choice; sequence, all 25 ordered pairs |
| Root range relation | R inside B; equal; min too small; max too large; unbounded R against finite B |
| Group shape | Empty; singleton; two children; nested same-kind group; outer optional group |
| Nested relation fault | None; root; first immediate child; second child; deeper descendant |
| Base unmatched child | Required; optional; empty group |
| Substitution expansion | Neither side head; R head; B head; both heads; actual member blocked/abstract/concrete |
| Type/content property at element leaf | Same; valid restriction; wrong expanded name; weakened nillability; fixed mismatch; insufficient block; dropped identity component |

**Finite dispatch.** Use the exact 25 post-normalization cells in
[B-092–B-116](branches/content-and-derivation.md#all-25-post-normalization-dispatch-cells).
Those cells are **definitions of this axis's permitted relations**, not a
shortcut that merely compares generated languages. Check range and leaf
properties conjunctively with the selected mapping. Element→group uses its
specified singleton handling; group→wildcard uses total cardinality and every
member's valid restriction of the base wildcard, including its own occurrence
range and namespace; sequence/all recurse preserve ordered matching where the
rule says ordered; sequence→all uses injective matching; choice mapping uses
its specified ordered RecurseLax condition; sequence→choice MapAndSum uses
range times particle-list count. Every unmatched required B term defeats a
mapping requiring unmatched base terms to be emptiable. Optional/empty B terms
can be skipped only on permitted recursive routes. Pointless groups normalize
only under their stated shape/range premises, not arbitrary singleton syntax.

**Concrete root.** B=`sequence(a[1,1],b[0,1])[1,1]`, R=`sequence(a[1,1])[1,1]`
with identical named string types: relation holds. Mutate B b to required:
relation fails unmatched-base emptiability. Keep b optional and mutate R a to
c: leaf name relation fails. Wrap both sides in `sequence(z,INNER)` with
required z distinct and unchanged: the same failure now lies at depth two,
while z's successful mapping remains. Repeat INNER under the choice and all
recursive alternatives that the dispatch cell permits; unsupported cross-kind
cells remain relation-false rather than being forced into another mapping.

**Range discriminator.** B wildcard `{a}` with range 1..2, R a sequence
of two names in a each1..1: every leaf restriction and total cardinality pass.
Move only the
second name to b: namespace predicate fails while total remains2. Restore a
and set enclosing R max=2: total4 exceeds B2. MapAndSum discriminator:
B choice of a/b repeated2..2, R sequence(a,b)1..1 has list-count product2;
changing R's max to2 makes product4, even if a different language-analysis
approximation would suggest a narrower range.

**Substitution and Q.** Expand head leaves using IP-009's **actual** group,
including transitive controls/abstract filtering. Source ambiguity over the
ordering of a synthesized substitution choice and ordered-choice mapping is
retained from the branch catalogue; affected cells are Q, not resolved by
sorting names or choosing a successful mapping. Bound recursion depth to the
listed root/immediate/deeper shapes for this finite packet; arbitrary depth
remains the parameterized rule, not a claim that depth2 proves all recursion.

## IP-013 — Namespace permission × component kind × chameleon × source reuse

Authority: [QName interpretation](../upstream/structures.html#src-qname),
[resolution](../upstream/structures.html#src-resolve),
[include](../upstream/structures.html#src-include),
[import](../upstream/structures.html#src-import),
[C-050–C-053/C-060 onward](branches/composition-and-identity.md).

| Axis | Domain |
| --- | --- |
| Referenced symbol space | Type; element; attribute; model group; attribute group; notation |
| Reference-bearing document namespace | Absent; urn:a; urn:b |
| QName binding | Correct prefix; alternate equivalent prefix; wrong namespace; unprefixed no default; unprefixed matching default; unbound prefix |
| Available component | Correct kind/name; same name wrong kind only; missing; same local name in other namespace |
| Namespace permission | Own namespace; matching import in this document; matching import only in including sibling/parent; no import |
| Acquisition | Direct document; include same namespace; include chameleon; import |
| Source reuse | Once; same source included twice same adopted namespace; same no-target source adopted independently into a and b |

**SC/P oracle.** Expand the reference QName in its own document namespace
context; targetNamespace alone does not supply an unprefixed QName's namespace.
Use the requested symbol space, with simple/complex types sharing one space.
Check **document-local** permission to reference a foreign/absent namespace,
then component availability/resolution. A component globally available through
another document does not bypass a required import in the referencing document.
Include requires same namespace or absent namespace to be adopted; import
requires its distinct-namespace contract and does not adopt a foreign schema
as chameleon. Consistent repeated inclusion of the same component identity is
not duplicate construction; distinct same-expanded-name definitions conflict.
No-target C adopted into a and b contributes two contextual component identities,
not one namespace chosen by first use.

**Exact source witness.** C.xsd has no targetNamespace and defines simple T
restricting integer, global e type T, and group G containing local e type T.
A.xsd target urn:a includes C.xsd and declares root r using a:T with
`xmlns:a="urn:a"`. B.xsd target urn:b includes the same C.xsd and uses b:T.
Both contextual T components exist. Swapping include order cannot collapse
one. Supply a caller D target urn:d importing a and b and referencing each
explicitly: both resolve to their own typed integer. A prefix rename preserves
this result; rebinding a to urn:b changes the target identity.

Remove D's import for b while retaining all b components from other supplied
sources: the b:T reference still violates D's namespace permission. Add a
same-named element b:T but no type b:T: type lookup remains unresolved, not
satisfied by the wrong symbol space. Include C twice into A: same contextual
component is reused; replace the second source by a distinct definition of
(urn:a,T): uniqueness fails. Replacing C's absent targetNamespace by urn:b
makes its inclusion into A a namespace-mismatch failure before value checking.

**Exclusions/masking.** Not every component kind has a directly expressible
instance lookup syntax; those cells observe component resolution through its
schema consumer. Import permission failure and missing component are separate
facts, even if both yield SC−. Different bytes/source identities cannot be
called “the identical component included twice” merely because names coincide.

### Imported base versus importer-added local form companion

Fix explicit source acquisition, valid import permission and type extension.
Vary only the two source `elementFormDefault` values independently. The base
source `base.xsd` is:

```xml
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:b" elementFormDefault="unqualified"
           attributeFormDefault="unqualified">
  <xs:complexType name="B">
    <xs:sequence><xs:element name="n" type="xs:string"/></xs:sequence>
  </xs:complexType>
</xs:schema>
```

The importer is:

```xml
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:b="urn:b" xmlns:d="urn:d" targetNamespace="urn:d"
           elementFormDefault="qualified" attributeFormDefault="unqualified">
  <xs:import namespace="urn:b" schemaLocation="base.xsd"/>
  <xs:complexType name="D">
    <xs:complexContent><xs:extension base="b:B">
      <xs:sequence><xs:element name="q" type="xs:string"/></xs:sequence>
    </xs:extension></xs:complexContent>
  </xs:complexType>
  <xs:element name="r" type="d:D"/>
</xs:schema>
```

Both source types have no attributes, block/final constraints, wildcards or
type hints. Both children are required exactly once with distinct names; base
content precedes the added child. In the table, `∅` means no namespace and every
shown root binds `d=urn:d` and `b=urn:b`.

| Base default | Importer default | Required `n` name | Required `q` name | Valid root content |
| --- | --- | --- | --- | --- |
| unqualified | unqualified | `(∅,n)` | `(∅,q)` | `<d:r><n>N</n><q>Q</q></d:r>` |
| unqualified | qualified | `(∅,n)` | `(urn:d,q)` | `<d:r><n>N</n><d:q>Q</d:q></d:r>` |
| qualified | unqualified | `(urn:b,n)` | `(∅,q)` | `<d:r><b:n>N</b:n><q>Q</q></d:r>` |
| qualified | qualified | `(urn:b,n)` | `(urn:d,q)` | `<d:r><b:n>N</b:n><d:q>Q</d:q></d:r>` |

For each admitted source pair, change only `n` to the opposite namespace class:
the base particle fails. Independently change only `q` to its opposite class:
the added particle fails. Keep order and all other facts fixed. Changing only
the importer default changes `q` and leaves inherited `n` unchanged; changing
only the base default changes `n` and leaves `q` unchanged. Prefix renaming
preserves expanded names. The base-unqualified/importer-qualified cell also
has this equivalent default-namespace spelling:

```xml
<r xmlns="urn:d"><n xmlns="">N</n><q>Q</q></r>
```

These outcomes follow the inherited declaration's own source context and the
importer's local declaration independently. They require the supplied source
closure and otherwise admitted component relation; they establish no automatic
network acquisition or executed corpus coverage.

## IP-014 — Redefinition × original identity × ancestry × chameleon references

Authority: [redefine](../upstream/structures.html#src-redefine),
[individual redefinition](../upstream/structures.html#src-expredef),
[extension ancestry](../upstream/structures.html#cos-ct-extends),
[C composition cases](branches/composition-and-identity.md).

| Axis | Domain |
| --- | --- |
| Redefined component | Simple type; complex type; model group; attribute group |
| Included document namespace | Same namespace; absent adopted namespace; different nonempty namespace |
| Self-reference | Correct original component once; absent; twice; wrong expanded name |
| Consumers | Reference in redefining document; reference in original document; a separately including/importing consumer |
| Type ancestry | Restriction only; extension; extension→restriction; extension→restriction→extension restoring removed feature; same chain without restoration |
| Original final / runtime block | Permits/forbids redefinition method × permits/blocks later actual-type/substitution use |
| Source closure | Redefined source resolved; unavailable; malformed schema |
| Repeated replacement | Direct; repeated; diamond reachability of the same resolved document, with equivalent/inequivalent replacements and same/different adopted context |

**SC/P oracle.** Redefine with actual redefinition children requires successful
source resolution and correct namespace/adoption. Redefined simple types use
restriction of their own expanded name; redefined complex types use restriction
or extension of their own name. That reference designates the **original**
component, while ordinary consumers, including those in the original document,
see the redefined component per the pervasive-reference semantics. Group
self-reference must occur exactly once at1..1; without it, the new group must
be a valid restriction of the original. Attribute-group self-reference occurs
exactly once; without it, uses/wildcard form a valid restriction, with no
implicit inheritance of omitted uses. Final constrains construction; block
constrains later use and is not a substitute for final.

Check the complete extension ancestry, including the requirement that it can
be rearranged into extension followed by restriction. A later extension cannot
restore a feature removed by an intervening restriction merely because its
immediate base accepts an extension syntax. Redefinition creates the specified
original/new identities; it does not authorize a competing global writer or
an ordinary cyclic base. No-target adoption follows IP-013 separately for each
namespace context.

**Equivalent-repeat product.** Hold each redefinition individually admitted
and compare the same resolved document redefined in the same way under the
same adopted namespace through direct, repeated and diamond paths. Equivalent
repetition is not by itself a duplicate-component violation. The individual
redefinitions still require identity comparison; avoiding repeated assembly
is permitted rather than a mandatory cache or object layout. Vary resolved
source identity, replacement definition and adoption context independently.
A repeated URI or similar content alone does not establish equivalence.
Inequivalent chains need their applicable identity/derivation constraints;
the repeat exemption supplies no general first-wins/last-wins oracle. Missing
sources or already-invalid replacements cannot isolate this identity product.

**Exact witness.** Old.xsd target urn:a contains T restriction integer with
maxInclusive10 and global e type a:T. New.xsd target urn:a redefines Old.xsd
with T restriction base a:T and maxInclusive5. The base reference inside the
redefinition denotes original T; e's ordinary reference uses new T. Element
e content5 passes, content7 fails even though e was written in Old.xsd.
Changing old T final to restriction makes construction fail. Changing runtime
block on e does not retroactively prohibit creation of T; it affects xsi:type
use under IP-008. Repeating the setup with Old.xsd absent target namespace
and contextual T references exercises adoption rather than a different new
namespace shortcut.

**Group discriminator.** Original G=`sequence(a,b?)`. Redefine G as
`sequence(group-ref G[1,1],c)` with the one permitted self-reference; a,b,c
is the expanded shape. Two self-references or self-reference0..1 is SC−.
Without self-reference, new G=`sequence(a)` is a restriction, whereas new
G=`sequence(c)` fails the leaf mapping. For attribute-group G with optional
x and required y, a no-self redefinition explicitly retaining only required
y can restrict it; omitting y fails, and omitted optional x is not inherited.
For a no-self replacement supplying y through a nonself group reference,
retain [CMP-Q002](source-questions.md#cmp-q002--restrictive-attribute-group-redefine-and-nonself-references).
Original inheritance and contribution from that separate referenced group are
different facts; the explanatory note supplies no independent grammar ban.

**Masking.** An unavailable Old.xsd prevents testing original/new identity.
Wrong namespace prevents testing ancestry. A type-final error is SC− while
a block error is a later V selection predicate; do not combine them into one
“redefinition failed” claim. The exact group restriction relation reuses
IP-012, including its source-qualified ordered-choice cases.

## IP-015 — Missing declaration/type × assessment route × strictness × descendants

Authority: [Schema-Validity Assessment](../upstream/structures.html#cvc-assess-elt),
[Assessment Outcome](../upstream/structures.html#sic-e-outcome),
[wildcards](../upstream/structures.html#cvc-wildcard),
[missing subcomponents](../upstream/structures.html#src-resolve).
Use parent r whose model admits u in urn:u through the stated wildcard. A
known u declaration, when present, has complex simple content integer and an
optional declared child variant separately selected so content-shape preconditions
are not conflated. Stipulated root/type inputs are processor-facing component
assessment cases, not extra instance attributes.

| Axis | Domain |
| --- | --- |
| Context route | Explicit declaration particle; wildcard strict; wildcard lax; wildcard skip; processor-stipulated declaration; processor-stipulated type |
| Declaration availability | Known complete; known but type subcomponent unavailable at the admitted partial-schema boundary; absent |
| Local xsi:type | Absent; usable concrete matching type; unrelated resolved type against stipulated base; unbound/malformed; unknown type name |
| Local data | Valid type content; invalid lexical/content; missing required attribute |
| Child assessment | No child; known-valid descendant; known-invalid descendant; unknown lax descendant; unknown strict/mustFind descendant; skipped descendant |
| Root contract | Entire parent; child u as independent validation root |

**Exclusions.** An ordinary fully constructed schema with an unresolved required
type reference cannot be called SC+ merely to create the partial-schema cell;
that cell requires the specification's missing-subcomponent/invocation boundary.
An explicit declaration particle cannot itself name an absent declaration in a
valid closed XML schema. Those unavailable-declaration combinations are
relation/invocation cases only. Content category is held fixed within a child
variant; an integer simple-content declaration cannot also admit element children.

**Oracle.** A declaration route requires a known declaration, completed local
evaluation, and an actual type when that evaluation reaches type validity.
A type route can use a stipulated type or a usable xsi:type; local type against
a stipulated base must derive under its controls. Successful type-only
assessment does not invent an element declaration. A usable xsi:type route
can strictly assess an element whose declaration is absent; strict wildcard
mustFind is not equivalent to “must have an element declaration” when the
specified type route succeeds. Skip prevents schema assessment through the
wildcard lookup route despite known declarations. If neither strict route is
available and context is non-skip, lax ur-type assessment may occur; it is
not successful validation against an imaginary known type.

The printed outcome rule's nested local-route block has an unintroduced
connective; [ASQ001](source-questions.md#asq001--element-assessment-outcome-route-connective)
retains that integration question. Under the ordinary applicable-route reading
used for these witnesses, P validity for a strictly assessed item is valid only if its own governing
local rule succeeds, no immediate child/attribute validity is invalid, and
no mustFind child/attribute has validity notKnown. Otherwise it is invalid.
A non-strictly-assessed item has validity notKnown under the outcome rule,
even when assessed descendants expose failures. Attempted=full only for a
strictly assessed item whose relevant children/attributes are full; attempted=
none for a non-strict item whose relevant children/attributes are all none;
otherwise partial. A known invalid descendant is not converted to missing
information, and an unknown skip/lax child is not automatically an invalid
known declaration. Capture both the item's P and aggregate report.

**Witness.** Wildcard strict u, no u declaration, instance
`<u:v xmlns:u="urn:u" xmlns:xs="http://www.w3.org/2001/XMLSchema"
xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="xs:integer">1</u:v>`
can use strict type-based assessment. Replace1 by bad: type V−. Remove xsi:type
and all declaration/type stipulation: no strict type/declaration route remains.
Under lax, u itself can be notKnown while a known invalid descendant is still
observed through the applicable assessment path; do not label u valid simply
because lax accepted unknown names. Under skip, a known integer declaration
must not turn bad into a schema-invalid skipped value; XML malformation still
fails before any skip semantics. Known parent with unknown mustFind child
notKnown is invalid, while the corresponding non-mustFind unknown child does
not satisfy that particular invalidity predicate.

**Observation.** Store context-determined marker, source of known type/declaration,
local validity, child outcomes, attempted and validity separately. A single
aggregate V− cannot distinguish known-invalid data, missing required schema
information, or lexical XML failure. Skipped identity fields remain I-002,
not an automatically imported XSD1.1 exemption.

## IP-016 — Regex grammar × XML normalization × facet provenance × value identity

Authority: [pattern](../upstream/datatypes.html#rf-pattern),
[Appendix F](../upstream/datatypes.html#regexs),
[whitespace](../upstream/datatypes.html#rf-whiteSpace),
[equality](../upstream/datatypes.html#equal).

| Axis | Domain |
| --- | --- |
| Base | xs:string; xs:token; xs:decimal; IL; IS; SI |
| Regex shape | Empty; `a`; `b`; `.`; `.*`; `[0-9 ]+`; `\d+`; `[a-z-[aeiou]]+`; `\p{L}\p{M}?`; `^a$` |
| Pattern source | One local pattern; local OR with literal `a`; inherited step followed by local literal `a` |
| Pattern XML transport | Direct permitted characters; XML character reference for a metacharacter; literal attribute LF; LF character reference |
| Candidate | Empty; `a`; `ba`; `^a$`; one LF; ` a `; `01`; U+0661; `e`+U+0301; U+00E9; `1 2` |
| Other value facet | None; enumeration of one type-valid representative (per base below) |

Enumeration representatives are string/token `a`, decimal `1`, IL `1 2`,
IS `1` (integer-selected), SI `1` (string-selected). All schemas first pass
facet applicability and value-literal admission. XML transport alternatives
apply to the exact metacharacter/LF subexpression they state; they do not
invent a reference where a pattern has no such character.

**Full oracle.** XML expands references and normalizes literal attribute
whitespace before regex parsing. An XML reference to dot produces wildcard
dot, while backslash+reference-to-dot produces literal dot. Attribute LF
becomes space unless preserved via character reference. Parse the resulting
XSD regex; unresolved bare-brace cases from D2-048 are outside the settled
regex domain, not silently rejected. `^a$` is literal, not anchored a.
Dot excludes LF/CR but includes other XML characters. `\d` is Unicode Nd;
`[0-9]` is ASCII. Subtraction removes vowels from lowercase letters;
`\p{L}\p{M}?` admits e+combining acute and precomposed é without declaring
them equal. Matching is whole-string, with no case folding/normalization.

Run the base's whitespace/union/list admission and apply pattern at its proper
whole-literal boundary. Same-step alternatives are OR; inherited groups are
AND. Finally apply enumeration by typed identity, not canonical spelling.
All facets must hold. A regex match cannot rescue an invalid decimal literal
U+0661, and an integer-selected IS value1 is not SI's string-selected value1.
For list input1 2 the list pattern sees the whole sequence; item rules still
apply. No byte/code-unit count or host-regex extension supplies an oracle.

**Witness.** String pattern `.` rejects LF but token pattern `.` on LF sees
empty after collapse and also rejects, for a different normalized input;
string pattern `.*` rejects an isolated LF, token `.*` accepts the empty
normalized value. String pattern `[0-9]+` rejects U+0661; `\d+` admits it,
but decimal with the same `\d+` still rejects it lexically. Decimal enumeration1
plus pattern `[0-9]+` admits01 yet excludes1.0, although both denote1 before
the lexical-pattern gate. Local patterns a and b admit either; inheriting a
then locally adding b admits neither. These discriminate all dependent stages.

**Bounds/observation.** Use the captured Unicode database/version for category
membership; future-assignment differences are Q until version evidence is
recorded. Direct/reference transport giving the same parser input is a
metamorphic companion, not a different semantic regex. Text splitting/comments
that preserve the character sequence do not change this product's result.

## IP-017 — Facet derivation × same-step/inherited policy × value order × fixed

Authority: [§4.3 facets](../upstream/datatypes.html#rf-facets),
[restriction matrix](branches/datatypes.md#d2-033--all-sixteen-inherited-bound-numbered-branches),
[duration/date order](../upstream/datatypes.html#duration-order).

| Axis | Domain |
| --- | --- |
| Family/base | Character string/URI length; binary octet length; list item length; QName/NOTATION ineffective length; decimal/integer digits; float/double bounds; temporal/duration bounds |
| Facet stage | One local; same-step pair; one inherited+one local; inherited through a vacuous intermediate |
| Base fixed | false; true for every fixable facet; prohibited fixed on pattern/enumeration |
| New endpoint/value relation | Below; equal alias; above; incomparable where the base defines it |
| Facet pair | length with minLength/maxLength; minLength with maxLength; totalDigits with fractionDigits; each of16 new/base bound name pairs |
| Value spelling | Canonical; valid equal alias; invalid base lexical form |
| Runtime candidate | Below/equal/above each effective bound; incomparable; lexical pattern pass/fail where independently constructible |

**Constructor normalization.** Each family uses its own unit and applicable
facets. Never put totalDigits on list, enumeration on boolean, or bounds on
QName to create a supposed runtime facet failure. QName/NOTATION length facets
are admitted no-ops: all candidates satisfy them, so their runtime negative
cell is infeasible by definition. NOTATION first has its required declared
enumeration; binary input is decoded before measuring octets.

**Complete constructor evaluator.** Check valid facet values, singleton
cardinality (except pattern/enumeration), applicability, fixed equality,
monotonic restriction, pair consistency, and ancestry-specific coexistence.
A tighter facet still violates fixed. Repeating exclusive endpoints uses the
source's explicit exception, not ordinary value membership alone. All16
bound relation rows at n=9/10/11 in D2-033 form this product's concrete finite
numeric instantiation; their clause-versus-total-SC outcomes remain distinct.
Its three opposite-lower-kind whole-SC cells retain
[D-072's inherited scope question](datatypes.md#d-072--questions-and-permissible-choices-that-must-remain-visible);
passing the numbered comparison alone does not resolve that constructor Q.
length with min/max needs the qualifying ancestor, not just numerical
consistency. Same-step/inherited patterns and enumeration sets use IP-003/016.
No rule rejects every empty resulting value space automatically.

**Runtime evaluator.** Under SC+, candidate must be base-lexically valid and
satisfy every effective facet. Compare typed aliases as equal. Indeterminate
duration/time comparisons do not satisfy a bound. NaN equals itself; inclusive
NaN bound admits only NaN and exclusive NaN bound admits no value. Non-NaN
bounds exclude NaN. Decimal digits constrain existence of the specified
i×10^-n representation, not raw digit counts. The same literal can satisfy
one unit/facet and fail another; preserve that vector.

**Witness and discriminators.** B string minLength1, T length2 is SC+ and
ab passes; putting length2 and minLength1 together on string is SC− despite
same numeric interval. B fixed maxLength3, T maxLength2 is SC− although
narrower. Decimal totalDigits2,fractionDigits2 admits `.99` and `00099.00`,
rejects`.001` and100; fractionDigits2 alone admits1.2300 but not1.231.
Duration minInclusiveP1M rejects P30D by indeterminacy but admits P32D.
Float maxInclusiveNaN admits NaN and rejects1; changing to exclusive rejects
both. A decimal pattern can distinguish1/01 while its enumeration cannot.

**Q boundary.** Partial-order **schema endpoint** restriction clauses do not
always state a unique stronger rejection condition for incomparable endpoints;
retain D2-048's source-qualified constructor question. This is distinct from
the settled runtime bound rule. Precision maxima and regex resource failures
are not datatype-invalid witnesses.
The inherited opposite-lower-kind Q likewise concerns construction; no runtime
product is credited until its schema is admitted.

## IP-018 — Invocation/conformance × acquisition × root × optional observations

Authority: [conformance](../upstream/structures.html#conformance),
[assessment](../upstream/structures.html#validation_outcome),
[schema representation](../upstream/structures.html#schema-repr),
[Web location conventions](../upstream/structures.html#schema-loc),
[Part2 conformance](../upstream/datatypes.html#conformance),
[processor-policy families](structures.md#required-interactions).

| Axis | Domain |
| --- | --- |
| Claimed level | Minimum semantic; XML-representation; full Web-interoperable layer as specified |
| Schema supply | Explicit valid closed component set; schema XML documents; instance hints only; no schema information |
| Acquisition policy | Fixed sources/no dynamic acquisition; dynamic retrieval succeeds; unavailable hinted location; additional schema denied by caller policy |
| Assessment entry | Declaration-stipulated; type-stipulated; expanded-name lookup; usable xsi:type; no strict route |
| Root | Whole document; selected subtree with retained in-scope namespace context |
| Data | Valid; known invalid child/attribute; missing declaration/type; malformed XML |
| Exposure | Validity only; partial optional PSVI; complete available required facts through an exposing interface |

**Oracle.** Semantic validation rules do not depend on whether schemas arrived
as XML or another admitted representation. XML-representation claims add syntax
and mapping obligations; they do not alter value identity. Acquisition strategy
and source availability are explicit preconditions, not semantic assertions
that a schema-invalid document became valid. Instance hints are hints, not a
redefinition of component namespace, roots, or declared types. A fixed-source
lower-layer processor need not dynamically load an instance hint. A claimed
full layer has its stated Web-access obligations; do not silently use that
claim for an explicit no-network lower-layer invocation. Dynamic acquisition
must not mutate established declarations so the same assessment disagrees with
bulk acquisition of the same component set.

Strict/type/lax/skip routes use IP-015; a subtree root has its own ID/identity
context under IP-006/007 and retains XML namespace context rather than erasing
prefixes. Malformed XML is rejected before semantic classification. Optional
PSVI exposure choices do not erase required semantic decisions (for example
nillable declaration information needed to evaluate key). Validity-only output
cannot prove the E1-56 P values or wildcard metadata provenance; mark those
observations unavailable, not implicitly satisfied.

**Schema-XML admission product.** For schema XML supply, cross the following
would-be schema-for-schemas results with representation constraints satisfied/
violated and component constraints satisfied/violated. These three error
classes remain independent; passing one does not waive another.

| Source-schema result | Schema-for-schemas condition in §5.1 |
| --- | --- |
| validity=valid, attempted=full | Satisfied |
| validity=valid, attempted=partial | Satisfied equally; full is not required |
| validity=invalid | Not satisfied |
| No established would-be result | Not established; lack of an actual pass is not itself a schema error |

This is a condition on what schema-for-schemas assessment would yield, not a
required extra implementation pass. [C-081](branches/composition-and-identity.md#c-081--schema-errors-are-not-instance-invalid-witnesses)
provides full/partial witnesses and preserves ASQ001's applicable-route
qualification. Only sources satisfying all three schema-error conditions can
support an instance-validity claim; unobserved source admission gives no such
proof.

**Web default and override product.** Expand every combination of these axes.
Hold a supported invocation, the chosen schema components correct, and the input well-formed; this
subproduct observes selection duties and policy, not successful retrieval or
instance validity. Join the core product only on the same context, chosen root,
schema and acquisition policy; incompatible assignments are infeasible.

| Axis | Domain |
| --- | --- |
| Processing context | Document processed from the Web; non-Web delivery |
| Purpose | General-purpose schema-aware processor; fixed-schema specialized processor |
| Root direction | None; user selects document element; user selects a subtree |
| Schema direction | None; user supplies matching-namespace schema; user supplies a different-namespace schema |
| Document/subtree namespaces | Each independently absent; urn:a; urn:b |
| Namespace dereference default | Attempt dereference; do not attempt dereference |
| User dereference direction | None; override with the opposite choice |

For a general-purpose processor assessing a Web document, absent root direction
requires the document element. Absent schema direction requires construction
of a schema corresponding to a schema document whose targetNamespace matches
the namespace name, if any, of the **selected assessment root**. A user-directed
subtree does not keep the document element's namespace as the default target.
With the respective user direction present, that default does not override
the user's choice; ordinary entry and component-validity rules still apply.
The two defaults are not imposed by this paragraph on specialized or non-Web
processing. Preserve the broader permitted invocation choices rather than
inventing rejection of such cases.

Within the Web schema-location conventions, either namespace-dereference
default must permit user-directed override. This requirement is separate from
the general-purpose root/schema defaults; specialization is not an exemption
stated for this override. No user direction leaves the chosen default; opposite
direction must be available for either default. Non-Web delivery has no
override obligation established by this Web clause. A permitted or required
attempt is not proof of successful retrieval, and policy selection is not
proof that the selected schema validates the input.

As a distinguishing policy witness, use a Web document in urn:a with a subtree
in urn:b. With no root or schema direction, the general-purpose defaults select
the document element and a matching urn:a schema document. Direct only the root
to the subtree: the default schema target becomes urn:b. Direct the schema too:
record that explicit selection instead of forcing a namespace-matching default.
Repeat both dereference defaults with opposite user direction. Classify the
specialized/non-Web counterparts under their actual applicability; do not
require this repository to perform network I/O to describe the product.

**Hint recommendation, timing and global-effect product.** Join these finite
axes to the same Web context, initial assessment root, admitted schema sources
and acquisition policy above. This is a separate join from namespace-URI
dereference defaults; its recommendations do not become mandatory fetches.

| Axis | Domain |
| --- | --- |
| Convention applicability | Applicable Web schema-location profile; outside that claimed profile |
| Hint form | Namespace/location pairs; noNamespaceSchemaLocation single location |
| Hint locations | Empty pair list; one location; multiple location entries where admitted |
| Addressed namespace | urn:a; absent, with the matching hint form |
| Initially validated element | Document element; selected subtree |
| Addressed-use footprint | None; inside root only; outside root only; both |
| First addressed use within root | Element; attribute; no such use |
| Earlier other-namespace use | Present; absent |
| Hint chronology | Strictly before first addressed use within root; strictly after; on that first item; no such use; hint absent |
| Contrary direction | None; invoking application directs otherwise; command-line option directs otherwise |
| Referent facts | No referent established; acquired correct schema; acquired invalid or wrong-namespace source |
| Carrier scope | Later use inside the hint carrier; later sibling after that carrier ends |

Keep form/namespace/entry combinations admitted: the single-location form
addresses absent namespace; an empty pair list has no location entry. With
an empty pair list, addressed namespace, referent and addressed-use chronology
are inapplicable. Ordinary namespace uses remain contextual facts; that empty
hint itself establishes no late-use violation. With no addressed use inside
the root, first-use kind/order is inapplicable and the
late-use premise is absent. With no hint, its form/referent/carrier axes are
inapplicable. The hint carrier in this timing family lies within the initial
root; preprocessing an outside hint belongs to the explicit acquisition
policy, not a fabricated occurrence inside the newly selected subtree.

Under the applicable convention and no contrary direction, processors **should**
attempt each schema-document location URI in the hint values. This is a
recommendation about location URIs, not the namespace-dereference default or
a prescribed request count. Contrary direction affects that recommendation;
it does not by itself waive applicable source-admission, stability or timing
conditions. Acquisition failure establishes neither a usable component nor
semantic invalidity by itself.

A hint strictly after the first addressed-namespace element **or attribute**
within the initially validated element violates the stated timing condition.
Use the item's namespace-name property; a QName value naming that namespace
does not itself give the containing item that namespace name.
First use of another namespace, or a use outside the selected initial root,
does not satisfy that premise. Do not decide same-item ordering from serialized
attribute order: establish the applicable source/Infoset chronology before
claiming the strict-after condition. For a pure timing contrast, keep needed
components already admitted and available so missing-schema failures do not
mask it. Absence of this late-use premise is not proof of complete validity.

After actual successful acquisition, definitions/declarations remain in effect
beyond the lexical scope of the hint's element. [XR-058](external-rules.md#nested-and-late-hints-within-xr-058)
supplies a nested-hint/later-sibling witness and its late-order mutation.
Repeat with an addressed attribute as first use, another namespace as the
earlier use, and an earlier use outside a selected subtree. Observe direction,
attempt, successful acquisition, component availability and assessment
separately. An unavailable observation remains a gap; no public network or
trace API is required by this catalogue.

**Witness.** Explicit schema r containing declared integer v validates
`<r><v>1</v></r>` and rejects v=bad regardless of a contradictory
xsi:schemaLocation hint under fixed-source policy. The unknown hinted source
is not fetched to change r's established declaration. Supply the same component
set dynamically or directly: when both policies admit it, the same data has
the same semantic outcome. Assess only `<sub><use ref="a"/></sub>` from a
larger document whose matching ID lies outside sub: subtree root does not
borrow that ID. A validity-only implementation can distinguish these V results
but cannot establish that an empty defaulted element contributed lexical+001
rather than canonical1; that remains an observation gap.

**Exclusions.** A policy-denied resolver call is an operational/policy outcome,
not SC− by an XSD component constraint. This specification catalogue does not
invoke network acquisition or require the repository to expose an API outside
its explicit contract. Record the claimed layer and inputs before comparing
any such case with a corpus result.

## IP-019 — Required dependent joins across product owners

This table defines five **full joins**, not pairwise supplements. Take the full
finite domains of the listed IP products, identify their shared facts, and
retain every tuple satisfying all stated construction/XML preconditions.
Conflicting assignments to the **same** declaration/type/namespace/field are
infeasible, not two competing authorities. Report each owner's result vector;
aggregate only after its prerequisites are known. An invalid construction or
undefined typed value masks later runtime observations, but not other defined
predicates. These joins add no source rule beyond their cited owners.

| Join | Shared axes and complete outcome composition | Concrete legal witness and discriminator |
| --- | --- | --- |
| J1: IP-001/003/004/005 | Element default/fixed lexical form, actual union/list type, nil/nillable, namespace context and identity-field value are one chain. First SC, then element/type/default rules, then E1-56 value/member P, then XPath cardinality/qualified tuples. Preserve I-001/I-003 at SC/nilled fields. | Two rows each contain empty defaulted integer v=+001 under unique field v: equal values create a duplicate. Change one explicit content to2: distinct. Make v nillable=true but nonnil under key: declaration condition fails independently; do not label that merely a duplicate failure. QName default p:n in schema urn:a remains that value when the instance rebinds p. |
| J2: IP-002/007/010/011/013 | Attribute source/constraint/use after derivation, explicit-versus-wildcard ownership, namespace permission, processContents, declaration-type ID eligibility and root IDREF are evaluated in that order. A source-resolved global ID declaration is not automatically an assessed wild ID under skip. | An empty complex type with no declared ID use, strict {urn:a} wildcard, global a:i ID, i=a and a declared IDREF a succeeds. Add optional declared x:ID but leave x absent: wild-ID conflict. Replace i with a known integer bad: assessment fails before ID eligibility; that cannot demonstrate an ID-reference defect. |
| J3: IP-008/009/012/014 | Original/redefined type identity, full ancestry/final, runtime block, actual substitution group, particle normalization/mapping and UPA are coupled. Construct first, then compute permitted substitutions, then consistency/UPA/restriction relations, then instance type/content. | h:B and m:E in a deterministic sequence(h,separator,m) are usable with valid affiliation. Redefine B so original consumer references its narrower replacement: recompute E ancestry before treating m as still substitutable. Blocking substitution can remove h/m overlap in a choice but does not legalize a final-prohibited type definition. Ordering-dependent synthesized choices remain Q. |
| J4: IP-005/006/007/015/018 | Validation root/context, selector reach, typed-assessment availability, root ID owners, recursively propagated identity table and reference scope are shared. A strict type-only child does not acquire a missing declaration; skip does not create a typed field or an outside-root ID. Apply I-002/I-003 and outcome P separately. | Parent scope use=1 can see a single child K entry1 under whole-root assessment. Move use into an empty sibling scope or assess it as a standalone subtree: no borrowed sibling/ancestor key table. Make the field skipped: record unavailable typed-value/source qualification rather than pretending it is an empty string or automatically applying the1.1 exemption. |
| J5: IP-003/004/016/017 | XML character sequence, datatype/member whitespace, regex parsing/group provenance, facet applicability/fixedness, typed equality/order and schema-literal namespace context must agree. Lexical equality/canonical form is not substituted for E; fixed use-level comparison remains IP-002. | Decimal pattern `[0-9]+` and enumeration1 admits01 but rejects1.0. QName enumeration p:n admits q:n with both prefixes urn:a, while pattern `p:n` rejects q:n. Move that pattern to an inherited group versus a same-step OR companion: recompute the whole lexical language before identity comparison. |

No join creates a new mandatory evaluation/diagnostic order in a processor;
the order here is a dependency order for deriving a sound expected result.
A failed antecedent prevents claiming a later observed rule, which is precisely
why each standalone product retains a legal witness and one-fact mutations.

## Required-product routing and remaining limits

| Mandatory Structures product | Product descriptions owning its dependent decisions |
| --- | --- |
| Names and composition | IP-004, IP-013, IP-014; J2/J3 |
| Defaults, type, nil and PSVI | IP-001–IP-005, IP-008; J1 |
| Attributes and derivation | IP-002, IP-007, IP-010, IP-011; J2 |
| Wildcard algebra and assessment | IP-007, IP-010, IP-015; J2/J4 |
| Substitution, model consistency and UPA | IP-008, IP-009, IP-012, IP-014; J3 |
| Particle restriction recursion | IP-009, IP-012, IP-014; J3 |
| Extension/restriction ancestry | IP-008, IP-011, IP-012, IP-014; J3 |
| Identity values | IP-001–IP-005, IP-016, IP-017; J1/J5 |
| Identity scope and propagation | IP-005, IP-006, IP-015, IP-018; J4 |
| ID context | IP-002, IP-007, IP-010, IP-011, IP-018; J2/J4 |
| Missing information and outcomes | IP-005, IP-010, IP-015, IP-018; J4 |
| Processor policy versus core | IP-013–IP-015, IP-018; J4 |
| Datatype facet, regex and value products | IP-003, IP-004, IP-016, IP-017; J5 |

These are bounded symbolic products with concrete source-derived witnesses,
mutations and outcome evaluators. Their descriptions do not claim generated
fixture files, execution, PSVI capture or corpus coverage. The remaining limits
are precise:

- The full tuples have not been emitted as individually numbered fixtures.
  The tables define their finite domains and shared-variable joins; a later
  expansion must preserve the stated exclusions, result vectors and Q labels.
- Unbounded recursion, repetition, value size and namespace-source graphs have
  finite boundary/shape representatives here. A proof or generator for all
  larger sizes is not supplied by these witnesses. IP-012's depth-two cases
  do not establish arbitrary-depth behavior.
- The identity simple-content/nil/skip interpretations, cvc-elt typed/canonical
  wording, declaration-only attribute-default synthesis, same-type nomination
  under both final exclusions, multioperand wildcard fold selection, anonymous
  component identity and substitution-choice ordering retain their recorded
  source qualifications. Part2's Unicode-version, double-extremal, bare-brace,
  leap-second/BCE/recurring-time and partial-order schema-endpoint questions in
  [D2-048](branches/datatypes.md#d2-048--source-tensions-and-exact-unresolved-or-permitted-oracles)
  remain Q wherever a joined tuple depends on them.
- Not every external XML/URI/Unicode/RFC production, every Appendix A syntactic
  context, or every arbitrary cross-product of **independent** capabilities is
  re-expanded here. Those are owned by the grammar/dependency catalogues and
  standalone branch obligations; this file closes the listed dependent products,
  not a claim that every possible software behavior is enumerable.
- A validity-only seam leaves P/K/property-provenance outcomes unobserved.
  A corpus comparison must mark those partial/unobservable, not covered merely
  because its valid/invalid flag agrees. Operational acquisition/resource failure
  and source-oracle disagreement remain distinct from a demonstrated semantic gap.
