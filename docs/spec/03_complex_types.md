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
- Non-whitespace text in element-only content causes validation error `cvc-complex-type.2.2`

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

**processContents Values:**

| Value | Meaning |
|-------|---------|
| `strict` | Must find declaration and validate |
| `lax` | Validate if declaration found, otherwise accept |
| `skip` | Accept without validation |

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
| Wildcard | Wildcard | Derived namespace ⊆ base namespace |
| Model group | Model group | Recursive check on children |
| Any particle | Empty | Base particle's `minOccurs="0"` |

**Sequence/Choice/All Mapping:**

- Sequence to sequence: each derived particle maps to corresponding base particle
- Choice to choice: each derived particle maps to some base particle
- All to all: each derived particle maps to a base particle (order-independent)

**Occurrence Range Subsumption:**

The derived occurrence range `[minD, maxD]` must be a subset of base `[minB, maxB]`:
- `minD ≥ minB`
- `maxD ≤ maxB`

### Attribute Wildcard Intersection

When attribute groups and a local `<xs:anyAttribute>` are combined, their namespace constraints are intersected (per `cos-aw-intersect`). The intersection must be expressible; if it is empty or not expressible, the schema is invalid.

Spec refs: docs/spec/xml/structures.xml#cos-nonambig, docs/spec/xml/structures.xml#cos-element-consistent, docs/spec/xml/structures.xml#cos-particle-restrict, docs/spec/xml/structures.xml#cos-aw-intersect.
