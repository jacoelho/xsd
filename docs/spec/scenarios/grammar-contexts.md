# Dated Appendix A: concrete grammar contexts

This document expands [G-001–G-013](grammar.md) into concrete grammar witnesses.
The inputs are the unchanged dated [Structures Appendix A fragment](../upstream/structures.xsd.txt)
and [Datatypes Appendix A fragment](../upstream/datatypes.xsd.txt), indexed in
[inventory.json](../inventory.json). Structures A includes Datatypes A; its
`xml:lang` reference uses the [retained XML namespace schema](../upstream/xml.xsd).
Use the [source record](../SOURCES.md) and [corrected reading](../corrected-reading.md)
for edition and erratum selection. E1-56 changes PSVI prose, not these grammars.

The exclusive ownership table accounts for **849/849 inventory nodes**:
**563 Structures + 286 Datatypes**. It includes helper declarations, reference
sites, derivation wrappers, particles, attributes and the displayed builtin
sketches. An XML syntax node is not automatically an independent requirement.
The table's subtree ownership and resolved context contracts distinguish actual
syntax decisions from wrappers and informative builtin sketches.

`A+` / `A−` mean admitted/rejected by the **dated Appendix A grammar**, including
its datatype admission and schema-document identity constraints. `S+` / `S−`
mean the additional normative representation/component constraints have also
been considered. A+ alone never proves S+. `CM` means a component/property
observation and `P` a PSVI observation; concrete XML examples are descriptions,
not fixtures that have been executed. No implementation or corpus supplies an
oracle here.

## Shared witness envelope and inheritance rules

Unless an example says otherwise, place it in the appropriate context within:

```xml
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:t="urn:t" xmlns:u="urn:u" targetNamespace="urn:t">
  <!-- composition directives, then declarations -->
</xs:schema>
```

Assume distinct, previously declared symbols when referenced: `t:e` is a global
string element; `t:a` a global string attribute; `t:G` a named sequence group;
`t:A` an attribute group containing optional unqualified string x; `t:S` a
named restriction of xs:string; `t:B` an empty named complex type. Prefixes may
be renamed consistently without changing names. Empty targetNamespace and
unbound QName prefixes are separate semantic/lexical mutations, not substitutes
for a missing required name. Examples for standalone helpers describe their
value/property admission; they do not authorize user redeclaration of builtins
in the XSD namespace.

In the effective-context tables, `ann?` means one optional **leading** annotation;
`attrs` means `(attribute | attributeGroup)*, anyAttribute?` in that order;
`model?` means one optional group-ref/all/choice/sequence at the whole-type slot;
`nested*` means zero or more local element/group-ref/choice/sequence/any children.
All element names in those expressions are in the XSD namespace. **F** means
foreign namespaced attributes through `##other/lax`: neither an undeclared
unqualified attribute nor an XSD-namespaced attribute is covered. All ordinary
annotated contexts inherit optional `id:ID` and F unless explicitly excluded.
Defaults shown below are schema-syntax defaults, distinct from a user element's
runtime default/fixed constraint.

An extension retains its base's children/attributes, then appends its own
particle/uses. A restriction supplies its restricted child model, inherits
unmentioned base attribute uses, overrides named uses, and removes only
explicitly prohibited uses. This matters: topLevelSimpleType retains `final`,
numFacet retains `fixed`, and noFixedFacet retains required `value` while
prohibiting `fixed`. Dropping all unmentioned inherited attributes is incorrect.

## Effective helper/type graph

The following resolves every named complex grammar type. Anonymous local types
are expanded in the owning syntax context below; they are not omitted by
following only named declarations. The XSD namespace builtins referenced by
attribute types remain the predefined datatypes, not ordinary restrictions
compiled from the informative primitive sketches.

| Grammar type(s) | Base and derivation | Effective consequence |
| --- | --- | --- |
| openAttrs | restriction of anyType | No ordinary children/uses; F wildcard. |
| annotated | extension of openAttrs | ann?, id:ID; F retained. |
| attribute → topLevelAttribute | extension of annotated → restriction of attribute | Local/ref-capable form; top form requires name and prohibits ref/form/use. |
| complexType → topLevelComplexType / localComplexType | extension of annotated; each concrete type restricts the abstract base | Top requires name; local prohibits name/abstract/final/block, retains mixed and id. |
| restrictionType → complexRestrictionType / simpleRestrictionType | annotated extension; both concrete derivatives restrict restrictionType | Base QName required throughout; complex branch restricts to model?, simple branch to optional inline simpleType/facets; attrs retained. |
| extensionType → simpleExtensionType | annotated extension → restriction | Base QName required; simple variant removes model particle but retains attrs. |
| element → topLevelElement / localElement | annotated extension; concrete restrictions of abstract element | Top forbids ref/form/occurrences; local forbids substitutionGroup/final/abstract. |
| group → realGroup → namedGroup / groupRef | annotated extension, then restrictions | named requires one model and no occurrences/ref; ref requires ref, no name/model body, retains occurrences. |
| group → explicitGroup → simpleExplicitGroup | restrictions | explicit admits nested* and occurrences, no name/ref; simple removes occurrences on named-group model children. |
| localElement → narrowMaxMin | restriction | All-member min/max each restricted to 0 or 1. |
| explicitGroup → all | restriction | ann?, element* only; outer min 0/1, max exactly 1. |
| wildcard | extension of annotated | ann?, namespace/default ##any; processContents/default strict. |
| attributeGroup → namedAttributeGroup / attributeGroupRef | annotated extension; restrictions of abstract attributeGroup | Named requires name and attrs; ref requires ref, no name/body. |
| keybase | extension of annotated | ann?, selector, field+; required name. keyref extends it with required refer. |
| simpleType → topLevelSimpleType / localSimpleType | annotated extension; restrictions of abstract simpleType | Exactly one restriction/list/union; top name required, local name/final prohibited. |
| facet → noFixedFacet / numFacet | annotated extension; restrictions | Required value inherited; fixed default false except prohibited by noFixedFacet; numFacet narrows value to nonNegativeInteger. |
| anyType display approximation | mixed complex type with lax any/anyAttribute | Informative approximation of builtin ur-type, not a replacement for its normative abstract definition. |

## Concrete context groups

### GC-001 — source roots and cross-part dependencies

Both appendix roots target the XSD namespace and set elementFormDefault=qualified
and blockDefault=#all. Structures' include supplies Datatypes declarations;
Datatypes refers back to Structures' annotated/annotation helpers. Treat the
pair as one schema-for-schemas component vocabulary, not two independently
closed executables. Structures' XML import supplies xml:lang. A retained local
resolver can satisfy those explicit edges; syntax analysis does not require
network retrieval.

The `.xsd.txt` files include presentation wrappers and a displayed DOCTYPE before
an XML declaration. They are faithful **display fragments**, not ready-to-parse
XML documents. An extraction that selects exactly the xs:schema element is a
separate mechanical transformation. Moving the XML declaration after DOCTYPE in
an actual XML document is XML-invalid; it does not prove an XSD grammar defect.
The root nodes/these include/import records are source-assembly owners; the
user-schema include/import syntax is independently GC-015.

