# Complex Type Definitions

## Contents

- [Complex Type Properties](#complex-type-properties)
- [Content Models](#content-models)
  - [Model Groups](#model-groups)
  - [Particles](#particles)
  - [Mixed Content](#mixed-content)
- [Type Derivation](#type-derivation)
  - [Derivation by Extension](#derivation-by-extension)
  - [Derivation by Restriction](#derivation-by-restriction)
  - [simpleContent and complexContent](#simplecontent-and-complexcontent)
- [Wildcards](#wildcards)
- [Abstract Types](#abstract-types)
- [Constraints](#constraints)
  - [Unique Particle Attribution (UPA)](#unique-particle-attribution-upa)
  - [Element Declarations Consistent](#element-declarations-consistent)
  - [Derivation Constraints](#derivation-constraints)
  - [Particle Valid (Restriction)](#particle-valid-restriction)
  - [Attribute Wildcard Intersection](#attribute-wildcard-intersection)

---

A **Complex Type Definition** describes the permitted content (elements, text) and attributes for an element. Every element with child elements or attributes has a complex type.

Complex types can be:

- **Named** (global) — declared at schema top level, reusable
- **Anonymous** — defined inline within an element declaration

## Complex Type Properties

| Property | Description |
|----------|-------------|
| `name` | Optional NCName (anonymous types have none) |
| `target namespace` | Either absent or a namespace URI |
| `base type definition` | The type this derives from |
| `derivation method` | `extension` or `restriction` |
| `final` | Subset of {`extension`, `restriction`} — blocks further derivation |
| `abstract` | Boolean — if true, cannot be used directly |
| `attribute uses` | Set of attribute declarations |
| `attribute wildcard` | Optional `<xs:anyAttribute>` |
| `content type` | One of: `empty`, simple type, or (particle, `mixed` or `element-only`) |
| `prohibited substitutions` | Subset of {`extension`, `restriction`} |
| `annotations` | Documentation |

Spec refs: docs/spec/xml/structures.xml#Complex_Type_Definitions, docs/spec/xml/structures.xml#Complex_Type_Definition_details.

## Content Models

The content model defines what child elements can appear and in what order.

### Model Groups

Model groups combine particles using three composition operators:

**Sequence** — children must appear in order:

```xml
<xs:complexType name="AddressType">
  <xs:sequence>
    <xs:element name="street" type="xs:string"/>
    <xs:element name="city" type="xs:string"/>
    <xs:element name="state" type="xs:string"/>
    <xs:element name="zip" type="xs:string"/>
  </xs:sequence>
</xs:complexType>
```

**Choice** — one of the children appears per occurrence of the choice:

```xml
<xs:complexType name="ContactType">
  <xs:choice>
    <xs:element name="email" type="xs:string"/>
    <xs:element name="phone" type="xs:string"/>
    <xs:element name="fax" type="xs:string"/>
  </xs:choice>
</xs:complexType>
```

Occurrence constraints on the `<xs:choice>` itself control whether the choice is optional or repeatable.

**All** — children can appear in any order (each at most once):

```xml
<xs:complexType name="PersonType">
  <xs:all>
    <xs:element name="firstName" type="xs:string"/>
    <xs:element name="lastName" type="xs:string"/>
    <xs:element name="middleName" type="xs:string" minOccurs="0"/>
  </xs:all>
</xs:complexType>
```

**All Group Restrictions (XSD 1.0):**

- Each particle in `<xs:all>` must have `minOccurs` and `maxOccurs` of 0 or 1
- The `<xs:all>` group itself must have `maxOccurs="1"` and `minOccurs` of 0 or 1
- Only `<xs:element>` declarations allowed inside `<xs:all>` (no groups, wildcards, or nested compositors)

*Note: XSD 1.1 relaxes these restrictions, allowing `maxOccurs > 1` on children and wildcards.*

### Particles

A **particle** pairs a term with occurrence constraints:

- **Element particle** — an element declaration with min/max occurs
- **Wildcard particle** — an `<xs:any>` allowing elements from specified namespaces
- **Model group particle** — a nested sequence/choice/all

**Named Model Groups** (`<xs:group>`) allow reuse:

```xml
<xs:group name="addressGroup">
  <xs:sequence>
    <xs:element name="street" type="xs:string"/>
    <xs:element name="city" type="xs:string"/>
  </xs:sequence>
</xs:group>

<xs:complexType name="CustomerType">
  <xs:sequence>
    <xs:element name="name" type="xs:string"/>
    <xs:group ref="addressGroup"/>
  </xs:sequence>
</xs:complexType>
```

### Mixed Content

Setting `mixed="true"` allows character data alongside child elements:

```xml
<xs:complexType name="ParagraphType" mixed="true">
  <xs:sequence>
    <xs:element name="bold" type="xs:string" minOccurs="0" maxOccurs="unbounded"/>
    <xs:element name="italic" type="xs:string" minOccurs="0" maxOccurs="unbounded"/>
  </xs:sequence>
</xs:complexType>
```

**Rules:**

- Text nodes can appear anywhere within the content
- Child elements must still match the content model
- Default is `mixed="false"` (element-only content)

**Mixed Content Validation:**

- Character data between elements is ignored for content model validation (only element order/count matters)
- In element-only content (`mixed="false"`), only whitespace text is allowed between elements
- Non-whitespace text in element-only content violates `cvc-complex-type.2.3`

**Mixed Content Inheritance:**

- When extending a type whose content type is element-only or mixed, `mixed` must match the base type; when the base is empty, the derived type may be element-only or mixed
- When restricting a type, `mixed` can be reduced from mixed to element-only if the particle restriction is valid; it cannot be increased from element-only to mixed

Spec refs: docs/spec/xml/structures.xml#cvc-complex-type, docs/spec/xml/structures.xml#coss-modelGroup, docs/spec/xml/structures.xml#cos-all-limited.

## Type Derivation

Complex types can inherit from other types via **extension** or **restriction**.

### Derivation by Extension

Extension adds new content to the base type:

```xml
<xs:complexType name="Address">
  <xs:sequence>
    <xs:element name="street" type="xs:string"/>
    <xs:element name="city" type="xs:string"/>
  </xs:sequence>
</xs:complexType>

<xs:complexType name="USAddress">
  <xs:complexContent>
    <xs:extension base="Address">
      <xs:sequence>
        <xs:element name="state" type="xs:string"/>
        <xs:element name="zip" type="xs:string"/>
      </xs:sequence>
      <xs:attribute name="country" type="xs:string" fixed="US"/>
    </xs:extension>
  </xs:complexContent>
</xs:complexType>
```

**Extension Rules:**

- Base type must not have `final="extension"`
- New elements are appended to the base content model
- New attributes can be added
- Cannot remove or change existing content

### Derivation by Restriction

Restriction narrows the base type—any valid restricted instance must also be valid against the base:

```xml
<xs:complexType name="RestrictedPurchaseOrder">
  <xs:complexContent>
    <xs:restriction base="PurchaseOrderType">
      <xs:sequence>
        <xs:element name="shipTo" type="USAddress"/>
        <xs:element name="billTo" type="USAddress"/>
        <xs:element ref="comment" minOccurs="1"/>  <!-- now required -->
        <xs:element name="items" type="Items"/>
      </xs:sequence>
      <xs:attribute name="orderDate" type="xs:date" use="required"/>
    </xs:restriction>
  </xs:complexContent>
</xs:complexType>
```

**Restriction Rules:**

- Base type must not have `final="restriction"`
- The derived content model must be a valid restriction of the base
- Can restrict element/attribute types to derived types
- Cannot introduce elements or attributes outside what the base allows; restriction can narrow wildcards or replace them with specific declarations

**Occurrence Constraint Rules:**

Occurrence ranges in a restriction must be subsets of the base type's ranges:

| Constraint | Rule |
|------------|------|
| `minOccurs` | Can stay same or **increase** (make more required) |
| `maxOccurs` | Can stay same or **decrease** (make less allowed) |

Examples of valid restrictions:
- `minOccurs="0"` → `minOccurs="1"` (optional → required) ✓
- `maxOccurs="unbounded"` → `maxOccurs="5"` ✓
- `minOccurs="1" maxOccurs="5"` → `minOccurs="2" maxOccurs="3"` ✓

Examples of **invalid** restrictions:
- `minOccurs="1"` → `minOccurs="0"` (required → optional) ✗
- `maxOccurs="5"` → `maxOccurs="10"` ✗

Extension after restriction must also satisfy
[cos-ct-extends 1.5](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-ct-extends):
an extension followed by a restriction must be possible from the ancestor whose
base is `anyType`. An original attribute `a:int` cannot be removed and then
restored as unrelated `a:string`; any such extension would have to retain the
original declaration. New attribute names remain allowed. The original
ancestor's requiredness and typed fixed-value constraints also remain binding.
An intermediate restriction's narrower type is not itself the original anchor.

### simpleContent and complexContent

Use these wrappers when deriving types:

**simpleContent** — element has text content only (plus optional attributes):

```xml
<xs:complexType name="PriceType">
  <xs:simpleContent>
    <xs:extension base="xs:decimal">
      <xs:attribute name="currency" type="xs:string" use="required"/>
    </xs:extension>
  </xs:simpleContent>
</xs:complexType>

<!-- Instance: <price currency="USD">19.99</price> -->
```

An enclosing `complexType`'s `mixed` attribute is lexically checked but has no
effect when `simpleContent` is present. The component has simple content,
regardless of that spelling. An explicit mixed `complexContent` extension cannot
turn that simple content into mixed content. An effectively empty extension can
still inherit the entire simple content of its base under representation
mapping 3.2.1.

Spec refs: [Complex Type Definition Representation, src-ct](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#src-ct)
and [content mapping](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-type).

**complexContent** — element has element/mixed content:

```xml
<xs:complexType name="ExtendedAddress">
  <xs:complexContent>
    <xs:extension base="Address">
      <xs:sequence>
        <xs:element name="country" type="xs:string"/>
      </xs:sequence>
    </xs:extension>
  </xs:complexContent>
</xs:complexType>
```

Spec refs: docs/spec/xml/structures.xml#cos-ct-extends, docs/spec/xml/structures.xml#derivation-ok-restriction, docs/spec/xml/structures.xml#cos-particle-extend, docs/spec/xml/structures.xml#cos-particle-restrict.

## Wildcards

Wildcards allow elements or attributes not explicitly declared:

### Element Wildcards (`<xs:any>`)

```xml
<xs:complexType name="ExtensibleType">
  <xs:sequence>
    <xs:element name="name" type="xs:string"/>
    <xs:any namespace="##other" processContents="lax" 
            minOccurs="0" maxOccurs="unbounded"/>
  </xs:sequence>
</xs:complexType>
```

**Namespace Values:**

| Value | Meaning |
|-------|---------|
| `##any` | Any namespace |
| `##other` | Any namespace except the target namespace (no-namespace is excluded) |
| `##local` | No namespace |
| `##targetNamespace` | The schema's target namespace |
| URI list | Specific namespace URIs (space-separated) |

An absent `namespace` attribute defaults to `##any`. A present empty or
whitespace-only list denotes the empty namespace set. It overlaps no wildcard,
including `##any`. A required wildcard with that set cannot consume an element;
it does not become an empty particle. `##other` excludes both the target
namespace and the absent namespace, so it is disjoint from a list containing
only those two values.

Spec refs: [Wildcard representation](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-openness)
and [Wildcard allows Namespace Name](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-wildcard-namespace).

**processContents Values:**

| Value | Meaning |
|-------|---------|
| `strict` | Assess through an element declaration or a known `xsi:type`; missing assessment is an error |
| `lax` | Use an available declaration or `xsi:type`; otherwise assess with `anyType` |
| `skip` | Skip schema assessment of the element and its subtree |

A known `xsi:type` can supply strict type assessment for an undeclared element.
Its value and the selected type's attributes and content still validate; an
unknown, malformed or unbound type name does not satisfy this condition. When
lax processing falls back to `anyType`, declared descendants still validate.
The library also rejects unresolved physical `xsi:type` on laxly assessed
elements under [selected WG correction 4299](INTERPRETATIONS.md#i-014--xsitype-under-element-wildcards);
this type-existence requirement is not explicit in the printed 2004 fallback.
Skip processing suppresses these schema checks throughout its subtree, while
XML well-formedness and namespace rules still apply. No selected type can waive
the wildcard's namespace constraint, and instance hints never load schemas.

These element-assessment rules differ from attribute wildcards, which have no
`xsi:type` fallback. See [Item Valid (Wildcard)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-wildcard),
[Schema-Validity Assessment (Element)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-assess-elt)
and [Assessment Outcome (Element)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#sic-e-outcome).

Restriction between element wildcards requires namespace and occurrence containment,
and processing at least as strong as the base (`strict` > `lax` > `skip`).
NSSubset clause 3 exempts the actual content wildcard of `xs:anyType`. An empty
extension inherits its base's entire content type, so an unchanged inherited
ur-type wildcard retains that exemption. An independently declared lax
wildcard with equal bounds and namespaces does not. Arbitrary ancestry from
`xs:anyType` is also insufficient.

This component-based exception differs from attribute restriction clause 4.3,
which names the immediate base **type**. See the published
[content mapping, 3.2.1](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#declare-type),
[NSSubset, clause 3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-NSSubset),
and [same-by-construction component identity](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#no-identity).

### Attribute Wildcards (`<xs:anyAttribute>`)

```xml
<xs:complexType name="ExtensibleElement">
  <xs:sequence>
    <xs:element name="content" type="xs:string"/>
  </xs:sequence>
  <xs:anyAttribute namespace="##any" processContents="skip"/>
</xs:complexType>
```

Spec refs: docs/spec/xml/structures.xml#Wildcards, docs/spec/xml/structures.xml#cvc-wildcard, docs/spec/xml/structures.xml#cvc-wildcard-namespace.

Note: `anyAttribute` does not make a schema invalid just because it could
admit ID-typed attributes. The “at most one ID attribute per element” rule is
checked during instance validation when concrete attributes are present.

## Abstract Types

An abstract complex type cannot be used directly—elements must use `xsi:type` to specify a concrete derived type:

```xml
<xs:complexType name="Shape" abstract="true">
  <xs:sequence>
    <xs:element name="color" type="xs:string"/>
  </xs:sequence>
</xs:complexType>

<xs:complexType name="Circle">
  <xs:complexContent>
    <xs:extension base="Shape">
      <xs:sequence>
        <xs:element name="radius" type="xs:decimal"/>
      </xs:sequence>
    </xs:extension>
  </xs:complexContent>
</xs:complexType>

<xs:element name="shape" type="Shape"/>
```

Instance must specify concrete type:

```xml
<shape xsi:type="Circle">
  <color>red</color>
  <radius>5.0</radius>
</shape>
```

Spec refs: docs/spec/xml/structures.xml#cvc-type, docs/spec/xml/structures.xml#xsi_type.

## Constraints

### Unique Particle Attribution (UPA)

Content models must be unambiguous—at each point, it must be clear which particle matches the next element without lookahead:

```xml
<!-- INVALID: ambiguous which <a> particle matches -->
<xs:choice>
  <xs:element name="a" type="Type1"/>
  <xs:element name="a" type="Type2"/>
</xs:choice>

<!-- INVALID: wildcard overlaps with element -->
<xs:choice>
  <xs:element name="foo" type="xs:string"/>
  <xs:any namespace="##any"/>
</xs:choice>
```

**UPA Violation Conditions:**

| Situation | Why it fails UPA |
|-----------|------------------|
| Same element name in choice | Cannot determine which branch |
| Element in same namespace as wildcard | Wildcard and element both match |
| Two wildcards with overlapping namespaces | Cannot determine which wildcard |
| Optional element followed by same name | Greedy matching becomes ambiguous |

**Valid alternatives:**

```xml
<!-- OK: different names in choice -->
<xs:choice>
  <xs:element name="bookItem" type="BookType"/>
  <xs:element name="cdItem" type="CDType"/>
</xs:choice>

<!-- OK: wildcard excludes target namespace where element is declared -->
<xs:sequence>
  <xs:element name="foo" type="xs:string"/>
  <xs:any namespace="##other" minOccurs="0"/>
</xs:sequence>
```

### Element Declarations Consistent

If the same element name and namespace are reachable via different paths, the type definitions must be the same named top-level type (same name and target namespace):

```xml
<!-- INVALID: same name, different types -->
<xs:sequence>
  <xs:choice>
    <xs:element name="item" type="Type1"/>
  </xs:choice>
  <xs:choice>
    <xs:element name="item" type="Type2"/>
  </xs:choice>
</xs:sequence>
```

### Derivation Constraints

- No circular type derivation (a type cannot derive from itself)
- `final` blocks specified derivation methods
- `block` on elements prevents type substitution via `xsi:type`
- Restricted types must be strict subsets of their base

### Particle Valid (Restriction)

When restricting a complex type, the derived content model must be a valid restriction of the base. The spec defines a recursive algorithm for this validation:

**Basic Rules:**

| Base Particle | Derived Particle | Valid If |
|---------------|------------------|----------|
| Element | Element | Same name/namespace, derived type ⊆ base type, occurrence ⊆ base |
| Wildcard | Element | Element's namespace matches wildcard |
| Wildcard | Wildcard | Namespace and occurrence containment, permitted processing strength |
| Model group | Model group | Recursive check on children |

Empty **content** is a separate complex-type restriction case. It may restrict
empty base content, or an element-only or mixed base whose particle is
emptiable (`derivation-ok-restriction` 5.3). The base's outer `minOccurs` need
not be zero: a choice containing an empty sequence is also emptiable. An
optional particle with a nonempty term still has element-only content and
must satisfy the particle restriction rules; emptiability alone is insufficient.

**Sequence/Choice/All Mapping:**

- Sequence to sequence, and all to all: complete ordered recursive mapping;
  unmapped base particles must be emptiable (`Recurse`). XSD 1.0 uses this
  ordered restriction rule even though all-group instance matching is unordered.
- Choice to choice: complete ordered recursive mapping; unmatched base
  alternatives may be skipped (`RecurseLax`). Nested choices and retained
  unbounded branches are permitted when their recursive restrictions succeed.
  Their presence alone does not require XSD 1.1.
- Sequence to all: complete injective mapping without order; unmapped base
  particles must be emptiable (`RecurseUnordered`).
- Sequence to choice: each derived member maps to a permitted base member;
  targets may be shared (`MapAndSum`). Its additional range check multiplies
  the sequence's occurrence bounds by its number of direct particles, then
  compares that product with the base choice's occurrence bounds. Aggregate
  effective element-count containment cannot replace this check.

**Occurrence Range Subsumption:**

The derived occurrence range `[minD, maxD]` must be a subset of base `[minB, maxB]`:
- `minD ≥ minB`
- `maxD ≤ maxB`

Restriction checks apply at each retained group boundary. The specified
pointless-group removals do not permit general multiplication of nested
repetition bounds: fitting the base's aggregate effective range alone does not
establish a valid structural restriction. For example, changing an outer
sequence from 2–3 repetitions to four is invalid even if tighter inner bounds
keep the total count within the base's effective range.

Pointlessness removes an empty sequence or all group, and an empty choice
only when its minimum is zero. A singleton all group is pointless even when
its outer minimum is zero; both its outer and member bounds still determine
the resulting occurrence range. Singleton sequence and choice groups require
unit bounds to be removed. Unit sequence-in-sequence and choice-in-choice
groups can also be flattened; nonunit boundaries remain.

The effective-content mapping separately erases a direct empty sequence or
all group, an optional empty choice, and a zero-maximum particle. A required
empty choice remains a present element-only particle and accepts no element
sequence, including the empty sequence. These rules do not turn every
particle accepting zero children into empty content.

A named model group's direct compositor cannot specify `minOccurs` or
`maxOccurs`; its references carry occurrence bounds. A repeated reference can
also contain a repeated nested compositor, and both boundaries still apply.

Spec refs: [Derivation Valid (Restriction, Complex), clause 5.3](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#derivation-ok-restriction),
[Complex Type Definition representation](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#Complex_Type_Definition_details),
[Element Sequence Valid](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-model-group),
[Particle Valid (Restriction), clause 2.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-particle-restrict),
[Occurrence Range OK](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#range-ok),
[Recurse](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-Recurse),
[RecurseLax](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-RecurseLax),
[RecurseUnordered](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-RecurseUnordered),
[MapAndSum](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#rcase-MapAndSum),
and [Model Group Definition representation](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#Model_Group_Definition_details).

### Attribute Wildcard Intersection

When attribute groups and a local `<xs:anyAttribute>` are combined, their
namespace constraints are intersected. An empty finite intersection is valid
and admits no additional attributes. An intersection that cannot be expressed
by the XSD 1.0 wildcard forms is invalid.

Spec ref: [Attribute Wildcard Intersection](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-aw-intersect).

Spec refs: docs/spec/xml/structures.xml#cos-nonambig, docs/spec/xml/structures.xml#cos-element-consistent, docs/spec/xml/structures.xml#cos-particle-restrict, docs/spec/xml/structures.xml#cos-aw-intersect.
