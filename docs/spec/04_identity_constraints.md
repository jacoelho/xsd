# Identity Constraints

## Contents

- [Overview](#overview)
- [Constraint Types](#constraint-types)
  - [xs:unique](#xsunique)
  - [xs:key](#xskey)
  - [xs:keyref](#xskeyref)
- [Selector and Field XPaths](#selector-and-field-xpaths)
- [Validation Rules](#validation-rules)
- [Example](#example)

---

## Overview

Identity constraints enforce uniqueness and referential integrity within an XML document, analogous to primary keys and foreign keys in databases. They are declared within element declarations and use XPath expressions to identify the scope and fields.

**Key Properties:**

| Property | Description |
|----------|-------------|
| `name` | NCName identifying the constraint |
| `selector` | XPath selecting the set of elements to constrain |
| `fields` | One or more XPaths selecting values within each selected element |
| `referenced key` | (keyref only) The key or unique constraint being referenced |

Spec refs: docs/spec/xml/structures.xml#c&Constraint;_Definitions.

## Constraint Types

### xs:unique

Specifies that field values must be unique within the selected scope. Elements with missing fields are excluded from the uniqueness check.

```xml
<xs:element name="employees">
  <xs:complexType>
    <xs:sequence>
      <xs:element name="employee" maxOccurs="unbounded">
        <xs:complexType>
          <xs:sequence>
            <xs:element name="name" type="xs:string"/>
            <xs:element name="email" type="xs:string" minOccurs="0"/>
          </xs:sequence>
          <xs:attribute name="id" type="xs:string"/>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>

  <!-- Each employee must have a unique id -->
  <xs:unique name="uniqueEmployeeId">
    <xs:selector xpath="employee"/>
    <xs:field xpath="@id"/>
  </xs:unique>
</xs:element>
```

**Behavior:**

- If any field evaluates to empty (absent), that element is excluded from checking
- Duplicate values among remaining elements cause a validation error

### xs:key

Like `xs:unique`, but additionally requires that all fields must be present for every selected element.

```xml
<xs:element name="catalog">
  <xs:complexType>
    <xs:sequence>
      <xs:element name="product" maxOccurs="unbounded">
        <xs:complexType>
          <xs:sequence>
            <xs:element name="name" type="xs:string"/>
            <xs:element name="price" type="xs:decimal"/>
          </xs:sequence>
          <xs:attribute name="sku" type="xs:string" use="required"/>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>

  <!-- Every product must have a unique, non-null sku -->
  <xs:key name="productKey">
    <xs:selector xpath="product"/>
    <xs:field xpath="@sku"/>
  </xs:key>
</xs:element>
```

**Behavior:**

- All selected elements must have values for all fields
- All field value combinations must be unique
- Missing fields cause a validation error (unlike `xs:unique`)
- Fields must not select elements whose declarations are `nillable="true"`

### xs:keyref

Specifies that field values must correspond to existing values of a referenced `xs:key` or `xs:unique`.

```xml
<xs:element name="order">
  <xs:complexType>
    <xs:sequence>
      <xs:element name="products">
        <xs:complexType>
          <xs:sequence>
            <xs:element name="product" maxOccurs="unbounded">
              <xs:complexType>
                <xs:attribute name="sku" type="xs:string" use="required"/>
                <xs:attribute name="name" type="xs:string"/>
              </xs:complexType>
            </xs:element>
          </xs:sequence>
        </xs:complexType>
      </xs:element>
      <xs:element name="lineItems">
        <xs:complexType>
          <xs:sequence>
            <xs:element name="item" maxOccurs="unbounded">
              <xs:complexType>
                <xs:attribute name="productSku" type="xs:string" use="required"/>
                <xs:attribute name="quantity" type="xs:positiveInteger"/>
              </xs:complexType>
            </xs:element>
          </xs:sequence>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>

  <xs:key name="productKey">
    <xs:selector xpath="products/product"/>
    <xs:field xpath="@sku"/>
  </xs:key>

  <!-- Each line item must reference an existing product -->
  <xs:keyref name="itemProductRef" refer="productKey">
    <xs:selector xpath="lineItems/item"/>
    <xs:field xpath="@productSku"/>
  </xs:keyref>
</xs:element>
```

**Behavior:**

- Each selected element's field values must match some tuple in the referenced key
- The number of fields must match the referenced key's field count
- Values of differing types can compare equal only if one type derives from the other and the value is in both value spaces

**Missing Fields in Keyref:**

Per the spec, if any field of a keyref evaluates to an empty node-set (the target node is absent or nilled), that entire tuple is **excluded** from referential checking. This is defined behavior:

- Absent field → tuple excluded from keyref check (no error)
- All fields present → tuple must exist in referenced key (error if not found)

**Keyref Resolution Scope:**

Keyref matching only succeeds against key/unique values available within the subtree of the element where the constraints are applied; keyrefs cannot target values outside that subtree.

Spec refs: docs/spec/xml/structures.xml#c&Constraint;_Definitions, docs/spec/xml/structures.xml#cvc-&constraint;.

## Selector and Field XPaths

The XPath expressions used in identity constraints are a restricted subset of XPath 1.0:

**Selector XPath:**

- Must select element nodes (not attributes or text)
- Can use child axis (explicit or abbreviated)
- Can use `|` for union
- No predicates allowed (unlike full XPath 1.0)

**Field XPath:**

- Must select a single node with simple type content
- Can select attributes (`@attr`) or elements with simple content
- Cannot select elements with complex content
- Paths may start with `.//` and may end in an attribute step

**Allowed Patterns:**

```
selector := path ( '|' path )*
path     := ('.//')? step ( '/' step )*
step     := '.' | nametest
nametest := QName | '*' | NCName ':' '*'
field    := fpath ( '|' fpath )*
fpath    := ('.//')? ( step '/' )* ( step | '@' nametest )
```

**Examples:**

```xml
<!-- Select all item elements anywhere under the context -->
<xs:selector xpath=".//item"/>

<!-- Select item elements that are direct children -->
<xs:selector xpath="item"/>

<!-- Select from multiple paths -->
<xs:selector xpath="books/book | magazines/magazine"/>

<!-- Field selecting an attribute -->
<xs:field xpath="@id"/>

<!-- Field selecting element content -->
<xs:field xpath="isbn"/>

<!-- Field with path -->
<xs:field xpath="author/@id"/>
```

Spec refs: docs/spec/xml/structures.xml#coss-&constraint;.

## Validation Rules

### Scope

Identity constraints are local to the element where they are defined:

- A constraint under `<purchaseReport>` only applies within each `<purchaseReport>` instance
- Separate `<purchaseReport>` elements have independent constraint checking

### Composite Keys

Multiple fields form a composite key—the combination must be unique:

```xml
<xs:key name="compositeKey">
  <xs:selector xpath="record"/>
  <xs:field xpath="@year"/>
  <xs:field xpath="@month"/>
  <xs:field xpath="@region"/>
</xs:key>
```

### Value Comparison

Field values are compared after schema normalization:

- Whitespace is collapsed according to the field's type
- For numeric types, value equality is used (e.g., `1.0` equals `1`)
- For string types, lexical comparison is used

### Validation Process

1. Evaluate the selector XPath to get the qualified node set
2. For each selected element, evaluate each field XPath
3. For `xs:key`: verify all fields have values; error if any missing
4. Collect value tuples for all qualifying elements
5. For `xs:unique`/`xs:key`: check for duplicate tuples
6. For `xs:keyref`: verify each tuple exists in the referenced key's set

Spec refs: docs/spec/xml/structures.xml#cvc-&constraint;.

## Example

A complete example with quarterly report data:

```xml
<xs:element name="purchaseReport">
  <xs:complexType>
    <xs:sequence>
      <xs:element name="regions">
        <xs:complexType>
          <xs:sequence>
            <xs:element name="zip" maxOccurs="unbounded">
              <xs:complexType>
                <xs:sequence>
                  <xs:element name="part" maxOccurs="unbounded">
                    <xs:complexType>
                      <xs:attribute name="number" type="xs:string" use="required"/>
                      <xs:attribute name="quantity" type="xs:integer"/>
                    </xs:complexType>
                  </xs:element>
                </xs:sequence>
                <xs:attribute name="code" type="xs:string" use="required"/>
              </xs:complexType>
            </xs:element>
          </xs:sequence>
        </xs:complexType>
      </xs:element>
      <xs:element name="parts">
        <xs:complexType>
          <xs:sequence>
            <xs:element name="part" maxOccurs="unbounded">
              <xs:complexType>
                <xs:attribute name="number" type="xs:string" use="required"/>
                <xs:attribute name="name" type="xs:string"/>
              </xs:complexType>
            </xs:element>
          </xs:sequence>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>

  <!-- Each zip code must be unique within the report -->
  <xs:unique name="uniqueZip">
    <xs:selector xpath="regions/zip"/>
    <xs:field xpath="@code"/>
  </xs:unique>

  <!-- Part numbers in the master list must be unique and present -->
  <xs:key name="partKey">
    <xs:selector xpath="parts/part"/>
    <xs:field xpath="@number"/>
  </xs:key>

  <!-- Parts referenced in regions must exist in master list -->
  <xs:keyref name="partRef" refer="partKey">
    <xs:selector xpath="regions/zip/part"/>
    <xs:field xpath="@number"/>
  </xs:keyref>
</xs:element>
```

**Validation Checks:**

1. `uniqueZip`: No two `<zip>` elements can have the same `@code`
2. `partKey`: Every `<part>` in `<parts>` must have a unique `@number`
3. `partRef`: Every `<part>` in `<regions>` must reference an existing part number

### ID/IDREF (Built-in Constraints)

The types `xs:ID` and `xs:IDREF` provide document-wide identity constraints independent of `xs:key`/`xs:keyref`:

| Type | Scope | Constraint |
|------|-------|------------|
| `xs:ID` | Entire document | All values must be unique |
| `xs:IDREF` | Entire document | Must match an existing ID |
| `xs:IDREFS` | Entire document | Each item must match an existing ID |

**Key Differences from key/keyref:**

| Aspect | ID/IDREF | key/keyref |
|--------|----------|------------|
| Scope | Always entire document | Defined by constraint's containing element |
| Declaration | Implicit via type | Explicit constraint declaration |
| Multiplicity | At most one ID attribute per element | Multiple keys per element allowed |
| Field selection | Always the attribute value | XPath expression |
| Composite keys | Not supported | Supported via multiple fields |

**ID Constraints:**

- All `xs:ID` values must be unique across the **entire document** (not just within a scope)
- Each element can have **at most one** attribute of type `xs:ID`
- ID values must be valid NCNames (no colons, must start with letter or underscore)

Note: The “at most one ID attribute per element” rule is enforced during
instance validation. A schema is not rejected solely because an `anyAttribute`
wildcard could admit ID-typed attributes; the violation is detected if it
occurs in an instance.

**IDREF Constraints:**

- Every `xs:IDREF` value must match some `xs:ID` value in the document
- `xs:IDREFS` is a whitespace-separated list; **each item** must match an existing ID
- IDREF validation occurs **after** the entire document is processed (forward references allowed)

**Error Codes:**

| Code | Condition |
|------|-----------|
| `cvc-id.1` | Duplicate ID value |
| `cvc-id.2` | IDREF value has no matching ID |
| `cvc-id.3` | Multiple ID attributes on same element |

These constraints are enforced automatically based on type, not via explicit constraint declarations.

Spec refs: docs/spec/xml/datatypes.xml#ID, docs/spec/xml/datatypes.xml#IDREF, docs/spec/xml/datatypes.xml#IDREFS.
