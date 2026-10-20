# Structures §§3.4–3.10: concrete branch audit

This is a non-normative, specification-first audit of complex types, attribute
uses/groups, model groups, particles, and wildcards. Its authority is the
unchanged [Second Edition](../../upstream/structures.html), read with the
[correction record](../../corrected-reading.md). No implementation or corpus
result supplies an expected outcome. The [catalogue method](../README.md)
distinguishes a branch description from an executed fixture.

The disposition index covers all **282** numbered source branches assigned to
§§3.4–3.10 in [inventory.json](../../inventory.json). Its `/branch/NNN` suffix is
a source-order inventory ordinal, **not** a normative clause number. Parent AND,
OR and case containers are marked `composite`; their cited cases distinguish
the child premises. `reviewed` means a concrete distinguishing description was
written, not that a processor was executed. Unnumbered normative conditions and
property mappings have their own index after the cases.

## Shared exact sketches and observation boundaries

Use XML namespaces `xs=http://www.w3.org/2001/XMLSchema`, `t=urn:t`, `u=urn:u`,
and `v=urn:v`. Unless stated otherwise a schema has targetNamespace `urn:t`,
elementFormDefault=`qualified`, attributeFormDefault=`unqualified`, empty
blockDefault/finalDefault. Local element names `a,b,c,d` mean `{urn:t}a` etc.;
attribute names `x,y,z` have absent namespace. A root is `{urn:t}r` declared with
the type under examination. Every unspecified declaration is concrete,
non-nillable, has no value/identity/substitution constraint or annotation, and
uses `xs:string`. Every unspecified range is explicitly **1..1**.

`e(a,m,n)` is a particle with that range and a **local** declaration for a.
`g(a,m,n)` uses the one globally named declaration for a. `seq([...],m,n)`,
`choice([...],m,n)` and `all([...],m,n)` are group particles with the listed
ordered children; the order is part of the component even for all. `W(S,p,m,n)`
is a wildcard particle with namespace constraint S and processContents p.
`A` admits all namespaces including absent; `N(t)` excludes **both urn:t and
absent**; `N(∅)` admits every present namespace; `{t,u,∅}` is an explicit set
containing urn:t, urn:u and absent. `∅` denotes the absent namespace; `{}` denotes
the **empty namespace set**, not an absent wildcard. `∞` is unbounded.

`CT(name;content;attrs;wildcard)` is a complete complex component. Content is
`empty`, a named simple type, `EO(P)` or `mixed(P)`. Defaults are base anyType,
derivation restriction, final/block empty, abstract false, annotations empty;
all non-root user base chains terminate in the predefined anyType. Types `B`,
`R`, `D` are distinct named `{urn:t}` types unless identity is stated. A use
`x:T?` is optional, `x:T!` required, `x:T=default("v")` or `fixed("v")` supplies
its use constraint. Its declaration has absent value constraint unless stated.
`AG(G;uses;wildcard)` and `MG(G;group)` are named group definitions. Attribute
uses inherited by identity remain the same uses; a newly written local attribute
creates its own declaration. `SInt` is a named restriction of xs:integer with
minInclusive=0. `SStr` restricts xs:string with maxLength=3. `K` is one named
identity-constraint component; `{K}` means identity membership, not a fresh copy.

A component-relation case tests the named relation on R/B without first putting
it inside another invalid schema. **Rel+ / Rel−** mean that relation holds/fails;
they are not assertions that arbitrary standalone XML is schema-valid.
**SC+ / SC−** mean schema construction succeeds/fails; **V+ / V−** are instance
validity; **CM** is the mapped component/property observation; **P** is a named
PSVI property. Where an XML wrapper would violate another constraint first,
keep the component-relation oracle separate and record that fixture seam.
No expected diagnostic spelling/order follows from these descriptions.

## Complex mappings and validation

### B-001 — effective block and final controls

