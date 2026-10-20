# XSD 1.0 source authority and corrections

The reference baseline is the English **28 October 2004 Second Edition**,
read with the approved Second Edition errata. This note records source
authority and acquisition boundaries; it does not compare implementations or
corpus coverage. Sources were checked on 17 October 2026.

## Editions and authority

| Source | Role |
| --- | --- |
| [Part 0: Primer, Second Edition](https://www.w3.org/TR/2004/REC-xmlschema-0-20041028/primer.html) | Explanatory material; the whole Primer is non-normative, including its appendices. |
| [Part 1: Structures, Second Edition](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html) | Normative language definition, with explicitly non-normative sections identified below. |
| [Part 2: Datatypes, Second Edition](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html) | Normative datatype definition, with explicitly non-normative sections identified below. |
| [First Edition errata](https://www.w3.org/2001/05/xmlschema-errata.html) | Historical corrections already incorporated into the Second Edition; do not apply them again. |
| [Second Edition errata](https://www.w3.org/2004/03/xmlschema-errata.html) | Approved corrections to apply when reading the Second Edition. |

The Second Edition is a corrected edition of XSD 1.0, not XSD 1.1. Its
publication status says the English text is normative and identifies the XML
source and visible-change XHTML as non-normative alternative formats. The
[visible-change Structures document](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures-with-errata.html)
explicitly compares the Second Edition with May 2001. Its filename does not
mean that post-2004 errata have been applied.

The checked official publication links still identify the 2004 text; they do
not provide a linked consolidation incorporating E1-56. In particular, the
visible-change document retains the old canonical-form wording in §3.3.1.
Use the published text plus the separate approved erratum as the authoritative
combination. A repository reading overlay is commentary, not another W3C
edition.

The [First Edition errata introduction](https://www.w3.org/2001/05/xmlschema-errata.html)
records closure on 18 March 2004, with E1-55, E2-65, E2-66, and E2-67 added
afterward in response to review. Its revision is dated 15 October 2004. Those
late corrections belong to the published Second Edition baseline too; the
March Proposed Edited Recommendation is not the final baseline.

## Complete post-Second-Edition errata inventory

The [approved errata page](https://www.w3.org/2004/03/xmlschema-errata.html),
revision 1.2 of 19 August 2010, contains exactly one entry: **E1-56, Error**.
It explicitly lists no Part 0 or Part 2 errata. This inventory follows that
published list, rather than treating every issue-tracker discussion as an
approved correction.

| ID | Affected clauses | Reading rule |
| --- | --- | --- |
| E1-56 | Structures §3.2.5, Attribute Validated by Type | Attribute normalized value and union-member PSVI properties require successful type validation. |
| E1-56 | Structures §3.3.1, element value constraints | An applied default supplies the constraint's lexical form for PSVI calculation; it creates no character children. Mixed content has no schema-normalized-value property. |
| E1-56 | Structures §3.3.5, Element Validated by Type | A nilled element has no schema-normalized value. Otherwise, simple types and simple content use the applied default's lexical form, or a successfully validated instance value; other cases are absent. Union-member properties require that normalized value. |
| E1-56 | Structures §3.3.5, schema specified | The property is `schema` exactly when Element Locally Valid (Element) clause 5.1 applies; otherwise it is `infoset`. Remove the old additional canonical-form assertion. |

The official wording, including its references and branch ordering, is in
[E1-56](https://www.w3.org/2004/03/xmlschema-errata.html). The overlay applies
all its changes together; it does not extend defaulting to nilled elements or
change the input children. Clause 3.3.4 remains unchanged by this erratum.

[Bug 6836](https://www.w3.org/Bugs/Public/show_bug.cgi?id=6836) is marked
`CLOSED FIXED`, version `1.0 only`. Its disposition records approval, with
modifications, on **9 July 2010**, and publication on **19 August 2010**.
[Bug 2632](https://www.w3.org/Bugs/Public/show_bug.cgi?id=2632) is the related
`1.1 only` issue. Its draft proposals and intermediate discussion are not
alternative XSD 1.0 patch text. Apply the final published E1-56 wording, not
the earlier proposal referring to canonical representations. Bugs concerning
XDM are context, not changes to the XSD 1.0 specification.

Related later publications are not additional XSD 1.0 errata:

| Publication | Status and application boundary |
| --- | --- |
| [Processing XML 1.1 documents with XML Schema 1.0 processors, 11 May 2005](https://www.w3.org/TR/2005/NOTE-xml11schema10-20050511/) | Working Group Note describing an XML 1.1 interoperability strategy, with acknowledged changes in accepted names. It does not amend the repository's XML 1.0-only contract. |
| [Unicode block names for use in XSD regular expressions, 9 June 2011](https://www.w3.org/TR/2011/NOTE-xsd-unicode-blocknames-20110609/) | Working Group Note explicitly containing no normative information. Useful block-name history; it cannot replace XSD 1.0 Appendix F or the chosen Unicode database. |
| [An XSD datatype for IEEE floating-point decimal, 9 June 2011](https://www.w3.org/TR/2011/NOTE-xsd-precisionDecimal-20110609/) | Separate Working Group Note defining `precisionDecimal`; it does not add an XSD 1.0 built-in datatype. |

## Appendix scope

Keep the exact publication labels. An appendix without an explicit
non-normative label must not be discarded merely because it is an appendix.
Examples and notes inside normative material remain explanatory; a table of
contents alone does not make every sentence a conformance requirement.

| Part | Appendices and publication labels |
| --- | --- |
| Primer | A Acknowledgements; B Simple Types and their Facets; C Using Entities; D Regular Expressions; E Index, including element and attribute indices. All are non-normative because the Primer is non-normative. |
| Structures | A Schema for Schemas; B References; C Outcome Tabulations (C.1 validation rules, C.2 PSVI contributions, C.3 representation constraints, C.4 component constraints); D Required Information Set Items and Properties: explicitly normative. E Schema Components Diagram; F Glossary; G DTD for Schemas; H UPA Analysis; I References; J Acknowledgements: explicitly non-normative. |
| Datatypes | A Schema for Datatype Definitions: explicitly normative. B DTD for Datatype Definitions; G Glossary; I Acknowledgements: explicitly non-normative. C Datatypes and Facets; D ISO 8601 Date and Time Formats; E Adding durations to dateTimes; F Regular Expressions: not labeled non-normative. H References distinguishes H.1 Normative from H.2 Non-normative. |

The appendix inventories are visible in the
[Primer](https://www.w3.org/TR/2004/REC-xmlschema-0-20041028/primer.html),
[Structures](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html),
and [Datatypes](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html)
contents. Retain the unqualified Datatypes appendices in requirement analysis:
they contain date-format restrictions, duration arithmetic, and regex grammar.

Structures §§3.14.1 and 3.14.2 are themselves marked non-normative; the simple
type definition and its XML representation are owned by Part 2. Structures
§1.3 gives Appendix A plus the Schema Representation Constraints precedence
over a conflicting XML representation illustration. Both Appendix A schemas
also distinguish their normative syntactic constraints from non-normative
`documentation` text.

## Source and appendix acquisition

Preserve original bytes, source URL, retrieval date, content type, and hash.
Use dated publication URLs for the baseline. The following are exact official
resource URLs, grouped to distinguish required reading from source rebuilds.
Availability of less common auxiliary URLs must be checked during acquisition;
this note does not turn a failed fetch into a successful snapshot.

### Published documents and correction history

| Resource | Exact URL |
| --- | --- |
| Primer HTML | <https://www.w3.org/TR/2004/REC-xmlschema-0-20041028/primer.html> |
| Primer XML | <https://www.w3.org/TR/2004/REC-xmlschema-0-20041028/primer.xml> |
| Primer historical diff | <https://www.w3.org/TR/2004/REC-xmlschema-0-20041028/primer-with-errata.html> |
| Structures HTML | <https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.html> |
| Structures XML | <https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.xml> |
| Structures historical diff | <https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures-with-errata.html> |
| Datatypes HTML | <https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.html> |
| Datatypes XML | <https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.xml> |
| Datatypes historical diff | <https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes-with-errata.html> |
| First Edition errata | <https://www.w3.org/2001/05/xmlschema-errata.html> |
| Second Edition errata | <https://www.w3.org/2004/03/xmlschema-errata.html> |
| E1-56 disposition | <https://www.w3.org/Bugs/Public/show_bug.cgi?id=6836> |

### External appendix bodies and source dependencies

The existing `xml/structures.xml` and `xml/datatypes.xml` are not complete
reading copies. They retain deleted historical text under `diff="del"`, and
Appendix A refers to external files through empty `eg` elements. Structures
outcome tabulations and glossaries also require source transformation.
Accordingly, raw XML text extraction cannot prove complete coverage.

| Resource | Exact URL |
| --- | --- |
| Structures Appendix A body | <https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.xsd.txt> |
| Structures Appendix G body | <https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.dtd.txt> |
| Datatypes Appendix A body | <https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.xsd.txt> |
| Datatypes Appendix B body | <https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.dtd.txt> |
| Structures diagram | <https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/components.gif> |
| Structures diagram map | <https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/image-map.html> |
| Datatypes diagram | <https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/type-hierarchy.gif> |
| Datatypes diagram map | <https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/image-map.html> |

The `.xsd.txt` bodies are presentation fragments with `pre`/CDATA wrappers,
not directly usable XML schemas. The
[Structures fragment](https://www.w3.org/TR/2004/REC-xmlschema-1-20041028/structures.xsd.txt)
includes `datatypes.xsd` and imports `http://www.w3.org/2001/xml.xsd`. Its
DOCTYPE names `XMLSchema.dtd`. The
[Datatypes fragment](https://www.w3.org/TR/2004/REC-xmlschema-2-20041028/datatypes.xsd.txt)
also names `XMLSchema.dtd`. The DTD for datatype definitions is a module of
the schema DTD, not a standalone grammar. Preserve both embedded appendix
content and independently distributed resources; their roles differ.

Each XML source names local `schemaSpecs.dtd`, which includes `xmlspec.dtd`.
The current checked-in copies satisfy that entity-parsing dependency. For
fresh acquisition, the corresponding dated directories provide these relative
resource names. The XML stylesheet processing instruction names
`xmlschema_nodiffs.xsl`; any rebuild must recursively capture its XSL imports
and `document()` resources too. Rendered official HTML avoids making source
rebuild completeness a prerequisite for semantic reading.

The publication headers link these independent, **undated** resources:

| Resource | Official URL |
| --- | --- |
| Combined schema for schemas | <https://www.w3.org/2001/XMLSchema.xsd> |
| Schema DTD | <https://www.w3.org/2001/XMLSchema.dtd> |
| Datatype DTD module | <https://www.w3.org/2001/datatypes.dtd> |
| Built-in datatypes in a separate namespace | <https://www.w3.org/2001/XMLSchema-datatypes.xsd> |
| Imported XML namespace declarations | <https://www.w3.org/2001/xml.xsd> |

These URLs require retrieval hashes because their paths are not edition
identifiers. A fresh undated resource must not silently replace the dated
Appendix A text as the 2004 authority. A complete executable schema-source
closure follows all include/import and external DTD references found in the
retrieved bytes; a complete reading closure includes the normative HTML's
embedded appendix text even when an auxiliary file is unavailable.

The acquired files demonstrate this distinction: the dated
[`datatypes.xsd.txt`](upstream/datatypes.xsd.txt) identifies Part 2 revision
1.4 of 29 May 2004, whereas the independently served
[`XMLSchema.xsd`](upstream/XMLSchema.xsd) identifies its incorporated Part 2
revision as 1.3 of 23 January 2004. They are not interchangeable copies of
Appendix A. The `.xsd.txt` fragments also place an XML declaration after the
displayed DOCTYPE; executable extraction must be recorded as a transformation,
not described as an unchanged W3C source file.

## Normative reference boundary

Do not replace dated dependencies with whatever a latest-version URL serves.
The following inventory follows Structures Appendix B and Datatypes H.1.
Record inconsistent or stale bibliography details as source issues; do not
silently choose a modern replacement.

| Dependency | Edition or exact official source |
| --- | --- |
| XML | Structures cites [XML 1.0 Second Edition, 6 October 2000](https://www.w3.org/TR/2000/REC-xml-20001006/). Datatypes labels Second Edition but its source entity links the [14 August 2000 Working Draft](https://www.w3.org/TR/2000/WD-xml-2e-20000814/). Preserve that discrepancy. |
| XML Namespaces | [14 January 1999 Recommendation](https://www.w3.org/TR/1999/REC-xml-names-19990114/). |
| XML Infoset | Structures cites the [16 March 2001 Working Draft](https://www.w3.org/TR/2001/WD-xml-infoset-20010316/); do not silently replace it with the later Infoset Recommendation. |
| XPath | [XPath 1.0, 16 November 1999](https://www.w3.org/TR/1999/REC-xpath-19991116/), with the XSD identity-constraint XPath subset defined by Structures. |
| XPointer | [8 January 2001 Working Draft](https://www.w3.org/TR/2001/WD-xptr-20010108/), as listed in Structures Appendix B. |
| XML Schema Requirements | [15 February 1999 Note](https://www.w3.org/TR/1999/NOTE-xml-schema-req-19990215). |
| XML Base | [27 June 2001 Recommendation](https://www.w3.org/TR/2001/REC-xmlbase-20010627/). |
| XLink escaping | [XLink 1.0, 27 June 2001, §5.4](https://www.w3.org/TR/2001/REC-xlink-20010627/#link-locators); only its URI-reference escaping procedure is normatively referenced. |
| URI syntax | [RFC 2396](https://www.rfc-editor.org/rfc/rfc2396) and [RFC 2732](https://www.rfc-editor.org/rfc/rfc2732); RFC 3986 is not an XSD 1.0 erratum. |
| IPv6 text dependency | RFC 2732 delegates `IPv6address` to [RFC 2373, July 1998](dependencies/rfc2373.txt), whose original text and correction-status records are retained; see the boundary below. |
| MIME/base64 | [RFC 2045](https://www.rfc-editor.org/rfc/rfc2045), subject to XSD's own lexical rules. |
| Language tags | The `language` datatype invokes [RFC 3066, January 2001](dependencies/rfc3066.txt). The published H.1 bibliography also retains [RFC 1766, March 1995](dependencies/rfc1766.txt). |
| Unicode | [Unicode Character Database 3.1.0](https://www.unicode.org/Public/3.1-Update/UnicodeCharacterDatabase-3.1.0.html), plus the specification's explicit allowances and regex category/block definitions. |
| Floating point | The [1985 technical text](dependencies/ieee754-1985.pdf) and [author-corrected 1990 report](dependencies/clinger-1990-corrected.pdf) are retained with their edition qualifications below. The [2002 interpretation](dependencies/ieee754-1985-interpretation-2002.pdf) is separate from XSD errata. |

The [published Datatypes H.1](upstream/datatypes.html#normative-biblio) prints
1995 beside RFC 3066; the retained RFC header identifies January 2001 and says
it obsoletes RFC 1766. The XML-source deleted-history qualification is a
separate source view: it does not remove the RFC 1766 entry from the published
HTML. These bibliography details do not replace XSD's `language` lexical rule
or add a modern language-tag registry policy.

This is the direct normative-reference inventory, not a claim that every
referenced standard and its transitive bibliography has been mirrored. Source
acquisition should identify unavailable or separately licensed dependencies
explicitly. ISO 8601 appears in Datatypes' non-normative bibliography; the
binding XSD lexical rules and deviations remain in the XSD text.

### Floating-point source versions

The [IEEE archive file record](https://ieeemilestones.ethw.org/File:Ieee754-1985.pdf)
identifies the retained [technical text](dependencies/ieee754-1985.pdf).
Its opening editorial note declares reformatting and spelling/punctuation
corrections. Keep that qualification: these are unchanged downloaded bytes,
not a facsimile of the original publication. The [official interpretation](dependencies/ieee754-1985-interpretation-2002.pdf),
dated 8 October 2002, concerns division by zero; it does not amend XSD.

The University of Oregon's [CIS-TR-90-01](dependencies/clinger-1990-corrected.pdf),
5 June 1990, identifies itself as an author-corrected version of the cited
Clinger paper. Its final notes add a positive-error premise to Corollary 3,
change Theorem 5's precision premise from four to five, and clarify compiler
control of arithmetic destinations. Do not label this report as the identical
ACM proceedings artifact or these corrections as W3C errata. Section 8 leaves
bounded-exponent overflow/underflow policy to the particular number system.

The retained versions remove the previous full-text acquisition limit.
The [external-rule scenarios](scenarios/external-rules.md#floating-point-and-precision-evidence)
still distinguish the published XSD double interval from IEEE parameters and
conditional IEEE overflow behavior from an unqualified XSD oracle. Individual
publisher notices remain in the PDFs; W3C document terms do not license them.

### Indirect IPv6 syntax dependency

[RFC 2732 §3](dependencies/rfc2732.txt) extends RFC 2396's host grammar with
bracketed IPv6 literals and delegates their address syntax to
[RFC 2373](dependencies/rfc2373.txt). The retained July 1998 text contains both
§2.2's conventional representations and Appendix B's reference ABNF. Their
mixed IPv4 forms disagree: §2.2 gives `::13.1.68.3`, whereas Appendix B's
optional colon before IPv4 permits a third colon and cannot generate that
particular example. The unbounded hexadecimal sequence also does not by itself
encode §2.2's eight-piece address size.

The official records distinguish two reports. [Erratum 455](dependencies/rfc2373-errata-455.html)
is **Verified** and corrects only a bibliography number, RFC 1993 to RFC 1933.
[Erratum 456](dependencies/rfc2373-errata-456.html), proposing a mixed-address
grammar repair, is **Rejected**; its verifier notes that RFC 3513 replaced the
document. That proposed grammar is not an approved patch to this retained
dependency. Nor does obsolescence silently replace XSD's cited editions with
RFC 3513, RFC 4291 or RFC 3986.

Use the original clauses together and preserve the disagreement where they
do not determine one result. [XR-049](scenarios/external-rules.md#ipv6-text-forms-within-xr-049)
records concrete source-qualified contrasts. Retaining this dependency does
not import IPv6 routing or address-assignment policies into anyURI validation.

## Reuse and the corrected reading layer

Keep W3C originals unchanged, including copyright notices and publication
status. The specification's document-use link currently resolves to the
[2023 Document License](https://www.w3.org/copyright/document-license-2023/).
It allows attributed implementation-support derivatives but prohibits their
publication for use as a technical specification. The
[2002 license](https://www.w3.org/copyright/document-license-2002/) did not
grant general derivative rights.

The [W3C intellectual-rights FAQ](https://www.w3.org/copyright/intellectual-rights/)
expressly distinguishes permitted external annotations from copying and
modifying a specification to insert annotations. Therefore use a separate,
authored reading overlay keyed to exact source clauses and the published
E1-56 instructions. Present the authoritative baseline and corrections
together through that overlay; do not distribute a silently patched document
as a new W3C specification. This keeps the corrected interpretation usable
without changing or obscuring the official text's provenance.

This note is independently authored implementation reference material. W3C
source documents retain their original copyright and authority; repository
analysis, scenario descriptions, and coverage classifications are non-normative.
The [accompanying attribution](NOTICE.md) and inventory metadata preserve the
source URLs, original copyright/status, and implementation-support notice.

## Issue history used during scenario analysis

The [interpretation register](INTERPRETATIONS.md) records retained W3C issue
pages concerning simple-content identity fields, skipped subtrees and nilled
fields. It also uses the unchanged [Recommendation comment register](upstream/rec-comments.html),
the [6 January 2004 NSRecurse reply](upstream/nsrecurse-reply-2004.html),
and the [14 February 2002 substitution-order reply](upstream/substitution-order-reply-2002.html).
These added snapshots have the `interpretation-history-not-erratum` role and
individual retrieval metadata in the source manifest. The member-minimum
adjudication follows the unchanged published constraint; the implicit-order
record preserves under-specification. An editor's acknowledgment, drafting
instruction or later-version fix does not silently amend the 2004 baseline.
