# XML Schema Primer

## Contents

- [Basic Structure of a Schema](#basic-structure-of-a-schema)
- [Example: Purchase Order Schema](#example-purchase-order-schema)
- [Key Concepts Illustrated](#key-concepts-illustrated)
- [Advanced Concepts Preview](#advanced-concepts-preview)

---

XML Schema defines the structure and data types of XML documents. A schema is a collection of component definitions (elements, attributes, types, etc.) that constrain what an XML document can contain. An XML document that conforms to a schema is called an **instance document**.

This Primer introduces the main features of XML Schema 1.0 through examples. For complete normative details, see the Structures and Datatypes specifications.

## Basic Structure of a Schema

A schema is typically written as an XML document using the XML Schema vocabulary (elements like `<xs:element>`, `<xs:complexType>`, etc., in the XML Schema namespace). The outermost element is `<xs:schema>`, which may specify:

- A `targetNamespace` for the schema
- Form defaults for element and attribute qualification (`elementFormDefault`, `attributeFormDefault`)

Inside, the schema contains:

- **Declarations** — for elements and attributes
- **Definitions** — for complex and simple types
- **Model groups** — reusable content models
- **Attribute groups** — reusable attribute sets

## Example: Purchase Order Schema

The following schema defines a simple purchase order format (simplified from the W3C Primer):

```xml
<xsd:schema xmlns:xsd="http://www.w3.org/2001/XMLSchema"
            elementFormDefault="unqualified"
            attributeFormDefault="unqualified">
  <xsd:annotation>
    <xsd:documentation xml:lang="en">
      Purchase order schema for Example.com.
      Copyright 2000 Example.com. All rights reserved.
    </xsd:documentation>
  </xsd:annotation>

  <!-- Element declarations -->
  <xsd:element name="purchaseOrder" type="PurchaseOrderType"/>
  <xsd:element name="comment" type="xsd:string"/>

  <!-- Complex type definitions -->
  <xsd:complexType name="PurchaseOrderType">
    <xsd:sequence>
      <xsd:element name="shipTo" type="USAddress"/>
      <xsd:element name="billTo" type="USAddress"/>
      <xsd:element ref="comment" minOccurs="0"/>
      <xsd:element name="items" type="Items"/>
    </xsd:sequence>
    <xsd:attribute name="orderDate" type="xsd:date"/>
  </xsd:complexType>

  <xsd:complexType name="USAddress">
    <xsd:sequence>
      <xsd:element name="name" type="xsd:string"/>
      <xsd:element name="street" type="xsd:string"/>
      <xsd:element name="city" type="xsd:string"/>
      <xsd:element name="state" type="xsd:string"/>
      <xsd:element name="zip" type="xsd:decimal"/>
    </xsd:sequence>
    <xsd:attribute name="country" type="xsd:NMTOKEN" fixed="US"/>
  </xsd:complexType>

  <xsd:complexType name="Items">
    <xsd:sequence>
      <xsd:element name="item" minOccurs="0" maxOccurs="unbounded">
        <xsd:complexType>
          <xsd:sequence>
            <xsd:element name="productName" type="xsd:string"/>
            <xsd:element name="quantity">
              <xsd:simpleType>
                <xsd:restriction base="xsd:positiveInteger">
                  <xsd:maxExclusive value="100"/>
                </xsd:restriction>
              </xsd:simpleType>
            </xsd:element>
            <xsd:element name="USPrice" type="xsd:decimal"/>
            <xsd:element ref="comment" minOccurs="0"/>
            <xsd:element name="shipDate" type="xsd:date" minOccurs="0"/>
          </xsd:sequence>
          <xsd:attribute name="partNum" type="SKU" use="required"/>
        </xsd:complexType>
      </xsd:element>
    </xsd:sequence>
  </xsd:complexType>

  <!-- Simple type definition -->
  <xsd:simpleType name="SKU">
    <xsd:restriction base="xsd:string">
      <xsd:pattern value="\d{3}-[A-Z]{2}"/>
    </xsd:restriction>
  </xsd:simpleType>
</xsd:schema>
```

### Corresponding Instance Document

Here is an XML document valid against the above schema:

```xml
<purchaseOrder orderDate="1999-10-20">
  <shipTo country="US">
    <name>Alice Smith</name>
    <street>123 Maple Street</street>
    <city>Mill Valley</city>
    <state>CA</state>
    <zip>90952</zip>
  </shipTo>
  <billTo country="US">
    <name>Robert Smith</name>
    <street>8 Oak Avenue</street>
    <city>Old Town</city>
    <state>PA</state>
    <zip>95819</zip>
  </billTo>
  <comment>Hurry, my lawn is going wild</comment>
  <items>
    <item partNum="872-AA">
      <productName>Lawnmower</productName>
      <quantity>1</quantity>
      <USPrice>148.95</USPrice>
      <comment>Confirm this is electric</comment>
    </item>
    <item partNum="926-AA">
      <productName>Baby Monitor</productName>
      <quantity>1</quantity>
      <USPrice>39.98</USPrice>
      <shipDate>1999-05-21</shipDate>
    </item>
  </items>
</purchaseOrder>
```

## Key Concepts Illustrated

### Global Elements and Types

- **Global elements** (`purchaseOrder`, `comment`) are declared at the schema top level
- **Global types** (`PurchaseOrderType`, `USAddress`, `Items`) are reusable blueprints

### Element and Type Relationship

The schema separates element declarations from type definitions. The element `purchaseOrder` is declared with type `PurchaseOrderType`. The same type could be used by multiple elements if needed.

### Local vs Global Definitions

- `USAddress` and `Items` are **global** type definitions (reusable anywhere)
- The type of `<item>` is **anonymous** (defined inline, tied to that context)
- The `quantity` element has an **anonymous simple type** restriction

### Default and Fixed Values

The attribute `country` in `USAddress` has `fixed="US"`:

- If present in the instance, it must equal `"US"`
- If omitted, the value `"US"` is assumed

Using `default="value"` instead would supply a default that can be overridden.

### Occurrence Constraints

- `minOccurs="0"` makes an element optional
- `maxOccurs="unbounded"` allows unlimited repetition
- Default is `minOccurs="1"` and `maxOccurs="1"`

### Facets on Simple Types

- The `SKU` type uses a `pattern` facet to constrain values to a regex
- The `quantity` element uses `maxExclusive` to cap the value at 100

### Namespace and Qualification

This schema uses no `targetNamespace`, meaning all names are unqualified. The `elementFormDefault="unqualified"` indicates local elements appear without namespace prefix in instances.

## Advanced Concepts Preview

XML Schema provides additional features for complex scenarios:

### Namespaces and Qualification

Schemas typically assign a target namespace and qualify element names. This becomes important when combining schemas or ensuring global uniqueness.

### Multiple Schema Files

Schemas can be split across files using:

- `<xs:include>` — incorporates another schema with the same target namespace
- `<xs:import>` — references components from a different namespace
- `<xs:redefine>` — includes and modifies components (deprecated in XSD 1.1)

### Type Derivation

Complex types can be **extended** or **restricted**:

- **Extension** adds new content (elements or attributes)
- **Restriction** narrows allowed content (tightening occurrence, fixing values)

### Substitution Groups

Allow one element to stand in for another when declared as part of a substitution group. This enables polymorphism in content models.

### Identity Constraints

Express uniqueness and referential integrity:

- `<xs:unique>` — values must be unique within a scope
- `<xs:key>` — like unique, but fields must be present
- `<xs:keyref>` — foreign key reference to a key

### Wildcards

- `<xs:any>` — allows elements from specified namespaces
- `<xs:anyAttribute>` — allows attributes from specified namespaces

### Nil Values

Setting `xsi:nil="true"` indicates an element is present but has no content. The element's declaration must have `nillable="true"`.

### Annotations

`<xs:annotation>` and `<xs:documentation>` allow schema authors to include human-readable notes without affecting validation.
