# XSD 1.0 Part 2: specification-derived scenario catalogue

This catalogue describes the decision space of **XML Schema Part 2: Datatypes,
Second Edition, 28 October 2004**, independently of repository code and test
corpora. The [published Recommendation](../upstream/datatypes.html) is the
reference; [its XML source](../xml/datatypes.xml) contains historical
`diff="del"` text that does not contribute current requirements. The published
HTML also supplies generated tables and appendix material absent from XML
placeholders. The Second Edition incorporates the first-edition corrections.
Later corrections and source provenance belong to the [source register](../README.md).

Part 2 has **19 primitive types, 25 built-in derived types, and the special
`anySimpleType` definition**. Its appendices run **A–I**, not A–L. `anyType` is
owned by Part 1. This document does not claim that a finite example set enumerates
all strings, all values, all derivation depths, or every possible schema.

## How to expand the catalogue

A family is an obligation to generate cases, not a claim of executable coverage.
For every family preserve its ID, source fragment, selected decisions, input,
expected result, and observation boundary when creating individual scenarios.
Use the following finite partitions, retaining the parameter that generates the
unbounded family:

1. For each grammar alternative: one shortest accepted form, an ordinary form,
   an empty/omitted form where permitted, and one mutation violating only that
   production. For repeated productions use 0, 1, 2, and a parameterized larger
   count; do not assume a processor-specific upper bound is a language rule.
2. For each boundary `b`: below, equal, above; for a partial order add
   incomparable. For an interval cross lower/upper inclusivity and each endpoint
   relationship. For a fixed value cross omitted, equal, and changed.
3. For a conjunction isolate each failing operand with all others satisfied;
   include all satisfied and selected simultaneous failures. For an alternative
   cover each accepting branch, overlap, and none. Preserve ordered alternatives.
4. For each applicable datatype/facet pair use both the primitive and at least
   one restriction descendant. Cross atomic/list/union where their rules differ,
   direct/inherited facets, named/anonymous definitions, and schema/instance
   lexical contexts. Cross independent axes only when the expected result remains
   defined; exclude impossible combinations explicitly.
5. Separate XML processing from datatype processing. Record the character
   sequence after XML parsing and before datatype whitespace handling. Exercise
   literal text, character references, entity expansion, attribute normalization,
   and CDATA where those produce different sequences. XML-invalid documents do
   not prove a datatype rejection.

Observation codes used below:

| Code | Observable contract |
| --- | --- |
| **S** | Schema XML representation accepted/rejected, or schema component construction valid/invalid. Keep these distinct. |
| **V** | Instance literal accepted/rejected against the constructed type. |
| **E** | Typed equality/order, observed directly where exposed, otherwise through enumeration, bounds, fixed values, or identity constraints with Part 1. |
| **P** | Required component, selected member type, normalized value, or other PSVI fact; where the API does not expose it, record an observation gap. |
| **K** | Canonical representation, only when serialization is exposed. Acceptance of a noncanonical valid spelling remains a separate V obligation. |
| **I** | Informative explanation, recommendation, optional support, or source question; do not score as a mandatory rejection. |

“Reject schema” means violation of an applicable representation/component rule;
“reject value” assumes a valid schema. No family requires a particular diagnostic
string, evaluation strategy, internal representation, or failure ordering unless
the cited specification says so.

## Type system, construction, and conformance

### D-001 — Scope, names, characters, and version boundary