### GC-002 — openAttrs and annotated are real inherited restrictions

A local attribute witness `<xs:attribute name="a" u:flag="x"/>` is A+ when
u:flag has no available declaration. Change to unqualified `flag` or
`xs:flag`: A−. Supply a foreign declaration u:flag of type xs:boolean and use
`u:flag="wrong"`: A− under lax **when the declaration is available**; use
`true`: A+. Lax is not skip. Namespace declarations are not ordinary attributes.

`<xs:attribute name="a" id="n"><xs:annotation/></xs:attribute>` is A+.
Two leading annotation children, an annotation after inline simpleType, or
nonwhitespace text in attribute grammar are A−. Whitespace, comments and PIs
between legal children are allowed. An unqualified lookalike `<attribute>` or
foreign `<u:attribute>` is not an admitted XSD child outside a wildcard payload.

ID admission uses NCName normalization: id=`a` is admitted, id=`1a` is not;
`id=" a "` normalizes to `a`. Put `id="a"` on two ordinary schema nodes:
A− for duplicate ID even if their component names differ. Reuse the spelling in
two separate schema documents: document ID uniqueness alone does not reject it.

### GC-003 — exact control-token helper values

| Helper / use | Concrete A+ values | Concrete A− or separately constrained values |
| --- | --- | --- |
| formChoice: element/attribute form and schema form defaults | qualified, unqualified, surrounding XML whitespace | `Qualified`, empty, `qualified unqualified`. |
| reducedDerivationControl: item of derivationSet | extension, restriction | substitution, list, union. |
| derivationSet: complexType block/final, element final | empty list, extension, restriction, `extension restriction`, #all | `#all extension`, substitution, list. Repeated list members are lexically admitted; set interpretation deduplicates by semantic mapping. |
| typeDerivationControl / fullDerivationSet: schema finalDefault | extension, restriction, list, union and their list combinations; empty; #all | substitution; `#all union`. |
| blockSet: element block and schema blockDefault | extension, restriction, substitution; empty; combinations; #all | list, union, `#all substitution`. |
| derivationControl: shared item vocabulary | all five single tokens substitution/extension/restriction/list/union | `Restriction`, `#all` as one derivationControl item. #all belongs to the set union's separate token branch. |
| simpleDerivationSet: named simpleType final | restriction, list, union; combinations; empty; #all | extension, substitution, `#all list`. |

Use `<xs:complexType name="T" block="extension restriction"/>`, then mutate
only block to `substitution`: A−. On `<xs:element name="e" block="substitution"/>`
the same token is A+. On `<xs:simpleType name="S" final="list union"><xs:restriction
base="xs:string"/></xs:simpleType>` it is the simple set, not complex derivationSet.
The component final/block mappings filter schema defaults by applicable method;
a syntactically valid schema default can contain a token ignored for one kind.

### GC-004 — schema body and its seven identity keys

Effective root children are `(include|import|redefine|annotation)*` followed by
zero or more `(simpleType|complexType|group|attributeGroup|element|attribute|
notation), annotation*`. Prefix composition directives may be mixed with
annotations. Once a global definition appears, later include/import/redefine
is A−; an annotation may still follow any global definition.

Empty schema is A+. A schema with a simpleType, annotation, element, annotation,
attribute is A+ in that order; exchanging independent definitions is A+.
Insert import after the simpleType: A−. Root attributes targetNamespace:anyURI,
version:token, id:ID, xml:lang, form defaults, blockDefault/finalDefault are
optional; both form defaults default unqualified and both control defaults
empty. Unknown unqualified root attribute is A−; F applies. Empty
`targetNamespace=""` is lexically A+ anyURI but S− by the no-empty-namespace
rule; omission instead represents absent namespace.

Keys enforce name uniqueness separately for global element, attribute, group,
attributeGroup, notation, combined simple+complex type, and identity constraints
selected by `.//xs:key|.//xs:unique|.//xs:keyref`. Two global elements both name=e
are A−; an element e and attribute e are A+; simpleType T plus complexType T is
A−. Two identity definitions named K under different element declarations are
A−. Definition names are **not** document IDs: name=e plus id=e in different
roles does not itself collide. Missing required global names also fail their
own contextual type, so avoid using that masked error to prove a key failure.

### GC-005 — occurs, defRef and the particle helper alternatives

`occurs` supplies minOccurs:nonNegativeInteger and maxOccurs:allNNI, each
optional/default 1. allNNI is integer or exact token unbounded. On local element,
choice, sequence, group-ref and any: omitted, 0, 1, 2, large finite integers and
max=unbounded are A+ lexical branches; -1, 1.5, `Unbounded`, min=unbounded are
A−. min=2,max=1 is A+ lexically but S−; min=max=0 is a permitted source shape
eliminated from the semantic particle model. Narrow all contexts override this
helper as GC-013 describes.

