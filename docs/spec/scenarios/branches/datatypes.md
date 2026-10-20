# Part 2 concrete branch audit

These are specification-derived descriptions, not executable fixtures or corpus
coverage claims. They instantiate the [Part 2 families](../datatypes.md) using
the [scenario method](../README.md) and the [corrected reading](../../corrected-reading.md).
The authority is the dated [Second Edition](../../upstream/datatypes.html);
its [Appendix A text](../../upstream/datatypes.xsd.txt) supplies the dated grammar.
No implementation or corpus was consulted for expected results.

## Shared construction and observation conventions

The following sketches are exact substitutions into this schema. Prefixes
`xs`, `p`, and `q` have the bindings shown. Unprefixed user type names below
refer to the no-namespace declarations. Replace `BODY`, `TYPE`, and `TEXT` with
the stated XML; do not emit those placeholder words.

```xml
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:p="urn:a" xmlns:q="urn:a">
  BODY
  <xs:element name="v" type="TYPE"/>
</xs:schema>
```

```xml
<v xmlns:p="urn:a" xmlns:q="urn:a">TEXT</v>
```

- `R(T; facets)` means `<xs:simpleType name="T"><xs:restriction
  base="BASE">facets</xs:restriction></xs:simpleType>` with BASE explicitly
  stated in its row. `F(name,value)` means the exact element
  `<xs:name value="value"/>`; `fixed=true` adds `fixed="true"`.
- `L(T,item)` means `<xs:simpleType name="T"><xs:list itemType="item"/>
  </xs:simpleType>`. `U(T,members)` means `<xs:simpleType name="T">
  <xs:union memberTypes="members"/></xs:simpleType>`.
- `""` is an empty character sequence, not absence of an element/attribute.
  `U+XXXX` denotes the exact single Unicode character, with concatenation
  stated explicitly. For example `a + U+0009 + b` means the three characters
  a, tab, b, written as `a&#x9;b` in element content. Backslashes in regex
  code spans are actual backslashes; they are not JSON string escaping.
- **S+ / S−** mean correct/incorrect schema construction. **V+ / V−** mean
  datatype-valid/invalid value under a correct schema. **E** means typed
  equality/order; **P** a component/PSVI fact; **K** canonical output when
  exposed. **X** is a failed XML/namespace precondition. **Q** marks an
  unresolved or explicitly permitted oracle, never a scored V− or S−.
- A row with several named inputs gives the expected outcome for each input.
  Local row labels such as `a/b/c` are subdivisions of the enclosing D2 ID.
  Invalid schemas have no claimed subsequent instance result. A comparison
  clause can be false even when a different constraint also rejects the same
  schema; such necessary overlap is stated rather than concealed.
- Canonicalization and component/PSVI observations are separate from validity.
  A validity-only interface cannot establish them. E witnesses can use
  enumeration/fixed/identity constraints where appropriate under Part 1;
  those observations do not add a canonical serializer requirement.

## Construction and lexical admission

### D2-001 — Names, builtin namespace, and XML character boundary