Source: [§§1–3.1](../upstream/datatypes.html#Intro),
[namespace considerations](../upstream/datatypes.html#namespaces).

Generate built-in references with any prefix bound to
`http://www.w3.org/2001/XMLSchema`; prefix spelling does not change type identity.
An unbound prefix or the same local name in a different namespace does not name
that built-in. The optional datatype-only schema uses a different namespace;
do not silently equate it with the schema namespace. Distinguish schema
vocabulary names, type references, and values of type QName.

Cross empty strings, ASCII, non-ASCII XML characters, supplementary characters,
combining sequences, and XML-forbidden characters. Datatype string values use
XML characters, not bytes or UTF-16 code units. Reject forbidden characters at
the XML boundary. Do not import XSD 1.1 types or rules, such as `dateTimeStamp`,
`yearMonthDuration`, `dayTimeDuration`, assertions, explicitTimezone, or year
zero, into the XSD 1.0 inventory. **S/V/E**.

### D-002 — Lexical space, value space, and canonical space

Source: [§§2.1–2.3.1](../upstream/datatypes.html#datatype),
[terminology](../upstream/datatypes.html#terminology).

For every type with aliases construct `(a,b)` denoting the same value and a
third spelling denoting a different value. Both valid aliases must validate;
a lexical pattern can distinguish aliases while an enumeration compares their
values. Examples: decimal `2.0`/`02.00`, boolean `true`/`1`, hexadecimal
`aF`/`AF`, UTC offset aliases, and whitespace aliases of lists. A canonical
spelling is a representative, not the entire lexical space. Apply canonical
requirements only to types for which Part 2 specifies them. Test empty strings
separately from absent content and absent attributes, whose assessment/defaults
belong to Part 1. String matching is exact: case differences and precomposed
versus decomposed Unicode sequences remain distinct. **V/E/K**.

### D-003 — Equality, disjoint primitives, and ordered comparisons

Source: [§4.2.1](../upstream/datatypes.html#equal),
[§4.2.2](../upstream/datatypes.html#rf-ordered).

Generate reflexive, symmetric, and transitive equality cases; distinct
lexical aliases are equal when their values are identical. Restriction types
share their atomic ancestor's values: integer `1` and decimal `1.0` can be equal.
Different primitive value spaces are disjoint: decimal `1`, float `1`, double
`1`, boolean `true`, and string `1` are not interchangeable values. Equal octets
in hexBinary and base64Binary remain in different primitive spaces. A union
contains its members' values without merging different primitive spaces.

For any pair of values, equality and inequality are exhaustive and mutually
exclusive. Equal values are indistinguishable as values; lexical spellings and
declared-type/member provenance remain separate observations. Cross equal
aliases and unequal values with each equality law; do not substitute a host
operator or lexical comparison for the datatype relation.

For every ordered primitive cover `<`, `=`, `>`, and, where defined,
incomparable; unordered strings/lists must not acquire a language-defined order
from host-language comparison. Cross derived siblings with common ancestors,
overlapping and disjoint restrictions, and unions whose selected members have
same/different primitive ancestors. NaN is covered by D-024. **E/P**.

For a source-defined strict partial order, require irreflexivity (`a < a` is
false), asymmetry (`a < b` excludes `b < a`), and transitivity (`a < b` and
`b < c` imply `a < c`). A total order additionally gives exactly one of
`a < b`, `b < a`, or `a = b`. Incomparable means unequal with neither strict
direction; values from different primitive spaces are incomparable. Use
admitted pairs/triples with each premise satisfied and independently absent,
including equality and a genuinely incomparable partial-order pair. These
are value-relation contracts, not added schema-rejection rules or an ordering
requirement for an unordered datatype. **E**.

### D-004 — Component identity and the simple ur-type

Source: [§4.1.1](../upstream/datatypes.html#dc-defn),
[§4.1.6](../upstream/datatypes.html#anySimpleType-component),
[§2.5.2](../upstream/datatypes.html#primitive-vs-derived),
[Structures §2.2.1.2](../upstream/structures.html#Simple_Type_Definition).

Named types are identified by name plus target namespace; same local names in
different namespaces are distinct, duplicate named definitions in the same
namespace violate uniqueness, and anonymous types do not acquire identity from
invented names. Cross absent namespace/present namespace and global/local
construction. Verify atomic primitive/item/member properties appropriate to the
variety, including the non-empty ordered member sequence for union.

`anySimpleType` has an unconstrained lexical space and the primitive values
plus lists of primitive values in its conceptual value space. Its base is the
ur-type; its facets/final set are empty and its variety is absent. It is not an
ordinary user-defined union or atomic primitive. Observe unrestricted character
content where used and keep restrictions allowed by Part 1 separate from
invented facets on the absent variety. **S/V/P**.

Structures explicitly leaves its lexical-to-value mapping unspecified and
does not constrain processor behavior where that mapping is implicated,
including enumeration, construction of default attributes/elements, and
identity-constraint checks involving such items. Partition mapping implicated
versus unimplicated, the actual admitted carrier, and available versus
unavailable value observations. Do not choose a string mapping or unique
typed/default/identity oracle from lexical acceptance alone. Specified lexical
and whitespace behavior and constraints independent of that map still apply;
the qualification does not authorize an otherwise illegal direct atomic
restriction of anySimpleType or waive ordinary constrained-type rules. **S/V/P/E**.

### D-005 — Restriction, list, union, and finality

Source: [§4.1.1–4.1.2](../upstream/datatypes.html#dc-defn),
[restriction](../upstream/datatypes.html#derivation-by-restriction),
[list](../upstream/datatypes.html#derivation-by-list),
[union](../upstream/datatypes.html#derivation-by-union).

Restriction preserves variety and narrows lexical/value spaces through
applicable facets. List and union construction use `anySimpleType` as base;
item and member dependencies are not restriction-base edges. Cover primitive,
built-in-derived, and user-derived starting types; zero-facet restriction;
multi-step inheritance; and cycles through each kind of dependency with the
Part 1 derivation rules.

For each of `restriction`, `list`, `union`, cross its presence/absence in the
relevant source type's final set and whether that derivation is attempted.
`#all` blocks all three; an empty set blocks none. Explicit `final` overrides
schema `finalDefault`; absent values inherit its relevant tokens or the empty
set. Legal `extension` in `finalDefault` does not contribute simple-type
finality. `substitution` is outside that schema-root attribute's grammar and
must not be treated as an admitted token to filter out.
Use one, several, reordered, and whitespace-separated tokens; invalid tokens
or mixing `#all` with other tokens violate the representation. Test prohibited
derivation through nested/list-member/union-member paths, not only a direct
restriction. **S/P**.

### D-006 — XML simpleType envelope

Source: [§4.1.2](../upstream/datatypes.html#xr-defn),
[Appendix A](../upstream/datatypes.html#schema).

A simpleType contains optional leading annotation and exactly one restriction,
list, or union. Cover missing constructor, two constructors, reordered or
repeated annotation, extra children, and non-whitespace character content.
A top-level simpleType requires an NCName name; a local simpleType prohibits
name and final. Test absent, empty, malformed, and valid names/final values.
Attribute `id` follows XML ID syntax/uniqueness. Cross allowed foreign
attributes with forbidden unqualified/XSD attributes according to the
schema-for-schemas wildcard, including known foreign attributes that fail lax
assessment. Annotation is optional and must not change the datatype's value
semantics. **S/P**.

### D-007 — Constructor source alternatives and reference resolution

Source: [§4.1.3](../upstream/datatypes.html#defn-rep-constr),
[§4.1.2](../upstream/datatypes.html#xr-defn).

For restriction cross base attribute present/absent with one inline simpleType
present/absent: exactly one is required. Apply the same four combinations to
list's itemType. More than one inline type violates their grammar. A union
allows memberTypes, inline simpleTypes, or both, but its effective sequence
must not be empty; an empty or whitespace-only memberTypes is sufficient only
if inline members exist. A one-member union is permitted by §4.1 despite the
informal “greater than 1” phrase in §2.5.1.3; retain that wording conflict in
D-072 rather than inventing a two-member constraint.

For every QName type reference test bound/unbound prefixes, default namespace,
unknown types, complex rather than simple types, malformed QName, and a
forward reference resolved by the complete schema. Reference syntax does not
substitute for successful component resolution. **S/P**.

### D-008 — List item-type legality

Source: [§2.5.1.2](../upstream/datatypes.html#list-datatypes),
[§4.1.5](../upstream/datatypes.html#cos-list-of-atomic),
[Part 1 simple-type properties](../upstream/structures.html#st-props-correct).

Accept atomic item types and unions whose effective members are atomic. Reject
a list as item type and a union item type that includes a list member, including
through nested unions/restrictions. Cover union flattening and an item type with
its own pattern/enumeration/range facets; each list item must satisfy that type.
Final-list prohibition belongs to the item type. An atomic string/URI item type
can contain spaces in isolation, but list tokenization never groups spaces into
one item by quoting or escaping. **S/V/P**.

### D-009 — List tokenization and empty values

Source: [§2.5.1.2](../upstream/datatypes.html#list-datatypes),
[whiteSpace](../upstream/datatypes.html#rf-whiteSpace).

For lists cross empty input, all XML whitespace, one token, two tokens, and many
tokens with leading/trailing/repeated space, tab, LF, and CR. Normalization is
fixed collapse: generic lists admit an empty sequence unless a facet excludes
it. NMTOKENS, IDREFS, and ENTITIES have minLength 1. Nonbreaking space and other
Unicode separators are not XML whitespace delimiters. A list of strings
`a b` has two items, never one; quotes/backslash do not provide a list quoting
syntax. Isolate a malformed first/middle/last item with all others valid.
**V/E/P**.

### D-010 — List facets, equality, and canonical form

Source: [§2.5.1.2](../upstream/datatypes.html#list-datatypes),
[§4.1.2.2](../upstream/datatypes.html#derivation-by-list).

List length measures items, not lexical characters. A list-level pattern sees
the complete normalized list literal; an item-type pattern sees each item.
For instance pattern `123 (\d+\s)*456` admits `123 456` and
`123 987 456`; it is not applied independently to `123` and `456`.
Cross a valid list literal failing only the list pattern with one failing only
an item facet. List enumeration compares sequences: same length and equal
corresponding item values, with order and duplicates significant. Thus decimal
lists `1 2` and `01.0 2.00` are equal; `2 1` and `1 1 2` differ.
Canonical list items use their item type's canonical forms and canonical
space separation. Do not impose scalar numeric facets directly on a list.
**S/V/E/K**.

### D-011 — Union assembly and member order

Source: [§2.5.1.3](../upstream/datatypes.html#union-datatypes),
[§4.1.2.3](../upstream/datatypes.html#derivation-by-union),
[union cycles](../upstream/datatypes.html#cos-no-circular-unions).

The member sequence is memberTypes attribute entries in order, then inline
simpleTypes in order; nested union members are replaced by their member
sequences. Cover attribute-only, inline-only, mixed, nested, duplicate members,
and self/indirect cycles. Direct/indirect circular unions are invalid, not
infinite runtime searches. With integer then string, `1` chooses integer;
with string then integer, it chooses string. With atomic then list, an overlap
can produce different value kinds. Failure of an earlier member allows a later
member to accept. Crossing definition order with overlapping lexical spaces
must preserve the selected value/member semantics. **S/V/E/P**.

### D-012 — Union assessment, member whitespace, and restriction

Source: [union](../upstream/datatypes.html#union-datatypes),
[whiteSpace](../upstream/datatypes.html#rf-whiteSpace),
[datatype-valid](../upstream/datatypes.html#cvc-datatype-valid).

Cross no member match, exactly one match, multiple matches; members with
preserve/replace/collapse; atomic/list members; member restrictions; union
pattern/enumeration; inherited union restrictions. Union has no directly
applicable whiteSpace facet: successful member whitespace handling matters.
A string-first union can retain whitespace that an integer-first union
collapses. Restrictions still require union-level facets in addition to valid
member assessment; do not treat any lexical member match alone as success.
Use Part 1 xsi:type scenarios to select a legal member/derived type explicitly
and observe changes to selected type and value. Canonical union forms use the
canonical representations of the appropriate member types. Where members
sharing an atomic ancestor have equal values but different canonical grammars,
retain [D-072's union-canonical qualification](#d-072--questions-and-permissible-choices-that-must-remain-visible)
before inferring K from the selected member. No independent universal numeric
canonical form is prescribed. **S/V/E/P/K**.

### D-013 — Fundamental-facet derivation

Source: [§4.2](../upstream/datatypes.html#rf-fund-facets),
[Appendix C](../upstream/datatypes.html#app-fundamental-facets).

Generate each branch of these component rules, without replacing the published
classification with an intuitive mathematical calculation:

| Property | Atomic | List | Union |
| --- | --- | --- | --- |
| ordered | Inherit base classification | false | Common ancestor other than anySimpleType: ancestor's classification; all members false: false; otherwise partial |
| bounded | Both lower and upper bound facets: true; otherwise false, subject to primitive definitions | length, or both minLength and maxLength: true; otherwise false | Every member bounded and common ancestor: true; otherwise false |
| cardinality | Finite base stays finite; infinite base becomes finite with length/maxLength/totalDigits, or both bounds plus fractionDigits or a date/g* base; otherwise countably infinite | length, or both minLength and maxLength: finite; otherwise countably infinite | All members finite: finite; otherwise countably infinite |
| numeric | Inherit base | false | All members numeric: true; otherwise false |

The union `ordered` condition explicitly excludes anySimpleType as the common
ancestor; the printed `bounded` condition does not. Preserve FM13's intended
ancestor-scope qualification for all-bounded members from different primitive
branches. Do not insert an unprinted common-primitive or non-ur-type condition.

Keep the abstract [bound definitions](../upstream/datatypes.html#rf-bounded)
separate from these component calculations. For a restricted value space
`V` contained in an ordered space `U`, a bound itself belongs to `U` and
must satisfy its relation for **every** `v` in `V`:

| Bound | Required relation for all v in V |
| --- | --- |
| Inclusive upper u | u ≥ v |
| Exclusive upper u | u > v |
| Inclusive lower l | l ≤ v |
| Exclusive lower l | l < v |

Abstract boundedness requires an upper bound of either kind and a lower bound
of either kind. Partition the containing ordered space, subset premise,
candidate bound in/out of that space, included/excluded endpoint, and a member
satisfying/violating/incomparable with the proposed bound. One satisfying
sample cannot prove the quantified condition. A bound need not belong to `V`;
these definitions supply no blanket rejection of an empty restriction and do
not replace the published classification formulas. **E/P**.

For each union rule isolate one member defeating the “all” condition and examine
the applicable common-ancestor wording. For atomic cardinality distinguish bounded decimal
without fractionDigits (infinite) from fixed-scale bounded decimal (finite),
and bounded date from bounded dateTime (arbitrary fractional seconds).
Do not infer a total ordered component merely because a pattern happens to
remove incomparable values from a partial-order primitive. Appendix C's
published built-in classifications, including bounded float/double, are direct
reference cases. Record source tensions in D-072. **S/P/I**.

### D-014 — Conformance levels and permitted precision limits

Source: [§5](../upstream/datatypes.html#conformance),
[decimal](../upstream/datatypes.html#decimal),
[duration precision](../upstream/datatypes.html#year-sec-conformance).

Minimal conformance implements all component constraints and validation rules.
XML schema representation conformance additionally implements representation
constraints and the specified component mapping. A non-XML schema construction
interface need not claim XML representation conformance. Test those claims
separately; a passing lexical parser is insufficient evidence for either level.

Required minima are 18 decimal digits, four year digits, and millisecond
fractional-second precision where the specification's shared note applies.
Application-defined maxima are permitted only with clearly documented maxima.
Cross below/at/above each minimum and each documented maximum; unsupported
precision beyond a permitted documented maximum is not the same claim as
invalid XSD syntax. Large signed year values, duration components, and decimal
scale need independent cases; the minimum is not a license to truncate or round
accepted exact decimal/temporal values silently. Regex Unicode version support
has its own minimum/future-version distinction in D-063–D-064. **S/V/E/I**.

## Every built-in datatype

### D-015 — string

Source: [§3.2.1](../upstream/datatypes.html#string).

Accept empty and arbitrary finite XML character sequences, retaining whitespace
and case. Reject markup children as simple content through Part 1 rather than
calling them string characters. Cross one supplementary character with a
multi-code-point grapheme: length is one and multiple characters respectively,
not UTF-8 bytes, UTF-16 units, or user-perceived glyphs. `é` and `e` plus combining
acute are different values and may have different lengths. Exercise lexical
patterns and character-length facets on these cases. **V/E**.

### D-016 — normalizedString and token

Source: [normalizedString](../upstream/datatypes.html#normalizedString),
[token](../upstream/datatypes.html#token).

normalizedString derives from string with replace; token derives from
normalizedString with collapse. Input ` a\t b\n ` maps to ` a  b  ` under
replace and `a b` under collapse (backslash notation here denotes the actual
characters). Replace does not trim or collapse existing spaces. Token can
contain single internal spaces; it is not restricted to one word. Empty and
all-whitespace values normalize to empty for token and remain valid absent
other facets. Non-XML Unicode whitespace remains unchanged. Cross restrictions
that tighten whitespace with those attempting to loosen it. **S/V/E/P**.

### D-017 — language

Source: [§3.3.3](../upstream/datatypes.html#language),
[Appendix A definition](../upstream/datatypes.html#schema).

Derives from token with lexical pattern
`[a-zA-Z]{1,8}(-[a-zA-Z0-9]{1,8})*`. Accept `en`, `EN`, `en-US`, and syntactically
permitted extension/private-use forms; reject empty, numeric initial subtag,
underscore, leading/trailing hyphen, empty subtag, non-ASCII letters, and
subtags of 9 characters. Cross 1/8/9-character subtags and zero/one/multiple
hyphens. Collapse applies before the pattern; internal spaces remain invalid.
Values are strings, so `en` and `EN` are distinct for enumeration even when a
language application would compare tags without case. Do not replace this
rule with a modern BCP 47 registry or network lookup. Record any stronger
RFC 3066 semantic question separately. **V/E/I**.

### D-018 — NMTOKEN, Name, and NCName

Source: [NMTOKEN](../upstream/datatypes.html#NMTOKEN),
[Name](../upstream/datatypes.html#Name),
[NCName](../upstream/datatypes.html#NCName),
[XML and namespace references](../upstream/datatypes.html#normative-biblio).

NMTOKEN permits one or more XML NameChar characters; Name additionally constrains
the initial character; NCName excludes colons throughout. NMTOKEN and Name each derive from token; NCName derives from Name. Cross empty, letter, underscore, digit initial,
colon initial/internal/repeated, hyphen/dot initial/internal, combining character
initial/internal, and each admitted/excluded Unicode name range endpoint.
`1` is NMTOKEN but not Name; `a:b` is Name but not NCName; `_a` is NCName.
None of these values performs namespace expansion merely because it contains
a colon. Whitespace is collapsed before assessment; internal spaces fail all
three. Attribute-only use for NMTOKEN is a compatibility recommendation,
not an unconditional rejection of element content. **V/E/I**.

### D-019 — ID, IDREF, and IDREFS

Source: [ID](../upstream/datatypes.html#ID),
[IDREF](../upstream/datatypes.html#IDREF),
[IDREFS](../upstream/datatypes.html#IDREFS).

ID and IDREF restrict NCName; IDREFS is a list of IDREF with minLength 1.
Lexical cases include valid NCName, colon/digit-start/space failure, surrounding
whitespace collapse, and empty list failure. Distinguish these datatype checks
from Part 1's validation-context ID/IDREF bindings. Cross duplicate/no-duplicate
owners, forward/backward/missing references, multiple IDREFS tokens, and full
document versus validation-subtree roots. Preserve the direct-root
[IBS-Q001](source-questions.md#ibs-q001--root-id-eligibility-and-successful-declaration-validation)
success/contribution question. Separate assessment episodes do not acquire a
complete session-state contract from the same-context predicate.
Recommendation to use only on attributes does not itself prohibit element
values. Union/list-derived identity behavior requires the relevant Part 1 rule,
not inference from storage strings. **V/E/P/I**, with Part 1 for identity state.

### D-020 — ENTITY, ENTITIES, and NMTOKENS

Source: [ENTITY](../upstream/datatypes.html#ENTITY),
[ENTITIES](../upstream/datatypes.html#ENTITIES),
[NMTOKENS](../upstream/datatypes.html#NMTOKENS),
[String Valid](../upstream/structures.html#cvc-simple-type),
[required infoset properties](../upstream/structures.html#infoset).

ENTITY restricts NCName but its values must identify unparsed entities declared
in that instance document's DTD. Cross undeclared/parsed/unparsed entity,
matching/different spelling, and declarations in separate documents. ENTITIES
is a non-empty list of ENTITY; one invalid item fails the value. NMTOKENS is a
non-empty list of NMTOKEN; it admits tokens such as `1` and `a:b` that IDREFS
and ENTITIES cannot admit as NCNames. Test all three lists with 0/1/2 items,
XML whitespace normalization, duplicates, and inherited minLength 1. Empty
ordinary user lists versus these non-empty built-ins is a required opposing
case. Distinguish a processor's external-DTD policy from the ENTITY value-space
contract; lack of declarations cannot prove that NCName alone implements ENTITY.
Attribute-only use is a compatibility recommendation. **S/V/E/I**.

### D-021 — boolean

Source: [§3.2.2](../upstream/datatypes.html#boolean).

Exactly `true`, `false`, `1`, `0` after fixed collapse are valid; `True`, `FALSE`,
`+1`, `00`, empty, and internal whitespace are invalid. `true` equals `1` and
`false` equals `0`; canonical forms are `true` and `false`. A pattern may select
only a spelling, e.g. `true|false`. **Enumeration is not an applicable boolean
facet in XSD 1.0**; direct boolean enumeration is an invalid schema, even though
boolean values have equality. Bounds, length, and digits are also inapplicable.
Cross these schema failures with valid pattern restrictions. **S/V/E/K**.

### D-022 — decimal

Source: [§3.2.3](../upstream/datatypes.html#decimal).

Use ASCII decimal digits with optional leading `+`/`-`, optional decimal point
when the fractional part is absent, and at least one digit overall. Accept
`0`, `+0`, `-0`, `001`, `.5`, `5.`, `+001.2300`; reject `.`, sign alone,
exponent notation, commas/grouping, Unicode digits, NaN/INF, and internal space.
Fixed collapse permits surrounding XML whitespace. Values are exact decimal
numbers; precision/scale in a spelling does not create distinct values:
`2.0 = 2.00`, `-0 = 0`. Large integers and tiny fractions must not silently
become IEEE approximations. Cross 18-digit required support with documented
maxima. Canonical forms prohibit `+`, require a point and at least one digit
each side, and remove redundant leading/trailing zeros. The conventional zero
output `0.0` retains [D-072's zero-sign source qualification](#d-072--questions-and-permissible-choices-that-must-remain-visible)
before scoring `0.0` as the uniquely prescribed zero output. Digits facets
constrain value representations, not raw character counts.
**V/E/K**.

### D-023 — float and double lexical mapping and rounding

Source: [float](../upstream/datatypes.html#float),
[double](../upstream/datatypes.html#double).

For each type cross decimal mantissa with absent/`E`/`e` exponent, exponent
sign, and leading zeros. Accept `12`, `.5`, `5.`, `-0`, `1e+02`, `INF`,
`-INF`, `NaN`; reject `+INF`, `inf`, `nan`, exponent without digits,
hexadecimal host-language floats, internal whitespace, and Unicode digits.
Map finite decimal literals to the closest value in the specified binary value
space, choosing the even result on a tie. Generate exact representable values,
values just below/at/above adjacent-value midpoints, mantissa carry, smallest
positive values, normal/subnormal transitions, zero underflow, largest finite
values, and overflow. Exercise both signs, exponents with many digits, and
float/double differing results. Use exact arithmetic/reference definitions for
expected results rather than the implementation under comparison. Preserve the
published double value-space wording question in D-072. **V/E**.

### D-024 — float/double identity, NaN, zero, and bounds

Source: [float](../upstream/datatypes.html#float),
[double](../upstream/datatypes.html#double).

Each datatype has **one zero and one NaN** for schema value identity: `-0 = +0`
and `NaN = NaN`. NaN is incomparable with every other value; infinities bound
all finite values in their natural order. Cross enumeration of zero/NaN/finite
values, aliases that round to the same binary value, and primitive float versus
double (disjoint despite equal-looking numbers).

For each of four bounds use NaN and an ordinary finite/infinite endpoint.
An inclusive NaN bound leaves only NaN; an exclusive NaN bound leaves an empty
space. Any non-NaN bound excludes NaN. Cross `-INF`/`INF` inclusive/exclusive
bounds with finite extremes; e.g. `maxExclusive=INF` excludes positive infinity
as well as NaN. A union can add a NaN-only space back. Do not use IEEE
computational `NaN != NaN` as the schema equality oracle. **S/V/E**.

### D-025 — float/double canonical representations

Source: [float canonical](../upstream/datatypes.html#float-canonical-representation),
[double canonical](../upstream/datatypes.html#double-canonical-representation).

Canonical finite forms use `E`, explicit exponent with no leading `+` or
redundant zeros, a decimal point, one nonzero digit to its left for nonzero
values, and at least one digit to its right with no redundant fractional zeros.
Zero is `0.0E0` regardless of lexical sign. Preserve `INF`, `-INF`, `NaN`.
Cross already-canonical forms, valid aliases, positive/negative exponents,
powers of ten, and rounding-induced changes in mantissa/exponent. Canonical
output is about the mapped binary value, not a textual rewrite of the input
mantissa; the serialization observation must expose enough precision to
identify that value under the normative mapping. **K/E**.

### D-026 — integer lexical and canonical forms

Source: [§3.3.13](../upstream/datatypes.html#integer),
[Appendix A](../upstream/datatypes.html#schema).

integer restricts decimal with fractionDigits fixed 0 and an integer lexical
pattern. Accept `0`, `-0`, `+0`, `001`, `-001`; reject `1.0`, `1.`, `.0`,
`1E0`, sign alone, Unicode digits, and internal whitespace. A decimal value may
be mathematically integral without its spelling being legal for integer.
Canonical integer forms omit `+` and leading zeros. The conventional zero
output `0` retains [D-072's zero-sign source qualification](#d-072--questions-and-permissible-choices-that-must-remain-visible).
Cover very large positive/negative integers and
inherited fractionDigits fixed 0. Each derived integer type adds its bounds
under Appendix A's inherited lexical grammar; preserve the unsigned prose
qualification in D-027. **S/V/E/K**.

### D-027 — Every integer-derived range and derivation edge

Source: [§§3.3.14–3.3.25](../upstream/datatypes.html#nonPositiveInteger),
[Appendix A](../upstream/datatypes.html#schema).

For every finite endpoint generate endpoint−1, endpoint, endpoint+1 in canonical
and noncanonical integer spellings; for one-sided ranges include several values
far along the unbounded direction. `-0` denotes zero and is accepted exactly
where zero is in the range. Every row also runs D-026.

| Type | Immediate base | Inclusive value range |
| --- | --- | --- |
| nonPositiveInteger | integer | ≤ 0 |
| negativeInteger | nonPositiveInteger | ≤ −1 |
| long | integer | −9223372036854775808 … 9223372036854775807 |
| int | long | −2147483648 … 2147483647 |
| short | int | −32768 … 32767 |
| byte | short | −128 … 127 |
| nonNegativeInteger | integer | ≥ 0 |
| unsignedLong | nonNegativeInteger | 0 … 18446744073709551615 |
| unsignedInt | unsignedLong | 0 … 4294967295 |
| unsignedShort | unsignedInt | 0 … 65535 |
| unsignedByte | unsignedShort | 0 … 255 |
| positiveInteger | nonNegativeInteger | ≥ 1 |

The type names do not select host integer widths for unbounded types.
Under Appendix A's complete definitions, `unsignedByte` retains the signed
integer pattern and accepts a leading plus and `-0`; sign syntax alone does
not determine range validity. Chapter 3's unsigned lexical paragraphs describe
digits without that optional sign. Preserve [D-072's source-integration qualification](#d-072--questions-and-permissible-choices-that-must-remain-visible)
with these Appendix A-based acceptance statements; do not invent a contrary
rejection from the type name. Cross inherited and newly tightened bounds,
fixed fractionDigits, enumeration, and totalDigits. **S/V/E/K**.

### D-028 — duration lexical grammar

Source: [§3.2.6.1](../upstream/datatypes.html#duration-lexical-repr),
[Appendix D](../upstream/datatypes.html#isoformats).

Generate optional leading minus, mandatory `P`, ordered optional Y/M/D,
optional `T` followed by ordered H/M/S, with at least one component overall.
`T` is present iff a time component is present. Components use unsigned ASCII
integers except seconds, which allow a fractional part with digits on both
sides of the point. Zero-valued components may appear or be omitted while at
least one remains. Accept `P0D`, `PT0S`, `P1Y2M3DT4H5M6.7S`, `P1347M`,
`-P120D`, and components exceeding conventional calendar field widths.

Reject empty, `P`, `PT`, `P1DT`, `+P1D`, `P-1D`, repeated/out-of-order
designators, missing number/designator, fractions on nonseconds components,
`PT.5S`, `PT1.S`, lowercase designators, weeks/basic ISO duration formats,
and timezone suffixes. Cross date-only/time-only/both, negative zero, omitted
zero fields, and documented year/fraction precision support. **V/E**.

### D-029 — duration equality and partial order

Source: [§3.2.6.2–3.2.6.4](../upstream/datatypes.html#duration-order),
[Appendix E](../upstream/datatypes.html#adding-durations-to-dateTimes).

Normalize equivalent year/month and day/time contributions for semantic
comparison: `P1Y = P12M`, `P1D = PT24H`, `PT1M = PT60S`, and signed zero
durations are equal. A month is not a fixed number of days. Compare durations
by adding each to **all four** reference dateTimes:
`1696-09-01T00:00:00Z`, `1697-02-01T00:00:00Z`,
`1903-03-01T00:00:00Z`, `1903-07-01T00:00:00Z`.

Use equal results, all-less, all-greater, less/equal mixtures, and conflicting
results. The partial order includes `P1M > P27D`, `P1M < P32D`, but
`P1M` is incomparable with `P28D` through `P31D`. Similarly partition
`P1Y` against 364/365/366/367 days and the five-month cases in the source.
Reverse operands and negate durations. Bounds treat indeterminate comparison
as false. Restricted year/month-only and day/time-only subsets can be totally
ordered semantically; no XSD 1.1 duration types are introduced. Part 2 gives no
standalone canonical-duration algorithm; do not invent one as a required
serializer contract. **V/E/I**.

The source also permits an optimization table for comparing small month counts
against days. Its complete listed rows are:

| Months | Minimum days | Maximum days |
| --- | --- | --- |
| 1 | 28 | 31 |
| 2 | 59 | 62 |
| 3 | 89 | 92 |
| 4 | 120 | 123 |
| 5 | 150 | 153 |
| 6 | 181 | 184 |
| 7 | 212 | 215 |
| 8 | 242 | 245 |
| 9 | 273 | 276 |
| 10 | 303 | 306 |
| 11 | 334 | 337 |
| 12 | 365 | 366 |
| 13 | 393 | 397 |

For every listed month count, compare days below/at/above each endpoint and
inside the interval, using the four-anchor definition as the result oracle.
An optional shortcut must agree with that definition; the table alone does
not supply an unstated endpoint or interior rule. Its trailing ellipsis is
neither a 13-month value-space limit nor a formula authorizing extrapolation.
No implementation is required to use this optimization.

### D-030 — dateTime field grammar and Gregorian dates

Source: [§3.2.7.1](../upstream/datatypes.html#dateTime-lexical-representation),
[Appendix D](../upstream/datatypes.html#formatdetails).

Use optional minus plus at least four year digits, fixed two-digit month/day,
uppercase `T`, fixed two-digit hour/minute/whole seconds, optional `.` plus
one or more fraction digits, and optional timezone. No plus sign on year,
no year zero, and no leading zero on a year longer than four digits. Accept
`0001`, `-0001`, and `10000` year forms; reject `0000`, `-0000`, `001`,
`010000`, basic dates, week/ordinal dates, space/lowercase-t separators,
missing seconds, and fractions without digits.

For months test 00/01/12/13; days 00/01 and the 28/29/30/31/32 transitions in
each month-length class. Leap years follow the proleptic Gregorian rule,
including pre-1582 dates, divisible-by-4, nonleap century, and divisible-by-400
century. Cross positive/negative years with the no-year-zero boundary, retaining
the source question in D-072 for arithmetic at that boundary. Hours 00–23 are
ordinary; 24 is legal only with zero minutes and seconds, including zero-valued
fraction. `24:00:00` denotes next-day midnight; `24:00:00.001` is invalid.
Minutes are 00–59; leap-second cases are D-070 rather than an automatic blanket
`ss <= 59` assumption. **V/E**.

### D-031 — Timezone syntax, normalization, and absence

Source: [§3.2.7.3](../upstream/datatypes.html#dateTime-timezones),
[Appendix D.3.4](../upstream/datatypes.html#timeZonePermited).

For every temporal type permitting a zone, cross absent, `Z`, `+00:00`,
`-00:00`, positive/negative offsets, minute 00/59/60, hour 13/14/15,
and ±14 with minute 00/01. Only the zero-minute ±14 endpoints are valid;
zone syntax is fixed `±hh:mm` or uppercase `Z`. Reject named zones,
missing colon/digits, second offsets, doubled suffixes, and intervening spaces.

Zero-offset aliases are identical. UTC conversion subtracts a positive offset
and adds a negative offset; cross previous/next day, month, year, leap-day,
and no-year-zero boundaries. An absent timezone is not UTC and must not be
filled from process locale, system timezone, daylight-saving rules, or xml:base.
Offsets are numeric durations, not regional timezone databases. **V/E/K**.

### D-032 — Temporal two-timeline order and facet comparisons

Source: [§3.2.7.4](../upstream/datatypes.html#dateTime-order),
[temporal types](../upstream/datatypes.html#dateTime).

Both zoned values normalize to UTC before comparison; two unzoned values
compare their fields. A zoned and unzoned value may be incomparable. Cross
both operand orders with differences less than 14h, exactly 14h, and greater
than 14h, including a fractional second on either side of the boundary.
Exactly 14h remains indeterminate under the two-timeline definition. Use
positive/negative offsets to ensure fieldwise lexical comparison gives the
wrong result. Lexically identical local fields with and without `Z` are
not identical values.

Apply the type-specific reference context for time and g* comparisons; do
not compare their raw spellings. For all four bounds include equal,
determinate less/greater, and indeterminate candidates. An indeterminate
comparison does not satisfy a bound. Cross bound values with/without zones,
and inherited facets whose endpoints differ only in timezone spelling.
**S/V/E**.

### D-033 — dateTime canonical form

Source: [§3.2.7.2](../upstream/datatypes.html#dateTime-canonical-representation).

Canonical zoned values use UTC `Z`; unzoned values remain unzoned. Remove
trailing fractional zero digits and omit an empty fraction; hour is never 24.
Examples: `2000-12-31T24:00:00Z` maps to
`2001-01-01T00:00:00Z`; `2002-10-10T12:00:00-05:00` maps to
`2002-10-10T17:00:00Z`; `.1200` maps to `.12`.
Cross normalization with month/year rollover, negative years, and arbitrary
supported fractional precision. Preserve actual value equality rather than
canonicalizing a local date/time without applying its offset. **K/E**.

### D-034 — time

Source: [§3.2.8](../upstream/datatypes.html#time).

Accept `hh:mm:ss` with optional fractional seconds and timezone, with the
same field restrictions as dateTime, including midnight `24:00:00` alias.
Reject dates, missing seconds, basic forms, lowercase suffixes, and malformed
fractions. Canonical midnight is `00:00:00`; canonical zoned times use `Z`.
Cross timezone normalization that wraps midnight, fractions, and zoned/absent
comparisons using the prescribed arbitrary-date interpretation rather than
system date. Both zoned and both unzoned pairs are ordered, mixed pairs can
be indeterminate. Recurrence must not accidentally retain a caller's real
calendar date. Leap seconds remain D-070. **V/E/K**.

### D-035 — date as an interval and recoverable timezone

Source: [§3.2.9](../upstream/datatypes.html#date),
[canonical date](../upstream/datatypes.html#date-canonical-representation).

A date denotes a top-open one-day interval, not just a point at midnight.
Accept legal signed year-month-day plus optional zone; reject time suffix,
invalid dates, year zero, and other ISO date formats. Apply the calendar and
year partitions of D-030. Zoned interval identity can have aliases across
calendar days: `2002-10-10+13:00` equals `2002-10-09-11:00`.

Canonicalization uses the date of the UTC-normalized interval midpoint and the
recoverable timezone in the prescribed range **−11:59 through +12:00**.
It does not force all date timezones to Z. Cross offsets −14:00, −12:00,
−11:59, zero, +12:00, +12:01, +13:00, +14:00 and rollover in both directions.
Same lexical date with different ordinary offsets generally denotes different
intervals. Cross interval equality with enumeration/fixed values and partial
order with bound facets; an implementation that drops the offset cannot be
used as the oracle. **V/E/K**.

### D-036 — gYearMonth and gYear

Source: [gYearMonth](../upstream/datatypes.html#gYearMonth),
[gYear](../upstream/datatypes.html#gYear).

Use complete year-month or year respectively, with optional timezone and the
same signed expanded-year/no-year-zero rules. Accept `1999-05`, `1999`,
`-0001`, `10000`, with valid zones where appropriate; reject left truncation,
missing mandatory month for gYearMonth, month 00/13, plus year sign, and extra
day/time fields. A year/month interval is calendar based, not a fixed duration
of 365/30 days. Compare starting instants as specified; cross same-kind zoned,
absent, mixed-zone pairs and rollover. Do not pretend arbitrary calendars
convert one-to-one to Gregorian periods; the specification's caution is
informative, not an input-dependent rejection. **V/E/I**.

### D-037 — gMonthDay, gDay, and gMonth

Source: [gMonthDay](../upstream/datatypes.html#gMonthDay),
[gDay](../upstream/datatypes.html#gDay),
[gMonth](../upstream/datatypes.html#gMonth).

Exact lexical cores are `--MM-DD`, `---DD`, and **`--MM`** respectively, each
with optional timezone. The obsolete `--MM--` spelling is not the published
Second Edition gMonth grammar. No extra leading sign or alternative truncation
is allowed. Test each hyphen count, missing/extra field, leading zeros,
month 00/01/12/13, and day 00/01/28/29/30/31/32.

For gMonthDay use an arbitrary leap year: `--02-29` is valid but `--02-30`
and `--04-31` are invalid. gDay allows `---31` because its reference month
has 31 days. These are recurring calendar values, not dates tied to the
current year/month. Compare starting instants in the prescribed reference
context, including zone changes across missing fields, equal aliases, and
mixed zone indeterminacy. Do not borrow a nonleap host year. **V/E**.

### D-038 — Appendix E arithmetic primitives and ordered steps

Source: [Appendix E](../upstream/datatypes.html#adding-durations-to-dateTimes).

Use the specified floor quotient and modulo, not truncation toward zero.
Cross negative/zero/positive arguments and exact/nonexact multiples, e.g.
`fQuotient(-1,3) = -1`, `modulo(-1,3) = 2`. Repeat with low/high shifted
month range 1…12. maximumDayInMonthFor must normalize out-of-range months and
apply Gregorian leap-year rules.

For `S + D`, add month then year, preserve zone, carry seconds/minutes/hours,
pin the original day into the destination month, then add day/carry and loop
across month/year boundaries. Generate each carry/borrow boundary and more
than one loop iteration. `2000-01-31 + P1M` becomes `2000-02-29`;
`2001-01-31 + P1M` becomes `2001-02-28`. Negative durations must borrow
correctly. Omitted D fields act as zero; omitted S fields use their minimum
for calculation and are removed again afterward. Thus
`2000-01 + (-P3M) = 1999-10` and `2000-01-12 + PT33H = 2000-01-13`.
Cross fractional seconds, large components, preserved zones, missing fields,
and the BCE/CE interpretation question. The copied zone is that of the chosen
algorithm input S; [D2-020](branches/datatypes.md#d2-020--appendix-e-primitives-carries-pinning-missing-fields)
qualifies the mapping from a nonzero-offset literal to those input fields.
No public arithmetic API is required;
the semantics remain observable through duration/temporal comparison. **E/P/I**.

### D-039 — Appendix E noncommutativity, associativity, and leap arithmetic

Source: [Appendix E.1–E.2](../upstream/datatypes.html#adding-durations-to-dateTimes),
[addition order](../upstream/datatypes.html#adding-durations-to-instants-commutativity-associativity).

Keep E.2's separate duration-plus-duration operation: add corresponding fields
without overflow into another field. With all other fields zero, month fields
11 and 2 produce month 13 without carrying to year; minute fields 59 and 2
produce minute 61 without carrying to hour. These are field-calculation
observations, not a unique required lexical serialization. Keep them distinct
from D-029's value equality and from calendar pinning/carry when a duration
is added to a dateTime. [D2-020](branches/datatypes.md#d2-020--appendix-e-primitives-carries-pinning-missing-fields)
retains the concrete contrasts.

Adding durations to dates is order-sensitive because month pinning loses
information: `(2000-03-30 + P1D) + P1M = 2000-04-30`, while
`(2000-03-30 + P1M) + P1D = 2000-05-01`. Include leap and nonleap variants,
negative durations, month-end and ordinary-day controls. Do not replace
sequential addition with combined component arithmetic without proof.

The appendix treats leap-second values as overflows and uses 60 seconds per
minute thereafter: adding `PT1M` and adding `PT60S` give the same result.
Test second 59/60 plus fractional increments, minute/day rollover, and
subtraction. Duration ordering must not depend on a live leap-second table
or future IERS announcements. Distinguish this stable arithmetic definition
from lexical leap-second admission in D-070. **E/I**.

### D-040 — hexBinary

Source: [§3.2.15](../upstream/datatypes.html#hexBinary).

Accept empty and even-length sequences of ASCII hexadecimal digits, either
case; reject odd counts, nonhex letters, `0x` prefixes, internal whitespace,
and Unicode digit lookalikes. Collapse allows surrounding XML whitespace but
does not erase internal spaces. `0aFF` and `0AFF` denote equal octets;
canonical form uses uppercase. Length facets count decoded octets, so a
four-character spelling has length two. Cross high/low nibble 0/F,
upper/lower/mixed case, 0/1/many octets, pattern on lexical spelling, and
enumeration on decoded values. **V/E/K**.

### D-041 — base64Binary grammar, padding, and whitespace

Source: [§3.2.16](../upstream/datatypes.html#base64Binary).

Expand every alternative of `Base64Binary`, `B64S`, `B16S`, `B04S`, `B64`,
`B16`, `B04`, and `Canonical-base64Binary`. The alphabet is A–Z/a–z/0–9/`+`/`/`
with `=` only as final padding. Nonwhitespace length must be a multiple of
four. Tail alternatives are no padding, one `=` with zero unused bits, or two
`=` with zero unused bits. `B16` is `[AEIMQUYcgkosw048]`; `B04` is `[AQgw]`.
Accept empty, `TWFu`, `TWE=`, `TQ==`; reject `TQ`, `TQ=`, `TQ===`,
`=TQ=`, `TR==`, `TWF=` and nonalphabet characters. A permissive decoder that
ignores invalid pad bits is not the specification oracle.

Cross spaces/tabs/LF/CR before, after, between alphabet characters, and between
padding characters. Fixed collapse runs before the grammar, permitting those
normalized single-space positions; non-XML whitespace is not silently erased.
Do not impose MIME's 76-character line limit: longer lines are valid. Decode
Unicode characters after XML character decoding rather than assuming original
US-ASCII bytes. Canonical form contains no whitespace; length is decoded octets,
with 0/1/2/3-byte tails and longer values. Pattern remains lexical; enumeration
uses binary value identity. **V/E/K**.

### D-042 — anyURI and the referenced escaping procedure

Source: [§3.2.17](../upstream/datatypes.html#anyURI),
[normative references](../upstream/datatypes.html#normative-biblio).

Generate empty, relative, absolute, fragment-only, query-only, and relative
references with query/fragment. Apply fixed collapse then the **XLink §5.4 URI
reference escaping procedure** and RFC 2396 as amended by RFC 2732. Cross
non-ASCII characters, spaces, reserved delimiters, percent escapes (valid,
incomplete, nonhex), fragment delimiters, authority/IPv6 bracket forms, and
characters requiring UTF-8 percent encoding. Record raw and escaped strings
separately where the referenced algorithm distinguishes them.

A valid relative reference must not be absolutized during schema validation;
changing xml:base must not change its datatype validity/value merely to satisfy
XLink's unrelated use-time absolutization rule. No network fetch, resource
existence check, scheme-specific policy, media-type fragment validation, or
modern browser URL repair is required. Spaces are discouraged, not categorically
forbidden, when the escaping procedure yields a legal reference. Apply character
length and lexical pattern to the datatype representation, not to a fetched
resource or a host URL object's rewritten spelling. Exact reference-derived
URI negative cases require the cited RFC/XLink clauses as additional oracle
sources. **V/E/I**.

### D-043 — QName context, identity, and ineffective length facets

Source: [§3.2.18](../upstream/datatypes.html#QName),
[length validation](../upstream/datatypes.html#cvc-length-valid).

QName's lexical grammar is XML Namespaces QName; its value is namespace name
plus NCName local part. Cross unprefixed/prefixed values, default namespace
present/absent, bound/unbound prefix, multiple colons, empty prefix/local,
invalid NCNames, and namespace shadowing. Two prefixes bound to one namespace
with the same local name denote equal values; the same prefix rebound to a
different namespace does not. Namespace resolution occurs in the literal's
own context, including schema contexts for enumeration/fixed literals and
instance contexts for data values.

Patterns constrain QName spelling; enumeration constrains the expanded value.
Length, minLength, and maxLength are permitted but deprecated and **every
admitted value of those facets is satisfied by every QName value**; do not
count prefix or namespace URI characters. Cross facet 0/1/large with short and
long QNames. Deprecation is not a schema rejection. There is no context-free
canonical prefix spelling prescribed here. **S/V/E/P/I**.

### D-044 — NOTATION declaration and enumeration requirements

Source: [§3.2.19](../upstream/datatypes.html#NOTATION),
[enumeration requirement](../upstream/datatypes.html#enumeration-required-notation).

A usable NOTATION-derived type must specify enumeration; direct use of NOTATION
is an error. Cross direct/derived use, absent/one/multiple enumeration values,
valid/undeclared notation names, namespace aliases/shadowing, and inheritance
of an established enumeration. Values are QNames of notations declared in the
current schema, not arbitrary NCNames or DTD entity names. Each enumeration
literal must resolve to an allowed notation. Length/minLength/maxLength are
permitted deprecated no-op facets as for QName; pattern is lexical and
enumeration uses QName identity. Attribute-only/no-target-namespace use is a
compatibility recommendation, distinct from the mandatory enumeration and
declaration requirements. **S/V/E/I**.

## Facet applicability, representation, inheritance, and interaction

### D-045 — Complete facet applicability matrix

Source: [§4.1.5, applicable facets](../upstream/datatypes.html#cos-applicable-facets).

For **each row × each of the twelve facets**, construct a restriction with
one well-formed facet. Applicable combinations proceed to their value,
restriction, and fixed rules; every absent combination rejects the schema.
Derivation from a primitive descendant does not broaden the set. The variety
rule takes precedence over its items'/members' primitive facets.

Abbreviations: `L` = length/minLength/maxLength; `P` = pattern; `E` = enumeration;
`W` = whiteSpace; `B` = minInclusive/minExclusive/maxInclusive/maxExclusive;
`D` = totalDigits/fractionDigits.

| Variety or primitive base | Applicable facets |
| --- | --- |
| list, any item type | L P E W |
| union, any members | P E |
| string and descendants | L P E W |
| boolean | P W |
| decimal and descendants | D P W E B |
| float, double | P E W B |
| duration, dateTime, time, date, gYearMonth, gYear, gMonthDay, gDay, gMonth | P E W B |
| hexBinary, base64Binary, anyURI | L P E W |
| QName, NOTATION | L P E W, with L ineffective/deprecated |

Each grouped row expands to every named type/facet pair, not one example per
group. `fixed` does not make an inapplicable facet applicable. Fundamental
facets are properties, not user-specified constraining facets. **S**.

### D-046 — Facet XML, cardinality, fixed, and annotation

Source: [§4.3](../upstream/datatypes.html#rf-facets),
[single facet](../upstream/datatypes.html#src-single-facet-value),
[Appendix A](../upstream/datatypes.html#schema).

For each of the twelve facet elements cross missing/present `value`, empty
versus valid lexical value, optional single annotation, extra/reordered
children, foreign attributes, `id`, and unexpected attributes. length,
minLength, maxLength, and fractionDigits require nonNegativeInteger values;
totalDigits requires positiveInteger; whiteSpace requires exactly one of its
three tokens; pattern requires a valid XSD regex; bounds/enumeration require
admissible typed literals in the appropriate base/context. Leading `+` and
zeros in numeric facet attributes are assessed by their datatype, not a
stricter invented unsigned-digit grammar.

The [whiteSpace value attribute](../upstream/XMLSchema.xsd#whiteSpace) is
restricted from xs:NMTOKEN. XML-S padding therefore collapses before mode
membership. In an otherwise legal restriction of xs:string, `value=" replace "`
is S+, while `value="Replace"`, empty or
whitespace-only values, `value="replace collapse"`, and NBSP-padded `replace`
are S−. This source-attribute normalization is separate from applying the
selected operation to instance data.

All facets except pattern/enumeration occur at most once per derivation step;
repeated equal singleton values are still forbidden. Pattern/enumeration may
repeat. `fixed` is permitted on the other ten facets and defaults false;
accept its boolean aliases `true/false/1/0`, reject other spellings.
**pattern/enumeration prohibit `fixed`**, even `fixed="false"`.

For each fixable facet cross inherited fixed false/true, omitted local facet,
equal local value, tightening value, and loosening value. A fixed value cannot
be changed even to a tighter one. Fixedness cannot be undone in a descendant
by `fixed="false"`; test across two or more restriction steps. Fixed bound
comparison uses typed value equality rather than literal equality. Inherited
facet constraints continue when omitted; annotations do not replace them.
**S/P**.

### D-047 — Length/minLength/maxLength value assessment

Source: [length](../upstream/datatypes.html#rf-length),
[minLength](../upstream/datatypes.html#rf-minLength),
[maxLength](../upstream/datatypes.html#rf-maxLength).

For `n = 0, 1, parameterized n`, test actual lengths `n−1` where possible,
`n`, and `n+1`; length requires equality, minLength ≥, maxLength ≤.
Measure string/anyURI in characters, binary in decoded octets, list in items.
QName/NOTATION always satisfy length checks after valid facet construction.
Run after whitespace handling and value interpretation: token surrounding
spaces do not count, binary padding does not count, and list item spellings
can be arbitrarily long without increasing item count. Cross empty values,
Unicode supplementary/combining characters, binary padding/case aliases,
and list whitespace aliases. **V/E**.

### D-048 — Length restriction and cross-facet ancestry

Source: [length constraints](../upstream/datatypes.html#length-coss),
[minLength constraints](../upstream/datatypes.html#minLength-coss),
[maxLength constraints](../upstream/datatypes.html#maxLength-coss).

Inherited length must be repeated unchanged. minLength may stay equal or
increase, never decrease; maxLength may stay equal or decrease, never increase.
Effective minLength must not exceed maxLength. Fixedness can forbid otherwise
valid tightening. Test each below/equal/above relation and multi-step ancestry.

length has an additional provenance rule: a minLength/maxLength can coexist
with length only when that bound is inherited unchanged from a restriction
ancestor where length was absent, and respectively minLength ≤ length or
length ≤ maxLength. Thus introducing length=3 under a base with minLength=1
is valid; introducing length=3 and minLength=1 together is not equivalent.
Cross both sides, same-step versus inherited bounds, unchanged versus tightened
bounds, and whether an ancestor already supplied length. Do not simplify this
to numeric interval consistency alone. **S/V**.

### D-049 — Pattern groups and inheritance

Source: [§4.3.4](../upstream/datatypes.html#rf-pattern),
[multiple patterns](../upstream/datatypes.html#src-multiple-patterns).

A pattern facet denotes a lexical language; malformed regex rejects the
schema. Repeated patterns in **one restriction step are alternatives (OR)**;
patterns inherited from **different steps all apply (AND)**. Example: local
patterns `a` and `b` admit either; a later restriction pattern `a` admits only
a. An inherited `a` intersected with local `b` admits no value, without turning
that alone into an invalid schema. Cover one/many groups, overlapping/disjoint
languages, empty regex, empty matches, and base lexical validity even when a
pattern would accept an otherwise malformed builtin literal.

Patterns see normalized lexical forms, not canonical forms: decimal pattern
`[0-9]+` can admit `01` while excluding `.1` even when other value facets
could admit the values. Cross patterns with enumeration/bounds/digits, list
whole-literal patterns, and union/member patterns. Reordering OR alternatives
must not change acceptance; member selection remains separately ordered.
Full regex grammar appears in D-057–D-066. **S/V/E**.

### D-050 — Enumeration value identity and narrowing

Source: [§4.3.5](../upstream/datatypes.html#rf-enumeration),
[enumeration restriction](../upstream/datatypes.html#enumeration-valid-restriction).

Repeated local enumeration literals form a set (OR), not a conjunction. Each
literal must denote a value in the base value space under its own namespace
and whitespace context. Reject a new value outside the base range/enumeration
or with malformed base lexical form; duplicates and lexical aliases do not
create distinct allowed values. A restriction can select a subset, not expand
an inherited enumeration; omit it to retain the inherited set.

Cross decimal/integer aliases, rounded floats, NaN identity, binary aliases,
QName namespace aliases, temporal timezone aliases, equivalent durations,
list sequences, and union members from same/different primitive spaces.
Candidate literal must still satisfy its own lexical facets, even if it denotes
an enumerated value. Distinguish an enumeration value's membership in the base
value space from an arbitrary canonical spelling's membership in the derived
pattern language. No `fixed` attribute is legal; boolean does not allow this
facet, while NOTATION requires it. **S/V/E**.

### D-051 — Whitespace operation and restriction lattice

Source: [§4.3.6](../upstream/datatypes.html#rf-whiteSpace),
[restriction](../upstream/datatypes.html#whiteSpace-valid-restriction).

preserve changes nothing; replace changes tab/LF/CR individually to U+0020;
collapse then collapses runs of spaces and trims both ends. Use every one of
the four XML whitespace characters alone, in mixed runs, at each edge, between
characters, and as the whole literal; include NBSP and Unicode line separators
as unchanged controls. XML parsing may already have normalized literals, so
character references are needed to distinguish preserved CR/LF/tab cases.

Allowed derivation edges are preserve→preserve/replace/collapse,
replace→replace/collapse, collapse→collapse; all reverse edges reject schema.
String starts preserve, normalizedString replace, token collapse. Other atomic
primitives and every list have collapse fixed; unions have no whiteSpace
facet and use the successful member's behavior. Cross omitted/equal/tighter
values, inherited fixed values, multiple derivation steps, and subsequent
length/pattern/enumeration checks. whiteSpace is a normalization rule rather
than an additional cvc-whiteSpace validation predicate. **S/V/E/P**.

### D-052 — Four bound facets during value assessment

Source: [§4.3.7–4.3.10](../upstream/datatypes.html#rf-maxInclusive).

For an endpoint b, minInclusive admits x≥b, minExclusive x>b, maxInclusive
x≤b, maxExclusive x<b, using numeric or chronological order as appropriate.
Generate lower/equal/higher/incomparable candidates for each applicable base,
including a discrete integer range with no value between adjacent exclusive
endpoints and dense decimal/fractional-time ranges. Use typed equality for
noncanonical aliases. Incomparable durations/temporal values do not satisfy a
bound; float/double NaN has the D-024 special identity consequences.

Cross two bounds with enumeration/pattern/digits and all four inclusivity
combinations. Neither accepting a nearby lexical spelling nor parsing an
endpoint is sufficient to prove facet satisfaction. **V/E**.

### D-053 — Same-step bound pairs and consistency

Source: [bound constraints](../upstream/datatypes.html#maxInclusive-coss),
[maxExclusive](../upstream/datatypes.html#maxExclusive-coss),
[minExclusive](../upstream/datatypes.html#minExclusive-coss),
[minInclusive](../upstream/datatypes.html#minInclusive-coss).

Reject both maxInclusive and maxExclusive in one derivation step, and both
minInclusive and minExclusive for the same datatype as stated by the rule.
Its scope differs from the upper-pair rule's explicit same-step wording.
Retain [D-072's inherited opposite-lower-kind question](#d-072--questions-and-permissible-choices-that-must-remain-visible)
before promoting an allowed numbered comparison to whole-schema success.
For local lower/upper pairs use these exact source relationships; equal
exclusive endpoints are not all handled by one generic “nonempty interval”
test:

| Lower | Upper | Required relationship |
| --- | --- | --- |
| minInclusive a | maxInclusive b | a ≤ b |
| minExclusive a | maxExclusive b | a ≤ b |
| minExclusive a | maxInclusive b | a < b |
| minInclusive a | maxExclusive b | a < b |

Cross below/equal/above and partial-order indeterminate endpoints; preserve
source ambiguities rather than treating unknown comparison as greater.
With integer minExclusive=1 and maxExclusive=2 the represented set is empty
although a<b; emptiness alone is not a separate prohibited-schema rule here.
Test local duplicates separately from inclusive/exclusive pairs. **S/V/I**.

### D-054 — Every inherited-bound restriction relationship

Source: [maxInclusive restriction](../upstream/datatypes.html#maxInclusive-valid-restriction),
[maxExclusive restriction](../upstream/datatypes.html#maxExclusive-valid-restriction),
[minExclusive restriction](../upstream/datatypes.html#minExclusive-valid-restriction),
[minInclusive restriction](../upstream/datatypes.html#minInclusive-valid-restriction).

Each cell gives a relation whose violation the corresponding rule prohibits
for **new endpoint n** against **inherited endpoint b**. Generate below,
equal, above, and where meaningful incomparable for every cell, plus fixed
and nonfixed ancestors. Endpoint lexical admission/value-space constraints
must also be satisfied; matrix consistency alone is insufficient.

| New facet | Base minInclusive b | Base minExclusive b | Base maxInclusive b | Base maxExclusive b |
| --- | --- | --- | --- | --- |
| minInclusive n | n ≥ b | n > b | n ≤ b | n < b |
| minExclusive n | n ≥ b | n ≥ b | n ≤ b | n < b |
| maxInclusive n | n ≥ b | n > b | n ≤ b | n < b |
| maxExclusive n | n > b | n > b | n ≤ b | n ≤ b |

The table preserves deliberate equality distinctions in the published rules,
including exclusive-to-exclusive repetition. Do not replace it with “the
facet literal must validate as an ordinary instance of the fully restricted
base” when that would reject repetition of an existing exclusive endpoint.
Conversely, do not discard other base facets that constrain admissible facet
values. Cover changing inclusive to exclusive and vice versa, retaining
omitted inherited bounds, equal aliases, and multi-step transitive narrowing.
For incomparable endpoints use the published partial-order rules and D-072's
source questions rather than inventing a total order. **S/V/E/I**.

### D-055 — totalDigits and fractionDigits

Source: [§4.3.11](../upstream/datatypes.html#rf-totalDigits),
[§4.3.12](../upstream/datatypes.html#rf-fractionDigits).

For totalDigits=t, a value must be expressible as `i × 10^(-n)` with integer
i,n, `|i| < 10^t` and `0 ≤ n ≤ t`. fractionDigits=f requires such an expression
with `0 ≤ n ≤ f`, without a bound on i. These are value restrictions, not
counts of characters typed by the author. For t=2, `99`, `9.9`, `.99`,
`00099.00` satisfy totalDigits; `100`, `.001` do not. For f=2,
`1.2300` and `0001.23` satisfy fractionDigits while `1.231` does not.
`1000` requires four total digits; deleting trailing integer zeros is not a
valid shortcut because n is nonnegative. Cover zero, signs, tiny fractions,
large whole values, and lexical aliases at each boundary.

Schema values require t>0 and f≥0, f≤t when both effective facets exist;
restriction may keep or decrease either facet, never increase it. A fixed
facet cannot change. Integer inherits fractionDigits fixed 0, so even a larger
local value is invalid; fractionDigits=0 does not itself rewrite decimal's
lexical grammar to integer's. Cross t/f inherited versus local, absent facets,
equal/tighter/looser values, fixedness, and min/max bounds that leave a small
or empty value space. **S/V/E**.

### D-056 — Cross-facet and derivation interaction generator

Source: [datatype-valid](../upstream/datatypes.html#cvc-datatype-valid),
[facet-valid](../upstream/datatypes.html#cvc-facet-valid),
[restriction](../upstream/datatypes.html#derivation-by-restriction).

For every legal facet subset from D-045 generate an all-satisfied case and,
where constructible, a case failing only each member. Essential opposing axes:

- Lexical pattern versus value enumeration: equivalent decimal/binary/QName/
  temporal spellings can differ in pattern acceptance while sharing a value.
- Whitespace versus length/pattern/enumeration: normalization changes the
  assessed literal/value, while XML attribute normalization is an earlier step.
- Bounds versus digits: value inside the interval can fail precision/scale;
  a precision-admitted value can be outside the interval.
- List-level facets versus item-level facets: item acceptance and sequence
  acceptance are separate requirements, including empty lists.
- Union-level facets versus selected member facets: membership alone does not
  discharge the union restriction; member order can affect typed identity.
- Fixed facets versus numeric narrowing: a smaller value is still an illegal
  change when fixed. Omitted facets inherit; explicit equal aliases do not
  loosen a restriction.
- Same-step versus inherited pattern/enumeration/length/bounds: source-step
  provenance can change validity even when a flattened bag looks similar.

Use one valid schema for value failures and separate invalid schemas for
construction failures. A contradictory schema is not automatically invalid
merely because no instance can satisfy it; require a cited schema constraint.
**S/V/E/P**.

## Appendix F: every regular-expression production and semantic branch

### D-057 — regExp, branch, piece, and atom

Source: [Appendix F](../upstream/datatypes.html#regexs),
[regExp](../upstream/datatypes.html#regex),
[branch](../upstream/datatypes.html#branch),
[piece](../upstream/datatypes.html#piece),
[atom](../upstream/datatypes.html#atom).

Expand `regExp = branch ('|' branch)*`, `branch = piece*`,
`piece = atom quantifier?`, and atom alternatives normal character,
character class, parenthesized regExp. Empty regex, empty branch, and `()`
match the empty string; leading/trailing/interior empty alternatives such as
`|a`, `a|`, `a||b` are legal consequences of the grammar. Cross zero/one/many
pieces/alternatives, nested grouping, precedence (`ab|cd` versus `a(b|c)d`),
and empty/nonempty matching. Reject unmatched parentheses, missing atoms before
quantifiers, and unsupported special-group syntax.

Matching is implicitly whole-string: pattern `a` accepts only `a`, not `ba`
or `ab`; use `.*a.*` for containment, subject to dot's newline exclusion.
`^` and `$` are not start/end anchors in this language. No match-extraction,
backreferences, lookaround, lazy/possessive quantifier semantics, or flags are
specified. A host regex accepting extra syntax is not sufficient. **S/V**.

### D-058 — quantifier, quantity, quantRange, quantMin, QuantExact

Source: [quantifier grammar](../upstream/datatypes.html#quant),
[quantity](../upstream/datatypes.html#quantity),
[quantRange](../upstream/datatypes.html#quantRange),
[quantMin](../upstream/datatypes.html#quantMin),
[QuantExact](../upstream/datatypes.html#quantExact).

Exercise `?`, `*`, `+`, `{n}`, `{n,}`, `{n,m}` with ASCII nonnegative integers
and n≤m. For an atom/group use counts 0/1/2, n−1/n/n+1, and m−1/m/m+1;
include `{0}`, `{0,0}`, `{0,}`, `{0,1}`, and leading-zero counts.
Empty-matching atoms still obey language semantics, e.g. `()*` admits only
empty. As quantifiers, `{,m}`, signed counts, Unicode digits, missing
digits/delimiters, internal spaces, and n>m are not legal forms; repeated
quantifiers such as `a**` and `a+?` are also invalid. Bare-brace strings have
the production/prose tension recorded in D-072; do not mistake a failed
quantifier parse for a settled whole-pattern oracle in those cases. Large
counts remain parameterized grammar cases; a documented resource limit is not
a new language production. **S/V/I**.

### D-059 — Char, normal characters, and SingleCharEsc

Source: [Char](../upstream/datatypes.html#char),
[SingleCharEsc](../upstream/datatypes.html#singlecharesc).

Generate each regex metacharacter literal through its legal escape and test
its unescaped syntactic role. The single-character escape inventory is
`\n \r \t \\ \| \. \- \^ \? \* \+ \{ \} \( \) \[ \]`.
Each denotes exactly its one specified character. Normal characters are XML
characters outside the production's metacharacter set; `^`, `$`, and `-`
outside classes are literal characters, not anchors or ranges. An XML character
reference yields the referenced character **before** regex parsing, so
`&#x2E;` is a wildcard dot unless escaped at the regex layer.

Reject dangling backslash and unrecognized escapes such as `\a`, `\b`, `\f`,
`\v`, `\x41`, `\u0041`, and `\1`; do not reinterpret them using host rules.
A doubled regex backslash matches a literal backslash. Cross XML attribute
escaping (`&amp;`, `&lt;`, character references) with regex escaping; document
which character string the regex parser actually receives. The published Char
production and prose disagree about bare braces and the metacharacter listing
omits the alternation bar; retain D-072 before assigning expectations to those
disputed bare-character cases. **S/V/I**.

### D-060 — charClass, charClassExpr, charGroup, positive/negative groups

Source: [character classes](../upstream/datatypes.html#charcter-classes),
[charClass](../upstream/datatypes.html#charClass),
[charClassExpr](../upstream/datatypes.html#charClassExpr),
[charGroup](../upstream/datatypes.html#chargroup),
[posCharGroup](../upstream/datatypes.html#poschargroup),
[negCharGroup](../upstream/datatypes.html#negchargroup).

A class expression brackets a nonempty positive group, negative group, or
subtraction. Positive group concatenation means set union, not a multi-character
sequence: `[ab]` matches one `a` or one `b`. Cross one/many literals,
ranges/escapes, duplicates/overlap, and reordered components. Negative `[^ab]`
means XML characters outside that set; test inside/outside membership,
newline/CR, supplementary characters, and XML-forbidden controls rejected
before matching. `[]` and `[^]` lack the required positive group and are
invalid. Caret at the class's first position negates; literal caret there
must be escaped, while permitted later caret positions follow the range
rules. A class always consumes one character even when its set is empty;
quantification can still admit empty strings. **S/V**.

### D-061 — charRange, seRange, charOrEsc, XmlChar, XmlCharIncDash

Source: [range productions](../upstream/datatypes.html#charrange),
[seRange](../upstream/datatypes.html#serange),
[charOrEsc](../upstream/datatypes.html#charoresc),
[XmlChar](../upstream/datatypes.html#xmlchar),
[XmlCharIncDash](../upstream/datatypes.html#XmlCharIncDash).

For singleton/range alternatives generate equal endpoints, increasing range,
decreasing range (invalid), adjacent code points, supplementary endpoints,
and membership immediately below/at/inside/at/above endpoints. A range's
endpoints may be literal XML characters or single-character escapes; a
multicharacter/category escape such as `\d` or `\p{L}` cannot serve as a
single endpoint. Test escaped delimiters as endpoints and escaped newline/tab.

Unescaped `-` is literal only at the beginning/end of a positive group;
else it separates a range or introduces subtraction. `[-a]`, `[a-]`, and
`[a\-z]` differ from `[a-z]`. Literal backslash, brackets, and an initial
caret require the production-specific escaping. Test ambiguous-looking
combinations by the disambiguation prose, not by a host library's permissive
class parser; raw XML character-reference notation in the source grammar
is not an extra runtime escape language. **S/V**.

### D-062 — charClassSub and nesting

Source: [subtraction](../upstream/datatypes.html#charclasssub).

A positive/negative group followed by `-` and a bracketed class expression
subtracts the latter set from the former. `[a-z-[aeiou]]` admits lowercase
consonants; `[a-z-[a-z]]` admits no character; `[^a-[b]]` excludes both a
and b from the XML-character universe. Cross disjoint/overlapping/subset/equal
sets, ranges/category escapes, negative minuends, negative subtrahends,
and nested subtraction such as `[a-z-[d-w-[m-o]]]`.

Subtraction is structurally the final operation in its containing class:
reject missing brackets, missing minuend, and trailing group material after
the subtrahend where the grammar does not permit it. Do not interpret
`[a-z&&[^aeiou]]` as an added intersection operator or chained set operators
from another regex dialect. Empty set matching versus empty string matching
must stay distinct. **S/V**.

### D-063 — catEsc, complEsc, charProp, IsCategory, and every category

Source: [category escapes](../upstream/datatypes.html#catesc),
[complement](../upstream/datatypes.html#complesc),
[charProp](../upstream/datatypes.html#charprop),
[categories](../upstream/datatypes.html#cats).

For every legal name test `\p{X}` and `\P{X}`, independently and in positive,
negative, and subtracted classes. Names are case-sensitive and braces are
required. The complete grammar partitions are:

| Production | Legal names |
| --- | --- |
| [Letters](../upstream/datatypes.html#lets) | L, Lu, Ll, Lt, Lm, Lo |
| [Marks](../upstream/datatypes.html#marks) | M, Mn, Mc, Me |
| [Numbers](../upstream/datatypes.html#nums) | N, Nd, Nl, No |
| [Punctuation](../upstream/datatypes.html#punc) | P, Pc, Pd, Ps, Pe, Pi, Pf, Po |
| [Separators](../upstream/datatypes.html#seps) | Z, Zs, Zl, Zp |
| [Symbols](../upstream/datatypes.html#syms) | S, Sm, Sc, Sk, So |
| [Others](../upstream/datatypes.html#others) | C, Cc, Cf, Co, Cn |

Generate a member and nonmember of every category, including non-ASCII Nd,
combining marks, private-use, unassigned, and supplementary values supported
by the selected Unicode baseline. Aggregate categories include their listed
subcategories. `Cs` is excluded: surrogate code units are not XML characters.
Reject unknown/malformed category names, wrong case, empty braces, omitted
braces, and host-property spellings outside this grammar.

The Recommendation requires support for properties of the Unicode database
current at its publication, while encouraging later versions; the bibliography
links a historical Unicode 3.1 update. Pin and record a Unicode version for
expected membership, and separate baseline obligations from permitted future
version changes. Do not label a changed assignment a definite failure without
that version evidence. **S/V/I**.

### D-064 — IsBlock and every listed block boundary

Source: [block escapes](../upstream/datatypes.html#blockesc),
[Unicode bibliography](../upstream/datatypes.html#UnicodeDB).

The grammar is `Is` followed by ASCII letters/digits/hyphens; recognized block
names are Unicode block names with whitespace removed. For **every row of
Appendix F's block table**, instantiate lower−1/lower/lower+1,
upper−1/upper/upper+1 membership cases, restricted to XML characters, for
`\p{IsName}` and its complement. This parameterization covers every listed
block without copying a second mutable Unicode range database into this
catalogue. Include both discontiguous `Specials` ranges; membership is their
union. Include blocks with unassigned code points: block membership does not
require assignment to a letter or another non-Cn category.

Cross names with spaces, wrong case, missing Is, illegal punctuation, unknown
blocks, and supplementary/future blocks under the recorded Unicode support
version. HighSurrogates, LowSurrogates, and HighPrivateUseSurrogates are
excluded by this specification. A host Unicode script property is not a block
property. Separate required historical block support from encouraged future
support and from XML character admission. **S/V/I**.

### D-065 — charClassEsc, MultiCharEsc, and WildcardEsc

Source: [escapes](../upstream/datatypes.html#charclassesc),
[MultiCharEsc](../upstream/datatypes.html#multicharesc),
[WildcardEsc](../upstream/datatypes.html#wildcardesc).

Expand charClassEsc's single/category/complement/multicharacter alternatives.
For each shorthand test a positive member, excluded member, complement,
and use within a class/range subtraction:

| Escape | Required set |
| --- | --- |
| `.` | Every XML character except LF and CR |
| `\s`, `\S` | Space, tab, LF, CR; and its complement |
| `\i`, `\I` | XML initial name characters including underscore and colon; and complement |
| `\c`, `\C` | XML NameChar; and complement |
| `\d`, `\D` | Unicode Nd; and complement |
| `\w`, `\W` | All characters except P, Z, C categories; and complement |

Dot includes tab and supplementary characters but not newline/CR. `\s` does
not include NBSP; `\d` includes non-ASCII digits even though numeric datatype
lexical grammars use ASCII. `\w` is not ASCII alphanumeric/underscore:
combining marks are included, while underscore (Pc) is excluded. `\i`/`\c`
use the referenced XML name rules, not the implementation language's identifier
syntax. Complements are relative to the XML-character universe. **S/V**.

### D-066 — Regex transport, exact character matching, and unsupported features

Source: [Appendix F](../upstream/datatypes.html#regexs),
[terminology](../upstream/datatypes.html#terminology),
[level of Unicode support](../upstream/datatypes.html#multicharesc).

Cross regex text written with direct characters, XML character/entity
references, attribute literal whitespace, and references preserving LF/CR/tab.
A literal `&`/`<` must first be valid XML; resulting regex punctuation still
needs regex escaping. Match supplementary code points as characters, not
surrogate pairs; XML forbids isolated surrogates. Do not perform case folding,
Unicode normalization, locale collation, or grapheme-cluster matching.
Precomposed and combining sequences remain different languages unless the
pattern explicitly allows both.

Exercise captures/backreferences/lookaround/inline flags/anchors as unsupported
syntax or literal characters according to the actual productions; avoid the
blanket claim that every familiar host extension is a syntax error (`^`/`$`
are literals). The appendix targets Unicode regex Level 1; Level 2 and
normalization-sensitive matching are future aspirations, not requirements.
Large/deep/repeated patterns provide parameterized resource-limit scenarios,
whose operational outcomes must be recorded separately from language validity.
**S/V/I**.

## Appendices, referenced definitions, and source questions

### D-067 — Appendix A schema-for-datatypes coverage

Source: [Appendix A](../upstream/datatypes.html#schema),
[local extracted schema text](../upstream/datatypes.xsd.txt).

The appendix schema is normative for the syntactic constraints it expresses;
its documentation and illustrative primitive definitions are not replacements
for the built-in semantic definitions. Cover each global simpleType and
facet declaration and each supporting grammar component:
`simpleDerivation`, `simpleDerivationSet`, `simpleType`,
`topLevelSimpleType`, `localSimpleType`, `facets`,
`simpleRestrictionModel`, `restriction`, `list`, `union`,
`facet`, `noFixedFacet`, and `numFacet`. Its derivationControl utility type is
not an extra public built-in datatype. Its primitive “magic” definitions do
not license users to manufacture arbitrary new primitive types by restriction
of anySimpleType.

Expand every sequence/choice/occurrence/attribute use/default/prohibition and
wildcard branch using D-006/D-007/D-045/D-046. Cross global/local type grammar,
constructor sources, optional annotation, required value, numeric facet value
types, boolean fixed aliases/default, and fixed prohibition on enumeration/
pattern. The schema cannot express every prose constraint: syntactic schema
validity alone does not discharge XOR, applicability, fixedness, or inherited
facet consistency. URI-addressable built-in/facet/facet-use identifiers identify
specification definitions; they do not require dynamic loading. **S/P/I**.

### D-068 — Appendix B DTD is informative and incomplete

Source: [Appendix B](../upstream/datatypes.html#dtd-for-datatypeDefs),
[local DTD text](../upstream/datatypes.dtd.txt).

The DTD is explicitly nonnormative, not standalone, and designed for inclusion
in XMLSchema.dtd. Its parameter entities support namespace-prefix and foreign
attribute customization. Use it as a syntax cross-check, never as an oracle
that overrides Appendix A or prose constraints. Generate opposing cases that
fit its loose declarations but violate mandatory XSD constraints: missing
global name, invalid final token, invalid typed facet value, repeated singleton
facet, simultaneous exclusive/inclusive bound, both/neither base source,
and empty union. Its boolean declaration limited to true/false does not repeal
schema boolean lexical aliases 1/0. Record DTD-valid versus XSD-valid separately.
**I/S**.

### D-069 — Appendix C complete built-in fundamental-facet table

Source: [Appendix C.1](../upstream/datatypes.html#app-fundamental-facets).

Compare every built-in row's four properties as component facts, if exposed.
The complete partition is:

| Built-ins | ordered | bounded | cardinality | numeric |
| --- | --- | --- | --- | --- |
| string, hexBinary, base64Binary, anyURI, QName, NOTATION | false | false | countably infinite | false |
| boolean | false | false | finite | false |
| float, double | partial | true | finite | true |
| decimal, integer, nonPositiveInteger, negativeInteger, nonNegativeInteger, positiveInteger | total | false | countably infinite | true |
| duration, dateTime, time, date, gYearMonth, gYear, gMonthDay, gDay, gMonth | partial | false | countably infinite | false |
| normalizedString, token, language, NMTOKEN, NMTOKENS, Name, NCName, ID, IDREF, IDREFS, ENTITY, ENTITIES | false | false | countably infinite | false |
| long, int, short, byte, unsignedLong, unsignedInt, unsignedShort, unsignedByte | total | true | finite | true |

Each grouped row means every listed built-in, not one representative. Do not
silently “correct” finite recurring-calendar domains or the general derivation
formula from set-theoretic intuition; any source inconsistency is a D-072
question. Derivation branches are D-013; lexical/value behavior remains covered
by D-015–D-044. Lack of component-property exposure is an observation gap,
not evidence of a pass or failure. **P/I**.

### D-070 — Appendix D format exceptions and leap-second edge cases

Source: [Appendix D.1](../upstream/datatypes.html#formatdetails),
[truncation](../upstream/datatypes.html#truncatedformats),
[deviations](../upstream/datatypes.html#deviantformats).

Run a type-by-format matrix for duration/dateTime/time/date/gYearMonth/gYear/
gMonthDay/gDay/gMonth. Only each type's explicit format is accepted; general
ISO 8601 basic, week, ordinal, reduced, and truncated forms do not become legal
by being ISO forms. Cross permitted optional minus/expanded years/zones with
forbidden plus/year-zero/unsupported truncation, and ASCII versus Unicode
digits. Apply Gregorian leap-year rules before 1582 as well as afterward.

The appendix explicitly discusses second 60 and fractional leap seconds,
local-time positions of UTC leap seconds, and overflow handling when the
month/day is inappropriate. Generate UTC and offset forms, genuine and
inappropriate candidate dates, second 59/60/61, fractions, and minute/day
rollover. **Do not state that XSD 1.0 simply forbids all second-60 literals.**
The historical March/June/September/December wording and interaction with the
body's field limits require a source-qualified oracle; retain D-072's question
for cases not unambiguously determined by those clauses. Appendix E's
specified arithmetic treats leap-second fields as overflow independently of
any live leap-second schedule. **V/E/I**, unresolved cases remain questions.

### D-071 — Appendices G, H, and I, and external normative dependencies

Source: [Glossary G](../upstream/datatypes.html#normative-glossary),
[References H](../upstream/datatypes.html#biblio),
[Acknowledgements I](../upstream/datatypes.html#acknowledgments).

G is a **nonnormative glossary**, even though its fragment is named
`normative-glossary`; it routes terms back to definitions and adds no validation
rule. H separates normative references from informative ones. I is
acknowledgements and creates no processor obligations. These are covered by
classification, not invented negative instances.

External dependency obligations must retain the exact cited scope/version:
XML 1.0 character/name/normalization rules, XML Namespaces QName/NCName/context,
Part 1 schema/PSVI/document identity rules, XML Base where actually referenced,
IEEE floating-point context plus XSD's different identity and precise lexical
mapping, XLink **only its §5.4 escaping procedure**, RFC 2396 plus RFC 2732,
RFC 2045's alphabet without its optional line length limit, RFC 3066 language
tag definition, accurate rounding reference, and the Unicode database/version
rules. ISO 8601, Unicode regex Level 2 aspirations, the Primer, and the
informative references cannot override XSD's own lexical definitions.

A future scenario importing an external rule needs its own clause citation
and a local source snapshot if absent. This catalogue identifies those seams;
it does not assert that citing the Part 2 bibliography alone exhaustively
expands every external standard. **I**, with **S/V/E** at each owning seam.

### D-072 — Questions and permissible choices that must remain visible

Source: [Recommendation](../upstream/datatypes.html), with clauses cited in
the corresponding families.

These are source/observation questions, not implementation conclusions or
permission to choose an arbitrary answer:

| Question | Required treatment before scoring a generated case |
| --- | --- |
| Unicode baseline and future revisions | Record database version and whether a membership/name is required baseline support or encouraged later support; use publication-time language and bibliography evidence together. |
| Double value-space wording | The published double paragraph gives exponent range −1075…970 while describing IEEE double; preserve that exact source discrepancy for extremal-value oracles rather than silently using a host float model. Ordinary correctly rounded cases are unaffected. |
| Numeric canonical zero sign | The [generic canonical definition](../upstream/datatypes.html#canonical-lexical-representation) requires one-to-one value representation, but the [decimal](../upstream/datatypes.html#decimal-canonical-representation), [integer](../upstream/datatypes.html#integer-canonical-repr) and several descendant clauses do not expressly remove minus-zero. Their conventional `0.0`/`0` outputs need that source integration before unique K scoring. [nonPositiveInteger](../upstream/datatypes.html#nonPositiveInteger-canonical-repr) explicitly omits zero's sign, and float/double explicitly prescribe `0.0E0`; those controls remain determined. The conventional outputs are not proved wrong and two alternative output policies are not thereby permitted. |
| Unsigned lexical prose and complete definitions | The four [unsigned Chapter 3 descriptions](../upstream/datatypes.html#unsignedLong-lexical-representation) say decimal digits, while [the derived overview](../upstream/datatypes.html#built-in-derived) directs complete definitions to [Appendix A](../upstream/datatypes.html#schema). A supplies the signed integer pattern and bounds-only unsigned derivations. Retain that exact basis for signed aliases such as `+1` and `-0`; do not silently erase the prose seam or select a contrary V− policy. Partition each unsigned subtype, sign, zero/positive/negative value and range endpoint. |
| Duration total-order context | The [two field-based total-order rows](../upstream/datatypes.html#total-order-durations) are followed by a timezone required/prohibited phrase, although duration's grammar has no timezone. Record the printed premise; D-028's timezone-suffix rejection remains. No feasible required-zone duration branch or new order failure is inferred. |
| Date explanatory signs and direction | [Date's explanatory UTC-day examples and nearby interval/range notes](../upstream/datatypes.html#date) contain direction/sign discrepancies relative to its operative offset and interval definitions. Keep the operative interval/midpoint and optional-sign grammar separate from those examples; no alternative date oracle or approved correction follows. |
| Regex bare braces and metacharacters — RXQ001/RGQ001 | The [Char production](../upstream/datatypes.html#char) omits braces from its excluded set although [the prose](../upstream/datatypes.html#dt-metac) calls them metacharacters; the prose list omits alternation bar while the grammar excludes it. Preserve one question under both aliases. Escaped literals and well-formed quantifiers remain unambiguous. |
| Leap seconds | Reconcile Appendix D's admission/inappropriate-date guidance, body field limits, and Appendix E's overflow arithmetic. Do not import XSD 1.1's rules or require a live leap-second table. |
| Appendix D truncation list | [D.2](../upstream/datatypes.html#truncatedformats) lists date/gMonth/gYear as right-truncated forms, while [gYearMonth's own definition](../upstream/datatypes.html#gYearMonth-lexical-repr) explicitly calls its form right-truncated. Preserve that omission and the nearby exception-count wording as a source-recording question. The specific type grammars remain described; this is no approved correction or permission for arbitrary ISO truncation. |
| BCE/CE arithmetic | Lexical year zero is forbidden and −0001 denotes 1 BCE; Appendix E arithmetic needs an explicit mapping around that boundary. Record any unresolved source interpretation instead of using astronomical numbering silently. |
| Union cardinality wording | §2.5.1.3's informal “greater than 1” contrasts with §4.1's nonempty sequence, XML grammar, and representation constraint permitting one member. Use the concrete component/representation rules and preserve the wording issue. |
| Union canonical overlap | [§2.3.1](../upstream/datatypes.html#canonical-lexical-representation) defines canonical literals one-to-one with values, while [§2.5.1.3](../upstream/datatypes.html#union-datatypes) uses the appropriate member type. Equal values shared by [common-ancestor members](../upstream/datatypes.html#equal) with differing canonical grammars, such as [integer](../upstream/datatypes.html#integer-canonical-repr) and [decimal](../upstream/datatypes.html#decimal-canonical-representation), require an explicit integration qualification. This catalogue chooses neither a value-only nor a selected-member output policy; these are not thereby two permitted processor choices. Ordinary disjoint-member cases remain unaffected. Use [IP-003's canonical subproduct](interactions.md#ip-003--unionlist-selection--lexical-facets--value-facets--typed-tuples); do not score ambiguous K from acceptance or member metadata alone. |
| Fundamental-facet formulas versus mathematical sets | Published list cardinality/boundedness formulas and recurring-calendar built-in classifications may differ from an intuitive cardinality calculation. Classify the specified property, keep semantic set questions separate. |
| Indeterminate bound endpoints | Value assessment requires satisfaction of a partial-order bound. Schema restriction clauses phrase prohibited comparisons differently; preserve those clauses and obtain an authoritative interpretation before claiming an indeterminate endpoint must be rejected by a stronger inferred condition. |
| Inherited opposite lower-bound kinds | [minInclusive/minExclusive coexistence](../upstream/datatypes.html#minInclusive-minExclusive) says same datatype, while the upper-kind condition says same derivation step. Facet merging removes inherited same-kind facets, so new minExclusive over inherited minInclusive and the converse need an explicit integration of that scope before whole-schema success. D2-033 retains all named comparison-clause results but qualifies three otherwise-positive total-S cells. No same-step rewrite, blanket rejection or arbitrary processor choice is selected. |
| Exclusive endpoint repetition | Published restriction rules permit certain equal exclusive endpoints; ordinary instance-validity checks against that same exclusive bound cannot be substituted blindly. Preserve value admission versus endpoint-special-case evidence. |
| language lexical pattern versus RFC semantics | The built-in pattern is mandatory and historical RFC 3066 is referenced. Do not add a modern registry requirement; source any proposed additional semantic rejection. |
| anyURI equality/escaping representation | Preserve the raw lexical value, whitespace-normalized value, and escaped URI reference where relevant. Do not assume percent-decoding, case folding, URI normalization, absolutization, or network equivalence creates datatype value identity without a clause. |
| Canonical exposure | No public serializer/PSVI API is required by a catalogue family. Classify absent observation as not directly observable; use valid Part 1 witnesses where available. Do not claim a canonical duration/g* prefix algorithm absent a source rule. |
| Precision/resource maxima | Separate invalid language input from permitted documented precision limits and implementation operational limits. A resource refusal is not a datatype-invalid oracle. |
| Recommendations and errors | Attribute-only/no-target-namespace compatibility recommendations and discouraged/deprecated constructs do not become mandatory rejection rules. The [generic error definition](../upstream/datatypes.html#dt-error) leaves results undefined and independently permits detection/reporting and recovery, without prescribing their strategy. Preserve applicable [must](../upstream/datatypes.html#dt-must) and conformance obligations; those generic permissions do not waive them. |

RXQ002 names the Unicode support-version premise above. The
[category](../upstream/datatypes.html#catesc) and
[block](../upstream/datatypes.html#blockesc) rules require the publication-time
baseline and encourage future support; a changed assignment or later block
needs that version premise, not an automatic required rejection. The separate
2011 Note does not amend the 2004 baseline silently.

FFP004/006/008 retain concrete cardinality contrasts. A length-2 list of
unrestricted integers is finite under the printed component formula despite
its infinite item combinations. An otherwise unbounded string restricted only
to enumeration values `a` and `b` has a finite value space, while enumeration
alone is not a finite trigger in the atomic formula. A pattern admitting only
those values has the analogous formula-versus-set distinction. Preserve the
published component calculation and the semantic question separately;
boolean validity observes neither cardinality output. FM13's common-ancestor
wording qualification remains in D-013. The numerical-source conflicts retain
their [XR-115/116](external-rules.md) owners.

## Named-constraint traceability

This table accounts for every current `constraintnote` ID in the Part 2 source.
It covers component constraints (`cos`), representation constraints (`src`),
and validation rules (`cvc`); prose/grammar obligations without such names are
covered above. IDs identify fragments in the published standard, not
repository error codes.

| Specification constraint | Scenario families |
| --- | --- |
| `enumeration-required-notation` | D-044, D-050 |
| `src-single-facet-value` | D-046 |
| `src-list-itemType-or-simpleType` | D-007 |
| `src-restriction-base-or-simpleType` | D-007 |
| `src-union-memberTypes-or-simpleTypes` | D-007, D-011 |
| `cvc-facet-valid` | D-046–D-056 |
| `cvc-datatype-valid` | D-002, D-008–D-012, D-015–D-056 |
| `cos-applicable-facets` | D-045 |
| `cos-list-of-atomic` | D-008 |
| `cos-no-circular-unions` | D-011 |
| `cvc-length-valid` | D-043, D-044, D-047 |
| `length-minLength-maxLength` | D-048 |
| `length-valid-restriction` | D-048 |
| `cvc-minLength-valid` | D-043, D-044, D-047 |
| `minLength-less-than-equal-to-maxLength` | D-048 |
| `minLength-valid-restriction` | D-048 |
| `cvc-maxLength-valid` | D-043, D-044, D-047 |
| `maxLength-valid-restriction` | D-048 |
| `src-multiple-patterns` | D-049 |
| `cvc-pattern-valid` | D-049, D-057–D-066 |
| `src-multiple-enumerations` | D-050 |
| `cvc-enumeration-valid` | D-003, D-050 |
| `enumeration-valid-restriction` | D-050 |
| `whiteSpace-valid-restriction` | D-051 |
| `cvc-maxInclusive-valid` | D-024, D-029, D-032, D-052 |
| `minInclusive-less-than-equal-to-maxInclusive` | D-053 |
| `maxInclusive-valid-restriction` | D-054 |
| `cvc-maxExclusive-valid` | D-024, D-029, D-032, D-052 |
| `maxInclusive-maxExclusive` | D-053 |
| `minExclusive-less-than-equal-to-maxExclusive` | D-053 |
| `maxExclusive-valid-restriction` | D-054 |
| `cvc-minExclusive-valid` | D-024, D-029, D-032, D-052 |
| `minInclusive-minExclusive` | D-053 |
| `minExclusive-less-than-maxInclusive` | D-053 |
| `minExclusive-valid-restriction` | D-054 |
| `cvc-minInclusive-valid` | D-024, D-029, D-032, D-052 |
| `minInclusive-less-than-maxExclusive` | D-053 |
| `minInclusive-valid-restriction` | D-054 |
| `cvc-totalDigits-valid` | D-055 |
| `totalDigits-valid-restriction` | D-055 |
| `cvc-fractionDigits-valid` | D-055 |
| `fractionDigits-totalDigits` | D-055 |
| `fractionDigits-valid-restriction` | D-055 |

## Coverage boundary

The catalogue spans §§1–5, every built-in datatype, every named Part 2
constraint, every Appendix F production, and appendices A–I. It provides
parameterized generation rules and expected semantic partitions; it is **not**
a generated corpus, implementation audit, or corpus gap report. Individual
concrete scenarios and imported external-standard obligations still need
source-qualified expectations, and D-072's unresolved questions must remain
visible during that work. Corpus coverage must later be measured against
these obligations, including positive, negative, boundary, and context cases;
the existence of this document proves no implementation conformance.
