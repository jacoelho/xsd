# XSD 1.0 corrected reading

Read the [published Structures](upstream/structures.html) and
[Datatypes](upstream/datatypes.html) together with the corrections below.
This is independently authored reading commentary. The unchanged W3C texts
and [approved errata](upstream/errata-second-edition.html#e1-56) remain the
authority; this file is not a new edition of the specification.

The baseline is the **28 October 2004 Second Edition**, which already includes
First Edition corrections. The only subsequently published approved correction
is **E1-56**. Part 0 and Part 2 have no entries on the captured Second Edition
errata page. [Source and edition evidence](SOURCES.md) explains this boundary.

## E1-56 applied reading

The following seven changes apply together. References to numbered clauses
mean clauses of [Element Locally Valid (Element)](upstream/structures.html#cvc-elt),
not similarly numbered sections of another document. Type validation and
validity against an entire declaration are distinct: a value can satisfy its
simple type while another constraint makes the containing item invalid.

| Change | Location in the base | Current reading |
| --- | --- | --- |
| E-01 | [§3.2.5, attribute schema-normalized value](upstream/structures.html#a-schema_normalized_value) | Supply the normalized value as validated only when the attribute's normalized value validates against its governing type. Otherwise the PSVI property is absent. |
| E-02 | [§3.2.5, attribute union member](upstream/structures.html#sic-attrType) | In the paragraph preceding the lightweight member fields, require the attribute value to validate against the governing union type. The earlier full member-component wording is unchanged; preserve the exact-site qualification below. A failed member match does not identify an actual member. |
| E-03 | [§3.3.1, element value constraints](upstream/structures.html#Element_Declaration_details) | When an empty element uses a default, calculate PSVI results as if the constraint's **lexical form** supplied its content. A fixed constraint behaves the same way for empty content; nonempty content must have the required value. Replace the old canonical-form assertion. |
| E-04 | [§3.3.1, inserted explanatory note](upstream/structures.html#Element_Declaration_details) | Applying an element default creates no character information items. A mixed-content element has no schema-normalized-value property; its default remains discoverable through schema-default and schema-specified properties. |
| E-05 | [§3.3.5, element schema-normalized value](upstream/structures.html#e-schema_normalized_value) | Use the ordered cases below. Replace the entire old two-case definition. |
| E-06 | [§3.3.5, element union member](upstream/structures.html#sic-eltType) | In the paragraph preceding the lightweight member fields, require nonabsent schema-normalized value and a governing simple or simple-content union. Its actual member validated that **schema-normalized value**. The earlier full member-component wording is unchanged. |
| E-07 | [§3.3.5, schema specified](upstream/structures.html#e-schema_specified) | Within Element Default Value's declaration-local-assessment entry, the result is `schema` exactly when clause 5.1 applied; otherwise it is `infoset`. Preserve the entry premise below. Delete the old additional assertion that the PSVI contains a canonical lexical representation. |

For E-05, evaluate these branches in order:

1. If clause 3.2 applied, the property is absent.
2. If the governing type is neither simple nor complex with simple content,
   the property is absent.
3. If clause 5.1 applied, use the value constraint's **lexical form**.
4. Otherwise, if the element's normalized value validates against the simple
   type under [String Valid](upstream/structures.html#cvc-simple-type), use
   that normalized value as validated.
5. Otherwise the property is absent.

Do not replace step 3 with canonical formatting. Do not insert text children
at step 3. Do not require the entire element declaration assessment to succeed
at step 4 when the corrected rule only asks whether the simple value validates.

Keep `sic-eltType`'s unchanged outer premise: the element is valid against
its type under Element Locally Valid (Type). Declaration-only fixed or identity
failures can coexist with that premise. Invalid attributes on a complex
simple-content type defeat type validity; successful content validation alone
does not establish its required normalized/member contributions. Preserve that
scope question when interpreting the amended inner cases rather than silently
applying them to every type-invalid element. Invalid lexical/content values
also fail that outer entry; an amended inner absent case does not itself
require universal property absence outside it.

E-02 and E-06 explicitly replace the later paragraphs before the lightweight
member fields, not the earlier full `[member type definition]` descriptions.
Those unchanged descriptions still require a member that actually validated.
Preserve the [full/lightweight integration qualification](scenarios/source-questions.md#psvi-q001--union-member-amendment-sites-and-contributor-scope),
especially member acceptance with governing-union facet failure for attributes,
and contributor-entry/nil/default/value premises for elements. This identifies
patch scope without inventing another correction, two permitted policies or
a member-presence/absence oracle for the qualified boundary.

Keep [Element Default Value](upstream/structures.html#sic-eltDefault)'s
unchanged entry premise: local validity under
[Element Locally Valid (Element)](upstream/structures.html#cvc-elt) has been
assessed. That assessment need not succeed. E-07's `otherwise` applies
within this contributor's entry, not to every element assessed by any route.
For a processor-stipulated complex type with no known element declaration
and no declaration-local assessment, type-based evaluation under
[§5.2](upstream/structures.html#validation_outcome) and
[`cvc-assess-elt` 1.2](upstream/structures.html#c-td) does not itself satisfy
this entry premise. This contributor therefore does not itself require
`infoset` there merely because clause 5.1 did not apply; this is no universal
prohibition on a processor exposing a property.

## Scenarios specific to the correction

Each row is a family to instantiate at the PSVI boundary. A validator exposing
only success/failure cannot establish all these outcomes. That is an
observability limitation to record during the later comparison, not permission
to omit the scenarios.

| ID | Setup and contrasting cases | Required outcome |
| --- | --- | --- |
| E-101 | Present attribute; valid versus invalid lexical value for its declared simple type. Also make a separately fixed-value constraint fail while the lexical value remains type-valid. | Type-valid input has a schema-normalized value; type-invalid input does not. A separate declaration error does not itself negate type validity. |
| E-102 | Union attribute; first member matches, only a later member matches, no member matches. Include two matching members with different normalization/value mappings. | Report the actual validating member only on a successful union validation, using the specification's member selection. No member properties on failed matching. |
| E-103 | Empty element with default/fixed `xs:integer` lexical form `+001`; compare explicit `+001`, explicit `1`, and no constraint. | An applied default contributes its lexical form `+001`, rather than canonical `1`, for the corrected PSVI calculation. Existing character children are not rewritten. No default is invented without a constraint. |
| E-104 | Empty element whose simple type has whitespace processing; compare lexical default `  a  ` and explicitly supplied content. | Keep the distinction between constraint lexical form in the applied-default branch and normalized instance value in the ordinary validation branch. Check type/value facts separately from the recorded string. |
| E-105 | The same valid default on simple type, complex simple content, mixed content, and a type where value constraints are illegal. | Simple and simple-content cases use E-05. Mixed content has no schema-normalized value. Illegal schema value constraints fail schema construction under the unchanged component rules. |
| E-106 | Empty defaulted element with no children; then add a comment/PI, a whitespace character, or a child element. | Comment/PI presence does not create character content. Distinguish the exact element/character-children test in clause 5.1 from lexical whitespace collapse. Text that normalizes to empty is still character content. |
| E-107 | Nillable defaulted element with `xsi:nil=true`, `1`, `false`, `0`, absent nil, and a nonnillable declaration. | A clause-3.2 nilled element gets no schema-normalized value and uses `infoset` for schema-specified because 5.1 did not apply. Other cases follow their own validity and default branches. Invalid nil usage is not rescued by defaulting. |
| E-108 | Valid and invalid instance values for simple/simple-content elements with no applied default. Contrast declaration-only fixed/identity errors with invalid complex-type attributes. | Preserve E-05's inner value cases and sic-eltType's outer type-validity premise. A declaration error alone need not remove type-valid contributions. Invalid complex attributes defeat that outer premise; do not require normalized/member properties merely because the content value validated. Retain the scope qualification instead of inferring from aggregate validity or content validity alone. |
| E-109 | Defaulted union element; use ordered members whose whitespace or typed mappings differ, then explicit instance values and a failed value. | Within the contributor's entry, the amended lightweight member paragraph validates the resulting nonabsent schema-normalized value. Preserve the unchanged full-member wording and E-108's outer-scope qualification for type-invalid cases; the inner cases supply no universal presence/absence result outside that entry. |
| E-110 | Known declaration with cvc-elt local validity assessed: applied default versus applied fixed versus explicit content equal to fixed versus empty content without constraint; retain both successful and failed declaration-local assessments. Contrast an admitted element evaluated against a known, nonabstract processor-stipulated complex simple-content type, with no known element declaration and no cvc-elt declaration-local assessment. | Within Element Default Value's entry, `schema specified` is `schema` exactly when clause 5.1 applied; otherwise `infoset`. Typed equality to a fixed value alone does not mean the schema supplied the value. The type-only contrast does not enter this contributor; its inner `otherwise` does not itself require `infoset`. This establishes neither a universal absence requirement nor rejection of type-only assessment. |
| E-111 | Capture element children before and after every applied-default case, including mixed content. Compare with absent defaulted attributes. | Element defaulting preserves the empty character-child list. Attribute defaulting still contributes attributes under its separate rules; the erratum does not merge the two mechanisms. |

## Other standards and historical material

The [Primer](upstream/primer.html) explains the language but supplies no
additional normative features. The `*-with-errata.html` snapshots show the
2001-to-2004 editorial history; they do **not** include E1-56. Unresolved bug
reports, XSD 1.1 rules, and newer versions of referenced standards are not
automatically corrections to this baseline. Record an actual source tension
as a question with both clauses, rather than selecting the current
implementation's behavior as the oracle.
The [source-question catalogue](scenarios/source-questions.md) preserves known
text-integration and source-version questions without treating them as errata.

## Printed primitive-type summary

[Structures §3.14.7](upstream/structures.html#d0e16395) prints `number` in its
primitive list and omits `decimal`, `QName` and `NOTATION`; its earlier
[simple-type example](upstream/structures.html#Simple_Type_Definitions) also
uses `xs:number`. The same summary says primitive facets are empty.
Its own reference points to the actual
[Part 2 primitive definitions](upstream/datatypes.html#built-in-primitive-datatypes),
which include decimal, QName and NOTATION. Part 2's
[whiteSpace rules](upstream/datatypes.html#rf-whiteSpace) specify preserve for
string and fixed collapse for the other atomic primitives. The
[Appendix A primitive sketches](upstream/datatypes.html#schema) are informative
displays of those definitions.

Follow the actual Part 2 definitions: the printed summary supplies no
`xs:number` builtin and does not remove required whitespace processing.
This records the unchanged published discrepancy. No retained approved
amendment replaces that paragraph; this note is not an approved erratum or a
new primitive inventory.

## Identity-constraint type wording

The [§3.11.1 introduction](upstream/structures.html#Identity-constraint_Definition_details)
says differing-type values can be equal only when one type derives from the
other. The formal
[Identity-constraint Satisfied](upstream/structures.html#cvc-identity-constraint)
rule defines its tuple comparison through Part 2
[Equal](upstream/datatypes.html#equal) on type-determined values.

For two independently defined restrictions of integer that both successfully
supply `1`, the values are the same integer value under the
[existing equality rules](scenarios/datatypes.md#d-003--equality-disjoint-primitives-and-ordered-comparisons).
Do not add a declared-field-type ancestry veto to that comparison. Two distinct
simple children `v` and `w`, selected by `v|w` with field `.`, supply equal
complete tuples for one `unique` constraint. Different declared type names do
not leave this result unspecified. This records the introductory wording's
scope without creating a new equality policy, approved patch or unresolved
sibling-tuple oracle.

## Particle oracle boundaries

The unchanged NSRecurseCheckCardinality rule checks both immediate member
restrictions and aggregate range. Its published member check retains the
original wildcard minimum. The acknowledged historical bug does not supply
an approved zero-minimum replacement; [I-004](INTERPRETATIONS.md#i-004--published-member-minimum-rule)
records the resulting corpus oracle and its intent qualification.

Substitution-set expansion supplies no implicit-choice list order. The
historical proposal to waive order was not incorporated into this baseline.
[I-005](INTERPRETATIONS.md#i-005--implicit-substitution-choice-order) separates
that under-specification from executable order-independent consequences.
Neither record is an additional approved correction.
