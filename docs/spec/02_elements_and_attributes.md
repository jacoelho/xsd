# Element and Attribute Declarations

## Contents

- [Element Declarations](#element-declarations)
  - [Element Declaration Properties](#element-declaration-properties)
  - [Occurrence Constraints](#occurrence-constraints)
  - [Substitution Groups](#substitution-groups)
  - [Abstract Elements](#abstract-elements)
  - [Element Form (Qualification)](#element-form-qualification)
- [Attribute Declarations](#attribute-declarations)
  - [Attribute Declaration Properties](#attribute-declaration-properties)
  - [Attribute Groups](#attribute-groups)
  - [Attribute Form (Qualification)](#attribute-form-qualification)
- [Validation Summary](#validation-summary)
  - [Default/Fixed Value Processing](#defaultfixed-value-processing)

---

XML Schema uses declarations to define elements and attributes that can appear in XML documents.

- **Element Declaration** — defines an element's name, content type, and other properties
- **Attribute Declaration** — defines an attribute's name and simple type

Declarations can be **global** (top-level, under `<xs:schema>`) or **local** (nested within complex type definitions).

## Element Declarations

Every element in an instance document must be matched by an element declaration or accepted by a wildcard (with `processContents="lax"` or `processContents="skip"` allowing undeclared elements).

### Element Declaration Properties

| Property | Description |
|----------|-------------|
| `name` | An NCName (non-colonized name) |
| `target namespace` | Either absent or a namespace URI |
| `type definition` | A simple type or complex type definition |
| `scope` | Global or a complex type definition |
| `value constraint` | Optional: a pair of (value, `default` or `fixed`) |
| `nillable` | Boolean indicating if `xsi:nil="true"` is allowed |
| `identity-constraint definitions` | Set of key/unique/keyref constraints |
| `substitution group affiliation` | Optional: a top-level element this can substitute for |
| `substitution group exclusions` | Subset of {`extension`, `restriction`} |
| `disallowed substitutions` | Subset of {`substitution`, `extension`, `restriction`} |
| `abstract` | Boolean indicating element cannot appear directly |
| `annotation` | Optional documentation |

### Occurrence Constraints

Use `minOccurs` and `maxOccurs` on element particles to control repetition:

```xml
<xs:sequence>
  <xs:element name="comment" type="xs:string" minOccurs="0" maxOccurs="1"/>
  <xs:element name="item" type="ItemType" minOccurs="1" maxOccurs="unbounded"/>
</xs:sequence>
```

**Rules:**

- Omitted attributes default to `minOccurs="1"` and `maxOccurs="1"`
- `maxOccurs="unbounded"` allows unlimited repetition
- Only valid on particles within model groups (sequence, choice, all)

### Substitution Groups

A global element can be designated as a **substitution group head**. Other elements can substitute for it if they:

1. Declare `substitutionGroup="headElement"`
2. Have a type derived from the head's type

```xml
<xs:element name="item" type="ItemType" abstract="true"/>
<xs:element name="book" type="BookType" substitutionGroup="item"/>
<xs:element name="cd" type="CDType" substitutionGroup="item"/>
```

In an instance, `<book>` or `<cd>` can appear wherever `<item>` is expected.

**Substitution Rules:**

- The substituting element's type must be validly derived from the head's type per the type derivation rules
- Only global element declarations can be substitution group heads or members; actual membership excludes abstract declarations and applies the head's blocking constraints
- The head can block substitution via `block="substitution"`
- The `final` attribute on the head can restrict derivation methods

**Transitive Substitution:**

Substitution is transitive with type derivation requirements:
- If C substitutes for B and B substitutes for A, then C can substitute for A
- However, C's type must be derivable from A's type (not just from B's type)
- The entire derivation chain must be valid: C's type → B's type → A's type

```xml
<xs:element name="A" type="TypeA"/>
<xs:element name="B" type="TypeB" substitutionGroup="A"/>  <!-- TypeB derives from TypeA -->
<xs:element name="C" type="TypeC" substitutionGroup="B"/>  <!-- TypeC derives from TypeB -->
<!-- C can substitute for A because TypeC → TypeB → TypeA forms valid derivation chain -->
```

**Block and Final Attributes:**

| Attribute | On | Effect |
|-----------|-----|--------|
| `block="substitution"` | Element | Prevents any substitution group member from substituting |
| `block="extension"` | Element | Blocks `xsi:type` to types derived by extension |
| `block="restriction"` | Element | Blocks `xsi:type` to types derived by restriction |
| `final="extension"` | Element | Prevents elements with extension-derived types from joining substitution group |
| `final="restriction"` | Element | Prevents elements with restriction-derived types from joining substitution group |

The `blockDefault` and `finalDefault` attributes on `<xs:schema>` set defaults for element declarations and type definitions where applicable.

### Abstract Elements

An element with `abstract="true"` cannot appear directly in an instance—only its substitution group members can:

```xml
<xs:element name="vehicle" type="VehicleType" abstract="true"/>
<xs:element name="car" type="CarType" substitutionGroup="vehicle"/>
```

Attempting to use `<vehicle>` in an instance causes a validation error.

### Element Form (Qualification)

Whether an element requires a namespace prefix depends on its declaration:

**Global elements** are always in the target namespace. If the schema has no `targetNamespace`, global elements are in no namespace and appear unprefixed:

```xml
<!-- Schema with targetNamespace="http://example.com/IPO" -->
<xs:element name="purchaseOrder" type="PurchaseOrderType"/>

<!-- Instance must use the namespace -->
<ipo:purchaseOrder xmlns:ipo="http://example.com/IPO">
  ...
</ipo:purchaseOrder>
```

**Local elements** follow `elementFormDefault`:

- `elementFormDefault="unqualified"` (default) — local elements have no namespace prefix
- `elementFormDefault="qualified"` — local elements must use the target namespace

Individual declarations can override via `form="qualified"` or `form="unqualified"`.

Spec refs: docs/spec/xml/structures.xml#cElement_Declarations, docs/spec/xml/structures.xml#Element_Declaration_details, docs/spec/xml/structures.xml#cos-equiv-class, docs/spec/xml/structures.xml#cos-equiv-derived-ok-rec, docs/spec/xml/structures.xml#coss-modelGroup.

## Attribute Declarations

Attributes carry additional information on elements. Each attribute to be validated needs a corresponding declaration.

### Attribute Declaration Properties

| Property | Description |
|----------|-------------|
| `name` | An NCName |
| `target namespace` | Either absent or a namespace URI |
| `type definition` | A simple type definition |
| `scope` | Global or a complex type definition |
| `value constraint` | Optional: a pair of (value, `default` or `fixed`) |
| `required` | Whether the attribute must appear (`use="required"`) |
| `annotation` | Optional documentation |

**Examples:**

```xml
<!-- Optional attribute with type -->
<xs:attribute name="orderDate" type="xs:date"/>

<!-- Required attribute -->
<xs:attribute name="partNum" type="SKU" use="required"/>

<!-- Fixed value attribute -->
<xs:attribute name="country" type="xs:NMTOKEN" fixed="US"/>

<!-- Default value attribute -->
<xs:attribute name="currency" type="xs:string" default="USD"/>
```

**Behavior:**

- `use="optional"` (default) — attribute may be omitted
- `use="required"` — attribute must appear
- `use="prohibited"` — attribute must not appear (for restriction)
- `fixed="value"` — if present, must equal value; if absent, value is assumed
- `default="value"` — if absent, value is supplied; can be overridden

### Attribute Groups

Attribute groups name and reuse a common set of attributes:

```xml
<xs:attributeGroup name="commonAttrs">
  <xs:attribute name="id" type="xs:ID" use="required"/>
  <xs:attribute name="lang" type="xs:language" use="optional"/>
</xs:attributeGroup>

<xs:complexType name="MyType">
  <xs:sequence>
    <xs:element name="content" type="xs:string"/>
  </xs:sequence>
  <xs:attributeGroup ref="commonAttrs"/>
</xs:complexType>
```

### Attribute Form (Qualification)

Unlike elements, attributes are **unqualified by default**:

- Global attributes are in the target namespace; they must be prefixed unless the schema has no `targetNamespace` (then they are unprefixed)
- Local attributes follow `attributeFormDefault` (default is `unqualified`)

```xml
<!-- Schema with targetNamespace -->
<xs:schema targetNamespace="http://example.com/ns"
           attributeFormDefault="unqualified">
  <!-- Local attribute 'code' appears without prefix in instances -->
  <xs:complexType name="RegionType">
    <xs:attribute name="code" type="xs:string"/>
  </xs:complexType>
</xs:schema>

<!-- Instance: attribute has no prefix -->
<region code="US-CA"/>
```

Spec refs: docs/spec/xml/structures.xml#cAttribute_Declarations, docs/spec/xml/structures.xml#Attribute_Declaration_details, docs/spec/xml/structures.xml#cAttribute_Group_Definitions.

## Validation Summary

For an element to be valid:

1. Its qualified name must match a declaration's `{name}` and `{target namespace}`
2. Its content must be valid according to the `{type definition}`
3. If `xsi:nil="true"` is present, `{nillable}` must be true, there must be no fixed value constraint, and content must be empty
4. If a `{value constraint}` exists:
   - For `default`: applied only when the element has no element or character children
   - For `fixed`: if not empty, the content value must match the fixed constraint value
5. All identity constraints must be satisfied

**Default/Fixed Value Processing:**

| Condition | Default | Fixed |
|-----------|---------|-------|
| Element is nilled (`xsi:nil="true"`) | Not applied | Not checked |
| Element has no element or character children | Default value applied in PSVI | Fixed value applied in PSVI |
| Element has element or character content | No effect | Content must match fixed value |

Whitespace-only text still counts as character content and does not trigger default/fixed value application.

The default/fixed value must be valid for the element's type. For complex types with simple content, the value applies to the text content.

For an attribute to be valid:

1. It must be declared in the element's complex type or allowed by a wildcard
2. Its value must conform to the `{type definition}`
3. If `fixed`, the value must match
4. If `use="required"`, it must be present

**Schema Constraints:**

- No duplicate element declarations with same name and namespace in the same scope
- No duplicate attribute declarations with same name and namespace in the same type
- Default/fixed values must be valid for the declared type
- Substitution group members must have compatible types with the head

Spec refs: docs/spec/xml/structures.xml#cvc-elt, docs/spec/xml/structures.xml#cvc-attribute.
