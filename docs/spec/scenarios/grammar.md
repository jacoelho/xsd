# Schema representation scenario matrix

This matrix expands the normative syntax in Structures
[Appendix A](../upstream/structures.html#normative-schemaSchema) and Datatypes
[Appendix A](../upstream/datatypes.html#schema). Its parameters are the
`appendix_schema_grammar` records in [inventory.json](../inventory.json),
extracted from the **dated** appendix fragments. Their paths distinguish local
and global uses of the same element name. Annotation documentation inside
those schemas is explanatory, not an extra grammar rule.

An admitted syntax shape still has to satisfy the representation and component
constraints in [Structures](structures.md) and [Datatypes](datatypes.md).
Passing the schema-for-schemas alone never proves schema correctness.

For each family, instantiate **every applicable grammar path**. A ref resolves
to its declaration; an extension includes its base; a restriction uses the
restricted grammar; omitted occurrence bounds mean one. Do not flatten away
choice alternatives, referenced groups, local declarations, anonymous types,
or wildcards when selecting cases. Retain both a minimal satisfying witness
and one isolated mutation for each forbidden shape.

## G-001 Element identity and context

For every `element` name/ref, use its expanded name at each admitting grammar
path. Change only its prefix, then its namespace, then its local name. Prefix
renaming preserves identity; a changed expanded name requires another
admitted branch or wildcard. Move a valid construct between top-level,
local-declaration, reference, anonymous-type, and derivation contexts. Admission
depends on the destination grammar, even when the local name is unchanged.

Exercise unknown XSD elements, unqualified lookalikes, foreign elements outside
annotations, and foreign payload within `appinfo`/`documentation`. Apply the
actual wildcard and process-contents rules in that context rather than a
global “ignore foreign XML” policy.

## G-002 Required, optional, prohibited, and defaulted attributes

For every declared attribute use, test absent, one valid lexical value, and
one invalid lexical value. Required absence fails; prohibited presence fails;
optional absence follows its effective default/fixed rule. For every declared
default, compare omitted and explicitly supplied equivalent values. Separate
XML normalization from the attribute's simple-type normalization.

For an inherited attribute use, repeat at each extension/restriction boundary.
If a restriction changes occurrence or value constraints, use the restricted
contract. Do not infer the effective use from one source node in isolation.

## G-003 Attribute vocabulary and namespaces

Add one undeclared unqualified attribute, one XSD-namespace attribute, and one
attribute from another namespace at each applicable complex type. Admission
depends on the effective attribute uses and `anyAttribute`, including the
wildcard's namespace constraint and process-contents mode. In a lax wildcard,
a found declaration still validates the attribute; lack of a declaration is
a different branch.

Keep namespace declarations separate from ordinary attributes. Give two
different prefixes the same namespace and repeat the same expanded attribute
name: XML namespace admission rejects the duplicate before XSD semantics.

## G-004 Attribute type partitions

For every attribute `type`/inline simple type, instantiate its complete
applicable datatype family. At minimum, distinguish `ID`, `NCName`, `QName`,
URI reference, boolean, nonnegative/positive integer, enumeration, union, and
list. A QName with an undeclared prefix is not equivalent to a valid QName
whose component cannot be resolved. `maxOccurs="unbounded"` is a separate
union branch, not an integer overflow sentinel.

Enumerations need each admitted token, an unknown token, case changes, empty
input, surrounding XML whitespace, and a multiple-token value when only one
is allowed. Set-valued controls need empty list, each member, combinations,
`#all`, and mixing `#all` with members. Apply additional semantic constraints
after the lexical grammar.

## G-005 Particle cardinality

For every particle, vary the relevant child count below, at, and above each
finite bound. Include zero, one, explicit zero maximum, omitted bounds,
finite maximum, and unbounded maximum wherever the grammar admits them.
Bound comparison and invalid bound pairs are semantic constraints, not
permission to ignore the lexical grammar.

For nested nullable groups, count occurrences of the **term** at the owning
particle, not merely the number of emitted element children. Repeat a nullable
term zero/one/many times and combine it with a mandatory neighbor. Derivation
checks and instance matching need separate outcomes.

## G-006 Sequence, choice, and all

For each sequence, supply children in declared order, exchange two distinct
positions, omit each mandatory position, and duplicate each bounded position.
For each choice, select every alternative; combine two alternatives only when
the enclosing occurrences admit another choice occurrence. For `all`, permute
children, omit an optional child, omit a required child, and duplicate a child.
Use the XSD 1.0 constraints on `all` and its particles; a more permissive XSD
1.1 shape is not admitted automatically.

Do this for direct groups and references. An empty or singleton group retains
the compositor's semantics. Grammar membership, UPA, and valid restriction
are three separate questions.

## G-007 Child grammar and annotation positions

For every complex type/group grammar, select every choice branch, omit every
optional child, and repeat every repeatable child. Move `annotation` before,
between, and after the semantic children. Test duplicate annotation envelopes,
wrong ordering, unexpected simple/complex-content combinations, and a child
from another derivation branch. Derive expected acceptance from the effective
grammar at that exact path, not a global allowed-child-name list.

## G-008 Text, mixed content, and opaque payload

At every complex-content grammar boundary, insert XML whitespace and then
non-whitespace character data; distinguish element-only, empty, simple, and
mixed types. Comments and processing instructions do not supply character
information items. For `documentation` and `appinfo`, exercise text, nested
foreign elements, namespace redeclarations, and allowed attributes under
their own grammar. Do not interpret payload lookalikes as schema declarations.

## G-009 Base and reference resolution

For each `type`, `base`, `ref`, `itemType`, `memberTypes`, `refer`, and
`substitutionGroup` position, distinguish lexical failure, prefix binding,
absent namespace, correct symbol space, wrong component kind, unresolved
component, and a reference cycle. A type name, attribute name, and element
name may share a spelling in distinct symbol spaces. Inspect `final` and
blocked derivation at the use that requires them.

Use a base with inherited constraints so that extension/restriction/reference
cannot pass by dropping base state. Include chameleon and imported namespace
contexts from the Structures composition families.

## G-010 Schema identity attributes

For every `id` position, use a valid unique ID, repeated ID on another schema
element, malformed ID, and equivalent lexical values after normalization.
Repeat the same spelling in a different schema document to distinguish the
schema-document uniqueness rule from component-name uniqueness. Foreign
annotation payload follows the rule applicable to that payload; it must not
silently become a retained schema component.

## G-011 Facet grammar versus facet meaning

For each facet element, vary required `value`, optional `fixed`, optional
`id`, optional annotation, and foreign attributes. Then apply the datatype
facet rules: applicability, duplicate facet occurrences, fixed inheritance,
mutually exclusive bounds, repeated pattern/enumeration grouping, and values
outside the base space. A syntactically valid `value` string can still fail
facet admission or produce an empty derived value space.

## G-012 Schema-for-schemas self-description

Keep the dated Appendix A bodies and independently distributed schemas
distinct. Check their declarations and references against the captured
grammar inventory, including helper types/groups used only by the syntax.
Do not interpret informative builtin definitions as replacing Part 2's
normative builtin semantics. The published fragments have presentation
wrappers and a displayed DOCTYPE/XML-declaration ordering; converting them
to executable files is a separate recorded transformation.

## G-013 DTD appendices and entity examples

Read Structures Appendix G and Datatypes Appendix B as explicitly
non-normative. Compare DTD-valid/XSD-invalid and XSD-valid/DTD-invalid shapes
where the formalisms differ. A DTD cannot establish type derivation, expanded
QName resolution, facets, or UPA. The Primer's entity examples require an
XML processor supplying the corresponding infoset; they do not require a
validator library to fetch external entities or schemas dynamically.

## Concrete expansion and audit status

[Concrete grammar contexts](grammar-contexts.md) resolves the dated helper
types, inheritance, references and contextual restrictions into specific
witnesses. Its ownership partition accounts for the inventory nodes; the
generic families here remain the mutation strategy applied to those contexts.

These are described grammar obligations, not evidence that a fixture exists
for every path. The later audit must record each applicable instantiation,
semantic prerequisite, oracle, and observable result. The inventory includes
helper schema components so that syntax cases cannot disappear merely because
no existing implementation branch or corpus filename suggests them.