[§3.4.2](../../upstream/structures.html#declare-type). Construct `<complexType
name="R" block=""/>` under `blockDefault="#all"`: CM prohibited substitutions
is `{}`. Omit block: CM is `{extension,restriction}`. Set block=`restriction`:
CM is `{restriction}`. Set schema blockDefault=`substitution`: CM is `{}` for
complex types. Repeat with final/finalDefault, including defaults
`list union restriction`: CM final is `{restriction}`. An explicit empty final
overrides a nonempty finalDefault. These are SC+ mappings, not rejection cases.

### B-002 — complex metadata and non-inherited finality

[§3.4.1–2](../../upstream/structures.html#Complex_Type_Definition_details).
Global `<complexType name="B" final="extension"/>` maps name B, namespace urn:t;
an anonymous type inside r has absent name but still namespace urn:t. A
no-targetNamespace document makes the type namespace absent. Derive R by
restriction from B, with no final attribute and empty schema finalDefault:
R.final is empty, despite B.final containing extension. R can then be extended
where all other constraints hold. abstract omitted/false maps false; true maps
true and prevents direct type assessment. Put annotations at complexType,
complexContent/simpleContent, and derivation children: CM annotations contains
the corresponding set; removing one removes that annotation only. With no
source annotation, preserve the [CTS-Q001](../source-questions.md#cts-q001--complex-type-annotations-with-no-source-annotation)
set-versus-absence question. A named type R does not require the instance element
to be named R: `<t:r/>` is V+ for its
empty type, while `<t:R/>` lacks r's declaration under declaration-led lookup.

### B-003 — local attributes in both content paths

[§3.4.2](../../upstream/structures.html#declare-type). B is a complex type with
simple content xs:string and no attributes. Define R by simpleContent extension
with `<attribute name="x" type="xs:integer"/>`: CM has exactly one optional
x:integer use, content xs:string; `<t:r x="2">s</t:r>` V+, x=`s` V−.
Repeat with B empty and complexContent extension: R has empty content and the
same x use; `<t:r x="2"/>` V+. Omitting the attribute gives no local use in each
mapping. This separates the two source occurrences of the same mapping rule.

### B-004 — referenced attribute group in both content paths

[§§3.4.2,3.6.2](../../upstream/structures.html#declare-attributeGroup).
`AG(G;{x:integer?,y:string!};absent)`. Replace B-003's local x with
`<attributeGroup ref="t:G"/>` in simpleContent and complexContent. Each CM
contains G's two uses; `<t:r y="a"/>` is V+ for the complex empty-content path,
missing y V−, and x=`bad` V− when present. The reference does not create a
runtime wrapper or a new independent attribute-name scope.

### B-005 — inherit, replace and prohibit uses

[§3.4.2](../../upstream/structures.html#declare-type). Use B with
`{x:integer?,y:string?}` and either simple xs:string content or empty content.
Extension adding z:string? retains x and y by identity. Restriction explicitly
redeclaring x:SInt replaces the inherited x but inherits y; x=`-1` becomes V−,
x=`0` V+. Restriction `<attribute name="x" use="prohibited"/>` removes x:
no-x instance V+, x present V− with no wildcard. Replace x through a referenced
AG containing x:SInt and observe the same replacement. An explicit z with a
different expanded name does not suppress x. Repeat each mutation for both
simpleContent and complexContent to distinguish their mapping occurrences.

### B-006 — local wildcard presence

[§3.4.2](../../upstream/structures.html#declare-type). With no group/base wildcard,
no anyAttribute maps an absent local/complete wildcard. Add
`<anyAttribute namespace="urn:u" processContents="skip"><annotation
id="wa"/></anyAttribute>`: CM local wildcard is `{u}`, skip, annotation wa.
Unknown u:x attribute V+, unknown t:x V−. Omit namespace/processContents:
CM becomes A/strict, so unknown u:x now fails strict assessment.

### B-007 — groups without wildcard do not create one

[§3.4.2](../../upstream/structures.html#declare-type). Reference `AG(G;{x:string?};
absent)`, first without local anyAttribute and then with `{u}/lax`. Complete
wildcard is respectively absent and `{u}/lax`, exactly the local value. Add a
second group with no wildcard: neither output changes. This is the no-contributing-
group branch even though attributeGroup children exist.

### B-008 — local wildcard controls intersection metadata

[§3.4.2](../../upstream/structures.html#declare-type). G1 wildcard `{t,u}/strict`
annotation g1; G2 `{u,v}/skip` annotation g2; local anyAttribute `{u,v}/lax`
annotation local. CM complete wildcard is `{u}/lax` with annotation local.
Swap G1/G2: same result. u:x without declaration is laxly unassessed; t:x/v:x
are V− by namespace. Change G2 to N(t) and G1 to N(u), remove local finite
restriction: the required intersection is unexpressible, SC−.

### B-009 — first contributing group controls processing

[§3.4.2](../../upstream/structures.html#declare-type). No local anyAttribute.
A leading AG0 has no wildcard; G1 has `{t,u}/strict`, G2 `{u,v}/skip`.
CM complete wildcard is `{u}/strict` and annotation absent. Swap G1/G2:
namespace remains `{u}`, processing becomes skip; unknown u:x is V− in the
first type, allowed/unassessed in the second. AG0 must not win the processing
choice. A single G1 gives its constraint/processing but complete annotation
still follows the specified absent-annotation mapping.

### B-010 — restriction does not inherit base wildcard implicitly

[§3.4.2](../../upstream/structures.html#declare-type). B has empty content and
attribute wildcard `{t,u}/lax`. Restrict with local `{u}/strict`: CM R wildcard
is `{u}/strict`, not a union with B. Restrict with neither local nor group
wildcard: CM R wildcard absent; unknown u:x V−. Using a group-only `{u}/lax`
also gives exactly that complete wildcard.

### B-011 — extension retains base when complete wildcard absent

[§3.4.2](../../upstream/structures.html#declare-type). B empty content, base
wildcard `{u}/lax` annotation base; extend with no anyAttribute or contributing
group. R wildcard is B's wildcard, including its processing/annotation. Change
B wildcard to absent too: R wildcard absent. A simple xs:string base has no
attribute wildcard and therefore follows the absent-base case.

### B-012 — extension unions namespace constraints, takes complete metadata

[§3.4.2](../../upstream/structures.html#declare-type). B wildcard `{t}/strict`
annotation base; complete local wildcard `{u}/skip` annotation local. Extension
maps R wildcard `{t,u}/skip` annotation local. t:x unknown is now allowed
unassessed; it does not retain strict merely because t originated in B. Swap
base and local: set unchanged, processing strict and annotation base-from-new-
local. N(t) base with complete `{∅}/lax` makes unexpressible union, SC−.

### B-013 — extension with no base wildcard

[§3.4.2](../../upstream/structures.html#declare-type). Extend an empty complex B
with absent wildcard using local `{u}/lax` annotation a: R retains exactly the
complete wildcard. Repeat extending xs:string via simpleContent: identical
attribute wildcard result, content remains xs:string. Without local/group
wildcards both R wildcards are absent.

### B-014 — simple-content restriction starts from explicit inline type

[§3.4.2](../../upstream/structures.html#declare-type). B simple content xs:string.
R simpleContent restriction has inline simpleType restricting xs:string with
maxLength=4, plus outer minLength=2. CM R's simple content admits lengths 2–4:
`ab`/`abcd` V+, `a`/`abcde` V−. Omitting outer minLength restores length 0–4.
`<t:r>ab<t:a/></t:r>` is V− because a simple-content type forbids element
children even when the concatenated character text fits the facets.
An inline xs:integer restriction is not a valid derived simple content from
B's xs:string and cannot be made legal merely by the mapping.

### B-015 — simple-content restriction starts from inherited type

[§3.4.2](../../upstream/structures.html#declare-type). B simple content SStr
(maxLength=3). R simpleContent restriction has no inline simpleType and adds
minLength=2. CM content restricts SStr: `ab`/`abc` V+, `a`/`abcd` V−.
R with no facets retains SStr's restriction, not unconstrained xs:string.

### B-016 — mixed emptiable base to simple content

[§3.4.2](../../upstream/structures.html#declare-type). B is
`mixed(seq([e(a,0,1)],1,1))`. R simpleContent restriction includes inline
simpleType restricting xs:integer with minInclusive=0; `0` V+, `-1` V− and
`<t:a/>` V−. Remove inline simpleType: SC−. Set B's a min=1: SC− because B is
not emptiable. Use extension instead of restriction: SC− for this mixed base.
Outer `<complexType mixed="true">` on a legal simpleContent case remains
SC+ and does not turn its resulting simple content into mixed.

### B-017 — extend complex simple content

[§3.4.2](../../upstream/structures.html#declare-type). B has simple content SInt
and x:string?. R simpleContent extension adds y:string?. R content is exactly
SInt and both attributes remain; `<t:r x="a" y="b">0</t:r>` V+, `-1` V−.
No new facet restriction is implied by extension; facet children there are
forbidden by the representation grammar.

### B-018 — extend simple base

[§3.4.2](../../upstream/structures.html#declare-type). R simpleContent extension
base xs:integer adds x:string?. CM base=xs:integer, method=extension,
content=xs:integer; `<t:r x="a">2</t:r>` V+, `two` V−. Using restriction with
the simple base is SC−; replacing the base with complex B from B-017 permits
restriction and extension subject to their other rules.

### B-019 — effective mixed priority

[§3.4.2](../../upstream/structures.html#declare-type). For the same restriction
of anyType with seq([a],1,1), use outer mixed=true and inner
complexContent mixed=false: CM EO and `text<t:a/>` V−. Outer false/inner true:
CM mixed and that content V+. Inner absent/outer true gives mixed; both absent
give EO. This overlapping-premise witness proves inner priority.

### B-020 — effective empty content variants

[§3.4.2](../../upstream/structures.html#declare-type). With effective mixed=false,
restriction of anyType with (a) no model, (b) empty sequence, (c) empty all,
(d) empty choice min=0 max=1 all map content=empty. In each `<t:r/>` and `<t:r><!--c--><?p x?></t:r>` are V+,
`<t:r> </t:r>` V−. Empty choice min=1 max=1 is not in this empty-content
mapping: its language has no valid sequence, even `<t:r/>` V−. An annotation
inside an otherwise empty group does not make its model nonempty.

### B-021 — mixed empty content is a real empty sequence model

[§3.4.2](../../upstream/structures.html#declare-type). Repeat B-020(a–d) with
effective mixed=true: CM is mixed(seq([],1,1)); plain text V+, any element
child V−. The particle range is 1..1, not the source empty choice's 0..1.
No character children are synthesized; this case concerns the type mapping.

### B-022 — nonempty effective model remains its particle

[§3.4.2](../../upstream/structures.html#declare-type). Source sequence
min=0 max=2 containing a: CM effective content retains seq([a],0,2).
Do not replace it with range 1..1 merely because the surrounding type has one
content slot. Use a group ref to MG(G;seq([a,b],1,1)) min=2 max=3: effective
particle has reference range 2..3 and term G's group; `ab ab` V+,
`ab`/four repetitions V−.

### B-023 — restriction content mapping

[§3.4.2](../../upstream/structures.html#declare-type). Restrict anyType with
B-020's empty effective content: CM empty. With seq([a],1,1) and mixed=false:
CM EO with that particle; mixed=true: CM mixed with that particle. The group,
not a duplicate reconstructed language, is the content particle. EO accepts
XML whitespace (space/tab/LF/CR after XML normalization) between children but
rejects ordinary text and U+00A0 nonbreaking space; mixed accepts all of those
characters while retaining the same element model.

### B-024 — extension content mapping

[§3.4.2](../../upstream/structures.html#declare-type). B=EO(seq([a],1,1)).
Extension with effective content empty inherits exactly B.content. Empty B
extended with seq([b],0,2) maps EO of that local particle. Nonempty B extended
with seq([b],1,1) maps EO(seq([B.particle,local.particle],1,1)); `ab` V+,
`ba`, `a`, `b` V−. Repeat both nonempty types mixed: same ordered elements,
with arbitrary text allowed. Mixed mismatch is rejected by derivation rules.

### B-025 — complexContent needs a complex base

[`src-ct`](../../upstream/structures.html#src-ct). complexContent restriction
or extension base xs:string is SC−. Replace with a complex empty B and use an
empty restriction/extension: SC+. This distinguishes wrong base category from
an unresolved QName and from the separately valid simpleContent simple-base
extension in B-018.

### B-026 — all simpleContent base-category alternatives

[`src-ct`](../../upstream/structures.html#src-ct). Complex simple-content B with
R extension and restriction is SC+ under B-014/015/017; mixed emptiable B plus
restriction/inline type is SC+ under B-016; simple base plus extension is SC+
under B-018. Complex empty B, EO(seq([a],1,1)) B, mixed nonemptiable B, mixed
emptiable B plus extension, and simple base plus restriction are SC−.
Remove the inline type only from the mixed restriction: SC− isolates its
extra required child. Required wildcard intersections use B-008/B-009/B-089.

### B-027 — complex property domains and derivation cycles

[`ct-props-correct`](../../upstream/structures.html#ct-props-correct).
A complete CT with name R, namespace t, base anyType, method restriction,
final/block empty, abstract false, attrs empty, wildcard absent, content empty,
annotations empty is structurally well formed. Set abstract to a string rather
than boolean, method to list, or content to an attribute declaration: component
shape failure. Name absent and namespace absent are legal alternatives.
B.base=R and R.base=B is SC−; a longer B→D→R→B cycle is SC−. A type containing
an optional element typed by itself but with base anyType is SC+ and admits
finite nested elements. anyType's predefined self-base is legal; a user
self-base is not. Simple base+restriction fails; simple base+extension uses
B-018 and its simple content.

### B-028 — duplicate attributes and declared ID limits

[`ct-props-correct`](../../upstream/structures.html#ct-props-correct).
R includes distinct local declarations x:string and x:integer via two groups:
SC− despite differing type. Rename second to y: SC+. Put x in namespace u
instead: no expanded-name collision. Two distinct uses x:ID? and y:ID! are
SC− even when an instance would omit x. Replace y type with xs:string: SC+.
Using a restriction of ID for y remains SC−. Shared use identity reused by
set construction is not a second distinct declaration.

### B-029 — extension finality on complex and simple bases

[`cos-ct-extends`](../../upstream/structures.html#cos-ct-extends).
Extend empty complex B with empty R: SC+ when B.final={} or {restriction};
SC− when B.final includes extension. A simple base S requires content exactly S.
Published clause 2.2 also excludes extension from S.final, but normative
[Part 2's component domain](../../upstream/datatypes.html#defn-final) permits only
restriction, list and union. An extension-present simple base is therefore
outside the legal component domain, including component-level constructions.
Part 1's simple-type property tableau is explicitly non-normative.

### B-030 — extension retains base attribute identity/type

[`cos-ct-extends`](../../upstream/structures.html#cos-ct-extends).
B attrs {x:integer?}, R derived extension attrs {same x,y:string?}: Rel+.
Drop x, rename it z, change namespace to u, or change its type to SInt:
Rel− for extension retention, one mutation at a time. A separate restriction
may narrow the type; extension's retained type must be the same type.
Use component relations to isolate this rule from XML assembly, which already
inherits x rather than legitimately deleting it.

### B-031 — extension wildcard retention

[`cos-ct-extends`](../../upstream/structures.html#cos-ct-extends).
B wildcard {u}/lax. R wildcard {u,v}/skip satisfies this namespace retention
clause; stricter processContents is not required by this extension clause.
R wildcard absent or {v}/lax fails; {u}/lax succeeds. The actual XML extension
must still compute its wildcard by B-011–B-013 and satisfy expressibility.

### B-032 — extension content alternatives and categories

[`cos-ct-extends`](../../upstream/structures.html#cos-ct-extends).
B/R both content xs:integer: Rel+; R content SInt instead: Rel−. Both empty:
Rel+. B empty/R EO(seq([a],1,1)): Rel+. B EO(seq([a],1,1)),
R EO(seq([B.particle,e(b,1,1)],1,1)): Rel+. Change only R to mixed: Rel−;
start with both mixed: Rel+. Remove R's model while B is nonempty: Rel−.
Move the retained base to second sequence position: Rel− by particle extension.

### B-033 — extension cannot restore a previously restricted feature

[`cos-ct-extends` 1.5](../../upstream/structures.html#cos-ct-extends).
A has EO(seq([a],1,1)); B extends A with optional attribute x:string;
C restricts B and prohibits x; D extends C and introduces x:string again.
The restore path fails the full ancestry extension-then-restriction condition:
SC− even though immediate C→D appears locally admissible. D adding new y while
leaving x prohibited is the opposing SC+ path. Repeat with an element feature:
B appends b optional, C removes b, D appends b again; restoration must not be
accepted merely by prefix checking the immediate base. Keep different element
names during this witness so UPA does not mask the ancestry condition.

### B-034 — complex restriction base and finality

[`derivation-ok-restriction`](../../upstream/structures.html#derivation-ok-restriction).
Restrict empty B to empty R: Rel+ with B.final={} or {extension}; Rel− with
restriction in B.final. A simple xs:string base is Rel− here even when R's
simple content accepts only strings; use a legal complex simple-content B
instead to reach later content branches.

### B-035 — attribute occurrence restriction

[`derivation-ok-restriction` 2.1.1](../../upstream/structures.html#derivation-ok-restriction).
For same x declaration type xs:string, B optional/R optional and B optional/R
required are Rel+; B required/R required Rel+; B required/R optional Rel−.
Keep all content/wildcards/fixed constraints unchanged. This four-cell table
separates the OR alternatives B-not-required and R-required.

### B-036 — attribute simple-type restriction

[`derivation-ok-restriction` 2.1.2](../../upstream/structures.html#derivation-ok-restriction).
B x:integer?, R x:SInt?: Rel+ and x=`-1` becomes V− under R. R x:string? is
Rel−; R x:integer? by same type identity is Rel+. A named restriction of SInt
with maxInclusive=2 is a nested Rel+; a derivation path violating a base final
is not a legal source type and must be reported as that earlier schema error.

### B-037 — effective value constraint precedence

[`derivation-ok-restriction` 2.1.3](../../upstream/structures.html#derivation-ok-restriction).
B use x:string has no constraint: R none/default/fixed all meet this clause.
B default("a"): R none or fixed("b") also meets this clause. B fixed("a"):
R fixed("a") Rel+, R none/default("a")/fixed("b") Rel−. If B use has no
constraint but its declaration fixed("a"), the same fixed-preservation result
applies. A use-level fixed constraint takes precedence in determining effective
constraint; a conflicting use/declaration pair is separately invalid by B-043.
Use xs:string literals here so typed/canonical lexical disagreement cannot
mask the effective-source or same-string decision.

### B-038 — new attributes need base wildcard admission

[`derivation-ok-restriction` 2.2](../../upstream/structures.html#derivation-ok-restriction).
B no uses, wildcard {u}/lax. R adds {u}x:string?: Rel+. Change x namespace to
t, then absent: Rel−. Remove B wildcard: Rel−. R does not need to retain B's
wildcard to add this explicitly permitted use. With matching x already in B,
the matched-use checks B-035–B-037 take precedence; a permissive wildcard cannot
excuse changing x's type to unrelated xs:string.

### B-039 — every required base use survives

[`derivation-ok-restriction` 3](../../upstream/structures.html#derivation-ok-restriction).
B attrs {x:string!,y:string?}. R retains x! and omits y: Rel+; R omits x,
marks x optional, or provides only {u}x!: Rel−. A default or fixed constraint
on an optional replacement does not make it a required use. In XML restriction,
explicit prohibition of required x is SC−.

### B-040 — restricted attribute wildcard

[`derivation-ok-restriction` 4](../../upstream/structures.html#derivation-ok-restriction).
B {u,v}/lax, R {u}/strict: Rel+. R absent also meets the wildcard condition.
R {u}/lax Rel+; R {u}/skip Rel−; R {u,v,t}/strict Rel−; B absent/R {u}/strict
Rel−. Against anyType's A/lax wildcard, R {u}/skip is allowed by the explicit
strength exception. Independently use all nine B/R processing pairs on equal
{u}: strict→strict, lax→lax/strict, skip→skip/lax/strict pass; the remaining
three weakening pairs fail for an ordinary base.

### B-041 — restriction content alternatives

[`derivation-ok-restriction` 5](../../upstream/structures.html#derivation-ok-restriction).
Separate complete pairs: (a) B=anyType, R empty: Rel+; (b) B simple-content
xs:integer, R SInt: Rel+, R xs:string Rel−; (c) B mixed(seq([a?],1,1)), R
simple-content xs:integer with required inline source type: Rel+, make a
required: Rel−; (d) B empty/R empty Rel+; (e) B EO(seq([a?],1,1)) or the same
mixed, R empty Rel+, make a required Rel−; (f) B mixed(seq([a],1,1)), R EO
same particle Rel+; (g) both mixed same particle Rel+; (h) B EO, R mixed Rel−;
(i) same EO categories but R=b rather than a Rel− by particle relation.
An EO pointless empty sequence against empty B follows normalization; the
opposing nonpointless seq([a],1,1) does not. Do not replace simple content with
content=empty merely because a simple restriction admits only the empty string.

### B-042 — complex derivation recursion and identity

[`cos-ct-derived-ok`](../../upstream/structures.html#cos-ct-derived-ok).
For blocked set {extension,restriction}, D=B identity Rel+. D immediately
restricts B: Rel+ with blocked={} or {extension}, Rel− with {restriction}.
D extends C, C restricts B: Rel+ with {}, Rel− with extension blocked at D,
and Rel− with restriction blocked at C. B=unrelated named U gives Rel−;
reaching a nonmatching anyType boundary terminates failure rather than looping
through anyType.self-base. For the simple recursion branch, D is a complex
simpleContent extension of SInt and B=xs:integer: Rel+ with {} through simple
base derivation, Rel− when D's extension is blocked. Same-looking separate
anonymous types do not acquire identity merely from equal properties; retain
the specification's acknowledged identity-option boundary in §3.4.6.

## Attribute uses/groups and models

### B-043 — attribute-use properties and fixed comparison

[§3.5](../../upstream/structures.html#cAttributeUse). A use requires a boolean
required property and an attribute declaration; required=`optional` string or
a type component in place of declaration is malformed CM. A declaration with
fixed("a") and use absent constraint is allowed; use fixed("a") allowed;
use default("a") or fixed("b") fails `au-props-correct`. Declared xs:boolean
fixed true with use fixed true: input x=`true` passes both declaration and
`cvc-au`; x=`1` has equal typed declaration value but its normalized spelling
is not canonical `true`, so the **published use-level lexical comparison**
fails. Keep declaration/value and use-level string oracles separate; this
reading is not inferred from common processor behavior. Prohibited XML use
creates no attribute-use component rather than required=false with a ban flag.

### B-044 — synthesized attribute PSVI

[`sic-attrDefault`](../../upstream/structures.html#sic-attrDefault).
Empty R has local use x:integer? fixed("02"). Missing x on `<t:r/>` contributes
attribute local name x, absent namespace, schema normalized/default `2`,
validity valid, attempted full and schema specified schema. The contribution
separately says the added items **should also** have corresponding full type
properties, with member properties where appropriate, or their lightweight
alternatives. Retain this
recommendation's modality, and apply independently established Attribute
Validated by Type/E1-56 duties at their own entry; no general omission policy
or mandatory absence follows. Present x=`2` is infoset-origin instead; present
x=`3` V−. Change the use to required with fixed constraint: missing x remains
V− and is not repaired by optional-use synthesis. With namespace u, the added
attribute's namespace is u; there is no default namespace declaration synthesis.
The added attribute validation context is the nearest ancestor carrying schema
information (normally the assessment root). Recommended union-member metadata
retains the applicable member selection and full/lightweight qualifications;
this does not create new
character children. No use constraint means no synthesized attribute under this contribution's
predicate. A referenced declaration-level constraint and absent use constraint
must retain their distinct source properties for the clause's exact predicate;
see the tension record below rather than merging them silently.

### B-045 — attribute-group component/representation conditions

[§3.6](../../upstream/structures.html#cAttribute_Group_Definitions).
`AG(G;{x:string?};absent)` is well formed; G ref in R imports its use. Definition
name G with absent or t namespace and optional annotation are valid component
properties; use a wildcard component as a member of its attribute-use set:
shape failure. G contains two distinct x declarations: SC−; x and {u}x: SC+.
x:ID and y:ID-derived: SC− even optional; x:ID and y:string: SC+.
G→G and G→H→G ordinary references are SC−. Acyclic G→H containing y is SC+.
Redefine's exactly permitted self-reference belongs to §4.2.2 and must not be
rejected as ordinary circular expansion. Group wildcard N(t) intersect N(u)
is unexpressible, SC−; `{t,u}` intersect `{u,v}` maps `{u}`, SC+.

### B-046 — named model group versus reference

[§3.7](../../upstream/structures.html#declare-namedModelGroup).
`MG(G;seq([a,b],1,1))`; a type using `<group ref="t:G" minOccurs="2"
maxOccurs="3"/>` has a particle 2..3 whose term is G's group. `ab ab` and
three ab repetitions V+; one/four repetitions V−. A named definition maps its
name/namespace/group/optional annotation, not the reference's occurrence range.
An explicit annotation `definition` belongs to MG(G); a reference annotation
`reference` does not replace G's model-group annotation or create a new group
definition.
An annotation component in place of its required model group fails the
component shape. Ref with 0..0 creates no particle; the definition still exists.
Missing/extra model children or occurrences on a named definition's child
violate the contextual grammar, not a new instance cardinality rule.

### B-047 — group containment cycles versus type recursion

[`mg-props-correct`](../../upstream/structures.html#mg-props-correct).
Group G containing a particle whose term is G fails. G contains H, H contains G
also fails at depth two; G→H→I→G is the deeper companion. A noncyclic nested
sequence/choice model succeeds. A group containing e(a) whose type recursively
contains the same element type but whose model-group graph does not point back
to G is legal type recursion. Component compositor must be all/choice/sequence;
`interleave` or a nonparticle in its ordered particles list fails its shape.

### B-048 — group sequence/choice/all validation

[`cvc-model-group`](../../upstream/structures.html#cvc-model-group).
Sequence [a?,b!,c?] accepts `b`, `ab`, `bc`, `abc`; rejects `ba` and empty.
Choice [a!,seq([b,c],1,1)] accepts `a` and `bc`; rejects `ab`, `b` and empty.
All [a!,b?] accepts `a`,`ab`,`ba`; rejects empty, `aa`, and `ac`.
Empty sequence/all accept empty; empty choice accepts nothing. Outer
choice([],0,1) admits empty via zero occurrence, whereas choice([],1,1) does
not. Empty subsequences used for optional members are legal partitions.

**Nested choice and repetition.** Define named G as
sequence(choice(a?,b?),c!), then use G through one reference with range2..2.
Use distinct unqualified local names a/b/c, all with admitted named xs:string
type, and otherwise satisfied source/component/UPA/EDC conditions. The inner
choice and sequence each occur once; a and b each have range0..1, c has1..1.
`cc`, `acc`, `bcc`, `acbc` and `bcac` are valid. `c` and `ccc` fail the whole-
group count; `acb` lacks the second required c; `abcc` cannot put a and b in
one choice occurrence before its c. Different whole G occurrences can select
different branches. On each empty inner choice, either optional branch can
validate empty: the rule asks for a successful branch, not a unique one.

Remove c, retaining the outer reference's2..2 range. The whole G term is now
nullable: empty, `a`, `ab` and `ba` are valid, using empty pieces when needed;
`aaa` is invalid because each whole occurrence can consume at most one item.
These contrasts join B-057's repetition counting with the recursive model-
group rule, rather than using particle-restriction examples as V evidence.
They prescribe no backtracking algorithm, observable chosen split or new
whole-product execution credit.

### B-049 — all placement/ranges

[`cos-all-limited`](../../upstream/structures.html#cos-all-limited).
All [a!,b?] as an entire complex type model is SC+ with outer 1..1 or 0..1.
The same all stored in MG(G) and used as an entire type model with max=1 is
SC+. Place G's all as a member of a sequence or choice, or give its whole-model
reference max=2: SC−. A member with max=2/∞ fails; a member with source
min=max=0 disappears before surviving-component checks. Member max=1 with
min=0 or 1 succeeds. Abstract component outer range 2..1 also fails general
particle correctness; do not count that earlier range failure as proof only
of all placement.

### B-050 — same-name element consistency

[`cos-element-consistent`](../../upstream/structures.html#cos-element-consistent).
A deterministic sequence has two local a declarations typed by the same named
xs:string: SC+. Give each a separately written anonymous xs:string restriction:
SC− because the repeated declaration types lack names. Give one type SStr and
one xs:string: SC− despite derivation. Give two named T types with different
namespaces t/u: SC− despite equal local type names. Change the second element
to {u}a: no same-expanded-element-name collision. Repeat nested group expansion
and implicit substitution member a versus explicit a to reach indirect and
implicit containment. Keep sequence positions fixed 1..1 to avoid a masking
UPA failure.

### B-051 — UPA without future lookahead or value tests

[`cos-nonambig`](../../upstream/structures.html#cos-nonambig).
Choice [a,a] fails even when the two a declarations have the same named type.
Sequence [a?,a] fails; sequence [a,a] with fixed 1..1 positions succeeds.
Choice [e(a),W({t},skip)] fails; the same wildcard restricted to {u} does not
overlap a. Two choice wildcards {t,u} and {u,v} fail; {t} and {u} do not.
Head h with concrete member a in substitution group overlaps an explicit a,
and a wildcard permitting member a's namespace overlaps the h particle even
if it excludes h's own namespace. Sequence [a?,b,a] is deterministic because b
separates positions; [a?,seq([],1,1),a] remains ambiguous. Duplicating a named
group reference does not identify its distinct particle positions.

## Ranges and particle relations

### B-052 — sequence/all effective total ranges

[`cos-seq-range`](../../upstream/structures.html#cos-seq-range).
seq([e(a,1,2),e(b,0,3)],2,4) has effective range **2..20** (sum 1..5 times
2..4). Replace b with seq([c,d],0,2), effective 0..4: total **2..24**.
all([a!,b?],1,1) has **1..2**. Empty sequence/all, including outer 0..∞,
has **0..0**; an outer ∞ does not make a zero-total model infinite.
seq([e(a,0,1)],0,∞) has **0..∞**. A child e(a,1,∞) makes the enclosing maximum
∞ even with finite outer maximum. Check the integer sums/products themselves,
not inferred small instance samples.

### B-053 — choice effective total ranges

[`cos-choice-range`](../../upstream/structures.html#cos-choice-range).
choice([e(a,1,2),e(b,3,4)],2,3) has effective **2..12**.
Replace b with seq([c,d],0,2): child effective 0..4, choice total **0..12**.
Empty choice's effective numeric range is **0..0**, although its nonoptional
validation language is empty. choice([],0,∞) stays 0..0. A nonzero alternative
with unbounded outer max gives unbounded total max; a child alternative with
max ∞ also gives ∞. This numeric helper does not itself validate the language.

### B-054 — particle correctness and source-zero elimination

[`p-props-correct`](../../upstream/structures.html#p-props-correct).
Source local element/group/sequence/choice/any range 0..0 contributes no
particle. A surviving abstract particle min=0,max=0 fails the property's
max≥1 condition; do not confuse this abstract invalid component with valid
source elimination. Ranges 0..1,1..1,2..3,0..∞ are legal; 2..1 fails min≤max.
min=-1, max=-1 or a string as minimum fail the property domain. Missing source
occurrences map 1..1. Wrong-kind term (attribute use instead of element/group/
wildcard) fails component shape; an unresolved term is handled under §5.3,
not silently converted into an empty valid group.

### B-055 — wildcard particle counts and every-item admission

[`cvc-particle` 1](../../upstream/structures.html#cvc-particle).
W({u},skip,2,3): sequences of one u:a V−, two/three V+, four V−; replace the
second of two items by t:a: V− despite correct length. W({u},skip,2,∞) accepts
four and every finite count ≥2; empty fails. W({u},skip,0,1) accepts empty.
Use well-formed elements so XML syntax does not mask wildcard validity.

### B-056 — local/global/substituting element particle selection

[`cvc-particle` 2](../../upstream/structures.html#cvc-particle).
Local e(a,2,3) accepts two/three t:a, rejects one/four, t:b and u:a. Global
g(a,2,3) with concrete a gives the same count/name results and selects global
a as context-determined declaration. Abstract global head h with concrete
a affiliated and allowed substitution accepts a and selects a's declaration;
h itself fails when abstract. Set h.block=substitution: a no longer matches.
Remove global a, break its type derivation, or make its substitution blocked:
no successful substitute. A local declaration named h does not accept a from
a same-name global head's group. W({t},skip) does not admit a u:a member merely
because its head h is in t. These cases separate match from subsequent local
content validation.

### B-057 — model particle repetition partitions

[`cvc-particle` 3](../../upstream/structures.html#cvc-particle).
seq([a,b],2,3) accepts `abab`/`ababab`; rejects `ab`, `abababab`, and `abba`.
Counts refer to complete group occurrences, not individual child count.
seq([e(a,0,1)],2,3) accepts empty because two empty subsequences can satisfy the
minimum repetition count. seq([a,b],2,∞) accepts four complete ab pairs.
The same logic recursively applies when one group contains another; a bad
second group's ordering fails even though total count is within bounds.

### B-058 — valid particle extension

[`cos-particle-extend`](../../upstream/structures.html#cos-particle-extend).
E=B by component identity: Rel+. B=seq([a,b],1,1);
E=seq([B-copy,e(c,1,1)],1,1), where B-copy differs only in annotation: Rel+.
Change B-copy's a range to 0..1, swap a/b, or place it second: Rel−.
Keep identical first child but change E outer range to 0..1 or 1..2: Rel−.
A different group kind around the same accepted language is not this extension.

### B-059 — identity particle restriction

[`cos-particle-restrict`](../../upstream/structures.html#cos-particle-restrict).
R and B are the **same** particle object/component: Rel+, even when its term
kind would require a recursive rule for distinct particles. A separately built
copy is not justified by this identity shortcut; evaluate its normalized kind
and relevant structural rule. Distinct component particles can still restrict
successfully by the later rules; pointer inequality is not a rejection rule.

### B-060 — substitution expansion for restriction dispatch

[`cos-particle-restrict` 2.1](../../upstream/structures.html#cos-particle-restrict).
Global concrete h and concrete member a of the same named type: g(h,0,2)
expands to choice([g(h,1,1),g(a,1,1)],0,2). Abstract h leaves only concrete
members; an abstract member is absent from the actual group. Filtering uses
h's disallowed substitutions and the member-to-h type derivation path, including
h's and intermediate complex types' prohibited substitutions. A member's own
`block="substitution"` does not exclude it from h's group; with the same named
type there is no derivation method for a type-method block to intersect. Apply
the expansion independently when the head occurs in R, B, or both. A head with
no other actual group member stays an element term, including when a potential
member is filtered by blocking. For a head with members a,b, sequence [a,b]
may restrict its repeated expanded choice via MapAndSum rather than an
incorrect element:element comparison. Clause 2.1 gives no ordering of the set-
derived choice. [I-005](../../INTERPRETATIONS.md#i-005--implicit-substitution-choice-order)
classifies this as published under-specification, with an unpublished order-waiver
proposal. Use order-independent mappings or unit generated singletons for hard
expectations; do not invent a generated list order.

### B-061 — pointless sequence cases and opposing contexts

[`cos-particle-restrict` 2.2](../../upstream/structures.html#cos-particle-restrict).
Ignore seq([],1,1), and seq([],0,2) under the empty-sequence case. Ignore
seq([a],1,1) as singleton; with range 0..2 the singleton is not ignored.
Within seq([a,seq([b,c],1,1),d],1,1), flatten the inner sequence to ordered
[a,b,c,d]. Give the inner range 1..2: it is no longer pointless by that rule.
Put inner seq([b,c],1,1) inside a choice: multi-member group no longer qualifies
by same-parent-kind. Retain the group's annotation-only difference as irrelevant
to this normalization, not as a change to occurrence or element names.

### B-062 — pointless all cases

[`cos-particle-restrict` 2.2](../../upstream/structures.html#cos-particle-restrict).
Empty all and all with exactly one a are ignored by the literal pointless-all
rules. All with [a,b] is not. The published all normalization does not add the
sequence/choice outer-1..1 premise; do not import it. XML all placement/range
constraints still apply before a schema reaches this relation; direct abstract
relations can expose the normalization without an illegal nested-all wrapper.

### B-063 — pointless choice cases

[`cos-particle-restrict` 2.2](../../upstream/structures.html#cos-particle-restrict).
Ignore choice([],0,1); retain choice([],1,1). Ignore choice([a],1,1); retain
choice([a],0,2). Flatten choice([a,choice([b,c],1,1)],1,1) to [a,b,c]; with
inner range 1..2 retain the inner group. A multi-member inner choice under a
sequence does not qualify by same-kind parent. These opposing examples prevent
normalizing every syntactically nested choice indiscriminately.

### B-064 — occurrence range relation

[`range-ok`](../../upstream/structures.html#range-ok). For B=2..4, R=2..4,
3..3 and 4..4 are Rel+; R=1..4 isolates too-small minimum, R=2..5 isolates
too-large maximum; R=2..∞ fails against finite B. B=2..∞ admits R=2..∞ and
R=3..5. B=0..∞ admits optional 0..1. Equal max with decreased min still fails;
increased min with unbounded R against finite max still fails.

### B-065 — element restriction expanded names and ranges

[`NameAndTypeOK`](../../upstream/structures.html#rcase-NameAndTypeOK).
R=e(a,2,3), B=e(a,1,4), same named xs:string and other default properties:
Rel+. Change R local name to b or namespace to u: Rel−. Retain name/namespace
but R=0..3 or 2..5: Rel−. Local versus global scope is not part of expanded-name
equality; it selects the later property branch rather than bypassing the name
and range requirements.

### B-066 — global/global element shortcut and its feasibility

[`NameAndTypeOK` 3.1](../../upstream/structures.html#rcase-NameAndTypeOK).
R=g(a,2,3), B=g(a,1,4), both referring to the **same** global a declaration:
Rel+ through global/global branch after name/range checks. R=g(b,2,3) fails
name first. Independently changing nillable/type/fixed of just R's global a
while keeping B's global a at the same expanded name is not a legal single-
schema setup: global uniqueness forces identity. Do not manufacture that
impossible branch-isolation witness; use B-067–B-071 with local declarations
to isolate each local-property premise instead.

### B-067 — element restriction nillability

[`NameAndTypeOK` 3.2.1](../../upstream/structures.html#rcase-NameAndTypeOK).
Same local a/string, range 1..1: B.nillable=true/R=false passes, both true
passes, both false passes, B=false/R=true fails. All other properties equal.
This is a schema relation; xsi:nil lexical/content behavior is a separate
instance oracle and cannot prove the structural restriction by itself.

### B-068 — element restriction fixed constraint

[`NameAndTypeOK` 3.2.2](../../upstream/structures.html#rcase-NameAndTypeOK).
Local a typed xs:string. B none allows R none/default("x")/fixed("x");
B default("x") allows R none/fixed("y") for this condition. B fixed("x")
requires R fixed("x"); R absent/default("x")/fixed("y") fail. Use integer
B fixed("02")/R fixed("2") for the equal-typed-value companion; distinguish
this from use-level lexical comparison in B-043.

### B-069 — element restriction identity-constraint set direction

[`NameAndTypeOK` 3.2.3](../../upstream/structures.html#rcase-NameAndTypeOK).
Local a B.IDCs={K,L}, R.IDCs={K}: Rel+; R={} also meets this clause;
R={K,L,M} or R={M} fails. The set comparison uses actual definition identities,
not equality of selector strings or a general belief that adding constraints
must be a valid restriction. XML declaration copies with new same-name identity
components can fail global identity uniqueness earlier; use an abstract
component relation to isolate this prescribed subset direction.

### B-070 — element restriction block inclusion

[`NameAndTypeOK` 3.2.4](../../upstream/structures.html#rcase-NameAndTypeOK).
B.block={extension}; R.block={extension,restriction}: Rel+. R={extension}
passes, R={} or {restriction} fails. A stricter block set is allowed; equality
of accepted element names alone does not prove the required block superset.

### B-071 — element restriction type derivation

[`NameAndTypeOK` 3.2.5](../../upstream/structures.html#rcase-NameAndTypeOK).
Local a B.type=xs:integer, R.type=SInt: Rel+; same named xs:integer passes;
R.type=xs:string fails. Complex B.type with optional b, R.type restricting it
to required b passes; R.type extending B with c fails because extension is in
the prohibited set {extension,list,union}. B.type is union(xs:integer,xs:string),
R.type=xs:integer: the member-of-union derivation alternative is Rel+.
Repeat one nested valid restriction and one chain containing an extension to
ensure the prohibited set is checked recursively.

The supplied set is interpreted by the appropriate derivation predicate;
[`Type Derivation OK (Simple)`](../../upstream/structures.html#cos-st-derived-ok)
makes only restriction relevant. For a component-relation companion, use
admitted local declarations with the same expanded name and range 1..1,
nillable=false, absent value constraints, and empty identity/block sets.
B.type=xs:anySimpleType; R.type is an admitted list of xs:integer or an
admitted union of xs:integer and xs:string. With the simple predicate's
clause 2.1 satisfied, its clause 2.2.3 supplies Rel+; the list/union tokens in
the supplied set do not veto this route. This establishes the guarded
component relation, not a complete XML restriction fixture or runtime/PSVI
observation. Keep the complex-extension negative above.

### B-072 — element against wildcard

[`NSCompat`](../../upstream/structures.html#rcase-NSCompat).
R=e(a,2,3), B=W({t},strict,1,4): Rel+. Change B to {u}, or R to unqualified a
while B=N(u): Rel− namespace. R=0..3 or 2..5 fails range separately.
Change B strict→lax→skip: namespace/range relation unchanged; no additional
processContents comparison is specified for an element against a wildcard.

### B-073 — element as singleton group

[`RecurseAsIfGroup`](../../upstream/structures.html#rcase-RecurseAsIfGroup).
R=e(a,1,1). B=all([a!,b?],1,1), seq([a!,b?],1,1), or choice([a!,b!],1,1)
all allow the corresponding singleton wrapper and give Rel+. Make b required
in all/sequence: Rel− due unmapped nonemptiable B member; choice still passes.
Change B sequence to [b!,a!] or R to b where only a appears: Rel−. R's original
range remains inside the synthetic wrapper, whose outer range is exactly 1..1.

### B-074 — wildcard against wildcard restriction

[`NSSubset`](../../upstream/structures.html#rcase-NSSubset).
R=W({t},strict,2,3), B=W({t,u},lax,1,4): Rel+. Widen R namespaces to {t,u,v},
min to 0, max to 5, or weaken R to skip: each separately Rel−.
On equal namespace/range, the nine processing pairs give the strength table
in B-040. Restricting anyType's **specific** A/lax content wildcard to
W({t},skip,0,∞) is allowed by the ur-type exception; an ordinary user's A/lax
wildcard does not acquire that exception by structural similarity.

### B-075 — group against wildcard with total cardinality

[`NSRecurseCheckCardinality`](../../upstream/structures.html#rcase-NSRecurseCheckCardinality).
R=seq([e(a,1,1),e(b,1,1)],1,1), B=W({t},skip,1,2): both members independently
restrict B and total 2..2 restricts B, Rel+. B.max=1: each member still fits
but total fails. Restore B.max=2 and change b namespace to u: total fits but
member recursion fails. For all([a!,b?],1,1) use B=W({t},skip,0,2): Rel+;
for choice([a!,b!],1,1) use B=W({t},skip,1,1): Rel+. A nested sequence whose
second descendant is u:c reaches the same failing namespace at depth two.

The member recursion uses B's original range, not a copy with minimum zero.
Against B1..2, all([a!,b?],1,1) therefore fails b's member minimum even though
its total1..2 fits. [I-004](../../INTERPRETATIONS.md#i-004--published-member-minimum-rule)
records the published-baseline decision and acknowledged historical bug.
Keep member-minimum and aggregate-minimum opponents separate.

### B-076 — ordered Recurse for sequence and all

[`Recurse`](../../upstream/structures.html#rcase-Recurse).
B=seq([a!,b?,c!],1,1), R=seq([a!,c!],1,1): Rel+; omitted b is emptiable.
Make B.b required: Rel−. R=seq([c!,a!],1,1) fails ordered mapping; add d to R
fails completeness; change R.a type to unrelated xs:integer fails the mapped
member relation. For all use B=all([a!,b?,c!],1,1), R=all([a!,c!],1,1):
Rel+ in listed order, reversed [c!,a!] Rel− **despite identical unordered
instance language**. Outer B range 0..1/R 1..1 fits; R 0..1/B 1..1 fails.
Nest the ordered sequence pair inside corresponding choices with distinct
other alternatives to reach recursive Recurse without ambiguous siblings.

### B-077 — ordered RecurseLax for choices

[`RecurseLax`](../../upstream/structures.html#rcase-RecurseLax).
B=choice([a!,b!,c!],1,2), R=choice([a!,c!],1,1): Rel+ although unmapped B.b
is not emptiable. R=[c!,a!] fails order; R=[a!,d!] fails completeness;
R outer max=3 fails range. A mapped member with unrelated type fails recursion.
There is no permission here to accept arbitrary reordering just because choice
language is unordered. Two R members cannot both map to one B position in
an order-preserving mapping whose successor must follow its predecessor.

### B-078 — unordered injective sequence-to-all mapping

[`RecurseUnordered`](../../upstream/structures.html#rcase-RecurseUnordered).
B=all([a!,b!,c?],1,1), R=seq([b!,a!],1,1): Rel+. Required c instead of
optional c fails unmapped-base emptiability. R=[a!,a!,b!] fails injectivity:
two R particles cannot map to B.a. R=[b!,d!] fails complete member mapping;
R.a with unrelated type fails recursion. Outer R range widened beyond B fails
even if every leaf mapping works.

### B-079 — MapAndSum exact product

[`MapAndSum`](../../upstream/structures.html#rcase-MapAndSum).
B=choice([a!,b!],2,4), R=seq([a!,b!],1,2): product (1×2)..(2×2)=2..4,
Rel+. R=[a!,a!] also maps to B.a twice and passes this noninjective relation.
R=[a!,d!] fails member mapping. R max=3 gives product 2..6 and fails B.max;
B.min=3 with original R fails product minimum. B.max=∞ admits R.max=∞
when all member conditions hold. Opposing conservative case: R=[a?,b?],
outer 1..1, B choice [a?,b?] outer 0..1; leaf maps succeed but prescribed
product is 2..2 versus 0..1 and fails, even if a more general containment test
would reason from child optionality. Do not replace the rule by effective range.

### B-080 — emptiability alternatives

[`cos-group-emptiable`](../../upstream/structures.html#cos-group-emptiable).
e(a,0,1) is emptiable; e(a,1,1) is not. seq([a?,b?],1,1) and
choice([a!,b?],1,1) are emptiable through effective minimum zero;
seq([a!,b?],1,1) is not. Outer min=0 makes even required-child groups emptiable.
Empty choice min=1 has computed minimum zero yet no valid sequence: retain
this distinction between the specification's helper relation and a direct
language-nonemptiness test.

## Wildcards and algebra

### B-081 — namespace representation mapping

[§3.10.2](../../upstream/structures.html#declare-openness).
Under targetNamespace t, `namespace="##targetNamespace ##local urn:u"` maps
{t,∅,u}; repeated urn:u makes no second set member. In a no-targetNamespace
schema, `##targetNamespace` maps {∅}, exactly as `##local`; the combined list
still has one absent member. ##other maps N(t) with a target namespace and
N(∅) without one. Omitted namespace maps A; omitted processContents maps strict.
Explicit `##any` maps A. An any particle with no occurrence attributes maps
1..1; 0..0 creates no wildcard particle. The same namespace/process/annotation
mapping supplies anyAttribute without particle occurrences.

### B-082 — namespace admission's alternatives

[`cvc-wildcard-namespace`](../../upstream/structures.html#cvc-wildcard-namespace).
A admits namespace t,u,∅. N(t) admits u, rejects t, rejects ∅ independently
of its inequality to t. N(∅) admits t/u and rejects ∅. Finite {t,∅} admits
exactly t and ∅, rejects u; {} admits none. Namespace identity uses the complete
namespace string: urn:t and urn:t/ differ. QName prefix aliases for urn:t do
not change admission.

### B-083 — lax/strict/skip context and assessment

[`cvc-wildcard`](../../upstream/structures.html#cvc-wildcard).
The same matching u:a is assigned no context-determined declaration by lax,
mustFind by strict, skip by skip. With a global u:a declared xs:integer and
text `2`, strict/lax assessment succeeds; text `bad` fails both. With no u:a
component and no usable xsi:type, strict cannot fully validate, lax permits
notKnown/none fallback at that item. With usable xsi:type=xs:integer on the
undeclared u:a, strict element type assessment can proceed. Under skip, even
an available integer declaration does not cause `bad` to be schema assessed;
well-formed XML is still required. For attributes no xsi:type path exists.
Namespace-denied t:a fails under **each** process value, before this assignment
can turn it into permitted content.

### B-084 — all intensional subset branches

[`cos-ns-subset`](../../upstream/structures.html#cos-ns-subset).
Any of A,N(t),N(∅),{t},{} is below A. N(t) below N(t) and N(∅) below N(∅)
succeed; N(t) below N(u), or below N(∅), fails the **specified intensional
relation**, even where a set-theoretic argument suggests containment.
{t} below {t,u}, {t} below {t}, and {} below {} succeed; {t,u} below {t}
fails. {u} below N(t) succeeds; {t} below N(t) fails the negated namespace;
{u,∅} below N(t) fails absent membership. {} below N(t) succeeds.
A below {t} or N(t) fails. N(t) below any finite set fails.

### B-085 — union equality, universal, finite sets

[`cos-aw-union`](../../upstream/structures.html#cos-aw-union).
Union({t},{t})={t}; Union(N(t),N(t))=N(t); Union(A,{u})=A;
Union({t,u},{u,∅})={t,u,∅}; Union({},{u})={u}; Union({},{})={}.
Swap operands in each nonidentical pair: result unchanged. These namespace
outputs alone do not select processContents; B-012 is that separate mapping.

### B-086 — union of distinct negations

[`cos-aw-union` 4](../../upstream/structures.html#cos-aw-union).
Union(N(t),N(u))=N(∅), not A: no operand admits absent.
Union(N(t),N(∅))=N(∅) too. t and u namespaces are admitted by the resulting
N(∅), while unqualified attributes remain denied. The equal-negation control
N(t) with N(t) stays N(t), not N(∅).

### B-087 — four negation-plus-set union quadrants

[`cos-aw-union` 5](../../upstream/structures.html#cos-aw-union).
Hold N(t): with S={t,∅} result A; with S={t} result N(∅);
with S={∅} result **unexpressible**; with S={u} (or {}) result N(t).
All four use explicit finite operands rather than prose placeholders.
Swap operand order without changing result. The unexpressible XML extension
is SC−; approximating it with A would wrongly admit namespace t, while N(t)
would wrongly reject absent.

### B-088 — absent-negation union quadrants

[`cos-aw-union` 6](../../upstream/structures.html#cos-aw-union).
Union(N(∅),{∅})=A; Union(N(∅),{u,∅})=A;
Union(N(∅),{u})=N(∅); Union(N(∅),{})=N(∅).
These separate the namespace-test-is-absent case from B-087's present t case.

### B-089 — every intersection branch

[`cos-aw-intersect`](../../upstream/structures.html#cos-aw-intersect).
Intersect(N(t),N(t))=N(t); Intersect(A,{u})={u};
Intersect(N(t),{t,u,∅})={u}; Intersect(N(∅),{u,∅})={u};
Intersect({t,u},{u,v})={u}; Intersect({t},{u})={};
Intersect(N(t),N(u)) is **unexpressible**; Intersect(N(t),N(∅))=N(t).
Swap operands for every asymmetric-looking form. All-disallowing {} is a
present expressible wildcard; it is not absent. Integrating the unexpressible
intersection through two contributing groups is SC− regardless of whether
an instance happens to use no attributes.

### B-090 — iterative operations and intermediate expressibility

[§3.10.6](../../upstream/structures.html#cos-aw-intersect).
Union({t},{u},{∅}) folds to {t,u,∅}; intersection({t,u},{u,v},{u}) folds to {u}.
Union(N(t),{∅},{t}) encounters an unexpressible first pair under the stated
iterative procedure, while first unioning {∅} with {t} gives A after combining
N(t). Intersect(N(t),N(u),{v}) likewise has an unexpressible first pair, while
first combining N(u) with {v} yields {v}. Record the source-order/choice-of-
fold tension: the Recommendation's multi-value wording requires expressible
intermediates but does not provide a universal choice strategy. Do not label
every ordering a mandatory rejection or silently approximate intermediates.

### B-091 — wildcard property domains and no per-wrapper validation

[§3.10.1/6](../../upstream/structures.html#Wildcard_details).
A wildcard requires a namespace constraint in one of its three forms and
processing exactly strict/lax/skip. Wrong process=`validate`, constraint pair
(`not`, integer 7), or annotation of wrong component kind fails CM shape.
An absent annotation, A, N(∅), and finite {} are individually legal component
values. Attribute-use/group/model-group/particle/wildcard wrappers do not add
PSVI contribution rules where their own subsection says none; assessment is
observed through the contained declaration/type and the enclosing element.
Local declaration scope, reference identity and wildcard processing remain
observable CM facts rather than extra serial XML elements.

## All 25 post-normalization dispatch cells

Each case below is a direct component relation, with no containing same-kind
group that would normalize away the top-level kind. Group terms have at least
two particles. All unspecified leaf types/properties follow the shared sketch;
all listed ranges are exact. Forbidden means the §3.9.6 dispatch has no permitted
mapping, **not** merely that the supplied positive language sample fails.

### B-092 — element:element

[`dispatch`](../../upstream/structures.html#cos-particle-restrict).
R=e(a,1,1), B=e(a,0,2): Rel+ NameAndTypeOK. R=e(b,1,1) with the same B: Rel−.
B-065–B-071 independently distinguish every applicable property premise.

### B-093 — element:wildcard

R=e(a,1,1), B=W({t},skip,0,2): Rel+ NSCompat. Change only B set to {u}: Rel−.

### B-094 — element:all

R=e(a,1,1), B=all([a!,b?],1,1): Rel+ RecurseAsIfGroup. Make b required: Rel−.

### B-095 — element:choice

R=e(a,1,1), B=choice([a!,b!],1,1): Rel+ RecurseAsIfGroup/RecurseLax.
R=e(c,1,1) with same B: Rel−.

### B-096 — element:sequence

R=e(a,1,1), B=seq([a!,b?],1,1): Rel+ RecurseAsIfGroup/Recurse.
Make b required: Rel−.

### B-097 — wildcard:element

R=W({t},skip,1,1), B=e(a,1,1): **Rel− forbidden**, even if the only observed
instance happened to be a. The wildcard is not a declared a element.

### B-098 — wildcard:wildcard

R=W({t},strict,1,1), B=W({t,u},lax,0,2): Rel+ NSSubset.
R.process=skip with ordinary B: Rel−. B-074 isolates other conjuncts.

### B-099 — wildcard:all

R=W({t},strict,1,1), B=all([a!,b?],1,1): **Rel− forbidden**.

### B-100 — wildcard:choice

R=W({t},strict,1,1), B=choice([a!,b!],1,1): **Rel− forbidden**.

### B-101 — wildcard:sequence

R=W({t},strict,1,1), B=seq([a!,b?],1,1): **Rel− forbidden**.

### B-102 — all:element

R=all([a!,b?],1,1), B=e(a,1,1): **Rel− forbidden**. Do not simplify this
two-particle all to a singleton merely because b is optional.

### B-103 — all:wildcard

R=all([a!,b?],1,1), B=W({t},skip,0,2): Rel+ NSRecurseCheckCardinality.
B.max=1: Rel− total range. B-075 supplies namespace failure too.

### B-104 — all:all

R=all([a!,b?],1,1), B=all([a!,b?],0,1): Rel+ Recurse in listed order.
Reverse R's children: Rel− ordered mapping, regardless of instance permutation.

### B-105 — all:choice

R=all([a!,b?],1,1), B=choice([a!,b!],1,2): **Rel− forbidden** after retaining
the nonpointless all, even if some language-containment argument seems plausible.

### B-106 — all:sequence

R=all([a!,b?],1,1), B=seq([a!,b?],1,1): **Rel− forbidden**.

### B-107 — choice:element

R=choice([a!,b!],1,1), B=e(a,1,1): **Rel− forbidden**.

### B-108 — choice:wildcard

R=choice([a!,b!],1,1), B=W({t},skip,1,1): Rel+ NSRecurseCheckCardinality.
Change b namespace to u: Rel− member recursion.

### B-109 — choice:all

R=choice([a!,b!],1,1), B=all([a?,b?],1,1): **Rel− forbidden**.

### B-110 — choice:choice

R=choice([a!,c!],1,1), B=choice([a!,b!,c!],1,2): Rel+ RecurseLax.
Reverse R order: Rel−. Unmapped required B.b does not itself cause failure.

### B-111 — choice:sequence

R=choice([a!,b!],1,1), B=seq([a?,b?],1,1): **Rel− forbidden**.

### B-112 — sequence:element

R=seq([a!,b?],1,1), B=e(a,1,1): **Rel− forbidden**. The two-member sequence
is not pointless at a root relation merely because b is optional.

### B-113 — sequence:wildcard

R=seq([a!,b!],1,1), B=W({t},skip,1,2): Rel+ NSRecurseCheckCardinality.
B.max=1: Rel−. Total count failure is independent of each child relation.

### B-114 — sequence:all

R=seq([b!,a!],1,1), B=all([a!,b!,c?],1,1): Rel+ RecurseUnordered.
R=[a!,a!,b!] fails injectivity. Making c required fails unmatched-base rule.

### B-115 — sequence:choice

R=seq([a!,b!],1,2), B=choice([a!,b!],2,4): Rel+ MapAndSum.
R outer max=3 gives product 2..6 and Rel−, independent of leaf compatibility.

### B-116 — sequence:sequence

R=seq([a!,c!],1,1), B=seq([a!,b?,c!],1,1): Rel+ Recurse.
Make b required or reverse R order: Rel− for different premises.

## Integration witnesses and source boundaries

### B-117 — missing subcomponents and legal property alternatives

[§5.3](../../upstream/structures.html#conformance-missing) qualifies property-
correctness rules. Give R a reference to missing group G instead of its required
model term: do not map it to seq([]). When assessment requires it, the governing
component is unusable and full assessment cannot be claimed. A missing required
attribute declaration/type is analogous; it is not a wrong-kind component.
For each named group/complex type property, absent **name** is permitted only
where its tableau permits anonymous types, absent namespace is a legal domain
alternative, absent optional annotation/wildcard is legal, empty ordered
particle/use sets are legal, and empty string is not an absent namespace.
A named model/attribute group cannot acquire an absent required name simply
because anonymous complex types can. These are CM/assessment distinctions.

### B-118 — explicit interacting product witnesses

Use the fixed following finite products, not pairwise sampling:

- Base attribute x optional/required × R x optional/required × B fixed absent/
  fixed("a") × R constraint absent/fixed("a")/fixed("b"). All 24 component
  pairs have matching xs:string name/type and empty content. Expected relation
  is conjunction of B-035 and B-037: required cannot relax and a base fixed
  needs the same R fixed. A failure in one condition does not prove the other.
- Attribute-wildcard restriction with an ordinary base: B={u}, processing
  strict/lax/skip × R processing strict/lax/skip × R namespace {u}/{u,v}.
  These 18 pairs separate namespace and strength: {u,v} fails subset regardless
  of processing; {u} also requires R processing to be at least as strong as B.
  Against the actual predefined anyType, B is fixed at A/lax. R processing
  strict/lax/skip × R namespace {u}/{u,v} gives six more pairs, all permitted
  under the attribute-strength exception with the other premises satisfied.
  Varying anyType's B processing to strict or skip describes 12 impossible
  setups, not missing tests: [its predefined shape](../../upstream/structures.html#ur-type-itself)
  is fixed. A user type copying A/lax retains ordinary-base strength checks.
  The [attribute rule's base-type identity exception](../../upstream/structures.html#derivation-ok-restriction)
  is distinct from the [particle rule's wildcard identity exception](../../upstream/structures.html#rcase-NSSubset).
- B mixed/EO × R mixed/EO × identical/narrowed/invalid-name particle. Use
  B.seq([a!,b?],1,1), narrowed R.seq([a!],1,1), invalid R.seq([c!],1,1).
  EO→mixed always fails category; other categories still require permitted
  particle restriction. For mixed→EO the R text restriction is observable V−
  on ordinary character content but does not change ordered element matching.
- AG first wildcard strict/skip × local absent/lax × namespace intersection
  {u}/{} × unknown u:x present/absent. Processing comes from local if present,
  otherwise first contributing group; {} denies u:x under every processing
  value. An absent wildcard and a present {} both deny every unmatched ordinary
  attribute, so instance mutations under that same model cannot distinguish
  them. Use component observation or a separate schema construction/derivation
  contrast to establish wildcard presence.
- Restriction dispatch after substitution expansion × member abstractness ×
  head `block="substitution"` present/absent × R/B head on neither/R/B/both sides.
  Use h with members a,b, all named xs:string. A member's own block does not
  filter that member from h's group; type-method filtering needs a separate
  nonempty derivation path. The expanded choice consists only of actual members.
  Ordered-choice expectations are conditional on a permitted established order
  as recorded below; do not claim a universal order absent from the source.

## Unresolved specification tensions; no invented rejection

- [`cos-ct-derived-ok` identity note](../../upstream/structures.html#no-identity)
  explicitly leaves some component identity cases incomplete. Tests requiring
  equality of separately built anonymous components need a declared permitted
  identity choice; named/global identity and identity by construction do not.
- [`cos-particle-restrict` 2.1](../../upstream/structures.html#cos-particle-restrict)
  turns a substitution **set** into a choice particle list without specifying
  its order, while RecurseLax imposes order preservation. B-060/B-118 must keep
  that missing ordering premise visible, rather than inferring source order or
  arbitrary sorting from a particular implementation. [I-005](../../INTERPRETATIONS.md#i-005--implicit-substitution-choice-order)
  records the historical drafting disposition; it supplies no approved waiver.
- [`sic-attrDefault`](../../upstream/structures.html#sic-attrDefault) predicates
  synthesis on a nonabsent **use** constraint, then refers to an effective
  constraint whose definition also permits a declaration fallback. The ref-use
  mapping only supplies its explicitly present default/fixed attributes. B-044
  records use-constraint-present outputs and keeps absent-use/global-default
  behavior as a text-integration question; do not manufacture an extra mapping
  step silently. This does not affect the direct effective-constraint checks
  of B-037.
- [Multi-value union/intersection](../../upstream/structures.html#cos-aw-union)
  requires expressible pairwise intermediates but does not completely specify
  operand selection. B-090 distinguishes exact binary outputs from folding
  choices. A final set that is mathematically expressible is not by itself a
  proof that an arbitrary prescribed intermediate was expressible.
- The surprising canonical-string `cvc-au`, R-identity-constraint-subset,
  ordered all/choice mapping, and empty-choice numeric helper conditions above
  are deliberate literal oracles from the retained edition. E1-56 does not
  change them. A later standards correction would require new authority before
  changing these expectations; common processor agreement is insufficient.

No XML fixture execution, independent PSVI capture, implementation comparison,
or corpus coverage result is claimed here. The 282 branch dispositions prove
that each indexed decision has been examined and assigned concrete examples;
they do not prove that every unbounded recursive depth/count has been executed.

### B-119 — exact predefined anyType shape

[§3.4.7](../../upstream/structures.html#ur-type-itself). The predefined anyType
has name anyType/namespace XSD, base itself, method restriction, empty final,
block and attribute-use sets, abstract false, mixed content with outer sequence
1..1 containing W(A,lax,0,∞), and attribute wildcard A/lax. Arbitrary text and
an undeclared well-formed u:a child or u:x attribute are allowed with lax
assessment. If u:a is known as xs:integer, text `bad` fails; the predefined
wildcards are not skip. A user type copying the same wildcard values still
has an ordinary base chain and does not acquire anyType identity exceptions.

### B-120 — special xsi attributes and nil still check attributes

[`cvc-complex-type`](../../upstream/structures.html#cvc-complex-type).
r is nillable=true, declared with empty concrete R having required x:string
and no wildcard. `<t:r x="a" xsi:nil="true"/>` is V+; omit x: V− despite nil
suppressing ordinary content checks. xsi:type naming the same permitted R and
well-formed xsi:schemaLocation/noNamespaceSchemaLocation attributes do not
require ordinary declared attribute uses for those four built-in names.
xsi:other is not covered by that exception and fails with no wildcard. Make R
abstract: direct actual-type assessment fails even on a nilled r; use a legal
concrete derived D through xsi:type to regain a permissible actual type, while
retaining x's requirement. Type block/prohibited substitution gates that use
as in B-001/B-042; abstract base types themselves remain legal schema bases.

### B-121 — wildcard ID attributes and absent declared ID use

[`cvc-complex-type` 5](../../upstream/structures.html#cvc-complex-type).
Global {u}i and {u}j are xs:ID; R empty, no declared attributes, wildcard
{u}/strict. Zero such attributes is V+; u:i=`a` alone V+; u:i=`a` and u:j=`b`
V− for more than one wild ID, without duplicate-ID strings masking the count.
Add optional declared x:ID use to R: u:i alone now V− even when x is absent.
Remove the wildcard ID and provide x alone: V+. Repeat wildcard lax with both
u declarations available: same wild-ID rules. With skip they do not enter the
wild-ID set through schema assessment; their declaration names being known is
not enough to override skip. A wildcard attribute declared xs:string is not a
wild ID. A restriction of ID remains in the wild-ID category.

## Unnumbered property and prose audit

All [32 indexed component properties](../../inventory.json) in these sections
have the following concrete owners. Required/optional/domain distinctions are
part of the cases, not an instruction to trust a structurally similar object.
Some PSVI properties do not use the component-property class in the inventory;
they are explicitly included here too.

| Source property or unnumbered condition | Concrete cases and observation |
| --- | --- |
| ct-name, ct-target_namespace | B-002, B-027, B-117: named/anonymous and namespace absent/present; invalid absent name on named groups is not generalized from types. CM. |
| ct-base_type_definition, derivation_method | B-018, B-025, B-027, B-034, B-042, B-119: simple/complex base, extension/restriction, invalid cycle, legal anyType exception. SC/CM/Rel. |
| ct-final | B-001, B-002, B-029, B-034: explicit/defaulted/method subsets and no inheritance. CM/SC. |
| ct-abstract | B-002, B-027, B-120: concrete versus abstract actual type; derived concrete type and nil do not erase other requirements. CM/V. |
| ct-attribute_declarations | B-003–B-005, B-028, B-030, B-035–B-039: local/group/base set membership and identity; duplicate/ID and required/fixed rules. CM/Rel/V. |
| ct-attribute_wildcard | B-006–B-013, B-031, B-040: absent/present, local/group/base metadata and namespace algebra. CM/Rel/V. |
| content_type | B-014–B-026, B-032, B-041: empty/simple/EO/mixed; category mapping and restrictions. CM/V. |
| ct-exact | B-001, B-042, B-120: applicable method filtering and derived-type substitution effect. CM/Rel/V. |
| ct-annotations | B-002, B-008–B-012: collection at all mapped source locations and exact wildcard annotation provenance. CM. |
| required, attribute, au-value_constraint | B-035, B-037, B-039, B-043, B-044: boolean/declaration domain, no prohibited-use component, use/declaration constraints. CM/Rel/V/P. |
| ag-name, ag-target_namespace, ag-annotation | B-045, B-117: required name; namespace absent/present; optional annotation; wrong-kind property. CM. |
| ag-attribute_declarations, ag-attribute_wildcard | B-004, B-007–B-010, B-045: expanded sets, duplicate names/ID, wildcard computation and circularity. CM/SC. |
| mg-name, mg-target_namespace, model_group, mg-annotation | B-046, B-117: named definition and its group, reference identity and range, optional annotation. CM. |
| compositor, particles, amg-annotation | B-047–B-053, B-058, B-061–B-063: ordered particle list/domain, cycle, all bounds, annotation exception in recursive identity comparison. CM/SC/Rel/V. |
| p-min_occurs, p-max_occurs, term | B-052–B-057, B-064: finite/unbounded ranges, source zero elimination, wrong-kind/absent term and counts versus group repetitions. CM/Rel/V. |
| namespace_constraint, process_contents, w-annotation | B-006–B-013, B-074, B-081–B-091: all legal forms and invalid values; admission, strength and annotation provenance. CM/Rel/V. |
| sic-attrDefault's added local name/namespace, schema normalized value/default, validity, attempted, schema specified, type/member alternatives | B-044: explicit use constraint, missing/present/required input and canonical typed output; retain the separate should-also recommendation for type/member alternatives and independently applicable contributor entries. P. Validation context is the nearest ancestor element carrying schema information; in the ordinary whole-document case it is the assessment root. |
| cvc-au's nonnumbered canonical lexical comparison | B-043: boolean true versus 1 keeps type-equal values separate from lexical-use match. V. |
| Group definitions do not validate independently or add standalone PSVI | B-004, B-046, B-091: their expanded uses/terms affect enclosing assessment, without an extra instance wrapper. CM/V/P. |
| Empty model-group validation and empty subsequence partitions | B-048, B-053, B-057, B-080: empty language versus zero effective numeric range is explicit. Rel/V. |
| Effective-total-range arithmetic and finite/unbounded maxima | B-052, B-053, B-075, B-079: exact numeric outputs and conservative MapAndSum formula. CM/Rel. |
| Complete/order-preserving/injective mapping definitions | B-076–B-079 and B-094–B-116: reversed order, duplicate target, missing required base, and noninjective allowed MapAndSum. Rel. |
| Type names versus element names; type/attribute symbol spaces and uniqueness | B-002, B-028, B-050: names are component identifiers, not element-name requirements; local duplicate declarations are separately constrained. CM/SC/V. |
| Normalization before particle-kind dispatch and every forbidden matrix cell | B-059–B-063, B-092–B-116: identity, substitution set expansion and literal pointless-group conditions. Rel. |
| Wildcard namespace relation ignores substitution head namespace | B-056, B-082, B-083: actual u:a versus head t:h. V. |
| Wildcard processContents assignment versus downstream validity | B-083, B-121: context none/mustFind/skip and known-invalid/unknown attribute or element. CM/V/P. |
| More than two namespace operands | B-090: finite exact folds and unresolved operand-selection tension. CM/SC/O. |
| Predefined ur-type and identity limits | B-027, B-042, B-074, B-119: specified self-base/lax wildcards and exception by identity. CM/Rel/V. |

### Unnumbered named rules

| Rule without numbered inventory branches in this packet | Concrete owner |
| --- | --- |
| `cvc-au` | B-043: explicit canonical-string comparison and fixed declaration/use coherence. |
| `sic-attrDefault` | B-044: every specified added-attribute property and exact admitting predicate. |
| `src-model_group_defn`, `mgd-props-correct` | B-046, B-049, B-117: definition/ref mapping, contextual grammar and required group shape. |
| `src-model_group` | B-047, B-049, B-054: model and particle constraints both apply. |
| `cos-nonambig` | B-051: actual/implicit overlaps and positional distinguishability. |
| `cos-seq-range`, `cos-choice-range` | B-052, B-053: exact arithmetic and zero/unbounded branches. |
| `rcase-RecurseAsIfGroup` | B-073: exact 1..1 wrapper and each target compositor. |
| `src-wildcard`, `w-props-correct` | B-054, B-081, B-091: particle/mapping constraints and wildcard property domain. |

## Numbered-branch disposition index

Every row uses the source-order synthetic key. Published clause numbering is
recorded in the explanation because several property mappings restart at 1.
The cases attached to a composite apply to its child decision tree together;
a composite does not create an additional independent rejection rule.

| Inventory key | Cases | Disposition | Explanation |
| --- | --- | --- | --- |
| `structures/declare-type/branch/001` | B-001 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/002` | B-001 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/003` | B-001 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/004` | B-003 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/005` | B-004 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/006` | B-005 | composite | Published 3; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/007` | B-005 | reviewed | Published 3.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/008` | B-005 | reviewed | Published 3.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/009` | B-006 | composite | Published 1; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/010` | B-006 | reviewed | Published 1.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/011` | B-006 | reviewed | Published 1.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/012` | B-007, B-008, B-009 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/013` | B-007 | reviewed | Published 2.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/014` | B-008, B-009 | composite | Published 2.2; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/015` | B-008 | reviewed | Published 2.2.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/016` | B-009 | reviewed | Published 2.2.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/017` | B-010, B-011, B-012, B-013 | composite | Published 3; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/018` | B-010 | reviewed | Published 3.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/019` | B-011, B-012, B-013 | composite | Published 3.2; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/020` | B-011, B-013 | composite | Published 3.2.1; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/021` | B-011, B-012 | reviewed | Published 3.2.1.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/022` | B-013 | reviewed | Published 3.2.1.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/023` | B-011, B-012, B-013 | composite | Published 3.2.2; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/024` | B-011, B-012 | composite | Published 3.2.2.1; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/025` | B-011 | reviewed | Published 3.2.2.1.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/026` | B-012 | reviewed | Published 3.2.2.1.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/027` | B-013 | reviewed | Published 3.2.2.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/028` | B-014, B-015 | composite | Published 1; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/029` | B-014 | reviewed | Published 1.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/030` | B-015 | reviewed | Published 1.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/031` | B-016 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/032` | B-017 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/033` | B-018 | reviewed | Published 4; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/034` | B-003 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/035` | B-004 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/036` | B-005 | composite | Published 3; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/037` | B-005 | reviewed | Published 3.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/038` | B-005 | reviewed | Published 3.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/039` | B-019 | composite | Published 1; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/040` | B-019 | reviewed | Published 1.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/041` | B-019 | reviewed | Published 1.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/042` | B-019 | reviewed | Published 1.3; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/043` | B-020, B-021, B-022 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/044` | B-020, B-021 | composite | Published 2.1; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/045` | B-020, B-021 | reviewed | Published 2.1.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/046` | B-020, B-021 | reviewed | Published 2.1.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/047` | B-020, B-021 | reviewed | Published 2.1.3; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/048` | B-021 | reviewed | Published 2.1.4; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/049` | B-020 | reviewed | Published 2.1.5; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/050` | B-022 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/051` | B-023, B-024 | composite | Published 3; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/052` | B-023 | composite | Published 3.1; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/053` | B-023 | reviewed | Published 3.1.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/054` | B-023 | composite | Published 3.1.2; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/055` | B-023 | reviewed | Published 3.1.2.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/056` | B-023 | reviewed | Published 3.1.2.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/057` | B-024 | composite | Published 3.2; container is decided by its cited child witnesses. |
| `structures/declare-type/branch/058` | B-024 | reviewed | Published 3.2.1; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/059` | B-024 | reviewed | Published 3.2.2; cited setup and mutations distinguish this condition. |
| `structures/declare-type/branch/060` | B-024 | reviewed | Published 3.2.3; cited setup and mutations distinguish this condition. |
| `structures/src-ct/branch/001` | B-025 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/src-ct/branch/002` | B-026 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/src-ct/branch/003` | B-026 | composite | Published 2.1; container is decided by its cited child witnesses. |
| `structures/src-ct/branch/004` | B-014, B-015, B-017 | reviewed | Published 2.1.1; cited setup and mutations distinguish this condition. |
| `structures/src-ct/branch/005` | B-016 | reviewed | Published 2.1.2; cited setup and mutations distinguish this condition. |
| `structures/src-ct/branch/006` | B-018 | reviewed | Published 2.1.3; cited setup and mutations distinguish this condition. |
| `structures/src-ct/branch/007` | B-016, B-026 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/src-ct/branch/008` | B-027, B-028, B-029, B-030, B-031, B-032, B-033, B-034, B-035, B-036, B-037, B-038, B-039, B-040, B-041, B-042 | composite | Published 3; cross-reference to complete complex property/derivation constraints, whose separate premises have the cited witnesses. |
| `structures/src-ct/branch/009` | B-008, B-009, B-089 | reviewed | Published 4; cited setup and mutations distinguish this condition. |
| `structures/cvc-complex-type/branch/001` | B-120 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cvc-complex-type/branch/002` | B-020, B-021, B-023, B-120 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/cvc-complex-type/branch/003` | B-020 | reviewed | Published 2.1; cited setup and mutations distinguish this condition. |
| `structures/cvc-complex-type/branch/004` | B-014, B-015, B-017, B-018 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/cvc-complex-type/branch/005` | B-023 | reviewed | Published 2.3; cited setup and mutations distinguish this condition. |
| `structures/cvc-complex-type/branch/006` | B-022, B-024, B-048 | reviewed | Published 2.4; cited setup and mutations distinguish this condition. |
| `structures/cvc-complex-type/branch/007` | B-003, B-004, B-043, B-083, B-120 | composite | Published 3; container is decided by its cited child witnesses. |
| `structures/cvc-complex-type/branch/008` | B-003, B-043 | reviewed | Published 3.1; cited setup and mutations distinguish this condition. |
| `structures/cvc-complex-type/branch/009` | B-006, B-083 | composite | Published 3.2; container is decided by its cited child witnesses. |
| `structures/cvc-complex-type/branch/010` | B-006 | reviewed | Published 3.2.1; cited setup and mutations distinguish this condition. |
| `structures/cvc-complex-type/branch/011` | B-082, B-083 | reviewed | Published 3.2.2; cited setup and mutations distinguish this condition. |
| `structures/cvc-complex-type/branch/012` | B-004, B-039, B-120 | reviewed | Published 4; cited setup and mutations distinguish this condition. |
| `structures/cvc-complex-type/branch/013` | B-121 | composite | Published 5; container is decided by its cited child witnesses. |
| `structures/cvc-complex-type/branch/014` | B-121 | reviewed | Published 5.1; cited setup and mutations distinguish this condition. |
| `structures/cvc-complex-type/branch/015` | B-121 | reviewed | Published 5.2; cited setup and mutations distinguish this condition. |
| `structures/ct-props-correct/branch/001` | B-027, B-117 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/ct-props-correct/branch/002` | B-018, B-027 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/ct-props-correct/branch/003` | B-027, B-119 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/ct-props-correct/branch/004` | B-028 | reviewed | Published 4; cited setup and mutations distinguish this condition. |
| `structures/ct-props-correct/branch/005` | B-028 | reviewed | Published 5; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/001` | B-029, B-030, B-031, B-032, B-033 | composite | Published 1; container is decided by its cited child witnesses. |
| `structures/cos-ct-extends/branch/002` | B-029 | reviewed | Published 1.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/003` | B-030 | reviewed | Published 1.2; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/004` | B-031 | reviewed | Published 1.3; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/005` | B-032 | composite | Published 1.4; container is decided by its cited child witnesses. |
| `structures/cos-ct-extends/branch/006` | B-032 | reviewed | Published 1.4.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/007` | B-032 | reviewed | Published 1.4.2; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/008` | B-032 | composite | Published 1.4.3; container is decided by its cited child witnesses. |
| `structures/cos-ct-extends/branch/009` | B-032 | reviewed | Published 1.4.3.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/010` | B-032 | composite | Published 1.4.3.2; container is decided by its cited child witnesses. |
| `structures/cos-ct-extends/branch/011` | B-032 | reviewed | Published 1.4.3.2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/012` | B-032 | composite | Published 1.4.3.2.2; container is decided by its cited child witnesses. |
| `structures/cos-ct-extends/branch/013` | B-032 | reviewed | Published 1.4.3.2.2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/014` | B-032, B-058 | reviewed | Published 1.4.3.2.2.2; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/015` | B-033 | reviewed | Published 1.5; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/016` | B-018, B-029, B-032 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/cos-ct-extends/branch/017` | B-018, B-032 | reviewed | Published 2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-extends/branch/018` | B-029 | reviewed | Published 2.2; the extension-present negative is outside Part 2's legal simple-type component domain. |
| `structures/derivation-ok-restriction/branch/001` | B-034 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/002` | B-035, B-036, B-037, B-038 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/003` | B-035, B-036, B-037 | composite | Published 2.1; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/004` | B-035 | composite | Published 2.1.1; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/005` | B-035 | reviewed | Published 2.1.1.1; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/006` | B-035 | reviewed | Published 2.1.1.2; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/007` | B-036 | reviewed | Published 2.1.2; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/008` | B-037 | composite | Published 2.1.3; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/009` | B-037 | reviewed | Published 2.1.3.1; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/010` | B-037 | reviewed | Published 2.1.3.2; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/011` | B-038 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/012` | B-039 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/013` | B-040 | composite | Published 4; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/014` | B-040 | reviewed | Published 4.1; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/015` | B-040, B-084 | reviewed | Published 4.2; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/016` | B-040, B-074 | reviewed | Published 4.3; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/017` | B-041 | composite | Published 5; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/018` | B-041 | reviewed | Published 5.1; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/019` | B-041 | composite | Published 5.2; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/020` | B-041 | reviewed | Published 5.2.1; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/021` | B-041 | composite | Published 5.2.2; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/022` | B-041 | reviewed | Published 5.2.2.1; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/023` | B-041 | reviewed | Published 5.2.2.2; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/024` | B-041 | composite | Published 5.3; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/025` | B-041 | reviewed | Published 5.3.1; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/026` | B-041 | composite | Published 5.3.2; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/027` | B-041 | reviewed | Published 5.3.2.1; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/028` | B-041 | reviewed | Published 5.3.2.2; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/029` | B-041 | composite | Published 5.4; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/030` | B-041 | composite | Published 5.4.1; container is decided by its cited child witnesses. |
| `structures/derivation-ok-restriction/branch/031` | B-041 | reviewed | Published 5.4.1.1; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/032` | B-041 | reviewed | Published 5.4.1.2; cited setup and mutations distinguish this condition. |
| `structures/derivation-ok-restriction/branch/033` | B-041, B-065, B-076 | reviewed | Published 5.4.2; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-derived-ok/branch/001` | B-042 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-derived-ok/branch/002` | B-042 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/cos-ct-derived-ok/branch/003` | B-042 | reviewed | Published 2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-derived-ok/branch/004` | B-042 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-derived-ok/branch/005` | B-042 | composite | Published 2.3; container is decided by its cited child witnesses. |
| `structures/cos-ct-derived-ok/branch/006` | B-042 | reviewed | Published 2.3.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-derived-ok/branch/007` | B-042 | composite | Published 2.3.2; container is decided by its cited child witnesses. |
| `structures/cos-ct-derived-ok/branch/008` | B-042 | reviewed | Published 2.3.2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ct-derived-ok/branch/009` | B-042 | reviewed | Published 2.3.2.2; cited setup and mutations distinguish this condition. |
| `structures/au-props-correct/branch/001` | B-043, B-117 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/au-props-correct/branch/002` | B-043 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/src-attribute_group/branch/001` | B-045 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/src-attribute_group/branch/002` | B-045, B-089 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/src-attribute_group/branch/003` | B-045 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/ag-props-correct/branch/001` | B-045, B-117 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/ag-props-correct/branch/002` | B-045 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/ag-props-correct/branch/003` | B-045 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/cvc-model-group/branch/001` | B-048 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cvc-model-group/branch/002` | B-048 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/cvc-model-group/branch/003` | B-048 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/mg-props-correct/branch/001` | B-047, B-117 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/mg-props-correct/branch/002` | B-047 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/cos-all-limited/branch/001` | B-049 | composite | Published 1; container is decided by its cited child witnesses. |
| `structures/cos-all-limited/branch/002` | B-049 | reviewed | Published 1.1; cited setup and mutations distinguish this condition. |
| `structures/cos-all-limited/branch/003` | B-049 | reviewed | Published 1.2; cited setup and mutations distinguish this condition. |
| `structures/cos-all-limited/branch/004` | B-049 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/cos-element-consistent/branch/001` | B-050 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cos-element-consistent/branch/002` | B-050 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/cos-element-consistent/branch/003` | B-050 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/cvc-particle/branch/001` | B-055 | composite | Published 1; container is decided by its cited child witnesses. |
| `structures/cvc-particle/branch/002` | B-055 | reviewed | Published 1.1; cited setup and mutations distinguish this condition. |
| `structures/cvc-particle/branch/003` | B-055 | reviewed | Published 1.2; cited setup and mutations distinguish this condition. |
| `structures/cvc-particle/branch/004` | B-055, B-082 | reviewed | Published 1.3; cited setup and mutations distinguish this condition. |
| `structures/cvc-particle/branch/005` | B-056 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/cvc-particle/branch/006` | B-056 | reviewed | Published 2.1; cited setup and mutations distinguish this condition. |
| `structures/cvc-particle/branch/007` | B-056 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/cvc-particle/branch/008` | B-056 | composite | Published 2.3; container is decided by its cited child witnesses. |
| `structures/cvc-particle/branch/009` | B-056 | reviewed | Published 2.3.1; cited setup and mutations distinguish this condition. |
| `structures/cvc-particle/branch/010` | B-056 | reviewed | Published 2.3.2; cited setup and mutations distinguish this condition. |
| `structures/cvc-particle/branch/011` | B-056 | reviewed | Published 2.3.3; cited setup and mutations distinguish this condition. |
| `structures/cvc-particle/branch/012` | B-057 | composite | Published 3; container is decided by its cited child witnesses. |
| `structures/cvc-particle/branch/013` | B-057 | reviewed | Published 3.1; cited setup and mutations distinguish this condition. |
| `structures/cvc-particle/branch/014` | B-057 | reviewed | Published 3.2; cited setup and mutations distinguish this condition. |
| `structures/cvc-particle/branch/015` | B-057 | reviewed | Published 3.3; cited setup and mutations distinguish this condition. |
| `structures/p-props-correct/branch/001` | B-054, B-117 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/p-props-correct/branch/002` | B-054 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/p-props-correct/branch/003` | B-054 | reviewed | Published 2.1; cited setup and mutations distinguish this condition. |
| `structures/p-props-correct/branch/004` | B-054 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-extend/branch/001` | B-058 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-extend/branch/002` | B-058 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-restrict/branch/001` | B-059 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-restrict/branch/002` | B-060, B-061, B-062, B-063, B-092, B-093, B-094, B-095, B-096, B-097, B-098, B-099, B-100, B-101, B-102, B-103, B-104, B-105, B-106, B-107, B-108, B-109, B-110, B-111, B-112, B-113, B-114, B-115, B-116 | composite | Published 2; all 25 post-normalization dispatch cells plus normalization/substitution witnesses. |
| `structures/cos-particle-restrict/branch/003` | B-060 | reviewed | Published 2.1; actual-member expansion required; list ordering tension retained in B-060. |
| `structures/cos-particle-restrict/branch/004` | B-061, B-062, B-063 | composite | Published 2.2; container is decided by its cited child witnesses. |
| `structures/cos-particle-restrict/branch/005` | B-061 | reviewed | Published 2.2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-restrict/branch/006` | B-061 | composite | Published 2.2.2; container is decided by its cited child witnesses. |
| `structures/cos-particle-restrict/branch/007` | B-061 | reviewed | Published 2.2.2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-restrict/branch/008` | B-061 | composite | Published 2.2.2.2; container is decided by its cited child witnesses. |
| `structures/cos-particle-restrict/branch/009` | B-061 | reviewed | Published 2.2.2.2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-restrict/branch/010` | B-061 | reviewed | Published 2.2.2.2.2; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-restrict/branch/011` | B-062 | reviewed | Published 2.2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-restrict/branch/012` | B-062 | reviewed | Published 2.2.2; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-restrict/branch/013` | B-063 | reviewed | Published 2.2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-restrict/branch/014` | B-063 | composite | Published 2.2.2; container is decided by its cited child witnesses. |
| `structures/cos-particle-restrict/branch/015` | B-063 | reviewed | Published 2.2.2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-restrict/branch/016` | B-063 | composite | Published 2.2.2.2; container is decided by its cited child witnesses. |
| `structures/cos-particle-restrict/branch/017` | B-063 | reviewed | Published 2.2.2.2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-particle-restrict/branch/018` | B-063 | reviewed | Published 2.2.2.2.2; cited setup and mutations distinguish this condition. |
| `structures/range-ok/branch/001` | B-064 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/range-ok/branch/002` | B-064 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/range-ok/branch/003` | B-064 | reviewed | Published 2.1; cited setup and mutations distinguish this condition. |
| `structures/range-ok/branch/004` | B-064 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/rcase-NameAndTypeOK/branch/001` | B-065 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/rcase-NameAndTypeOK/branch/002` | B-064, B-065 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/rcase-NameAndTypeOK/branch/003` | B-066, B-067, B-068, B-069, B-070, B-071 | composite | Published 3; container is decided by its cited child witnesses. |
| `structures/rcase-NameAndTypeOK/branch/004` | B-066 | reviewed | Published 3.1; same-name globals share identity; independent-property mutations infeasible. |
| `structures/rcase-NameAndTypeOK/branch/005` | B-067, B-068, B-069, B-070, B-071 | composite | Published 3.2; container is decided by its cited child witnesses. |
| `structures/rcase-NameAndTypeOK/branch/006` | B-067 | reviewed | Published 3.2.1; cited setup and mutations distinguish this condition. |
| `structures/rcase-NameAndTypeOK/branch/007` | B-068 | reviewed | Published 3.2.2; cited setup and mutations distinguish this condition. |
| `structures/rcase-NameAndTypeOK/branch/008` | B-069 | reviewed | Published 3.2.3; cited setup and mutations distinguish this condition. |
| `structures/rcase-NameAndTypeOK/branch/009` | B-070 | reviewed | Published 3.2.4; cited setup and mutations distinguish this condition. |
| `structures/rcase-NameAndTypeOK/branch/010` | B-071 | reviewed | Published 3.2.5; cited setup and mutations distinguish this condition. |
| `structures/rcase-NSCompat/branch/001` | B-072 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/rcase-NSCompat/branch/002` | B-064, B-072 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/rcase-NSSubset/branch/001` | B-064, B-074 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/rcase-NSSubset/branch/002` | B-074, B-084 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/rcase-NSSubset/branch/003` | B-074 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/rcase-NSRecurseCheckCardinality/branch/001` | B-075 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/rcase-NSRecurseCheckCardinality/branch/002` | B-052, B-053, B-075 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/rcase-Recurse/branch/001` | B-064, B-076 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/rcase-Recurse/branch/002` | B-076 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/rcase-Recurse/branch/003` | B-076 | reviewed | Published 2.1; cited setup and mutations distinguish this condition. |
| `structures/rcase-Recurse/branch/004` | B-076 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/rcase-RecurseLax/branch/001` | B-064, B-077 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/rcase-RecurseLax/branch/002` | B-077 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/rcase-RecurseUnordered/branch/001` | B-064, B-078 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/rcase-RecurseUnordered/branch/002` | B-078 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/rcase-RecurseUnordered/branch/003` | B-078 | reviewed | Published 2.1; cited setup and mutations distinguish this condition. |
| `structures/rcase-RecurseUnordered/branch/004` | B-078 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/rcase-RecurseUnordered/branch/005` | B-078 | reviewed | Published 2.3; cited setup and mutations distinguish this condition. |
| `structures/rcase-MapAndSum/branch/001` | B-079 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/rcase-MapAndSum/branch/002` | B-079 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/cos-group-emptiable/branch/001` | B-080 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cos-group-emptiable/branch/002` | B-052, B-053, B-080 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/declare-openness/branch/001` | B-081 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/declare-openness/branch/002` | B-081 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/cvc-wildcard/branch/001` | B-083 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cvc-wildcard/branch/002` | B-083 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/cvc-wildcard/branch/003` | B-083 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/cvc-wildcard-namespace/branch/001` | B-082 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cvc-wildcard-namespace/branch/002` | B-082 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/cvc-wildcard-namespace/branch/003` | B-082 | reviewed | Published 2.1; cited setup and mutations distinguish this condition. |
| `structures/cvc-wildcard-namespace/branch/004` | B-082 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/cvc-wildcard-namespace/branch/005` | B-082 | reviewed | Published 2.3; cited setup and mutations distinguish this condition. |
| `structures/cvc-wildcard-namespace/branch/006` | B-082 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/cos-ns-subset/branch/001` | B-084 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cos-ns-subset/branch/002` | B-084 | composite | Published 2; container is decided by its cited child witnesses. |
| `structures/cos-ns-subset/branch/003` | B-084 | reviewed | Published 2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ns-subset/branch/004` | B-084 | reviewed | Published 2.2; cited setup and mutations distinguish this condition. |
| `structures/cos-ns-subset/branch/005` | B-084 | composite | Published 3; container is decided by its cited child witnesses. |
| `structures/cos-ns-subset/branch/006` | B-084 | reviewed | Published 3.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ns-subset/branch/007` | B-084 | composite | Published 3.2; container is decided by its cited child witnesses. |
| `structures/cos-ns-subset/branch/008` | B-084 | reviewed | Published 3.2.1; cited setup and mutations distinguish this condition. |
| `structures/cos-ns-subset/branch/009` | B-084 | reviewed | Published 3.2.2; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-union/branch/001` | B-085 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-union/branch/002` | B-085 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-union/branch/003` | B-085 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-union/branch/004` | B-086 | reviewed | Published 4; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-union/branch/005` | B-087 | composite | Published 5; container is decided by its cited child witnesses. |
| `structures/cos-aw-union/branch/006` | B-087 | reviewed | Published 5.1; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-union/branch/007` | B-087 | reviewed | Published 5.2; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-union/branch/008` | B-087 | reviewed | Published 5.3; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-union/branch/009` | B-087 | reviewed | Published 5.4; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-union/branch/010` | B-088 | composite | Published 6; container is decided by its cited child witnesses. |
| `structures/cos-aw-union/branch/011` | B-088 | reviewed | Published 6.1; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-union/branch/012` | B-088 | reviewed | Published 6.2; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-intersect/branch/001` | B-089 | reviewed | Published 1; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-intersect/branch/002` | B-089 | reviewed | Published 2; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-intersect/branch/003` | B-089 | reviewed | Published 3; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-intersect/branch/004` | B-089 | reviewed | Published 4; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-intersect/branch/005` | B-089 | reviewed | Published 5; cited setup and mutations distinguish this condition. |
| `structures/cos-aw-intersect/branch/006` | B-089 | reviewed | Published 6; cited setup and mutations distinguish this condition. |