Source: [§§3.1, 3.2.1](../../upstream/datatypes.html#namespaces).

| Exact setup/input | Expected result and seam |
| --- | --- |
| TYPE=`xs:integer`, TEXT=`1`; then rename xs consistently to x | S+, V+; both identify the same built-in (P). |
| Bind xs=`urn:not-xsd`, retain TYPE=`xs:integer` with no such declaration | S− unresolved type; local name alone is insufficient. |
| TYPE=`xs:string`, TEXT=`""`, `a`, U+1D11E, U+0301 | V+ for each; U+1D11E counts as one character. |
| TYPE=`xs:string`, XML `<v>&#x0;</v>` or `<v>&#xD800;</v>` | X under XML 1.0, not a datatype-invalid witness. |
| TYPE=`xs:anySimpleType`, TEXT=`  x  ` | V+; anySimpleType is the special ur-type, not an ordinary union. |
| TYPE=`xs:dateTimeStamp` without a user declaration | S−; no such XSD 1.0 builtin. |

### D2-002 — Constructor sources and grammar

Source: [§4.1.2–4.1.3](../../upstream/datatypes.html#xr-defn),
[Appendix A](../../upstream/datatypes.html#schema).
Each fragment below is the entire child of `<xs:simpleType name="T">`.

| Fragment | Expected construction |
| --- | --- |
| `<xs:restriction base="xs:integer"/>` | S+; zero local facets allowed. |
| `<xs:restriction><xs:simpleType><xs:restriction base="xs:integer"/></xs:simpleType></xs:restriction>` | S+ inline-base alternative. |
| `<xs:restriction/>` | S− neither base source. |
| `<xs:restriction base="xs:integer"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:restriction>` | S− both base sources. |
| `<xs:list itemType="xs:integer"/>` | S+. |
| `<xs:list><xs:simpleType><xs:restriction base="xs:integer"/></xs:simpleType></xs:list>` | S+ inline-item alternative. |
| `<xs:list/>` | S− neither item source. |
| `<xs:list itemType="xs:integer"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:list>` | S− both item sources. |
| `<xs:union memberTypes="xs:integer"/>` | S+ one-member union, as required by the nonempty component sequence and representation constraint. |
| `<xs:union><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:union>` | S+ inline-only member. |
| `<xs:union memberTypes="xs:integer"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:union>` | S+ both sources; integer member precedes string. |
| `<xs:union/>`; `<xs:union memberTypes="  "/>` | S− each: no effective member. |
| `<xs:union memberTypes="  "><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:union>` | S+; empty attribute plus one child is sufficient. |
| `<xs:restriction base="p:missing"/>` with no urn:a type missing | S− unresolved component. |
| `<xs:restriction base="u:T"/>` with no binding u | S− unresolved QName prefix. |
| Empty simpleType; restriction followed by list | S− missing constructor; S− two constructors. |

Concrete envelope companions: top-level `<xs:simpleType>` without name is S−;
name=`a:b` is S− NCName; an inline simpleType with `name="local"` or
`final="restriction"` is S−. One leading annotation before the constructor is
S+; two annotations or one after the constructor is S−. These assess XML
representation, not the value space.

### D2-003 — final, finalDefault, and derivation dependency

Source: [§4.1.1–4.1.2](../../upstream/datatypes.html#dc-defn).

Define B as restriction of xs:integer and then construct T by each stated path.

| B/schema setting and T | Expected S/P |
| --- | --- |
| B final omitted; schema finalDefault omitted; T restriction B | S+; B final is empty. |
| B final=`restriction`; T restriction B | S− blocked restriction. |
| B final=`list`; `L(T,B)` | S− blocked list derivation. |
| B final=`union`; `U(T,"B xs:string")` | S− blocked union membership. |
| B final=`#all`; each of the previous three T paths | S− each path. |
| schema finalDefault=`restriction`; B explicit final=`""`; T restriction B | S+; explicit empty final overrides default. |
| schema finalDefault=`extension`; B final omitted; T restriction B | S+; legal default extension contributes no simple-type finality. |
| schema finalDefault=`substitution`, alone or beside extension | S−; substitution is outside the root finalDefault grammar, before any simple-type filtering. |
| B final=`restriction list`; T union `B xs:string` | S+ union, while restriction/list attempts remain S−. |
| B final=`#all restriction`, or `final="nonsense"` | S− lexical representation. |

A user type named xs:integer in the schema namespace is not a way to redefine
the mandatory builtin; same local name in `urn:a` is a different named component.
Duplicate named B definitions in one namespace are S−; distinct anonymous
inline types need not have different invented names.

### D2-004 — Lists, item alternatives, empty and sequence semantics

Source: [list construction](../../upstream/datatypes.html#derivation-by-list),
[list validity](../../upstream/datatypes.html#cvc-datatype-valid),
[Part 1 properties](../../upstream/structures.html#st-props-correct).

| Setup and concrete input | Expected result |
| --- | --- |
| `L(T,xs:integer)`; TEXT=`""`, `   `, `1`, `1 2`, `  1&#x9;2&#xA; ` | S+; V+ empty/empty/one/two/two items (P). |
| Same; `1 x 2` | V− middle item not integer. |
| Same; `1` + U+00A0 + `2` | V− NBSP does not delimit items. |
| `L(T,xs:string)`; `a b` | V+, two string items, not one. |
| `L(A,xs:integer)` then `L(T,A)` | S− item variety list. |
| `U(A,"xs:integer xs:string")`, `L(T,A)`; `1 a` | S+, V+ two atomic items of different member types. |
| `L(A,xs:integer)`, `U(B,"A xs:string")`, `L(T,B)` | S− union item type contains a list member. |
| T restriction of integer list with pattern `123 (\d+\s)*456`; `123 456`, `123 987 456`, `123 987` | V+, V+, V− respectively; pattern sees the complete list. |
| Decimal list T with enumeration `1 2`; `01.0 2.00`, `2 1`, `1 1 2` | V+, V−, V− respectively; E compares ordered item values and item count. |
| Same decimal list without enumeration; input `01.0 2.00` | K=`1.0 2.0` if canonical output is exposed. |

### D2-005 — Union order, nesting, member whitespace, and restrictions

Source: [union assembly](../../upstream/datatypes.html#derivation-by-union),
[selection](../../upstream/datatypes.html#union-datatypes),
[whitespace](../../upstream/datatypes.html#rf-whiteSpace).

| Setup/input | Expected V/P/E |
| --- | --- |
| `U(T,"xs:integer xs:string")`; `1`, `large` | V+ both; member integer then string respectively. |
| `U(T,"xs:string xs:integer")`; `1` | V+ member string; value differs from integer-selected `1`. |
| `U(A,"xs:integer xs:string")`; T members `A xs:decimal`; `1` | V+ first effective member integer after flattening. |
| T memberTypes=`xs:integer` plus inline restriction xs:string; `large` | V+ inline string member after attribute member fails. |
| `U(T,"xs:integer xs:boolean")`; `large` | V− no member. |
| `U(T,"xs:string xs:integer")`; `  1  ` | V+ selected string retains spaces. |
| `U(T,"xs:integer xs:string")`; `  1  ` | V+ selected integer uses collapse and value 1. |
| B=`U(B,"xs:integer xs:string")`; T restriction B with enumeration `1`; input `01`, `2`, `large` | V+, V−, V−; selected integer value 1 equals enumerated integer. |
| B=`U(B,"xs:integer xs:boolean")`; T restriction B with pattern `[0-9]+`; input `1`, `true` | V+, V−; lexical member validity alone is insufficient. |
| T restriction of a union with whiteSpace=`collapse` | S− inapplicable facet, even if every member itself collapses. |

The xsi:type override is a Part 1 seam: under the integer/string union,
`<v xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
xmlns:xs="http://www.w3.org/2001/XMLSchema" xsi:type="xs:string">1</v>`
selects string when allowed by governing derivation rules. A later union
restriction failing its facets contributes no successfully validated actual
member under E1-56; do not infer P solely from a partially matching member.

### D2-006 — Circular unions and dependency cycles

Source: [cos-no-circular-unions](../../upstream/datatypes.html#cos-no-circular-unions),
[Part 1 simple-type properties](../../upstream/structures.html#st-props-correct).

`U(T,"T xs:string")` is S− direct union cycle.
`U(A,"B xs:string")` plus `U(B,"A xs:integer")` is S− indirect cycle.
`U(A,"xs:integer xs:string")`, `U(T,"A xs:decimal")` is S+ acyclic nesting.
A restriction A base B plus restriction B base A is S− through Part 1's base
cycle rule; it is not evidence for the union-cycle branch. The seam is schema
construction; no instance is needed or assessed.

### D2-007 — Facet applicability, all twelve names

Source: [applicable facets table](../../upstream/datatypes.html#cos-applicable-facets).

Each row is the exact BASE/facet content of R(T). These distinguish all twelve
names without reusing a malformed facet value as an applicability witness.

| Facet | S+ restriction | S− restriction |
| --- | --- | --- |
| length | xs:string; F(length,2) | xs:integer; F(length,2) |
| minLength | xs:hexBinary; F(minLength,1) | xs:decimal; F(minLength,1) |
| maxLength | xs:anyURI; F(maxLength,3) | xs:date; F(maxLength,3) |
| pattern | xs:boolean; F(pattern,true) | A union has pattern too; fundamental facet `<xs:ordered value="total"/>` is S− unknown grammar, not a pattern prohibition. |
| enumeration | xs:decimal; F(enumeration,1) | xs:boolean; F(enumeration,true) |
| whiteSpace | xs:string; F(whiteSpace,collapse) | Union of xs:integer/xs:string; F(whiteSpace,collapse) |
| minInclusive | xs:integer; F(minInclusive,1) | xs:string; F(minInclusive,a) |
| minExclusive | xs:date; F(minExclusive,2000-01-01) | xs:base64Binary; F(minExclusive,TQ==) |
| maxInclusive | xs:duration; F(maxInclusive,P1D) | xs:QName; F(maxInclusive,p:a) |
| maxExclusive | xs:double; F(maxExclusive,10) | List of integer; F(maxExclusive,10) |
| totalDigits | xs:decimal; F(totalDigits,2) | xs:float; F(totalDigits,2) |
| fractionDigits | xs:decimal; F(fractionDigits,2) | xs:time; F(fractionDigits,2) |

Additional concrete variety contrasts: integer-list with length=2 is S+ and
`1 2` V+; union of integer/string with enumeration=1 is S+ and `01` V+;
that same union with length=1 is S−. QName with length=0 is S+ despite the
facet's deprecation and ineffectiveness (D2-024).

### D2-008 — Facet representation, singleton and fixed inheritance

Source: [single facet](../../upstream/datatypes.html#src-single-facet-value),
[§4.3](../../upstream/datatypes.html#rf-facets),
[Appendix A](../../upstream/datatypes.html#schema).

R(T,xs:string) with `<xs:length/>` is S− missing value;
length=`-1`, `1.0`, or `x` is S−, while length=`+02` is S+ value 2.
R(T,xs:decimal) totalDigits=`0` is S−; fractionDigits=`0` is S+.
`<xs:pattern value="["/>` is S− invalid regex, and
`<xs:enumeration value="x"/>` on xs:integer is S− base literal.
Two `<xs:length value="2"/>` children are S− even when equal;
two patterns or enumerations are allowed (D2-028/D2-029).

The following exact B→T sketches instantiate **all ten fixable facets**.
B is R(B,BASE) with the stated facet and `fixed="true"`.
T restricts B; each cell supplies its entire local facet.

| BASE / B facet | T equal: S+ | T changed: S− |
| --- | --- | --- |
| xs:string / length=2 | length=2 | length=1 |
| xs:string / minLength=1 | minLength=1 | minLength=2 |
| xs:string / maxLength=3 | maxLength=3 | maxLength=2 |
| xs:string / whiteSpace=replace | whiteSpace=replace | whiteSpace=collapse |
| xs:decimal / minInclusive=0 | minInclusive=0.0 | minInclusive=1 |
| xs:decimal / minExclusive=0 | minExclusive=0.0 | minExclusive=1 |
| xs:decimal / maxInclusive=10 | maxInclusive=10.0 | maxInclusive=9 |
| xs:decimal / maxExclusive=10 | maxExclusive=10.0 | maxExclusive=9 |
| xs:decimal / totalDigits=3 | totalDigits=3 | totalDigits=2 |
| xs:decimal / fractionDigits=2 | fractionDigits=2 | fractionDigits=1 |

For every row, T with no local facet is S+ and retains the inherited
restriction. The changed column is S− because a fixed facet cannot change.
Except for length, these changes tighten the ordinary facet constraint.
Changing length selects a disjoint value set and independently violates
[ordinary length restriction](../../upstream/datatypes.html#length-valid-restriction),
so that negative does not isolate fixedness.
Adding `fixed="false"` to the changed facet does not unlock it. An intermediate
zero-facet restriction C of B followed by the same changed T is also S−.
In the equal column, `fixed="1"` is a valid boolean alias; `fixed="True"` is
S−. For the remaining two facets, `<xs:pattern value="a" fixed="false"/>`
and `<xs:enumeration value="a" fixed="true"/>` on string are S− because
fixed is prohibited, independent of its value. One facet annotation is S+;
two or annotation after extra value text is representation-invalid.

### D2-009 — Whitespace three operations and both numbered restriction branches

Source: [whiteSpace](../../upstream/datatypes.html#rf-whiteSpace),
[restriction](../../upstream/datatypes.html#whiteSpace-valid-restriction).

Use TEXT=` a&#x9; b&#xA; `, the seven characters space,a,tab,space,b,LF,space.
String preserves it; normalizedString yields ` a  b  `; token yields `a b`.
Token TEXT=`&#x9;&#xA;  ` yields empty; token `a`+U+00A0+`b` retains NBSP.
All are V+ unless another facet excludes the resulting value.

| B mode → local T mode | Expected S |
| --- | --- |
| preserve → preserve / replace / collapse | S+ / S+ / S+ |
| replace → replace / collapse / preserve | S+ / S+ / S− (numbered branch 2) |
| collapse → collapse / replace / preserve | S+ / S− / S− (numbered branch 1 for both failures) |

Here B is xs:string restricted by the stated whiteSpace, without fixed.
For xs:integer or a list, changing inherited collapse to replace is S− fixed
as well as contrary to the lattice. For string B replace then T collapse with
length=3, input ` a  b ` is V+ after collapse (`a b`); length=4 yields V−.
There is no cvc-whiteSpace predicate: this is normalization followed by other
value/lexical checks.

### D2-010 — Concrete lexical witnesses for every builtin

Source: each linked [§3 builtin](../../upstream/datatypes.html#built-in-datatypes)
and the dated [Appendix A](../../upstream/datatypes.html#schema).
Use TYPE=`xs:TYPE` in each ordinary row. A comma-separated input list gives
separate exact strings, never a single comma-containing value. These are
illustrations of the complete family rules, not a claim to exhaust each type.

| Type/source | V+ literals and meaning | V− literal or explicitly different seam |
| --- | --- | --- |
| [string](../../upstream/datatypes.html#string) | `""`, ` a `, U+1D11E | XML character U+0000 is X, not V−; string has no datatype-level exclusion among XML character sequences. |
| [boolean](../../upstream/datatypes.html#boolean) | `true`, `false`, `1`, `0`, ` true ` | `True`, `+1`, `00`, `""` |
| [decimal](../../upstream/datatypes.html#decimal) | `.5`, `5.`, `+001.2300`, `-0` | `.`, `1e0`, `INF`, U+0661 |
| [float](../../upstream/datatypes.html#float) | `1e+02`, `.5`, `INF`, `-INF`, `NaN`, `-0` | `+INF`, `nan`, `1E`, `0x1p0` |
| [double](../../upstream/datatypes.html#double) | `1E2`, `5.`, `NaN`, `+0` | `Infinity`, `+INF`, `1e+`, U+0661 |
| [duration](../../upstream/datatypes.html#duration) | `P0D`, `PT0S`, `-P1M`, `P1Y2M3DT4H5M6.7S`, `PT90M` | `P`, `PT`, `P1DT`, `+P1D`, `P-1D`, `PT.5S`, `PT1.S`, `P1W` |
| [dateTime](../../upstream/datatypes.html#dateTime) | `2000-02-29T23:59:59.123Z`, `2000-12-31T24:00:00Z`, `-0001-01-01T00:00:00` | `1900-02-29T00:00:00`, `0000-01-01T00:00:00`, `2000-01-01t00:00:00`, `2000-01-01T24:00:00.1` |
| [time](../../upstream/datatypes.html#time) | `13:20:00-05:00`, `24:00:00`, `00:00:00.000Z` | `13:20`, `25:00:00`, `00:60:00`, `24:00:01` |
| [date](../../upstream/datatypes.html#date) | `2000-02-29`, `10000-01-01+14:00`, `-0001-01-01` | `1900-02-29`, `0000-01-01`, `2000-1-01`, `2000-01-01T00:00:00` |
| [gYearMonth](../../upstream/datatypes.html#gYearMonth) | `1999-05`, `-0001-12Z`, `10000-01` | `1999-00`, `1999-13`, `0000-01`, `99-05` |
| [gYear](../../upstream/datatypes.html#gYear) | `1999`, `-0001`, `10000Z` | `0000`, `+1999`, `01999`, `999` |
| [gMonthDay](../../upstream/datatypes.html#gMonthDay) | `--02-29`, `--12-31Z` | `--02-30`, `--04-31`, `-02-29` |
| [gDay](../../upstream/datatypes.html#gDay) | `---01`, `---31+01:00` | `---00`, `---32`, `--31` |
| [gMonth](../../upstream/datatypes.html#gMonth) | `--01`, `--12Z` | `--00`, `--13`, `--01--` |
| [hexBinary](../../upstream/datatypes.html#hexBinary) | `""`, `0aFF`, ` 0AFF ` | `A`, `0xAA`, `0A FF`, `GG` |
| [base64Binary](../../upstream/datatypes.html#base64Binary) | `""`, `TWFu`, `TWE=`, `TQ==`, ` T Q = = ` | `TQ`, `TQ=`, `TR==`, `TWF=`, `=TQ=` |
| [anyURI](../../upstream/datatypes.html#anyURI) | `""`, `../a`, `#f`, `a b`, `http://[::1]/x` after prescribed escaping | `%ZZ` fails RFC2396 escape syntax; see D2-045 for the referenced algorithm boundary. |
| [QName](../../upstream/datatypes.html#QName) | `p:a`, `q:a`, `a` in shared namespace context | `u:a` unbound u, `p:a:b`, `:a`, `p:1` |
| [NOTATION](../../upstream/datatypes.html#NOTATION) | Under D2-011's derived N: `n` | Direct TYPE=xs:NOTATION is S−; N value `missing` is V−. |
| [normalizedString](../../upstream/datatypes.html#normalizedString) | `a&#x9;b` normalizes to `a b`; ` a  b ` preserved apart from replace | No XML-character-only literal is rejected solely by this builtin; normalization, not rejection, realizes the restriction. |
| [token](../../upstream/datatypes.html#token) | ` a  b ` normalizes to `a b`; whitespace-only normalizes empty | No datatype-only lexical failure among XML character strings; add a facet to obtain V−. |
| [language](../../upstream/datatypes.html#language) | `en`, `EN`, `en-US`, `x-private` | `1en`, `en_US`, `en-`, `abcdefghi`, `en--US` |
| [NMTOKEN](../../upstream/datatypes.html#NMTOKEN) | `1`, `a:b`, `-x`, `a`+U+0301 | `""`, `a b` |
| [NMTOKENS](../../upstream/datatypes.html#NMTOKENS) | `1 a:b`, ` a&#x9;b ` | `""`, all-whitespace; inherited minLength=1 |
| [Name](../../upstream/datatypes.html#Name) | `a:b`, `_x`, `:x` | `1x`, `-x`, `""` |
| [NCName](../../upstream/datatypes.html#NCName) | `a`, `_x`, `a`+U+0301 | `a:b`, `:x`, `1x`, initial U+0301 |
| [ID](../../upstream/datatypes.html#ID) | `a`, `_x` | `a:b`, `1x`; duplicate document IDs are a separate Part 1 failure. |
| [IDREF](../../upstream/datatypes.html#IDREF) | `a` lexical/typed value | `a:b`; unresolved document reference is separately assessed under Part 1. |
| [IDREFS](../../upstream/datatypes.html#IDREFS) | `a b` lexical two-item list | `""`, `a p:b` |
| [ENTITY](../../upstream/datatypes.html#ENTITY) | `img` with D2-011 unparsed-entity declaration | `missing`, `parsed` under D2-011 context |
| [ENTITIES](../../upstream/datatypes.html#ENTITIES) | `img img` with same declaration | `""`, `img missing` |
| [integer](../../upstream/datatypes.html#integer) | `+001`, `-0`, `123456789012345678` | `1.0`, `1.`, `1e0` |
| [nonPositiveInteger](../../upstream/datatypes.html#nonPositiveInteger) | `-1`, `0`, `+0` | `1` |
| [negativeInteger](../../upstream/datatypes.html#negativeInteger) | `-2`, `-1` | `-0`, `0`, `1` |
| [long](../../upstream/datatypes.html#long) | `-9223372036854775808`, `9223372036854775807` | `-9223372036854775809`, `9223372036854775808` |
| [int](../../upstream/datatypes.html#int) | `-2147483648`, `2147483647` | `-2147483649`, `2147483648` |
| [short](../../upstream/datatypes.html#short) | `-32768`, `32767` | `-32769`, `32768` |
| [byte](../../upstream/datatypes.html#byte) | `-128`, `127` | `-129`, `128` |
| [nonNegativeInteger](../../upstream/datatypes.html#nonNegativeInteger) | `-0`, `0`, `1` | `-1` |
| [unsignedLong](../../upstream/datatypes.html#unsignedLong) | `0`, `18446744073709551615` | `-1`, `18446744073709551616` |
| [unsignedInt](../../upstream/datatypes.html#unsignedInt) | `0`, `4294967295` | `-1`, `4294967296` |
| [unsignedShort](../../upstream/datatypes.html#unsignedShort) | `0`, `65535` | `-1`, `65536` |
| [unsignedByte](../../upstream/datatypes.html#unsignedByte) | `-0` under Appendix A's complete-definition reading qualified in D-072; `255` | `-1`, `256` |
| [positiveInteger](../../upstream/datatypes.html#positiveInteger) | `1`, `+001` | `-1`, `-0`, `0` |

All integer-derived rows additionally reject `1.0` and accept their in-range
endpoint with redundant leading zeros. This is a shared lexical case, not a
host-width rule for unbounded integer. NMTOKEN and Name each derive from token;
NCName derives from Name. Name's apparent lexical subset relation does not
create an NMTOKEN→Name derivation edge.

### D2-011 — NOTATION, ENTITY, and document-dependent identity setup

Source: [NOTATION](../../upstream/datatypes.html#enumeration-required-notation),
[ENTITY](../../upstream/datatypes.html#ENTITY),
[ID/IDREF](../../upstream/datatypes.html#ID).

Use these exact BODY definitions for N and TYPE=N:

```xml
<xs:notation name="n" public="urn:public:n"/>
<xs:simpleType name="N"><xs:restriction base="xs:NOTATION">
  <xs:enumeration value="n"/>
</xs:restriction></xs:simpleType>
```

TEXT=`n` is V+; `missing` is V−. Replacing enumeration `n` by `missing`
is S− undeclared notation value. Removing enumeration and using bare
NOTATION as the governing type is S−. T restriction N without new enumeration
retains the inherited enumeration; `n` remains V+. The compatibility advice
to use NOTATION on attributes with no target namespace does not create a
mandatory element-value rejection.

For ENTITY use TYPE=xs:ENTITY and the following exact document prolog:

```xml
<!DOCTYPE v [
  <!NOTATION png SYSTEM "image/png">
  <!ENTITY img SYSTEM "img.png" NDATA png>
  <!ENTITY parsed "text">
]>
<v>img</v>
```

`img` is V+; replacing content by `missing` or `parsed` is V− under the
ENTITY value-space requirement. The external resource is not fetched to
prove the unparsed declaration. ENTITIES `img img` is V+; `img missing`
is V−. Removing the DTD removes the declaration precondition. Different
processor DTD-access policies must be recorded; an NCName-only admission result
cannot establish ENTITY conformance.

For document identity under Part 1 use a no-namespace root permitting an
unbounded repeated choice of `<i id="..."/>` and `<r ref="..."/>`; each i
requires an id attribute typed xs:ID and each r a ref attribute typed xs:IDREF.
`<i id="a"/><r ref="a"/>` succeeds; two i elements with id=a fail document
uniqueness; only `<r ref="a"/>` fails reference resolution at document end;
`<r ref="a"/><i id="a"/>` is a valid forward-reference ordering. IDs are
NCNames and remain document-scoped, not namespace-expanded QName tuples.

### D2-012 — Canonical and normalized forms are different observations

Source: [canonical representation](../../upstream/datatypes.html#canonical-lexical-representation),
[date/time/binary sections](../../upstream/datatypes.html#built-in-primitive-datatypes).
All inputs below are V+; K describes canonical output, with the zero-sign
question explicitly retained where the local canonical clause is incomplete.

| Type/input | K expected |
| --- | --- |
| boolean `1`, `0` | `true`, `false` |
| decimal `+001.2300`, `5` | `1.23`, `5.0` |
| decimal `-0`; integer `-0` | Conventional `0.0` / `0`; unique K remains source-qualified by [D-072](../datatypes.md#d-072--questions-and-permissible-choices-that-must-remain-visible), not a second permitted output policy |
| integer `+001` | `1` |
| nonPositiveInteger `-0` | `0`; its canonical clause explicitly omits zero's sign |
| float/double `+00100.00e+00`, `-0` | `1.0E2`, `0.0E0` |
| float/double `INF`, `-INF`, `NaN` | unchanged special spellings |
| hexBinary `0aff` | `0AFF` |
| base64Binary ` T Q = = ` | `TQ==` |
| dateTime `2000-12-31T24:00:00.000Z` | `2001-01-01T00:00:00Z` |
| dateTime `2002-10-10T12:00:00.1200-05:00` | `2002-10-10T17:00:00.12Z` |
| time `24:00:00` | `00:00:00` |
| time `13:20:00-05:00` | `18:20:00Z` |
| date `2002-10-10+13:00` | `2002-10-09-11:00` |
| decimal list `01.0 2.00` | `1.0 2.0` |

A union selecting string for `+001` has string value `+001`; one selecting
integer has canonical form `1`. QName has no universal canonical prefix chosen
by Part 2. Part 2 supplies no standalone canonical-duration algorithm.
E1-56's defaulted element PSVI uses the constraint's lexical form, so a default
`+001` need not become canonical `1`; K must not be inferred from that P seam.

### D2-013 — Typed equality versus lexical aliases and primitive disjointness

Source: [equal](../../upstream/datatypes.html#equal).
The pair notation below compares **typed values**, not raw character strings.

| Left / right | E expected |
| --- | --- |
| decimal `2.0` / decimal `02.00` | equal |
| integer `1` / decimal `1.0` | equal, common decimal primitive |
| decimal `1` / float `1` | unequal, disjoint primitives |
| float `1` / double `1` | unequal, disjoint primitives |
| boolean `true` / boolean `1` | equal |
| boolean `true` / integer `1` | unequal |
| string `en` / string `EN` | unequal; no case folding |
| string U+00E9 / string `e` + U+0301 | unequal; no Unicode normalization |
| QName `p:a` / QName `q:a`, shared bindings | equal |
| QName `p:a` under urn:a / QName `p:a` rebound urn:b | unequal |
| hexBinary `0a` / hexBinary `0A` | equal |
| hexBinary `4D` / base64Binary `TQ==` | unequal primitive spaces, despite identical byte payload |
| duration `P1Y` / `P12M`; `P1D` / `PT24H`; `PT1M` / `PT60S` | equal for each pair under specified temporal arithmetic |
| dateTime `2002-10-10T12:00:00-05:00` / `2002-10-10T17:00:00Z` | equal |
| dateTime `2000-01-01T00:00:00` / same spelling with `Z` | unequal, different timelines |

Enumeration witness: R(T,xs:decimal; enumeration=2.0) admits `02.00` and rejects
`2.01`. If T also has pattern `2\.0`, `02.00` fails the lexical pattern even
though E equals the enumerated value. Restrictions preserve atomic identity;
union selection can change which primitive value is denoted.

### D2-014 — Floating-point rounding, NaN, and single zero

Source: [float](../../upstream/datatypes.html#float),
[double](../../upstream/datatypes.html#double).

| Exact typed input/operation | Expected result |
| --- | --- |
| float `16777216`, `16777217`, `16777218` | V+; middle maps to 16777216 by ties-to-even, endpoints to themselves. |
| float `1.000000059604644775390625` | Exactly halfway between 1 and 1+2^-23; maps to 1. |
| float `1.000000059604644775390626` | Maps to 1+2^-23; decimal input is above the midpoint. |
| double `9007199254740992`, `9007199254740993`, `9007199254740994` | V+; middle maps to 9007199254740992 by ties-to-even. |
| float `0`, `-0`, `+0` | One identical value, not signed-zero-distinct. |
| float NaN compared to float NaN | Equal by schema identity. |
| float NaN versus 1, INF, or -INF | Unequal and incomparable. |
| R(T,xs:float; enumeration=NaN); `NaN`, `1` | V+, V−. |
| R(T,xs:double; minInclusive=NaN); `NaN`, `1` | V+, V−. |
| R(T,xs:double; minExclusive=NaN); `NaN`, `1` | V−, V−; exclusive NaN bound leaves no value. |
| R(T,xs:float; maxInclusive=INF); `INF`, `1`, `NaN` | V+, V+, V−. |
| R(T,xs:float; maxExclusive=INF); `INF`, `1`, `NaN` | V−, V+, V−. |

The symmetric maxInclusive=NaN singleton and maxExclusive=NaN empty-space
cases are equally determined by the explicit source paragraph. The double
smallest-exponent wording conflicts with its IEEE description; extremal
subnormal/underflow numeric expectations remain Q in D2-048, rather than being
silently supplied by a host float parser. Ordinary midpoint cases above do
not depend on that discrepancy.

## Calendar, duration, and fundamental properties

### D2-015 — Duration order reference points

Source: [duration order](../../upstream/datatypes.html#duration-order),
[facet comparison](../../upstream/datatypes.html#facet-comparison-for-durations).
Use the four reference dates verbatim: 1696-09-01, 1697-02-01, 1903-03-01,
1903-07-01, each at `T00:00:00Z`.

| Left / right duration | E expected |
| --- | --- |
| P1M / P27D | greater at all four points |
| P1M / P28D | incomparable (some equality, some greater) |
| P1M / P30D | incomparable (different signs at reference points) |
| P1M / P31D | incomparable (some equality, some less) |
| P1M / P32D | less at all four points |
| P1Y / P364D | greater |
| P1Y / P365D; P1Y / P366D | incomparable for both |
| P1Y / P367D | less |
| P12M / P1Y | equal |
| -P1M / -P32D | greater, reversing the positive less-than result |

R(T,xs:duration; minInclusive=P1M) admits `P32D` and `P1M` but rejects
`P30D` and `P27D`. R(T,xs:duration; maxExclusive=P1M) admits `P27D` but
rejects `P1M`, `P30D`, and `P32D`. These make the indeterminate comparison
observable without exposing arithmetic. Month/day mixing is not evaluated by
a fixed 30-day conversion.

### D2-016 — Gregorian boundaries and expanded years

Source: [dateTime lexical form](../../upstream/datatypes.html#dateTime-lexical-representation),
[Appendix D](../../upstream/datatypes.html#formatdetails).
Use TYPE=xs:date unless the full dateTime is written.

| Exact literals | Outcome |
| --- | --- |
| `2000-02-29`, `2004-02-29`, `1600-02-29`, `1200-02-29` | V+ Gregorian leap years, including before 1582. |
| `1900-02-29`, `2001-02-29`, `1500-02-29` | V− nonleap year/century. |
| `2000-04-30`, `2000-04-31`, `2000-01-31`, `2000-01-32` | V+, V−, V+, V−. |
| `2000-00-01`, `2000-13-01`, `2000-01-00` | V− each. |
| `0001-01-01`, `-0001-01-01`, `10000-01-01`, `-10000-01-01` | V+ where expanded precision is supported; four-digit forms are required support. |
| `0000-01-01`, `-0000-01-01`, `+2000-01-01`, `02000-01-01` | V− each, independent of implementation precision. |
| dateTime `2000-01-01T24:00:00.000` | V+, same value as `2000-01-02T00:00:00`. |
| dateTime `2000-01-01T24:00:00.001`, `2000-01-01T23:60:00` | V− each. |

A supported expanded year is never silently narrowed. Exact behavior above a
clearly documented permitted precision maximum is an operational/conformance
choice, not a claim that the lexical grammar forbids the spelling.

### D2-017 — Every timezone grammar boundary and UTC carry

Source: [timezones](../../upstream/datatypes.html#dateTime-timezones).
Append each suffix to `2000-01-01T00:00:00`.

| Suffix | Outcome |
| --- | --- |
| absent, `Z`, `+00:00`, `-00:00`, `+13:59`, `-13:59`, `+14:00`, `-14:00` | V+ each; only the three explicit zero zones are identical zones. |
| `+14:01`, `-14:01`, `+15:00`, `+00:60`, `+1:00`, `+0100`, `z`, `UTC`, ` Z`, `ZZ` | V− each. |

E: `2000-01-01T00:00:00+01:00` equals
`1999-12-31T23:00:00Z`; `2000-02-29T23:30:00-01:00` equals
`2000-03-01T00:30:00Z`. The absent suffix is not equivalent to `Z` and
cannot be supplied from the machine's timezone. All eight temporal primitive
types permit an optional suffix; duration does not (`P1DZ` is V−).

### D2-018 — Mixed-timezone order, exact fourteen-hour edge

Source: [dateTime order](../../upstream/datatypes.html#dateTime-order).
Let A=`2000-01-02T00:00:00` (no zone). Compare the following B to A:

| B | E expected |
| --- | --- |
| `2000-01-01T09:59:59Z` | B < A, separation greater than 14h |
| `2000-01-01T10:00:00Z` | incomparable, exact 14h edge |
| `2000-01-01T10:00:00.001Z` | incomparable, inside edge |
| `2000-01-02T00:00:00Z` | incomparable, same local clock fields |
| `2000-01-02T14:00:00Z` | incomparable, exact other edge |
| `2000-01-02T14:00:00.001Z` | B > A |

Reversing the operands reverses determined comparisons and preserves
indeterminacy. The opening two-timeline definition explicitly says differences
of **14 hours or less** are incomparable; this settles equality at the edges
despite the algorithm's later explanatory strict inequalities. Bounds with
A as endpoint reject all incomparable candidates. For two absent-zone values,
`2000-01-01T00:00:00 < 2000-01-02T00:00:00`; for two zoned values,
`2000-01-01T01:00:00+01:00 = 2000-01-01T00:00:00Z`.

### D2-019 — Date intervals, time recurrence, and g* reference contexts

Source: [date](../../upstream/datatypes.html#date),
[time](../../upstream/datatypes.html#time),
[gMonthDay](../../upstream/datatypes.html#gMonthDay).

| Typed pair or literal | Expected result |
| --- | --- |
| date `2002-10-10+13:00` / `2002-10-09-11:00` | E equal one-day intervals; K latter spelling. |
| date `2002-10-10+12:00` | K same date and +12:00, within recoverable-zone range. |
| date `2002-10-10-12:00` | K `2002-10-11+12:00`, equal interval, not UTC-midnight truncation. |
| time `24:00:00` / `00:00:00` | E equal recurring midnight. |
| time `23:30:00-01:00` / `00:30:00Z` | UTC normalization wraps time-of-day; recurring time equality must follow the prescribed arbitrary-date semantics; the handling of day wrap in cross-date identity is flagged Q in D2-048 rather than inferred from a clock-only implementation. |
| gMonthDay `--02-29`, `--02-30` | V+, V− using an arbitrary leap year. |
| gDay `---31`, `---32` | V+, V− using a reference month with 31 days. |
| gMonth `--01`, `--01--` | V+, V− under the dated Second Edition grammar. |
| gYearMonth `1999-12` / `2000-01`; gYear `1999` / `2000` | E less for each unzoned pair. |
| gMonth `--01` / `--02`; gDay `---01` / `---02` | E less within the prescribed common reference context. |

The precise recurring-calendar timezone/canonical questions cannot be settled
by selecting the current year or dropping absent fields. Their unresolved
cases remain separate from the unambiguous lexical witnesses above.

### D2-020 — Appendix E primitives, carries, pinning, missing fields

Source: [Appendix E](../../upstream/datatypes.html#adding-durations-to-dateTimes).
These are E arithmetic observations, directly or through duration comparison;
no public arithmetic function is required.

| Exact operation | Required result |
| --- | --- |
| fQuotient(-1,3), modulo(-1,3) | -1, 2 (floor, not truncation toward zero) |
| fQuotient(0,3), modulo(3,3) | 0, 0 |
| modulo(0,1,13), fQuotient(0,1,13) | 12, -1 |
| maximumDayInMonthFor(2000,2), (1900,2), (2000,13) | 29, 28, 31 |
| `2000-01-31 + P1M` | `2000-02-29`, day pinned after month addition |
| `2001-01-31 + P1M` | `2001-02-28` |
| `2000-03-01T00:00:00Z + (-PT1S)` | `2000-02-29T23:59:59Z` |
| `2000-12-31T23:59:59.5Z + PT0.75S` | `2001-01-01T00:00:00.25Z` |
| `2000-01-12T12:13:14Z + P1Y3M5DT7H10M3.3S` | `2001-04-17T19:23:17.3Z` |
| `2000-01 + (-P3M)` | `1999-10`, missing day/time remain absent |
| `2000-01-12 + PT33H` | `2000-01-13`, missing time fields removed after calculation |
| `2000-01-31 + P62D` | `2000-04-02`, multiple month loops |
| `(2000-03-30 + P1D) + P1M` | `2000-04-30` |
| `(2000-03-30 + P1M) + P1D` | `2000-05-01`, so sequential operations are order-sensitive |
| E.2 fieldwise sum of P11M and P2M, other fields zero | month=13, year=0; no field overflow |
| E.2 fieldwise sum of PT59M and PT2M, other fields zero | minute=61, hour=0; no field overflow |

The fieldwise duration sums are separate from instant carry/pinning; they
do not prescribe a unique serialized spelling. [D-029](../datatypes.md#d-029--duration-equality-and-partial-order)
owns the optional duration-order optimization table and its listed boundaries;
its four-anchor definition remains the result oracle.

E copies `S[zone]` to the result. That states what happens to the chosen
algorithm input fields; it does not establish that appending +02:00 to these
lexical inputs and postponing dateTime's UTC value conversion preserves
calendar pinning. State the literal-to-S coordinate mapping and justify any
claimed equivalence before using such a nonzero-offset lexical calculation
as an oracle. These unzoned/Z examples assert no separate nonzero-offset
lexical-coordinate policy. This is a qualification of the described interface,
not a new normative ambiguity or normalization rule. BCE/CE crossing and
second-60 input admission remain D2-048 questions; it would be false precision
to calculate them by an unrecorded astronomical-year conversion.

### D2-021 — Fundamental facet branches, with concrete component sketches

Source: [§4.2](../../upstream/datatypes.html#rf-fund-facets),
[Appendix C](../../upstream/datatypes.html#app-fundamental-facets).
These are **P**, not additional validity predicates.

| Exact component setup | Required P by the published rule |
| --- | --- |
| R(T,xs:integer; minInclusive=1) | ordered=total, numeric=true, bounded=false, cardinality=countably infinite. |
| R(T,xs:integer; minInclusive=1,maxInclusive=2) | bounded=true, cardinality=finite (inherited fractionDigits=0). |
| R(T,xs:decimal; minInclusive=1,maxInclusive=2) | bounded=true, cardinality=countably infinite without fractionDigits. |
| Same plus fractionDigits=2 | cardinality=finite. |
| R(T,xs:string; maxLength=2) | cardinality=finite; ordered=false, numeric=false. |
| R(T,xs:date; minInclusive=2000-01-01,maxInclusive=2000-01-02) | cardinality=finite. |
| R(T,xs:dateTime; minInclusive=2000-01-01T00:00:00,maxInclusive=2000-01-02T00:00:00) | cardinality=countably infinite absent a scale restriction. |
| L(T,xs:integer), no facets | ordered=false, numeric=false, bounded=false, cardinality=countably infinite. |
| Restrict that list by length=2 | ordered=false, numeric=false, bounded=true; published cardinality formula says finite even with infinite item values (source tension, not inferred mathematics). |
| U(T,"xs:byte xs:short") | ordered=total, numeric=true, bounded=true, cardinality=finite. |
| U(T,"xs:integer xs:string") | ordered=partial, numeric=false, bounded=false, cardinality=countably infinite. |
| U(T,"xs:string xs:boolean") | ordered=false because every member unordered. |
| U(T,"xs:byte xs:float") | ordered=partial; numeric=true; cardinality=finite. FM13 preserves the bounded common-ancestor wording: ordered explicitly excludes anySimpleType, bounded does not. Do not add an unprinted exclusion or a union-of-numeric-ranges shortcut. |
| xs:float and xs:double | ordered=partial, numeric=true, bounded=true, cardinality=finite. |

A pattern restricting a partial-order temporal type to zoned values does not
change the inherited `ordered` component to total merely because the admitted
subset is semantically totally ordered. Equal has no corresponding schema
component; the value-space equality laws remain D2-013.

### D2-022 — Conformance, precision minima, and recommendations

Source: [§5](../../upstream/datatypes.html#conformance),
[decimal minimum](../../upstream/datatypes.html#decimal),
[year/seconds minimum](../../upstream/datatypes.html#year-sec-conformance).

Decimal `123456789012345678` must be supported (18 digits).
`1234567890123456789` is a valid lexical/value-space case whose support can
be limited by a clearly documented permitted maximum. dateTime
`9999-12-31T23:59:59.123` exercises the required four year digits and three
fractional digits; `10000-01-01T00:00:00.1234` exceeds both minima without
becoming invalid syntax. Silently dropping `.0004` from an accepted exact
value is not authorized by documenting a precision maximum.

A non-XML schema API can satisfy minimal conformance through component and
validation rules without claiming XML-representation conformance. An API
claiming XML representation conformance must reject both/neither constructor
sources in D2-002 as well. Attribute-only advice for ID/IDREF/NMTOKEN/ENTITY,
discouraged spaces in anyURI, and deprecated QName length facets are not
mandatory value rejection rules. NOTATION's required enumeration is mandatory.

## Named constraints: concrete facet branches

### D2-023 — Facet-valid and datatype-valid dispatch

Source: [cvc-facet-valid](../../upstream/datatypes.html#cvc-facet-valid),
[cvc-datatype-valid](../../upstream/datatypes.html#cvc-datatype-valid).

| Clause shape and exact setup | Positive / negative witness |
| --- | --- |
| Lexical parent, pattern branch: R(T,xs:integer; pattern=`[0-9]+`) | `12` V+; `+12` V− pattern; `12.0` V− integer lexical space even though a different broad pattern could accept it. |
| No pattern, atomic: zero-facet R(T,xs:integer) | `+12` V+; `12.0` V− base lexical space. |
| No pattern, list: L(T,xs:integer) | `1 2` V+; `1 x` V− item; `""` V+ empty list. |
| No pattern, union: U(T,"xs:integer xs:boolean") | `12` V+ integer; `true` V+ boolean; `large` V− neither. |
| Value/facet stage: R(T,xs:integer; maxInclusive=10) | `10` V+; `11` V− after valid integer lexical admission. |
| Facet-valid dispatcher for length: R(T,xs:string; length=2) | `ab` V+; `a` V−. |

The numbered lexical parent and its no-pattern parent are composites, not
extra independent acceptance routes. D2-024–D2-034 instantiate the particular
facet predicates. The generic cvc-facet-valid clause delegates to those
predicates; no additional invented facet rule is needed.

### D2-024 — All length/min/max measurement alternatives

Source: [length](../../upstream/datatypes.html#cvc-length-valid),
[minLength](../../upstream/datatypes.html#cvc-minLength-valid),
[maxLength](../../upstream/datatypes.html#cvc-maxLength-valid).
For each row construct three separate R(T,BASE): length=2, minLength=2,
maxLength=2. Outcomes are triples in that order.

| BASE and exact candidate | length / minLength / maxLength outcome |
| --- | --- |
| xs:string `a`; `ab`; `abc` | (V−,V−,V+); (V+,V+,V+); (V−,V+,V−) |
| xs:string U+1D11E; U+1D11E followed by `a` | one character: (V−,V−,V+); two characters: all V+ |
| xs:anyURI `a`; `ab`; `abc` | same three triples as string; count datatype characters, not fetched resource size |
| xs:hexBinary `AA`; `AABB`; `AABBCC` | one/two/three octets: same three triples |
| xs:base64Binary `TQ==`; `TWE=`; `TWFu` | one/two/three decoded octets: same three triples |
| xs:QName `p:a`; `p:longname` | all V+ for both; each admitted length facet value is ineffective |
| N from D2-011, value `n` | all V+ for all three facets; NOTATION has the same ineffective measurement branch |
| Integer list `1`; `1 2`; `1 2 3` | one/two/three items: same triples |

For QName and N also construct length=0, minLength=100, maxLength=0:
`p:a`/`n` remain V+ for each. Ordinary string/hex/list length=0 admits empty
and rejects one item/character/octet; minLength=0 admits both; maxLength=0
rejects nonempty. The atomic parent comprises string/URI, binary, and
QName/NOTATION alternatives; list is its other variety branch.

### D2-025 — length with minLength: numeric and provenance conjuncts

Source: [length-minLength-maxLength clauses 1, 1.1, 1.2](../../upstream/datatypes.html#length-minLength-maxLength).

| Exact B→T construction | Expected S; distinguishing premise |
| --- | --- |
| B=R(B,xs:string; minLength=1); T=R(T,B; length=2) | S+; inherited 1≤2, ancestor supplied same min without length. `ab` V+, `a` V−. |
| Same B; T length=0 | S− numeric conjunct 1≤0 false; ancestry still true. |
| T=R(T,xs:string; length=2,minLength=1) | S− ancestry conjunct: no ancestor with that minLength. Numeric 1≤2 alone is insufficient. |
| B minLength=1; T length=2,minLength=2 | S− required same minLength value in a qualifying ancestor absent; numerical consistency alone is insufficient. |
| B minLength=1; C=R(C,B; length=2); T=R(T,C) with no facets | S+ inherited combination; no extra local source needed. |
| B length=2; T length=3 | S− inherited length equality rule. |
| B length=2; T length=2 | S+; unchanged length. |

The first composite clause is reviewed through its two conjunct witnesses;
there is no independent “length and minLength always forbidden” oracle.

### D2-026 — length with maxLength: numeric and provenance conjuncts

Source: [length-minLength-maxLength clauses 2, 2.1, 2.2](../../upstream/datatypes.html#length-minLength-maxLength).

| Exact B→T construction | Expected S; distinguishing premise |
| --- | --- |
| B=R(B,xs:string; maxLength=3); T=R(T,B; length=2) | S+; inherited 2≤3 and qualifying ancestor. `ab` V+, `abc` V−. |
| Same B; T length=4 | S− numeric conjunct false; qualifying ancestry true. |
| T=R(T,xs:string; length=2,maxLength=3) | S− no qualifying ancestor, despite 2≤3. |
| B maxLength=3; T length=2,maxLength=2 | S− no qualifying ancestor with maxLength=2. |
| B maxLength=3; C=R(C,B; length=2); T=R(T,C) | S+ unchanged inherited combination. |

### D2-027 — Length-bound restriction monotonicity

Source: [minLength restrictions](../../upstream/datatypes.html#minLength-coss),
[maxLength restrictions](../../upstream/datatypes.html#maxLength-coss).

| Base and local facet | Expected construction |
| --- | --- |
| B minLength=2; T minLength=1 / 2 / 3 | S− / S+ / S+ |
| B maxLength=2; T maxLength=1 / 2 / 3 | S+ / S+ / S− |
| xs:string; local minLength=1,maxLength=2 | S+; `a`,`ab` V+ and empty/`abc` V− |
| xs:string; local minLength=2,maxLength=2 | S+ exact length interval |
| xs:string; local minLength=3,maxLength=2 | S− min exceeds max |

Fixed companions in D2-008 reject the otherwise legal tightening cases.
Reusing these with list/binary measures changes the unit, not the schema
inequality. Values 1/2/3 here are exact nonnegative integer facet values.

### D2-028 — Pattern OR within a step, AND across steps

Source: [multiple patterns](../../upstream/datatypes.html#src-multiple-patterns),
[pattern-valid](../../upstream/datatypes.html#cvc-pattern-valid).

B=R(B,xs:string; pattern=a,pattern=b) is S+; `a` and `b` V+, `ab` V−.
T=R(T,B; pattern=a) is S+; `a` V+, `b` V−. C=R(C,B; pattern=c) is S+
with an empty accepted language, not an automatically incorrect schema;
`a`,`b`,`c` are all V−. Empty pattern on string admits only empty, not `a`.
String pattern `a` rejects `ba` and `ab`; pattern `.*a.*` admits both.
R(T,xs:integer; pattern=`.*`) still rejects `x` because base lexical admission
is not replaced by a regex match. These are the concrete numbered
cvc-pattern-valid predicate and cvc-datatype-valid pattern branch witnesses.

### D2-029 — Enumeration set, inherited subset, and literal contexts

Source: [§4.3.5](../../upstream/datatypes.html#rf-enumeration).

| Exact construction/input | Expected outcome |
| --- | --- |
| B=R(B,xs:integer; enumeration=1,enumeration=2); `01`,`2`,`3` | S+; V+, V+, V− |
| Same B; T enumeration=2 | S+; `2` V+, `1` V− |
| Same B; T enumeration=3 | S− member outside base value space |
| R(T,xs:integer; enumeration=1.0) | S− enumeration literal not type-valid for integer |
| R(T,xs:decimal; enumeration=1.0,enumeration=01.00) | S+ one semantic value; `+1` V+ |
| R(T,xs:QName; enumeration=p:a); instance q:a under shared bindings | V+ equal expanded name |
| Same; instance p:a rebound to urn:b on v | V− different value despite same prefix spelling |
| R(T,xs:string; enumeration=abc,pattern=`[A-Z]+`) | S+ but `abc` V− pattern; no magic uppercasing |

Omitting a local enumeration retains the inherited restriction. Repeated
local enumerations form a set, not repeated singleton predicates. Booleans
and NOTATION have their distinct applicability/requirement cases in D2-007
and D2-011.

### D2-030 — Four numerical bound predicates

Source: [maxInclusive](../../upstream/datatypes.html#cvc-maxInclusive-valid),
[maxExclusive](../../upstream/datatypes.html#cvc-maxExclusive-valid),
[minExclusive](../../upstream/datatypes.html#cvc-minExclusive-valid),
[minInclusive](../../upstream/datatypes.html#cvc-minInclusive-valid).
Use independent R(T,xs:decimal) with one stated bound, endpoint 10.

| Facet | Input 9 | Input 10.00 | Input 11 |
| --- | --- | --- | --- |
| maxInclusive | V+ | V+ | V− |
| maxExclusive | V+ | V− | V− |
| minExclusive | V− | V− | V+ |
| minInclusive | V− | V+ | V+ |

These instantiate each rule's numbered numeric=true branch with lexical
admission already satisfied. NaN and infinities use D2-014; integer restrictions
with the same endpoint use the same order but have a discrete value space.

### D2-031 — Four chronological bound predicates

Source: same [four predicates](../../upstream/datatypes.html#cvc-maxInclusive-valid),
[dateTime order](../../upstream/datatypes.html#dateTime-order).
Use independent R(T,xs:dateTime) with one bound at
`2000-01-02T00:00:00Z`. Candidates A=`2000-01-01T00:00:00Z`,
B=`2000-01-02T00:00:00+00:00`, C=`2000-01-03T00:00:00Z`,
Q=`2000-01-02T00:00:00` without zone.

| Facet | A | B | C | Q |
| --- | --- | --- | --- | --- |
| maxInclusive | V+ | V+ | V− | V− |
| maxExclusive | V+ | V− | V− | V− |
| minExclusive | V− | V− | V+ | V− |
| minInclusive | V− | V+ | V+ | V− |

Each is numeric=false branch 2; equal aliases use chronological identity,
not spelling. Duration's independent indeterminate-order witness is D2-015.

### D2-032 — Same-step bounds and endpoint pairs

Source: [§§4.3.7–4.3.10 constraints](../../upstream/datatypes.html#maxInclusive-coss).
Use R(T,xs:decimal) with the stated facets.

| Local facets | Expected result |
| --- | --- |
| maxInclusive=10,maxExclusive=10 | S− forbidden two upper facets in one step |
| minInclusive=0,minExclusive=0 | S− forbidden two lower facets |
| minInclusive=9,maxInclusive=10; minInclusive=10,maxInclusive=10 | S+ both; equal pair admits exactly 10 |
| minInclusive=11,maxInclusive=10 | S− |
| minExclusive=9,maxExclusive=10; minExclusive=10,maxExclusive=10 | S+ both by the explicit ≤ constraint; equal pair admits no values |
| minExclusive=11,maxExclusive=10 | S− |
| minExclusive=9,maxInclusive=10 | S+ |
| minExclusive=10,maxInclusive=10 | S− strict relation required |
| minInclusive=9,maxExclusive=10 | S+ |
| minInclusive=10,maxExclusive=10 | S− strict relation required |
| xs:integer base, minExclusive=1,maxExclusive=2 | S+ but `1`,`2` V− and no integer lies between |

Schema emptiness is not itself a general forbidden condition. The equal
exclusive/exclusive pair is retained from the published same-step constraint,
not “repaired” to match an implementation's convenience.

### D2-033 — All sixteen inherited-bound numbered branches

Source: [maxInclusive restriction](../../upstream/datatypes.html#maxInclusive-valid-restriction),
[maxExclusive restriction](../../upstream/datatypes.html#maxExclusive-valid-restriction),
[minExclusive restriction](../../upstream/datatypes.html#minExclusive-valid-restriction),
[minInclusive restriction](../../upstream/datatypes.html#minInclusive-valid-restriction).

For every row B is R(B,xs:decimal; BASE-FACET=10), without fixed.
T is R(T,B; NEW-FACET=n), evaluated at **n=9,10,11** as three separate schemas.
The “clause” column says whether the named numbered comparison clause itself
permits the value. The “whole S” column additionally includes endpoint admission
and same-type lower/upper consistency; a co-failure is not a new isolated proof.

| New facet / base facet | Number in new-facet restriction rule | Clause permits n=9/10/11 | Whole S at n=9/10/11 |
| --- | --- | --- | --- |
| maxInclusive / maxInclusive | 1 | yes / yes / no | S+ / S+ / S− |
| maxInclusive / maxExclusive | 2 | yes / no / no | S+ / S− / S− |
| maxInclusive / minInclusive | 3 | no / yes / yes | S− / S+ / S+ |
| maxInclusive / minExclusive | 4 | no / no / yes | S− / S− / S+ |
| maxExclusive / maxExclusive | 1 | yes / yes / no | S+ / S+ / S−; equal exclusive endpoint uses the explicit exception in §4.3.8 |
| maxExclusive / maxInclusive | 2 | yes / yes / no | S+ / S+ / S− |
| maxExclusive / minInclusive | 3 | no / no / yes | S− / S− / S+ |
| maxExclusive / minExclusive | 4 | no / no / yes | S− / S− / S+ |
| minExclusive / minExclusive | 1 | no / yes / yes | S− / S+ / S+; equal exclusive endpoint uses §4.3.9's exception |
| minExclusive / maxInclusive | 2 | yes / yes / no | S+ / S− / S−; equality also violates minExclusive < maxInclusive |
| minExclusive / minInclusive | 3 | no / yes / yes | S− / Q / Q; inherited opposite-lower-kind scope in D-072 |
| minExclusive / maxExclusive | 4 | yes / no / no | S+ / S− / S− |
| minInclusive / minInclusive | 1 | no / yes / yes | S− / S+ / S+ |
| minInclusive / maxInclusive | 2 | yes / yes / no | S+ / S+ / S− |
| minInclusive / minExclusive | 3 | no / no / yes | S− / S− / Q; inherited opposite-lower-kind scope in D-072 |
| minInclusive / maxExclusive | 4 | yes / no / no | S+ / S− / S− |

All nonempty accepted restrictions can be observed by their remaining interval,
e.g. base minInclusive=10 and T maxInclusive=11 admits `10`/`11` but rejects
`9`/`12`. Fixed versions are D2-008. Values 9/11 rejected for leaving B's
space can necessarily co-fail endpoint admission; the numbered comparison is
still explicitly audited, but that schema does not prove isolated diagnostic
selection. Partial-order endpoint cases remain Q in D2-048 where the wording
does not supply a unique stronger schema-invalid conclusion.
The three Q cells above pass their named numbered comparison but retain
[D-072's inherited lower-kind scope question](../datatypes.md#d-072--questions-and-permissible-choices-that-must-remain-visible)
before whole-schema success. Their comparison columns and independent
comparison failures are unchanged; Q supplies no new rejection oracle.

### D2-034 — Digits validation, inheritance, fixedness, and interaction

Source: [totalDigits](../../upstream/datatypes.html#rf-totalDigits),
[fractionDigits](../../upstream/datatypes.html#rf-fractionDigits).

| Exact type or chain/input | Expected S/V |
| --- | --- |
| R(T,xs:decimal; totalDigits=2); `99`, `9.9`, `.99`, `00099.00` | S+, V+ each |
| Same; `100`, `.001` | V− each: no allowed i,n representation |
| R(T,xs:decimal; fractionDigits=2); `1.23`, `1.2300`, `0001.23` | V+ each; lexical trailing zeros do not increase required scale |
| Same; `1.231` | V− |
| R(T,xs:decimal; totalDigits=0) | S− facet value must be positiveInteger |
| R(T,xs:decimal; fractionDigits=-1) | S− facet value must be nonNegativeInteger |
| B totalDigits=3; T totalDigits=2 / 3 / 4 | S+ / S+ / S− |
| B fractionDigits=2; T fractionDigits=1 / 2 / 3 | S+ / S+ / S− |
| xs:decimal; totalDigits=2,fractionDigits=2 | S+ |
| xs:decimal; totalDigits=2,fractionDigits=3 | S− fractionDigits > totalDigits |
| B totalDigits=2; T fractionDigits=3 | S− effective inherited/local interaction |
| R(T,xs:integer; fractionDigits=1) | S− inherited fixed zero |
| R(T,xs:decimal; fractionDigits=0); `1.0` | V+ mathematically integral; differs from xs:integer lexical rejection |
| R(T,xs:decimal; totalDigits=3,fractionDigits=1,minInclusive=36.4,maxInclusive=40.5); `36.4`,`40.5`,`36.35`,`41.0` | V+,V+,V− fraction/value boundary,V− upper bound |

Use `39.99` as the precision-only failure inside the last interval and `41`
as a digits-admitted value failing only the range. With totalDigits=2, value
`1000` is V−; deleting trailing integer zeros to pretend it has one digit is
forbidden because the exponent n is nonnegative.

## Appendix F: concrete production witnesses

All regex cases use R(T,xs:string; pattern=PATTERN) unless stated otherwise.
An invalid regex is S−; matching/nonmatching strings under a correct regex are
V+/V−. Pattern value XML attributes must escape `&` and `<` before the regex
parser sees them. Whole-string matching is assumed as explicitly specified.

### D2-035 — regExp, branch, piece, atom

Source: [regExp](../../upstream/datatypes.html#regex),
[branch](../../upstream/datatypes.html#branch),
[piece](../../upstream/datatypes.html#piece),
[atom](../../upstream/datatypes.html#atom).

PATTERN empty is S+ and admits empty only; `a` is V−.
PATTERN `ab` admits `ab`, rejects `a` and `abc`.
PATTERN `a|b` admits `a` and `b`, rejects `ab` and empty.
PATTERN `|a`, `a|`, and `a||b` are each S+ with an empty branch and admit empty;
`a||b` also admits `a` and `b`. PATTERN `()` admits empty, rejects `a`.
PATTERN `a(b|c)d` admits `abd`,`acd`, rejects `ad`,`abcd`.
PATTERN `(ab)*` admits empty, `ab`, `abab`, rejects `aba`.
These concretely cover normal-character, class, and parenthesized atoms when
combined with PATTERN `[ab]` admitting `a`/`b` but rejecting `ab`.

PATTERN `(`, `)`, or `a(b` is S− unbalanced group. PATTERN `*a` is S− missing
atom. PATTERN `a` rejects `ba`/`ab`, whereas `.*a.*` admits both; no searching
API semantics are substituted. Recursive witnesses include `(a(b|c))*`
admitting `abac`, rejecting `aba`, and `[a-z-[d-w-[m-o]]]` in D2-040.

### D2-036 — All quantifier/quantity alternatives and exact counts

Source: [quantifier](../../upstream/datatypes.html#quant),
[quantity](../../upstream/datatypes.html#quantity),
[quantRange](../../upstream/datatypes.html#quantRange),
[quantMin](../../upstream/datatypes.html#quantMin),
[QuantExact](../../upstream/datatypes.html#quantExact).

| PATTERN | V+ exact strings | V− exact strings |
| --- | --- | --- |
| `a?` | empty, `a` | `aa` |
| `a*` | empty, `a`, `aa` | `b` |
| `a+` | `a`, `aa` | empty, `b` |
| `a{2}` | `aa` | `a`, `aaa` |
| `a{2,3}` | `aa`, `aaa` | `a`, `aaaa` |
| `a{2,}` | `aa`, `aaa`, `aaaa` | empty, `a` |
| `a{0}`, `a{0,0}` | empty | `a` |
| `a{0,1}` | empty, `a` | `aa` |
| `a{02}` | `aa` | `a`, `aaa` |
| `()*` | empty | `a` |

`a**`, `a+?`, and `a??` are S− consecutive quantifiers; `a{3,2}` violates
n≤m. `{,2}`, `{+2}`, `{٢}`, `{2, 3}`, and `{2` are not quantifier productions.
As **whole patterns** their unescaped braces meet the published Char/prose
conflict recorded in D2-048, so that conflict must be settled before marking
each such entire pattern S−. PATTERN `a{1000000}` is a valid language-form
case with V+ input defined exactly as one million a characters and V− input
999999 a characters; an operational regex budget refusal is not a different
language oracle. This is a parameterized large witness with exact counts,
not a demand to inline the million-character string here.

### D2-037 — Char and every SingleCharEsc

Source: [Char](../../upstream/datatypes.html#char),
[SingleCharEsc](../../upstream/datatypes.html#singlecharesc).
Each pair below is an independent exact PATTERN → its one-character V+ input.
Input `x` is V− for every listed pattern; an empty input is also V−.

| PATTERN | Exact V+ character |
| --- | --- |
| `\n` | U+000A |
| `\r` | U+000D |
| `\t` | U+0009 |
| `\\` | U+005C backslash |
| backslash followed by U+007C | U+007C vertical bar |
| `\.` | U+002E dot |
| `\-` | U+002D hyphen |
| `\^` | U+005E caret |
| `\?` | U+003F question mark |
| `\*` | U+002A asterisk |
| `\+` | U+002B plus |
| `\{` | U+007B left brace |
| `\}` | U+007D right brace |
| `\(` | U+0028 left parenthesis |
| `\)` | U+0029 right parenthesis |
| `\[` | U+005B left bracket |
| `\]` | U+005D right bracket |

PATTERN `^a$` admits exactly `^a$`, rejects `a`; those are literal characters,
not anchors. PATTERN `-` admits `-`. Normal PATTERN `A` admits `A`, rejects `a`.
A single terminal backslash, `\a`, `\b`, `\f`, `\v`, `\x41`, `\u0041`,
and `\1` are S−: none is a recognized single/multi/property escape.
Braces' unescaped role is a source tension, not resolved by the escaped cases.

### D2-038 — charClass, charClassExpr, charGroup, positive and negative groups

Source: [charClass](../../upstream/datatypes.html#charClass),
[charClassExpr](../../upstream/datatypes.html#charClassExpr),
[charGroup](../../upstream/datatypes.html#chargroup),
[posCharGroup](../../upstream/datatypes.html#poschargroup),
[negCharGroup](../../upstream/datatypes.html#negchargroup),
[charClassEsc](../../upstream/datatypes.html#charclassesc).

| PATTERN | Required result |
| --- | --- |
| `[ab]` | V+ `a`,`b`; V− empty,`ab`,`c` |
| `[aba]` | Same language as `[ab]`; duplicate ranges do not add multiplicity. |
| `[a-cx\d]` | V+ `a`,`c`,`x`,`3`, U+0661; V− `d`,`y` |
| `[^ab]` | V+ `c`, LF, U+1D11E; V− `a`,`b`,empty |
| `[a^]` | V+ `a`,`^`; V− `b` |
| `[\^a]` | V+ `^`,`a`; no negation |
| `[.]` | V+ literal dot, V− `a`; dot inside a positive group is a literal range character. |
| `[]`, `[^]`, `[a`, `a]` | S− missing/nonempty group or unmatched bracket as applicable. |

The class complement is over XML characters: U+0000 fails XML before any
negated class could accept it. `\p{Lu}` as a standalone class escape admits
`A`, rejects `a`; `\P{Lu}` admits `a`, rejects `A`. This covers both
property/complement charClassEsc alternatives independently of square brackets.

### D2-039 — charRange, seRange, charOrEsc, XmlChar, XmlCharIncDash

Source: [range productions](../../upstream/datatypes.html#charrange),
[seRange](../../upstream/datatypes.html#serange),
[charOrEsc](../../upstream/datatypes.html#charoresc),
[XmlChar](../../upstream/datatypes.html#xmlchar),
[XmlCharIncDash](../../upstream/datatypes.html#XmlCharIncDash).

| PATTERN | Concrete outcome |
| --- | --- |
| `[a-c]` | V+ `a`,`b`,`c`; V− U+0060,`d` |
| `[a-a]` | V+ `a`; V− `b` |
| `[z-a]` | S− decreasing endpoints |
| `[\n-\r]` | V+ LF/CR; tab V−; intervening XML-forbidden controls are X, not accepted input strings |
| `[\--0]` | V+ `-`,`.`,`/`,`0`; V− `1`; escaped hyphen serves as lower endpoint |
| `[-a]`, `[a-]` | V+ `-`,`a`; V− `b`; boundary hyphen literal |
| `[a\-z]` | V+ `a`,`-`,`z`; V− `b` (no a-z range) |
| `[\d-a]` | S− a multicharacter class cannot serve as a singleton range endpoint |
| `[a-\p{Lu}]` | S− category class is not one endpoint |
| `[\[-\]]` | V+ `[`, backslash, `]`; V− `a`; escaped bracket endpoints have increasing code points |

A direct supplementary-range sketch `[U+10000-U+10002]` means the exact
pattern made from `[`, those literal endpoints, `-`, `]`; U+10000/U+10002
are V+, U+10003 V−. It uses code points, never UTF-16 surrogate ranges.

### D2-040 — charClassSub, nested subtraction, and placement

Source: [charClassSub](../../upstream/datatypes.html#charclasssub).

| PATTERN | Concrete outcome |
| --- | --- |
| `[a-z-[aeiou]]` | V+ `b`,`z`; V− `a`,`e`,`A` |
| `[a-z-[a-z]]` | No one-character member; `a` and empty V− |
| `[a-z-[a-z]]*` | Empty V+; `a` V−; empty set differs from empty string |
| `[^a-[b]]` | V+ `c`; V− `a`,`b` |
| `[a-z-[^aeiou]]` | V+ `a`,`e`; V− `b`; subtracting complement retains vowels |
| `[a-z-[d-w-[m-o]]]` | V+ `a`,`m`,`n`,`o`,`x`; V− `d`,`l`,`p`,`w` |
| `[\p{L}-[A-Z]]` | V+ `a`, U+03B1; V− `A`,`3` |
| `[a-z-[aeiou]x]` | S− material after subtraction operand in same group |
| `[a-z-[]]` | S− empty subtrahend expression |

PATTERN `[a-z&&[^aeiou]]` is not a specified intersection expression;
its bare nested `[` does not satisfy this subtraction grammar. It must not
be treated as another regex dialect's set intersection. Group subtraction
requires its `-` and bracketed operand.

### D2-041 — Every category production and recognized category value

Source: [catEsc](../../upstream/datatypes.html#catesc),
[complEsc](../../upstream/datatypes.html#complesc),
[charProp](../../upstream/datatypes.html#charprop),
[IsCategory](../../upstream/datatypes.html#cats),
[Letters](../../upstream/datatypes.html#lets),
[Marks](../../upstream/datatypes.html#marks),
[Numbers](../../upstream/datatypes.html#nums),
[Punctuation](../../upstream/datatypes.html#punc),
[Separators](../../upstream/datatypes.html#seps),
[Symbols](../../upstream/datatypes.html#syms),
[Others](../../upstream/datatypes.html#others),
[retained Unicode 3.1 database](../../dependencies/UnicodeData-3.1.0.txt).

Each row fixes PATTERN=`\p{NAME}`, a one-character V+ member and V− nonmember.
PATTERN=`\P{NAME}` reverses those two outcomes. This makes both property
escape alternatives concrete for **every recognized category token**.
The table pins historical category membership; later permitted Unicode
assignments are not silently used as the baseline.

| NAME | V+ exact code point | V− exact code point |
| --- | --- | --- |
| L | U+0041 | U+0030 |
| Lu | U+0041 | U+0030 |
| Ll | U+0061 | U+0030 |
| Lt | U+01C5 | U+0030 |
| Lm | U+02B0 | U+0030 |
| Lo | U+01BB | U+0030 |
| M | U+0300 | U+0041 |
| Mn | U+0300 | U+0041 |
| Mc | U+0903 | U+0041 |
| Me | U+0488 | U+0041 |
| N | U+0030 | U+0041 |
| Nd | U+0030 | U+0041 |
| Nl | U+16EE | U+0041 |
| No | U+00B2 | U+0041 |
| P | U+0021 | U+0041 |
| Pc | U+005F | U+0041 |
| Pd | U+002D | U+0041 |
| Ps | U+0028 | U+0041 |
| Pe | U+0029 | U+0041 |
| Pi | U+00AB | U+0041 |
| Pf | U+00BB | U+0041 |
| Po | U+0021 | U+0041 |
| Z | U+0020 | U+0041 |
| Zs | U+0020 | U+0041 |
| Zl | U+2028 | U+0041 |
| Zp | U+2029 | U+0041 |
| S | U+0024 | U+0041 |
| Sm | U+002B | U+0041 |
| Sc | U+0024 | U+0041 |
| Sk | U+005E | U+0041 |
| So | U+00A6 | U+0041 |
| C | U+0009 | U+0041 |
| Cc | U+0009 | U+0041 |
| Cf | U+070F | U+0041 |
| Co | U+E000 | U+0041 |
| Cn | U+0378 | U+0041 |

`\p{Cs}`, `\p{lu}`, `\p{Letter}`, `\p{}`, and `\pLu` are S− category
syntax/name cases. `Cs` is expressly excluded; isolated surrogates cannot
reach matching through XML. `\p{L}` matches A but does not imply an entire
base-plus-combining-mark sequence matches one character; `A` plus U+0301 is
V− under a single `\p{L}`, V+ under `\p{L}\p{M}`.

### D2-042 — IsBlock, all published block intervals, and complements

Source: [IsBlock and its table](../../upstream/datatypes.html#blockesc),
[retained historical Blocks file](../../dependencies/Blocks-3.1.0.txt).
Each row is one exact published interval. PATTERN is `\p{IsNAME}`;
its listed endpoints are V+, and its listed outside neighbors V− unless X is
stated. The complement `\P{IsNAME}` reverses V+/V− for XML-admitted
characters. For a singleton interval both endpoints are the same literal.
The two Specials intervals are one union; the table's neighbors lie outside
both. These are concrete numeric Unicode literals under the shared notation.

| NAME | V+ lower and upper literals | Immediately outside literals |
| --- | --- | --- |
| BasicLatin | U+0000 is X; use U+0020 inside block, U+007F | no lower code point, U+0080 |
| Latin-1Supplement | U+0080, U+00FF | U+007F, U+0100 |
| LatinExtended-A | U+0100, U+017F | U+00FF, U+0180 |
| LatinExtended-B | U+0180, U+024F | U+017F, U+0250 |
| IPAExtensions | U+0250, U+02AF | U+024F, U+02B0 |
| SpacingModifierLetters | U+02B0, U+02FF | U+02AF, U+0300 |
| CombiningDiacriticalMarks | U+0300, U+036F | U+02FF, U+0370 |
| Greek | U+0370, U+03FF | U+036F, U+0400 |
| Cyrillic | U+0400, U+04FF | U+03FF, U+0500 |
| Armenian | U+0530, U+058F | U+052F, U+0590 |
| Hebrew | U+0590, U+05FF | U+058F, U+0600 |
| Arabic | U+0600, U+06FF | U+05FF, U+0700 |
| Syriac | U+0700, U+074F | U+06FF, U+0750 |
| Thaana | U+0780, U+07BF | U+077F, U+07C0 |
| Devanagari | U+0900, U+097F | U+08FF, U+0980 |
| Bengali | U+0980, U+09FF | U+097F, U+0A00 |
| Gurmukhi | U+0A00, U+0A7F | U+09FF, U+0A80 |
| Gujarati | U+0A80, U+0AFF | U+0A7F, U+0B00 |
| Oriya | U+0B00, U+0B7F | U+0AFF, U+0B80 |
| Tamil | U+0B80, U+0BFF | U+0B7F, U+0C00 |
| Telugu | U+0C00, U+0C7F | U+0BFF, U+0C80 |
| Kannada | U+0C80, U+0CFF | U+0C7F, U+0D00 |
| Malayalam | U+0D00, U+0D7F | U+0CFF, U+0D80 |
| Sinhala | U+0D80, U+0DFF | U+0D7F, U+0E00 |
| Thai | U+0E00, U+0E7F | U+0DFF, U+0E80 |
| Lao | U+0E80, U+0EFF | U+0E7F, U+0F00 |
| Tibetan | U+0F00, U+0FFF | U+0EFF, U+1000 |
| Myanmar | U+1000, U+109F | U+0FFF, U+10A0 |
| Georgian | U+10A0, U+10FF | U+109F, U+1100 |
| HangulJamo | U+1100, U+11FF | U+10FF, U+1200 |
| Ethiopic | U+1200, U+137F | U+11FF, U+1380 |
| Cherokee | U+13A0, U+13FF | U+139F, U+1400 |
| UnifiedCanadianAboriginalSyllabics | U+1400, U+167F | U+13FF, U+1680 |
| Ogham | U+1680, U+169F | U+167F, U+16A0 |
| Runic | U+16A0, U+16FF | U+169F, U+1700 |
| Khmer | U+1780, U+17FF | U+177F, U+1800 |
| Mongolian | U+1800, U+18AF | U+17FF, U+18B0 |
| LatinExtendedAdditional | U+1E00, U+1EFF | U+1DFF, U+1F00 |
| GreekExtended | U+1F00, U+1FFF | U+1EFF, U+2000 |
| GeneralPunctuation | U+2000, U+206F | U+1FFF, U+2070 |
| SuperscriptsandSubscripts | U+2070, U+209F | U+206F, U+20A0 |
| CurrencySymbols | U+20A0, U+20CF | U+209F, U+20D0 |
| CombiningMarksforSymbols | U+20D0, U+20FF | U+20CF, U+2100 |
| LetterlikeSymbols | U+2100, U+214F | U+20FF, U+2150 |
| NumberForms | U+2150, U+218F | U+214F, U+2190 |
| Arrows | U+2190, U+21FF | U+218F, U+2200 |
| MathematicalOperators | U+2200, U+22FF | U+21FF, U+2300 |
| MiscellaneousTechnical | U+2300, U+23FF | U+22FF, U+2400 |
| ControlPictures | U+2400, U+243F | U+23FF, U+2440 |
| OpticalCharacterRecognition | U+2440, U+245F | U+243F, U+2460 |
| EnclosedAlphanumerics | U+2460, U+24FF | U+245F, U+2500 |
| BoxDrawing | U+2500, U+257F | U+24FF, U+2580 |
| BlockElements | U+2580, U+259F | U+257F, U+25A0 |
| GeometricShapes | U+25A0, U+25FF | U+259F, U+2600 |
| MiscellaneousSymbols | U+2600, U+26FF | U+25FF, U+2700 |
| Dingbats | U+2700, U+27BF | U+26FF, U+27C0 |
| BraillePatterns | U+2800, U+28FF | U+27FF, U+2900 |
| CJKRadicalsSupplement | U+2E80, U+2EFF | U+2E7F, U+2F00 |
| KangxiRadicals | U+2F00, U+2FDF | U+2EFF, U+2FE0 |
| IdeographicDescriptionCharacters | U+2FF0, U+2FFF | U+2FEF, U+3000 |
| CJKSymbolsandPunctuation | U+3000, U+303F | U+2FFF, U+3040 |
| Hiragana | U+3040, U+309F | U+303F, U+30A0 |
| Katakana | U+30A0, U+30FF | U+309F, U+3100 |
| Bopomofo | U+3100, U+312F | U+30FF, U+3130 |
| HangulCompatibilityJamo | U+3130, U+318F | U+312F, U+3190 |
| Kanbun | U+3190, U+319F | U+318F, U+31A0 |
| BopomofoExtended | U+31A0, U+31BF | U+319F, U+31C0 |
| EnclosedCJKLettersandMonths | U+3200, U+32FF | U+31FF, U+3300 |
| CJKCompatibility | U+3300, U+33FF | U+32FF, U+3400 |
| CJKUnifiedIdeographsExtensionA | U+3400, U+4DB5 | U+33FF, U+4DB6 |
| CJKUnifiedIdeographs | U+4E00, U+9FFF | U+4DFF, U+A000 |
| YiSyllables | U+A000, U+A48F | U+9FFF, U+A490 |
| YiRadicals | U+A490, U+A4CF | U+A48F, U+A4D0 |
| HangulSyllables | U+AC00, U+D7A3 | U+ABFF, U+D7A4 |
| PrivateUse | U+E000, U+F8FF | U+DFFF is X, U+F900 |
| CJKCompatibilityIdeographs | U+F900, U+FAFF | U+F8FF, U+FB00 |
| AlphabeticPresentationForms | U+FB00, U+FB4F | U+FAFF, U+FB50 |
| ArabicPresentationForms-A | U+FB50, U+FDFF | U+FB4F, U+FE00 |
| CombiningHalfMarks | U+FE20, U+FE2F | U+FE1F, U+FE30 |
| CJKCompatibilityForms | U+FE30, U+FE4F | U+FE2F, U+FE50 |
| SmallFormVariants | U+FE50, U+FE6F | U+FE4F, U+FE70 |
| ArabicPresentationForms-B | U+FE70, U+FEFE | U+FE6F, U+FEFF |
| Specials | U+FEFF, U+FEFF | U+FEFE, U+FF00 |
| HalfwidthandFullwidthForms | U+FF00, U+FFEF | U+FEFF, U+FFF0 |
| Specials | U+FFF0, U+FFFD | U+FFEF, U+FFFE is X |

PATTERN `\p{IsBasicLatin}` admits A, rejects U+0080;
`\p{IsGreek}` admits U+03B1, rejects A. `\p{IsBasic Latin}` is S− because
the grammar has no internal space; `\p{isGreek}` and `\p{IsNotABlock}`
are S− unrecognized names. `\p{IsHighSurrogates}` is expressly excluded,
not a method to admit surrogate code units. Membership does not imply an
assigned character: an unassigned code point inside an admitted block is still
in that block. Supplementary blocks appearing in the retained historical
Unicode database but absent from the appendix table, and later version names,
need recorded Unicode-version support under D2-048 before a conformance score.

### D2-043 — MultiCharEsc, WildcardEsc, and every shorthand

Source: [MultiCharEsc](../../upstream/datatypes.html#multicharesc),
[WildcardEsc](../../upstream/datatypes.html#wildcardesc).
Each row is one-character matching; the complement row has its own concrete
witness instead of merely being inferred from a host character class.

| PATTERN | Exact V+ input | Exact V− input |
| --- | --- | --- |
| `.` | tab, A, U+1D11E | LF, CR |
| `\s` | U+0020, tab, LF, CR | U+00A0, A |
| `\S` | U+00A0, A | U+0020, tab |
| `\i` | A, underscore, colon, U+00C0 | digit 1, hyphen, U+0301 |
| `\I` | digit 1, hyphen, U+0301 | A, underscore, colon |
| `\c` | A, digit 1, hyphen, colon, U+0301 | U+0020 |
| `\C` | U+0020 | A, digit 1, hyphen |
| `\d` | ASCII 1, U+0661 | A, U+2160 (Nl) |
| `\D` | A, U+2160 | ASCII 1, U+0661 |
| `\w` | A, digit 1, U+0301, plus sign | underscore (Pc), space (Zs), tab (Cc) |
| `\W` | underscore, space, tab | A, digit 1, U+0301 |

`\i`/`\c` use the referenced XML name rules; modern programming-language
identifier rules are not substituted. `\w` excludes all P/Z/C categories,
so underscore is not a word character here. Decimal datatype input U+0661 is
V− even though regex `\d` admits it. Single dot consumes one supplementary
code point, so U+1D11E is V+ and two copies V−.

### D2-044 — XML transport, exact characters, and excluded extensions

Source: [Appendix F](../../upstream/datatypes.html#regexs),
[exact matching](../../upstream/datatypes.html#terminology).

| Exact schema/input | Expected result |
| --- | --- |
| `<xs:pattern value="&#x2E;"/>`; value A | V+ because XML produces unescaped regex dot |
| `<xs:pattern value="\&#x2E;"/>`; value dot / A | V+ / V− because XML produces regex `\.` |
| `<xs:pattern value="&amp;"/>`; XML `<v>&amp;</v>` | V+ exact ampersand |
| `<xs:pattern value="a&#xA;b"/>`; value `a`+LF+`b` | V+; same value with space instead of LF V− |
| `<xs:pattern value="a&#xA;b"/>`; literal schema line break replaces the reference | XML attribute normalization changes the regex to `a b`; LF input V−, space input V+ |
| Pattern U+00E9; value `e`+U+0301 | V− no normalization |
| Pattern `[A-Z]`; value `a` | V− no case folding |
| `(?i)a`, `(?:a)`, `(?=a)a`, `(a)\1` | S− special group syntax/backreference absent from grammar |
| Pattern `^a$`; value a | V− because ^/$ are literals, not a syntax-error assertion |

CDATA input `<v><![CDATA[a&b]]></v>` and `<v>a&amp;b</v>` have the same character
sequence for datatype matching. An XML-invalid unescaped ampersand cannot be
used to prove a regex rejection. No Unicode Level 2 grapheme/normalization
feature is mandated by the appendix's Level 1 target.

## External seams, appendices, and dispositions

### D2-045 — anyURI primary-source escaping and scheme-independent validity

Source: [anyURI](../../upstream/datatypes.html#anyURI),
[XLink §5.4](../../dependencies/xlink-1.0.html#link-locators),
[RFC2396](../../dependencies/rfc2396.txt),
[RFC2732](../../dependencies/rfc2732.txt).

| Exact anyURI literal | Expected semantic result |
| --- | --- |
| empty, `../a`, `?q`, `#f` | V+ URI references; no absolutization required during validation. |
| `a b` | V+ after internal space escapes to `a%20b`; discouraged space use is not V−. |
| `caf`+U+00E9 | V+ after UTF-8 escaping to `caf%C3%A9`. |
| `%20` | Valid existing escape, not rewritten to a literal space for identity without another rule. |
| `%ZZ`, `%`, `%2` | V− illegal percent-escape structure after the XLink procedure, which does not escape literal percent. |
| `http://[::1]/` | V+ IPv6 literal authority under RFC2732. |
| `nosuchscheme:abc` | No registered-scheme/network-existence rejection is required; generic scheme syntax is sufficient. |
| `a b` under xml:base=`http://one/` versus `http://two/` | Both V+; validation must not absolutize the relative datatype value. |

TEXT=`  a   b  ` first collapses to `a b`, then the URI-reference escaping
projection is `a%20b`. Scheme-specific fragment validity and resource access
are not minimally conforming datatype obligations. URI spelling identity
questions remain distinct from generic URI-reference admission; D2-048 does
not assume percent-decoding, host lowercasing, or path normalization is E.

### D2-046 — Appendix A grammar, B limitations, C properties, G–I metadata

Source: [A](../../upstream/datatypes.html#schema),
[B](../../upstream/datatypes.html#dtd-for-datatypeDefs),
[C](../../upstream/datatypes.html#app-fundamental-facets),
[G](../../upstream/datatypes.html#normative-glossary),
[H](../../upstream/datatypes.html#biblio),
[I](../../upstream/datatypes.html#acknowledgments).

Concrete Appendix A witnesses are D2-002/007/008: required global name,
prohibited local name/final, constructor choices, one leading annotation,
facet numeric types, boolean fixed aliases, and noFixedFacet's prohibition.
A foreign unqualified attribute `extra="x"` on simpleType is S−;
`xmlns:f="urn:foreign" f:extra="x"` is admitted by the ##other lax wildcard
when no governing foreign declaration is available. If a known foreign
attribute has type xs:integer, `f:extra="x"` fails its lax assessment while
`f:extra="1"` satisfies it. Annotation payload does not alter value acceptance.
The helper derivationControl is not an additional public builtin type.

Appendix B is nonnormative and not standalone. Its loose `name #IMPLIED`
does not make a nameless global simpleType correct; its boolean enum true/false
does not repeal valid `fixed="1"`. A duplicate length facet can fit its
repeated facet content model while remaining S− by src-single-facet-value.
DTD-validity is therefore not an oracle for those component rules.

Appendix C's builtin property row `boolean: ordered=false,bounded=false,
cardinality=finite,numeric=false` and `byte: total,true,finite,true` are exact
P witnesses. `gMonth: partial,false,countably infinite,false` is the published
classification despite its finite-looking recurring domain; D2-048 records
the source/formula tension. G's glossary is nonnormative despite its
`normative-glossary` anchor. H's references identify dependencies, while I's
acknowledgements add no instance/schema predicate. They receive informative
classification, not fabricated invalid examples. D2-016–D2-020 and D2-035–D2-044
supply the concrete D/E/F appendix decisions.

### D2-047 — Composite, infeasible, and observation dispositions

Source: [conformance](../../upstream/datatypes.html#conformance),
[constraints terminology](../../upstream/datatypes.html#constraints-and-contributions).

The branch mapping below uses `reviewed` for a source branch with concrete
witnesses and an explicit disposition. This does not mean every possible
interaction has become a fixture or a successful validation run.

- Parent branches (`cvc-datatype-valid` lexical parent/no-pattern parent,
  `cvc-length-valid` atomic parent and its min/max analogues) are reviewed
  composites through concrete child alternatives; they are not independent
  additional tests or separate semantic fallbacks.
- QName/NOTATION length branches have **no rejecting value witness once the
  facet is admitted**, because the source says every length facet value is
  satisfied. D2-024's zero/large no-op examples are the distinguishing oracle.
- An inapplicable facet or malformed schema has no V outcome; it cannot count
  as a proof of the intended runtime facet. An XML-forbidden character has no
  datatype/regex outcome. A temporal comparison input needing an unsupported
  or unresolved source interpretation remains Q.
- For inherited-bound clauses, some false comparisons inevitably also violate
  base endpoint admission. D2-033 reports this co-constraint rather than
  claiming the failure isolates a diagnostic path.
- `P`/`K` facts require an exposing seam or a valid distinguishing E witness.
  Absence of PSVI/canonical output is not a pass and does not require the
  library to expose a new API for this audit.

### D2-048 — Source tensions and exact unresolved or permitted oracles

Source: cited clauses below, and the captured
[errata boundary](../../corrected-reading.md). None of these is resolved by
current implementation behavior.

| Concrete question/witness | Disposition and primary-source reason |
| --- | --- |
| Mixed-zone separation exactly 14h in D2-018 | **Settled incomparable:** opening §3.2.7.4 explicitly says 14h or less, despite later shorthand “otherwise” inequalities omitting equality. |
| One-member union in D2-002 | **Settled S+:** §4.1.1 nonempty member sequence, representation constraint, and Appendix A allow it; informal §2.5.1.3 “greater than 1” does not impose a second member. |
| gMonth `--01` versus `--01--` | **Settled V+ / V−:** dated §3.2.14 current form is `--MM`; deleted historical XML text is excluded. |
| Repeated exclusive endpoint in D2-033 | **Settled S+ for otherwise valid same-facet endpoint repetition:** §4.3.8/4.3.9 introductory text explicitly allows equality to the base exclusive endpoint. A generic base instance-validity check would conflict. |
| PATTERN `a{`, `{`, `}` as whole regexes | **Q unresolved — RXQ001/RGQ001:** Char production excludes no braces while normal-character prose calls braces metacharacters. Escaped braces and legal quantifiers are settled; this publication does not supply an unambiguous bare-brace oracle. |
| Smallest double positive value and decimal underflow tie | **Q unresolved extremal oracle:** §3.2.5 describes IEEE double but prints exponent lower bound −1075. Midpoint witnesses D2-014 avoid the inconsistency. |
| dateTime `2000-12-31T23:59:60Z`; `2000-01-01T00:00:60Z`; second 61 | **Q affected lexical/chronological cases:** Appendix D permits second 60 and discusses leap seconds at appropriate/inappropriate dates; body field limits and historical schedule wording do not reduce to a blanket ≤59 parser rule. No live leap-second table is required by Appendix E. |
| Adding PT1M versus PT60S to an admitted second-60 dateTime | **Settled same result under Appendix E arithmetic:** leap seconds are treated as overflow and subsequent minutes have 60 seconds. This does not settle lexical admission of every second-60 input. |
| `-0001-12-31 + P1D` at BCE/CE seam | **Q arithmetic interpretation:** lexical year zero is forbidden and −0001 denotes 1 BCE; Appendix E raw year arithmetic needs a consistent historical-to-integer mapping not silently specified by a host astronomical calendar. |
| time `23:30:00-01:00` versus `00:30:00Z`; recurring g* timezone rollover | **Q exact cross-reference identity where day/year carry matters:** source says recurring values and arbitrary reference date but supplies no standalone canonical g* algorithm; record the chosen primary-source interpretation before assigning an E result. Ordinary same-reference lexical/order witnesses remain determined. |
| Fixed-length list of unrestricted integer, length=2, cardinality | **Published P finite per §4.2.4 formula**, while mathematical item-space reasoning suggests an infinite set. Preserve the specified classification and label this source tension instead of substituting an intuitive P. |
| gMonth cardinality | **Published P countably infinite in Appendix C**; do not silently replace with a mathematical finite count. |
| Unicode code point whose category changed after the bibliography's 3.1 database | **Q version-dependent — RXQ002:** publication-time-current support is required and later revisions encouraged; record version and exact primary evidence. D2-041 pins a retained dataset for its concrete stable cases. |
| Unknown block in a later Unicode version, e.g. `\p{IsEmoticons}` | **Permitted support question**, not automatically a required rejection under the source's future-version allowance. Historical-table names D2-042 are required witnesses. |
| anyURI `a%62` versus `ab` as values | **Q value-identity/normalization seam:** generic URI syntax admission is determined, but no assumed percent-decoding/case-folding/absolutization establishes E from the anyURI paragraph alone. |
| decimal 19-digit input with documented 18-digit maximum | **Permitted documented precision limit**, not malformed lexical/value-space input. Required 18-digit case D2-022 remains mandatory. |

A Q cell is an outstanding source-qualified question, not a completed
acceptance/rejection oracle. These limits are retained so future corpus
comparison cannot mislabel an implementation choice or unobserved fact as a
confirmed specification gap.

## Every regex production accounted for

The published appendix has **37 current productions**. Number 19
`xmlcharref`/`nt-XmlCharRef` is deleted historical XML; current `wildcardesc`
is numbered 37a. It is not a thirty-eighth current rule.

| Inventory production ID | Published anchor | Concrete cases |
| --- | --- | --- |
| `nt-regExp` | `regex` | D2-035 |
| `nt-branch` | `branch` | D2-035 |
| `nt-piece` | `piece` | D2-035, D2-036 |
| `nt-quantifier` | `quant` | D2-036 |
| `nt-quantity` | `quantity` | D2-036 |
| `nt-quantRange` | `quantRange` | D2-036 |
| `nt-quantMin` | `quantMin` | D2-036 |
| `nt-QuantExact` | `quantExact` | D2-036 |
| `nt-atom` | `atom` | D2-035, D2-038 |
| `nt-Char` | `char` | D2-037, D2-044, D2-048 |
| `nt-charClass` | `charClass` | D2-038, D2-043 |
| `nt-charClassExpr` | `charClassExpr` | D2-038 |
| `nt-charGroup` | `chargroup` | D2-038, D2-040 |
| `nt-posCharGroup` | `poschargroup` | D2-038 |
| `nt-negCharGroup` | `negchargroup` | D2-038 |
| `nt-charClassSub` | `charclasssub` | D2-040 |
| `nt-charRange` | `charrange` | D2-039 |
| `nt-seRange` | `serange` | D2-039 |
| `nt-charOrEsc` | `charoresc` | D2-039 |
| `nt-XmlChar` | `xmlchar` | D2-039 |
| `nt-XmlCharIncDash` | `XmlCharIncDash` | D2-039 |
| `nt-charClassEsc` | `charclassesc` | D2-038, D2-041, D2-043 |
| `nt-SingleCharEsc` | `singlecharesc` | D2-037 |
| `nt-catEsc` | `catesc` | D2-041 |
| `nt-complEsc` | `complesc` | D2-041 |
| `nt-charProp` | `charprop` | D2-041, D2-042 |
| `nt-IsCategory` | `cats` | D2-041 |
| `nt-Letters` | `lets` | D2-041 |
| `nt-Marks` | `marks` | D2-041 |
| `nt-Numbers` | `nums` | D2-041 |
| `nt-Punctuation` | `punc` | D2-041 |
| `nt-Separators` | `seps` | D2-041 |
| `nt-Symbols` | `syms` | D2-041 |
| `nt-Others` | `others` | D2-041 |
| `nt-IsBlock` | `blockesc` | D2-042 |
| `nt-MultiCharEsc` | `multicharesc` | D2-043 |
| `nt-WildcardEsc` | `wildcardesc` | D2-043 |

## All named constraints and numbered-branch dispositions

The first table includes all 43 named constraints, including those without
numbered subclauses. The second table gives every one of the inventory's 58
numbered keys an explicit disposition. Source fragments are the constraint IDs
in the [dated Part 2](../../upstream/datatypes.html); numbers are recorded in
[the inventory](../../inventory.json). A table row is traceability evidence,
not proof that a branch has been executed against any corpus.

| Named constraint ID | Concrete cases | Disposition |
| --- | --- | --- |
| `enumeration-required-notation` | D2-011 | reviewed: concrete schema/value witnesses above |
| `src-single-facet-value` | D2-008 | reviewed: concrete schema/value witnesses above |
| `src-list-itemType-or-simpleType` | D2-002 | reviewed: concrete schema/value witnesses above |
| `src-restriction-base-or-simpleType` | D2-002 | reviewed: concrete schema/value witnesses above |
| `src-union-memberTypes-or-simpleTypes` | D2-002, D2-005 | reviewed: concrete schema/value witnesses above |
| `cvc-facet-valid` | D2-023, D2-024, D2-028, D2-029, D2-030, D2-031, D2-034 | reviewed: concrete schema/value witnesses above |
| `cvc-datatype-valid` | D2-023, D2-004, D2-005 | reviewed: concrete schema/value witnesses above |
| `cos-applicable-facets` | D2-007 | reviewed: concrete schema/value witnesses above |
| `cos-list-of-atomic` | D2-004 | reviewed: concrete schema/value witnesses above |
| `cos-no-circular-unions` | D2-006 | reviewed: concrete schema/value witnesses above |
| `cvc-length-valid` | D2-024 | reviewed: concrete schema/value witnesses above |
| `length-minLength-maxLength` | D2-025, D2-026 | reviewed: concrete schema/value witnesses above |
| `length-valid-restriction` | D2-025 | reviewed: concrete schema/value witnesses above |
| `cvc-minLength-valid` | D2-024 | reviewed: concrete schema/value witnesses above |
| `minLength-less-than-equal-to-maxLength` | D2-027 | reviewed: concrete schema/value witnesses above |
| `minLength-valid-restriction` | D2-027 | reviewed: concrete schema/value witnesses above |
| `cvc-maxLength-valid` | D2-024 | reviewed: concrete schema/value witnesses above |
| `maxLength-valid-restriction` | D2-027 | reviewed: concrete schema/value witnesses above |
| `src-multiple-patterns` | D2-028 | reviewed: concrete schema/value witnesses above |
| `cvc-pattern-valid` | D2-028, D2-035 | reviewed: concrete schema/value witnesses above |
| `src-multiple-enumerations` | D2-029 | reviewed: concrete schema/value witnesses above |
| `cvc-enumeration-valid` | D2-029, D2-013 | reviewed: concrete schema/value witnesses above |
| `enumeration-valid-restriction` | D2-029 | reviewed: concrete schema/value witnesses above |
| `whiteSpace-valid-restriction` | D2-009 | reviewed: concrete schema/value witnesses above |
| `cvc-maxInclusive-valid` | D2-030, D2-031, D2-014 | reviewed: concrete schema/value witnesses above |
| `minInclusive-less-than-equal-to-maxInclusive` | D2-032 | reviewed: concrete schema/value witnesses above |
| `maxInclusive-valid-restriction` | D2-033 | reviewed: concrete schema/value witnesses above |
| `cvc-maxExclusive-valid` | D2-030, D2-031, D2-014 | reviewed: concrete schema/value witnesses above |
| `maxInclusive-maxExclusive` | D2-032 | reviewed: concrete schema/value witnesses above |
| `minExclusive-less-than-equal-to-maxExclusive` | D2-032 | reviewed: concrete schema/value witnesses above |
| `maxExclusive-valid-restriction` | D2-033 | reviewed: concrete schema/value witnesses above |
| `cvc-minExclusive-valid` | D2-030, D2-031, D2-015 | reviewed: concrete schema/value witnesses above |
| `minInclusive-minExclusive` | D2-032 | reviewed: concrete schema/value witnesses above |
| `minExclusive-less-than-maxInclusive` | D2-032 | reviewed: concrete schema/value witnesses above |
| `minExclusive-valid-restriction` | D2-033 | reviewed: concrete schema/value witnesses above |
| `cvc-minInclusive-valid` | D2-030, D2-031, D2-015 | reviewed: concrete schema/value witnesses above |
| `minInclusive-less-than-maxExclusive` | D2-032 | reviewed: concrete schema/value witnesses above |
| `minInclusive-valid-restriction` | D2-033 | reviewed: concrete schema/value witnesses above |
| `cvc-totalDigits-valid` | D2-034 | reviewed: concrete schema/value witnesses above |
| `totalDigits-valid-restriction` | D2-034 | reviewed: concrete schema/value witnesses above |
| `cvc-fractionDigits-valid` | D2-034 | reviewed: concrete schema/value witnesses above |
| `fractionDigits-totalDigits` | D2-034 | reviewed: concrete schema/value witnesses above |
| `fractionDigits-valid-restriction` | D2-034 | reviewed: concrete schema/value witnesses above |

| Inventory branch key | Concrete cases | Status | Disposition / exact source branch |
| --- | --- | --- | --- |
| `datatypes/cvc-facet-valid/branch/001` | D2-023, D2-024, D2-028, D2-029, D2-030, D2-031, D2-034 | reviewed | clause 1 is a dispatcher/composite; concrete specific predicates, not an extra independent rule |
| `datatypes/cvc-datatype-valid/branch/001` | D2-023 | reviewed | lexical parent/composite; pattern and three no-pattern variety alternatives |
| `datatypes/cvc-datatype-valid/branch/002` | D2-023, D2-028 | reviewed | pattern-present branch, with valid integer grammar retained |
| `datatypes/cvc-datatype-valid/branch/003` | D2-023 | reviewed | no-pattern parent/composite; atomic/list/union concrete alternatives |
| `datatypes/cvc-datatype-valid/branch/004` | D2-023 | reviewed | atomic: +12 valid, 12.0 invalid integer lexical form |
| `datatypes/cvc-datatype-valid/branch/005` | D2-004, D2-023 | reviewed | list: empty and 1 2 valid; 1 x invalid |
| `datatypes/cvc-datatype-valid/branch/006` | D2-005, D2-023 | reviewed | union: integer and boolean alternatives valid; large rejected |
| `datatypes/cvc-datatype-valid/branch/007` | D2-023, D2-030, D2-034 | reviewed | value-facet conjunction; valid lexical 11 fails maxInclusive=10 |
| `datatypes/cvc-length-valid/branch/001` | D2-024 | reviewed | atomic parent/composite of three measurement alternatives |
| `datatypes/cvc-length-valid/branch/002` | D2-024 | reviewed | string and anyURI character count: 1/2/3 exact characters |
| `datatypes/cvc-length-valid/branch/003` | D2-024 | reviewed | hexBinary/base64Binary decoded octets: 1/2/3 exact octets |
| `datatypes/cvc-length-valid/branch/004` | D2-024 | reviewed | QName/NOTATION no-op branch; rejecting admitted value is infeasible by source |
| `datatypes/cvc-length-valid/branch/005` | D2-024 | reviewed | list item count: 1/2/3 exact items |
| `datatypes/length-minLength-maxLength/branch/001` | D2-025 | reviewed | minLength coexistence parent/conjunction |
| `datatypes/length-minLength-maxLength/branch/002` | D2-025 | reviewed | minLength <= length: 1<=2 true, 1<=0 false |
| `datatypes/length-minLength-maxLength/branch/003` | D2-025 | reviewed | qualifying ancestor with same minLength and no length; same-step case disproves ancestry |
| `datatypes/length-minLength-maxLength/branch/004` | D2-026 | reviewed | maxLength coexistence parent/conjunction |
| `datatypes/length-minLength-maxLength/branch/005` | D2-026 | reviewed | length <= maxLength: 2<=3 true, 4<=3 false |
| `datatypes/length-minLength-maxLength/branch/006` | D2-026 | reviewed | qualifying ancestor with same maxLength and no length; same-step case disproves ancestry |
| `datatypes/cvc-minLength-valid/branch/001` | D2-024 | reviewed | atomic parent/composite of three measurement alternatives |
| `datatypes/cvc-minLength-valid/branch/002` | D2-024 | reviewed | string and anyURI character count: 1/2/3 exact characters |
| `datatypes/cvc-minLength-valid/branch/003` | D2-024 | reviewed | hexBinary/base64Binary decoded octets: 1/2/3 exact octets |
| `datatypes/cvc-minLength-valid/branch/004` | D2-024 | reviewed | QName/NOTATION no-op branch; rejecting admitted value is infeasible by source |
| `datatypes/cvc-minLength-valid/branch/005` | D2-024 | reviewed | list item count: 1/2/3 exact items |
| `datatypes/cvc-maxLength-valid/branch/001` | D2-024 | reviewed | atomic parent/composite of three measurement alternatives |
| `datatypes/cvc-maxLength-valid/branch/002` | D2-024 | reviewed | string and anyURI character count: 1/2/3 exact characters |
| `datatypes/cvc-maxLength-valid/branch/003` | D2-024 | reviewed | hexBinary/base64Binary decoded octets: 1/2/3 exact octets |
| `datatypes/cvc-maxLength-valid/branch/004` | D2-024 | reviewed | QName/NOTATION no-op branch; rejecting admitted value is infeasible by source |
| `datatypes/cvc-maxLength-valid/branch/005` | D2-024 | reviewed | list item count: 1/2/3 exact items |
| `datatypes/cvc-pattern-valid/branch/001` | D2-028, D2-035 | reviewed | literal in regex language: a/b local OR, inherited AND, whole-string counterexamples |
| `datatypes/whiteSpace-valid-restriction/branch/001` | D2-009 | reviewed | collapse to replace/preserve forbidden |
| `datatypes/whiteSpace-valid-restriction/branch/002` | D2-009 | reviewed | replace to preserve forbidden |
| `datatypes/cvc-maxInclusive-valid/branch/001` | D2-030 | reviewed | numeric=true: decimal candidates 9,10,11 |
| `datatypes/cvc-maxInclusive-valid/branch/002` | D2-031, D2-015 | reviewed | numeric=false: dateTime before/equal/after/incomparable; duration opposing cases |
| `datatypes/maxInclusive-valid-restriction/branch/001` | D2-033 | reviewed | matrix row for maxInclusive source clause 1; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/maxInclusive-valid-restriction/branch/002` | D2-033 | reviewed | matrix row for maxInclusive source clause 2; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/maxInclusive-valid-restriction/branch/003` | D2-033 | reviewed | matrix row for maxInclusive source clause 3; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/maxInclusive-valid-restriction/branch/004` | D2-033 | reviewed | matrix row for maxInclusive source clause 4; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/cvc-maxExclusive-valid/branch/001` | D2-030 | reviewed | numeric=true: decimal candidates 9,10,11 |
| `datatypes/cvc-maxExclusive-valid/branch/002` | D2-031, D2-015 | reviewed | numeric=false: dateTime before/equal/after/incomparable; duration opposing cases |
| `datatypes/maxExclusive-valid-restriction/branch/001` | D2-033 | reviewed | matrix row for maxExclusive source clause 1; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/maxExclusive-valid-restriction/branch/002` | D2-033 | reviewed | matrix row for maxExclusive source clause 2; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/maxExclusive-valid-restriction/branch/003` | D2-033 | reviewed | matrix row for maxExclusive source clause 3; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/maxExclusive-valid-restriction/branch/004` | D2-033 | reviewed | matrix row for maxExclusive source clause 4; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/cvc-minExclusive-valid/branch/001` | D2-030 | reviewed | numeric=true: decimal candidates 9,10,11 |
| `datatypes/cvc-minExclusive-valid/branch/002` | D2-031, D2-015 | reviewed | numeric=false: dateTime before/equal/after/incomparable; duration opposing cases |
| `datatypes/minExclusive-valid-restriction/branch/001` | D2-033 | reviewed | matrix row for minExclusive source clause 1; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/minExclusive-valid-restriction/branch/002` | D2-033 | reviewed | matrix row for minExclusive source clause 2; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/minExclusive-valid-restriction/branch/003` | D2-033 | reviewed | matrix row for minExclusive source clause 3; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/minExclusive-valid-restriction/branch/004` | D2-033 | reviewed | matrix row for minExclusive source clause 4; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/cvc-minInclusive-valid/branch/001` | D2-030 | reviewed | numeric=true: decimal candidates 9,10,11 |
| `datatypes/cvc-minInclusive-valid/branch/002` | D2-031, D2-015 | reviewed | numeric=false: dateTime before/equal/after/incomparable; duration opposing cases |
| `datatypes/minInclusive-valid-restriction/branch/001` | D2-033 | reviewed | matrix row for minInclusive source clause 1; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/minInclusive-valid-restriction/branch/002` | D2-033 | reviewed | matrix row for minInclusive source clause 2; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/minInclusive-valid-restriction/branch/003` | D2-033 | reviewed | matrix row for minInclusive source clause 3; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/minInclusive-valid-restriction/branch/004` | D2-033 | reviewed | matrix row for minInclusive source clause 4; n=9/10/11; clause and total-schema outcomes separated for necessary co-failures |
| `datatypes/cvc-totalDigits-valid/branch/001` | D2-034 | reviewed | 99/9.9/.99 and zero-padded alias valid; 100/.001 invalid under t=2 |
| `datatypes/cvc-fractionDigits-valid/branch/001` | D2-034 | reviewed | 1.23/1.2300 valid and 1.231 invalid under f=2 |

## Actual limits of this branch audit

Every current named/numbered Part 2 constraint and every current regex
production has an explicit concrete witness or an explained composite/no-op
branch. The builtin table includes all 44 builtins plus anySimpleType separately;
all twelve facets, all ten fixable facet kinds, and all sixteen inherited-bound
comparison clauses have concrete sketches. Source questions are listed in
D2-048, not hidden as completed pass/fail oracles.

This finite audit does not inline every unbounded string/value/recursion depth.
It also does not claim a complete concrete product of all facet subsets,
union/member/facet/context combinations, all Unicode-version changes, or every
externally referenced RFC/XML rule. Those remain the parameterized obligations
of the family catalogue and dependency boundary references. No corpus fixture,
implementation conformance result, or demonstrated gap follows merely from
these descriptions. Appendix metadata has informative dispositions; operational
limits and absent observation seams remain distinct from semantic invalidity.
