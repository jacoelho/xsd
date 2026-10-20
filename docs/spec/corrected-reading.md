# XSD 1.0 corrected reading

Read the [published Structures](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html) and
[Datatypes](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html) together with the corrections below.
This is independently authored reading commentary. The unchanged W3C texts
and [approved errata](https://www.w3.org/2004/03/xmlschema-errata.html#e1-56) remain the
authority; this file is not a new edition of the specification.

The baseline is the **28 October 2004 Second Edition**, which already includes
First Edition corrections. The only subsequently published approved correction
is **E1-56**. Part 0 and Part 2 have no entries on the captured Second Edition
errata page. [Source and edition evidence](SOURCES.md) explains this boundary.

## E1-56 applied reading

The following seven changes apply together. References to numbered clauses
mean clauses of [Element Locally Valid (Element)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-elt),
not similarly numbered sections of another document. Type validation and
validity against an entire declaration are distinct: a value can satisfy its
simple type while another constraint makes the containing item invalid.

| Change | Location in the base | Current reading |
| --- | --- | --- |
| E-01 | [§3.2.5, attribute schema-normalized value](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#a-schema_normalized_value) | Supply the normalized value as validated only when the attribute's normalized value validates against its governing type. Otherwise the PSVI property is absent. |
| E-02 | [§3.2.5, attribute union member](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#sic-attrType) | In the paragraph preceding the lightweight member fields, require the attribute value to validate against the governing union type. The earlier full member-component wording is unchanged; preserve the exact-site qualification below. A failed member match does not identify an actual member. |
| E-03 | [§3.3.1, element value constraints](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#Element_Declaration_details) | When an empty element uses a default, calculate PSVI results as if the constraint's **lexical form** supplied its content. A fixed constraint behaves the same way for empty content; nonempty content must have the required value. Replace the old canonical-form assertion. |
| E-04 | [§3.3.1, inserted explanatory note](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#Element_Declaration_details) | Applying an element default creates no character information items. A mixed-content element has no schema-normalized-value property; its default remains discoverable through schema-default and schema-specified properties. |
| E-05 | [§3.3.5, element schema-normalized value](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-schema_normalized_value) | Use the ordered cases below. Replace the entire old two-case definition. |
| E-06 | [§3.3.5, element union member](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#sic-eltType) | In the paragraph preceding the lightweight member fields, require nonabsent schema-normalized value and a governing simple or simple-content union. Its actual member validated that **schema-normalized value**. The earlier full member-component wording is unchanged. |
| E-07 | [§3.3.5, schema specified](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#e-schema_specified) | Within Element Default Value's declaration-local-assessment entry, the result is `schema` exactly when clause 5.1 applied; otherwise it is `infoset`. Preserve the entry premise below. Delete the old additional assertion that the PSVI contains a canonical lexical representation. |

For E-05, evaluate these branches in order:

1. If clause 3.2 applied, the property is absent.
2. If the governing type is neither simple nor complex with simple content,
   the property is absent.
3. If clause 5.1 applied, use the value constraint's **lexical form**.
4. Otherwise, if the element's normalized value validates against the simple
   type under [String Valid](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-simple-type), use
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
Preserve the full/lightweight integration qualification (PSVI-Q001 in
[interpretations](INTERPRETATIONS.md)),
especially member acceptance with governing-union facet failure for attributes,
and contributor-entry/nil/default/value premises for elements. This identifies
patch scope without inventing another correction, two permitted policies or
a member-presence/absence oracle for the qualified boundary.

Keep [Element Default Value](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#sic-eltDefault)'s
unchanged entry premise: local validity under
[Element Locally Valid (Element)](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-elt) has been
assessed. That assessment need not succeed. E-07's `otherwise` applies
within this contributor's entry, not to every element assessed by any route.
For a processor-stipulated complex type with no known element declaration
and no declaration-local assessment, type-based evaluation under
[§5.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#validation_outcome) and
[`cvc-assess-elt` 1.2](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#c-td) does not itself satisfy
this entry premise. This contributor therefore does not itself require
`infoset` there merely because clause 5.1 did not apply; this is no universal
prohibition on a processor exposing a property.

## Observation and controls

Success/failure alone cannot prove corrected PSVI values or member metadata.
Keep those outcomes unobservable where the API exposes no such output.
Executable controls distinguish lexical defaults such as `+001` from canonical
`1`, comments/PIs from actual character children, nil from nonnil empty content,
simple/simple-content from mixed types, and declaration-only failures from
failures of type validity. Attribute synthesis remains a separate mechanism.
The corpus manifest owns case identities and expectations.

## Other standards and historical material

The [Primer](https://www.w3.org/TR/2004/REC-xmlschema-0-20041028/primer.html) explains the language but supplies no
additional normative features. The `*-with-errata.html` snapshots show the
2001-to-2004 editorial history; they do **not** include E1-56. Unresolved bug
reports, XSD 1.1 rules, and newer versions of referenced standards are not
automatically corrections to this baseline. Record an actual source tension
as a question with both clauses, rather than selecting the current
implementation's behavior as the oracle.
[Interpretations](INTERPRETATIONS.md) preserve known text-integration and
source-version questions without treating them as errata.

## Printed primitive-type summary

[Structures §3.14.7](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#d0e16395) prints `number` in its
primitive list and omits `decimal`, `QName` and `NOTATION`; its earlier
[simple-type example](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#Simple_Type_Definitions) also
uses `xs:number`. The same summary says primitive facets are empty.
Its own reference points to the actual
[Part 2 primitive definitions](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#built-in-primitive-datatypes),
which include decimal, QName and NOTATION. Part 2's
[whiteSpace rules](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#rf-whiteSpace) specify preserve for
string and fixed collapse for the other atomic primitives. The
[Appendix A primitive sketches](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#schema) are informative
displays of those definitions.

Follow the actual Part 2 definitions: the printed summary supplies no
`xs:number` builtin and does not remove required whitespace processing.
This records the unchanged published discrepancy. No retained approved
amendment replaces that paragraph; this note is not an approved erratum or a
new primitive inventory.

## Identity-constraint type wording

The [§3.11.1 introduction](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#Identity-constraint_Definition_details)
says differing-type values can be equal only when one type derives from the
other. The formal
[Identity-constraint Satisfied](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#cvc-identity-constraint)
rule defines its tuple comparison through Part 2
[Equal](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#equal) on type-determined values.

For two independently defined restrictions of integer that both successfully
supply `1`, the values are the same integer value under the
[Part 2 equality rules](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html#equal).
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
