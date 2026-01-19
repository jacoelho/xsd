# Validation Rules and Process

## Contents

- [Overview](#overview)
- [Validation Process](#validation-process)
- [Element Validation](#element-validation)
  - [Element Locally Valid (Element)](#element-locally-valid-element)
  - [Element Locally Valid (Type)](#element-locally-valid-type)
  - [Element Locally Valid (Complex Type)](#element-locally-valid-complex-type)
- [Attribute Validation](#attribute-validation)
- [Type Override with xsi:type](#type-override-with-xsitype)
  - [Type Derivation OK](#type-derivation-ok)
- [Nil Elements](#nil-elements)
- [Post-Schema-Validation Infoset (PSVI)](#post-schema-validation-infoset-psvi)
- [Error Codes](#error-codes)

---

## Overview

Schema-validity assessment determines whether an XML document conforms to a schema. The process:

1. Matches elements and attributes to schema declarations
2. Validates content against type definitions
3. Enforces occurrence, identity, and derivation constraints
4. Augments the document with type information (PSVI)

Spec refs: docs/spec/xml/structures.xml#cvc-elt, docs/spec/xml/structures.xml#cvc-type, docs/spec/xml/structures.xml#cvc-complex-type.

## Validation Process

Validation operates top-down:

1. **Locate schema components** — Find declarations for the document's elements and attributes using namespace and name matching, `xsi:schemaLocation` hints, or application configuration

2. **Validate root element** — Match to a global element declaration and validate against its type

3. **Recursively validate children** — For each child element:
   - Find its declaration via the parent's content model
   - Validate against its type definition
   - Check occurrence constraints (minOccurs/maxOccurs)

4. **Validate attributes** — Match each attribute to a declaration and validate its value

5. **Check identity constraints** — After processing content, verify key/unique/keyref and ID/IDREF constraints

6. **Determine outcome** — Valid if all rules pass; invalid with specific error codes otherwise

Spec refs: docs/spec/xml/structures.xml#cvc-assess-elt, docs/spec/xml/structures.xml#cvc-assess-attr.

## Element Validation

### Element Locally Valid (Element)

An element is valid against its declaration if:

| Rule | Requirement |
|------|-------------|
| Name match | Element's QName matches declaration's `{name}` and `{target namespace}` |
| Type valid | Content is valid per `{type definition}` |
| Not abstract | Declaration is not `abstract="true"` |
| Nillable | If `xsi:nil="true"`, declaration has `nillable="true"` |
| Nil content | If nilled, element has no content (no child elements, no text) |
| Value constraint | If empty (no element or character children) with default/fixed, value is applied; if fixed and not empty, content must match |
| Identity constraints | All key/unique/keyref on the declaration are satisfied |

### Element Locally Valid (Type)

Given an element with type T (after `xsi:type` resolution):

**If T is abstract:**

- Element is invalid

**If T is a simple type:**

- Element must have no child elements
- Text content must be valid for type T
- No attributes other than `xsi:*` are allowed when the type is a simple type definition

**If T is a complex type:**

- Element must satisfy "Element Locally Valid (Complex Type)"

### Element Locally Valid (Complex Type)

An element with complex type CT is valid if:

**Content Model:**

- Child elements match CT's particle (sequence/choice/all)
- Order and counts satisfy occurrence constraints
- No unexpected elements (unless allowed by wildcard)
- No character data if not `mixed="true"` (whitespace allowed)

**Child Elements:**

- Each child is recursively valid against its declaration/type

**Attributes:**

- All required attributes are present
- All present attributes are declared or allowed by wildcard
- All attribute values are valid for their types
- Fixed values match if specified

**End of Content:**

- Any remaining required particles are satisfied
- No missing required children

Spec refs: docs/spec/xml/structures.xml#cvc-elt, docs/spec/xml/structures.xml#cvc-type, docs/spec/xml/structures.xml#cvc-complex-type.

## Attribute Validation

For each attribute on an element:

1. **Find declaration** — Match by name/namespace in the element's complex type `{attribute uses}` or `{attribute wildcard}`

2. **Check presence rules:**
   - If `use="required"` and missing → error
   - If `use="prohibited"` and present → error

3. **Validate value:**
   - Parse according to declared simple type
   - Check all type facets (pattern, enumeration, bounds, etc.)

4. **Apply constraints:**
   - If `fixed`, value must equal fixed value
   - If `default` and absent, default is supplied in PSVI

5. **Wildcard processing** (if matched by `xs:anyAttribute`):
   - `strict` — must find declaration, must validate
   - `lax` — validate if declaration found, otherwise accept
   - `skip` — accept without validation

Spec refs: docs/spec/xml/structures.xml#cvc-attribute, docs/spec/xml/structures.xml#cvc-au, docs/spec/xml/structures.xml#cvc-wildcard.

## Type Override with xsi:type

An instance element can specify a different type via `xsi:type`:

```xml
<shape xsi:type="CircleType">
  <radius>5</radius>
</shape>
```

**Validation Rules:**

1. Resolve `xsi:type` value to a type definition T
2. Verify T is derived from the element's declared type D
3. Check derivation is not blocked:
   - Element's `block` attribute doesn't include the derivation method
   - Type D's `{prohibited substitutions}` doesn't block it
4. Validate element content against type T

**Derivation Checking:**

- T must be the same as D, or
- T derives from D by extension (if not blocked), or
- T derives from D by restriction (if not blocked)

### Type Derivation OK

The "Type Derivation OK" constraint determines if type T can substitute for type D:

**For Simple Types:**

| Condition | Valid |
|-----------|-------|
| T = D | Yes |
| T restricts D (directly or transitively) | Yes, unless D has `final="restriction"` |
| T is a list and D is `anySimpleType` | Yes |
| T is a union and D is `anySimpleType` | Yes |

**For Complex Types:**

| Condition | Valid |
|-----------|-------|
| T = D | Yes |
| T extends D (directly or transitively) | Yes, unless blocked |
| T restricts D (directly or transitively) | Yes, unless blocked |
| D is `anyType` | Always yes (anyType is the ur-type) |

**Blocking Mechanisms:**

| Mechanism | Location | Effect |
|-----------|----------|--------|
| `block="extension"` | Element declaration | Blocks xsi:type to extension-derived types |
| `block="restriction"` | Element declaration | Blocks xsi:type to restriction-derived types |
| `final="extension"` | Type definition | Prevents extension derivation |
| `final="restriction"` | Type definition | Prevents restriction derivation |
| `{prohibited substitutions}` | Complex type | Inherited blocking from type definition |

**Derivation Chain Check:**

To verify T derives from D, the validator walks up T's derivation chain:

```
T → T.base → T.base.base → ... → D → ... → anyType
```

At each step, the derivation method (extension/restriction) must not be blocked.

Spec refs: docs/spec/xml/structures.xml#xsi_type, docs/spec/xml/structures.xml#cos-ct-derived-ok, docs/spec/xml/structures.xml#cos-st-derived-ok.

## Nil Elements

Elements can be explicitly nil using `xsi:nil="true"`:

```xml
<shipDate xsi:nil="true"/>
```

**Requirements:**

- Element declaration must have `nillable="true"`
- Element must have no content (no child elements, no text)
- Attributes are still allowed and validated according to the type definition
- A fixed value constraint is not allowed when the element is nilled

**PSVI:**

- Element's `[nil]` property is set to true
- Element's `[schema normalized value]` is absent

Spec refs: docs/spec/xml/structures.xml#xsi_nil, docs/spec/xml/structures.xml#cvc-elt.

## Post-Schema-Validation Infoset (PSVI)

Validation augments each element and attribute with additional properties:

**Element Contributions:**

| Property | Description |
|----------|-------------|
| `[validation context]` | Nearest ancestor element with `[schema information]` (or this element if it has it) |
| `[validity]` | `valid`, `invalid`, or `notKnown` |
| `[validation attempted]` | `full`, `partial`, or `none` |
| `[schema error code]` | List of error codes if invalid; otherwise absent |
| `[schema information]` | Present on the validation root element; set of namespace schema information items |
| `[element declaration]` | Optional; item isomorphic to the declaration component |
| `[type definition]` | The type used; either a component item or a name/namespace/anonymous summary |
| `[member type definition]` | For union simple types; the member type that validated the value |
| `[nil]` | Whether the element was nilled |
| `[schema normalized value]` | If applicable and valid: the normalized value for simple/simple-content types; if a value constraint is applied due to empty content, the canonical lexical representation of the constraint value; otherwise absent |
| `[schema default]` | Canonical lexical representation of the value constraint, if present; otherwise absent |
| `[schema specified]` | `schema` if the value constraint was applied due to empty content; otherwise `infoset` |

**Attribute Contributions:**

| Property | Description |
|----------|-------------|
| `[validation context]` | Nearest ancestor element with `[schema information]` |
| `[validity]` | `valid`, `invalid`, or `notKnown` |
| `[type definition]` | The simple type used; either a component item or a name/namespace/anonymous summary |
| `[member type definition]` | For union simple types; the member type that validated the value |
| `[schema normalized value]` | Normalized value if valid against the governing type; otherwise absent |
| `[schema default]` | Canonical lexical representation of the value constraint, if present; otherwise absent |
| `[schema specified]` | `schema` if the attribute was supplied by a value constraint due to absence; otherwise `infoset` |
| `[schema error code]` | List of error codes if invalid; otherwise absent |
| `[attribute declaration]` | Optional; item isomorphic to the declaration component |

Default application affects PSVI properties only; it does not add character children to the element. Defaulted attributes are added to the attribute set with `[schema specified] = schema`.

Spec refs: docs/spec/xml/structures.xml#sic-eltType, docs/spec/xml/structures.xml#sic-eltDefault, docs/spec/xml/structures.xml#sic-attrType, docs/spec/xml/structures.xml#sic-attrDefault.

## Error Codes

XML Schema defines `cvc-*` error codes for specific validation failures:

### Element Errors

| Code | Description |
|------|-------------|
| `cvc-elt.1` | Cannot find declaration for element |
| `cvc-elt.2` | Element matches abstract declaration |
| `cvc-elt.3.1` | xsi:nil on non-nillable element |
| `cvc-elt.3.2` | Nilled element has content |
| `cvc-elt.4.1` | No declaration resolvable for element |
| `cvc-elt.4.2` | Type not validly derived for xsi:type |
| `cvc-elt.4.3` | Type is abstract |
| `cvc-elt.5.1` | Element type is not simple (for simple content) |
| `cvc-elt.5.2.1` | Fixed value mismatch |
| `cvc-elt.5.2.2` | Element content invalid for type |

### Complex Type Errors

| Code | Description |
|------|-------------|
| `cvc-complex-type.2.1` | Empty content required but found |
| `cvc-complex-type.2.2` | Element-only has non-whitespace text |
| `cvc-complex-type.2.3` | Simple content has element children |
| `cvc-complex-type.2.4` | Content model not satisfied |
| `cvc-complex-type.3.1` | Unexpected attribute |
| `cvc-complex-type.3.2` | Attribute wildcard validation failure |
| `cvc-complex-type.4` | Required attribute missing |

### Attribute Errors

| Code | Description |
|------|-------------|
| `cvc-attribute.1` | Attribute not declared |
| `cvc-attribute.3` | Attribute value invalid for type |
| `cvc-attribute.4` | Fixed value mismatch |

### Type Errors

| Code | Description |
|------|-------------|
| `cvc-type.1` | Type definition is abstract |
| `cvc-type.2` | Type is simple but element has element content |
| `cvc-type.3.1` | Content not valid for simple type |

### Identity Constraint Errors

| Code | Description |
|------|-------------|
| `cvc-identity-constraint.1` | Keyref refers to unknown key |
| `cvc-identity-constraint.2` | Duplicate key value |
| `cvc-identity-constraint.3` | Key field has no value |
| `cvc-identity-constraint.4.1` | Duplicate unique value |
| `cvc-identity-constraint.4.2` | Keyref value not found in referenced key |
| `cvc-identity-constraint.4.3` | Keyref field count mismatch |

### ID/IDREF Errors

| Code | Description |
|------|-------------|
| `cvc-id.1` | Duplicate ID value |
| `cvc-id.2` | IDREF value has no matching ID |
| `cvc-id.3` | Multiple ID attributes on element |

### Datatype Errors

| Code | Description |
|------|-------------|
| `cvc-datatype-valid.1.1` | Value not in lexical space |
| `cvc-datatype-valid.1.2.1` | Pattern facet not satisfied |
| `cvc-datatype-valid.1.2.2` | Enumeration facet not satisfied |
| `cvc-datatype-valid.1.2.3` | Numeric bounds not satisfied |
| `cvc-length-valid` | Length constraint not satisfied |
| `cvc-minLength-valid` | Minimum length not satisfied |
| `cvc-maxLength-valid` | Maximum length not satisfied |
| `cvc-totalDigits-valid` | Total digits exceeded |
| `cvc-fractionDigits-valid` | Fraction digits exceeded |

### Assessment Errors

| Code | Description |
|------|-------------|
| `cvc-assess-elt.1.1.1` | No matching global element declaration |
| `cvc-assess-elt.1.1.2` | Element cannot be assessed (no schema) |
| `cvc-assess-elt.1.1.3` | Lax wildcard - element skipped |
| `cvc-assess-attr.1.1` | No matching attribute declaration |

### Particle and Model Group Errors

| Code | Description |
|------|-------------|
| `cvc-particle.1.1` | Element does not match particle |
| `cvc-particle.1.2` | Too few occurrences |
| `cvc-particle.1.3` | Too many occurrences |
| `cvc-model-group.1` | Content does not match model group |
| `cvc-au.1` | Attribute use not satisfied |

### Wildcard Errors

| Code | Description |
|------|-------------|
| `cvc-wildcard-namespace.1` | Element namespace not allowed by wildcard |
| `cvc-wildcard-namespace.2` | Attribute namespace not allowed by wildcard |

### Derivation Errors

| Code | Description |
|------|-------------|
| `cvc-type-derivation-ok.1` | Type not derived from base |
| `cvc-type-derivation-ok.2` | Derivation blocked by final |

Spec refs: docs/spec/xml/structures.xml#outcomes.
