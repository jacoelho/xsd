# Schema Components

## Contents

- [Overview](#overview)
- [Built-in Types](#built-in-types)
  - [anyType (The Ur-Type)](#anytype-the-ur-type)
  - [anySimpleType](#anysimpletype)
- [Schema Element](#schema-element)
- [Annotations](#annotations)
- [Model Groups](#model-groups)
- [Attribute Groups](#attribute-groups)
- [Notations](#notations)
- [Component Uniqueness](#component-uniqueness)

---

## Overview

A schema is composed of various **schema components**—abstract representations of declarations and definitions. The main component types are:

| Component | Purpose |
|-----------|---------|
| Schema | Container for all components |
| Element Declaration | Defines elements |
| Attribute Declaration | Defines attributes |
| Complex Type Definition | Defines element content models |
| Simple Type Definition | Defines value constraints |
| Model Group Definition | Reusable content model fragments |
| Attribute Group Definition | Reusable attribute sets |
| Notation Declaration | Declares notations |
| Identity Constraint Definition | Key, unique, keyref |
| Annotation | Documentation and application info |

Spec refs: docs/spec/xml/structures.xml#components.

## Built-in Types

XML Schema defines special built-in types that serve as the foundation of the type system:

### anyType (The Ur-Type)

`anyType` is the root of all type definitions—both simple and complex types ultimately derive from it.

**Properties:**

- All complex types derive from `anyType` (directly or transitively)
- Can contain any content: elements, attributes, mixed content, or simple content
- Elements declared with `type="xs:anyType"` accept any well-formed XML content
- Serves as the implicit base type when no base is specified in a complex type definition

```xml
<!-- Element accepts any content -->
<xs:element name="data" type="xs:anyType"/>

<!-- Equivalent: complexType with no explicit base derives from anyType -->
<xs:complexType name="MyType">
  <xs:sequence>
    <xs:element name="child" type="xs:string"/>
  </xs:sequence>
</xs:complexType>
```

Spec refs: docs/spec/xml/structures.xml#key-urType, docs/spec/xml/datatypes.xml#dt-anySimpleType.
**Validation:**

- Content model: allows any elements in any order
- Attributes: allows any attributes
- Mixed content: allowed
- `xsi:type` can specify any type (since all types derive from anyType)

### anySimpleType

`anySimpleType` is the base of all simple type definitions.

**Properties:**

- All 19 primitive types derive directly from `anySimpleType`
- All derived simple types (by restriction, list, or union) derive from it transitively
- Can be used directly as an element or attribute type, but is rarely useful because it accepts any simple content
- In the type hierarchy: `anyType` → `anySimpleType` → primitive types

```xml
<!-- This is rarely useful in practice -->
<xs:element name="value" type="xs:anySimpleType"/>
```

**Relationship:**

```
anyType (ur-type, base of everything)
    │
    ├── anySimpleType (base of all simple types)
    │   ├── string
    │   ├── decimal
    │   ├── boolean
    │   └── ... (all primitives)
    │
    └── (all user-defined complex types)
```

## Schema Element

The `<xs:schema>` element is the root of every schema document:

```xml
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/ns"
           xmlns:tns="http://example.com/ns"
           elementFormDefault="qualified"
           attributeFormDefault="unqualified"
           blockDefault=""
           finalDefault=""
           version="1.0">
  
  <!-- Schema content: imports, includes, declarations, definitions -->
  
</xs:schema>
```

### Schema Attributes

| Attribute | Description |
|-----------|-------------|
| `targetNamespace` | Namespace for global components |
| `elementFormDefault` | Default form for local elements (`qualified`/`unqualified`) |
| `attributeFormDefault` | Default form for local attributes |
| `blockDefault` | Default `block` for elements and types |
| `finalDefault` | Default `final` for types |
| `version` | Schema version (informational) |
| `xml:lang` | Language for documentation |

### Schema Children (in order)

- Zero or more of `xs:include`, `xs:import`, `xs:redefine`, or `xs:annotation` (in any order)
- Then zero or more schema top components (`xs:element`, `xs:attribute`, `xs:complexType`, `xs:simpleType`, `xs:group`, `xs:attributeGroup`, `xs:notation`), each optionally followed by one or more `xs:annotation`

Spec refs: docs/spec/xml/structures.xml#declare-schema.

## Annotations

Annotations provide documentation and application-specific information:

```xml
<xs:element name="customer">
  <xs:annotation>
    <xs:documentation xml:lang="en">
      Represents a customer in the system.
    </xs:documentation>
    <xs:appinfo source="http://example.com/tools">
      <tool:validation level="strict"/>
    </xs:appinfo>
  </xs:annotation>
  <xs:complexType>
    ...
  </xs:complexType>
</xs:element>
```

### xs:annotation

Container for documentation and app info. Can appear as the first child of most schema elements.

### xs:documentation

Human-readable documentation:

```xml
<xs:documentation xml:lang="en" source="http://example.com/docs">
  This type represents a monetary amount with currency.
</xs:documentation>
```

| Attribute | Description |
|-----------|-------------|
| `xml:lang` | Language of the documentation |
| `source` | URI of external documentation |

### xs:appinfo

Application-specific information:

```xml
<xs:appinfo source="http://example.com/codegen">
  <codegen:javaClass>com.example.Customer</codegen:javaClass>
</xs:appinfo>
```

| Attribute | Description |
|-----------|-------------|
| `source` | URI identifying the application/tool |

**Content:** Any well-formed XML (typically from other namespaces)

### Where Annotations Can Appear

Almost every schema construct can have an annotation as its first child:

- `xs:schema`
- `xs:element`, `xs:attribute`
- `xs:complexType`, `xs:simpleType`
- `xs:group`, `xs:attributeGroup`
- `xs:sequence`, `xs:choice`, `xs:all`
- `xs:restriction`, `xs:extension`
- `xs:key`, `xs:unique`, `xs:keyref`
- Facets (`xs:pattern`, `xs:enumeration`, etc.)

Spec refs: docs/spec/xml/structures.xml#cAnnotations.

## Model Groups

Model group definitions allow reuse of content model fragments:

### xs:group Definition

```xml
<xs:group name="addressGroup">
  <xs:sequence>
    <xs:element name="street" type="xs:string"/>
    <xs:element name="city" type="xs:string"/>
    <xs:element name="state" type="xs:string"/>
    <xs:element name="zip" type="xs:string"/>
  </xs:sequence>
</xs:group>
```

### xs:group Reference

```xml
<xs:complexType name="CustomerType">
  <xs:sequence>
    <xs:element name="name" type="xs:string"/>
    <xs:group ref="addressGroup"/>
    <xs:element name="phone" type="xs:string"/>
  </xs:sequence>
</xs:complexType>
```

### Group Properties

| Property | Description |
|----------|-------------|
| `name` | NCName (required for definition) |
| `ref` | QName reference (for reference) |
| `minOccurs` | Minimum occurrences (on reference) |
| `maxOccurs` | Maximum occurrences (on reference) |

### Group Constraints

- A group definition must contain exactly one model group (`xs:sequence`, `xs:choice`, or `xs:all`)
- Groups cannot be recursive (a group cannot reference itself directly or indirectly)
- An `xs:all` group can only appear as the model group of a named group, or as the term of a particle with `maxOccurs="1"` that is the content model of a complex type; it cannot be nested or repeated

Spec refs: docs/spec/xml/structures.xml#coss-modelGroup, docs/spec/xml/structures.xml#cos-all-limited.

### Example with Choice

```xml
<xs:group name="contactGroup">
  <xs:choice>
    <xs:element name="email" type="xs:string"/>
    <xs:element name="phone" type="xs:string"/>
    <xs:sequence>
      <xs:element name="email" type="xs:string"/>
      <xs:element name="phone" type="xs:string"/>
    </xs:sequence>
  </xs:choice>
</xs:group>
```

## Attribute Groups

Attribute group definitions allow reuse of attribute sets:

### xs:attributeGroup Definition

```xml
<xs:attributeGroup name="commonAttrs">
  <xs:attribute name="id" type="xs:ID"/>
  <xs:attribute name="class" type="xs:NMTOKENS"/>
  <xs:attribute name="style" type="xs:string"/>
</xs:attributeGroup>
```

### xs:attributeGroup Reference

```xml
<xs:complexType name="DivType">
  <xs:sequence>
    <xs:any namespace="##any" processContents="lax" 
            minOccurs="0" maxOccurs="unbounded"/>
  </xs:sequence>
  <xs:attributeGroup ref="commonAttrs"/>
  <xs:attribute name="title" type="xs:string"/>
</xs:complexType>
```

### Attribute Group Content

An attribute group can contain:

- `xs:attribute` declarations
- `xs:attributeGroup` references (to other groups)
- `xs:anyAttribute` wildcard

```xml
<xs:attributeGroup name="extendedAttrs">
  <xs:attributeGroup ref="commonAttrs"/>
  <xs:attribute name="lang" type="xs:language"/>
  <xs:anyAttribute namespace="##other" processContents="lax"/>
</xs:attributeGroup>
```

### Constraints

- No circular references between attribute groups
- No duplicate attribute names when groups are combined
- At most one `xs:anyAttribute` in the resolved attribute set

Spec refs: docs/spec/xml/structures.xml#cAttribute_Group_Definitions.

## Notations

Notation declarations identify non-XML data formats:

```xml
<xs:notation name="gif" public="image/gif" system="gif-viewer.exe"/>
<xs:notation name="jpeg" public="image/jpeg"/>
```

### Notation Properties

| Attribute | Description |
|-----------|-------------|
| `name` | NCName identifying the notation |
| `public` | Public identifier (MIME type or formal identifier) |
| `system` | System identifier (URI of handler/viewer) |

### Using Notations

Types derived from `xs:NOTATION` restrict attribute values to declared notation names. Using `xs:NOTATION` directly is a schema error; the restriction must include `enumeration`.

```xml
<xs:simpleType name="ImageFormat">
  <xs:restriction base="xs:NOTATION">
    <xs:enumeration value="gif"/>
    <xs:enumeration value="jpeg"/>
  </xs:restriction>
</xs:simpleType>

<xs:attribute name="format" type="ImageFormat"/>
```

### Notation Constraints

- Notation names must be unique within the schema
- Types derived from `xs:NOTATION` can only be used for attributes (not elements)
- Restriction from `xs:NOTATION` must include `enumeration`
- Values must match a declared notation name
- For compatibility, NOTATION should only be used in schemas with no target namespace

Spec refs: docs/spec/xml/structures.xml#cNotation_Declarations, docs/spec/xml/datatypes.xml#NOTATION.

## Component Uniqueness

Within a target namespace, certain components must have unique names:

| Component Type | Uniqueness Scope |
|----------------|------------------|
| Element declarations | Global elements in same namespace |
| Attribute declarations | Global attributes in same namespace |
| Type definitions | All types (simple + complex) in same namespace |
| Model groups | All groups in same namespace |
| Attribute groups | All attribute groups in same namespace |
| Notations | All notations in same namespace |
| Identity constraints | Within containing element declaration |

Spec refs: docs/spec/xml/structures.xml#composition-schemaImport.

### Name Conflicts

```xml
<!-- ERROR: Two global elements with same name -->
<xs:element name="item" type="xs:string"/>
<xs:element name="item" type="ItemType"/>

<!-- ERROR: Type name conflicts (simple vs complex) -->
<xs:simpleType name="Amount">...</xs:simpleType>
<xs:complexType name="Amount">...</xs:complexType>

<!-- OK: Element and type can share names -->
<xs:element name="customer" type="customer"/>
<xs:complexType name="customer">...</xs:complexType>
```

### Local vs Global

Local declarations (within complex types) don't conflict with global declarations or each other across types:

```xml
<!-- OK: Local elements with same name in different types -->
<xs:complexType name="Type1">
  <xs:sequence>
    <xs:element name="value" type="xs:string"/>
  </xs:sequence>
</xs:complexType>

<xs:complexType name="Type2">
  <xs:sequence>
    <xs:element name="value" type="xs:integer"/>
  </xs:sequence>
</xs:complexType>
```