`-0` and `-00` are admitted zero spellings for nonNegativeInteger under
[Part 2 §3.3.20.1](../upstream/datatypes.html#nonNegativeInteger-lexical-representation).
Thus minOccurs=-0 and numeric maxOccurs=-0 are A+ where the occurrence
attribute is allowed; apply the usual value-based range check afterward.
An omitted maximum of 1 with minimum -0 is valid. This differs from -1,
whose value is outside the datatype, and from a prohibited occurrence
attribute, where even a default-valued spelling is forbidden.

`defRef` merely supplies optional name:NCName and ref:QName; it does **not**
express their mutual exclusion or one-required condition. A local
`<xs:element name="p" ref="t:e"/>` and `<xs:attribute/>` can pass that helper's
attribute grammar but fail representation constraints. Contextual derivatives
prohibit/require particular uses below.

`typeDefParticle` selects group-ref, all, choice or sequence; it does not admit
a bare local element or any at a whole complex-type model slot. `nestedParticle`
selects local element, group-ref, choice, sequence or any, excluding all.
`particle` includes all too for the abstract group helper's wider base model;
concrete restrictions must not inherit that extra branch accidentally. Thus
`<complexType name="T"><sequence><element name="p"/></sequence></complexType>`
is A+, a direct `<element name="p"/>` there is A−, and nested all under sequence
is A− despite its presence in the wider helper.

### GC-006 — attribute global, local named, local reference

| Context/type | Effective attributes beyond id/F | Children |
| --- | --- | --- |
| Global attribute / topLevelAttribute | name:NCName required; type:QName, default:string, fixed:string optional; ref/form/use prohibited | ann?, localSimpleType? |
| Local declaration / attribute | name:NCName, ref:QName, type:QName, form:formChoice, default/fixed:string optional; use optional/default optional, values optional/required/prohibited | ann?, localSimpleType? |
| Local ref / same attribute grammar | Same grammar as local declaration; prose removes declaration-only ref combinations | ann?, inline simpleType syntactically optional but forbidden by ref prose |

Global `<xs:attribute name="a" type="xs:string"/>` is A+/S+; remove name or add
ref, form or use: A−, each mutation independently. Local `<xs:attribute name="a"
use="required"/>` is A+/S+; local `<xs:attribute ref="t:a"/>` is A+/S+ with
usable reference. Add type or inline simpleType to the ref: A+ by this grammar,
S− by `src-attribute`. Both default and fixed are likewise A+ by independent
attributes but S− together. `default="x" use="required"` is A+ but S−.
Inline simpleType is anonymous and cannot carry name/final (GC-019). Both type
and inline simpleType are A+ syntax but S− representation. Children cannot be
repeated or reordered to put annotation last.

### GC-007 — attrDecls and attributeGroup definition/reference

`attrDecls` expands to `(local attribute | attributeGroup-ref)*, anyAttribute?`.
Two attributes, a ref, another attribute, and final anyAttribute are A+; an
attribute after anyAttribute or two anyAttribute children is A−. Empty attrs
is admitted. This exact expansion is reused in complex shorthand, derivations
and named attribute groups; it is not a free order of all three child names.

Named `<xs:attributeGroup name="A"><xs:attribute name="x"/></xs:attributeGroup>`
is A+; omit name, add ref, or add occurrences: A−. Local
`<xs:attributeGroup ref="t:A"><xs:annotation/></xs:attributeGroup>` is A+;
name, missing ref, attribute child, anyAttribute child or a second annotation
are A−. Named definitions have ann?, attrs, name required and ref prohibited;
refs inherit id/F only and require QName ref. The abstract attributeGroup base
has optional name/ref and attrs but is not the effective local-ref contract.
References merge use sets and wildcards semantically. Repeated membership of
the same use counts once; two distinct use members with equal declaration
names/namespaces or two distinct ID-derived use members fail. Ordinary circular
group references remain S− independently of syntax; redefine retains S-065's
specific original-component rules.
For nonself references inside a restrictive redefine, preserve
[CMP-Q002](source-questions.md#cmp-q002--restrictive-attribute-group-redefine-and-nonself-references)
rather than applying the explanatory explicit-only note as a new grammar ban.

### GC-008 — named and anonymous complexType

| Context | Effective attributes beyond id/F | Effective children |
| --- | --- | --- |
| Global / topLevelComplexType | name required; mixed:boolean default false; abstract:boolean default false; block/final:derivationSet optional | ann?, exactly one complexTypeModel alternative |
| Anonymous / localComplexType under element | mixed:boolean default false; name/abstract/block/final prohibited | Same child alternatives |
| Abstract grammar base complexType | Optional name; same mixed/abstract/block/final uses | Used to derive the effective contexts; not an instruction to accept its optional-name shape globally |

complexTypeModel selects simpleContent, complexContent, **or** `model?, attrs`.
The shorthand branch permits no model and no attributes, so global
`<xs:complexType name="T"/>` is A+/S+. Anonymous
`<xs:element name="e"><xs:complexType/></xs:element>` is A+/S+; add name,
abstract, block or final to its inner complexType: A− each. Remove name from
the global declaration: A−. mixed=`1` is A+ boolean; mixed=`yes` A−.

A named shorthand type can contain an empty sequence followed by local
attributes and final anyAttribute. Moving those attributes before sequence,
combining simpleContent and complexContent, putting shorthand attributes beside
a simpleContent child, or two content wrappers is A−. A named type with
mixed=true and simpleContent is A+ and, under `src-ct`'s note, the outer mixed
has no effect; the appendix's explanatory comment saying it is not allowed is
not normative and does not establish rejection. A supplied complexContent
mixed overrides the outer setting semantically; that is not two syntax errors.

### GC-009 — complexContent extension/restriction

Both complexContent wrappers admit ann? then **exactly one** local extension or
restriction, plus optional mixed:boolean and inherited id/F. Both derivation
children have required base:QName and inherited id/F; their effective content
is `ann?, model?, attrs`. The complexRestrictionType restriction removes the
simpleRestrictionModel choice from the wider restrictionType base. The optional
choice wrapper in its grammar is structural, not permission to provide both
models or a second derivation child.

Within named T, `<xs:complexContent><xs:restriction base="xs:anyType"><xs:sequence/>
<xs:attribute name="x"/></xs:restriction></xs:complexContent>` is A+ and can be
S+. Extension from t:B with the same sequence/attribute body is A+ under its
own semantic base. Remove base, omit the derivation child, add another one,
insert inline simpleType or a facet in complexContent restriction, put a model
after an attribute, or add mixed to the derivation child: A− independently.
`base="xs:string"` remains A+ QName syntax but S− because complexContent
requires a complex base. Empty base is not a valid QName: A−, distinct from a
well-formed QName that does not resolve.

The named helper restrictionType admits an optional choice between typeDefParticle
and simpleRestrictionModel before attrs; the effective contextual restriction
selects the narrower one. extensionType permits model? before attrs;
simpleExtensionType below removes it. Treat these as resolved restrictions,
not a union of all ancestor alternatives.

### GC-010 — simpleContent extension/restriction

simpleContent has id/F, ann?, exactly one local derivation child, and **no mixed
attribute**. Each child requires base:QName and inherits id/F. Restriction
admits `ann?, localSimpleType?, facet*, attrs`; extension admits `ann?, attrs`
only. The inherited base requirement survives both restricting helper types.

`<xs:simpleContent><xs:extension base="xs:string"><xs:attribute name="x"/>
</xs:extension></xs:simpleContent>` is A+/S+. Add a sequence, inline simpleType,
or facet to that extension: A− each. A restriction of a known complex simple-
content base t:C with maxLength followed by x attribute is A+; move the facet
after x or add two inline simpleType children: A−. A restriction from simple
xs:string rather than a complex simple-content/mixed-emptiable base is A+ but
S−; that category condition is outside this lexical/base attribute grammar.
For a mixed emptiable base, absence of the inline simpleType can still be A+
but is S− under `src-ct`. Facet repeatability is syntax, not permission for every
repeated semantic facet. The effective facet alternative is GC-021–GC-024.

### GC-011 — global/local/reference element and all-member derivative

| Context/type | Attributes beyond id/F | Children |
| --- | --- | --- |
| Global / topLevelElement | name required; type/substitutionGroup:QName, default/fixed:string, nillable:boolean default false, abstract:boolean default false, final:derivationSet, block:blockSet; ref/form/minOccurs/maxOccurs prohibited | ann?, optional local simpleType **or** local complexType, then identityConstraint* |
| Local / localElement | name:NCName; ref/type:QName; default/fixed:string, nillable default false, block:blockSet, form:formChoice; min/max default 1; substitutionGroup/final/abstract prohibited | Same effective body |
| Local ref / same localElement grammar | Same syntax; `src-element` removes name and declaration-only attributes/children | ann? only after prose restrictions |
| Child of all / narrowMaxMin | LocalElement's retained uses except each occurrence is 0 or 1, default 1 | Same declaration/ref child body; other semantic restrictions still apply |

Global `<xs:element name="e" type="xs:string"/>` is A+/S+. Add ref, form or
minOccurs: A− each. Local `<xs:element name="p" minOccurs="0" maxOccurs="2"/>`
in a sequence is A+; add abstract/final/substitutionGroup: A−. A local
`<xs:element ref="t:e" minOccurs="0"/>` is A+/S+ with usable ref; add type,
name, nillable, block, form, default/fixed, inline type or identity definition:
these can pass the **local grammar** but are S− by reference representation
constraints. Annotation is still admitted on a ref.

An inline simpleType and complexType together are A−; a type attribute plus one
inline type is A+ but S−. Identity definitions must follow the inline type, not
precede it; multiple key/unique/keyref definitions are admitted subject to their
own name/semantic rules. Omitted type and inline type both absent is A+/S+:
type comes from substitution affiliation or anyType, not a syntax error.
The outer element's default/fixed attributes do not become defaults on the
schema grammar's own boolean/occurrence uses.

### GC-012 — group definition, group reference, choice/sequence

| Effective context | Attributes beyond id/F | Body |
| --- | --- | --- |
| Named group / namedGroup | name required; ref/min/max prohibited | ann?, exactly one all/choice/sequence using its special definition-child type |
| Ref group / groupRef | ref required; name prohibited; min/max default 1 | ann? only |
| Choice/sequence in ordinary model / explicitGroup | min/max default 1; name/ref prohibited | ann?, nested* |
| Choice/sequence as named-group's sole model / simpleExplicitGroup | name/ref/min/max prohibited | ann?, nested* |
| Wider group/realGroup bases | defRef/occurs; wider particle* or optional one model | Intermediate derivation contracts, narrowed by the four contexts above |

`<xs:group name="G"><xs:sequence><xs:element name="p"/></xs:sequence></xs:group>`
is A+. Remove the required model, add a second model, put an element directly
under named group, or add occurrences to G: A− each. Add minOccurs=`1` even to
its sole sequence child: A−; the value being the usual default does not make a
prohibited attribute legal. The same sequence with minOccurs=`1` as an ordinary
complex-type model is A+.

`<xs:group ref="t:G" minOccurs="2" maxOccurs="3"><xs:annotation/></xs:group>`
at the type model slot is A+. Add name or sequence body, or remove ref: A−.
Ordinary choice/sequence can be empty, repeat children, and nest other choice/
sequence/group-ref/any/local elements. Add name/ref or nested all: A−. A model
can be A+ yet violate UPA or element consistency; concrete grammar order is not
an implementation content-matching algorithm. Named-group reference occurrence
controls uses of the group; occurrences cannot be stored on its definition child.

### GC-013 — all and its member restrictions

Ordinary all's effective body is ann?, localElement* through allModel, but each
member uses narrowMaxMin: min and max are independently 0 or 1, default 1.
The all itself has min 0/1 default 1 and max exactly 1/default 1. Its name/ref
are prohibited. A named group's local all child further prohibits **both**
occurrence attributes, inheriting its narrowed member body.

`<xs:all minOccurs="0"><xs:element name="p" minOccurs="0" maxOccurs="1"/>
<xs:element name="q"/></xs:all>` is A+. Set outer min=2, outer max=0/2/unbounded,
member min=2 or member max=2/unbounded: A− each. Member min=max=0 is A+ and
semantically eliminated; member min=1,max=0 is A+ by the two independent
restricted attribute datatypes but S− by range ordering. Add sequence/choice/
any/group-ref as an all child: A−. The same all as a named group's sole model
must omit outer min/max; adding min=0 is A− there.

All placement/whole-model restrictions still apply after syntax. The source's
`allModel` choice wrapper with an unbounded occurrence is a grammar-construction
wrapper; it does not let a user all contain arbitrary particle kinds. Instance
permutations of all children do not justify reordering restriction component
lists contrary to the derivation rule.

### GC-014 — any, anyAttribute, namespaceList

Both inherit ann?/id/F and optional namespace:namespaceList default ##any,
processContents enum skip/lax/strict default strict. any extends wildcard with
occurs; anyAttribute has no occurs. An any in a sequence with
namespace=`urn:u ##local`, processContents=skip, min=0,max=unbounded is A+.
The same attributes on anyAttribute are A− only for the occurrence attributes;
namespace/processContents are retained. Additional child, duplicate annotation,
unknown process token or case change `Skip` is A−.

namespaceList is a union of exact token ##any/##other with a **list** whose items
are anyURI or exact ##targetNamespace/##local. Empty list is A+ and maps an
empty namespace set, not an absent wildcard. `urn:u urn:v ##local` is A+;
##other alone is A+. The exclusive ##any/##other token branch cannot be mixed
with list tokens: `##other urn:u` and `##any urn:u` are A−. Neither reserved
spelling can escape through the list's anyURI alternative: XLink escaping
preserves `#`, and RFC 2396 permits one fragment delimiter, not a second raw
`#` inside its fragment. Unknown `##bogus` is likewise A−, rather than a new
reserved keyword. This conclusion uses the actual anyURI item admission, not
an implementation's permissive string parser.

Prefix namespaces do not expand URI spellings in namespace lists: `u:space`
can be a URI-reference spelling, not a QName referring to xmlns:u. ##targetNamespace
and ##local receive their specified semantic substitutions; target namespace
absent is distinct from present empty. Wildcard union/intersection/processing
strength and ##other's exclusion of absent namespace are component constraints,
not additional element-child grammar alternatives.

### GC-015 — include, import and redefine

include/import inherit ann?, id/F. include requires schemaLocation:anyURI;
import has optional namespace:anyURI and schemaLocation:anyURI. Empty import
and include with schemaLocation=`a.xsd` are A+. Remove include's schemaLocation:
A−; its empty string is A+ URI syntax, with resolution/base semantics separate.
Give either a type/group/element child rather than annotation: A−.

redefine extends openAttrs directly rather than annotated: it declares its own
id and required schemaLocation, and permits `(annotation|simpleType|complexType|
group|attributeGroup)*` in arbitrary repetition/order. It does **not** admit
ordinary global element/attribute/notation definitions. Two annotation envelopes
separated by two redefinitions are A+; two leading annotations on include are
A−. Redefine annotation-only/empty body remains A+ with schemaLocation present.
Substantive redefinitions need a successfully resolved suitable source and
self-base/restriction rules; A+ is not proof of those semantic conditions.
Composition directives occur only in the schema prefix from GC-004.

### GC-016 — identity definitions, selector and field

keybase resolves annotated plus `selector, field+` and required name:NCName.
unique/key use keybase; keyref adds required refer:QName. Thus unique/key admit
id/F/name but no refer; keyref admits it and requires it. Each selector/field
has id/F, ann? and required xpath with its own token-derived pattern type.

`<xs:key name="K"><xs:selector xpath="t:row"/><xs:field xpath="@id"/></xs:key>`
is A+; a second field is A+; zero fields, two selectors, fields before selector,
missing xpath/name, extra child content, or refer on key is A− each.
The keyref version adds refer=`t:K`; omit it: A−; `refer="missing:K"` with
unbound prefix is A− QName lexical interpretation, while bound `t:Missing`
is A+ but may be S−/incomplete for missing referenced component.

Concrete selector pattern strings `t:row`, `.//t:row`, `t:a|t:b`, `*`, `t:*`,
and `.` reach its named-step/wildcard/self/union paths. Field `@id`, `t:p/@id`,
`.//t:p`, and union `@a|@b` reach final attribute/child/descendant/union paths.
Malformed path `a[` and empty string fail the intended selector pattern and
restricted XPath grammar; XSD 1.0 regex matches whole literals, not an arbitrary
substring. Schema `xs:pattern` facets are
not a substitute for the prose XPath grammar and namespace semantics.
The appendix patterns also explicitly include `child::`/`attribute::` forms;
the prose EBNF and whitespace-token requirements need an independent check.
The source discrepancy is recorded below rather than silently dropping one
source or promising a unique A/S outcome for every full-axis spelling.

<a id="gc-017-notation-declarations-and-identifier-domains"></a>
### GC-017 — notation declarations and identifier domains

The **dated Appendix A** notation type extends annotated and has required
name:NCName, **optional public:token and optional system:anyURI**. Its `public`
helper is only a token restriction; the helper does not encode every public-
identifier character requirement. Children are ann?; id/F are inherited.

`<xs:notation name="N" system="n.bin"/>` is A+ and satisfies the component's
at-least-one-identifier condition. Public-only and public+system are also A+.
`<xs:notation name="N"/>` is A+ syntax but S− because neither identifier is
present. Missing name, arbitrary child content, or an occurrence attribute is
A−. Public whitespace is token-normalized; an otherwise token-admitted public
string still needs the abstract public-identifier domain.

The [published 2004 representation summary](../upstream/structures.html#element-notation)
and Appendix A agree: only `name` is required; `public` and `system` are each
optional. The historical 2001–2004 change markup removes the older required
`public` label; it does not describe a conflict in the published 2004 text.
The component tableau independently requires at least one identifier property.
Keep an absent attribute distinct from a present empty value, whose identifier
domain still applies.

### GC-018 — annotation/appinfo/documentation and lax payload

annotation extends openAttrs, declares id:ID and permits any number/order of
appinfo/documentation children. Empty annotation and documentation/appinfo/
documentation interleaving are A+. Direct text other than XML whitespace or an
unwrapped arbitrary child under annotation is A−. More than one annotation
can occur where schema/redefine's own grammar permits it; that does not change
ordinary annotated's ann? bound.

appinfo/documentation are independently declared mixed complex types, **not**
extensions of annotated. Each permits mixed text and repeated any/lax children,
optional source:anyURI and F; documentation additionally uses xml:lang.
An ordinary unqualified `id` is not inherited there. Text plus unknown foreign
`<u:payload/>` is A+; source=`notes.xml` A+; id=`n` is A−; a DTD declaration identifying it as an ID does not add a
missing grammar attribute use. `xml:lang="en-GB"` on documentation uses its declared XML attribute;
on appinfo the XML namespace falls under F and available declarations still
matter. Invalid available u:flag:boolean or an available declared payload that
fails its type is A− under lax. Well-formed unknown payload is different from
known-invalid payload.

Markup in payload is not translated into an active schema declaration merely
because it uses an XSD-looking local name. However the **schema-for-schemas
assessment** wildcard is lax, not skip: a known XSD declaration can validate
such a payload, and the schema root's descendant identity selector does not
syntactically exclude annotation descendants. Keep that boundary visible when
using schema-document key/ID expectations; do not claim complete grammar-level
opacity from the compiler's separate annotation handling. `source` is not
fetched as part of instance validation.

### GC-019 — named and anonymous simpleType

| Context/type | Effective attributes beyond id/F | Effective children |
| --- | --- | --- |
| Global / topLevelSimpleType | name:NCName required; final:simpleDerivationSet optional and inherited | ann?, exactly one restriction/list/union |
| Anonymous / localSimpleType | name and final prohibited | Same single derivation choice |
| Abstract simpleType base | name/final optional in the base; narrowed by concrete contexts | annotated's ann? followed by simpleDerivation group |

`<xs:simpleType name="S" final="restriction"><xs:restriction base="xs:string"/>
</xs:simpleType>` is A+/S+. Remove name globally: A−. As the inline type of an
element/attribute or derivation, omit name/final: A+; adding either is A− even
if the supplied name is unique. Zero derivation children, restriction followed
by list, or annotation after the derivation are A−. Restriction/list/union are
three distinct admitted alternatives at each anonymous nesting position,
including inline types in list, union and restriction; they do not all collapse
to a string restriction in the grammar.

### GC-020 — simple-type restriction, list and union

| Construct | Effective attributes beyond id/F | Body |
| --- | --- | --- |
| Simple-type restriction | base:QName optional | ann?, inline localSimpleType?, facet* |
| List | itemType:QName optional | ann?, inline localSimpleType? |
| Union | memberTypes:QName list optional | ann?, inline localSimpleType* |

`<xs:restriction base="xs:string"><xs:minLength value="1"/></xs:restriction>`
is A+/S+. Replace base by one anonymous string simpleType child: A+/S+.
Keep **both** sources, or neither: A+ grammar but S− by the exclusive-base
representation condition. A second inline type, inline type after a facet,
an attribute declaration child or base=`bad::name` is A−. The same local name
restriction under complexContent/simpleContent uses GC-009/GC-010 instead;
its base and child grammar must not be borrowed from this global declaration.

`<xs:list itemType="xs:integer"/>` and list with one anonymous integer type are
A+/S+; both/neither sources are A+ but S−. List with two inline types or facet
children is A−. A list whose selected item type is another list can be A+
syntax while S− type construction.

`<xs:union memberTypes="xs:integer xs:string"/>` is A+/S+. One/two inline
simpleTypes, or references followed by inline types, are A+; memberTypes=` `
is an empty QName list A+, but without an inline member gives S− empty union.
A QName item with unbound prefix is A−; a bound missing type is syntactically
A+ with resolution handled separately. Direct facet children on union are A−;
restriction of an already declared union supplies facets through restriction.
Union lexical member ordering and nested-union flattening are semantic facts,
not permission to reorder the inline XML children around annotation.

### GC-021 — shared facet vocabulary and inherited facet contract

The facets group selects exactly the 12 declared names minExclusive,
minInclusive, maxExclusive, maxInclusive, totalDigits, fractionDigits, length,
minLength, maxLength, enumeration, whiteSpace and pattern. The surrounding
simpleRestrictionModel makes this choice repeatable after at most one inline
type. Different facet orderings are A+; moving a facet before an inline type is
A−. An invented precisionDecimal/assertion/explicitTimezone facet is A− for
this XSD 1.0 grammar.

The generic facet type extends annotated with **required value:anySimpleType**
and optional fixed:boolean/default false. Thus each of min/max Inclusive/
Exclusive admits ann?/id/F/value/fixed. A decimal restriction containing
`<xs:minInclusive value="0" fixed="true"/>` is A+/S+; omit value or add
non-annotation child: A−. `fixed="1"` is A+ boolean, `yes` A−. `value="not-a-number"`
can be A+ anySimpleType but S− when evaluated as a bound for xs:decimal.
The generic value attribute's absence of an explicit type is **not** a hidden
requirement that every bound use a decimal lexical form at syntax admission.

Repeat minLength or mix minExclusive with minInclusive: the unbounded facet
choice may remain A+, while applicable duplicate/mutual-exclusion/fixed/facet-
restriction rules decide S. Facets allowed by a generic syntax type can still
be inapplicable to the restricted datatype, such as totalDigits on xs:string.
One generic valid-value example cannot prove that separate semantic domain.

### GC-022 — numeric facet types and totalDigits narrowing

numFacet restricts facet's value to **required nonNegativeInteger**, retaining
fixed:boolean/default false, id/F and ann?. fractionDigits, length, minLength,
maxLength use numFacet; totalDigits has an anonymous restriction narrowing value
to **positiveInteger**, again retaining fixed and id.

For each of fractionDigits/length/minLength/maxLength, value=`0`, `1`, `+2`,
`002` is A+; absent, `-1`, `1.5`, `unbounded` is A−. totalDigits value=`1` or
`+2` is A+; `0` is A−. The opposing pair length=0 A+ versus totalDigits=0 A−
proves the anonymous narrowing was resolved. `<xs:length value="2" fixed="true"/>`
is A+ because restriction did not remove inherited fixed; an implementation
that admits only the explicitly repeated value attribute misses this use.
Legal syntax still needs an appropriate base and facet consistency: fractionDigits=3
with totalDigits=2 is not made valid merely by both numeric attributes passing.

### GC-023 — enumeration and pattern prohibit inherited fixed

noFixedFacet restricts facet, explicitly prohibits fixed, and retains required
value and ann?/id/F. enumeration uses it; pattern adds an anonymous restriction
making value specifically xs:string while retaining the prohibition.

String restriction `<xs:enumeration value="x"/>` and `<xs:pattern value="[a-z]+"/>`
are A+/S+; add fixed=`false` or `true` to either: A− even though false is the
generic facet default. Remove value: A−. An empty enumeration value is A+ and
may be S+ for xs:string; an empty pattern is A+ lexical syntax and has its
regex-defined semantics. Pattern value `[` is A+ xs:string attribute admission
but S− as malformed XSD regex. Two enumeration/pattern children are A+ syntax;
their grouping across restriction steps is a Part 2 semantic rule, not ordinary
XML conjunction/order inference. The noFixedFacet helper must not acquire
fixed again by treating the restriction as extension.

### GC-024 — whiteSpace's anonymous enumeration

whiteSpace uses a facet restriction retaining required value, optional
fixed:boolean/default false, id/F and ann?. Its anonymous value type is an
NMTOKEN enumeration of preserve/replace/collapse. All three tokens, including
surrounding whitespace normalized by NMTOKEN, are A+; empty, `Preserve`,
`collapse replace`, or missing value is A−.

`<xs:whiteSpace value="collapse" fixed="1"/>` is A+. A new string restriction
can use replace or collapse subject to base facets; trying preserve when a
base already fixes collapse is A+ grammar but S− facet derivation. The inherited
fixed use is present here, unlike GC-023. This anonymous enum is not the unrelated
schema `use` or processContents enum merely because all have token-valued uses.

### GC-025 — all 19 primitive display sketches

Datatypes simpleType[1..19] each has a restriction of anySimpleType and a
whiteSpace facet; these builtin definitions are explicitly **informative
sketches**, not an executable definition of the real primitives. Only string
uses preserve without fixed=true; all other listed primitive sketches use
collapse fixed=true. Do not allow ordinary user atomic types to derive
straight from anySimpleType merely because the primitive sketch does.

The following concrete content witnesses identify each primitive reference
reachable from user `type` attributes. Their detailed lexical/value semantics
come from the corresponding normative Part 2 clause; the appendix sketches
alone deliberately do not encode them all.

| Primitive | Concrete admitted value | Distinguishing invalid/mutated value or condition |
| --- | --- | --- |
| string | ` a ` including retained spaces | XML-forbidden character is an XML precondition failure, not an xs:string facet failure. |
| boolean | true, 1 | True. |
| float | INF, NaN, 1E2 | infinity. |
| double | -INF, 1E2 | 1E. |
| decimal | +1.20 | 1E2. |
| duration | P1D, -PT1S | P, P1Y1D1M with out-of-order components. |
| dateTime | 2000-02-29T00:00:00Z | 2001-02-29T00:00:00Z. |
| time | 12:00:00+01:00 | 12:60:00. |
| date | 2000-02-29 | 2001-02-29. |
| gYearMonth | 2000-02 | 2000-13. |
| gYear | 2000 | 0000 in XSD 1.0. |
| gMonthDay | --02-29 | --02-30. |
| gDay | ---31 | ---32. |
| gMonth | --05 | --13; treatment of obsolete trailing-dash spelling follows the normative datatype text, not this unexpressive sketch. |
| hexBinary | 0A, empty | A (odd digit count), 0G. |
| base64Binary | TQ==, empty | TQ= (bad padding length). |
| anyURI | docs/a.xsd | Bad percent escape `%GG` under the normative URI lexical rule; escaping/provenance is a separate dependency condition. |
| QName | t:e with t bound | missing:e with prefix unbound, or t::e. |
| NOTATION | t:N under a derived enumeration and declared notation N | Raw direct NOTATION use is forbidden; the appendix's magic primitive sketch does not remove declaration/enumeration requirements. |

Every pair uses the actual predefined type. Validating just a copied primitive
sketch would wrongly admit many listed negatives and is not a conformance oracle.

### GC-026 — string/name/list-derived definitions

Datatypes simpleType[20..31] retain concrete derivation facts: normalizedString
replaces XML tab/LF/CR with spaces; token collapses whitespace; language restricts
token by its language pattern; NMTOKEN by name characters; Name by initial/name
characters; NCName subtracts colon; ID/IDREF/ENTITY derive from NCName.
IDREFS/ENTITIES/NMTOKENS each restrict an anonymous list and add minLength=1.
All helper inline simpleType/list/restriction/minLength nodes belong to these
builtin definitions, rather than extra user syntax contexts. Unlike the
primitive approximations, §3.3 explicitly refers to Appendix A for complete
derived-builtin definitions; their recorded restrictions are substantive.

Concrete witnesses: value `a&#x9;b` normalizes to `a b` as normalizedString;
` a  b ` becomes `a b` as token. language `en-GB` passes and `en_GB` fails.
NMTOKEN `1:a` passes where Name `1a` fails; Name `p:a` passes where NCName `p:a`
fails. ID and IDREF lexical `a` pass; `1a` fails. ID uniqueness/unresolved IDREF
and ENTITY unparsed-declaration checks are additional Structures rules.
`a b` is a two-item list for the three plural types; empty/whitespace-only
fails minLength=1, while an ordinary user list without that facet may be empty.
`a  b` and ` a b ` collapse to the same list; this does not make a failed entity
or IDREF referent valid. These examples check the inherited definition facts against the normative
types, without turning annotation metadata into rules.

### GC-027 — integer-family definitions and exact endpoints

integer restricts decimal with fractionDigits fixed 0 and the integer lexical
pattern. Concrete `+002` passes; `2.0` fails its lexical space despite its decimal
value being integral. The remaining named restrictions inherit that lexical
condition and whitespace handling while adding the recorded inclusive bounds.

| Type | Valid interval/endpoints | Concrete opposing boundary |
| --- | --- | --- |
| nonPositiveInteger | 0 and -1 | 1 fails. |
| negativeInteger | -1 and -2 | 0 fails. |
| long | -9223372036854775808 through 9223372036854775807 | One less or one greater fails. |
| int | -2147483648 through 2147483647 | -2147483649, 2147483648 fail. |
| short | -32768 through 32767 | -32769, 32768 fail. |
| byte | -128 through 127 | -129, 128 fail. |
| nonNegativeInteger | 0 and arbitrary nonnegative integer | -1 fails; no invented 64-bit maximum. |
| unsignedLong | 0 through 18446744073709551615 | -1 and 18446744073709551616 fail. |
| unsignedInt | 0 through 4294967295 | -1 and 4294967296 fail. |
| unsignedShort | 0 through 65535 | -1 and 65536 fail. |
| unsignedByte | 0 through 255 | -1 and 256 fail. |
| positiveInteger | 1 and arbitrary larger integer | 0 fails; no invented 64-bit maximum. |

These endpoint/facet nodes supply the complete derived definitions referenced
by Part 2 §3.3. They are not discarded as informative merely because the earlier
primitive sketches are described that way. Read their inclusive bounds and
inherited restrictions together with the corresponding datatype clauses. A user `<xs:maxInclusive value="256"/>`
under xs:unsignedByte is A+ facet syntax but S− widening, not a new bound.

### GC-028 — retained notation declarations and anyType approximation

The two top-level notation records XMLSchemaStructures and XML are concrete
notation definitions within the displayed schema. Their public/system strings
are retained provenance and examples, not user vocabulary that every schema
must redeclare. Their shape instantiates GC-017. They do not authorize the
library to retrieve those system URIs during validation.

The final named complexType anyType is a mixed type with any 0..unbounded/lax
and anyAttribute/lax. The source labels it an approximation of the real ur-type.
A known xs:integer child with invalid text can fail lax assessment; an unknown
well-formed child is tolerated. Copying this approximation into a user schema
does not reproduce builtin anyType identity/self-base or its derivation
exceptions. The four nodes are accounted for without pretending they introduce
four more public schema-language productions.

### GC-029 — context-changing and default/identity interaction witnesses

The following concrete paired observations link effective context contracts:

- Move `<xs:element name="e" minOccurs="1"/>` from a sequence to schema root:
  local A+, global A−. Move `<xs:attribute name="a" use="optional"/>` from a
  type body to root: local A+, global A−. An explicitly default-valued prohibited
  attribute is still prohibited.
- Move `<xs:complexType name="T"/>` under an element: global A+, anonymous
  A− due name. Remove name and keep mixed=true: anonymous A+. For simpleType,
  remove both name and final when moving it inline; inherited id remains allowed.
- Sequence min=0 as a whole type model is A+; the same sequence as a named
  group's model is A− because simpleExplicitGroup prohibits occurrences.
  All member max=0 is A+ with min=0; outer all max=0 is A−.
- Omit generic bound fixed or set false: both A+, same false default. Set false
  explicitly on enumeration or pattern: A− because noFixedFacet removes the use.
  Likewise numFacet retains fixed while localSimpleType removes final.
- Under schema blockDefault=#all, an explicit complexType block="" overrides
  the default semantically; the grammar distinguishes absent from empty list
  without rejecting either. There is no default injected into a prohibited use.
- Reuse id=`n` across two schema documents versus twice within one: the first
  has no cross-document ID failure; the second does. Duplicate component names
  in different global symbol spaces remain permitted; duplicate simple/complex
  type names share a key and fail. Raw spelling before token/ID normalization
  does not create a second identity.

These witnesses are necessary to catch lost restrictions, inherited-use drops,
or globally keyed local-name admission. They do not depend on any processor's
internal representation or diagnostic code.

### GC-030 — attribute lexical classes at their actual source positions

| Lexical class and concrete schema position | Distinguishing values and expected syntax result |
| --- | --- |
| NCName: declaration name, id's ID base | `a`, `_a` A+; `p:a`, `1a`, empty A−. ID adds document uniqueness; component names do not become IDs. |
| QName: type, base, ref, itemType, refer, substitutionGroup | `xs:string`/`t:e` with binding A+ lexical; `t::e`, unbound `z:e` A−. Bound-but-wrong-kind/missing references remain distinct semantic cases. |
| QName list: union memberTypes | `xs:string xs:integer`, whitespace-only empty list A+; one malformed/unbound item A−. Empty construction remains S− without another member source. |
| Boolean: mixed, abstract, nillable, facet fixed | true/false/1/0 and collapsed surrounding whitespace A+; True/yes/empty A−. Context can prohibit the attribute before its value matters. |
| Integer/union: min/max and numeric facet value | +2/002 A+; 2.0 A−. max unbounded is its own branch; totalDigits=0 and all.max=0 are contextually A− as above. |
| String: declaration default/fixed, pattern value, generic facet value | Empty and XML-legal text A+ lexical; value/type/facet meaning can still make S−. XML attribute normalization runs first; character references preserve distinctions lost for literal whitespace. |
| Token: version, public | ` a  b ` normalizes to `a b`, A+; no extra version-number grammar. Public-identifier semantics remain GC-017. |
| anyURI: targetNamespace, source, schemaLocation, import namespace | Empty and relative references are admitted lexical possibilities; a missing required attribute is different. Empty targetNamespace is S−; unavailable source is not necessarily S−. Normative URI admission and fragment handling remain dependency-owned. |
| xml:lang on schema/documentation | The dated language type admits `en`/`en-GB` and rejects `en_GB`. Empty-language behavior requires the actual imported declaration: the retained 2009 xml.xsd union admits empty when that body is selected. Preserve [ANQ001](source-questions.md#anq001--annotation-xmllang-and-the-imported-source-version); the later distribution establishes neither the 2004 import body nor a guessed language-code registry. |

Each row instantiates the context-specific uses listed above; it is not a claim
that moving the same attribute to every element preserves admission. The helper
controls and occurrence subclasses have their complete finite token sets or
boundary classes in GC-003/GC-005/GC-013/GC-022/GC-024.

## Exclusive inventory ownership

A selector `S:complexType[3,4]` means the two exact top-level roots
`structures/schema/complexType[3]` and `[4]` **and every indexed descendant**.
`D` means datatypes. `1..5` is an inclusive range. `S:schema`/`D:schema` match
only their exact root records, not descendants. These top-level subtree
selectors are disjoint; a helper's references elsewhere are owned by the
referencing context while its definition is owned here. The counts refer to
inventory nodes, not a claim that a complexContent wrapper is an additional
language feature.

| Exclusive owner | Exact root selectors, with descendant closure except schema | Nodes | Disposition |
| --- | --- | ---: | --- |
| GC-001 | `S:schema`; `D:schema`; `S:include[1]`; `S:import[1]` | 4 | Source roots/dependencies; user composition syntax separately GC-015. |
| GC-002 | `S:complexType[1,2]` | 10 | Shared helper inheritance, child/id/wildcard contracts. |
| GC-003 | `S:simpleType[1..5,7]`; `D:simpleType[45,46]` | 59 | Control value unions/lists/enumerations, including anonymous item types. |
| GC-004 | `S:element[1]`; `S:group[1,2]` | 54 | Effective schema body and seven document-level key/selector/field groups. |
| GC-005 | `S:simpleType[6]`; `S:attributeGroup[1,2]`; `S:group[3..5]` | 32 | Occurrence/name/ref helpers and wider/narrower particle alternatives. |
| GC-006 | `S:complexType[3,4]`; `S:element[12]` | 28 | Effective attribute declaration contexts. |
| GC-007 | `S:group[6]`; `S:complexType[25..27]`; `S:element[13]` | 29 | Attribute-use sequence and named/ref group contexts. |
| GC-008 | `S:group[7]`; `S:complexType[5..7]`; `S:element[5]` | 36 | ComplexType model and named/anonymous header restrictions. |
| GC-009 | `S:complexType[8..10]`; `S:element[3]` | 33 | Wider restriction/extension helpers and effective complexContent types. |
| GC-010 | `S:complexType[11,12]`; `S:element[4]` | 23 | Effective simpleContent restrictions of wider helpers. |
| GC-011 | `S:complexType[13..15]`; `S:element[6]` | 48 | Global/local/ref-capable element and inline-type/identity children. |
| GC-012 | `S:complexType[16..21]`; `S:element[8..10]` | 66 | Named group/ref and ordinary/definition-child choice/sequence. |
| GC-013 | `S:group[8]`; `S:complexType[22,23]`; `S:element[7]` | 40 | All model and anonymous occurrence restrictions. |
| GC-014 | `S:complexType[24]`; `S:element[2,11]`; `S:simpleType[8]` | 30 | Wildcard syntax, occurrence extension and namespaceList value grammar. |
| GC-015 | `S:element[14..16]` | 20 | Include/import/redefine contexts and anonymous types. |
| GC-016 | `S:element[17..21]`; `S:complexType[28]`; `S:group[9]` | 35 | Selector/field patterns, identity body and keyref extension. |
| GC-017 | `S:element[22]`; `S:simpleType[9]` | 9 | Notation and token-valued public helper. |
| GC-018 | `S:element[23..25]` | 21 | Annotation and independently mixed/lax appinfo/documentation types. |
| GC-019 | `D:complexType[1..3]`; `D:element[1]`; `D:group[1]` | 29 | Named/anonymous simpleType and derivation choice. |
| GC-020 | `D:group[3]`; `D:element[2..4]` | 26 | Restriction/list/union anonymous contexts. |
| GC-021 | `D:complexType[4]`; `D:group[2]`; `D:element[5..8]` | 23 | Generic bound facet contract and complete facet-name choice. |
| GC-022 | `D:complexType[6]`; `D:element[9..13]` | 19 | Numeric value restriction and totalDigits anonymous narrowing. |
| GC-023 | `D:complexType[5]`; `D:element[14,16]` | 16 | Prohibited fixed; enumeration and pattern type narrowing. |
| GC-024 | `D:element[15]` | 13 | whiteSpace's restricted value enumeration and inherited attributes. |
| GC-025 | `D:simpleType[1..19]` | 57 | Informative primitive serialization sketches; true primitives are predefined. |
| GC-026 | `D:simpleType[20..31]` | 39 | Derived-builtin string/name/list definitions and inherited facets. |
| GC-027 | `D:simpleType[32..44]` | 44 | Derived integer-family definitions and exact bounds. |
| GC-028 | `S:notation[1,2]`; `S:complexType[29]` | 6 | Retained source notation declarations and explicit ur-type approximation. |
| **Total** | **All exact roots and their indexed descendants** | **849** | **563 S + 286 D; zero unassigned, zero multiply owned.** |

GC-029/GC-030 are concrete **cross-context** witnesses and have no exclusive
node subtree: they reuse the owners above deliberately. Their presence does
not inflate 849 to a larger invented denominator.

Within each owner, complexContent/extension/restriction/sequence/choice/list/
union/anonymous type nodes are resolved **structural contributors** to the
stated effective contract. They are not dismissed because they lack a public
name. Attribute prohibition/default/type restrictions, particle choices and
anonymous value enumerations remain binding; a wrapper carrying no additional
fact is a structural disposition, not a fabricated independent negative case.
Annotation subtrees were excluded by the inventory generator; GC-018 separately
covers the user syntax for annotations and does not claim those absent source
annotation nodes were among the 849.

## Source differences and remaining limits

- **Notation identifier domains:** GC-017 separates the individually optional
  public/system attributes from the component's at-least-one-property rule.
  The dated representation summary and Appendix A agree; token admission does
  not replace the public-identifier domain.
- **Resolved non-normative comments:** the complexType/mixed/simpleContent
  comment cannot create a prohibition missing from the grammar and explicitly
  denied by the prose note. The ordinary appinfo/documentation DTD id declarations
  do not create an id attribute use in their effective Appendix A types.
- **Selector/field pattern versus prose:** full-axis spellings and whitespace
  token boundaries need the prose restricted-XPath check as well as the
  appendix pattern. The patterns' explanatory prose is not authority to expand
  the normative EBNF; interpretations of an expression's abbreviated form must
  be recorded explicitly when selecting an oracle. Plain `a[`/empty selector
  negatives do not isolate that tension.
- **Resolved reserved wildcard mixtures:** GC-014 checks the anyURI alternate
  path using [XLink escaping](../dependencies/xlink-1.0.html#link-locators) and
  [RFC 2396](../dependencies/rfc2396.txt): raw double-# spellings do not become
  valid list items. Empty list remains separately admitted. URI-scheme-specific
  application rules are not invented as extra syntax rejections.
- **Builtin serialization roles:** primitive magic sketches and the ur-type
  approximation do not replace normative builtin semantics. Derived definitions
  referenced by §3.3 do contribute their recorded restrictions. The inventory
  cannot assign one blanket informative status to every builtin-related node.
- **External XML namespace schema:** the retained xml.xsd offers a language-or-
  empty union for xml:lang. Its provenance/version is distinct from the dated
  2004 fragments; an empty-language case must identify which external declaration
  was supplied instead of claiming the appendix alone fixes that external byte
  sequence.

Coverage here means a complete exclusive accounting plus concrete resolved
context descriptions. It does **not** claim 849 independently executed fixtures,
a universally determined outcome for every unresolved external/prose tension,
full Cartesian generation of all datatype values, corpus coverage, or
implementation conformance. Further fixture work must preserve A versus S
outcomes, inherited restrictions, external declarations, semantic prerequisites
and the exact observable seam specified by these descriptions.
