# Interpretation records separate from approved corrections

The [approved correction layer](corrected-reading.md) applies published XSD
1.0 errata. The records here explain source questions that arise during
scenario construction. Their snapshots have role
`interpretation-history-not-erratum` in [the source manifest](source-manifest.json).
A closed issue is not, by itself, a new normative XSD 1.0 amendment.

## I-001 — complex types with simple content as identity fields

The published [identity rule](upstream/structures.html#cvc-identity-constraint)
says the selected field node has a simple type; nearby prose speaks of its
content/value being simple. [W3C issue 10236](upstream/issue-10236.html)
records the resulting ambiguity and the WG's earlier conclusion that simple
content of a complex type was intended to participate. The issue is marked
for both 1.0 and 1.1; its 2010 resolution adds a clarification entry to the
1.1 change list, rather than publishing an additional Second Edition erratum.

Concrete pair: an integer `v` field and a `v` with complex simple content
extending integer, both containing `1`. The documented WG interpretation
admits both as value-bearing fields. An element-only complex `v` containing
an `x` child is a distinct nonsimple negative. Do not score a literal
1.0-text rejection of the simple-content case as a demonstrated normative
defect without preserving this interpretation boundary. Record the intended
valid result as **WG clarification**, separate from an unambiguous published
1.0 requirement. See [C-005](scenarios/branches/composition-and-identity.md#c-005--simple-values-and-field-ordering).

## I-002 — fields inside a skipped subtree

[Issue 1937](upstream/issue-1937.html) is explicitly version **1.1 only**.
Its adopted resolution treats the skipped subtree as absent for identity
processing. That version tag matters: it cannot silently add an XSD 1.0
exception to the published selector, node-cardinality, and simple-value
conditions.

Concrete pair: a selector or field reaches `v` beneath a `processContents=skip`
element, versus the same `v` assessed through a declaration. Preserve the
actual typed-value/assessment facts for each. A skipped node is not a
type-valid empty string. Keep the XSD 1.0 consequence source-qualified where
the necessary value is unavailable; do not use the 1.1 skip exemption as
proof of a mandatory 1.0 acceptance result.

## I-003 — nilled field values

[Issue 2219](upstream/issue-2219.html) asks whether nilled selected elements
count as missing values. Its disposition explicitly describes the accepted
**1.1** behavior: nil acts as a missing field for `unique`, whereas a key
still needs field values. [Issue 10236](upstream/issue-10236.html) relates
that history to the later identity-rule revisions. Neither record is listed
as a new approved 1.0 Second Edition erratum.

Concrete distinctions: absent `v`; nonnil string `<v/>`; and string `v`
with `xsi:nil=true`. XPath can select the nilled **node**, even though E1-56
gives it no schema-normalized value. Calling that selection an empty node
set conflates two different predicates. [C-009](scenarios/branches/composition-and-identity.md#c-009--nilled-and-invalid-field-values-are-not-lexical-empty-strings)
keeps the 1.0 qualified-value interpretation visible. Independently, a key
field assessed through a nillable declaration fails the published key
condition even if its instance is not nilled.

## I-004 — published member-minimum rule

**Status:** published XSD 1.0 oracle resolved; acknowledged bug and proposed
repair remain interpretation history.

[NSRecurseCheckCardinality](upstream/structures.html#rcase-NSRecurseCheckCardinality)
requires both recursive restriction of every immediate member against the
original wildcard particle and restriction of the group's effective total
range. For an element member, [NSCompat](upstream/structures.html#rcase-NSCompat)
invokes [Occurrence Range OK](upstream/structures.html#range-ok), including
the original base minimum. Passing the aggregate check does not waive that
member check. The rule does not replace the base minimum with zero.

[R-240](upstream/rec-comments.html#pfiNSRecurseCheckCardinality) identifies
this consequence and proposes such a replacement. Thompson's
[6 January 2004 reply](upstream/nsrecurse-reply-2004.html) confirms the
particle reading and acknowledges a bug. This is evidence of intended repair,
not published replacement text. The retained Second Edition still prints both
checks; the [approved errata](upstream/errata-second-edition.html) do not
amend this rule.

For this repository's published-1.0 oracle, a member minimum below the base
minimum fails even when the aggregate fits. An all group with required a and
optional b against wildcard1..2 is a minimal distinction: total1..2 fits,
but b0..1 does not. Sequence and repeated-choice consumers must likewise
separate member and aggregate bounds. Ha070, Ha080 and R013 retain their
upstream valid provenance in `schema.oracle.w3c`; their effective schema
expectation is invalid under this baseline. R013's retained instance supplies
no validation credit under the invalid-schema expectation.

Report acceptance as divergence from the published rule, qualified by the
acknowledged bug and interoperability history. Do not present acceptance as
proof of an approved repair, or literal rejection as contrary to published
XSD 1.0. This interpretation record adds no erratum.

## I-005 — implicit substitution-choice order

**Status:** underspecified in published XSD 1.0; proposed order waiver is not
an approved amendment.

[Particle restriction 2.1](upstream/structures.html#cos-particle-restrict)
constructs a choice from an actual substitution-group set without assigning
its list order. [RecurseLax](upstream/structures.html#rcase-RecurseLax)
requires an ordered mapping. [R-122](upstream/rec-comments.html#pfiSubGrpRestrict)
records this problem and an instruction to draft an erratum; Thompson's
[14 February 2002 reply](upstream/substitution-order-reply-2002.html)
proposes waiving order for implicit choices. The register's drafting status
is distinct from an approved correction. Neither the retained Second Edition
nor its approved errata supplies that waiver or a canonical ordering.

Do not infer declaration order, QName sorting, a shared generated order, or
existential reordering from processor behavior. Order-sensitive implicit-choice
cases remain without a unique published-1.0 expectation. Explicit source
choices still have their printed order and ordinary RecurseLax requirements.

Source-supported executable witnesses can avoid the missing premise.
[MapAndSum](upstream/structures.html#rcase-MapAndSum) does not require an
ordered target mapping, so either explicit sequence order can restrict a
matching repeated generated base choice. A unit generated singleton also
normalizes without an ordering decision. For such witnesses, prove the same
outcome for every relevant enumeration. This is a method of choosing safe
examples, not a new universal-order validity rule or a required processor
algorithm. Actual membership, filtering, ranges and other derivation rules
still apply.

## Applying these records in the later corpus audit

Required 1.0 result, WG interpretation, 1.1 change, processor choice,
unspecified behavior, and unavailable observation are distinct evidence
classes. A fixture can demonstrate conformance to the documented intended
interpretation without proving that the unamended 1.0 wording uniquely
requires it. Corpus deviations involving these seams must cite both the
published rule and this source classification. This preserves the question
without choosing the implementation or corpus itself as the oracle.
