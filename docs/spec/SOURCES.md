# XSD 1.0 source authority

The baseline is the English **28 October 2004 Second Edition**, read with
approved Second Edition errata and the selected corrections below. Historical
acquisitions on 17–19 October 2026 are recorded in [source-manifest.json](source-manifest.json). Its URLs, hashes,
byte lengths and retrieval metadata describe those downloads; path entries are
historical audit locations, not a current inventory. Bulk snapshots are not
bundled. The preexisting `xml/` source copies remain historical aids.

## Editions and approved corrections

| Source | Authority |
| --- | --- |
| [Primer](https://www.w3.org/TR/2004/REC-xmlschema-0-20041028/primer.html) | Explanatory throughout, including appendices. |
| [Structures](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html) and [Datatypes](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html) | Normative language definitions, subject to their explicitly non-normative labels. |
| [First Edition errata](https://www.w3.org/2001/05/xmlschema-errata.html) | Already incorporated into the final Second Edition, including corrections added after the March 2004 proposed edition. Do not apply twice. |
| [Second Edition errata](https://www.w3.org/2004/03/xmlschema-errata.html) | Captured revision 1.2, 19 August 2010, contains only E1-56 and no Part 0/Part 2 entries. Apply the [corrected reading](corrected-reading.md). |

The English published text is normative. Raw XML retains deleted history and
external/generated placeholders; its extraction cannot prove complete reading
coverage. Visible-change HTML compares 2001 with 2004 and does not incorporate
E1-56. [Bug 6836](https://www.w3.org/Bugs/Public/show_bug.cgi?id=6836)
records E1-56 approval with modifications on 9 July 2010 and publication on
19 August. Earlier proposals, issue closure and XSD 1.1 changes are not automatically
approved XSD 1.0 amendments. Deliberate library adoptions are listed below.

## Selected corrections

A later specification can clarify an XSD 1.0 defect without requiring XSD 1.1
support. Adopt such a correction individually when primary evidence establishes
its relevance to 1.0, its behavior fits the library's contracts, and opposing
corpus witnesses can distinguish the change. Record its scope here and in the
owning interpretation; do not import surrounding 1.1 features or infer authority
from validator agreement alone.

| Correction | Deliberately adopted behavior | Owner |
| --- | --- | --- |
| W3C [1890](https://www.w3.org/Bugs/Public/show_bug.cgi?id=1890), with [4363](https://www.w3.org/Bugs/Public/show_bug.cgi?id=4363), both classified 1.0/1.1 | XSD/XSI namespace references need no import; component kind, existence and source authority still apply. | [I-006](INTERPRETATIONS.md#i-006--xsdxsi-schema-references) |
| W3C [4363](https://www.w3.org/Bugs/Public/show_bug.cgi?id=4363), June 2007 resolution | XSI defaults/fixed values supply no absent attributes. Construction checks, physical intrinsic assessment and required presence remain independent. | [I-007](INTERPRETATIONS.md#i-007--xsi-attribute-uses-and-psvi-contributions) |
| W3C [4299](https://www.w3.org/Bugs/Public/show_bug.cgi?id=4299#c2), classified 1.0/1.1, March 2007 resolution | Physical `xsi:type` must resolve to a type even on an undeclared laxly assessed element. Skip remains unassessed; lookup uses compiled components only. | [I-014](INTERPRETATIONS.md#i-014--xsitype-under-element-wildcards) |
| W3C [1937](https://www.w3.org/Bugs/Public/show_bug.cgi?id=1937), 1.1-only resolution | Exclude skipped selector and field nodes from identity assessment; retain assessed-key completeness and lax/recovery distinctions. | [I-002](INTERPRETATIONS.md#i-002--fields-inside-a-skipped-subtree) |
| W3C [10236](https://www.w3.org/Bugs/Public/show_bug.cgi?id=10236) | Complex types with simple content supply typed identity fields. | [I-001](INTERPRETATIONS.md#i-001--complex-types-with-simple-content-as-identity-fields) |
| W3C [2219](https://www.w3.org/Bugs/Public/show_bug.cgi?id=2219) | Nilled simple/simple-content fields are selected nodes without tuple values; retain nillable-declaration key exclusion. | [I-003](INTERPRETATIONS.md#i-003--nilled-field-values) |
| W3C [R-240](https://www.w3.org/2001/05/xmlschema-rec-comments.html#pfiNSRecurseCheckCardinality) | Zero the wildcard minimum only for immediate-member comparisons; keep original aggregate constraints. | [I-004](INTERPRETATIONS.md#i-004--published-member-minimum-rule) |
| W3C [R-122](https://www.w3.org/2001/05/xmlschema-rec-comments.html#pfiSubGrpRestrict) | Waive generated substitution-choice order on either operand; keep explicit sibling boundaries. | [I-005](INTERPRETATIONS.md#i-005--implicit-substitution-choice-order) |
| Later [attribute-use validity](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#cvc-au) | Compare ordinary fixed attributes by typed value after lexical/facet validation. | [I-009](INTERPRETATIONS.md#i-009--fixed-attribute-use-values) |
| Later [simple derivation](https://www.w3.org/TR/2012/REC-xmlschema11-1-20120405/#cos-st-derived-ok), identified as a 1.0 error in §G.1.8 | Preserve effective restricted-union facets in nested construction and forbid member-type substitution that bypasses them. | [I-010](INTERPRETATIONS.md#i-010--restricted-unions-retain-their-constraints) |
| [F&O regex correction note](https://www.w3.org/TR/2017/REC-xpath-functions-31-20170321/#regex-syntax) | Treat braces/bar consistently as metacharacters, retaining XSD literal `^`/`$`. | [I-011](INTERPRETATIONS.md#i-011--regex-metacharacter-grammar) |
| Later Structures §G.1.10 and `src-resolve` | Use effective global attribute constraints when local use has no override; resolve keyrefs to key/unique components. Already implemented. | [Correction audit](XSD11_CORRECTIONS.md) |
| Later [double value space](https://www.w3.org/TR/2012/REC-xmlschema11-2-20120405/#double) | Use IEEE-compatible integer-exponent limits −1074…971. Already implemented; no blanket overflow-policy adoption. | [Floating-point scope](#floating-point-source-versions) |

These are explicit library corrections to the pinned baseline, separately
from published Second Edition errata. Several use final 2012 text; R-122/R-240
select bounded historical repairs rather than the surrounding 1.1 algorithms.
The [audit](XSD11_CORRECTIONS.md) distinguishes fixes, existing behavior and
excluded language changes. They do not enable HTTP loading, implicit XML/XLink declarations or
other XSD 1.1 features.

## Appendix scope

| Part | Publication labels |
| --- | --- |
| Primer | All appendices are non-normative. |
| Structures | A Schema for Schemas; B References; C Outcome Tabulations; D Required Information Set Items and Properties are explicitly normative. E–J are explicitly non-normative. |
| Datatypes | A Schema for Datatype Definitions is explicitly normative; B DTD, G Glossary and I Acknowledgements are explicitly non-normative. C Datatypes and Facets, D ISO 8601 Formats, E Duration Arithmetic and F Regular Expressions have no non-normative label. H separates normative and non-normative references. |

Structures §§3.14.1–3.14.2 are non-normative; Part 2 owns simple-type definitions
and their XML representation. Structures §1.3 gives Appendix A and Schema
Representation Constraints precedence over conflicting representation
illustrations. Appendix A syntax is normative; its documentation prose is not.
Notes and examples do not acquire normative force from their surrounding section.

Dated appendix bodies and independently distributed schemas differ. The
[dated Part 2 appendix](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.xsd.txt)
identifies revision 1.4 (29 May 2004), while the acquired undated
[XMLSchema.xsd](https://www.w3.org/2001/XMLSchema.xsd) incorporates revision
1.3 (23 January 2004). The `.xsd.txt` bodies contain display wrappers; executable
extraction is a transformation. The acquired [xml.xsd](https://www.w3.org/2001/xml.xsd)
was changed in 2009; it does not establish the declaration selected by a 2004
import. Undated URLs cannot silently replace dated authority.

### Attribute declarations in other vocabularies

Structures [§3.2.7](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html#no-xsi)
defines the four XSI built-in attribute declarations. XML's predefined prefix
does not define an XSD component set or select a version of `xml.xsd`. XLink
attribute declarations likewise come from the supplied schema, rather than the
namespace URI or XSD's reference to XLink 1.0 URI escaping.

For imported XML/XLink attributes, use the supplied declaration and the ordinary
Structures attribute, import and QName-resolution rules. A custom declaration
can expose precedence defects without importing an entire vocabulary schema.
Such a project fixture is not a copy of W3C `xml.xsd` or `xlink.xsd` and must
identify its own purpose and source clauses. XML language/space semantics and
schema-document syntax remain separate from XSD declaration selection. In
particular, accepting an arbitrary `xml:lang` string is not an independent
XML-conformance oracle.

The acquired 2009 `xml.xsd` permits empty `xml:lang`; this does not determine
which XML declaration a 2004 schema used. Tests must supply the declaration whose
behavior they require. Missing imports do not authorize HTTP retrieval under
this library's explicit-source contract. See the
[namespace guide](07_namespaces.md#xml-and-xlink-schema-declarations).

## Referenced-standard boundaries

| Dependency | Edition and caveat |
| --- | --- |
| XML | Structures cites [XML 1.0 Second Edition, 6 October 2000](https://www.w3.org/TR/2000/REC-xml-20001006/). Datatypes labels Second Edition but links the [14 August draft](https://www.w3.org/TR/2000/WD-xml-2e-20000814/); preserve the discrepancy. |
| Namespaces, Infoset, XPath | [Namespaces 1999](https://www.w3.org/TR/1999/REC-xml-names-19990114/), [Infoset 16 March 2001 draft](https://www.w3.org/TR/2001/WD-xml-infoset-20010316/) and [XPath 1.0](https://www.w3.org/TR/1999/REC-xpath-19991116/), with XSD's restricted selector/field syntax. Later editions and full XPath are not implied. |
| URI references | [RFC 2396](https://www.rfc-editor.org/rfc/rfc2396), [RFC 2732](https://www.rfc-editor.org/rfc/rfc2732), [XLink 1.0 §5.4 escaping](https://www.w3.org/TR/2001/REC-xlink-20010627/#link-locators) and [XML Base 2001](https://www.w3.org/TR/2001/REC-xmlbase-20010627/). No automatic RFC 3986 replacement, absolutization, URI normalization or network-equivalence oracle. |
| Language and binary | [RFC 3066, January 2001](https://www.rfc-editor.org/rfc/rfc3066) is invoked by language despite its bibliography's printed 1995 date; [RFC 1766](https://www.rfc-editor.org/rfc/rfc1766) also remains listed. XSD owns its lexical pattern and string equality; no modern registry policy follows. [RFC 2045](https://www.rfc-editor.org/rfc/rfc2045) does not override XSD's tighter base64 grammar. |
| Unicode | [UCD 3.1.0](https://www.unicode.org/Public/3.1-Update/UnicodeCharacterDatabase-3.1.0.html), subject to publication-time support and encouraged future support. State the dataset/version before scoring changed assignments or later blocks; XML name rules are a separate boundary. |
| Background | [Requirements 1999 Note](https://www.w3.org/TR/1999/NOTE-xml-schema-req-19990215) and [XPointer January 2001 draft](https://www.w3.org/TR/2001/WD-xptr-20010108/) do not add unadopted features or arbitrary XPointer evaluation. ISO 8601 is non-normative bibliography; XSD's explicit lexical rules and deviations govern. |

The [XML 1.1 interoperability Note](https://www.w3.org/TR/2005/NOTE-xml11schema10-20050511/),
[Unicode block-name Note](https://www.w3.org/TR/2011/NOTE-xsd-unicode-blocknames-20110609/)
and [precisionDecimal Note](https://www.w3.org/TR/2011/NOTE-xsd-precisionDecimal-20110609/)
are not additional XSD 1.0 errata. They do not change this library's XML 1.0
contract, silently update its regex baseline or add a built-in precisionDecimal.

### Floating-point source versions

The acquired [IEEE 754-1985 technical text](https://ieeemilestones.ethw.org/File:Ieee754-1985.pdf)
declares reformatting and spelling/punctuation corrections; it is not an
original-publication facsimile. The [8 October 2002 interpretation](https://standards.ieee.org/wp-content/uploads/import/documents/interpretations/754-1985_interp.pdf)
concerns division by zero and is not an XSD amendment. Oregon's
[CIS-TR-90-01](https://scholarsbank.uoregon.edu/bitstreams/ba79372c-1283-483a-9915-24e45d1417dd/download)
(5 June 1990) is an author-corrected Clinger report, not the identical ACM
proceedings artifact: its final notes add a positive-error premise to Corollary
3, change Theorem 5's precision premise from four to five and clarify destination
arithmetic. Section 8 leaves bounded overflow/underflow policy to the number system.

Datatypes §3.2.5 prints double integer-exponent limits **−1075…970**; IEEE's
precision 53 and Emin −1022/Emax 1023 yield **−1074…971**. The library explicitly adopts those corrected extrema;
corpus cases retain that source qualification. This does not settle every
overflow threshold. IEEE nearest rounding with default
non-trapping overflow has a conditional float threshold `2^128−2^103`; that
condition does not establish an unqualified XSD infinity-mapping oracle.
Ordinary source-supported midpoint rounding remains distinct. Decimal requires
at least 18 digits; distinguish documented precision/resource refusals from
lexical/value-space invalidity. Do not derive decimal or temporal precision
expectations from host floating-point rounding.

### Indirect IPv6 syntax dependency

RFC 2732 delegates address syntax to [RFC 2373](https://www.rfc-editor.org/rfc/rfc2373).
Its §2.2 mixed-address example `::13.1.68.3` conflicts with Appendix B's
optional-colon ABNF; the unbounded hexadecimal sequence also fails to encode
the eight-piece limit alone. [Erratum 455](https://www.rfc-editor.org/errata/eid455)
is verified and fixes only a bibliography number.
[Erratum 456](https://www.rfc-editor.org/errata/eid456), proposing grammar repair,
is rejected. Preserve the disagreement; obsolescence does not substitute
RFC 3513/4291 or turn the rejected proposal into an approved patch.

## Use of these notes

[Interpretations](INTERPRETATIONS.md) distinguish published rules, WG intent,
XSD 1.1 changes and unresolved source integration. Processor output and corpus
expectations cannot settle those boundaries. PSVI/component-output limitations
are observation limits, and product exclusions are separate from source conflicts.
These notes are independent implementation commentary under the accompanying
[attribution](NOTICE.md), not a patched W3C publication or a second expectation registry.
