# External rules at the XSD boundary

These witnesses refine X-001–X-015 in
[Primer and boundaries](primer-and-boundaries.md). Their oracles come from
the retained primary sources below, not an implementation or corpus. They
describe XSD-relevant uses of the dependencies; they do not claim complete
conformance coverage of XML, XPath, MIME, or Unicode as separate products.
Use the [scenario method](README.md), [edition record](../SOURCES.md), and
[corrected XSD reading](../corrected-reading.md).

## Applicability and observation

| Class | Meaning |
| --- | --- |
| XML-WF | The XML document/entity is not well-formed before XSD assessment. |
| NS | Namespaces conformance fails even if the underlying XML Name syntax parses. |
| PRE | A required Infoset fact or external premise is unavailable/inconsistent; this is not a demonstrated XSD value failure. |
| XSD | A schema-construction, instance-value, identity, or PSVI rule determines the result after external admission. |
| CHOICE | The cited standard permits a processor option or version/profile choice. Record the selected alternative. |
| CAP | Required minimum capability or documented implementation limit; a limit failure is distinct from semantic invalidity. |
| PRODUCT | Explicit repository scope: XML 1.0, UTF-8, explicit schema sources, no dynamic network/schema-hint loading. A valid broader-standard input can remain outside that scope. |
| OPEN | The retained sources do not establish one unqualified oracle; preserve the stated tension or missing authority. |

Observation boundaries are **I** (bytes/parser/Infoset), **S** (schema
construction and available components), **V** (instance value/validity), **P**
(PSVI), and **R** (URI resolution/acquisition request). A reader/resolver failure
does not establish XML or XSD invalidity. A validity-only interface cannot prove
character preservation, namespace context, typed values, or URI request spelling.

Unless a row changes it, use schema root `r` with no target namespace, declared
`xs:string`; `xs` and `xsi` mean the standard XSD and instance namespaces.
Changing `r`'s type means changing only that declaration. Code-point notation
`U+hhhh` specifies the actual character, not a literal backslash escape in XML.
“Literal CR” means byte 0D in the UTF-8 document; `&#xD;` means the XML character
reference. Non-well-formed variants never reach the downstream XSD assertion.

