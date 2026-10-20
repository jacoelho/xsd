# Concrete cases: identity, simple types, schema composition, and assessment

This layer gives concrete inputs and consequences for Structures §§3.11–5
and its appendix boundaries. Read it with [the family catalogue](../structures.md),
the [corrected reading](../../corrected-reading.md), and [source attribution](../../NOTICE.md).
Expectations come from the captured Recommendation, not an implementation or
corpus. A case is a described witness, not a claim that an executable fixture
exists. Some abstract component or PSVI cases have no validity-only XML API.

`xs` denotes `http://www.w3.org/2001/XMLSchema`; `t` denotes `urn:a`; `p`
denotes `urn:b` unless a case explicitly rebinds it. Unprefixed example
instance elements have no namespace unless stated. Schema sketches supply
the ordinary declarations/content needed to admit the shown instance;
changing an identity constraint does not excuse an unrelated content error.
**S**, **V**, **P**, and **O** mean schema construction, instance validity,
PSVI/component observation, and processor invocation/choice. An abstract
component mutation is explicitly identified: XML grammar can forbid its
construction before that semantic predicate is reached.

## Identity constraints and XPath

Source: [§3.11](../../upstream/structures.html#cIdentity-constraint_Definitions),
[validation](../../upstream/structures.html#cvc-identity-constraint),
[selector](../../upstream/structures.html#c-selector-xpath), and
[field](../../upstream/structures.html#c-fields-xpaths).

### C-001 — selector context, descendants, and set identity

On `doc`, use selector `r` and field `@id`, with integer attributes.
`<doc><r id="1"/><box><r id="1"/></box></doc>` selects only the direct `r`;
`unique` succeeds. Change only the selector to `.//r`: both rows participate
and uniqueness fails. Selector `.` selects `doc` itself; use `doc/@id="1"`
to obtain one tuple. Selector `r|r` returns a node set containing each `r`
once, so one row does not duplicate itself. An abstract selector returning an
ancestor, attribute, or atomic number fails the selector's result contract.
An ordinary XML representation cannot express those results through the
admitted subset. **S/V/P**.

### C-002 — selector syntax and equivalent axes

Admit `.`, `r`, `r/s`, `.//r`, `*`, `p:*`, and `p:r|p:s` under a schema
binding `p=urn:b`. At the component/evaluation boundary, whitespace around
tokens in ` p:r / p:s | p:t ` does not change selection, and `child::p:r` is
an equivalent child-axis form. Their XML representations retain the narrower
Appendix A admission gate and [XPS-Q001](../source-questions.md#xps-q001--xpath-component-syntax-and-xml-representation).
Reject selector `@id`, `../r`, `r[1]`, `text()`, `count(r)`, and `r//s`:
full XPath support does not enlarge the XSD subset. An unbound `p` cannot
name `urn:b` rows. Retain maximal tokens such as `p:*`; splitting them into
`p`, `:`, `*` is not a different valid expression. **S**, then **V/P** for
the admitted forms.

### C-003 — field syntax and terminal attributes

For rows with `<v code="7">8</v>`, field `v/@code` yields integer 7 when
that attribute is declared integer; field `v` yields its simple value 8.
Admit `@id`, `@*`, `p:v/@p:code`, `.`, and `.//v` in appropriate simple-valued
setups. `attribute::id` and `child::v/attribute::code` are equivalent component
axis forms, conditional on representation admission under
[XPS-Q001](../source-questions.md#xps-q001--xpath-component-syntax-and-xml-representation).
Reject `@id/v`, `v/@id/@code`, `v[1]`, and `v/text()` as subset
syntax. For `@*`, one matching attribute gives one value; two matching
attributes reach C-004's cardinality failure. **S/V**.

### C-004 — field cardinality is checked before tuple qualification

Selector `r`, field `v`, with zero, one, or two simple `v` children per row:
zero supplies no field value; one supplies one value; two violate the
at-most-one-node rule. The two-node case is invalid for `unique`, `key`, and
`keyref`. In a two-field constraint (`v`, `@other`), leave `@other` absent
while keeping two `v` nodes: the cardinality error still exists. A missing
second field cannot hide a multiple-node first field. **V**.

### C-005 — simple values and field ordering

Select one `v` declared `xs:integer`, then replace its declaration by an
element-only complex type containing `x`. The integer supplies a field value;
the element-only complex value does not satisfy the simple-value premise.
For fields `@a`, `@b`, rows `(1,2)` and `(2,1)` are distinct ordered tuples;
rows `(1,2)` and `(1,2)` conflict under `unique`/`key`. Changing field order
on only a keyref changes the referenced tuple being sought. A complex type
with simple content must be distinguished from the element-only negative;
its simple-content value and E1-56 normalization require their own observation.
For a concrete complex type with simple content extending integer, `<v>1</v>`
is value-bearing under the documented WG clarification; retain the literal
1.0-wording qualification in [I-001](../../INTERPRETATIONS.md#i-001--complex-types-with-simple-content-as-identity-fields).
**V/P**, with that expectation classified as WG interpretation.

### C-006 — missing fields distinguish all three categories

Use selector `r`, field `@id`, and two rows without `@id`. `unique` has no
qualified tuples to compare and succeeds. `key` fails because every selected
row must supply a complete tuple. A `keyref` with these incomplete rows does
not demand referents for them. Add `id="9"` to one keyref row while its
referenced key table contains only 1: the complete tuple now fails referential
checking. An empty selector result is valid for all three categories. **V**.

### C-007 — value equality, defaults, and primitive distinctions

For decimal fields, `id="01"` and `id="1.0"` conflict; for string fields the
same two spellings do not. A decimal and its integer-derived value compare
within their shared primitive value space; a string `"1"` and decimal 1 do
not become equal merely because their renderings coincide. For a defaulted
integer attribute `id` with default `1`, an absent attribute and an explicit
`id="01"` create equal qualified tuples and fail uniqueness. List values
`1 2` and ` 1  2 ` of a list-of-integer type also compare equal. A lexical
failure prevents a value from serving as a successfully typed witness. **V/P**.

### C-008 — a key field's nillable declaration matters even when not nilled

Use key selector `r`, field `v`, with one `<v>1</v>`. With a nonnillable
integer `v`, the row can satisfy the key. Change only its declaration to
`nillable="true"`, keeping the same nonnil instance: the key's nonnillable
field-declaration condition fails. Adding `xsi:nil="false"` does not repair
that condition. This is distinct from checking whether a field value is
missing or whether the instance actually requested nil. **V**.

### C-009 — nilled and invalid field values are not lexical empty strings

Compare an absent `v`, `<v/>` of type string, and a validly nilled `v` of
type string. The empty nonnil string is an actual value; a nilled element
has no schema-normalized value under E1-56. Never substitute `""` for that
absent property to invent a tuple. Keys also fail C-008's separate declaration
condition. For `unique`/`keyref`, retain the exact field-value/qualified-set
interpretation as a source question if an observable tuple cannot be derived:
the literal node-cardinality wording must be read with the type-determined
value premise. [I-003](../../INTERPRETATIONS.md#i-003--nilled-field-values)
records why the later 1.1 disposition does not itself amend this 1.0 branch.
A type-invalid `<v>bad</v>` under integer is independently
invalid and cannot prove a particular missing-field outcome. **V/P/question**.

### C-010 — referenced component, arity, and tuple order

Define key `K` with fields `@a`, `@b`; keyref `R` with fields `@x`, `@y`
and `refer="K"` uses that intended component, subject to
[IDS-Q001](../source-questions.md#ids-q001--identity-constraint-refer-and-generic-qname-resolution).
Its category and arity are compatible. Give `R` one field: schema component
correctness fails before any instance comparison. Point `R` to another
keyref instead of a key/unique: it is the wrong referenced category. With
key tuple `(1,2)`, reference `(1,2)` succeeds and `(2,1)` fails. Referring to
a `unique` is permitted; it still requires the referenced node-table entry.
**S/V**.

### C-011 — repeated scopes do not create one global uniqueness domain

A global recursive `scope` declaration has `K` selecting its direct `r`
children, field `@id`. Two sibling `scope` instances each containing one
`r id="1"` satisfy their own keys. Two such rows inside one scope fail its
key. This setup also supplies the same identity-constraint **component** to
each recursive/sibling instance, which is necessary for C-012–C-014's upward
table propagation examples. Different constraints merely named alike are not
substitutes for the same definition. **V/P**.

### C-012 — propagate a child table and combine it with local tuples

In the recursive C-011 setup, add a keyref on `scope` selecting direct `use`
children. A child scope containing `r id="1"` and a parent `use rid="1"`
has a propagated K entry and the reference succeeds. Add local parent row
`r id="2"`: K's node table contains the local 2 and inherited 1, so references
to either can succeed. The inherited pair retains the selected child row's
node identity; it is not rebound to the parent element. **V/P**.

Keep that same K component and use optional xs:integer `id` attributes without
default/fixed constraints, integer `rid`, and otherwise admitted declarations/
content. The parent has one direct `r` whose `id` remains absent at field
evaluation after augmentation, a child scope with `r id="1"`, and `use rid="1"`.
The parent's own key fails field presence, but the child's K binding makes K
eligible under [`sic-key`](../../upstream/structures.html#sic-key). The local
qualified set is empty; the inherited `(1, child-row)` pair survives and the
parent keyref's individual reference predicate can match it. Remove the child:
the own-key failure remains, no prescribed K binding is available, and that
qualified reference cannot find its table. Both documents remain aggregate-
invalid from the missing local key field; their difference requires an
applicable table or individual-predicate observation, not binary validity.
Separately, an empty key selector satisfying its own key clause yields an
eligible binding with an empty table. These claims do not choose a table
winner when invalid local rows themselves contain duplicate key values.
Exposing the table is optional; no new inspection API is required. **V/P/O**.

Here is a complete schema for that contrast, with `R`'s intended referent
fixed to `K` as above. The recursive `ref="scope"` reuses the global element
declaration; its local `r` has no default/fixed value for `id`.

```xml
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="scope">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="r" minOccurs="0" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:integer" use="optional"/>
          </xs:complexType>
        </xs:element>
        <xs:element ref="scope" minOccurs="0" maxOccurs="unbounded"/>
        <xs:element name="use" minOccurs="0" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="rid" type="xs:integer" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="K">
      <xs:selector xpath="r"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:keyref name="R" refer="K">
      <xs:selector xpath="use"/>
      <xs:field xpath="@rid"/>
    </xs:keyref>
  </xs:element>
</xs:schema>
```

All instances below have no namespace. The table describes the outer `scope`;
inherited pairs retain the selected child row's node identity. These are
source-derived expectations, not executed corpus fixtures or observed PSVI.

| Instance | Outer K binding/table | Individual reference | Whole document |
| --- | --- | --- | --- |
| `<scope><scope><r id="1"/></scope><use rid="1"/></scope>` | Eligible; inherited integer 1 | Matches | Valid |
| `<scope><r/><scope><r id="1"/></scope><use rid="1"/></scope>` | Eligible through child; inherited integer 1 despite own-key failure | Matches | Invalid: missing local key field |
| `<scope><r/><use rid="1"/></scope>` | No prescribed K binding | Fails | Invalid: key and reference failures |
| `<scope><use rid="1"/></scope>` | Eligible empty table | Fails | Invalid: unmatched complete reference |

### C-013 — conflicting inherited entries are removed, not arbitrarily chosen

In C-012, use two child scopes whose respective rows both have ID 1, no local
parent row, and parent `use rid="1"`. Each child's key is individually valid;
their distinct row nodes conflict during upward propagation. Neither inherited
entry survives for 1, so the parent reference fails. A sole child entry would
succeed. One identical pair reached twice still denotes the same node and
does not create a distinct-node conflict. **V/P**.

### C-014 — a local entry wins over conflicting inherited candidates

Give the parent one direct `r id="1"` plus two child scopes each with a
distinct `r id="1"`. In the parent's K table, retain the local row and remove
the inherited conflicting candidates; parent `use rid="1"` succeeds. Add a
second **local** parent row with ID 1: local key validity now fails and cannot
be rescued by the conflict-resolution rule for inherited entries. Table
exposure itself is optional, but referential validity must behave as specified.
**V/P**.

### C-015 — identity declaration syntax and component properties

`<xs:key name="K"><xs:selector xpath="r"/><xs:field xpath="@id"/></xs:key>`
is the minimal one-field form in an element declaration. Omit the selector,
omit all fields, put a field before the selector, or omit `name`: the XML
representation fails. In abstract component input, require a nonempty field
sequence and a keyref-only referenced-key property; setting that property on
`unique`, or omitting it on `keyref`, violates the property tableau. Duplicate
identity-constraint names in the same namespace violate their single symbol
space even when attached to different declarations. **S/P**.

## Notations and annotations

Source: [notation declarations](../../upstream/structures.html#cNotation_Declarations)
and [annotations](../../upstream/structures.html#cAnnotations).

### C-020 — notation syntax and the component identifier requirement

Declare notation `n` with `public="image/example"`, system only, and both.
All three are admitted. The published 2004 representation summary and Appendix A
both make public and system individually optional. With neither identifier the XML
passes that syntax but the component is incorrect: its property tableau
requires at least one. A malformed name and duplicate expanded notation name
fail their own name/component rules. See [GC-017](../grammar-contexts.md)
for the source comparison. **S/P**.

### C-021 — NOTATION validation and PSVI alternatives

Restrict NOTATION by enumeration to `t:n`, with a declaration of that notation.
A matching QName value may validate; an undeclared notation or a QName outside
the enumeration cannot. A valid NOTATION attribute contributes either an
isomorphic notation item or the notation-system/notation-public summary pair
on its **parent element**. Do not require one exposure form. Two valid
NOTATION attributes are discouraged for compatibility; the chosen PSVI
notation in that case is unspecified, not a required schema rejection. **S/V/P**.

### C-022 — annotation payload, source URI, and ordering

Place `documentation` and `appinfo` containing text, foreign elements, and
an `xs:element` lookalike inside one `annotation`. The payload does not create
a global element declaration. Supply a valid relative `source` URI: it is
metadata, not a request to load that resource. A malformed `xml:lang` fails
the applicable representation type. Reorder appinfo/documentation children
where their grammar allows; move the annotation after a semantic child in a
construct requiring it first and the representation fails. An abstract
annotation keeps its ordered application/user information and associated
attributes; dropping those properties is a component observation failure.
**S/P**, with unchanged unrelated **V** behavior.

## Simple-type contracts at the Structures boundary

Source: [§3.14](../../upstream/structures.html#Simple_Type_Definitions),
[restriction](../../upstream/structures.html#cos-st-restricts), and
[derivation](../../upstream/structures.html#cos-st-derived-ok).
Part 2 owns the authoritative representation/type details; §3.14.1–.2 are
explicitly non-normative reproductions. These cases state their owning seam
rather than invent a second datatype implementation.

### C-030 — restriction/list alternatives and circular union references

`restriction base="xs:integer"` with no inline type is admitted. An inline
integer restriction with no `base` is the other admitted branch; both or
neither violate the exclusive choice. Apply the same pair to `list itemType`
versus its inline simple type. `U = union(xs:integer, xs:string)` is admitted;
`U = union(U, xs:string)` and `U = union(V); V = union(U)` are circular and
incorrect. A repeated member such as `union(xs:integer,xs:integer)` is not a
cycle by itself. **S**.

### C-031 — simple-type property validity, base termination, and final

Use a named integer restriction `T` with lower bound 0, and `R` restricting
T with lower bound 1. Its base chain terminates at decimal and the simple
ur-type. Mutate abstract bases into `T→R→T`: reject the circular component
graph. Set `T.final={restriction}` and derive R: reject the forbidden
restriction. An anonymous type with absent name is a legitimate component;
a missing required item/primitive/member property for its variety is not.
Do not equate an absent optional property with malformed component state.
**S/P**.

### C-032 — atomic base and facet applicability/restriction

An atomic restriction of integer with `minInclusive=0` is admitted; a
restriction claiming atomic variety while based on a list is an incorrect
abstract component. `length=2` on integer is inapplicable. For a string base
with `maxLength=5`, derived `maxLength=3` tightens the bound, 5 repeats it,
and 6 widens it illegally. A base fixed facet cannot be changed. Where XML
cannot construct the malformed variety/base combination, record its negative
as component input rather than pretending an XML fixture reaches it. **S/P**.

### C-033 — list construction, member variety, and item final

`L=list(xs:integer)` accepts `1 2`; `list(union(xs:integer,xs:boolean))`
accepts `1 true`. `list(L)` and a list whose item union contains L are
incorrect list-of-list constructions. With item type T having `final={list}`,
`list(T)` is forbidden even if its instance items validate as T. A newly
constructed list has the list whitespace facet, not a directly attached
length facet; add `length=2` by restricting L in a separate definition.
In abstract construction, extra constructor facets fail that constructor
branch independently of their usefulness on a later restriction. **S/V/P**.

### C-034 — list restriction preserves the list contract

Restrict `L=list(xs:integer)` with `length=2`: `1 2` validates, `1` does not.
If L already has `maxLength=3`, a further maximum 2 is admitted and 4 is an
illegal widening. A final-restriction L forbids the new type. In abstract
component input, a derived item `xs:int` can restrict integer; `xs:string`
cannot. Changing the derived base to an atomic type fails the list-base
condition. A list facet `totalDigits=2` is inapplicable, unlike its permitted
length/pattern/enumeration/whitespace facets. XML restriction normally inherits
its item type; changing that property is an abstract-only predicate witness.
**S/V/P**.

### C-035 — union construction and member flattening

`U=union(xs:integer,xs:string)` and a one-member union are admitted. Add an
inline boolean member after the `memberTypes` entries: its position follows
those entries. If an explicit member V is itself `union(xs:boolean,L)`, the
constructed member sequence replaces V with those members in order; lists
may be union members. A member with `final={union}` forbids this construction.
Constructor facets are empty; apply pattern/enumeration through a restriction
instead. An empty effective member sequence is incorrect. **S/P**, and **V**
using `1`, `true`, and a list literal to distinguish member spaces.

### C-036 — union restriction, member correspondence, and allowed facets

Restrict U by enumeration `1`: integer spelling `01` can denote the same
selected value under the applicable member, while an excluded value fails.
`length=1` on the union is inapplicable. A final-restriction U forbids a new
restriction. In abstract component input, base members `[integer,string]`
and derived `[int,token]` have the required corresponding derivations;
`[token,int]` does not satisfy that correspondence merely because both
types appear somewhere in the base union. A nonunion base fails this branch.
This abstract predicate is separate from XML's inherited member-sequence
mapping. **S/V/P**.

### C-037 — simple derivation is reflexive and checks every real step

For D=B=`xs:integer`, derivation succeeds even when the supplied prohibited
subset contains restriction: no step is traversed. For D=`xs:positiveInteger`
and B=`xs:integer`, an empty subset permits the direct restriction; a subset
containing restriction forbids it. Add intermediate restrictions T and R:
an indirect path must satisfy the same checks at every step. A list/union
can derive from the simple ur-type through the special branch. For
B=`union(xs:decimal,xs:string)`, integer can be derived through the decimal
member; boolean cannot. Ordinary unrelated types do not gain an ancestry
relation from overlapping lexical spellings. **S/P**, observable through
permitted type use where the XML representation exposes it.

### C-038 — effective facet composition

Restrict string first by `maxLength=5` and pattern `[a-z]+`; then restrict it
by `maxLength=3` and pattern `a.*`. The effective maximum is 3 and both
ancestral pattern steps apply: `abc` passes, `abcd` fails length, `zzz` fails
the derived pattern. Multiple patterns in the same restriction form their
specified alternative group; inherited pattern steps do not disappear.
Enumeration `[a,b]` followed by `[b]` retains only an admitted narrowing;
introducing `c` as though the earlier enumeration vanished is incorrect.
Variety and the atomic primitive remain those of the base. **S/V/P**.

### C-039 — ENTITY/ENTITIES require unparsed-entity infoset facts

Supply an infoset whose document has unparsed entity `logo` with notation
`gif`. ENTITY value `logo` passes the declared-entity check; `unknown` fails
despite being a valid NCName. A parsed entity of the same name does not
satisfy this predicate. ENTITIES `logo icon` needs both unparsed entities;
remove `icon` and it fails. Repeating the cases through restrictions preserves
the extra condition. A plain NCName `unknown` has no such entity requirement.
If an input policy cannot supply unparsed-entity facts, [Appendix D](../../upstream/structures.html#infoset)
says items of these types fail validation. Supporting that property is
recommended; the declared-entity predicates are still required. Do not invent
the missing declaration. **V/P**.

## Schema names, resolution, ID state, and component exposure

Source: [schema XML representation](../../upstream/structures.html#declare-schema),
[QName interpretation](../../upstream/structures.html#src-qname),
[schema resolution](../../upstream/structures.html#src-resolve), and
[instance resolution](../../upstream/structures.html#cvc-resolve-instance).

### C-050 — QName expansion has two independent name parts

With `p=urn:b`, schema QName `p:T` expands to `(urn:b,T)`. Rebinding only p
to `urn:c` changes the namespace, not local T. QName `T` uses the in-scope
default namespace if one exists, otherwise absent; the schema's
`targetNamespace="urn:a"` alone does not supply that default. `p:U` changes
only the local name. An unbound p fails namespace interpretation. Malformed
`p:T:U` fails QName lexical admission before component lookup. **S/P**.

### C-051 — component kind is part of resolution

Make distinct type, attribute, element, attribute-group, model-group, and
notation components all named `(urn:a,T)`. A reference at each of those six
sites selects its corresponding symbol space; it cannot resolve to the
element simply because that name was encountered first. The type space is
shared by simple and complex types. For instance-side component lookup,
repeat all six abstract requested kinds against that schema; missing local
name U or namespace `urn:c` fails to find a matching component. **S/P/V** at
the component consumers, not six new required instance syntaxes.

### C-052 — permission to reference a namespace is document-local

In a schema document with target `urn:a`, `t:T` can name its own namespace.
With no target namespace, unprefixed T and no default namespace can name a
no-namespace component. A target-`urn:a` document needs an import without
`namespace` to reference a no-namespace T; it needs `import namespace="urn:b"`
to reference p:T. Merely adding the foreign component to the global schema,
or placing the import only in a different including document, does not give
the reference-bearing document that permission. With the import but missing
component, distinguish permitted namespace from unsuccessful resolution.
**S/P**.

### C-053 — schema component uniqueness and property correctness

Two distinct global elements `(urn:a,e)` conflict. One element `(urn:a,e)`
and one type `(urn:a,e)` do not conflict across symbol spaces. A simple and
complex type both `(urn:a,T)` conflict in their shared space. Names with
different namespaces remain distinct. Including/importing the identical
component twice is not the same as constructing two distinct definitions
with equal names. For abstract schema properties, put an element into the
type-definition set: reject the malformed property assignment. Preserve
the missing-subcomponent qualification of §5.3 instead of treating every
absent optional property as an incorrect schema. **S/P**.

### C-054 — ID eligibility, list expansion, and declaration type

In one validation episode, validated attribute `id="a"` of type ID and
`ref="a"` of type IDREF contribute the string a; IDREFS `a b a` contributes
distinct table keys a and b, not a single key `"a b a"`. A derived ID
restriction preserves eligibility. A lexical-invalid ID is not successfully
validated and is not a successful eligibility witness. An unassessed item or
one with another validation context is outside this episode's eligible set.
A governing union which happens to select an ID member must be distinguished
from a declaration whose type itself is ID/derived-ID: the literal eligible
set predicate names the declaration type, not just a selected member. Selecting
an ID member alone therefore does not satisfy that declaration-type predicate.
**V/P**.

### C-055 — ID bindings point to the owning element

`<doc><item id="a"/><use ref="a"/></doc>` binds a to `item`. An ID-valued
child `<id>a</id>` makes its parent an owner under the binding definition;
the child value node and the owner node are distinct. A reference-only b
creates a table entry with no owner and fails the root's ID/IDREF condition.
Two different owner elements with a yield a two-owner binding and fail.
The binding is a **set of owner elements**: two qualifying ID-valued children
under the same owner do not create two distinct owners in that set. Keep
this literal rule distinct from an inferred global count of lexical ID
occurrences, and from separate restrictions on ID attribute uses. **V/P**.

### C-056 — validation roots isolate identity state

Assess `<doc><item id="a"/><sub><use ref="a"/></sub></doc>` as one document:
the reference can find a. Assess only `sub` as a separate validation root:
the outside ID is not in that episode's eligible set, so the reference has
no owner there. A prior validation episode's table cannot silently supply
current bindings. ID/IDREF and identity-constraint table structures need not
be exposed publicly; their specified validity effects remain required. **V/P/O**.

### C-057 — schema-information namespace and optional-detail partitions

Use a schema containing components for `urn:a`, `urn:b`, and absent
namespace. The validation root's schema-information set has an entry for
each, not only the namespace of the root element. A lightweight processor
may leave the schema-component detail set empty. If it provides that set,
it includes at least the named components actually used directly or through
anonymous contained components. For an in-memory source, document location
and document infoset may be absent; no fabricated path is required. A
no-target-namespace document contributing chameleon components is attributed
to the namespace to which it contributed them. **P**.

## Include, redefine, import, and location strategies

Source: [include](../../upstream/structures.html#src-include),
[redefine](../../upstream/structures.html#src-redefine),
[component replacement](../../upstream/structures.html#src-expredef),
[import](../../upstream/structures.html#src-import), and
[location strategies](../../upstream/structures.html#schema_reference).

### C-060 — successful source acquisition must yield an actual schema

Resolve an include to a well-formed `xs:schema` with valid declarations:
admit its components. Resolve it to `<notSchema/>`, malformed XML, or a
schema with an illegal declaration: successful byte retrieval does not
satisfy the schema referent condition. Supplying the same valid `schema`
information item directly is the other admitted representation. A fragment
identifying that item may be used; do not impose a mandatory whole-document
root or preferred media type absent a corresponding requirement. **S/O**.

### C-061 — include namespace matrix

For includer target `urn:a`, an included schema targeting `urn:a` is admitted,
one targeting `urn:b` is not, and a no-target-namespace schema is a chameleon
include. For an includer with no target, another no-target schema is admitted;
an included target-`urn:a` schema is not. Compare actual namespace values,
not prefix spellings or whether both documents use the same default namespace.
**S/P**.

### C-062 — include preserves existing components and adds the included set

Includer declares element a, included same-namespace document declares b:
both remain available. Repeat the include of the same document/component:
do not manufacture distinct duplicates merely from two directives. Two
different documents actually defining conflicting b components reach the
component-correctness error instead. Include a cycle of documents with
distinct valid declarations: traversal must reach the component-set semantics;
the cycle is not by itself a type/group derivation cycle. **S/P**.

### C-063 — chameleon adaptation includes qualified nested names and wildcards

A no-target schema declares global e, qualified local q, explicitly
unqualified local u, and an element wildcard constrained by `##local`.
Include it into `urn:a`: e and q acquire that target, u stays unqualified,
and the wildcard's absent namespace placeholder is adapted to `urn:a`.
Include the same source separately into `urn:b`: it contributes distinct
namespace-adapted components. The two contexts must not overwrite each
other's names or wildcard constraints. For a negated wildcard, adapt its
namespace operand too and retain the general wildcard matching conditions;
do not infer absent-namespace matching from set complement alone. **S/V/P**.

### C-064 — failure to locate an include is not a false schema referent

Keep the required `schemaLocation` attribute with a valid URI but let the
chosen strategy find no referent. The failed location attempt is not the
same schema error as C-060's retrieved nonschema. Missing components may
still prevent complete assessment when actually needed. Omit
`schemaLocation` itself: that is an independent representation error. An
operational resolver failure must not be relabeled proof that a valid
instance violates its datatype/content model. **S/O/P**.

### C-065 — redefine resolution depends on actual replacement children

A redefine containing a replacement type requires its location to resolve;
an unavailable referent violates that rule. A redefine containing only
annotation has no such successful-resolution requirement. If either form
does resolve, the referent must still be a valid schema, supplied as XML or
as a schema infoset item. A successfully retrieved nonschema is not excused
by having no replacements. **S/O**.

### C-066 — redefine namespace and unaffected-component preservation

Repeat C-061's same-target, both-absent, chameleon, and mismatched namespace
matrix for redefine. A base with types T and U, where only T is redefined,
retains U and replaces the named T. A no-target base redefined into `urn:a`
receives the same namespace adaptation as an include. A mismatch cannot be
repaired merely by giving the replacement type a matching prefix. **S/P**.

### C-067 — a redefined type derives from its own original component

Base T restricts string to maximum length 10. Redefine T by restriction with
`base="t:T"` and maximum length 5: the base reference means the original T,
not a cycle back to the replacement. Length 5 passes and 6 fails wherever
the resulting named T is used. A replacement simple type using `list` instead
of restriction, or using `base="t:Other"`, is not this permitted redefine.
A complex T may restrict or extend its original; using another base violates
the same self-base rule. **S/V/P**.

### C-068 — redefine visibility follows the replacement, not stale references

In the base document, element e has type t:T from C-067; a second component
also references T. After redefining T to maximum length 5, six-character
content of e fails even though e's source was not edited. The original
type survives as the replacement's unnamed base; it is not a second public
type still resolved by t:T. A reference from the redefining document likewise
selects the replacement except at the special base/self-reference position.
**S/V/P**.

Reach the same original document through direct, repeated and diamond paths
with equivalent replacements in the same adopted namespace. Once the
individual redefinitions' identity is established, equivalent same-way
repetition does not itself create a duplicate-component violation. Change
one replacement or its adopted context: equivalence is no longer established
by the repeated source URI, and the actual identity/derivation constraints
must decide the result. Preserve S-008's open identity cases; no general
first-wins/last-wins or structural-interning rule follows. **S/P/O**.

### C-069 — self-referencing model-group redefine uses exactly one unit occurrence

Base group G is sequence(a). A replacement sequence(group ref t:G, b)
with the self-reference at occurrence 1..1 uses the original group and can
admit a,b. Two self-references violate the uniqueness condition. Change the
self-reference to 0..1, 1..2, or 0..unbounded: each violates the required unit
bounds. Omitted occurrence attributes mean 1..1. A reference to unrelated H
does not count as G's self-reference. **S/V**.

### C-070 — model-group redefine without self-reference is a restriction

Base G is choice(a,b). Replace G with choice(a): it is a valid restricting
candidate and admits a while excluding b. Replace it with choice(c): it is
not a valid restriction of that base. A replacement with no original G to
resolve is incorrect even when its own model is otherwise valid. Use the
actual particle restriction rules, not unrestricted textual replacement.
**S/V**.

### C-071 — attribute-group redefine has two distinct paths

Base group A has optional a. A replacement referencing original A once and
adding optional b uses the self-reference path; two such A references fail.
Without self-reference, replacing A by an empty group can remove optional a;
it does not inherit a or its default automatically. If a was required,
dropping it is not a valid restriction. A replacement requiring original A
when no such base exists fails. Restrict its wildcard using the same subset
and process-contents rules as complex restriction. **S/V/P**.

A no-self replacement referencing a separate group H has a different mapping
question from inheritance of omitted original members. Preserve
[CMP-Q002](../source-questions.md#cmp-q002--restrictive-attribute-group-redefine-and-nonself-references)
before assigning a result from the explicit-only note; the ordinary examples
above retain their stated restriction rules.

### C-072 — import namespace and target matrix

In target `urn:a`, `import namespace="urn:b"` is permitted; importing
`urn:a` as a different namespace is not. Import without `namespace` is
permitted only when the enclosing schema has a target namespace. A resolved
`namespace="urn:b"` import must supply a schema targeting `urn:b`; one
targeting `urn:c` or absent is a mismatch. An import without namespace must
resolve to a no-target schema. **S**.

For unused `namespace=""` with no referent, retain
[CMP-Q001](../source-questions.md#cmp-q001--unused-present-empty-import-namespace).
Present-empty differs from absent; ordinary anyURI lexical admission does not
settle this semantic-domain scope question.

### C-073 — import availability, hints, and component union

With a permitted namespace import, no `schemaLocation` is required. A
catalogue or explicitly supplied source can provide the components. If the
strategy supplies a referent, validate it as in C-060 and add its component
set to the importing schema. A strategy supplying none is not itself a
schema error; unresolved uses remain distinct. Reimporting identical
components does not duplicate them. A processor may ignore later location
hints for a namespace, so two hints alone do not guarantee both resources
were used. Record the chosen source strategy when describing the outcome.
**S/O/P**.

### C-074 — each location strategy is permitted, not an obligatory network fetch

For namespace `urn:b` and hint `schemas/b.xsd`, describe five configurations:
use known components without lookup; look up the hint in a local catalogue;
look up `urn:b` in that catalogue; resolve the hint as a Web resource; or
resolve the namespace name. Any selected combination/order is permitted.
The repository's explicit-source processor can take the first path. A test
requiring network traffic merely because the hint exists has no such oracle.
Configuration options are recommended when possible, not a new validity rule.
**O/P**.

Under the applicable Web convention, no contrary application/CLI direction
also carries the separate **should-attempt-each-location** recommendation.
An override of that recommendation is not itself a waiver of applicable hint
timing rules. Use [IP-018](../interactions.md#ip-018--invocationconformance--acquisition--root--optional-observations)
for the full root/namespace/element-or-attribute/order/direction product.
Successful retrieval and component availability require their own evidence;
neither a hint nor this recommendation mandates a physical request count.

### C-075 — instance hints do not rename namespaces or repair schema errors

`xsi:schemaLocation="urn:b b.xsd"` supplies a namespace/location pair; the
hint's namespace need not equal the containing element's namespace. Multiple
pairs are admitted; an odd number is not the required pair list. A
`noNamespaceSchemaLocation` supplies one URI for absent namespace, not a
new default namespace. Relative locations use the owning base context.
Ignoring a hint cannot excuse a known incorrect schema or make an unknown
declaration a known one. **V/O**, with XML/XSI lexical admission separated
from optional source discovery.

## Assessment and appendix observations

Source: [§5](../../upstream/structures.html#conformance),
[missing components](../../upstream/structures.html#conformance-missing), and
[Appendices](../../upstream/structures.html#appendices).

### C-080 — three assessment entrypoints have different premises

Supply a known complex type T admitting one integer child x and assess
`<unknown><x>1</x></unknown>` directly against T: a global declaration named
unknown is not required by that entrypoint. Supply a declaration named e
instead: assessing `<other/>` against that declaration fails its name-match
premise. Start with no stipulated declaration/type: a declared e can be
strictly assessed, an unknown root with no usable type is lax/notKnown, and
an unknown root with a resolvable suitable `xsi:type` can obtain a governing
type through the specified assessment rules. **V/P/O**.

### C-081 — schema errors are not instance-invalid witnesses

For [S-072](../structures.md#s-072--schema-errors-and-validation-diagnostics)'s
schema-for-schemas boundary, supply the correct schema-for-schemas components.
Compare `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>` with the
same element carrying `xmlns:n="urn:n" n:note="x"`, with no declaration for
`{urn:n}note`. Its `##other`/lax attribute wildcard admits that attribute;
the unknown attribute is notKnown/none. Under the applicable-route reading
qualified by [ASQ001](../source-questions.md#asq001--element-assessment-outcome-route-connective),
the bare schema root is valid/full and the foreign-attribute root valid/partial.
Both satisfy §5.1's stated schema-for-schemas condition. Contrast an invalid
source, such as an undeclared unqualified attribute on the schema element.
The condition describes what assessment would yield; an implementation need
not perform a separate validation pass. No actual assessment result means
that result is unobserved, not that the source is necessarily schema-invalid.

Try to assess an instance after constructing a schema with an illegal
duplicate type, a representation-invalid declaration, or a component-invalid
restriction. A processor must not perform coherent schema assessment using
those nonschemas. No particular subsequent schema-construction recovery or
diagnostic ordering is mandated. Only after a correct schema is established
can an instance-invalid result prove its particular validation rule. **S/O**.

### C-082 — missing required subcomponents prevent full assessment

At the abstract schema boundary, take an otherwise admitted element whose
type reference is missing, or an attribute whose required type subcomponent
is absent after failed resolution. When assessment reaches that component,
apply §5.3's modified absent-declaration failure; it cannot report full
assessment for the whole document. Element assessment may continue laxly as
permitted. An unused unavailable component is not evidence that this exact
runtime path was reached. This case does not mandate that every product
accept a partially resolved schema at its compile API. **V/P/O**.

### C-083 — infoset and PSVI episode boundaries

Preserve input element/attribute/character/namespace facts required by
Appendix D. Applied element defaults create no text children (E1-56);
defaulted attributes are additive under their own rules. Every assessed
item's validation context points to the chosen validation root, which may
be a subtree. A second assessment of a previous PSVI is not given an
additional universal multi-episode contract by §5.2; do not turn incidental
retention behavior into a new XSD requirement. **P/O**.

### C-084 — Appendix A syntax is necessary but insufficient

`<xs:element name="e" type="xs:integer"/>` inside a schema satisfies its
basic declaration grammar. Add both `default` and `fixed`: even if each
attribute individually has an admitted string value, the representation
constraint rejects their combination. Conversely, DTD validity in the
non-normative Appendix G does not establish QName resolution, derivation,
or content-model correctness. Use the [grammar matrix](../grammar.md) for
each applicable Appendix A context, not this example as a blanket proof.
**S**.

### C-085 — appendix authority and non-executable material

Structures A–D retain their normative status. Appendix C is an independent
constraint/property enumeration; it does not replace the rule bodies.
Appendix D's missing unparsed-entity facts have C-039's consequences.
E (diagram), F (glossary), G (DTD), H (UPA analysis), I (informative references),
and J (acknowledgements) do not create additional mandatory rejections.
For H, use concrete overlapping element/wildcard/substitution cases from the
content cases to illustrate the normative UPA rule, keeping the explanatory
algorithm separate. This is a source classification, not an invented invalid
XML witness. **P/O/informative**.

## Named rules without numbered branches

| Source constraint | Concrete cases | Decision |
| --- | --- | --- |
| `src-identity-constraint` | C-015, C-010 | Grammar admission also requires correct identity-component properties. |
| `src-notation` | C-020 | The XML notation grammar and component constraints both apply. |
| `sic-notation-used` | C-021 | Valid NOTATION attribute contributes one of the permitted parent-element notation representations. |
| `n-props-correct` | C-020 | Public/system identifier alternatives at the abstract component boundary. |
| `src-annotation` | C-022 | Correct annotation representation includes the owning property constraints. |
| `an-props-correct` | C-022 | Ordered application/user information and associated attributes remain correctly represented. |
| `sic-schema` | C-057 | Root schema-information namespace groups and optional component/document detail. |

## Numbered-branch disposition index

Keys below are the inventory's source-order identifiers, **not** additional
normative clause numbers. `composite` means a parent condition whose concrete
conjunct/alternative witnesses are given by the cited cases. `informative`
also accounts for explicitly non-normative reproductions and empty numbering
slots; its reason states which. A `reviewed` entry establishes a described
scenario, not executed corpus coverage. An unresolved premise stays visible.

| Inventory key | Concrete cases | Disposition | Source/observation note |
| --- | --- | --- | --- |
| `structures/cvc-identity-constraint/branch/001` | C-001 | reviewed | Clause 1. |
| `structures/cvc-identity-constraint/branch/002` | C-001 | reviewed | Clause 2. |
| `structures/cvc-identity-constraint/branch/003` | C-004, C-005, C-009 | question | Clause 3. C-009 retains the absent typed-value interpretation question. |
| `structures/cvc-identity-constraint/branch/004` | C-006, C-007, C-008, C-009, C-010 | question | Clause 4. C-009 retains the absent typed-value interpretation question. |
| `structures/cvc-identity-constraint/branch/005` | C-007 | reviewed | Clause 4.1. |
| `structures/cvc-identity-constraint/branch/006` | C-006, C-007, C-008 | composite | Clause 4.2. |
| `structures/cvc-identity-constraint/branch/007` | C-006 | reviewed | Clause 4.2.1. |
| `structures/cvc-identity-constraint/branch/008` | C-007 | reviewed | Clause 4.2.2. |
| `structures/cvc-identity-constraint/branch/009` | C-008 | reviewed | Clause 4.2.3. |
| `structures/cvc-identity-constraint/branch/010` | C-010, C-012, C-013, C-014 | reviewed | Clause 4.3. |
| `structures/sic-key/branch/001` | C-012, C-013, C-014 | reviewed | Clause 1. |
| `structures/sic-key/branch/002` | C-012, C-014 | reviewed | Clause 2. |
| `structures/c-props-correct/branch/001` | C-015 | reviewed | Clause 1. |
| `structures/c-props-correct/branch/002` | C-010 | reviewed | Clause 2. |
| `structures/c-selector-xpath/branch/001` | C-002 | reviewed | Clause 1. |
| `structures/c-selector-xpath/branch/002` | C-002 | composite | Clause 2. |
| `structures/c-selector-xpath/branch/003` | C-002 | reviewed | Clause 2.1. |
| `structures/c-selector-xpath/branch/004` | C-002 | reviewed | Clause 2.2. |
| `structures/c-fields-xpaths/branch/001` | C-003 | reviewed | Clause 1. |
| `structures/c-fields-xpaths/branch/002` | C-003 | composite | Clause 2. |
| `structures/c-fields-xpaths/branch/003` | C-003 | reviewed | Clause 2.1. |
| `structures/c-fields-xpaths/branch/004` | C-003 | reviewed | Clause 2.2. |
| `structures/declare-datatype/branch/001` | C-030 | informative | Clause 1. Non-normative reproduction; Part 2 owns the rule. |
| `structures/declare-datatype/branch/002` | C-033, C-035 | informative | Clause 2. Non-normative reproduction; Part 2 owns the rule. |
| `structures/declare-datatype/branch/003` | C-033 | informative | Clause 1. Non-normative reproduction; Part 2 owns the rule. |
| `structures/declare-datatype/branch/004` | C-034 | informative | Clause 2. Non-normative reproduction; Part 2 owns the rule. |
| `structures/declare-datatype/branch/005` | C-035 | informative | Clause 1. Non-normative reproduction; Part 2 owns the rule. |
| `structures/declare-datatype/branch/006` | C-036 | informative | Clause 2. Non-normative reproduction; Part 2 owns the rule. |
| `structures/src-simple-type/branch/001` | C-031, C-032, C-033, C-034, C-035, C-036 | reviewed | Clause 1. |
| `structures/src-simple-type/branch/002` | C-030 | reviewed | Clause 2. |
| `structures/src-simple-type/branch/003` | C-030 | reviewed | Clause 3. |
| `structures/src-simple-type/branch/004` | C-030 | reviewed | Clause 4. |
| `structures/cvc-simple-type/branch/001` | C-032, C-034, C-036 | reviewed | Clause 1. |
| `structures/cvc-simple-type/branch/002` | C-039 | composite | Clause 2. |
| `structures/cvc-simple-type/branch/003` | C-039 | reviewed | Clause 2.1. |
| `structures/cvc-simple-type/branch/004` | C-039 | reviewed | Clause 2.2. |
| `structures/cvc-simple-type/branch/005` | C-039 | reviewed | Clause 2.3. |
| `structures/st-props-correct/branch/001` | C-031 | reviewed | Clause 1. |
| `structures/st-props-correct/branch/002` | C-031 | reviewed | Clause 2. |
| `structures/st-props-correct/branch/003` | C-031 | reviewed | Clause 3. |
| `structures/cos-st-restricts/branch/001` | C-032 | composite | Clause 1. |
| `structures/cos-st-restricts/branch/002` | C-032 | reviewed | Clause 1.1. |
| `structures/cos-st-restricts/branch/003` | C-031 | reviewed | Clause 1.2. |
| `structures/cos-st-restricts/branch/004` | C-032 | composite | Clause 1.3. |
| `structures/cos-st-restricts/branch/005` | C-032 | reviewed | Clause 1.3.1. |
| `structures/cos-st-restricts/branch/006` | C-032 | reviewed | Clause 1.3.2. |
| `structures/cos-st-restricts/branch/007` | C-033, C-034 | composite | Clause 2. |
| `structures/cos-st-restricts/branch/008` | C-033 | reviewed | Clause 2.1. |
| `structures/cos-st-restricts/branch/009` | C-033 | informative | Empty current numbering slot; no operative condition remains. |
| `structures/cos-st-restricts/branch/010` | C-033, C-034 | composite | Clause 2.3. |
| `structures/cos-st-restricts/branch/011` | C-033 | composite | Clause 2.3.1. |
| `structures/cos-st-restricts/branch/012` | C-033 | reviewed | Clause 2.3.1.1. |
| `structures/cos-st-restricts/branch/013` | C-033 | reviewed | Clause 2.3.1.2. |
| `structures/cos-st-restricts/branch/014` | C-034 | composite | Clause 2.3.2. |
| `structures/cos-st-restricts/branch/015` | C-034 | reviewed | Clause 2.3.2.1. |
| `structures/cos-st-restricts/branch/016` | C-034 | reviewed | Clause 2.3.2.2. |
| `structures/cos-st-restricts/branch/017` | C-034 | reviewed | Clause 2.3.2.3. |
| `structures/cos-st-restricts/branch/018` | C-034 | reviewed | Clause 2.3.2.4. |
| `structures/cos-st-restricts/branch/019` | C-034 | reviewed | Clause 2.3.2.5. |
| `structures/cos-st-restricts/branch/020` | C-035, C-036 | composite | Clause 3. |
| `structures/cos-st-restricts/branch/021` | C-035 | reviewed | Clause 3.1. |
| `structures/cos-st-restricts/branch/022` | C-035 | informative | Empty current numbering slot; no operative condition remains. |
| `structures/cos-st-restricts/branch/023` | C-035, C-036 | composite | Clause 3.3. |
| `structures/cos-st-restricts/branch/024` | C-035 | composite | Clause 3.3.1. |
| `structures/cos-st-restricts/branch/025` | C-035 | reviewed | Clause 3.3.1.1. |
| `structures/cos-st-restricts/branch/026` | C-035 | reviewed | Clause 3.3.1.2. |
| `structures/cos-st-restricts/branch/027` | C-036 | composite | Clause 3.3.2. |
| `structures/cos-st-restricts/branch/028` | C-036 | reviewed | Clause 3.3.2.1. |
| `structures/cos-st-restricts/branch/029` | C-036 | reviewed | Clause 3.3.2.2. |
| `structures/cos-st-restricts/branch/030` | C-036 | reviewed | Clause 3.3.2.3. |
| `structures/cos-st-restricts/branch/031` | C-036 | reviewed | Clause 3.3.2.4. |
| `structures/cos-st-restricts/branch/032` | C-036 | reviewed | Clause 3.3.2.5. |
| `structures/cos-st-derived-ok/branch/001` | C-037 | reviewed | Clause 1. |
| `structures/cos-st-derived-ok/branch/002` | C-037 | composite | Clause 2. |
| `structures/cos-st-derived-ok/branch/003` | C-037 | reviewed | Clause 2.1. |
| `structures/cos-st-derived-ok/branch/004` | C-037 | composite | Clause 2.2. |
| `structures/cos-st-derived-ok/branch/005` | C-037 | reviewed | Clause 2.2.1. |
| `structures/cos-st-derived-ok/branch/006` | C-037 | reviewed | Clause 2.2.2. |
| `structures/cos-st-derived-ok/branch/007` | C-037 | reviewed | Clause 2.2.3. |
| `structures/cos-st-derived-ok/branch/008` | C-037 | reviewed | Clause 2.2.4. |
| `structures/st-restrict-facets/branch/001` | C-038 | reviewed | Clause 1. |
| `structures/st-restrict-facets/branch/002` | C-038 | reviewed | Clause 2. |
| `structures/st-restrict-facets/branch/003` | C-038 | reviewed | Clause 3. |
| `structures/src-qname/branch/001` | C-050 | composite | Clause 1. |
| `structures/src-qname/branch/002` | C-050 | reviewed | Clause 1.1. |
| `structures/src-qname/branch/003` | C-050 | reviewed | Clause 1.2. |
| `structures/src-qname/branch/004` | C-050 | reviewed | Clause 1.3. |
| `structures/src-qname/branch/005` | C-050 | composite | Clause 2. |
| `structures/src-qname/branch/006` | C-050 | reviewed | Clause 2.1. |
| `structures/src-qname/branch/007` | C-050 | composite | Clause 2.2. |
| `structures/src-qname/branch/008` | C-050 | reviewed | Clause 2.2.1. |
| `structures/src-qname/branch/009` | C-050 | reviewed | Clause 2.2.2. |
| `structures/src-resolve/branch/001` | C-051, C-052 | composite | Clause 1. |
| `structures/src-resolve/branch/002` | C-051 | reviewed | Clause 1.1. |
| `structures/src-resolve/branch/003` | C-051 | reviewed | Clause 1.2. |
| `structures/src-resolve/branch/004` | C-051 | reviewed | Clause 1.3. |
| `structures/src-resolve/branch/005` | C-051 | reviewed | Clause 1.4. |
| `structures/src-resolve/branch/006` | C-051 | reviewed | Clause 1.5. |
| `structures/src-resolve/branch/007` | C-051 | reviewed | Clause 1.6. |
| `structures/src-resolve/branch/008` | C-050, C-051 | reviewed | Clause 2. |
| `structures/src-resolve/branch/009` | C-050, C-051 | reviewed | Clause 3. |
| `structures/src-resolve/branch/010` | C-052 | composite | Clause 4. |
| `structures/src-resolve/branch/011` | C-052 | composite | Clause 4.1. |
| `structures/src-resolve/branch/012` | C-052 | reviewed | Clause 4.1.1. |
| `structures/src-resolve/branch/013` | C-052 | reviewed | Clause 4.1.2. |
| `structures/src-resolve/branch/014` | C-052 | composite | Clause 4.2. |
| `structures/src-resolve/branch/015` | C-052 | reviewed | Clause 4.2.1. |
| `structures/src-resolve/branch/016` | C-052 | reviewed | Clause 4.2.2. |
| `structures/cvc-resolve-instance/branch/001` | C-051 | reviewed | Clause 1. |
| `structures/cvc-resolve-instance/branch/002` | C-051 | reviewed | Clause 2. |
| `structures/cvc-resolve-instance/branch/003` | C-051 | reviewed | Clause 3. |
| `structures/cvc-resolve-instance/branch/004` | C-051 | reviewed | Clause 4. |
| `structures/cvc-resolve-instance/branch/005` | C-051 | reviewed | Clause 5. |
| `structures/cvc-resolve-instance/branch/006` | C-051 | reviewed | Clause 6. |
| `structures/sic-id/branch/001` | C-054, C-056 | reviewed | Clause 1. |
| `structures/sic-id/branch/002` | C-054 | reviewed | Clause 2. |
| `structures/sic-id/branch/003` | C-054 | reviewed | Clause 1. |
| `structures/sic-id/branch/004` | C-054 | reviewed | Clause 2. |
| `structures/sic-id/branch/005` | C-055, C-056 | reviewed | Clause 1. |
| `structures/sic-id/branch/006` | C-055 | reviewed | Clause 2. |
| `structures/sch-props-correct/branch/001` | C-053 | reviewed | Clause 1. |
| `structures/sch-props-correct/branch/002` | C-053 | reviewed | Clause 2. |
| `structures/src-include/branch/001` | C-060, C-064 | composite | Clause 1. |
| `structures/src-include/branch/002` | C-060 | reviewed | Clause 1.1. |
| `structures/src-include/branch/003` | C-060 | reviewed | Clause 1.2. |
| `structures/src-include/branch/004` | C-061 | composite | Clause 2. |
| `structures/src-include/branch/005` | C-061 | reviewed | Clause 2.1. |
| `structures/src-include/branch/006` | C-061 | reviewed | Clause 2.2. |
| `structures/src-include/branch/007` | C-061, C-063 | reviewed | Clause 2.3. |
| `structures/src-include/branch/008` | C-062, C-063 | composite | Clause 3. |
| `structures/src-include/branch/009` | C-062 | reviewed | Clause 3.1. |
| `structures/src-include/branch/010` | C-063 | composite | Clause 3.2. |
| `structures/src-include/branch/011` | C-063 | reviewed | Clause 3.2.1. |
| `structures/src-include/branch/012` | C-063 | reviewed | Clause 3.2.2. |
| `structures/src-redefine/branch/001` | C-065 | reviewed | Clause 1. |
| `structures/src-redefine/branch/002` | C-065 | composite | Clause 2. |
| `structures/src-redefine/branch/003` | C-065 | reviewed | Clause 2.1. |
| `structures/src-redefine/branch/004` | C-065 | reviewed | Clause 2.2. |
| `structures/src-redefine/branch/005` | C-066 | composite | Clause 3. |
| `structures/src-redefine/branch/006` | C-066 | reviewed | Clause 3.1. |
| `structures/src-redefine/branch/007` | C-066 | reviewed | Clause 3.2. |
| `structures/src-redefine/branch/008` | C-066 | reviewed | Clause 3.3. |
| `structures/src-redefine/branch/009` | C-066 | composite | Clause 4. |
| `structures/src-redefine/branch/010` | C-066 | reviewed | Clause 4.1. |
| `structures/src-redefine/branch/011` | C-066, C-063 | reviewed | Clause 4.2. |
| `structures/src-redefine/branch/012` | C-067 | reviewed | Clause 5. |
| `structures/src-redefine/branch/013` | C-069, C-070 | composite | Clause 6. |
| `structures/src-redefine/branch/014` | C-069 | composite | Clause 6.1. |
| `structures/src-redefine/branch/015` | C-069 | reviewed | Clause 6.1.1. |
| `structures/src-redefine/branch/016` | C-069 | reviewed | Clause 6.1.2. |
| `structures/src-redefine/branch/017` | C-070 | composite | Clause 6.2. |
| `structures/src-redefine/branch/018` | C-070 | reviewed | Clause 6.2.1. |
| `structures/src-redefine/branch/019` | C-070 | reviewed | Clause 6.2.2. |
| `structures/src-redefine/branch/020` | C-071 | composite | Clause 7. |
| `structures/src-redefine/branch/021` | C-071 | reviewed | Clause 7.1. |
| `structures/src-redefine/branch/022` | C-071 | composite | Clause 7.2. |
| `structures/src-redefine/branch/023` | C-071 | reviewed | Clause 7.2.1. |
| `structures/src-redefine/branch/024` | C-071 | reviewed | Clause 7.2.2. |
| `structures/src-expredef/branch/001` | C-067, C-068 | composite | Clause 1. |
| `structures/src-expredef/branch/002` | C-068 | reviewed | Clause 1.1. |
| `structures/src-expredef/branch/003` | C-067, C-068 | reviewed | Clause 1.2. |
| `structures/src-expredef/branch/004` | C-069, C-071 | reviewed | Clause 2. |
| `structures/src-import/branch/001` | C-072 | composite | Clause 1. |
| `structures/src-import/branch/002` | C-072 | reviewed | Clause 1.1. |
| `structures/src-import/branch/003` | C-072 | reviewed | Clause 1.2. |
| `structures/src-import/branch/004` | C-073 | composite | Clause 2. |
| `structures/src-import/branch/005` | C-073 | reviewed | Clause 2.1. |
| `structures/src-import/branch/006` | C-073 | reviewed | Clause 2.2. |
| `structures/src-import/branch/007` | C-072 | composite | Clause 3. |
| `structures/src-import/branch/008` | C-072 | reviewed | Clause 3.1. |
| `structures/src-import/branch/009` | C-072 | reviewed | Clause 3.2. |
| `structures/schema_reference/branch/001` | C-074 | reviewed | Clause 1. |
| `structures/schema_reference/branch/002` | C-074 | reviewed | Clause 2. |
| `structures/schema_reference/branch/003` | C-074 | reviewed | Clause 3. |
| `structures/schema_reference/branch/004` | C-074 | reviewed | Clause 4. |
| `structures/schema_reference/branch/005` | C-074 | reviewed | Clause 5. |
| `structures/validation_outcome/branch/001` | C-080 | reviewed | Clause 1. |
| `structures/validation_outcome/branch/002` | C-080 | reviewed | Clause 2. |
| `structures/validation_outcome/branch/003` | C-080 | reviewed | Clause 3. |

## Remaining observation boundaries

The index does not claim executable evidence, complete expansion of every
Appendix A path, or corpus coverage. C-009 identifies an unresolved nilled-field
value premise; source-specific Unicode/URI/temporal questions remain in the
datatype and boundary catalogues. Schema-location strategies, abstract component
mutation, optional internal tables, and full PSVI need their named observing
boundaries. A validity-only result cannot certify those properties.
