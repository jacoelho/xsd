# Simple Types and Datatypes

## Contents

- [Overview](#overview)
- [Simple Type Properties](#simple-type-properties)
- [Type Varieties](#type-varieties)
- [Built-in Primitive Types](#built-in-primitive-types)
- [Built-in Derived Types](#built-in-derived-types)
- [Constraining Facets](#constraining-facets)
- [User-Derived Types](#user-derived-types)
- [Whitespace Normalization](#whitespace-normalization)
- [Type Validation](#type-validation)

---

## Overview

XML Schema Part 2: Datatypes defines simple types—types whose values are textual strings (as opposed to complex types which contain elements/attributes).

Simple types constrain the **lexical space** (valid string representations) and **value space** (abstract values) of element or attribute content.

Spec refs: docs/spec/xml/datatypes.xml#dt-datatype, docs/spec/xml/datatypes.xml#dt-lexical-space, docs/spec/xml/datatypes.xml#dt-value-space.

## Simple Type Properties

| Property | Description |
|----------|-------------|
| `name` | Optional NCName (anonymous types have none) |
| `target namespace` | Either absent or a namespace URI |
| `variety` | `atomic`, `list`, or `union` |
| `base type definition` | The type this derives from |
| `facets` | Constraining facets (pattern, length, bounds, etc.) |
| `fundamental facets` | Intrinsic properties (ordered, bounded, numeric, etc.) |
| `primitive type definition` | Ultimate primitive base (for atomic types) |

Spec refs: docs/spec/xml/datatypes.xml#rf-defn.

## Type Varieties

### Atomic

Single indivisible value (string, number, date, etc.):

```xml
<xs:simpleType name="Percentage">
  <xs:restriction base="xs:decimal">
    <xs:minInclusive value="0"/>
    <xs:maxInclusive value="100"/>
  </xs:restriction>
</xs:simpleType>
```

### List

Whitespace-separated sequence of atomic values:

```xml
<xs:simpleType name="IntegerList">
  <xs:list itemType="xs:integer"/>
</xs:simpleType>

<!-- Valid: "1 2 3 4 5" -->
```

**List Rules:**

- Items are separated by whitespace (space, tab, newline)
- Each item must be valid for the item type
- `length`, `minLength`, `maxLength` count items, not characters
- `pattern` on a list type applies to the **entire string** (not per-item); use pattern on the item type to constrain individual items
- Empty lists are allowed unless constrained by `length`/`minLength`, **except** the built-in list types `NMTOKENS`, `IDREFS`, and `ENTITIES`, which are defined as non-zero-length sequences (treat as implicit `minLength=1`)

### Union

Value can be any one of several member types:

```xml
<xs:simpleType name="StringOrInteger">
  <xs:union memberTypes="xs:string xs:integer"/>
</xs:simpleType>
```

**Union Rules:**

- Value is valid if it matches any member type
- Validation tries each member type in order
- The evaluation order can be overridden with `xsi:type`
- Only `pattern` and `enumeration` facets can be applied directly

Spec refs: docs/spec/xml/datatypes.xml#dt-atomic, docs/spec/xml/datatypes.xml#dt-list, docs/spec/xml/datatypes.xml#dt-union, docs/spec/xml/datatypes.xml#dt-memberTypes.

## Built-in Primitive Types

XML Schema defines 19 primitive datatypes:

Spec refs: docs/spec/xml/datatypes.xml#built-in-primitive-datatypes.

### String Types

| Type | Description | Example |
|------|-------------|---------|
| `string` | Any Unicode character sequence | `"Hello, World!"` |
| `anyURI` | URI reference | `"http://example.com/path"` |

### Boolean

| Type | Description | Lexical Values |
|------|-------------|----------------|
| `boolean` | True or false | `true`, `false`, `1`, `0` |

### Numeric Types

| Type | Description | Example |
|------|-------------|---------|
| `decimal` | Arbitrary precision decimal | `123.456`, `-0.5` |
| `float` | 32-bit IEEE 754 | `1.5E2`, `INF`, `-INF`, `NaN` |
| `double` | 64-bit IEEE 754 | `1.5E2`, `INF`, `-INF`, `NaN` |

### Date/Time Types

| Type | Format | Example |
|------|--------|---------|
| `dateTime` | `-?YYYY-MM-DDThh:mm:ss[.s+][Z\|(+\|-)hh:mm]` | `2025-01-18T14:30:00Z` |
| `date` | `-?YYYY-MM-DD[Z\|(+\|-)hh:mm]` | `2025-01-18` |
| `time` | `hh:mm:ss[.s+][Z\|(+\|-)hh:mm]` | `14:30:00` |
| `duration` | `-?PnYnMnDTnHnMnS` | `P1Y2M3DT4H5M6S` |
| `gYearMonth` | `-?YYYY-MM[Z\|(+\|-)hh:mm]` | `2025-01` |
| `gYear` | `-?YYYY[Z\|(+\|-)hh:mm]` | `2025` |
| `gMonthDay` | `--MM-DD[Z\|(+\|-)hh:mm]` | `--01-18` |
| `gDay` | `---DD[Z\|(+\|-)hh:mm]` | `---18` |
| `gMonth` | `--MM[Z\|(+\|-)hh:mm]` | `--01` |

`YYYY` is a four-or-more digit year; an optional leading '-' is allowed, '0000' is not allowed, and a leading '+' is not permitted. All g* types allow an optional time zone. For `duration`, an optional leading '-' is allowed and only the seconds field may be fractional.

Implementation note: fractional seconds are limited to 9 digits (nanosecond precision); longer fractions are rejected with an explicit error.

Implementation note: `xs:time` comparisons and range facets use the full UTC-normalized instant (including the reference date used during parsing). This means timezone offsets that cross midnight can change ordering, and derived facets must respect that ordering to match the W3C XSD 1.0 test suite (for example, a base `maxInclusive` of `12:00:00-10:00` makes a derived `maxInclusive` of `12:00:00-14:00` invalid because it is later when normalized to UTC).

### Binary Types

| Type | Description | Example |
|------|-------------|---------|
| `hexBinary` | Hex-encoded binary | `48656C6C6F` |
| `base64Binary` | Base64-encoded binary | `SGVsbG8=` |

### Name Types

| Type | Description |
|------|-------------|
| `QName` | Qualified name (prefix:local) | Requires namespace resolution |
| `NOTATION` | Declared notation name | Must match schema notation |

## Built-in Derived Types

Types derived from primitives via restriction:

Spec refs: docs/spec/xml/datatypes.xml#built-in-derived.

### From string

| Type | Description | Whitespace |
|------|-------------|------------|
| `normalizedString` | No CR, LF, TAB | replace |
| `token` | Collapsed, no leading/trailing spaces | collapse |
| `language` | Language code (BCP 47) | collapse |
| `Name` | XML Name | collapse |
| `NCName` | Non-colonized name | collapse |
| `NMTOKEN` | Name token | collapse |
| `NMTOKENS` | List of NMTOKENs (non-empty) | collapse |

### Identity Types (from NCName)

| Type | Description | Constraint |
|------|-------------|------------|
| `ID` | Unique identifier | Must be unique in document |
| `IDREF` | Reference to ID | Must match existing ID |
| `IDREFS` | List of IDREFs (non-empty) | Each must match existing ID |
| `ENTITY` | Unparsed entity name | Must match declared entity |
| `ENTITIES` | List of ENTITYs (non-empty) | Each must match declared entity |

### From decimal

| Type | Range |
|------|-------|
| `integer` | No fractional part |
| `nonPositiveInteger` | ≤ 0 |
| `negativeInteger` | < 0 |
| `nonNegativeInteger` | ≥ 0 |
| `positiveInteger` | > 0 |
| `long` | -2^63 to 2^63-1 |
| `int` | -2^31 to 2^31-1 |
| `short` | -32768 to 32767 |
| `byte` | -128 to 127 |
| `unsignedLong` | 0 to 2^64-1 |
| `unsignedInt` | 0 to 2^32-1 |
| `unsignedShort` | 0 to 65535 |
| `unsignedByte` | 0 to 255 |

### Type Hierarchy

The type hierarchy has two root types:

- **`anyType`** — The ur-type definition from which all complex types derive. It can contain any content (elements, attributes, text).
- **`anySimpleType`** — The base of all simple types. All 19 primitive types derive directly from it.

```
anyType (ur-type, base of all complex types)
    └── anySimpleType (base of all simple types)

anySimpleType
├── string
│   └── normalizedString
│       └── token
│           ├── language
│           ├── Name
│           │   └── NCName
│           │       ├── ID
│           │       ├── IDREF
│           │       └── ENTITY
│           └── NMTOKEN
├── decimal
│   └── integer
│       ├── nonPositiveInteger
│       │   └── negativeInteger
│       ├── nonNegativeInteger
│       │   ├── positiveInteger
│       │   └── unsignedLong
│       │       └── unsignedInt
│       │           └── unsignedShort
│       │               └── unsignedByte
│       └── long
│           └── int
│               └── short
│                   └── byte
├── float
├── double
├── boolean
├── duration
├── dateTime
├── date
├── time
├── gYearMonth
├── gYear
├── gMonthDay
├── gDay
├── gMonth
├── hexBinary
├── base64Binary
├── anyURI
├── QName
└── NOTATION
```

## Constraining Facets

Facets restrict the value/lexical space of a type:

Spec refs: docs/spec/xml/datatypes.xml#dt-constraining-facet.

### Length Facets

| Facet | Applies To | Description |
|-------|------------|-------------|
| `length` | string, binary, list | Exact length required |
| `minLength` | string, binary, list | Minimum length |
| `maxLength` | string, binary, list | Maximum length |

**Length Measurement:**

- For strings: number of characters
- For hexBinary: number of bytes (= hex digits / 2)
- For base64Binary: number of decoded bytes
- For lists: number of items

```xml
<xs:simpleType name="ZipCode">
  <xs:restriction base="xs:string">
    <xs:length value="5"/>
  </xs:restriction>
</xs:simpleType>
```

### Numeric Bounds

| Facet | Description |
|-------|-------------|
| `minInclusive` | Value ≥ bound |
| `minExclusive` | Value > bound |
| `maxInclusive` | Value ≤ bound |
| `maxExclusive` | Value < bound |

```xml
<xs:simpleType name="Percentage">
  <xs:restriction base="xs:decimal">
    <xs:minInclusive value="0"/>
    <xs:maxInclusive value="100"/>
  </xs:restriction>
</xs:simpleType>
```

### Digit Facets

| Facet | Applies To | Description |
|-------|------------|-------------|
| `totalDigits` | decimal types | Maximum total digits |
| `fractionDigits` | decimal types | Maximum digits after decimal |

```xml
<xs:simpleType name="Price">
  <xs:restriction base="xs:decimal">
    <xs:totalDigits value="10"/>
    <xs:fractionDigits value="2"/>
  </xs:restriction>
</xs:simpleType>
```

### Pattern

Regular expression constraint on lexical form:

```xml
<xs:simpleType name="PhoneNumber">
  <xs:restriction base="xs:string">
    <xs:pattern value="\d{3}-\d{3}-\d{4}"/>
  </xs:restriction>
</xs:simpleType>
```

**Pattern Notes:**

- Uses XML Schema regex syntax (see differences from Perl below)
- Pattern is anchored to entire value (no need for `^...$`)
- Multiple patterns are ANDed (value must match all)
- For list types, pattern applies to the **entire string** (use pattern on item type for per-item constraints)

**XML Schema Regex vs Perl/PCRE:**

| Feature | XSD 1.0 | Perl/PCRE |
|---------|---------|-----------|
| Anchors `^` `$` | Not used (implicit full match) | Explicit anchors |
| Lazy quantifiers `*?` `+?` | Not supported | Supported |
| Backreferences `\1` | Not supported | Supported |
| Possessive quantifiers `*+` | Not supported | Supported |
| Lookahead/lookbehind | Not supported | Supported |
| Unicode categories `\p{L}` | Supported | Supported |
| Unicode blocks `\p{IsBasicLatin}` | Supported | Different syntax |
| Character class subtraction `[a-z-[aeiou]]` | Supported | Not standard |
| Multi-char escapes `\s` `\d` `\w` | Supported (Unicode-aware) | Supported |
| `\i` `\c` (XML name chars) | Supported | Not available |

**XSD-specific character classes:**

- `\i` — initial XML name character (letter or `_`)
- `\I` — non-initial name character
- `\c` — XML name character (letter, digit, `.`, `-`, `_`, `:`)
- `\C` — non-name character

### Enumeration

Restricts to specific allowed values:

```xml
<xs:simpleType name="Size">
  <xs:restriction base="xs:string">
    <xs:enumeration value="small"/>
    <xs:enumeration value="medium"/>
    <xs:enumeration value="large"/>
  </xs:restriction>
</xs:simpleType>
```

### Whitespace

Controls whitespace normalization:

| Value | Effect |
|-------|--------|
| `preserve` | Keep all whitespace as-is |
| `replace` | Replace CR, LF, TAB with spaces |
| `collapse` | Replace, then collapse multiple spaces, trim ends |

```xml
<xs:simpleType name="NormalizedName">
  <xs:restriction base="xs:string">
    <xs:whiteSpace value="collapse"/>
  </xs:restriction>
</xs:simpleType>
```

**Restriction Rule:** The `whiteSpace` facet can only be **strengthened** during derivation by restriction:

- `preserve` → `replace` or `collapse` ✓
- `replace` → `collapse` ✓
- `collapse` → `replace` or `preserve` ✗ (schema error)

### Facet Applicability

| Facet | string | numeric | date/time | binary | list |
|-------|--------|---------|-----------|--------|------|
| length | ✓ | | | ✓ | ✓ (items) |
| minLength | ✓ | | | ✓ | ✓ (items) |
| maxLength | ✓ | | | ✓ | ✓ (items) |
| pattern | ✓ | ✓ | ✓ | ✓ | ✓ (whole) |
| enumeration | ✓ | ✓ | ✓ | ✓ | ✓ |
| whiteSpace | ✓ | ✓ | ✓ | ✓ | ✓ |
| minInclusive | | ✓ | ✓* | | |
| maxInclusive | | ✓ | ✓* | | |
| minExclusive | | ✓ | ✓* | | |
| maxExclusive | | ✓ | ✓* | | |
| totalDigits | | decimal | | | |
| fractionDigits | | decimal | | | |

*For `duration`: facets are applicable but indeterminate comparisons fail validation

## User-Derived Types

### By Restriction

Narrow base type with additional facets:

```xml
<xs:simpleType name="PositiveDecimal">
  <xs:restriction base="xs:decimal">
    <xs:minExclusive value="0"/>
  </xs:restriction>
</xs:simpleType>
```

### By List

Create whitespace-separated list of base type:

```xml
<xs:simpleType name="Coordinates">
  <xs:list itemType="xs:decimal"/>
</xs:simpleType>

<!-- Valid: "1.0 2.5 3.7" -->
```

### By Union

Allow values from multiple types:

```xml
<xs:simpleType name="SizeOrNumber">
  <xs:union>
    <xs:simpleType>
      <xs:restriction base="xs:string">
        <xs:enumeration value="small"/>
        <xs:enumeration value="medium"/>
        <xs:enumeration value="large"/>
      </xs:restriction>
    </xs:simpleType>
    <xs:simpleType>
      <xs:restriction base="xs:positiveInteger"/>
    </xs:simpleType>
  </xs:union>
</xs:simpleType>

<!-- Valid: "small" or "42" -->
```

Spec refs: docs/spec/xml/datatypes.xml#rf-defn, docs/spec/xml/datatypes.xml#dt-derived.

## Whitespace Normalization

Whitespace handling occurs before facet validation:

| Type | Default whiteSpace |
|------|-------------------|
| `string` | preserve |
| `normalizedString` | replace |
| `token` and most others | collapse |
| All numeric types | collapse |
| All date/time types | collapse |
| `boolean` | collapse |

**Processing Order:**

1. Apply whitespace normalization per type
2. Check pattern facets
3. Check other facets (length, bounds, enumeration)
4. Parse into value space

Spec refs: docs/spec/xml/datatypes.xml#dt-whiteSpace.

## Type Validation

### Atomic Type Validation

1. Normalize whitespace according to type
2. Check pattern facets (if any)
3. Parse lexical form to value (type-specific parsing)
4. Check enumeration (value or lexical comparison)
5. Check bounds (minInclusive, maxExclusive, etc.)
6. Check length/digit facets

### List Type Validation

1. Collapse whitespace (always)
2. Check list-level pattern facets (against entire string)
3. Split on whitespace into items
4. Validate each item against item type (including item type's pattern)
5. Check list-level facets (length, minLength, maxLength, enumeration)

### Union Type Validation

1. Check union-level pattern (if any)
2. Check union-level enumeration (if any)
3. Validate against member types in the order listed until one succeeds
4. Valid if any member type accepts the value
5. PSVI records the member type that validated the value

### QName Validation

QName values require namespace context:

1. Parse as `prefix:localpart` or `localpart`
2. Resolve prefix to namespace URI (from instance document) when present
3. Error if prefix is undeclared
4. If no prefix is present, use the in-scope default namespace when present; otherwise the namespace name is absent (no namespace)
5. PSVI value is (namespace URI, local name) pair

### NOTATION Validation

NOTATION values must match declared notations:

1. Parse as QName
2. Resolve the prefix to a namespace URI (if present)
3. Look up in schema's notation declarations
4. Error if no matching notation is declared
5. The governing type must be derived from `xs:NOTATION` by restriction with `enumeration`; using `xs:NOTATION` directly is a schema error

### Special Values

**Float/Double:**

- `INF` — positive infinity
- `-INF` — negative infinity
- `NaN` — not a number (special comparison semantics below)

**NaN Comparison Semantics:**

`NaN` has special behavior in XSD (different from IEEE 754):

- `NaN` equals itself in XSD (`NaN == NaN` is true for schema purposes)
- `NaN` is incomparable with all other values (neither less than nor greater than)
- For ordering facets (`minInclusive`, etc.), since no other values are comparable with `NaN`:
  - A bound facet with `NaN` produces either a value space containing only `NaN` (inclusive) or empty (exclusive)
  - Any other bound facet value excludes `NaN` from the restricted value space

Note: XSD explicitly differs from IEEE 754 here. Per XSD 1.0 Datatypes: "This datatype differs from IEEE 754 in that there is only one NaN... for schema purposes NaN = NaN."

**Boolean:**

- `true`, `1` — true value (case-sensitive: `True` and `TRUE` are invalid)
- `false`, `0` — false value (case-sensitive: `False` and `FALSE` are invalid)

Durations must include at least one component (years, months, days, hours, minutes, or seconds).

### ID/IDREF Semantics

These types have document-wide constraints beyond their lexical form:

| Type | Constraint |
|------|------------|
| `ID` | All values unique in document; at most one per element |
| `IDREF` | Must match some ID in document |
| `IDREFS` | Each item must match some ID |

These are checked after the entire document is processed.

Spec refs: docs/spec/xml/datatypes.xml#dt-memberTypes, docs/spec/xml/datatypes.xml#QName, docs/spec/xml/datatypes.xml#NOTATION, docs/spec/xml/datatypes.xml#duration, docs/spec/xml/datatypes.xml#ID, docs/spec/xml/datatypes.xml#IDREF, docs/spec/xml/datatypes.xml#IDREFS.
