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

1. **Locate schema components** — Find declarations for the document's elements and attributes using namespace and name matching within the supplied component set. The XSD specification permits location hints; this library never loads schemas from instance hints or over HTTP.

2. **Validate root element** — Match to a global element declaration and validate against its type

3. **Recursively validate children** — For each child element:
   - Find its declaration via the parent's content model
   - Validate against its type definition
   - Check occurrence constraints (minOccurs/maxOccurs)

4. **Validate attributes** — Match each attribute to a declaration and validate its value

5. **Check identity constraints** — After processing content, verify key/unique/keyref and ID/IDREF constraints

6. **Determine outcome** — Valid if all rules pass; invalid with specific error codes otherwise

For ID/IDREF contribution eligibility and the validation-root check, retain
IBS-Q001 in [interpretation boundaries](INTERPRETATIONS.md#remaining-source-integration-boundaries).
Content validity and successful declaration validation remain distinct premises;
the outline does not settle that contribution dependency.

Spec refs: docs/spec/xml/structures.xml#cvc-assess-elt, docs/spec/xml/structures.xml#cvc-assess-attr.

## Element Validation

### Element Locally Valid (Element)

An element is valid against its declaration if:

| Rule | Requirement |
|------|-------------|
| Name match | Element's QName matches declaration's `{name}` and `{target namespace}` |
| Type valid | Content is valid per `{type definition}` |
| Not abstract | Declaration is not `abstract="true"` |
| Nillable | If `xsi:nil` is present, declaration has `nillable="true"`, even when the attribute's value is false |
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
- Only the four intrinsic XSI attributes (`type`, `nil`, `schemaLocation` and
  `noNamespaceSchemaLocation`) are allowed; the exception does not cover other
  names in the XSI namespace

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
- Present attributes other than the four intrinsic XSI names are declared or
  allowed by wildcard
- All attribute values are valid for their types
- Fixed values match if specified

**End of Content:**

- Any remaining required particles are satisfied
- No missing required children

Spec refs: docs/spec/xml/structures.xml#cvc-elt, docs/spec/xml/structures.xml#cvc-type, docs/spec/xml/structures.xml#cvc-complex-type.

## Attribute Validation

The four [intrinsic XSI attributes](07_namespaces.md#intrinsic-components-and-schema-references)
have dedicated lexical and element-start rules. `c-aam` excludes exactly these
names from ordinary attribute-use and wildcard assessment, including a use's
fixed constraint. The separate required-use rule still demands their physical
presence when a schema references them with `use="required"`. An invalid present
value still counts as present. XSI constraints never supply absent attributes.

For each ordinary attribute on an element:

1. **Find declaration** — Match by name/namespace in the element's complex type `{attribute uses}` or `{attribute wildcard}`

2. **Check presence rules:**
   - If `use="required"` and missing → error
   - If `use="prohibited"` with `default` → schema error (defaults are not allowed)
   - `use="prohibited"` maps to no attribute use, including when `fixed` is
     present. An actual attribute then needs another applicable use or wildcard;
     the discarded source item supplies neither a fixed check nor a default.
     In restriction its name prevents inheritance of a matching base use, but
     never removes an explicit local or group use; extension retains base uses

3. **Validate value:**
   - Parse according to declared simple type
   - Check all type facets (pattern, enumeration, bounds, etc.)

4. **Apply constraints:**
   - Declaration-owned and ordinary use-owned fixed constraints compare typed
     values after datatype/facet validation; equal lexical aliases are accepted
     under the [selected correction](INTERPRETATIONS.md#i-009--fixed-attribute-use-values)
   - An absent optional use with a default or fixed constraint supplies an
     attribute in the PSVI

5. **Wildcard processing** (if matched by `xs:anyAttribute`):
   - `strict` — must find declaration, must validate
   - `lax` — validate if declaration found, otherwise accept
   - `skip` — accept without validation

The library adopts WG correction 4363: XSI default/fixed constraints supply no
absent attribute or identity value. Their datatype and source constraints remain
checked during construction. Physical XSI attributes retain typed identity
values; see [I-007](INTERPRETATIONS.md#i-007--xsi-attribute-uses-and-psvi-contributions).

The upstream W3C `attP031` valid result conflicts with the published
`use="prohibited"` mapping. The corpus retains that result as
`instance.oracle.w3c` provenance and expects invalid for its actual attribute
without an applicable use or wildcard; see
[I-008](INTERPRETATIONS.md#i-008--prohibited-attribute-use-with-fixed).

Spec refs: docs/spec/xml/structures.xml#declare-attribute,
docs/spec/xml/structures.xml#cvc-attribute, docs/spec/xml/structures.xml#cvc-au,
docs/spec/xml/structures.xml#c-aam,
docs/spec/xml/structures.xml#sic-attrDefault,
docs/spec/xml/structures.xml#cvc-wildcard.

QName declaration constraints applied under `xsi:type` preserve their schema
namespace context under the selected [QName constraint interpretation](INTERPRETATIONS.md#i-013--qname-element-constraints-under-xsitype).
The canonical/context integration gap is qualified separately from ordinary
explicit instance text, which uses instance bindings.

## Type Override with xsi:type

An instance element can specify a different type via `xsi:type`:

```xml
<shape xsi:type="CircleType">
  <radius>5</radius>
</shape>
```

**Validation Rules:**

For a declared element, `xsi:type` must be a valid QName resolving to a known
simple or complex type. XML whitespace is collapsed, and unprefixed values use
the instance's default namespace. Namespace declarations on that start tag
apply before resolution. An unknown name, unbound prefix or malformed QName is
invalid; instance hints do not load additional types.

The selected type must be validly derived from the declared type. The blocking
constraint combines the element's `block` and the declared complex type's
`{prohibited substitutions}`. It applies to every method used on the path to
that declared type, including both methods in a mixed extension/restriction
chain. Intermediate type blocks are not added to this constraint; substitution
groups have a separate intermediate-type rule.

The actual type must be nonabstract, and its content and attributes must
validate. `xsi:nil="true"` does not bypass QName, type, derivation or abstractness
checks. An abstract element declaration remains unusable even with a concrete
`xsi:type`.

On undeclared elements assessed through wildcards, physical `xsi:type` has the
same QName and type-existence checks. The latter adopts
[WG correction 4299](INTERPRETATIONS.md#i-014--xsitype-under-element-wildcards)
for the printed lax-fallback gap. Skipped subtrees remain unassessed. See
[element wildcard processing](03_complex_types.md#element-wildcards-xsany).

### Type Derivation OK

Identity uses no derivation method. Naming the identical declared type remains
valid under `block="#all"` and type `final="#all"`; actual content still
validates. `final` controls construction of new derived types, rather than
rejecting use of an already admitted type as itself.

| Selected type T relative to declared type D | Derivation condition |
| --- | --- |
| T is D | Identity, with no methods |
| T restricts simple D | Restriction, subject to the blocking constraint |
| T is list/union and D is `anySimpleType` | Simple-type derivation, subject to restriction blocking |
| T derives through a member of an unconstrained union D | Allowed under the simple derivation relation |
| T only derives through a member of a facet-restricted union D | Rejected under the selected restricted-union correction |
| T extends/restricts complex D | Every method on the T-to-D path must be permitted |
| D is `anyType` | The path still carries its methods; implicit restriction is not exempt from blocking |

A complex type with simple content can reach a simple declared type through its
actual base chain. This does not let an arbitrary simple type substitute for a
complex type with similar text content. Traversal stops at the requested type;
methods and blocks on unrelated ancestors do not participate.

Spec refs: [Element Locally Valid, clause 4](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-elt),
[Type Derivation OK (Complex)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-ct-derived-ok),
[Type Derivation OK (Simple)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cos-st-derived-ok),
and [restricted-union interpretation](INTERPRETATIONS.md#i-010--restricted-unions-retain-their-constraints).

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

The presence of `xsi:nil="false"` or `xsi:nil="0"` also requires a nillable
declaration. It does not nil the element, so ordinary content rules apply.

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
| `[member type definition]` | When a schema-normalized value is present for a union simple/simple-content type, the member that validated that value |
| `[nil]` | Whether the element was nilled |
| `[schema normalized value]` | For nonnilled simple/simple-content cases: the constraint's lexical form when clause 5.1 applied; otherwise the normalized instance value if type-valid; otherwise absent. See the ordered E1-56 cases below. |
| `[schema default]` | Canonical lexical representation of the value constraint, if present; otherwise absent |
| `[schema specified]` | `schema` exactly when Element Locally Valid (Element) clause 5.1 applied; otherwise `infoset` |

**Attribute Contributions:**

| Property | Description |
|----------|-------------|
| `[validation context]` | Nearest ancestor element with `[schema information]` |
| `[validity]` | `valid`, `invalid`, or `notKnown` |
| `[type definition]` | The simple type used; either a component item or a name/namespace/anonymous summary |
| `[member type definition]` | For a successfully validated union attribute value, the member type that validated it; absent on failed type validation |
| `[schema normalized value]` | Normalized value if valid against the governing type; otherwise absent |
| `[schema default]` | Canonical lexical representation of the value constraint, if present; otherwise absent |
| `[schema specified]` | `schema` if the attribute was supplied by a value constraint due to absence; otherwise `infoset` |
| `[schema error code]` | List of error codes if invalid; otherwise absent |
| `[attribute declaration]` | Optional; item isomorphic to the declaration component |

Default application affects PSVI properties only; it does not add character children to the element. Defaulted attributes are added to the attribute set with `[schema specified] = schema`.

The [E1-56 corrected reading](corrected-reading.md) applies to these tables.
In particular, an applied element default contributes its lexical form, which
can differ from the canonical representation still used by `[schema default]`.
Mixed-content elements have no `[schema normalized value]`. Type validity is
distinct from validity against every constraint on the declaration.

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
| `cvc-assess-elt.1.1.3` | No global declaration available through name resolution |
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
