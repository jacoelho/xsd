# Namespaces in XML Schema

## Contents

- [Overview](#overview)
- [Namespace Declarations](#namespace-declarations)
- [Target Namespace](#target-namespace)
- [Form Defaults](#form-defaults)
- [Schema Composition](#schema-composition)
  - [xs:include](#xsinclude)
  - [Chameleon Include](#chameleon-include)
  - [xs:import](#xsimport)
  - [xs:redefine (Deprecated)](#xsredefine-deprecated)
- [Instance Namespace Attributes](#instance-namespace-attributes)
- [Reserved Namespaces](#reserved-namespaces)
- [Validation Considerations](#validation-considerations)

---

## Overview

XML Namespaces provide a mechanism to uniquely identify element and attribute names by associating them with a namespace URI. In XML Schema:

- Schema components belong to a **target namespace**
- Instance elements/attributes are matched to declarations by **expanded name** (namespace + local name)
- **Form defaults** control whether local declarations require namespace qualification

Spec refs: docs/spec/xml/structures.xml#declare-schema.

## Namespace Declarations

Namespaces are declared using reserved attributes:

### Default Namespace

```xml
<element xmlns="http://example.com/ns">
  <!-- Unprefixed elements are in http://example.com/ns -->
  <child>content</child>
</element>
```

- Applies to unprefixed element names within scope
- Does **not** apply to attributes
- `xmlns=""` undeclares the default (reverts to no namespace)

### Prefixed Namespace

```xml
<ipo:purchaseOrder xmlns:ipo="http://example.com/ipo">
  <ipo:shipTo>...</ipo:shipTo>
</ipo:purchaseOrder>
```

- Binds a prefix to a namespace URI
- Scope extends to the element and its descendants
- Prefix must be declared before use

Spec refs: docs/spec/xml/structures.xml#declare-schema.

## Target Namespace

A schema declares its target namespace on the `<xs:schema>` element:

```xml
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/ipo"
           xmlns:ipo="http://example.com/ipo">
  
  <!-- All global components belong to http://example.com/ipo -->
  <xs:element name="purchaseOrder" type="ipo:PurchaseOrderType"/>
  <xs:complexType name="PurchaseOrderType">
    ...
  </xs:complexType>
</xs:schema>
```

**Rules:**

- Global elements, attributes, types, groups are associated with the target namespace
- A schema without `targetNamespace` defines components in "no namespace"
- QName references resolve using in-scope namespaces; unprefixed values use the default namespace when present

Spec refs: docs/spec/xml/structures.xml#declare-schema.

## Form Defaults

Control whether local declarations require namespace qualification in instances:

### elementFormDefault

```xml
<xs:schema targetNamespace="http://example.com/ns"
           elementFormDefault="qualified">
```

| Value | Instance Requirement |
|-------|---------------------|
| `unqualified` (default) | Local elements have no namespace |
| `qualified` | Local elements must use target namespace |

### attributeFormDefault

```xml
<xs:schema targetNamespace="http://example.com/ns"
           attributeFormDefault="unqualified">
```

| Value | Instance Requirement |
|-------|---------------------|
| `unqualified` (default) | Local attributes have no namespace |
| `qualified` | Local attributes must use target namespace |

### Individual Override

Override form defaults on specific declarations:

```xml
<xs:element name="localElement" type="xs:string" form="qualified"/>
<xs:attribute name="localAttr" type="xs:string" form="unqualified"/>
```

### Example

```xml
<!-- Schema with elementFormDefault="unqualified" -->
<xs:schema targetNamespace="http://example.com/ns"
           xmlns:tns="http://example.com/ns"
           elementFormDefault="unqualified">
  <xs:element name="root" type="tns:RootType"/>
  <xs:complexType name="RootType">
    <xs:sequence>
      <xs:element name="child" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>

<!-- Valid instance: root is qualified, child is not -->
<tns:root xmlns:tns="http://example.com/ns">
  <child>content</child>
</tns:root>
```

Spec refs: docs/spec/xml/structures.xml#declare-schema.

## Schema Composition

### xs:include

Incorporates another schema with the **same** target namespace:

```xml
<xs:schema targetNamespace="http://example.com/ns">
  <xs:include schemaLocation="types.xsd"/>
  <!-- Components from types.xsd are now available -->
</xs:schema>
```

**Rules:**

- Included schema must have same target namespace (or no target namespace)
- `schemaLocation` is required and specifies the schema document to include
- Multiple includes of the same schema document are allowed; component identity rules still apply and processors may avoid reprocessing identical includes

### Chameleon Include

When an included schema has **no target namespace**, its components adopt the including schema's namespace:

```xml
<!-- base-types.xsd (no targetNamespace) -->
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="AddressType">
    <xs:sequence>
      <xs:element name="street" type="xs:string"/>
      <xs:element name="city" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>

<!-- main.xsd -->
<xs:schema targetNamespace="http://example.com/ns"
           xmlns:tns="http://example.com/ns">
  <xs:include schemaLocation="base-types.xsd"/>
  <!-- AddressType is now in http://example.com/ns namespace -->
  <xs:element name="address" type="tns:AddressType"/>
</xs:schema>
```

**Chameleon Behavior:**

- All global components from the included schema adopt the target namespace
- Type references within the included schema are resolved in the new namespace
- The same chameleon schema can be included into multiple schemas with different target namespaces

### xs:import

References components from a **different** namespace:

```xml
<xs:schema targetNamespace="http://example.com/ipo"
           xmlns:addr="http://example.com/address">
  <xs:import namespace="http://example.com/address"
             schemaLocation="address.xsd"/>
  
  <xs:complexType name="OrderType">
    <xs:sequence>
      <xs:element name="shipTo" type="addr:AddressType"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>
```

**Rules:**

- `namespace` attribute specifies the imported namespace
- `schemaLocation` is a **hint only** — the processor may use other means to locate the schema (catalog, pre-loaded schemas, etc.)
- Import with no `namespace` attribute imports "no namespace" components

**Multiple Imports:**

The same namespace can be imported multiple times:

```xml
<xs:import namespace="http://example.com/common" schemaLocation="common-types.xsd"/>
<xs:import namespace="http://example.com/common" schemaLocation="common-elements.xsd"/>
```

When a namespace is imported multiple times:
- All schema documents for that namespace are combined
- Component definitions must be consistent (no conflicts)
- Duplicate identical definitions are allowed

**Import without schemaLocation:**

```xml
<xs:import namespace="http://example.com/external"/>
```

The processor must locate the schema through other means (application-provided, catalog lookup, etc.).

**Importing No-Namespace Components:**

```xml
<xs:import schemaLocation="no-ns-types.xsd"/>
<!-- No namespace attribute = import from "no namespace" -->
```

### xs:redefine (Deprecated)

Includes and modifies components from another schema:

```xml
<xs:schema targetNamespace="http://example.com/ns">
  <xs:redefine schemaLocation="base.xsd">
    <xs:complexType name="BaseType">
      <xs:complexContent>
        <xs:extension base="BaseType">
          <xs:sequence>
            <xs:element name="extra" type="xs:string"/>
          </xs:sequence>
        </xs:extension>
      </xs:complexContent>
    </xs:complexType>
  </xs:redefine>
</xs:schema>
```

**Note:** `xs:redefine` is deprecated in XSD 1.1 in favor of `xs:override`.

Spec refs: docs/spec/xml/structures.xml#src-include, docs/spec/xml/structures.xml#src-import, docs/spec/xml/structures.xml#composition-schemaImport.

## Instance Namespace Attributes

The **XML Schema Instance** namespace (`http://www.w3.org/2001/XMLSchema-instance`) provides special attributes for instance documents:

### xsi:type

Overrides the declared type with a derived type:

```xml
<shape xsi:type="CircleType" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <radius>5</radius>
</shape>
```

### xsi:nil

Indicates element is explicitly null (requires `nillable="true"`):

```xml
<shipDate xsi:nil="true" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"/>
```

### xsi:schemaLocation

Provides schema location hints for namespaced elements:

```xml
<root xmlns="http://example.com/ns"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:schemaLocation="http://example.com/ns schema.xsd">
```

Format: space-separated pairs of (namespace URI, schema location)

### xsi:noNamespaceSchemaLocation

Provides schema location for elements with no namespace:

```xml
<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:noNamespaceSchemaLocation="schema.xsd">
```

Spec refs: docs/spec/xml/structures.xml#xsi_type, docs/spec/xml/structures.xml#xsi_nil, docs/spec/xml/structures.xml#xsi_schemaLocation.

## Reserved Namespaces

| Namespace | Prefix | Purpose |
|-----------|--------|---------|
| `http://www.w3.org/2001/XMLSchema` | `xs` or `xsd` | Schema definition language |
| `http://www.w3.org/2001/XMLSchema-instance` | `xsi` | Instance document attributes |
| `http://www.w3.org/XML/1998/namespace` | `xml` | XML built-in attributes (`xml:lang`, `xml:space`) |
| `http://www.w3.org/2000/xmlns/` | `xmlns` | Namespace declarations (implicit) |

**Constraints:**

- `xml` prefix is implicitly bound; must not be rebound to other URIs
- `xmlns` prefix is reserved for namespace declarations; cannot be used for elements
- No prefix may bind to the `xmlns` namespace URI

Spec refs: docs/spec/xml/structures.xml#declare-schema.

## Validation Considerations

### Element Matching

An instance element matches a declaration when:

1. Local names are identical
2. Namespaces match:
   - Global elements: instance namespace must equal target namespace
   - Local elements: depends on form (qualified or unqualified)

### Attribute Matching

An instance attribute matches a declaration when:

1. Local names are identical
2. Namespace requirements:
   - Global attributes: must be explicitly prefixed with target namespace unless the schema has no `targetNamespace`
   - Local attributes: typically no namespace (unqualified by default)

### No Namespace vs Absent

- "No namespace" means the element/attribute explicitly has no namespace URI
- This is different from "in the default namespace"
- Unqualified local declarations match elements with no namespace

### Common Patterns

**All-qualified (elements in target namespace):**

```xml
<xs:schema targetNamespace="http://example.com/ns"
           elementFormDefault="qualified"
           attributeFormDefault="unqualified">
```

**Mixed qualification:**

```xml
<!-- Global elements qualified, local elements unqualified -->
<xs:schema targetNamespace="http://example.com/ns"
           elementFormDefault="unqualified">
```

**No namespace:**

```xml
<!-- All components in no namespace -->
<xs:schema>
  <xs:element name="root">...</xs:element>
</xs:schema>
```

Spec refs: docs/spec/xml/structures.xml#cvc-elt, docs/spec/xml/structures.xml#cvc-attribute.