| Retained authority | Applies here | Witnesses and limits |
| --- | --- | --- |
| [XML 1.0 Second Edition, 6 October 2000](../dependencies/xml-1.0-second-edition.html) | XML syntax, character/name classes, entity/DTD processing, normalization. | XR-001–XR-024. The separately retained [14 August 2000 draft](../dependencies/xml-1.0-20000814.html) is the stale link behind Datatypes' Second Edition label; it is not silently substituted for the final Recommendation. |
| [Infoset Working Draft, 16 March 2001](../dependencies/infoset-20010316.html) and [Structures Appendix D](../upstream/structures.html#infoset) | Required abstract input facts, namespace consistency, entity information. | XR-019–XR-026. Use this cited draft, not the later Infoset Recommendation. |
| [Namespaces, 14 January 1999](../dependencies/namespaces-1999.html) | Expanded names, prefixes, defaults, uniqueness, NCName boundary. | XR-027–XR-035. Later Namespaces errata/editions are not implicitly incorporated. |
| [XPath 1.0, 16 November 1999](../dependencies/xpath-1.0.html), restricted by [selector](../upstream/structures.html#c-selector-xpath) and [field](../upstream/structures.html#c-fields-xpaths) rules | Selection, QName context, node sets, admitted abbreviation forms. | XR-036–XR-043. General XPath functions, arithmetic, and predicates are not added to XSD's restricted language. |
| [RFC 2396, August 1998](../dependencies/rfc2396.txt), [RFC 2732, December 1999](../dependencies/rfc2732.txt), its [RFC 2373 IPv6 dependency, July 1998](../dependencies/rfc2373.txt), [XLink 1.0 §5.4](../dependencies/xlink-1.0.html#link-locators), [XML Base, 27 June 2001](../dependencies/xml-base-2001.html) | URI-reference syntax, escaping, resolution where the XSD use requires it. | XR-044–XR-058. XSD imports XLink's escaping procedure, not its link traversal model or mandatory absolutization of anyURI values. IPv6 text/ABNF disagreement remains explicit. |
| [Unicode Database 3.1.0 description](../dependencies/unicode-3.1.html), [character data](../dependencies/UnicodeData-3.1.0.txt), [blocks](../dependencies/Blocks-3.1.0.txt), [XSD Appendix F](../upstream/datatypes.html#regexs) | Regex categories/blocks and version qualifications. XML's name/character productions remain separate. | XR-059–XR-065 and XR-070–XR-098. Not a complete mirror of the Unicode Standard or all UCD files. |
| [RFC 1766, March 1995](../dependencies/rfc1766.txt), [RFC 3066, January 2001](../dependencies/rfc3066.txt) | XML language identification versus XSD language datatype. | XR-100–XR-104. No automatic import of RFC 5646/current registry or full BCP 47 validation. |
| [RFC 2045, November 1996, §6.8](../dependencies/rfc2045.txt), overridden where stated by [base64Binary](../upstream/datatypes.html#base64Binary) | Alphabet and octet encoding; XSD owns its tighter lexical grammar. | XR-105–XR-109. MIME headers, multipart processing, and mail transport policies are outside this use. |
| [1985 technical text](../dependencies/ieee754-1985.pdf) and [corrected Clinger report](../dependencies/clinger-1990-corrected.pdf), cited in [Datatypes H.1](../upstream/datatypes.html#normative-biblio) | Float/double values and best decimal conversion, subject to [edition qualifications](../SOURCES.md#floating-point-source-versions). | XR-110–XR-117. The printed double interval and XSD overflow policy remain source-qualified; acquisition is no longer the limit. |
| [XML Schema Requirements, 15 February 1999](../dependencies/xml-schema-requirements.html), [XPointer Working Draft, 8 January 2001](../dependencies/xpointer-20010108.html) | Design background and component-reference discussion. | XR-118–XR-119. Neither turns every proposed feature or XPointer scheme into an XSD processor requirement. |

## XML admission and normalization

Authority: XML Second Edition [document grammar](../dependencies/xml-1.0-second-edition.html#sec-well-formed),
[characters](../dependencies/xml-1.0-second-edition.html#charsets),
[names](../dependencies/xml-1.0-second-edition.html#sec-common-syn),
[line ends](../dependencies/xml-1.0-second-edition.html#sec-line-ends),
[attribute normalization](../dependencies/xml-1.0-second-edition.html#AVNormalize),
and [entities](../dependencies/xml-1.0-second-edition.html#sec-references).

| ID | Concrete witness and isolated mutation | Expected result, class, observation |
| --- | --- | --- |
| XR-001 | `<r/>`; delete its only element; append `<s/>`; change to `<r></s>`; place nonwhitespace text before r. | Baseline well-formed; each mutation violates the document/root or matching-tag grammar. No XSD validity result follows from those XML-WF failures. I. |
| XR-002 | `<r a="1" b="2"/>`; rename b to a. Independently reverse attribute order and change quote delimiters without changing values. | Duplicate lexical attribute name XML-WF; order/quote changes preserve the attribute Infoset. Later expanded-name uniqueness is XR-030. I. |
| XR-003 | Body `&lt;&amp;&gt;&apos;&quot;`; compare numeric references for the same characters; replace `&amp;` with undeclared `&AMP;` without a DTD. | The first two deliver the same five characters and same xs:string value. Case-changed undeclared entity is XML-WF, not another spelling of amp. I/V. |
| XR-004 | `<r>&#65;&#x41;</r>`; change lowercase x to uppercase X; remove the semicolon; use `&#x0;`. | Baseline delivers `AA`; malformed references fail syntax; reference to zero violates legal-character constraint even though its digits parse. I, XML-WF. |
| XR-005 | Admit TAB/LF/CR, U+0020, U+D7FF, U+E000, U+FFFD, U+10000, U+10FFFF; contrast U+0000, U+0008, U+001F, U+D800, U+DFFF, U+FFFE, U+FFFF, U+110000, literal or referenced where representable. | Exactly XML Char's intervals govern admission. U+10FFFF and supplementary noncharacters within that interval are not excluded merely by Unicode noncharacter status; discouraged use is not mandatory rejection. Illegal references remain XML-WF. I. |
| XR-006 | `<r>a&lt;b</r>`; replace `&lt;` with literal `<`; compare text containing `]]&gt;` with literal `]]>`. | Escaped variants deliver text; unescaped markup delimiter or forbidden CDATA-close sequence in ordinary text is XML-WF. I. |
| XR-007 | `<r><![CDATA[a<b]]></r>` versus `<r>a&lt;b</r>`; add a comment/PI between a and b in `<r>ab</r>`. Then use comment `<!--a--b-->` or PI target `XML`. | First pair has identical character content; comments/PIs do not add characters to XSD initial value. Double-hyphen comment and reserved case-insensitive xml PI target are XML-WF. XML declaration is not a PI. I/V. |
| XR-008 | `<r>a CR b</r>`, `<r>a CR LF b</r>`, and `<r>a LF b</r>` using literal line breaks; compare `<r>a&#xD;b</r>`. | Literal variants deliver the same `a LF b`; character reference delivers CR. With xs:string they differ; with xs:token both normalize to `a b`. XML normalization occurs before XSD whitespace processing. I/V/P. |
| XR-009 | Literal NEL U+0085 or LINE SEPARATOR U+2028 between a and b, versus literal CR/LF. | NEL/LINE SEPARATOR are legal XML 1.0 Second Edition characters but are not its end-of-line normalization sequences or XML S whitespace. They remain in xs:token and do not split an XSD list merely because a host library considers them whitespace. I/V. |
| XR-010 | Undeclared/CDATA attribute `a="x TAB y"`; compare `a="x&#x9;y"`; then internal entity `<!ENTITY t "&#x9;">` used as `a="x&t;y"`. | Literal TAB becomes SPACE; direct character reference preserves TAB in the initial attribute value; entity replacement TAB is recursively normalized to SPACE. xs:string exposes the difference; xs:token later collapses all to `x y`. I/V/P. |
| XR-011 | DTD declares a as CDATA versus NMTOKENS; explicit value ` SPACE a SPACE SPACE b SPACE `, with declaration read. | XML initial values respectively preserve outer/repeated spaces and collapse to `a b`; XSD then applies its declared type's own whitespace rule. Removing DTD typing does not justify replaying tokenized normalization. I/V. |
| XR-012 | `<r xml:space="preserve"> SPACE a SPACE SPACE b SPACE </r>` under a type allowing xml:space and simple token content; change preserve to default. | xml:space signals application intent; it does not override the XSD token whiteSpace facet. Both type-normalized values are `a b`. Whether the attribute is admitted is a separate XSD attribute rule. V/P. |
| XR-013 | Element/attribute names `A`, `_A`, `A0`, `A-`, `A.`; mutate initial A to `0`, `-`, `.`, or U+0301. Compare Nmtoken `0-A` as content. | Name start versus continuation rules differ; invalid element names are XML-WF before XSD. Nmtoken permits those continuation characters initially; NCName additionally excludes colon. I/V. |
| XR-014 | U+0132 and U+10400 as string content, as names, and as xs:Name/NCName content; compare A. | Both are XML Char and Unicode 3.1 letters, but absent from XML Second Edition's Letter tables. Baseline names/name datatypes/name escapes reject them while string/category witnesses can admit them. Later XML editions' expanded name rules are not inferred. I/V. |
| XR-015 | UTF-8 `<r>é</r>` with absent declaration, UTF-8 declaration, or UTF-8 BOM; truncate the multibyte character, or declare UTF-16 while retaining the UTF-8 byte stream. Compare correctly encoded UTF-16 XML. | Malformed/mismatched encoding is XML-WF. Correct UTF-16 lies within general XML processor required support but outside this repository's UTF-8 input contract; classify PRODUCT/CAP rather than XSD-invalid. I. |
| XR-016 | XML declaration version1.0 versus absent; place declaration after a comment; change version to1.1. | First pair admitted. Displaced XML declaration fails prolog syntax. XML1.1 is outside the selected baseline/product, not an amendment inferred from a later interoperability Note. XML Second Edition permits a processor to signal unsupported versions. I, PRODUCT/CHOICE. |
| XR-017 | Internal DTD `<!ENTITY e "ab">`, body `&e;`; remove declaration; make e expand to `&e;`; make e expand to `<x>` without closing it. | With declaration read, baseline content becomes ab. Undeclared entity in this internal-only case, recursion, and unbalanced parsed replacement fail XML-WF; no literal `&e;` value may be invented. I. |
| XR-018 | External parsed entity e used in element content; move the same entity reference into an attribute; then use an unparsed entity reference directly in content. | Element inclusion depends on XML processing/acquisition; attribute direct/indirect external reference is XML-WF. Unparsed entities are not substituted through general entity references. Read/access failure is PRE/operational, distinct from XSD type failure. I. |

## DTD and Infoset prerequisites

Authority: [XML processor conformance](../dependencies/xml-1.0-second-edition.html#proc-types),
[Infoset draft](../dependencies/infoset-20010316.html), and
[Structures Appendix D](../upstream/structures.html#infoset). XML DTD validity
and XSD validity are different contracts; a non-validating XML processor is
not automatically a nonconforming XSD processor.

| ID | Concrete witness and isolated mutation | Expected result, class, observation |
| --- | --- | --- |
| XR-019 | Internal `<!ATTLIST r a CDATA "1">`, instance `<r/>`; XSD requires integer a. Compare removing the DTD default while leaving XSD unchanged. | Read DTD supplies a before XSD, satisfying the required attribute. Without it, required a is missing. The supplied input attribute is not an XSD-added default; inspect XML [specified] and PSVI provenance separately. I/V/P. |
| XR-020 | Same default declared only in an external DTD; compare validating XML processor reading it and non-validating processor choosing not to. | Selected XML processor behavior determines whether the initial Infoset has a. Non-validating omission can be permitted; then required a fails XSD on that received Infoset. Do not require an XSD processor to manufacture the unread default. CHOICE/PRE, I/V. |
| XR-021 | Internal parameter-entity declaration is read, then a reference to an unread external parameter entity, then `ATTLIST r a CDATA "1"`; standalone=no. | XML's non-validating processing boundary can prevent use of later entity/attribute-list declarations because unread declarations might override them. An assumption that every visible DTD default was applied is not justified. CHOICE/PRE, I. |
| XR-022 | DTD declares notation pic and unparsed entity logo SYSTEM `logo.bin` NDATA pic; attribute value logo has XSD type ENTITY. Delete entity, change it to a parsed entity, or provide value other. | Only declared unparsed logo satisfies the entity membership premise. A string being an NCName is insufficient. ENTITIES `logo other` fails when other lacks an unparsed declaration. I/V, XSD after prerequisite facts. |
| XR-023 | XR-022's otherwise identical Infoset exposes versus omits document unparsed-entity information. | Appendix D says that lack of this supported information makes ENTITY/ENTITIES fail validation. Record unavailable external facts, not a lexical NCName error or proof that logo was parsed. PRE/XSD, I/V. |
| XR-024 | Well-formed `<r>1</r>` has no DTD; separately a DTD declares r EMPTY but XSD declares integer. Compare `<r>oops</r>`. | DTD validity is not required to have an Infoset. XSD can validate 1 despite its failing an independently enabled DTD validity check; oops fails XSD integer. Keep the reported stage and selected XML validation mode. I/V, CHOICE. |
| XR-025 | Synthetic element lacks [children], attribute lacks [normalized value], or QName-valued content lacks both in-scope namespaces and reconstructible namespace attributes; contrast complete facts. | These omit XSD-required input properties. Treat as failed admission PRE, not an empty child list, empty value, or unbound-prefix semantic result. Unknown/no-value/empty-string/empty-set are distinct Infoset states. I. |
| XR-026 | A consistent synthetic Infoset represents `<r xmlns:p="urn:A"><p:x/></r>`; mutate child expanded namespace to urn:B while retaining only p=urn:A as its in-scope binding. Also leave an unexpanded entity item in an integer's content. | Serialization/construction must settle inconsistent namespace facts; an unread entity is not character content of the empty string. Record PRE unless the consuming interface explicitly defines a resolution. Streams/trees are interchangeable representations only when required facts agree. I. |

## Namespaces 1999

Authority: [declarations](../dependencies/namespaces-1999.html#ns-decl),
[QName](../dependencies/namespaces-1999.html#ns-qualnames),
[scoping](../dependencies/namespaces-1999.html#scoping-defaulting), and
[uniqueness](../dependencies/namespaces-1999.html#uniqAttrs).

| ID | Concrete witness and isolated mutation | Expected result, class, observation |
| --- | --- | --- |
| XR-027 | `<p:r xmlns:p="urn:A"/>`; remove declaration; move it onto the same start tag from the parent; rename p consistently. | Undeclared prefix fails NS. Same-tag declaration is in scope for the element itself; consistent prefix rename preserves expanded name and XSD selection. I/S/V. |
| XR-028 | `<r xmlns="urn:A"><x/><x xmlns=""/></r>`; use xmlns:p="" instead of xmlns="". | First x belongs to urn:A, second to no namespace; empty prefixed binding is forbidden by 1999 rules. Do not import Namespaces1.1 prefix undeclaration. I, NS. |
| XR-029 | `<p:r xmlns:p="urn:A"><p:x xmlns:p="urn:B"/><p:x/></p:r>`; add a sibling outside the shadowing scope. | Inner x namespace urn:B; following x namespace urn:A. Parent binding is restored by scope, not mutated globally. The two x declarations/types can differ accordingly. I/S/V. |
| XR-030 | `<r xmlns:p="urn:A" xmlns:q="urn:A" p:a="1" q:a="2"/>`; change q's URI to urn:B. | First duplicates expanded attribute name and fails NS; second has distinct names. `<r xmlns="urn:A" a="1" p:a="2" xmlns:p="urn:A"/>` is also distinct: default namespace does not apply to unprefixed a. I. |
| XR-031 | Use `<r xml:lang="en"/>` without xml declaration; explicitly bind xml to its defined URI; rebind xml to urn:B; use xmlns as an element prefix. | Builtin xml binding is available; contradictory rebinding and use of xmlns outside namespace declarations fail NS. Prefixes beginning any case of xml are reserved. Do not turn later-edition reserved-URI alias rules into asserted 1999 wording without a cited correction. I/OPEN. |
| XR-032 | Bind p to `http://example/A`, then `http://example/a`, then `http://example/%41`; separately compare `urn:A` with `urn:A/`. | Namespace identity is exact character identity after XML attribute normalization, without URI percent decoding, case folding, trailing-slash removal, or fetching. These are distinct names even if some retrieval targets coincide. I/S/V. |
| XR-033 | xmlns:p="../n" with otherwise legal QName p:r; compare urn:n. | The 1999 Namespaces text allows URI references, but the cited March2001 Infoset draft excludes relative namespace references from its defined Infosets. This is a PRE/source boundary, not an unbound-prefix error or XML-WF failure. An explicitly admitted product extension must be recorded as such. I/OPEN. |
| XR-034 | Element/attribute QName p:r; mutate to p:r:s, :r, p:, or digit-start local part. Put colon in an entity name or PI target. | QName shape and Namespaces NCName restrictions fail NS even when bare XML Name syntax would accept colons. The exact XML2e name-character table remains applicable. I. |
| XR-035 | Integer and QName elements both have `<r xmlns="urn:A">n</r>`; a schema reference type="n" has its own default namespace; selector xpath="n" is in that schema. | The element expanded name, QName-valued content, schema QName reference, and XPath name test are different uses. XSD QName resolution uses its defined namespace context; XPath1 unprefixed node tests select no-namespace nodes. Never copy the unprefixed-attribute-name rule to every QName-valued string. I/S/V. |

## XPath 1.0 only through the admitted subset

Use a schema declaring r with repeated children `{urn:A}row` and no-namespace
row, each with integer attribute code and optional child value. Schema prefix
p binds urn:A. A key/unique selector/field uses that schema context. These
are selection/cardinality witnesses; detailed identity equality and missing
field policies remain in the [Structures branch catalogue](branches/composition-and-identity.md).

Authority: XPath [node tests](../dependencies/xpath-1.0.html#node-tests),
[abbreviations](../dependencies/xpath-1.0.html#path-abbrev),
[node sets](../dependencies/xpath-1.0.html#node-sets), and the XSD selector/field
constraints, which take precedence over accepting arbitrary XPath1 expressions.

| ID | Concrete witness and isolated mutation | Expected result, class, observation |
| --- | --- | --- |
| XR-036 | selector `p:row`, field `@code`; change selector to `row` while schema default namespace is urn:A. | First selects qualified rows; second selects only no-namespace rows. Duplicate code values in only the qualified rows expose the difference: selection is not namespace-defaulted by the schema's default namespace. S/V. |
| XR-037 | selector p:row with schema p=urn:A; instance uses q=urn:A, then rebinds its own p=urn:B; finally remove schema binding p. | Instance prefix spelling/rebinding cannot change schema XPath interpretation. Removing schema p makes the expression's QName unresolved, an XSD schema-expression error. I/S/V. |
| XR-038 | selectors `*`, `p:*`, `p:row`, `.`, `./p:row`, `.//p:row`, and `p:row/value`; fields `.`, `@code`, `p:row/@code`. | Each has an admitted production with distinct selected nodes. `*` selects any child element, p:* only urn:A children; field terminal attribute axis is permitted. Changing selector to `@code` violates the selector subset rather than selecting an attribute target legally. S/V. |
| XR-039 | selector `child::p:row`, field `attribute::code`; compare abbreviations `p:row`, `@code`. | Their abbreviated forms satisfy XSD's explicit alternate admission route and select the same nodes. Do not reject every explicit axis just because the abbreviated EBNF does not spell `::`. S/V. |
| XR-040 | Admitted p:row; mutate to `p:row[1]`, `//p:row`, `p:box//p:row`, `..`, `ancestor::p:row`, `text()`, or `count(p:row)`. | General XPath1 validity is insufficient: each mutation lies outside XSD's required selector subset. The field subset similarly does not add predicates/functions/arithmetic. S, XSD. |
| XR-041 | selector `p:row | p:row` on one row code1, versus two distinct selected row elements each code1; field `@code | @code` versus `@code | @other` with both attributes present. | XPath node sets deduplicate repeated selection of the same node. First selector does not create a duplicate key row; two actual rows can. Repeated field branch still yields one node; two distinct attribute nodes violate XSD single-field-node cardinality. V. |
| XR-042 | ` p:row / value | other ` and field ` @ code `; contrast `p : row` and splitting `//` into `/ /`. | At the component-expression boundary, token-boundary whitespace is permitted; whitespace inside a QName or a multi-character token does not preserve that token. XML-schema admission additionally requires the representation constraints and Appendix A patterns: retain [XPS-Q001](source-questions.md#xps-q001--xpath-component-syntax-and-xml-representation), rather than inferring mandatory admission of these XML spellings from the component permission alone. Longest-token recognition prevents treating one expression spelling as a different path. S/OPEN for the representation integration. |
| XR-043 | Two selected integer fields spell `1` and `01`; change the second type to string, keeping text `01`. | XSD identity uses the schema-assessed typed values/equality, not XPath1 string-value or conversion of everything to XPath double. The integer pair is duplicate where unique/key applies; integer1 and string01 are distinct typed values. XPath numeric functions are not invoked. V. |

## URI references, escaping, and XML Base

Authority: RFC2396 §§2–5 and Appendix C; RFC2732 §§2–3; XLink1.0 §5.4;
XML Base [escaping](../dependencies/xml-base-2001.html#escaping),
[granularity](../dependencies/xml-base-2001.html#granularity), and
[matching](../dependencies/xml-base-2001.html#matching); XSD
[anyURI](../upstream/datatypes.html#anyURI). Distinguish a URI's lexical value,
its escaped resolver projection, namespace identity, and actual source access.

| ID | Concrete witness and isolated mutation | Expected result, class, observation |
| --- | --- | --- |
| XR-044 | anyURI content `a b`, `a%20b`, `é`, and `a\b`; compare space before/after content. | XSD whitespace collapse occurs first. XLink projection escapes internal SPACE as %20, é as UTF-8 %C3%A9, backslash as %5C; pre-existing %20 is not double-escaped. Raw datatype string/equality is not silently replaced by this projection. V/R. |
| XR-045 | anyURI `%20`; mutate to `%`, `%2`, or `%GG`. | Percent is exempt from XLink's automatic escaping; it must still form a valid RFC2396 escaped triplet. Mutations remain malformed references rather than becoming %25 and thereby being repaired. S/V/R, XSD lexical boundary. |
| XR-046 | References `g#s`, `g?y`, `g?`, `g#`, empty, and `#s`; add a second unescaped #. | Preserve undefined versus present-empty query/fragment. Empty and fragment-only references are admitted URI references. A second # does not become legal merely because the escaper preserves fragment delimiters. V/R. |
| XR-047 | `g:h`, `./g:h`, `1:g`, and `/tmp/a`; contrast a local filesystem name supplied through an explicit file-source API. | First is absolute opaque URI, second relative path, third invalid URI syntax after escaping, fourth absolute-path reference. A valid URI need not identify an existing local resource; filesystem naming is a separate source contract. V/R. |
| XR-048 | `mailto:not-a-mailbox`, `http://example.invalid/a`, and `urn:example:a` as anyURI values, without fetching. | Generic type validity does not enforce every scheme's context-specific rules or availability. No DNS/HTTP/mail operation is required to validate the value. Failure of later access is operational, not proof of an invalid anyURI. V/R, CHOICE/PRODUCT. |
| XR-049 | RFC2732's `http://[1080::8:800:200C:417A]/foo`; remove closing bracket; move bracketed text into an ordinary path segment. Add the explicit RFC2373 text-form contrasts below. | Published example is admitted IPv6-literal syntax; unbalanced/misplaced brackets are not repaired by XLink escaping. The retained RFC2373 prose and reference ABNF disagree on mixed forms and do not impose identical size bounds. Keep the exact source qualification; no newer URL parser or rejected correction supplies the oracle. V/R, OPEN. |
| XR-050 | Base `http://a/b/c/d;p?q`; resolve `g`, `./g`, `/g`, `//g`, `g:h`. | Results `http://a/b/c/g`, same, `http://a/g`, `http://g`, `g:h`. Distinguishes relative path, absolute path, authority replacement, and scheme override. R. |
| XR-051 | Same base; resolve `?y`, `g?y`, and `g#s`. | RFC2396 Appendix C gives `http://a/b/c/?y`, `http://a/b/c/g?y`, and `http://a/b/c/g#s`. Importing RFC3986's different query-only result changes the cited oracle. R. |
| XR-052 | Same base; resolve `../g`, `g/../h`, `g.`, `.g`, and `g?y/../x`. | Respectively `http://a/b/g`, `http://a/b/c/h`, `http://a/b/c/g.`, `http://a/b/c/.g`, `http://a/b/c/g?y/../x`. Only complete path dot-segments reduce; query content must not be path-normalized. R. |
| XR-053 | Same base; resolve `../../../g`, then `/../g`, then percent-spelled `%2E%2E/g`. | Above-root relative traversal is an error for which RFC2396 §5.2 explicitly permits retaining, removing, or avoiding traversal. Absolute-path `/../g` is not passed through the relative merge cleanup; escaped dots are not decoded into dot segments by that algorithm. Preserve the allowed result set, not one host-library repair. R/CHOICE. |
| XR-054 | Retrieved schema URI `http://a/base/main.xsd`; schema xml:base=`sub/`, nested include xml:base=`../other/`, schemaLocation=`x.xsd`; compare absolute schemaLocation=`http://b/x.xsd`. | Nested base becomes `http://a/base/other/`, relative location requests its x.xsd; absolute location overrides base. xml:base itself resolves against parent/entity base, not against the newly derived local base again. R/S. |
| XR-055 | Same schemaLocation=`x.xsd` under no xml:base, explicit empty xml:base, and xml:base=`http://b/`; document/entity URI is known. | No base uses inherited entity/document base; empty reference means the current base/current document according to RFC2396's same-document branch; absolute base changes the resolved location. Distinguish known empty reference from unknown document base. R/PRE. |
| XR-056 | Parsed external entity retrieved from `http://b/e.xml` contains element with relative URI attribute; compare an internal entity whose replacement text contributes that element. | External entity retrieval supplies its own base, subject to local xml:base; internal entity declaration location does not establish a separate base. Redirected external retrieval uses the final retrieval URI for Infoset base. Acquisition/redirect behavior must be stated before predicting a request. I/R. |
| XR-057 | anyURI value `../x#s` under two different xml:base values; compare schema include's schemaLocation `../x` under those bases. | anyURI validation must not absolutize the typed value; base changes do not change that datatype's relative spelling/value. Source-reference use can resolve to different resources. XLink's use-time absolutization is not imported into anyURI validation. V/R. |
| XR-058 | Same namespace URI in xmlns, schemaLocation hint, and explicit source URI; make HTTP unavailable and then supply identical bytes explicitly. Also use the nested/late hint contrasts below. | Namespace identity never requires dereferencing. Instance hints do not mandate discovery; this product admits explicit sources and excludes dynamic hint/network schema loads. A resolver/open failure is neither namespace inequality nor a completed schema semantic rejection. The Web-convention profile separately constrains hint ordering and global effect. I/R/S, PRODUCT/CHOICE. |

<a id="ipv6-text-forms-within-xr-049"></a>
### IPv6 text forms within XR-049

Apply [RFC2732 §3](../dependencies/rfc2732.txt)'s bracketed-host production to
[RFC2373 §2.2 and Appendix B](../dependencies/rfc2373.txt). Start with anyURI
content `http://[1080:0:0:0:8:800:200C:417A]/`, then its published compressed
form `http://[1080::8:800:200C:417A]/`. Both sources admit these representations;
hexadecimal case and omitted leading zeros within a piece do not change the
address. This does not make the two URI strings equal as XSD anyURI values.

Use `http://[::FFFF:129.144.52.38]/` for a mixed IPv4 tail admitted by both
sources. Separately compare `http://[::13.1.68.3]/` with
`http://[:::13.1.68.3]/`: the former follows §2.2's explicit compressed example,
but Appendix B's `hexpart [ ":" IPv4address ]` generates the latter instead.
The latter does not follow the prose's conventional form. A proposed repair
in [Erratum 456](../dependencies/rfc2373-errata-456.html) is **Rejected**, not an
approved replacement grammar.

Keep size constraints distinct from that colon discrepancy. Seven or nine
uncompressed hexadecimal pieces fit Appendix B's unbounded `hexseq`, while
§2.2's preferred form contains eight 16-bit pieces. A dotted piece of `256`
fits its `1*3DIGIT` ABNF but exceeds the prose's eight-bit piece. Record these
as source-disagreement witnesses, not unqualified positive or negative XSD
oracles. Multiple compression sites, explicit empty pieces outside the single
compression form, nonhexadecimal pieces and more than four hex digits per
piece exercise separate conditions. Prefix-length notation from §2.3 and
later zone-identifier extensions are not added to a bracketed host by copying
a more permissive modern parser.

Observe URI lexical admission or a use-time request as appropriate. No DNS or
IPv6 packet operation is needed, and network address-assignment rules do not
decide whether an anyURI lexical string is valid.

### Nested and late hints within XR-058

Authority: [Structures §4.3.2](../upstream/structures.html#schema-loc),
especially its fourth requirement. State that the Web-convention profile applies
and that the chosen acquisition policy uses the hint and successfully retrieves
`b.xsd`. That source targets `urn:b` and declares integer `b:item`. The already
available declarations for unqualified `r` and `h` admit both child orders
independently of `urn:b`, for example through lax wildcard content.

Use `<r xmlns:b="urn:b" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><h xsi:schemaLocation="urn:b b.xsd"/><b:item>7</b:item></r>`.
The nested hint precedes the first addressed name. Its effect survives the end
of `h`; it does not have namespace-declaration scope. After successful acquisition,
changing `7` to `bad` makes the later item invalid against its integer declaration.

Move the same `h` after `b:item`, keeping both content orders admitted: the hint
now occurs after the first addressed element and violates the stated ordering
requirement. Independently put an addressed `b:a` attribute before the hint to
exercise the attribute-first condition. Addressing a different, not-yet-used
namespace does not satisfy that late-use premise. Keep established components
stable; a later hint cannot replace an already established declaration.

These acquisition and ordering witnesses belong to that stated Web profile.
The repository's explicit-source interface does not expose this dynamic route;
its unsupported/ignored hint outcome cannot prove the profile's global-effect
or ordering behavior. Optional acquisition and the ordering requirement remain
separate questions.

The timing premise is relative to the **initially validated element**, including
addressed attributes as well as elements. An earlier use outside a selected
subtree does not establish the within-root premise; same-item ordering cannot
be inferred from serialized attribute order. Use
[IP-018's full hint product](interactions.md#ip-018--invocationconformance--acquisition--root--optional-observations)
for the separate should-attempt-each-location recommendation, contrary
application/CLI direction, exact first-use facts and global effect. Keep pure
timing witnesses supplied with usable components before assessment so missing
schema information does not mask the intended contrast.

## Unicode property and block inputs

Use xs:string with a single pattern facet and a **single-character** instance
value to avoid confusing the XSD pattern's match behavior with string length.
The declared data profile here is the retained Unicode3.1.0 files. Character
admission is still XML2e. Range markers such as `<Private Use, First>` and
`<..., Last>` in UnicodeData denote ranges; absence of an ordinary row inside
such a range does not mean unassigned.

| ID | Concrete witness and isolated mutation | Expected result, class, observation |
| --- | --- | --- |
| XR-059 | Pattern `\p{Lu}` versus `\p{Ll}` with A/a; `\p{L}` with each letter subcategory below; `\P{L}` with SPACE. | Category membership comes from declared UCD profile, without case folding. Aggregate category is the union of its listed subcategories; complement is relative to XML-admitted characters. V. |
| XR-060 | U+200B under `\p{Zs}` and `\p{Cf}`; U+0220 under `\p{Cn}` and `\p{Lu}`. | Retained3.1 data gives U+200B Zs and U+0220 unassigned. A later supported database may differ; do not use current runtime classification as proof about3.1. XSD encourages future-version support, so record the advertised version before judging a mismatch. V/CHOICE. |
| XR-061 | Pattern `\d` with ASCII0, U+0966, and U+00B2; same values as xs:integer content. | Regex digit means Nd, admitting first two but excluding superscript-two No. Integer lexical digits are ASCII0–9, so U+0966 is not admitted merely because \d matches it. V. |
| XR-062 | `\i` with A, underscore, colon, 0, U+0132; `\c` with the same plus U+0301. Compare xs:NCName content containing colon. | XML2e initial/name character productions own these escapes: A/_/: initial, 0 and combining acute continuation-only, U+0132 excluded. NCName excludes colon even though XML Name and \i admit it. Unicode letter category is not the XML Name table. V. |
| XR-063 | Patterns `\s`, `\w`, and `.` with TAB/LF/CR/SPACE/U+00A0/U+2028/A/underscore/plus. | \s is only XML S; \w excludes P/Z/C, so underscore Pc fails while plus Sm succeeds; dot excludes LF/CR but can match TAB and LINE SEPARATOR. Host regex definitions of word/space/dot cannot replace Appendix F. V. |
| XR-064 | `\p{IsBasicLatin}` with U+007F/U+0080; `\p{IsGreek}` with U+0370/U+03FF/U+0400 and unassigned U+0378; `\p{IsLatin-1Supplement}` versus case/space/hyphen mutations. | Block membership uses ranges, including unassigned characters; BasicLatin ends007F, Greek ends03FF. Block names strip whitespace from the source block name, preserve case/hyphen. Unknown names are a separately recorded syntax/profile question; no automatic Unicode loose-name matching is assumed. V/S. |
| XR-065 | `\p{Cs}` and HighSurrogates block escape; surrogate U+D800 as value; compare `\p{Co}` with U+E000 and supplementary Private Use. Also use IsDeseret with U+10400. | Cs is excluded from XSD's recognized category grammar, and surrogate values are XML-WF failures regardless regex support. Private Use XML characters are valid category/block witnesses. Deseret is in retained3.1 Blocks despite the publication's older displayed block table; preserve the table-versus-cited-database/version boundary before treating rejection as a unique normative defect. S/V/OPEN. |

Every recognized leaf category has the following concrete member. For each
row, use pattern `\p{category}` with that member: required match for this3.1
profile. Mutate only the value to A to obtain a nonmember, except Lu uses
lowercase a. Reverse the result for `\P{category}`. The parent category L/M/N/
P/Z/S/C must also match its member. Thus each row specifies an explicit
positive, isolated negative, complement, and aggregate witness rather than a
generic instruction to test a category.

| ID | Category | Member in retained UnicodeData3.1 |
| --- | --- | --- |
| XR-070 | Lu | U+0041 A |
| XR-071 | Ll | U+0061 a |
| XR-072 | Lt | U+01C5 capital D with small z with caron |
| XR-073 | Lm | U+02B0 modifier small h |
| XR-074 | Lo | U+05D0 Hebrew alef |
| XR-075 | Mn | U+0301 combining acute |
| XR-076 | Mc | U+0903 Devanagari visarga |
| XR-077 | Me | U+0488 enclosing Cyrillic sign |
| XR-078 | Nd | U+0030 digit zero |
| XR-079 | Nl | U+2160 Roman numeral one |
| XR-080 | No | U+00B2 superscript two |
| XR-081 | Pc | U+005F underscore |
| XR-082 | Pd | U+002D hyphen-minus |
| XR-083 | Ps | U+0028 opening parenthesis |
| XR-084 | Pe | U+0029 closing parenthesis |
| XR-085 | Pi | U+2018 left single quotation mark |
| XR-086 | Pf | U+2019 right single quotation mark |
| XR-087 | Po | U+0021 exclamation mark |
| XR-088 | Zs | U+0020 SPACE |
| XR-089 | Zl | U+2028 LINE SEPARATOR |
| XR-090 | Zp | U+2029 PARAGRAPH SEPARATOR |
| XR-091 | Sm | U+002B plus |
| XR-092 | Sc | U+0024 dollar |
| XR-093 | Sk | U+005E circumflex |
| XR-094 | So | U+00A6 broken bar |
| XR-095 | Cc | U+0009 TAB, passed as element character content |
| XR-096 | Cf | U+200E left-to-right mark |
| XR-097 | Co | U+E000 Private Use |
| XR-098 | Cn | U+0378 unassigned in3.1 |

For block boundaries, the retained Blocks file is the finite parameter table:
for each distinct normalized name, its **complete set of recorded intervals**
is the membership oracle. Emit each XML-admitted start/end as members and
their immediate XML-admitted neighbors as members or nonmembers according to
that complete set. This matters because Specials has two intervals and
Private Use has three, while three surrogate intervals admit no XML character
witness. Do not collapse duplicate block names to the last interval. This
defines the expansion for every retained row; the individual endpoint fixtures
have not been materialized or compared with a corpus here.

## Language tags and base64 at their importing boundary

Authority: XML2e [language identification](../dependencies/xml-1.0-second-edition.html#sec-lang-tag),
RFC1766 §2, RFC3066 §§2.1–2.2, XSD [language](../upstream/datatypes.html#language),
RFC2045 §6.8, and XSD [base64Binary](../upstream/datatypes.html#base64Binary).

| ID | Concrete witness and isolated mutation | Expected result, class, observation |
| --- | --- | --- |
| XR-100 | Language syntax `en`, `en-US`, `x-a1`; compare RFC1766 and RFC3066 grammar. | First two meet both syntaxes; numeric subtag in x-a1 meets3066 but not1766. XML2e explicitly mentions1766 or a successor; XSD Second Edition specifically cites3066. Do not impose the older all-letter-subtag grammar on xs:language. V. |
| XR-101 | xs:language content ` SPACE en-US SPACE `, `en-12345678`, `en-123456789`, `en_ US`, `en-`, `1en`, and empty. | After token collapse, first two satisfy XSD's explicit lexical pattern; the mutations violate subtag length/separator/primary/emptiness syntax. Syntax admission does not assert that a particular synthetic tag has a registered interpretation. V/XSD. |
| XR-102 | xs:language values en-US and EN-us; use enumeration en-US then provide EN-us. Contrast RFC language-tag matching with XSD string value equality. | RFC tag interpretation is case-insensitive; XSD language derives from token and does not itself case-fold strings. Do not automatically equate their XSD enumeration values merely because they identify the same natural language. V. |
| XR-103 | xs:language lexical value abcdefgh or a one-letter non-x/non-i primary; compare private-use x-foo. | The broad XSD lexical pattern and RFC3066's assignment/registration restrictions are different predicates. Full registry-dependent validity is not proved by the retained RFC alone; preserve OPEN where a unique value-space interpretation would need registry evidence or an interpretation decision. V/OPEN. |
| XR-104 | xml:lang=en on parent, xml:lang=fr on child; compare absent child attribute and empty xml:lang. | Language intent inherits until overridden, but no physical XML attribute is thereby added to the child for XSD attribute presence. Acceptance of empty language undeclaration cannot be inferred from the undated2009 xml.xsd helper as a correction to the2004 baseline. XML versus XSD declaration typing must be stated. I/V/OPEN. |
| XR-105 | base64 values empty, Zg==, Zm8=, Zm9v; compare =, Zg=, Zg===, and Z=g=. | Values respectively empty octets, f, fo, foo. Wrong quantum/padding placement/count is XSD-invalid even if a permissive MIME decoder accepts something. Output length means decoded octets0/1/2/3, not character count. V. |
| XR-106 | Zg== versus Zh==; Zm8= versus Zm9=. | First of each pair has permitted zero pad bits; mutations fail XSD's B04/B16 grammar even though a permissive decoder might discard those bits. Use XSD lexical grammar, not only decoded-byte equality. V. |
| XR-107 | Zg== with XML TAB/LF/CR/SPACE inserted between characters; insert U+00A0, `!`, or `_` instead. | XML whitespace collapse yields an admitted spelling of f. Non-XML whitespace, ignored-junk MIME characters, and URL-safe alphabet replacements fail XSD grammar. RFC2045's ignore-nonalphabet decoder behavior is not imported. I/V. |
| XR-108 | Concatenate 20 copies of Zm9v into one80-character line; compare wrapping after76 and after4 characters. | All encode the same60 octets and are admitted. The MIME76-column encoding rule must not become an XSD line-length restriction. Canonical XSD representation has no inserted whitespace. V. |
| XR-109 | Base64 encoding Cg== versus DQo=; XML entity encoded UTF-8 or another correctly decoded encoding delivering the same Zg== characters. | Values differ: LF octet versus CRLF octets; XSD does not apply MIME text newline canonicalization to arbitrary decoded octets. Decode XML characters before base64, not source bytes. Broader XML encodings remain PRODUCT-qualified as XR-015. I/V. |

## Floating-point and precision evidence

Authority: XSD [float](../upstream/datatypes.html#float),
[double](../upstream/datatypes.html#double),
[decimal](../upstream/datatypes.html#decimal), and their cited external sources.
The external texts are now retained with [edition qualifications](../SOURCES.md#floating-point-source-versions).
The XSD clauses own their lexical and best-approximation rules. IEEE §5.6's
weaker conversion allowance does not replace them. No arithmetic instruction,
exception flag, signed-zero distinction or alternate rounding mode becomes an
XSD validation requirement merely because it occurs in the dependency.

IEEE Table 1 gives double precision53, Emin−1022 and Emax1023. Rewriting its
fractional significand as an integer gives exponent limits−1074..971; this
conflicts with XSD's printed−1075..970. IEEE §4.1 supplies the conditional
nearest-mode overflow threshold below. Clinger §8 requires the bounded system's
policy; retaining that report alone does not settle XSD's infinity mapping.

For a rational `2^-k`, “exact decimal” means the unique terminating decimal
`5^k × 10^-k`; this defines an exact fixture spelling without a host float
formatting oracle. Facet/equality observations can distinguish rounded values
when an API exposes no typed numeric result.

| ID | Concrete witness and isolated mutation | Expected result, class, observation |
| --- | --- | --- |
| XR-110 | Float values16777216,16777217,16777218; double values9007199254740992,9007199254740993,9007199254740994. | First/last of each triple are representable; middle is halfway and rounds to the even first value. Enumeration of the first admits the middle spelling; enumeration of the last does not. This distinguishes24-bit/53-bit precision from arbitrary decimal integers. V. |
| XR-111 | Float exact decimal `1+2^-24` and values differing by `±2^-50`; double exact `1+2^-53` and values differing by `±2^-100`. | Midpoints round to1 by even choice; below to1; above to adjacent larger representable value. Compare exact rationally generated spellings, not an intermediate host double rounded before the float test. V. |
| XR-112 | Float exact2^-149, half2^-150, and values immediately below/above the half obtained by subtracting/adding exact2^-200. | Source-defined smallest positive float is2^-149; halfway ties to zero, below to zero, above to2^-149. Both signs map the zero result to XSD's single zero. This is a value-mapping witness, not a requirement to preserve an IEEE negative-zero bit. V. |
| XR-113 | Float/double literals 0,-0,+0,INF,-INF,NaN; mutate to +INF,inf,nan. | XSD has one zero and one NaN; its equality says NaN equals itself. Exact special spellings admitted, altered spellings rejected. Do not use IEEE NaN inequality, NaN payloads, signed-zero identity, or floating exception flags as the XSD equality oracle. V. |
| XR-114 | Float enumeration NaN and value NaN; bound minInclusive0 with NaN; bound minInclusiveNaN versus minExclusiveNaN. | Enumeration matches the single NaN. Ordinary bound excludes incomparable NaN. Inclusive NaN bound admits only NaN, exclusive NaN bound yields empty value space. XSD explicitly defines these effects despite differing computational IEEE comparisons. V. |
| XR-115 | Exact double2^-1075,2^-1074, `(2^53-1)×2^970`, and `(2^53-1)×2^971`; compare publication's stated exponent interval with conventional binary64 description. | Retained §3.2.5 states -1075..970; IEEE Table1/§3.2.2 give the contrasting integer-exponent interval -1074..971. This is a source conflict, not a missing-text limit or approved XSD correction. Extremal XSD oracles remain OPEN/source-qualified; midrange XR-110/111 is unaffected. V/OPEN. |
| XR-116 | Exact float maximum `F=(2^24-1)×2^104`; threshold `T=2^128−2^103`; exact integers T−1,T,T+1, their negatives, and 1E10000. | Under IEEE nearest rounding with default non-trapping overflow, F≤abs(d)<T maps to same-sign F and abs(d)≥T to same-sign infinity. This conditional rule exposes the boundary; it is not an unqualified XSD oracle. Clinger §8 leaves overflow policy to the bounded system; the XSD nearest-value/infinity interpretation remains OPEN. F itself is representable under XSD. V/OPEN. |
| XR-117 | Decimal123456789012345678 versus1234567890123456789; compare both with float/double approximation. | Minimum decimal support is18 decimal digits; above a documented supported precision limit can be CAP failure, not proof the lexical/value space excludes the number. Decimal has no float rounding merely because the host uses binary64. Distinguish documented limit, range/facet failure, and nearest-value mapping. V/CAP. |

## Other cited material and remaining source limits

| ID | Concrete applicability witness | Expected result, class, observation |
| --- | --- | --- |
| XR-118 | A requirements-document goal mentions a capability; construct a schema needing it, but no adopted Structures/Datatypes rule admits the proposed syntax. | A requirements Note is not an alternate grammar or an instruction to accept unadopted syntax. Trace any required acceptance to the adopted XSD clause. S/OPEN. |
| XR-119 | Use a fragment `#xpointer(xs:schema/xs:element[@name="person"])` to discuss a component's XML representation; compare explicit component selection already supplied by the application. | Structures' component-reference discussion notes a possible XPointer correspondence; it does not mandate arbitrary XPointer evaluation, network retrieval, or a schema-component-designator implementation. It does not add full XPath predicates to identity selector syntax. R/S, CHOICE/PRODUCT. |

The captured dependency set supports the witnesses' cited rules, not unlimited
transitive closure. In particular:

- RFC2732's detailed IPv6 grammar delegates to retained RFC2373; its prose/ABNF
  disagreements remain explicit rather than patched with rejected Erratum456. XLink/XML
  reference UTF-8 standards and several other transport definitions not fully
  mirrored here. XML's own legal-character/encoding constraints still apply.
- UnicodeData and Blocks support category/block decisions, including range
  markers and duplicate names. They do not replace the full Unicode3.1
  Standard, normalization/collation specifications, every auxiliary UCD file,
  or every later Unicode database. XSD string equality does not automatically
  normalize canonically equivalent Unicode strings.
- RFC1766/3066 give syntax and assignment mechanisms, not a retained historical
  ISO639/3166/IANA registry snapshot for every possible tag. RFC3066 obsoletes
  1766; newer BCP47/registry changes are not retroactive XSD lexical rewrites.
- ISO8601 is in XSD's non-normative bibliography. Its modern revisions cannot
  override the XSD1.0 year-zero prohibition or lexical date/time definitions.
  The printed double interval and overflow-policy interpretation remain material
  source questions after acquisition; see XR-115/116.
- Relative namespaces, Unicode version timing/old displayed block table,
  language registry semantics, and undated XML namespace helper changes need
  source-qualified comparison outcomes. None is resolved by an existing test
  expectation or a newer library's behavior.

No fixtures were executed, no corpus match was asserted, and no external
permission/retrieval capability was inferred from a syntactically valid input.
