# Primer features and assessment boundaries

The [Second Edition Primer](../upstream/primer.html) is non-normative. Its
examples reveal useful combinations to exercise, while Part 1/2 and the
[approved correction](../corrected-reading.md) supply the oracle. This index
accounts for the Primer's body and every appendix without promoting example
choices to requirements.

## Primer feature inventory

For each P-family, identify the intended Primer example, check its literal
XML/schema/component premises, then vary each relevant premise against the
linked normative family. A printed illustration or P link alone establishes
neither an admitted fixture nor corpus coverage. Preserve an early source
failure or missing premise; label any repaired counterpart as a separately
authored transformation. Parent sections introduce the child topics below;
the abstract/status/contents introduce no additional language construct.

| ID | Primer source | Questions and expected semantic boundary |
| --- | --- | --- |
| P-001 | [§§1–2.1](../upstream/primer.html#Intro) | Purchase-order schema versus instance; declaration versus type definition; schema namespace versus target namespace. Change each expanded name independently and assess using the supplied component set. |
| P-002 | [§2.2](../upstream/primer.html#DefnDeclars) | Named/anonymous types, element/attribute declarations, default/fixed values. Repeat with absent, empty, explicit equal and explicit unequal values; use typed equality and the E1-56 PSVI reading. |
| P-003 | [§2.2.1](../upstream/primer.html#OccurrenceConstraints) | Missing/defaulted and explicit occurrences; optional/required/prohibited attributes; zero/finite/unbounded particles. Check each bound and distinguish absent element from empty element. |
| P-004 | [§§2.2.2–2.2.3](../upstream/primer.html#Globals) | Global/local visibility, references, equal names in distinct symbol spaces, duplicate declarations within one space, same local name in distinct scopes. Resolution depends on both expanded name and component kind. |
| P-005 | [§§2.3–2.3.2](../upstream/primer.html#CreatDt) | Builtins/restriction/list/union; empty lists, inherited facets, member order and overlapping members. Apply [datatype families](datatypes.md), rather than treating example values as a complete lexical space. |
| P-006 | [§2.4](../upstream/primer.html#InlineTypDefn) | Anonymous versus named definitions; illegal name/reference combinations and inaccessible anonymous components. Similar content does not make two anonymous declarations the same named component. |
| P-007 | [§§2.5–2.5.4](../upstream/primer.html#typeContent) | Simple content plus attributes, mixed/empty/element-only content, `anyType`. Distinguish no text, XML whitespace, ordinary text, and child elements at each content kind. |
| P-008 | [§2.6](../upstream/primer.html#CommVers) | Documentation/appinfo, foreign attributes and payload, optional IDs/source/language. Payload is annotation content, not a new schema declaration. Version strings do not select a new XSD language automatically. |
| P-009 | [§§2.7–2.8](../upstream/primer.html#groups) | Sequence/choice/all, named groups and attribute groups. Exercise order, multiplicity, nested optionality, cycles, duplicate effective uses, and wildcard composition under the [Structures rules](structures.md). |
| P-010 | [§2.9](../upstream/primer.html#Nils) | Nillability versus optionality, absent/false/true nil, attributes on nilled elements, and forbidden child/text content. Nilled and missing do not have the same validity or defaulting effects. |
| P-011 | [§§3–3.4](../upstream/primer.html#NS) | Target namespace, form defaults, local overrides, global references, and no-target-namespace schemas. Test prefix renaming, explicit/default namespace changes, and unqualified attributes independently. IP-013's [imported-base companion](interactions.md) joins the two source form defaults. |
| P-012 | [§§4–4.1](../upstream/primer.html#IPO) | A schema spread across documents: include versus import, target-namespace agreement, chameleon inclusion, duplicate components, resolution failure and cycles. The abstract schema is a component set, not a filename. |
| P-013 | [§§4.2–4.4](../upstream/primer.html#DerivExt) | Extension/restriction and instance `xsi:type`: valid/invalid derivation, inherited attributes/content, blocked derivation, nil/default interactions and a type name bound to the wrong namespace. |
| P-014 | [§4.5](../upstream/primer.html#Redefine) | Redefining simple/complex types and model/attribute groups; legal self-reference, absent original, wrong kind, and invalid restriction. Redefinition has its own composition rules. |
| P-015 | [§§4.6–4.8](../upstream/primer.html#SubsGroups) | Substitution chains, abstract heads/members/types, `block` versus `final`, and derived-type use. Exercise transitive derivation and each prohibited method rather than only direct membership. |
| P-016 | [§§5–5.3](../upstream/primer.html#quartelyReport) | `unique`/`key`/`keyref`, missing/empty/nilled fields, composite tuples, local selector scope, typed equality, ID/IDREF document scope. Their scope and missing-field rules differ. |
| P-017 | [§§5.4–5.4.1](../upstream/primer.html#import) | Importing foreign/no-namespace components and using type libraries. Import permission, location hints, available components and namespace identity are separate decisions. IP-013's [imported-base companion](interactions.md) preserves inherited local names in their defining source context. |
| P-018 | [§5.5](../upstream/primer.html#any) | Element/attribute wildcards: each namespace-set form, process-contents mode, declaration available/unavailable, and present invalid content. `lax` does not skip validation when a declaration is available. |
| P-019 | [§§5.6–5.7](../upstream/primer.html#schemaLocation) | Instance hints and processor conformance. Hints do not guarantee acquisition; an explicit-source processor need not fetch them. Preserve assessment/validity/PSVI distinctions and processor options. |
| P-020 | [Appendix A](../upstream/primer.html#Acks) | Acknowledgements add no executable constraint. Retain provenance, with no invented acceptance rule. |
| P-021 | [Appendix B](../upstream/primer.html#SimpleTypeFacets) | Builtin/facet summary: cross-check every builtin and applicable facet with Part 2. An example/table omission cannot remove a normative datatype constraint. |
| P-022 | [Appendix C](../upstream/primer.html#usingEntities) | Entity declarations/references and the resulting XML infoset. Distinguish parsed/unparsed entities, ENTITY/ENTITIES validation and external entity access policy. IP-001's [character-element companion](interactions.md) separates entity replacement from child markup, parent content admission and child fixed-value PSVI. |
| P-023 | [Appendix D](../upstream/primer.html#regexAppendix) | Regex examples and their positive/negative neighbors; use Part 2 Appendix F for the grammar, Unicode rules, and matching semantics. |
| P-024 | [Appendix E, E.1/E.2](../upstream/primer.html#index) | Element and attribute index: cross-check all syntax vocabulary against both Appendix A grammars. The index supplies navigation, not another schema grammar. |

## Printed examples and prerequisites

Keep the unchanged printed source distinct from a corrected or completed
example. The following source classes qualify the expansion method above;
their governing rules already belong to the linked catalogues.

| Printed source class | Required disposition |
| --- | --- |
| Malformed comment closing tags in po.xml/po1.xml and qualified/defaulted/ipo variants | The literal fails XML admission before XSD. A legal closing tag in an authored guide is a separately corrected example; it does not certify the printed bytes. |
| `shipAndBill` group and `ItemDelivery` attribute group use `id` without `name` | A schema-document ID does not supply a named component's name. Compare a separately authored name-correct form under GC-007/GC-012 in [grammar contexts](grammar-contexts.md). |
| Ellipses, unshown `UKPostcode`, parent/namespace/import context or `xml:lang` declaration source | State the missing context/component/source premise before assigning validity. Do not fetch illustrative example.com schemas or silently supply a declaration. |
| Two substitution members in the original optional `comment` slot with maximum one | Each can substitute individually. Admitting two requires an independently admitted repeated particle; affiliation does not alter cardinality. |
| Abstract-head prose suggests both members must occur | The abstract head itself is forbidden. A required slot needs an admitted concrete substitute; an optional slot can be absent. Abstractness does not require both members. |
| `anyType` called unconstrained; strict wildcard told to obtain a schema; simple type said to allow no attributes | Preserve B-119, IP-015 and A-069's actual assessment/type/attribute conditions. These are explanatory shortcuts, without new acceptance or acquisition rules. |
| Blanket fixed-facet or enumeration-uniqueness wording | Use D2-008/D2-029's admitted facet contexts and typed-value rules in [datatype cases](branches/datatypes.md). |
| Include/import ordering, identical-field-type wording or Appendix D regex shortcuts | Retain the existing composition, formal typed-equality and Part 2 regex owners; the Primer supplies no alternative oracle. |

## External admission and processor questions

These X-families identify the trust boundaries that can otherwise mask XSD
scenarios. The source inventory retains the cited editions of XML,
Namespaces, Infoset, XPath, XML Base, XLink, URI RFCs, and Unicode. This is
an admission matrix; it does not claim to reproduce every rule of those
standards or their transitive references.

| ID | Source owner | Required partitions and oracle |
| --- | --- | --- |
| X-001 | [XML 1.0 §§2.1–2.4](../dependencies/xml-1.0-second-edition.html) | One document element versus zero/many; balanced tags; legal versus forbidden characters; markup escaping; comments/PI/CDATA boundaries. XML failure precedes XSD validity. |
| X-002 | [XML 1.0 §§2.8, 4.3.3](../dependencies/xml-1.0-second-edition.html) | Declaration absent/present, legal version, encoding declaration, byte order mark and actual encoding. The repository's UTF-8/XML 1.0 input contract is a product boundary; XML 1.1 is not added by a non-normative interoperability Note. |
| X-003 | [XML 1.0 §§2.11, 3.3.3](../dependencies/xml-1.0-second-edition.html) | Literal CR/CRLF/LF, character references to whitespace, and attribute normalization. Compare inputs yielding the same infoset and inputs that only look alike before normalization. Apply datatype whiteSpace afterward. |
| X-004 | [XML 1.0 §4](../dependencies/xml-1.0-second-edition.html) | Predefined/numeric/internal/external entities, missing entity declaration, parsed/unparsed distinction, recursion and replacement boundaries. A chosen source/DTD policy must not be mistaken for an XSD lexical rule. |
| X-005 | [Namespaces §§2–6](../dependencies/namespaces-1999.html) | Prefix bound/unbound/rebound, default namespace present/absent, reserved `xml`/`xmlns`, empty default declaration, illegal QName shape, and duplicate expanded attribute names. Different prefixes can name the same component. |
| X-006 | [Namespaces §6.2](../dependencies/namespaces-1999.html) | Default namespace on element names versus unprefixed attribute names. Separately exercise QName-valued **content**, whose resolution follows the XSD datatype/schema rule and its in-scope context. |
| X-007 | [Infoset](../dependencies/infoset-20010316.html), [Structures Appendix D](../upstream/structures.html#infoset) | Required element/attribute/character/namespace facts present, comments/PI ignored as appropriate, and unparsed-entity facts unavailable. ENTITY/ENTITIES cannot validate merely because their strings are NCNames. |
| X-008 | [XML Base](../dependencies/xml-base-2001.html) | Relative references under inherited/local base, empty base/reference, absolute reference overriding base, dot segments and fragment handling. Distinguish source identity, resolver input and namespace identity. |
| X-009 | [RFC 2396](../dependencies/rfc2396.txt), [RFC 2732](../dependencies/rfc2732.txt), [XLink §5.4](../dependencies/xlink-1.0.html) | Raw and escaped URI references, non-ASCII characters, percent escapes, authority/path/query/fragment presence, IPv6 brackets and malformed separators. Apply XSD's cited escaping procedure before the cited URI grammar where specified; do not import RFC 3986 automatically. |
| X-010 | [XPath 1.0](../dependencies/xpath-1.0.html), [Structures selector/field rules](../upstream/structures.html#c-selector-xpath) | Every admitted selector/field production, namespace-bound QName steps, wildcard steps, descendant abbreviation and union branches. Reject full-XPath features outside the XSD subset; prefix scope is that of the schema XPath expression. |
| X-011 | [Unicode 3.1](../dependencies/unicode-3.1.html), [Part 2 Appendix F](../upstream/datatypes.html#regexs) | Unicode categories, blocks, XML name escapes and admitted character ranges. Record the edition/allowed-version choice before judging newly assigned characters. Unicode property classification and XML character/name admission are separate predicates. |
| X-012 | [Structures §§2.4, 4.3](../upstream/structures.html#concepts-conformance) | Schema component input versus XML schema-document input; optional discovery; locations missing or unavailable; strict/lax/skip paths. Preserve permitted processor choices rather than inventing one required fetch policy. |
| X-013 | [Structures §§5.1–5.4](../upstream/structures.html#conformance) | Invalid schema, missing subcomponent, unknown declaration, local invalidity, assessment not attempted and operational failure. They must retain distinct catalogue classifications even if one API exposes fewer results. |
| X-014 | [Structures §§3.2.5, 3.3.5](../upstream/structures.html#sic-a-outcome), [E1-56](../corrected-reading.md) | Validity-only observation versus full PSVI observation. Record unavailable properties as not observable; do not infer PSVI correctness from a boolean validation result. |
| X-015 | [Structures §2.4](../upstream/structures.html#concepts-conformance), [Part 2 §5](../upstream/datatypes.html#conformance) | Precision, retained state, recursion and input/work limits. Distinguish required minimum capability, recommended capability, allowed implementation choice and finite product resource failure. A limit rejection alone is not evidence that the schema/document violates XSD. |

## Questions to carry into the comparison

Which product boundary observes each result? Which options does the standard
permit? Which XML and Unicode versions supply external facts? Did the parser
actually construct the infoset assumed by the XSD scenario? Is failure due
to the target rule, an earlier XML/schema error, unavailable source data, or
a resource bound? A fixture cannot answer a downstream question when its
premises never became true.
