package value

import (
	"strings"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

// TypeID indexes a simple type in a Program. IDs are stable within one
// program; they are not valid across programs.
type TypeID uint32

// NoType is the absent simple-type sentinel.
const NoType TypeID = ^TypeID(0)

// PrimitiveKind identifies one of the XSD 1.0 primitive datatype families.
type PrimitiveKind uint8

const (
	// PrimitiveString is the xs:string primitive value space.
	PrimitiveString PrimitiveKind = iota
	// PrimitiveBoolean is the xs:boolean primitive value space.
	PrimitiveBoolean
	// PrimitiveDecimal is the xs:decimal primitive value space.
	PrimitiveDecimal
	// PrimitiveFloat is the xs:float primitive value space.
	PrimitiveFloat
	// PrimitiveDouble is the xs:double primitive value space.
	PrimitiveDouble
	// PrimitiveDuration is the xs:duration primitive value space.
	PrimitiveDuration
	// PrimitiveDateTime is the xs:dateTime primitive value space.
	PrimitiveDateTime
	// primitiveTime is the xs:time primitive value space.
	primitiveTime
	// primitiveDate is the xs:date primitive value space.
	primitiveDate
	// primitiveGYearMonth is the xs:gYearMonth primitive value space.
	primitiveGYearMonth
	// primitiveGYear is the xs:gYear primitive value space.
	primitiveGYear
	// primitiveGMonthDay is the xs:gMonthDay primitive value space.
	primitiveGMonthDay
	// primitiveGDay is the xs:gDay primitive value space.
	primitiveGDay
	// primitiveGMonth is the xs:gMonth primitive value space.
	primitiveGMonth
	// PrimitiveHexBinary is the xs:hexBinary primitive value space.
	PrimitiveHexBinary
	// PrimitiveBase64Binary is the xs:base64Binary primitive value space.
	PrimitiveBase64Binary
	// PrimitiveAnyURI is the xs:anyURI primitive value space.
	PrimitiveAnyURI
	// PrimitiveQName is the xs:QName primitive value space.
	PrimitiveQName
	// PrimitiveNotation is the xs:NOTATION primitive value space.
	PrimitiveNotation
)

// validPrimitiveKind reports whether kind names a supported primitive family.
func validPrimitiveKind(kind PrimitiveKind) bool {
	return kind <= PrimitiveNotation
}

// Variety identifies the shape of a simple type.
type Variety uint8

const (
	// Atomic identifies a simple type with one primitive value.
	Atomic Variety = iota
	// List identifies a whitespace-separated sequence of item values.
	List
	// Union identifies a choice among member value spaces.
	Union
)

// WhitespaceMode is the XSD whiteSpace facet mode.
type WhitespaceMode uint8

const (
	// WhitespacePreserve retains XML whitespace during lexical normalization.
	WhitespacePreserve WhitespaceMode = iota
	// WhitespaceReplace maps XML whitespace characters to spaces.
	WhitespaceReplace
	// WhitespaceCollapse trims and coalesces XML whitespace runs.
	WhitespaceCollapse
)

func (m WhitespaceMode) valid() bool { return m <= WhitespaceCollapse }

// BuiltinKind identifies lexical rules attached to XSD-derived built-ins.
type BuiltinKind uint8

const (
	// BuiltinNone indicates that no derived builtin lexical rule applies.
	BuiltinNone BuiltinKind = iota
	// BuiltinInteger applies integer-family lexical rules.
	BuiltinInteger
	// builtinNameLexical applies xs:Name lexical rules.
	builtinNameLexical
	// builtinNCNameLexical applies xs:NCName lexical rules.
	builtinNCNameLexical
	// builtinNMTOKENLexical applies xs:NMTOKEN lexical rules.
	builtinNMTOKENLexical
	// builtinLanguageLexical applies xs:language lexical rules.
	builtinLanguageLexical
	// builtinEntity applies xs:ENTITY lexical rules.
	builtinEntity
)

// FacetMask records the facet families present on a type.
type FacetMask uint16

const (
	// FacetLength identifies the exact-length constraining facet.
	FacetLength FacetMask = 1 << iota
	// FacetMinLength identifies the minimum-length constraining facet.
	FacetMinLength
	// FacetMaxLength identifies the maximum-length constraining facet.
	FacetMaxLength
	// FacetTotalDigits identifies the totalDigits constraining facet.
	FacetTotalDigits
	// FacetFractionDigits identifies the fractionDigits constraining facet.
	FacetFractionDigits
	// FacetMinInclusive identifies the inclusive lower-bound facet.
	FacetMinInclusive
	// FacetMaxInclusive identifies the inclusive upper-bound facet.
	FacetMaxInclusive
	// FacetMinExclusive identifies the exclusive lower-bound facet.
	FacetMinExclusive
	// FacetMaxExclusive identifies the exclusive upper-bound facet.
	FacetMaxExclusive
	// FacetEnumeration identifies the enumeration constraining facet.
	FacetEnumeration
	// FacetPattern identifies the pattern constraining facet.
	FacetPattern
	// FacetWhiteSpace identifies the whiteSpace constraining facet.
	FacetWhiteSpace
)

const orderedFacetMask = FacetMinInclusive | FacetMaxInclusive | FacetMinExclusive | FacetMaxExclusive
const lengthFacetMask = FacetLength | FacetMinLength | FacetMaxLength

// Needs selects optional projections in a validation result.
type Needs uint8

const (
	// NeedCanonical requests the canonical lexical projection in a validation result.
	NeedCanonical Needs = 1 << iota
	// NeedIdentity requests the value-space identity projection.
	NeedIdentity
)

// Has reports whether n includes every bit in want.
func (n Needs) Has(want Needs) bool { return n&want != 0 }

// primitiveValueNeed reports which projections a primitive parser must
// retain for the enclosing value evaluation.
type primitiveValueNeed uint8

const (
	// primitiveNeedCanonical asks a primitive parser to retain canonical text.
	primitiveNeedCanonical primitiveValueNeed = 1 << iota
	// primitiveNeedLength asks a primitive parser to retain its length.
	primitiveNeedLength
	// primitiveNeedIdentity asks a primitive parser to retain identity data.
	primitiveNeedIdentity
)

// Has reports whether n includes every primitive projection in want.
func (n primitiveValueNeed) Has(want primitiveValueNeed) bool { return n&want != 0 }

// ExpandedName is the namespace URI and local-name pair returned by QName
// resolution. It is also the value-space payload retained for QName and
// NOTATION atoms.
type ExpandedName struct {
	Namespace string
	Local     string
}

// QNameResolver resolves one lexical QName into an expanded name. The boolean
// reports whether the lexical value was resolved in the active namespace
// context.
type QNameResolver func(string) (ExpandedName, bool)

// Resolver supplies the context-sensitive QName and NOTATION decisions that
// value validation cannot own. A nil function means no namespace context.
type Resolver struct {
	QName    QNameResolver
	Notation func(namespace, local string) bool
}

// Scratch is caller-owned reusable matcher state. It may be reused
// sequentially, but not concurrently, across Validate calls.
type Scratch struct {
	pattern xsdregex.Scratch
}

// PatternAccount is caller-owned pattern work shared by many top-level
// evaluations. Program.Validate mutates it through the pointer in
// EvaluationLimits; one account must not be used by concurrent evaluations,
// and it must be passed by pointer from its owner at each call, never
// snapshotted. A zero account has a zero limit and makes evaluation return
// ErrMetadata.
type PatternAccount struct{ limit, used uint64 }

// Reset sets the account's limit and clears its usage.
func (a *PatternAccount) Reset(limit uint64) {
	a.limit, a.used = limit, 0
}

// Limit returns the account's limit; a nil account has none.
func (a *PatternAccount) Limit() uint64 {
	if a == nil {
		return 0
	}
	return a.limit
}

// EvaluationLimits bound one top-level value evaluation. Every field must be
// positive and PatternTotal must be non-nil with a positive limit; otherwise
// evaluation returns ErrMetadata, even for programs without patterns.
//
// Work bounds type-graph traversal: each visited lexical value costs its byte
// length plus one. PatternWork bounds regular-expression matching across all
// pattern facets, list items and union attempts of the evaluation, in the
// units documented on xsdregex.MatchOptions. Exhausting it yields
// ErrPatternLimit. PatternTotal additionally bounds pattern work summed over
// every evaluation that shares the account; exhausting it yields
// ErrPatternTotalLimit.
type EvaluationLimits struct {
	PatternTotal *PatternAccount
	Work         uint64
	PatternWork  uint64
}

// Reset clears reusable matcher storage while retaining only bounded
// capacity.
func (s *Scratch) Reset(maxRetainedRunes int) {
	if s == nil {
		return
	}
	s.pattern.Reset(maxRetainedRunes)
}

// orderedFacetRelation compares one value against another. Incomparable is
// required for the partial orders of duration and timezone-absent temporals.
type orderedFacetRelation uint8

const (
	// orderedFacetLess means the left value is below the right value.
	orderedFacetLess orderedFacetRelation = iota
	// orderedFacetEqual means the values are equal in the ordered value space.
	orderedFacetEqual
	// orderedFacetGreater means the left value is above the right value.
	orderedFacetGreater
	// orderedFacetIncomparable means the primitive's order is partial.
	orderedFacetIncomparable
)

func orderedFacetRelationFromInt(n int) orderedFacetRelation {
	switch {
	case n < 0:
		return orderedFacetLess
	case n > 0:
		return orderedFacetGreater
	default:
		return orderedFacetEqual
	}
}

func (r orderedFacetRelation) valid() bool { return r <= orderedFacetIncomparable }

// compareFraction is the single fraction order for decimal, temporal and
// duration digits. Fractions are normalized without trailing zeroes, and empty
// means zero, so lexical order agrees with order after padding the shorter
// fraction with zeroes.
func compareFraction(a, b string) int {
	return strings.Compare(a, b)
}

// byteText lets lexical validators accept strings and borrowed byte slices.
type byteText interface {
	~string | ~[]byte
}

// orderedFacetLowerBoundAccepts reports whether relation satisfies an admitted
// lower bound with the given exclusivity.
func orderedFacetLowerBoundAccepts(exclusive bool, relation orderedFacetRelation) bool {
	if !relation.valid() {
		return false
	}
	return relation == orderedFacetGreater || relation == orderedFacetEqual && !exclusive
}

// orderedFacetUpperBoundAccepts reports whether relation satisfies an admitted
// upper bound with the given exclusivity.
func orderedFacetUpperBoundAccepts(exclusive bool, relation orderedFacetRelation) bool {
	if !relation.valid() {
		return false
	}
	return relation == orderedFacetLess || relation == orderedFacetEqual && !exclusive
}

// orderedFacetLowerRestricts reports whether an admitted derived lower bound
// narrows an admitted base lower bound.
func orderedFacetLowerRestricts(derivedExclusive, baseExclusive bool, relation orderedFacetRelation) bool {
	if !relation.valid() {
		return false
	}
	return relation == orderedFacetGreater || relation == orderedFacetEqual && (derivedExclusive || !baseExclusive)
}

// orderedFacetUpperRestricts reports whether an admitted derived upper bound
// narrows an admitted base upper bound.
func orderedFacetUpperRestricts(derivedExclusive, baseExclusive bool, relation orderedFacetRelation) bool {
	if !relation.valid() {
		return false
	}
	return relation == orderedFacetLess || relation == orderedFacetEqual && (derivedExclusive || !baseExclusive)
}

// facetCardinalityValue is an optional unsigned facet value.
type facetCardinalityValue struct {
	Value   uint32
	Present bool
}
