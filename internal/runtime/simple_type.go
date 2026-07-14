package runtime

import (
	"errors"
	"slices"
	"strconv"
	"strings"
)

// SimpleVariety identifies the runtime simple-type variety.
type SimpleVariety uint8

const (
	// SimpleVarietyAtomic is an atomic simple type.
	SimpleVarietyAtomic SimpleVariety = iota
	// SimpleVarietyList is a list simple type.
	SimpleVarietyList
	// SimpleVarietyUnion is a union simple type.
	SimpleVarietyUnion
)

// PrimitiveKind identifies the primitive datatype family of a simple type.
type PrimitiveKind uint8

const (
	// PrimitiveString is the xs:string primitive family.
	PrimitiveString PrimitiveKind = iota
	// PrimitiveBoolean is the xs:boolean primitive family.
	PrimitiveBoolean
	// PrimitiveDecimal is the xs:decimal primitive family.
	PrimitiveDecimal
	// PrimitiveFloat is the xs:float primitive family.
	PrimitiveFloat
	// PrimitiveDouble is the xs:double primitive family.
	PrimitiveDouble
	// PrimitiveDuration is the xs:duration primitive family.
	PrimitiveDuration
	// PrimitiveDateTime is the xs:dateTime primitive family.
	PrimitiveDateTime
	// PrimitiveTime is the xs:time primitive family.
	PrimitiveTime
	// PrimitiveDate is the xs:date primitive family.
	PrimitiveDate
	// PrimitiveGYearMonth is the xs:gYearMonth primitive family.
	PrimitiveGYearMonth
	// PrimitiveGYear is the xs:gYear primitive family.
	PrimitiveGYear
	// PrimitiveGMonthDay is the xs:gMonthDay primitive family.
	PrimitiveGMonthDay
	// PrimitiveGDay is the xs:gDay primitive family.
	PrimitiveGDay
	// PrimitiveGMonth is the xs:gMonth primitive family.
	PrimitiveGMonth
	// PrimitiveHexBinary is the xs:hexBinary primitive family.
	PrimitiveHexBinary
	// PrimitiveBase64Binary is the xs:base64Binary primitive family.
	PrimitiveBase64Binary
	// PrimitiveAnyURI is the xs:anyURI primitive family.
	PrimitiveAnyURI
	// PrimitiveQName is the xs:QName primitive family.
	PrimitiveQName
	// PrimitiveNotation is the xs:NOTATION primitive family.
	PrimitiveNotation
)

// ValidPrimitiveKind reports whether kind is a known primitive datatype family.
func ValidPrimitiveKind(kind PrimitiveKind) bool {
	switch kind {
	case PrimitiveString,
		PrimitiveBoolean,
		PrimitiveDecimal,
		PrimitiveFloat,
		PrimitiveDouble,
		PrimitiveDuration,
		PrimitiveDateTime,
		PrimitiveTime,
		PrimitiveDate,
		PrimitiveGYearMonth,
		PrimitiveGYear,
		PrimitiveGMonthDay,
		PrimitiveGDay,
		PrimitiveGMonth,
		PrimitiveHexBinary,
		PrimitiveBase64Binary,
		PrimitiveAnyURI,
		PrimitiveQName,
		PrimitiveNotation:
		return true
	default:
		return false
	}
}

// SimpleType is the runtime record for one simple type.
type SimpleType struct {
	Union []SimpleTypeID
	// UnionSources records direct union-member derivation edges until audited
	// publication. Published validation projections do not retain it.
	UnionSources []SimpleTypeID
	Facets       FacetSet
	Name         QName
	Base         SimpleTypeID
	ListItem     SimpleTypeID
	Variety      SimpleVariety
	Primitive    PrimitiveKind
	Final        DerivationMask
	Whitespace   WhitespaceMode
	Builtin      BuiltinValidationKind
	Identity     SimpleIdentityKind
	Fast         SimpleFastKind
	Missing      bool
	Scope        DeclarationScope
}

// SimpleTypeByID resolves a simple type ID against a simple-type table.
func SimpleTypeByID(types []SimpleType, id SimpleTypeID) (*SimpleType, bool) {
	if !ValidSimpleTypeID(id, len(types)) {
		return nil, false
	}
	return &types[id], true
}

// UsableSimpleType resolves a simple type for value validation. It rejects
// the missing-type sentinel recorded for unresolved optional imports.
func UsableSimpleType(types []SimpleType, id SimpleTypeID) (*SimpleType, bool) {
	st, ok := SimpleTypeByID(types, id)
	if !ok || st.Missing {
		return nil, false
	}
	return st, true
}

// MissingSimpleTypeLocalName returns the local name used for the compiler's
// sentinel simple type for unresolved references.
func MissingSimpleTypeLocalName() string {
	return "missing"
}

// MissingSimpleType returns the runtime declaration for the compiler sentinel
// used when schema compilation continues after an unresolved simple type.
func MissingSimpleType(name QName, base SimpleTypeID) SimpleType {
	st := SimpleType{
		Name:       name,
		Variety:    SimpleVarietyAtomic,
		Primitive:  PrimitiveString,
		Base:       base,
		ListItem:   NoSimpleType,
		Whitespace: WhitespaceCollapse,
		Missing:    true,
	}
	st.Fast = DeriveSimpleFastPathForSimpleType(st)
	return st
}

// WhitespaceMode identifies the whiteSpace facet value.
type WhitespaceMode uint8

const (
	// WhitespacePreserve preserves XML whitespace.
	WhitespacePreserve WhitespaceMode = iota
	// WhitespaceReplace replaces XML whitespace characters with spaces.
	WhitespaceReplace
	// WhitespaceCollapse replaces, collapses, and trims XML whitespace.
	WhitespaceCollapse
)

// ValidWhitespaceMode reports whether mode is a known whiteSpace facet value.
func ValidWhitespaceMode(mode WhitespaceMode) bool {
	switch mode {
	case WhitespacePreserve, WhitespaceReplace, WhitespaceCollapse:
		return true
	default:
		return false
	}
}

// ValidWhitespaceRestriction reports whether next preserves or tightens base.
func ValidWhitespaceRestriction(base, next WhitespaceMode) bool {
	if base == WhitespaceCollapse {
		return next == WhitespaceCollapse
	}
	if base == WhitespaceReplace {
		return next != WhitespacePreserve
	}
	return true
}

// SimpleIdentityKind identifies built-in simple-type identity behavior.
type SimpleIdentityKind uint8

const (
	// SimpleIdentityNone means values are not ID or IDREF values.
	SimpleIdentityNone SimpleIdentityKind = iota
	// SimpleIdentityID means values define XML IDs.
	SimpleIdentityID
	// SimpleIdentityIDREF means values reference XML IDs.
	SimpleIdentityIDREF
	// SimpleIdentityIDREFList means list items reference XML IDs.
	SimpleIdentityIDREFList
)

// FacetMask records which facet families are present or fixed on a simple type.
type FacetMask uint16

const (
	// FacetLength records the length facet.
	FacetLength FacetMask = 1 << iota
	// FacetMinLength records the minLength facet.
	FacetMinLength
	// FacetMaxLength records the maxLength facet.
	FacetMaxLength
	// FacetTotalDigits records the totalDigits facet.
	FacetTotalDigits
	// FacetFractionDigits records the fractionDigits facet.
	FacetFractionDigits
	// FacetMinInclusive records the minInclusive facet.
	FacetMinInclusive
	// FacetMaxInclusive records the maxInclusive facet.
	FacetMaxInclusive
	// FacetMinExclusive records the minExclusive facet.
	FacetMinExclusive
	// FacetMaxExclusive records the maxExclusive facet.
	FacetMaxExclusive
	// FacetEnumeration records the enumeration facet.
	FacetEnumeration
	// FacetPattern records the pattern facet.
	FacetPattern
	// FacetWhiteSpace is valid only in the fixed mask; whiteSpace itself is
	// always represented by the simple type's whitespace mode.
	FacetWhiteSpace
)

// FacetSet stores compiled facet values attached to a simple type.
type FacetSet struct {
	bounds         facetBounds
	patterns       stringPatternSteps
	Enumeration    []CompiledLiteral
	Length         uint32
	MinLength      uint32
	MaxLength      uint32
	TotalDigits    uint32
	FractionDigits uint32
	Present        FacetMask
	Fixed          FacetMask
}

type facetBounds [4]*CompiledLiteral

const (
	minInclusiveBoundIndex = iota
	maxInclusiveBoundIndex
	minExclusiveBoundIndex
	maxExclusiveBoundIndex
)

// CompiledLiteral stores a facet literal in lexical, canonical, and parsed
// value-space forms.
type CompiledLiteral struct {
	Lexical       string
	Canonical     string
	ResolvedNames []ResolvedValueName
	Actual        PrimitiveActualValue
	Type          SimpleTypeID
}

// NewCompiledLiteralForSimpleType constructs a facet literal cache with the
// type and QName-resolution proof needed for publication replay.
func NewCompiledLiteralForSimpleType(typ SimpleType, id SimpleTypeID, lexical, canonical string, resolvedNames []ResolvedValueName) CompiledLiteral {
	return CompiledLiteral{
		Lexical:       lexical,
		Canonical:     canonical,
		ResolvedNames: slices.Clone(resolvedNames),
		Actual:        compiledLiteralActualValue(typ, lexical),
		Type:          id,
	}
}

func compiledLiteralActualValue(typ SimpleType, lexical string) PrimitiveActualValue {
	if typ.Variety != SimpleVarietyAtomic {
		return PrimitiveActualValue{}
	}
	switch typ.Primitive {
	case PrimitiveQName, PrimitiveNotation:
		return PrimitiveActualValue{Kind: typ.Primitive, Valid: true}
	default:
		normalized := normalizeSimpleValueLexical(lexical, typ.Whitespace)
		parsed, err := ParsePrimitiveActual(typ.Primitive, normalized, PrimitiveNeedCanonical|PrimitiveNeedLength)
		if err != nil {
			return PrimitiveActualValue{}
		}
		return parsed.Actual
	}
}

// EqualCompiledLiterals reports whether two compiled facet literals represent
// the same primitive actual value.
func EqualCompiledLiterals(a, b *CompiledLiteral) bool {
	if a == nil || b == nil {
		return a == b
	}
	return EqualPrimitiveActualValues(a.Actual, a.Canonical, b.Actual, b.Canonical)
}

// SetFacet records a facet's presence and, when fixed is true, its fixedness.
func SetFacet(f *FacetSet, flag FacetMask, fixed bool) {
	f.Present |= flag
	if fixed {
		f.Fixed |= flag
	}
}

// SetFacetPresent records a non-fixed facet's presence.
func SetFacetPresent(f *FacetSet, flag FacetMask) {
	SetFacet(f, flag, false)
}

// SetBoundFacet records an ordered bound facet literal and its presence bit.
func SetBoundFacet(f *FacetSet, flag FacetMask, lit CompiledLiteral, fixed bool) {
	idx, ok := boundFacetIndex(flag)
	if !ok {
		return
	}
	lit.ResolvedNames = slices.Clone(lit.ResolvedNames)
	f.bounds[idx] = &lit
	SetFacet(f, flag, fixed)
}

// ClearFacet removes a facet from both presence and fixedness masks.
func ClearFacet(f *FacetSet, flag FacetMask) {
	f.Present &^= flag
	f.Fixed &^= flag
	switch flag {
	case FacetLength:
		f.Length = 0
	case FacetMinLength:
		f.MinLength = 0
	case FacetMaxLength:
		f.MaxLength = 0
	case FacetTotalDigits:
		f.TotalDigits = 0
	case FacetFractionDigits:
		f.FractionDigits = 0
	case FacetMinInclusive, FacetMaxInclusive, FacetMinExclusive, FacetMaxExclusive:
		clearBoundFacet(f, flag)
	case FacetEnumeration:
		f.Enumeration = nil
	case FacetPattern:
		f.patterns = stringPatternSteps{}
	default:
	}
}

func clearBoundFacet(f *FacetSet, flag FacetMask) {
	idx, ok := boundFacetIndex(flag)
	if ok {
		f.bounds[idx] = nil
	}
}

// SetWhiteSpaceFacetFixed records whiteSpace fixedness. The whiteSpace value
// itself lives on the simple type, so this only affects the fixed mask.
func SetWhiteSpaceFacetFixed(f *FacetSet, fixed bool) {
	if fixed {
		f.Fixed |= FacetWhiteSpace
	}
}

// FacetMaskShape is the simple-type facet bitset projection validated at
// runtime freeze.
type FacetMaskShape struct {
	Actual  FacetMask
	Present FacetMask
	Fixed   FacetMask
}

// FacetMaskShapeForFacetSet returns the runtime-owned bitset projection for f.
func FacetMaskShapeForFacetSet(f FacetSet) FacetMaskShape {
	return FacetMaskShape{
		Actual:  actualFacetMask(f),
		Present: f.Present,
		Fixed:   f.Fixed,
	}
}

func actualFacetMask(f FacetSet) FacetMask {
	actual := f.Present & cardinalityFacetMask
	if f.Length != 0 {
		actual |= FacetLength
	}
	if f.MinLength != 0 {
		actual |= FacetMinLength
	}
	if f.MaxLength != 0 {
		actual |= FacetMaxLength
	}
	if f.TotalDigits != 0 {
		actual |= FacetTotalDigits
	}
	if f.FractionDigits != 0 {
		actual |= FacetFractionDigits
	}
	if compiledLiteralPresent(f.bounds[minInclusiveBoundIndex]) {
		actual |= FacetMinInclusive
	}
	if compiledLiteralPresent(f.bounds[maxInclusiveBoundIndex]) {
		actual |= FacetMaxInclusive
	}
	if compiledLiteralPresent(f.bounds[minExclusiveBoundIndex]) {
		actual |= FacetMinExclusive
	}
	if compiledLiteralPresent(f.bounds[maxExclusiveBoundIndex]) {
		actual |= FacetMaxExclusive
	}
	if len(f.Enumeration) != 0 {
		actual |= FacetEnumeration
	}
	if f.patterns.count() != 0 {
		actual |= FacetPattern
	}
	return actual
}

func compiledLiteralPresent(lit *CompiledLiteral) bool {
	return lit != nil && (lit.Lexical != "" || lit.Canonical != "" || lit.Actual.Valid)
}

// BoundFacet returns a compiled ordered bound literal only when its presence
// bit and storage agree.
func BoundFacet(f FacetSet, flag FacetMask) (CompiledLiteral, bool) {
	idx, ok := boundFacetIndex(flag)
	if !ok || f.Present&flag == 0 || f.bounds[idx] == nil {
		return CompiledLiteral{}, false
	}
	return *f.bounds[idx], true
}

func boundFacetIndex(flag FacetMask) (int, bool) {
	switch flag {
	case FacetMinInclusive:
		return minInclusiveBoundIndex, true
	case FacetMaxInclusive:
		return maxInclusiveBoundIndex, true
	case FacetMinExclusive:
		return minExclusiveBoundIndex, true
	case FacetMaxExclusive:
		return maxExclusiveBoundIndex, true
	default:
		return 0, false
	}
}

const cardinalityFacetMask = FacetLength |
	FacetMinLength |
	FacetMaxLength |
	FacetTotalDigits |
	FacetFractionDigits

// ValidateFacetMaskShape validates that facet presence/fixed masks agree with
// the actual stored facet values.
func ValidateFacetMaskShape(shape FacetMaskShape) error {
	if shape.Present&FacetWhiteSpace != 0 {
		return errors.New("simple type facet presence mask cannot set whiteSpace")
	}
	if shape.Present != shape.Actual {
		return errors.New("simple type facet presence mask does not match actual facets")
	}
	if shape.Fixed&^(shape.Present|FacetWhiteSpace) != 0 {
		return errors.New("simple type facet fixed mask exceeds present facets")
	}
	return nil
}

// ValidateSimpleTypeFacetMaskShape validates stored facet masks and whether
// the stored facet families are legal for a simple type.
func ValidateSimpleTypeFacetMaskShape(variety SimpleVariety, primitive PrimitiveKind, shape FacetMaskShape) error {
	if err := ValidateFacetMaskShape(shape); err != nil {
		return err
	}
	if !FacetMaskAllowedForSimpleType(variety, primitive, shape.Actual) {
		return errors.New("simple type stores facet not allowed for type")
	}
	return nil
}

// ValidateSimpleTypeFacetMaskForSimpleType validates stored facet masks and
// facet-family applicability for st.
func ValidateSimpleTypeFacetMaskForSimpleType(st SimpleType) error {
	return ValidateSimpleTypeFacetMaskShape(st.Variety, st.Primitive, FacetMaskShapeForFacetSet(st.Facets))
}

// DecimalBoundFacetLiteral is the runtime projection of an ordered decimal
// facet literal needed to prove decimal bounds can be used without reparsing.
type DecimalBoundFacetLiteral struct {
	Present     bool
	ActualValid bool
	ActualKind  PrimitiveKind
}

// DecimalBoundFacetLiteralShape is the simple-type projection needed to
// validate cached decimal actual values for ordered decimal bound facets.
type DecimalBoundFacetLiteralShape struct {
	Variety      SimpleVariety
	Primitive    PrimitiveKind
	MinInclusive DecimalBoundFacetLiteral
	MaxInclusive DecimalBoundFacetLiteral
	MinExclusive DecimalBoundFacetLiteral
	MaxExclusive DecimalBoundFacetLiteral
}

// DecimalBoundFacetLiteralShapeForSimpleType returns the runtime-owned
// ordered decimal bound literal projection for st.
func DecimalBoundFacetLiteralShapeForSimpleType(st SimpleType) DecimalBoundFacetLiteralShape {
	minInclusive, hasMinInclusive := BoundFacet(st.Facets, FacetMinInclusive)
	maxInclusive, hasMaxInclusive := BoundFacet(st.Facets, FacetMaxInclusive)
	minExclusive, hasMinExclusive := BoundFacet(st.Facets, FacetMinExclusive)
	maxExclusive, hasMaxExclusive := BoundFacet(st.Facets, FacetMaxExclusive)
	return DecimalBoundFacetLiteralShape{
		Variety:      st.Variety,
		Primitive:    st.Primitive,
		MinInclusive: decimalBoundFacetLiteral(minInclusive, hasMinInclusive),
		MaxInclusive: decimalBoundFacetLiteral(maxInclusive, hasMaxInclusive),
		MinExclusive: decimalBoundFacetLiteral(minExclusive, hasMinExclusive),
		MaxExclusive: decimalBoundFacetLiteral(maxExclusive, hasMaxExclusive),
	}
}

func decimalBoundFacetLiteral(lit CompiledLiteral, present bool) DecimalBoundFacetLiteral {
	if !present {
		return DecimalBoundFacetLiteral{}
	}
	return DecimalBoundFacetLiteral{
		Present:     true,
		ActualValid: lit.Actual.Valid,
		ActualKind:  lit.Actual.Kind,
	}
}

// ValidateDecimalBoundFacetLiterals validates that ordered decimal bound
// facets carry cached decimal actual values. Non-decimal types do not require
// decimal actual values on their bound literals.
func ValidateDecimalBoundFacetLiterals(shape DecimalBoundFacetLiteralShape) error {
	if shape.Variety != SimpleVarietyAtomic || shape.Primitive != PrimitiveDecimal {
		return nil
	}
	for _, lit := range []DecimalBoundFacetLiteral{
		shape.MinInclusive,
		shape.MaxInclusive,
		shape.MinExclusive,
		shape.MaxExclusive,
	} {
		if lit.Present && (!lit.ActualValid || lit.ActualKind != PrimitiveDecimal) {
			return errors.New("decimal bound facet literal lacks decimal actual value")
		}
	}
	return nil
}

// ValidateDecimalBoundFacetLiteralsForSimpleType validates cached ordered
// decimal bound actual values for st.
func ValidateDecimalBoundFacetLiteralsForSimpleType(st SimpleType) error {
	return ValidateDecimalBoundFacetLiterals(DecimalBoundFacetLiteralShapeForSimpleType(st))
}

// FacetCardinalityValue is an optional unsigned facet value.
type FacetCardinalityValue struct {
	Value   uint32
	Present bool
}

// FacetCardinalityShape is the simple-type numeric facet projection used to
// validate relationships between length and digit-count facets.
type FacetCardinalityShape struct {
	Length         FacetCardinalityValue
	MinLength      FacetCardinalityValue
	MaxLength      FacetCardinalityValue
	TotalDigits    FacetCardinalityValue
	FractionDigits FacetCardinalityValue
}

// FacetCardinalityShapeForSimpleType returns the runtime-owned numeric facet
// projection for st.
func FacetCardinalityShapeForSimpleType(st SimpleType) FacetCardinalityShape {
	return FacetCardinalityShape{
		Length:         facetCardinalityValue(st.Facets.Length, st.Facets.Present&FacetLength != 0),
		MinLength:      facetCardinalityValue(st.Facets.MinLength, st.Facets.Present&FacetMinLength != 0),
		MaxLength:      facetCardinalityValue(st.Facets.MaxLength, st.Facets.Present&FacetMaxLength != 0),
		TotalDigits:    facetCardinalityValue(st.Facets.TotalDigits, st.Facets.Present&FacetTotalDigits != 0),
		FractionDigits: facetCardinalityValue(st.Facets.FractionDigits, st.Facets.Present&FacetFractionDigits != 0),
	}
}

// ValidateFacetCardinalityShape validates numeric facet relationships that
// depend only on effective facet values.
func ValidateFacetCardinalityShape(shape FacetCardinalityShape) error {
	if shape.Length.Present && shape.MinLength.Present && shape.Length.Value < shape.MinLength.Value {
		return errors.New("length cannot be less than minLength")
	}
	if shape.Length.Present && shape.MaxLength.Present && shape.Length.Value > shape.MaxLength.Value {
		return errors.New("length cannot exceed maxLength")
	}
	if shape.MinLength.Present && shape.MaxLength.Present && shape.MinLength.Value > shape.MaxLength.Value {
		return errors.New("minLength cannot exceed maxLength")
	}
	if shape.TotalDigits.Present && shape.FractionDigits.Present && shape.FractionDigits.Value > shape.TotalDigits.Value {
		return errors.New("fractionDigits cannot exceed totalDigits")
	}
	return nil
}

// ValidateFacetCardinalityAncestry validates the XSD 1.0 restriction-history
// requirement for effective length with minLength or maxLength. A matching
// direct-base bound is an inductive witness: validating every base row reaches
// the required ancestor in which that bound exists without length.
func ValidateFacetCardinalityAncestry(derived, base FacetCardinalityShape) error {
	if derived.Length.Present && derived.MinLength.Present && !facetCardinalityEqual(derived.MinLength, base.MinLength) {
		return errors.New("length requires an inherited minLength with the same value")
	}
	if derived.Length.Present && derived.MaxLength.Present && !facetCardinalityEqual(derived.MaxLength, base.MaxLength) {
		return errors.New("length requires an inherited maxLength with the same value")
	}
	return nil
}

// ValidateFacetCardinalityRestriction validates that derived numeric facets do
// not loosen the base type's length and digit-count facets.
func ValidateFacetCardinalityRestriction(derived, base FacetCardinalityShape) error {
	if !facetCardinalityRestricts(derived.Length, base.Length, facetCardinalityExact) {
		return errors.New("length must equal base length")
	}
	if !facetCardinalityRestricts(derived.MinLength, base.MinLength, facetCardinalityLower) {
		return errors.New("minLength cannot be less than base minLength")
	}
	if !facetCardinalityRestricts(derived.MaxLength, base.MaxLength, facetCardinalityUpper) {
		return errors.New("maxLength cannot exceed base maxLength")
	}
	if !facetCardinalityRestricts(derived.TotalDigits, base.TotalDigits, facetCardinalityUpper) {
		return errors.New("totalDigits cannot exceed base totalDigits")
	}
	if !facetCardinalityRestricts(derived.FractionDigits, base.FractionDigits, facetCardinalityUpper) {
		return errors.New("fractionDigits cannot exceed base fractionDigits")
	}
	return nil
}

// FixedFacetValues is the value-independent projection of simple facets whose
// fixedness can be validated without schema-private literal representations.
type FixedFacetValues struct {
	Length         FacetCardinalityValue
	MinLength      FacetCardinalityValue
	MaxLength      FacetCardinalityValue
	TotalDigits    FacetCardinalityValue
	FractionDigits FacetCardinalityValue
	Whitespace     WhitespaceMode
}

// FixedLiteralFacetPreservation is the caller-supplied preservation fact for a
// fixed ordered literal facet.
type FixedLiteralFacetPreservation struct {
	BasePresent    bool
	DerivedPresent bool
	Equal          bool
}

// FixedFacetPreservation is the projection needed to validate preservation of
// fixed simple facets across a restriction step.
type FixedFacetPreservation struct {
	BaseFixed    FacetMask
	Base         FixedFacetValues
	Derived      FixedFacetValues
	MinInclusive FixedLiteralFacetPreservation
	MaxInclusive FixedLiteralFacetPreservation
	MinExclusive FixedLiteralFacetPreservation
	MaxExclusive FixedLiteralFacetPreservation
}

// FixedFacetPreservationForSimpleTypes returns the runtime-owned projection
// needed to validate preservation of fixed facets from base to derived.
func FixedFacetPreservationForSimpleTypes(derived, base SimpleType) FixedFacetPreservation {
	derivedMinInclusive, derivedHasMinInclusive := BoundFacet(derived.Facets, FacetMinInclusive)
	derivedMaxInclusive, derivedHasMaxInclusive := BoundFacet(derived.Facets, FacetMaxInclusive)
	derivedMinExclusive, derivedHasMinExclusive := BoundFacet(derived.Facets, FacetMinExclusive)
	derivedMaxExclusive, derivedHasMaxExclusive := BoundFacet(derived.Facets, FacetMaxExclusive)
	baseMinInclusive, baseHasMinInclusive := BoundFacet(base.Facets, FacetMinInclusive)
	baseMaxInclusive, baseHasMaxInclusive := BoundFacet(base.Facets, FacetMaxInclusive)
	baseMinExclusive, baseHasMinExclusive := BoundFacet(base.Facets, FacetMinExclusive)
	baseMaxExclusive, baseHasMaxExclusive := BoundFacet(base.Facets, FacetMaxExclusive)
	return FixedFacetPreservation{
		BaseFixed:    base.Facets.Fixed,
		Base:         fixedFacetValuesForSimpleType(base),
		Derived:      fixedFacetValuesForSimpleType(derived),
		MinInclusive: fixedLiteralFacetPreservation(derivedMinInclusive, derivedHasMinInclusive, baseMinInclusive, baseHasMinInclusive),
		MaxInclusive: fixedLiteralFacetPreservation(derivedMaxInclusive, derivedHasMaxInclusive, baseMaxInclusive, baseHasMaxInclusive),
		MinExclusive: fixedLiteralFacetPreservation(derivedMinExclusive, derivedHasMinExclusive, baseMinExclusive, baseHasMinExclusive),
		MaxExclusive: fixedLiteralFacetPreservation(derivedMaxExclusive, derivedHasMaxExclusive, baseMaxExclusive, baseHasMaxExclusive),
	}
}

func fixedFacetValuesForSimpleType(st SimpleType) FixedFacetValues {
	shape := FacetCardinalityShapeForSimpleType(st)
	return FixedFacetValues{
		Length:         shape.Length,
		MinLength:      shape.MinLength,
		MaxLength:      shape.MaxLength,
		TotalDigits:    shape.TotalDigits,
		FractionDigits: shape.FractionDigits,
		Whitespace:     st.Whitespace,
	}
}

func fixedLiteralFacetPreservation(derived CompiledLiteral, derivedPresent bool, base CompiledLiteral, basePresent bool) FixedLiteralFacetPreservation {
	return FixedLiteralFacetPreservation{
		BasePresent:    basePresent,
		DerivedPresent: derivedPresent,
		Equal:          basePresent && derivedPresent && EqualCompiledLiterals(&derived, &base),
	}
}

// ValidateFixedFacetPreservation validates preservation of fixed simple facets.
// Ordered literal equality is provided by the caller as facts so runtime does
// not depend on schema-private literal values.
func ValidateFixedFacetPreservation(shape FixedFacetPreservation) error {
	fixed := shape.BaseFixed
	if fixed&^fixedFacetPreservationMask != 0 {
		return errors.New("fixed facet family cannot be preserved")
	}
	if fixed&FacetLength != 0 && !facetCardinalityEqual(shape.Derived.Length, shape.Base.Length) {
		return errors.New("fixed length facet cannot change")
	}
	if fixed&FacetMinLength != 0 && !facetCardinalityEqual(shape.Derived.MinLength, shape.Base.MinLength) {
		return errors.New("fixed minLength facet cannot change")
	}
	if fixed&FacetMaxLength != 0 && !facetCardinalityEqual(shape.Derived.MaxLength, shape.Base.MaxLength) {
		return errors.New("fixed maxLength facet cannot change")
	}
	if fixed&FacetTotalDigits != 0 && !facetCardinalityEqual(shape.Derived.TotalDigits, shape.Base.TotalDigits) {
		return errors.New("fixed totalDigits facet cannot change")
	}
	if fixed&FacetFractionDigits != 0 && !facetCardinalityEqual(shape.Derived.FractionDigits, shape.Base.FractionDigits) {
		return errors.New("fixed fractionDigits facet cannot change")
	}
	if fixed&FacetWhiteSpace != 0 && shape.Derived.Whitespace != shape.Base.Whitespace {
		return errors.New("fixed whiteSpace facet cannot change")
	}
	if fixed&FacetMinInclusive != 0 && !shape.MinInclusive.preserved() {
		return errors.New("fixed minInclusive facet cannot change")
	}
	if fixed&FacetMaxInclusive != 0 && !shape.MaxInclusive.preserved() {
		return errors.New("fixed maxInclusive facet cannot change")
	}
	if fixed&FacetMinExclusive != 0 && !shape.MinExclusive.preserved() {
		return errors.New("fixed minExclusive facet cannot change")
	}
	if fixed&FacetMaxExclusive != 0 && !shape.MaxExclusive.preserved() {
		return errors.New("fixed maxExclusive facet cannot change")
	}
	return nil
}

const fixedFacetPreservationMask = FacetLength |
	FacetMinLength |
	FacetMaxLength |
	FacetTotalDigits |
	FacetFractionDigits |
	FacetMinInclusive |
	FacetMaxInclusive |
	FacetMinExclusive |
	FacetMaxExclusive |
	FacetWhiteSpace

func (p FixedLiteralFacetPreservation) preserved() bool {
	return p.BasePresent && p.DerivedPresent && p.Equal
}

type facetCardinalityRestriction uint8

const (
	facetCardinalityExact facetCardinalityRestriction = iota
	facetCardinalityLower
	facetCardinalityUpper
)

func facetCardinalityRestricts(derived, base FacetCardinalityValue, restriction facetCardinalityRestriction) bool {
	if !base.Present {
		return true
	}
	if !derived.Present {
		return false
	}
	switch restriction {
	case facetCardinalityExact:
		return derived.Value == base.Value
	case facetCardinalityLower:
		return derived.Value >= base.Value
	case facetCardinalityUpper:
		return derived.Value <= base.Value
	default:
		return false
	}
}

func facetCardinalityEqual(a, b FacetCardinalityValue) bool {
	return a.Present == b.Present && (!a.Present || a.Value == b.Value)
}

// OrderedFacetBoundKind identifies whether a projected ordered facet bound is
// absent, inclusive, or exclusive.
type OrderedFacetBoundKind uint8

const (
	// OrderedFacetBoundAbsent means the facet bound is not present.
	OrderedFacetBoundAbsent OrderedFacetBoundKind = iota
	// OrderedFacetBoundInclusive means the bound came from an inclusive facet.
	OrderedFacetBoundInclusive
	// OrderedFacetBoundExclusive means the bound came from an exclusive facet.
	OrderedFacetBoundExclusive
)

// OrderedFacetBound is the value-independent projection of an ordered facet
// bound used for restriction checks.
type OrderedFacetBound struct {
	Kind OrderedFacetBoundKind
}

// OrderedFacetRelation is the comparison result for derived bound value
// against the base bound value.
type OrderedFacetRelation uint8

const (
	// OrderedFacetLess means the derived bound value is less than the base.
	OrderedFacetLess OrderedFacetRelation = iota
	// OrderedFacetEqual means the derived bound value equals the base.
	OrderedFacetEqual
	// OrderedFacetGreater means the derived bound value is greater than the base.
	OrderedFacetGreater
	// OrderedFacetIncomparable means the two bound values are not comparable.
	OrderedFacetIncomparable
)

// OrderedFacetStep is the projection of ordered facets declared in one
// restriction step.
type OrderedFacetStep struct {
	MinInclusive bool
	MinExclusive bool
	MaxInclusive bool
	MaxExclusive bool
}

// ValidateOrderedFacetStep validates mutually exclusive ordered facets within
// one restriction step.
func ValidateOrderedFacetStep(step OrderedFacetStep) error {
	if step.MinInclusive && step.MinExclusive {
		return errors.New("minInclusive and minExclusive cannot both be specified")
	}
	if step.MaxInclusive && step.MaxExclusive {
		return errors.New("maxInclusive and maxExclusive cannot both be specified")
	}
	return nil
}

// OrderedFacetStepHasBounds reports whether a restriction step declares any
// ordered facet bound.
func OrderedFacetStepHasBounds(step OrderedFacetStep) bool {
	return step.MinInclusive || step.MinExclusive || step.MaxInclusive || step.MaxExclusive
}

// OrderedFacetBaseRestriction is the runtime projection needed to validate
// that a restriction step's ordered facets do not loosen the base type.
type OrderedFacetBaseRestriction struct {
	Step                 OrderedFacetStep
	DerivedRestrictsBase bool
}

// ValidateOrderedFacetBaseRestriction validates the step-level ordered facet
// restriction policy after schema has projected primitive-specific bound
// comparison facts.
func ValidateOrderedFacetBaseRestriction(shape OrderedFacetBaseRestriction) error {
	if !OrderedFacetStepHasBounds(shape.Step) {
		return nil
	}
	if !shape.DerivedRestrictsBase {
		return errors.New("ordered facets cannot loosen base ordered facets")
	}
	return nil
}

// OrderedFacetBoundsValidation is the value-independent projection needed to
// validate lower/upper ordered facet consistency for one primitive family.
type OrderedFacetBoundsValidation struct {
	Primitive PrimitiveKind
	Lower     OrderedFacetBound
	Upper     OrderedFacetBound
	Relation  OrderedFacetRelation
}

// ValidateOrderedFacetBounds validates that projected lower/upper ordered
// bounds are consistent for the primitive family's total or partial order.
func ValidateOrderedFacetBounds(shape OrderedFacetBoundsValidation) error {
	label, partial, ok := orderedFacetBoundsPolicy(shape.Primitive)
	if !ok {
		return errors.New("primitive does not support ordered facet bounds")
	}
	consistent := OrderedFacetBoundsConsistent(shape.Lower, shape.Upper, shape.Relation)
	if partial {
		consistent = PartialOrderedFacetBoundsConsistent(shape.Lower, shape.Upper, shape.Relation)
	}
	if !consistent {
		return errors.New(label + " lower bound cannot exceed upper bound")
	}
	return nil
}

func orderedFacetBoundsPolicy(kind PrimitiveKind) (string, bool, bool) {
	switch kind {
	case PrimitiveDecimal:
		return "decimal", false, true
	case PrimitiveFloat, PrimitiveDouble:
		return "float", false, true
	case PrimitiveDuration:
		return "duration", true, true
	case PrimitiveDateTime, PrimitiveDate, PrimitiveTime:
		return "temporal", true, true
	case PrimitiveGYearMonth:
		return "gYearMonth", true, true
	case PrimitiveGYear:
		return "gYear", true, true
	case PrimitiveGMonthDay:
		return "gMonthDay", true, true
	case PrimitiveGDay:
		return "gDay", true, true
	case PrimitiveGMonth:
		return "gMonth", true, true
	default:
		return "", false, false
	}
}

// OrderedFacetBoundRestriction is the value-independent projection needed to
// validate one declared bound against the inherited base bound.
type OrderedFacetBoundRestriction struct {
	Facet    string
	Derived  OrderedFacetBound
	Base     OrderedFacetBound
	Relation OrderedFacetRelation
}

// ValidateOrderedFacetLowerRestriction validates that a derived lower bound
// does not loosen its inherited base lower bound.
func ValidateOrderedFacetLowerRestriction(shape OrderedFacetBoundRestriction) error {
	if !OrderedFacetLowerRestricts(shape.Derived, shape.Base, shape.Relation) {
		return errors.New(orderedFacetName(shape.Facet) + " cannot be less than base lower bound")
	}
	return nil
}

// ValidateOrderedFacetUpperRestriction validates that a derived upper bound
// does not loosen its inherited base upper bound.
func ValidateOrderedFacetUpperRestriction(shape OrderedFacetBoundRestriction) error {
	if !OrderedFacetUpperRestricts(shape.Derived, shape.Base, shape.Relation) {
		return errors.New(orderedFacetName(shape.Facet) + " cannot exceed base upper bound")
	}
	return nil
}

func orderedFacetName(name string) string {
	if name == "" {
		return "ordered facet"
	}
	return name
}

// OrderedFacetLowerBoundAccepts reports whether a value compared to a lower
// bound satisfies that bound.
func OrderedFacetLowerBoundAccepts(bound OrderedFacetBound, relation OrderedFacetRelation) bool {
	if !bound.valid() || !relation.valid() {
		return false
	}
	switch bound.Kind {
	case OrderedFacetBoundAbsent:
		return true
	case OrderedFacetBoundInclusive:
		return relation == OrderedFacetEqual || relation == OrderedFacetGreater
	case OrderedFacetBoundExclusive:
		return relation == OrderedFacetGreater
	default:
		return false
	}
}

// OrderedFacetUpperBoundAccepts reports whether a value compared to an upper
// bound satisfies that bound.
func OrderedFacetUpperBoundAccepts(bound OrderedFacetBound, relation OrderedFacetRelation) bool {
	if !bound.valid() || !relation.valid() {
		return false
	}
	switch bound.Kind {
	case OrderedFacetBoundAbsent:
		return true
	case OrderedFacetBoundInclusive:
		return relation == OrderedFacetEqual || relation == OrderedFacetLess
	case OrderedFacetBoundExclusive:
		return relation == OrderedFacetLess
	default:
		return false
	}
}

// OrderedFacetLowerRestricts reports whether a derived lower bound is at least
// as restrictive as a base lower bound.
func OrderedFacetLowerRestricts(derived, base OrderedFacetBound, relation OrderedFacetRelation) bool {
	if !derived.valid() || !base.valid() {
		return false
	}
	if !base.present() {
		return true
	}
	if !derived.present() || !relation.valid() {
		return false
	}
	return relation == OrderedFacetGreater ||
		relation == OrderedFacetEqual && (derived.exclusive() || !base.exclusive())
}

// OrderedFacetUpperRestricts reports whether a derived upper bound is at least
// as restrictive as a base upper bound.
func OrderedFacetUpperRestricts(derived, base OrderedFacetBound, relation OrderedFacetRelation) bool {
	if !derived.valid() || !base.valid() {
		return false
	}
	if !base.present() {
		return true
	}
	if !derived.present() || !relation.valid() {
		return false
	}
	return relation == OrderedFacetLess ||
		relation == OrderedFacetEqual && (derived.exclusive() || !base.exclusive())
}

// OrderedFacetBoundsConsistent reports whether lower and upper bounds are
// consistent for a total order.
func OrderedFacetBoundsConsistent(lower, upper OrderedFacetBound, relation OrderedFacetRelation) bool {
	if !lower.valid() || !upper.valid() {
		return false
	}
	if !lower.present() || !upper.present() {
		return true
	}
	if !relation.valid() {
		return false
	}
	return relation == OrderedFacetLess ||
		relation == OrderedFacetEqual && !lower.exclusive() && !upper.exclusive()
}

// PartialOrderedFacetBoundsConsistent reports whether lower and upper bounds
// are consistent for a partial order where incomparable bounds do not conflict.
func PartialOrderedFacetBoundsConsistent(lower, upper OrderedFacetBound, relation OrderedFacetRelation) bool {
	if !lower.valid() || !upper.valid() {
		return false
	}
	return relation == OrderedFacetIncomparable ||
		OrderedFacetBoundsConsistent(lower, upper, relation)
}

func (b OrderedFacetBound) present() bool {
	return b.Kind != OrderedFacetBoundAbsent
}

func (b OrderedFacetBound) exclusive() bool {
	return b.Kind == OrderedFacetBoundExclusive
}

func (b OrderedFacetBound) valid() bool {
	return b.Kind <= OrderedFacetBoundExclusive
}

func (r OrderedFacetRelation) valid() bool {
	return r <= OrderedFacetIncomparable
}

func validatePatternFacetRestriction(derived, base stringPatternSteps) error {
	if derived.count() < base.count() {
		return errors.New("simple type patterns loosen base")
	}
	step := derived.tail
	for count := derived.count(); count > base.count(); count-- {
		if step == nil {
			return errors.New("simple type patterns loosen base")
		}
		step = step.parent
	}
	if step != base.tail {
		return errors.New("simple type patterns loosen base")
	}
	return nil
}

func validateEnumerationFacetRestriction(derived, base []CompiledLiteral, compilationType SimpleTypeID) error {
	if sameCompiledLiteralStorage(derived, base) {
		return nil
	}
	if len(base) != 0 && len(derived) == 0 {
		return errors.New("simple type enumeration loosens base")
	}
	for i := range derived {
		if derived[i].Type != compilationType {
			return errors.New("simple type enumeration literal was not compiled against base")
		}
	}
	return nil
}

func sameCompiledLiteralStorage(a, b []CompiledLiteral) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// ValidateFacetRestrictionForSimpleTypes validates facet restriction rules
// between derived and base simple types that depend only on runtime facet data.
func ValidateFacetRestrictionForSimpleTypes(derived, base SimpleType) error {
	if err := ValidateFacetCardinalityRestriction(FacetCardinalityShapeForSimpleType(derived), FacetCardinalityShapeForSimpleType(base)); err != nil {
		return err
	}
	if !OrderedFacetSetRestricts(derived.Variety, derived.Primitive, derived.Facets, base.Facets) {
		return errors.New("simple type ordered facets loosen base")
	}
	if err := validateEnumerationFacetRestriction(derived.Facets.Enumeration, base.Facets.Enumeration, derived.Base); err != nil {
		return err
	}
	return validatePatternFacetRestriction(derived.Facets.patterns, base.Facets.patterns)
}

// ValidateFacetLegalityAndConsistencyForSimpleType validates facet bitsets,
// numeric relationships, and primitive facet-family consistency for st.
func ValidateFacetLegalityAndConsistencyForSimpleType(st SimpleType) error {
	if err := ValidateSimpleTypeFacetMaskForSimpleType(st); err != nil {
		return err
	}
	if err := ValidateDecimalBoundFacetLiteralsForSimpleType(st); err != nil {
		return err
	}
	if err := ValidateFacetCardinalityShape(FacetCardinalityShapeForSimpleType(st)); err != nil {
		return err
	}
	return ValidatePrimitiveFacetRestrictions(st, FacetSet{}, OrderedFacetStep{
		MinInclusive: st.Facets.Present&FacetMinInclusive != 0,
		MaxInclusive: st.Facets.Present&FacetMaxInclusive != 0,
		MinExclusive: st.Facets.Present&FacetMinExclusive != 0,
		MaxExclusive: st.Facets.Present&FacetMaxExclusive != 0,
	})
}

// FacetAllowedForSimpleType reports whether a facet family is legal for a
// simple type's variety and primitive datatype family.
func FacetAllowedForSimpleType(variety SimpleVariety, primitive PrimitiveKind, facet FacetMask) bool {
	switch variety {
	case SimpleVarietyAtomic:
		return atomicFacetAllowed(primitive, facet)
	case SimpleVarietyList:
		switch facet {
		case FacetLength, FacetMinLength, FacetMaxLength, FacetPattern, FacetEnumeration, FacetWhiteSpace:
			return true
		default:
			return false
		}
	case SimpleVarietyUnion:
		return facet == FacetPattern || facet == FacetEnumeration
	default:
		return false
	}
}

// FacetMaskAllowedForSimpleType reports whether every facet family in mask is
// legal for a simple type's variety and primitive datatype family.
func FacetMaskAllowedForSimpleType(variety SimpleVariety, primitive PrimitiveKind, mask FacetMask) bool {
	for facet := FacetLength; facet <= FacetWhiteSpace; facet <<= 1 {
		if mask&facet != 0 && !FacetAllowedForSimpleType(variety, primitive, facet) {
			return false
		}
		mask &^= facet
	}
	return mask == 0
}

func atomicFacetAllowed(kind PrimitiveKind, facet FacetMask) bool {
	switch facet {
	case FacetPattern, FacetEnumeration, FacetWhiteSpace:
		return true
	case FacetLength, FacetMinLength, FacetMaxLength:
		return primitiveHasLengthFacet(kind)
	case FacetMinInclusive, FacetMaxInclusive, FacetMinExclusive, FacetMaxExclusive:
		return primitiveHasOrderFacet(kind)
	case FacetTotalDigits, FacetFractionDigits:
		return kind == PrimitiveDecimal
	default:
		return false
	}
}

func primitiveHasLengthFacet(kind PrimitiveKind) bool {
	switch kind {
	case PrimitiveString, PrimitiveAnyURI, PrimitiveHexBinary, PrimitiveBase64Binary, PrimitiveQName, PrimitiveNotation:
		return true
	default:
		return false
	}
}

func primitiveHasOrderFacet(kind PrimitiveKind) bool {
	switch kind {
	case PrimitiveDecimal, PrimitiveFloat, PrimitiveDouble, PrimitiveDuration, PrimitiveDateTime, PrimitiveTime, PrimitiveDate,
		PrimitiveGYearMonth, PrimitiveGYear, PrimitiveGMonthDay, PrimitiveGDay, PrimitiveGMonth:
		return true
	default:
		return false
	}
}

// SimpleValue is the runtime result of validating one simple-typed lexical value.
type SimpleValue struct {
	Canonical string
	IDs       string
	IDRefs    string
	Identity  string
	Type      SimpleTypeID
}

// AtomicSimpleValueProjection is the runtime-owned simple-value payload
// projection for one validated atomic value.
type AtomicSimpleValueProjection struct {
	Canonical         string
	IdentityCanonical string
	Type              SimpleTypeID
	Primitive         PrimitiveKind
	Identity          SimpleIdentityKind
	Needs             SimpleValueNeed
}

// ListSimpleValueProjection is the runtime-owned simple-value payload projection
// for one validated list value.
type ListSimpleValueProjection struct {
	Canonical    string
	ItemIDRefs   string
	ItemIdentity string
	Type         SimpleTypeID
	Needs        SimpleValueNeed
}

// SimpleValuePayloadType is the simple-type projection needed to validate a
// cached simple value payload.
type SimpleValuePayloadType struct {
	Primitive PrimitiveKind
	Variety   SimpleVariety
	Identity  SimpleIdentityKind
}

// CanonicalText returns the value's canonical lexical form.
func (v SimpleValue) CanonicalText() string {
	return v.Canonical
}

// SimpleValueNeed reports which projections a simple-value validator must compute.
type SimpleValueNeed uint8

const (
	// SimpleNeedCanonical requests canonical text.
	SimpleNeedCanonical SimpleValueNeed = 1 << iota
	// SimpleNeedIdentity requests identity-key data.
	SimpleNeedIdentity
)

// Has reports whether n includes need.
func (n SimpleValueNeed) Has(need SimpleValueNeed) bool {
	return n&need != 0
}

// PrimitiveValueNeed reports which primitive parser projections are required.
type PrimitiveValueNeed uint8

const (
	// PrimitiveNeedCanonical requests canonical text from primitive parsing.
	PrimitiveNeedCanonical PrimitiveValueNeed = 1 << iota
	// PrimitiveNeedLength requests value length from primitive parsing.
	PrimitiveNeedLength
)

// Has reports whether n includes need.
func (n PrimitiveValueNeed) Has(need PrimitiveValueNeed) bool {
	return n&need != 0
}

// PrimitiveValueNeedShape is the simple-type projection used to decide which
// primitive parser outputs are required for full simple-value validation.
type PrimitiveValueNeedShape struct {
	Facets    FacetMask
	Primitive PrimitiveKind
	Builtin   BuiltinValidationKind
	Identity  SimpleIdentityKind
	Needs     SimpleValueNeed
}

// AtomicLengthFacetShape is the atomic simple-type projection used to decide
// whether runtime can compute and validate length facets without schema-private
// typed values.
type AtomicLengthFacetShape struct {
	Facets    FacetMask
	Primitive PrimitiveKind
	Builtin   BuiltinValidationKind
}

// ListSimpleValueNeedShape is the list simple-type projection used to decide
// which item-value outputs are required for full list validation.
type ListSimpleValueNeedShape struct {
	Facets   FacetMask
	Identity SimpleIdentityKind
	Needs    SimpleValueNeed
}

// ListSimpleValueNeedPlan reports the derived outputs needed while validating a
// list simple value.
type ListSimpleValueNeedPlan struct {
	ItemNeeds   SimpleValueNeed
	NeedStrings bool
}

// ListSimpleValueFacetPlan reports which composite list facet executors are
// needed for a validated list value.
type ListSimpleValueFacetPlan struct {
	ValidateLength  bool
	ValidateLexical bool
}

// UnionSimpleValueNeedShape is the union simple-type projection used to decide
// which member-value outputs are required for full union validation.
type UnionSimpleValueNeedShape struct {
	Facets   FacetMask
	Identity SimpleIdentityKind
	Needs    SimpleValueNeed
}

// SimpleValuePrimitiveNeeds derives the primitive parser projections required
// to build the requested simple-value result and evaluate stored facets.
func SimpleValuePrimitiveNeeds(shape PrimitiveValueNeedShape) PrimitiveValueNeed {
	var needs PrimitiveValueNeed
	if shape.Needs.Has(SimpleNeedCanonical) ||
		shape.Identity != SimpleIdentityNone ||
		shape.Primitive != PrimitiveDecimal && (shape.Facets&FacetEnumeration != 0 || shape.Needs.Has(SimpleNeedIdentity)) {
		needs |= PrimitiveNeedCanonical
	}
	if shape.Facets&(FacetLength|FacetMinLength|FacetMaxLength) != 0 &&
		!SimpleValueAtomicLengthFacets(AtomicLengthFacetShape{Facets: shape.Facets, Primitive: shape.Primitive, Builtin: shape.Builtin}) {
		needs |= PrimitiveNeedLength
	}
	return needs
}

// SimpleValueAtomicLengthFacets reports whether atomic length facets can be
// executed entirely from runtime-owned primitive lexical rules.
func SimpleValueAtomicLengthFacets(shape AtomicLengthFacetShape) bool {
	if shape.Facets&(FacetLength|FacetMinLength|FacetMaxLength) == 0 {
		return false
	}
	if shape.Builtin != BuiltinValidationNone {
		return false
	}
	switch shape.Primitive {
	case PrimitiveString, PrimitiveAnyURI, PrimitiveHexBinary, PrimitiveBase64Binary:
		return true
	default:
		return false
	}
}

// SimpleValueListNeeds derives the item validation projections and whether list
// canonical/normalized strings must be built.
func SimpleValueListNeeds(shape ListSimpleValueNeedShape) ListSimpleValueNeedPlan {
	needStrings := shape.Needs.Has(SimpleNeedCanonical) ||
		shape.Needs.Has(SimpleNeedIdentity) ||
		shape.Facets&(FacetPattern|FacetEnumeration) != 0 ||
		shape.Identity != SimpleIdentityNone
	var itemNeeds SimpleValueNeed
	if needStrings {
		itemNeeds = SimpleNeedCanonical
	}
	if shape.Needs.Has(SimpleNeedIdentity) {
		itemNeeds |= SimpleNeedIdentity
	}
	return ListSimpleValueNeedPlan{
		ItemNeeds:   itemNeeds,
		NeedStrings: needStrings,
	}
}

// SimpleValueListFacetPlan derives which list facet callbacks may be crossed.
func SimpleValueListFacetPlan(facets FacetMask) ListSimpleValueFacetPlan {
	return ListSimpleValueFacetPlan{
		ValidateLength:  facets&(FacetLength|FacetMinLength|FacetMaxLength) != 0,
		ValidateLexical: facets&(FacetPattern|FacetEnumeration) != 0,
	}
}

// SimpleValueUnionMemberNeeds derives the member validation projections needed
// for union facet evaluation and requested simple-value output.
func SimpleValueUnionMemberNeeds(shape UnionSimpleValueNeedShape) SimpleValueNeed {
	needs := shape.Needs
	if shape.Needs.Has(SimpleNeedCanonical) ||
		shape.Needs.Has(SimpleNeedIdentity) ||
		shape.Facets&FacetEnumeration != 0 ||
		shape.Identity != SimpleIdentityNone {
		needs |= SimpleNeedCanonical
	}
	return needs
}

// SimpleValueUnionFacetValidation reports whether union facet execution may be
// crossed after a member matches.
func SimpleValueUnionFacetValidation(facets FacetMask) bool {
	return facets&(FacetPattern|FacetEnumeration) != 0
}

// AtomicSimpleValue constructs the ID/IDREF and identity-key payload for a
// validated atomic simple value.
func AtomicSimpleValue(proj AtomicSimpleValueProjection) SimpleValue {
	v := SimpleValue{Canonical: proj.Canonical, Type: proj.Type}
	if proj.Needs.Has(SimpleNeedIdentity) {
		canonical := proj.Canonical
		if proj.IdentityCanonical != "" {
			canonical = proj.IdentityCanonical
		}
		v.Identity = SimpleIdentityKey(proj.Primitive, canonical)
	}
	switch proj.Identity {
	case SimpleIdentityID:
		v.IDs = proj.Canonical
	case SimpleIdentityIDREF:
		v.IDRefs = proj.Canonical
	case SimpleIdentityNone, SimpleIdentityIDREFList:
	}
	return v
}

// ListSimpleValue constructs the IDREF and identity-key payload for a validated
// list simple value.
func ListSimpleValue(proj ListSimpleValueProjection) SimpleValue {
	v := SimpleValue{
		Canonical: proj.Canonical,
		IDRefs:    proj.ItemIDRefs,
		Type:      proj.Type,
	}
	if proj.Needs.Has(SimpleNeedIdentity) {
		v.Identity = SimpleIdentityKey(PrimitiveString, proj.ItemIdentity)
	}
	return v
}

// AppendSimpleValueIDRefs appends IDREFs carried by item to refs, preserving the
// space-separated IDREF list form.
func AppendSimpleValueIDRefs(refs *strings.Builder, item SimpleValue) {
	if item.IDRefs == "" {
		return
	}
	if refs.Len() > 0 {
		refs.WriteByte(' ')
	}
	refs.WriteString(item.IDRefs)
}

// AppendSimpleValueListIdentity appends one length-framed item identity to a
// list identity projection. Length framing prevents adjacent item keys from
// colliding regardless of their primitive canonical representation.
func AppendSimpleValueListIdentity(identity *strings.Builder, item SimpleValue) bool {
	if item.Identity == "" {
		return false
	}
	var length [20]byte
	digits := strconv.AppendInt(length[:0], int64(len(item.Identity)), 10)
	identity.Write(digits)
	identity.WriteByte(':')
	identity.WriteString(item.Identity)
	return true
}

// BuiltinValidationKind identifies extra validation attached to built-in types.
type BuiltinValidationKind uint8

const (
	// BuiltinValidationNone applies no extra built-in validation.
	BuiltinValidationNone BuiltinValidationKind = iota
	// BuiltinValidationInteger validates integer lexical and value constraints.
	BuiltinValidationInteger
	// BuiltinValidationName validates xs:Name.
	BuiltinValidationName
	// BuiltinValidationNCName validates xs:NCName.
	BuiltinValidationNCName
	// BuiltinValidationNMTOKEN validates xs:NMTOKEN.
	BuiltinValidationNMTOKEN
	// BuiltinValidationLanguage validates xs:language.
	BuiltinValidationLanguage
	// BuiltinValidationEntity validates xs:ENTITY.
	BuiltinValidationEntity
	// BuiltinValidationXMLLang validates xml:lang.
	BuiltinValidationXMLLang
	// BuiltinValidationXMLSpace validates xml:space.
	BuiltinValidationXMLSpace
)

// ValidBuiltinValidationKind reports whether kind is a known built-in validator.
func ValidBuiltinValidationKind(kind BuiltinValidationKind) bool {
	switch kind {
	case BuiltinValidationNone,
		BuiltinValidationInteger,
		BuiltinValidationName,
		BuiltinValidationNCName,
		BuiltinValidationNMTOKEN,
		BuiltinValidationLanguage,
		BuiltinValidationEntity,
		BuiltinValidationXMLLang,
		BuiltinValidationXMLSpace:
		return true
	default:
		return false
	}
}

// SimpleValueBuiltinDerivedRuntimeOwned reports whether built-in derived
// lexical validation can execute without schema-private primitive values.
func SimpleValueBuiltinDerivedRuntimeOwned(kind BuiltinValidationKind) bool {
	switch kind {
	case BuiltinValidationNone,
		BuiltinValidationInteger,
		BuiltinValidationName,
		BuiltinValidationNCName,
		BuiltinValidationNMTOKEN,
		BuiltinValidationLanguage,
		BuiltinValidationEntity,
		BuiltinValidationXMLLang,
		BuiltinValidationXMLSpace:
		return true
	default:
		return false
	}
}

// SimpleFastKind identifies a specialized simple-type validation fast path.
type SimpleFastKind uint8

const (
	// SimpleFastNone disables specialized simple validation.
	SimpleFastNone SimpleFastKind = iota
	// SimpleFastInt enables the integer fast path.
	SimpleFastInt
)

// SimpleFastPathValidation is the runtime projection used to derive and
// validate stored simple-type fast-path metadata.
type SimpleFastPathValidation struct {
	FractionDigits           FacetCardinalityValue
	EnumerationSize          int
	PatternGroupSize         int
	Stored                   SimpleFastKind
	Variety                  SimpleVariety
	Primitive                PrimitiveKind
	Builtin                  BuiltinValidationKind
	Whitespace               WhitespaceMode
	HasTotalDigits           bool
	HasMinExclusive          bool
	HasMaxExclusive          bool
	HasLength                bool
	HasMinLength             bool
	HasMaxLength             bool
	MinInclusiveMatchesInt32 bool
	MaxInclusiveMatchesInt32 bool
}

// SimpleFastPathValidationForSimpleType returns the runtime-owned projection
// used to derive and validate stored simple-type fast-path metadata.
func SimpleFastPathValidationForSimpleType(st SimpleType) SimpleFastPathValidation {
	minInclusive, hasMinInclusive := BoundFacet(st.Facets, FacetMinInclusive)
	maxInclusive, hasMaxInclusive := BoundFacet(st.Facets, FacetMaxInclusive)
	return SimpleFastPathValidation{
		FractionDigits:           facetCardinalityValue(st.Facets.FractionDigits, st.Facets.Present&FacetFractionDigits != 0),
		EnumerationSize:          len(st.Facets.Enumeration),
		PatternGroupSize:         int(st.Facets.patterns.count()),
		Stored:                   st.Fast,
		Variety:                  st.Variety,
		Primitive:                st.Primitive,
		Builtin:                  st.Builtin,
		Whitespace:               st.Whitespace,
		HasTotalDigits:           st.Facets.Present&FacetTotalDigits != 0,
		HasMinExclusive:          st.Facets.Present&FacetMinExclusive != 0,
		HasMaxExclusive:          st.Facets.Present&FacetMaxExclusive != 0,
		HasLength:                st.Facets.Present&FacetLength != 0,
		HasMinLength:             st.Facets.Present&FacetMinLength != 0,
		HasMaxLength:             st.Facets.Present&FacetMaxLength != 0,
		MinInclusiveMatchesInt32: decimalBoundIntegerCanonicalEquals(minInclusive, hasMinInclusive, "-2147483648"),
		MaxInclusiveMatchesInt32: decimalBoundIntegerCanonicalEquals(maxInclusive, hasMaxInclusive, "2147483647"),
	}
}

func decimalBoundIntegerCanonicalEquals(lit CompiledLiteral, present bool, want string) bool {
	return present &&
		lit.Actual.Valid &&
		lit.Actual.Kind == PrimitiveDecimal &&
		lit.Actual.Decimal.IntegerCanonicalText() == want
}

// DeriveSimpleFastPathForSimpleType derives the specialized validation fast
// path from a frozen simple type.
func DeriveSimpleFastPathForSimpleType(st SimpleType) SimpleFastKind {
	return DeriveSimpleFastPath(SimpleFastPathValidationForSimpleType(st))
}

// ValidateSimpleFastPathForSimpleType validates the stored fast-path metadata
// against the runtime-owned simple-type projection.
func ValidateSimpleFastPathForSimpleType(st SimpleType) error {
	return ValidateSimpleFastPath(SimpleFastPathValidationForSimpleType(st))
}

// DeriveSimpleFastPath derives the specialized validation fast path from the
// frozen simple-type shape.
func DeriveSimpleFastPath(shape SimpleFastPathValidation) SimpleFastKind {
	if shape.Variety == SimpleVarietyAtomic &&
		shape.Primitive == PrimitiveDecimal &&
		shape.Builtin == BuiltinValidationInteger &&
		shape.Whitespace == WhitespaceCollapse &&
		!shape.HasTotalDigits &&
		shape.FractionDigits.Present &&
		shape.FractionDigits.Value == 0 &&
		!shape.HasMinExclusive &&
		!shape.HasMaxExclusive &&
		!shape.HasLength &&
		!shape.HasMinLength &&
		!shape.HasMaxLength &&
		shape.EnumerationSize == 0 &&
		shape.PatternGroupSize == 0 &&
		shape.MinInclusiveMatchesInt32 &&
		shape.MaxInclusiveMatchesInt32 {
		return SimpleFastInt
	}
	return SimpleFastNone
}

// ValidateSimpleFastPath validates the stored fast-path metadata against the
// facet shape that controls whether the fast path is semantically valid.
func ValidateSimpleFastPath(shape SimpleFastPathValidation) error {
	switch shape.Stored {
	case SimpleFastNone, SimpleFastInt:
	default:
		return errors.New("simple type fast path is invalid")
	}
	if shape.Stored != DeriveSimpleFastPath(shape) {
		return errors.New("simple type fast path does not match facets")
	}
	return nil
}

// SimpleValueRouteAction identifies the validator route for one simple value.
type SimpleValueRouteAction uint8

const (
	// SimpleValueRouteUntyped accepts a value with no simple type.
	SimpleValueRouteUntyped SimpleValueRouteAction = iota
	// SimpleValueRouteMissing rejects a value whose simple type is not usable.
	SimpleValueRouteMissing
	// SimpleValueRouteAtomic validates an atomic simple value.
	SimpleValueRouteAtomic
	// SimpleValueRouteList validates a list simple value.
	SimpleValueRouteList
	// SimpleValueRouteUnion validates a union simple value.
	SimpleValueRouteUnion
	// SimpleValueRouteInvalid rejects a corrupted simple-type variety.
	SimpleValueRouteInvalid
)

// SimpleValueRouteShape is the runtime simple-type projection needed to route
// a simple-value validation.
type SimpleValueRouteShape struct {
	Type    SimpleTypeID
	Variety SimpleVariety
	Known   bool
}

// SimpleValueBypassAction identifies a datatype executor that may validate a
// simple value without taking the full value-construction path.
type SimpleValueBypassAction uint8

const (
	// SimpleValueBypassNone means the full simple-value path is required.
	SimpleValueBypassNone SimpleValueBypassAction = iota
	// SimpleValueBypassAcceptString accepts an unconstrained xs:string-family value.
	SimpleValueBypassAcceptString
	// SimpleValueBypassValidateInt validates the stored integer fast path.
	SimpleValueBypassValidateInt
	// SimpleValueBypassValidateDecimal validates a decimal value without output.
	SimpleValueBypassValidateDecimal
	// SimpleValueBypassValidateStringPatterns validates string pattern facets without output.
	SimpleValueBypassValidateStringPatterns
	// SimpleValueBypassValidateStringEnumeration validates string enumeration facets without output.
	SimpleValueBypassValidateStringEnumeration
	// SimpleValueBypassValidateAnyURI validates an anyURI value without output.
	SimpleValueBypassValidateAnyURI
	// SimpleValueBypassValidateHexBinary validates a hexBinary value without output.
	SimpleValueBypassValidateHexBinary
	// SimpleValueBypassValidateBase64Binary validates a base64Binary value without output.
	SimpleValueBypassValidateBase64Binary
	// SimpleValueBypassValidateFloat validates a float/double value without output.
	SimpleValueBypassValidateFloat
	// SimpleValueBypassValidateDuration validates a duration value without output.
	SimpleValueBypassValidateDuration
	// SimpleValueBypassValidateBoolean validates a boolean value without output.
	SimpleValueBypassValidateBoolean
	// SimpleValueBypassValidateTemporal validates a temporal value without output.
	SimpleValueBypassValidateTemporal
	// SimpleValueBypassValidateDate validates a date value without output.
	SimpleValueBypassValidateDate
)

// SimpleValueBypassShape is the value-independent projection used to choose
// whether a simple-value validation may bypass full value construction.
type SimpleValueBypassShape struct {
	Facets    FacetMask
	Variety   SimpleVariety
	Primitive PrimitiveKind
	Builtin   BuiltinValidationKind
	Identity  SimpleIdentityKind
	Fast      SimpleFastKind
	Needs     SimpleValueNeed
}

// SimpleFixedStringFastPathShape is the projection used to decide whether a
// fixed attribute value can compare raw text directly with the cached value.
type SimpleFixedStringFastPathShape struct {
	Bypass     SimpleValueBypassAction
	Whitespace WhitespaceMode
	HasFixed   bool
}

// SimpleRawListFastPathAction identifies a raw list validator that may validate
// a list simple value without full item value construction.
type SimpleRawListFastPathAction uint8

const (
	// SimpleRawListFastPathNone means the full simple-value path is required.
	SimpleRawListFastPathNone SimpleRawListFastPathAction = iota
	// SimpleRawListFastPathValidateNMTOKENList validates raw xs:NMTOKEN list items.
	SimpleRawListFastPathValidateNMTOKENList
)

// SimpleRawListFastPathShape is the value-independent projection used to choose
// whether a list simple-value validation may bypass full item construction.
type SimpleRawListFastPathShape struct {
	ListFacets   FacetMask
	ListIdentity SimpleIdentityKind
	ItemFacets   FacetMask
	ItemVariety  SimpleVariety
	ItemBuiltin  BuiltinValidationKind
	ItemIdentity SimpleIdentityKind
	ItemKnown    bool
}

// SimpleRawUnionFastPathAction identifies whether a raw union validator may
// attempt member validation without full union value construction.
type SimpleRawUnionFastPathAction uint8

const (
	// SimpleRawUnionFastPathNone means the full simple-value path is required.
	SimpleRawUnionFastPathNone SimpleRawUnionFastPathAction = iota
	// SimpleRawUnionFastPathValidateMembers validates raw union members in order.
	SimpleRawUnionFastPathValidateMembers
)

// SimpleRawUnionMemberAction identifies how a raw union executor should
// validate one union member.
type SimpleRawUnionMemberAction uint8

const (
	// SimpleRawUnionMemberNone means the raw union path cannot use this member.
	SimpleRawUnionMemberNone SimpleRawUnionMemberAction = iota
	// SimpleRawUnionMemberTryBoolean validates the raw member as xs:boolean.
	SimpleRawUnionMemberTryBoolean
	// SimpleRawUnionMemberTryRaw validates the member through its raw simple-value path.
	SimpleRawUnionMemberTryRaw
)

// SimpleRawUnionFastPathShape is the value-independent and raw-input projection
// used to choose whether a union value may use raw member validation.
type SimpleRawUnionFastPathShape struct {
	Facets        FacetMask
	Identity      SimpleIdentityKind
	HasWhitespace bool
}

// SimpleRawUnionMemberShape is the projection used to choose the raw validation
// action for one union member.
type SimpleRawUnionMemberShape struct {
	Facets    FacetMask
	Variety   SimpleVariety
	Primitive PrimitiveKind
	Builtin   BuiltinValidationKind
	Identity  SimpleIdentityKind
	Fast      SimpleFastKind
	Known     bool
}

// SimpleValueRoute chooses the validation route for one simple value. The
// caller projects table presence and variety; runtime owns the policy for how
// those facts map to validation phases.
func SimpleValueRoute(shape SimpleValueRouteShape) SimpleValueRouteAction {
	if shape.Type == NoSimpleType {
		return SimpleValueRouteUntyped
	}
	if !shape.Known {
		return SimpleValueRouteMissing
	}
	switch shape.Variety {
	case SimpleVarietyAtomic:
		return SimpleValueRouteAtomic
	case SimpleVarietyList:
		return SimpleValueRouteList
	case SimpleVarietyUnion:
		return SimpleValueRouteUnion
	}
	return SimpleValueRouteInvalid
}

// SimpleValueBypass chooses the cheapest semantically valid validation action
// for a simple value shape. Runtime owns the phase-crossing policy for when an
// executor is admissible; individual executors are either runtime-owned or
// supplied by the caller.
func SimpleValueBypass(shape SimpleValueBypassShape) SimpleValueBypassAction {
	if shape.Variety != SimpleVarietyAtomic {
		return SimpleValueBypassNone
	}
	if canAcceptStringBypass(shape) {
		return SimpleValueBypassAcceptString
	}
	if shape.Identity != SimpleIdentityNone || shape.Needs != 0 {
		return SimpleValueBypassNone
	}
	if shape.Fast == SimpleFastInt {
		return SimpleValueBypassValidateInt
	}
	switch {
	case canValidateDecimalNoOutput(shape):
		return SimpleValueBypassValidateDecimal
	case canValidateStringPatternsNoOutput(shape):
		return SimpleValueBypassValidateStringPatterns
	case canValidateStringEnumerationNoOutput(shape):
		return SimpleValueBypassValidateStringEnumeration
	case canValidateAnyURINoOutput(shape):
		return SimpleValueBypassValidateAnyURI
	case canValidateHexBinaryNoOutput(shape):
		return SimpleValueBypassValidateHexBinary
	case canValidateBase64BinaryNoOutput(shape):
		return SimpleValueBypassValidateBase64Binary
	case canValidateFloatNoOutput(shape):
		return SimpleValueBypassValidateFloat
	case canValidateDurationNoOutput(shape):
		return SimpleValueBypassValidateDuration
	case canValidateBooleanNoOutput(shape):
		return SimpleValueBypassValidateBoolean
	case canValidateTemporalNoOutput(shape):
		return SimpleValueBypassValidateTemporal
	case canValidateDateNoOutput(shape):
		return SimpleValueBypassValidateDate
	default:
		return SimpleValueBypassNone
	}
}

// SimpleFixedStringFastPath reports whether a fixed attribute value can skip
// datatype validation and compare the instance text to the cached canonical text.
func SimpleFixedStringFastPath(shape SimpleFixedStringFastPathShape) bool {
	return shape.HasFixed &&
		shape.Whitespace == WhitespacePreserve &&
		shape.Bypass == SimpleValueBypassAcceptString
}

// SimpleRawListFastPath chooses the cheapest semantically valid raw list
// validator. Runtime owns the phase-crossing policy for when that executor is
// admissible.
func SimpleRawListFastPath(shape SimpleRawListFastPathShape) SimpleRawListFastPathAction {
	if shape.ListIdentity != SimpleIdentityNone || shape.ListFacets != 0 {
		return SimpleRawListFastPathNone
	}
	if !shape.ItemKnown ||
		shape.ItemVariety != SimpleVarietyAtomic ||
		shape.ItemBuiltin != BuiltinValidationNMTOKEN ||
		shape.ItemIdentity != SimpleIdentityNone ||
		shape.ItemFacets != 0 {
		return SimpleRawListFastPathNone
	}
	return SimpleRawListFastPathValidateNMTOKENList
}

// SimpleRawUnionFastPath chooses whether the raw union member executor is
// semantically admissible for the union shape and raw input.
func SimpleRawUnionFastPath(shape SimpleRawUnionFastPathShape) SimpleRawUnionFastPathAction {
	if shape.Identity != SimpleIdentityNone || shape.Facets != 0 || shape.HasWhitespace {
		return SimpleRawUnionFastPathNone
	}
	return SimpleRawUnionFastPathValidateMembers
}

// SimpleRawUnionMember chooses how the raw union executor should validate one
// member. Runtime owns boolean member execution; other members recurse through
// the raw simple-value dispatcher.
func SimpleRawUnionMember(shape SimpleRawUnionMemberShape) SimpleRawUnionMemberAction {
	if !shape.Known || shape.Identity != SimpleIdentityNone {
		return SimpleRawUnionMemberNone
	}
	action := SimpleValueBypass(SimpleValueBypassShape{
		Facets:    shape.Facets,
		Variety:   shape.Variety,
		Primitive: shape.Primitive,
		Builtin:   shape.Builtin,
		Identity:  shape.Identity,
		Fast:      shape.Fast,
	})
	if action == SimpleValueBypassValidateBoolean {
		return SimpleRawUnionMemberTryBoolean
	}
	return SimpleRawUnionMemberTryRaw
}

func canAcceptStringBypass(shape SimpleValueBypassShape) bool {
	return shape.Primitive == PrimitiveString &&
		shape.Builtin == BuiltinValidationNone &&
		shape.Identity == SimpleIdentityNone &&
		shape.Facets == 0
}

func canValidateDecimalNoOutput(shape SimpleValueBypassShape) bool {
	return shape.Primitive == PrimitiveDecimal &&
		shape.Builtin == BuiltinValidationNone &&
		shape.Facets&(FacetPattern|FacetEnumeration) == 0
}

func canValidateStringPatternsNoOutput(shape SimpleValueBypassShape) bool {
	return shape.Primitive == PrimitiveString &&
		shape.Builtin == BuiltinValidationNone &&
		shape.Facets == FacetPattern
}

func canValidateStringEnumerationNoOutput(shape SimpleValueBypassShape) bool {
	return shape.Primitive == PrimitiveString &&
		shape.Builtin == BuiltinValidationNone &&
		shape.Facets == FacetEnumeration
}

func canValidateAnyURINoOutput(shape SimpleValueBypassShape) bool {
	return shape.Primitive == PrimitiveAnyURI &&
		shape.Builtin == BuiltinValidationNone &&
		shape.Facets == 0
}

func canValidateHexBinaryNoOutput(shape SimpleValueBypassShape) bool {
	return shape.Primitive == PrimitiveHexBinary &&
		shape.Builtin == BuiltinValidationNone &&
		shape.Facets == 0
}

func canValidateBase64BinaryNoOutput(shape SimpleValueBypassShape) bool {
	return shape.Primitive == PrimitiveBase64Binary &&
		shape.Builtin == BuiltinValidationNone &&
		shape.Facets == 0
}

func canValidateFloatNoOutput(shape SimpleValueBypassShape) bool {
	return (shape.Primitive == PrimitiveFloat || shape.Primitive == PrimitiveDouble) &&
		shape.Builtin == BuiltinValidationNone &&
		shape.Facets == 0
}

func canValidateDurationNoOutput(shape SimpleValueBypassShape) bool {
	return shape.Primitive == PrimitiveDuration &&
		shape.Builtin == BuiltinValidationNone &&
		shape.Facets == 0
}

func canValidateBooleanNoOutput(shape SimpleValueBypassShape) bool {
	return shape.Primitive == PrimitiveBoolean &&
		shape.Builtin == BuiltinValidationNone &&
		shape.Facets == 0
}

func canValidateTemporalNoOutput(shape SimpleValueBypassShape) bool {
	switch shape.Primitive {
	case PrimitiveDateTime, PrimitiveTime, PrimitiveGYearMonth, PrimitiveGYear, PrimitiveGMonthDay, PrimitiveGDay, PrimitiveGMonth:
		return shape.Builtin == BuiltinValidationNone && shape.Facets == 0
	default:
		return false
	}
}

func canValidateDateNoOutput(shape SimpleValueBypassShape) bool {
	return shape.Primitive == PrimitiveDate &&
		shape.Builtin == BuiltinValidationNone &&
		shape.Facets == 0
}

// ValidateSimpleTypeFinalAllows validates that a simple-type final mask admits
// one simple derivation step.
func ValidateSimpleTypeFinalAllows(final, derivation DerivationMask) error {
	if !ValidSimpleFinalMask(final) {
		return errors.New("simple type final mask is invalid")
	}
	switch derivation {
	case DerivationRestriction, DerivationList, DerivationUnion:
	default:
		return errors.New("simple type final derivation is invalid")
	}
	if final&derivation != 0 {
		return errors.New("simple type final blocks " + simpleTypeFinalDerivationName(derivation))
	}
	return nil
}

func simpleTypeFinalDerivationName(derivation DerivationMask) string {
	switch derivation {
	case DerivationRestriction:
		return derivationSetRestrictionToken
	case DerivationList:
		return derivationSetListToken
	case DerivationUnion:
		return derivationSetUnionToken
	default:
		return "derivation"
	}
}

// SimpleTypeValidation is the runtime projection needed to validate frozen
// simple-type structural metadata.
type SimpleTypeValidation struct {
	Union      []SimpleTypeID
	Name       QName
	Base       SimpleTypeID
	ListItem   SimpleTypeID
	Variety    SimpleVariety
	Primitive  PrimitiveKind
	Final      DerivationMask
	Whitespace WhitespaceMode
	Builtin    BuiltinValidationKind
}

// NewSimpleTypeValidationForSimpleType returns the runtime-owned structural
// validation projection for st.
func NewSimpleTypeValidationForSimpleType(st SimpleType) SimpleTypeValidation {
	return CloneSimpleTypeValidation(SimpleTypeValidation{
		Union:      st.Union,
		Name:       st.Name,
		Base:       st.Base,
		ListItem:   st.ListItem,
		Variety:    st.Variety,
		Primitive:  st.Primitive,
		Final:      st.Final,
		Whitespace: st.Whitespace,
		Builtin:    st.Builtin,
	})
}

// SimpleTypeRestrictionValidation is the non-facet metadata that must be
// preserved by a simple-type restriction.
type SimpleTypeRestrictionValidation struct {
	Union      []SimpleTypeID
	ListItem   SimpleTypeID
	Variety    SimpleVariety
	Primitive  PrimitiveKind
	Builtin    BuiltinValidationKind
	Whitespace WhitespaceMode
}

// NewSimpleTypeRestrictionValidationForSimpleType returns the runtime-owned
// non-facet restriction projection for st.
func NewSimpleTypeRestrictionValidationForSimpleType(st SimpleType) SimpleTypeRestrictionValidation {
	return CloneSimpleTypeRestrictionValidation(SimpleTypeRestrictionValidation{
		Union:      st.Union,
		ListItem:   st.ListItem,
		Variety:    st.Variety,
		Primitive:  st.Primitive,
		Builtin:    st.Builtin,
		Whitespace: st.Whitespace,
	})
}

// SimpleTypeRefLimits are table sizes used to validate simple-type references
// in a frozen runtime schema.
type SimpleTypeRefLimits struct {
	SimpleTypeCount int
}

// SimpleTypeFinalRuntime supplies simple-type final constraints by ID.
type SimpleTypeFinalRuntime interface {
	SimpleTypeFinal(id SimpleTypeID) (DerivationMask, bool)
}

// ValidateSimpleTypeRuntime validates simple-type metadata that can be
// expressed in runtime vocabulary.
func ValidateSimpleTypeRuntime(names *NameTable, st SimpleTypeValidation, limits SimpleTypeRefLimits) error {
	if names == nil || !names.ValidQName(st.Name) {
		return errors.New("simple type references invalid name")
	}
	if st.Base != NoSimpleType && !validSimpleTypeRuntimeID(st.Base, limits) {
		return errors.New("simple type references invalid base")
	}
	if !ValidPrimitiveKind(st.Primitive) {
		return errors.New("simple type has invalid primitive")
	}
	if !ValidWhitespaceMode(st.Whitespace) {
		return errors.New("simple type has invalid whitespace mode")
	}
	if !ValidBuiltinValidationKind(st.Builtin) {
		return errors.New("simple type has invalid builtin validation kind")
	}
	if !ValidSimpleFinalMask(st.Final) {
		return errors.New("simple type final mask contains invalid derivation")
	}
	switch st.Variety {
	case SimpleVarietyAtomic:
		if st.ListItem != NoSimpleType {
			return errors.New("atomic simple type stores list item")
		}
		if len(st.Union) != 0 {
			return errors.New("atomic simple type stores union members")
		}
	case SimpleVarietyList:
		if !validSimpleTypeRuntimeID(st.ListItem, limits) {
			return errors.New("list simple type references invalid list item")
		}
		if len(st.Union) != 0 {
			return errors.New("list simple type stores union members")
		}
	case SimpleVarietyUnion:
		if st.ListItem != NoSimpleType {
			return errors.New("union simple type stores list item")
		}
		if len(st.Union) == 0 {
			return errors.New("union simple type has no members")
		}
		for _, member := range st.Union {
			if !validSimpleTypeRuntimeID(member, limits) {
				return errors.New("simple type references invalid union member")
			}
		}
	default:
		return errors.New("simple type has invalid variety")
	}
	return nil
}

func validSimpleTypeRuntimeID(id SimpleTypeID, limits SimpleTypeRefLimits) bool {
	return ValidSimpleTypeID(id, limits.SimpleTypeCount)
}

// SimpleTypeRestrictionRequired reports whether a simple type is a
// user-defined restriction that must be checked against its base.
func SimpleTypeRestrictionRequired(id, base SimpleTypeID, builtins BuiltinIDs) bool {
	if base == NoSimpleType || base == builtins.AnySimpleType {
		return false
	}
	return int(id) >= BuiltinSimpleTypeCount()
}

// ValidateSimpleTypeRestrictionRuntime validates non-facet metadata that must
// be preserved by a simple-type restriction.
func ValidateSimpleTypeRestrictionRuntime(derived, base SimpleTypeRestrictionValidation) error {
	if derived.Variety != base.Variety ||
		derived.Primitive != base.Primitive ||
		derived.Builtin != base.Builtin ||
		derived.ListItem != base.ListItem ||
		!slices.Equal(derived.Union, base.Union) {
		return errors.New("simple type semantic fields do not match base restriction")
	}
	if !ValidWhitespaceRestriction(base.Whitespace, derived.Whitespace) {
		return errors.New("simple type whitespace loosens base restriction")
	}
	return nil
}

// SimpleTypeIdentityNode is the graph metadata needed to derive ID/IDREF behavior.
type SimpleTypeIdentityNode struct {
	Base     SimpleTypeID
	ListItem SimpleTypeID
	Variety  SimpleVariety
}

// NewSimpleTypeIdentityNodeForSimpleType returns the graph metadata needed to
// derive ID/IDREF behavior from a simple type.
func NewSimpleTypeIdentityNodeForSimpleType(st SimpleType) SimpleTypeIdentityNode {
	return SimpleTypeIdentityNode{
		Base:     st.Base,
		ListItem: st.ListItem,
		Variety:  st.Variety,
	}
}

// SimpleTypeIdentityRuntime supplies simple-type identity metadata by ID.
type SimpleTypeIdentityRuntime interface {
	SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool)
}

// DerivedSimpleIdentityForSimpleType computes a simple type's ID/IDREF
// behavior from its base or item type.
func DerivedSimpleIdentityForSimpleType(rt SimpleTypeIdentityRuntime, st SimpleType) SimpleIdentityKind {
	return DerivedSimpleIdentity(rt, NewSimpleTypeIdentityNodeForSimpleType(st))
}

// DerivedSimpleIdentity computes a simple type's ID/IDREF behavior from its base or item type.
func DerivedSimpleIdentity(rt SimpleTypeIdentityRuntime, node SimpleTypeIdentityNode) SimpleIdentityKind {
	switch node.Variety {
	case SimpleVarietyAtomic:
		if base, ok := rt.SimpleTypeIdentity(node.Base); ok {
			return base
		}
	case SimpleVarietyList:
		if item, ok := rt.SimpleTypeIdentity(node.ListItem); ok && item == SimpleIdentityIDREF {
			return SimpleIdentityIDREFList
		}
	case SimpleVarietyUnion:
	}
	return SimpleIdentityNone
}

// ExpectedSimpleIdentity reports the stored identity expected for a runtime simple type.
func ExpectedSimpleIdentity(
	rt SimpleTypeIdentityRuntime,
	builtins BuiltinIDs,
	id SimpleTypeID,
	node SimpleTypeIdentityNode,
) SimpleIdentityKind {
	if id == builtins.ID {
		return SimpleIdentityID
	}
	if id == builtins.IDREF {
		return SimpleIdentityIDREF
	}
	return DerivedSimpleIdentity(rt, node)
}

// ValidateSimpleTypeIdentity validates the stored ID/IDREF behavior for a
// runtime simple type.
func ValidateSimpleTypeIdentity(
	rt SimpleTypeIdentityRuntime,
	builtins BuiltinIDs,
	id SimpleTypeID,
	node SimpleTypeIdentityNode,
	stored SimpleIdentityKind,
) error {
	if stored != ExpectedSimpleIdentity(rt, builtins, id, node) {
		return errors.New("simple type identity does not match derivation")
	}
	return nil
}

// ValidateSimpleValuePayload validates cached simple-value identity payloads
// against the simple-type metadata that produced them.
func ValidateSimpleValuePayload(value SimpleValue, typ SimpleValuePayloadType) error {
	switch typ.Identity {
	case SimpleIdentityID:
		if value.IDs != value.Canonical || value.IDRefs != "" {
			return errors.New("ID payload does not match canonical value")
		}
	case SimpleIdentityIDREF, SimpleIdentityIDREFList:
		if value.IDs != "" || value.IDRefs != value.Canonical {
			return errors.New("IDREF payload does not match canonical value")
		}
	case SimpleIdentityNone:
		if value.IDs != "" || value.IDRefs != "" {
			return errors.New("stores ID payload for non-ID type")
		}
	default:
		return errors.New("stores invalid simple identity kind")
	}
	if typ.Variety == SimpleVarietyList {
		if !validSimpleListIdentityKey(value.Identity) {
			return errors.New("identity payload does not match canonical value")
		}
		return nil
	}
	identity, ok := expectedSimpleValueIdentity(typ, value.Canonical)
	if !ok || value.Identity != identity {
		return errors.New("identity payload does not match canonical value")
	}
	return nil
}

func validSimpleListIdentityKey(key string) bool {
	if len(key) < 2 || key[0] != byte(PrimitiveString) || key[1] != '\x1e' {
		return false
	}
	payload := key[2:]
	for payload != "" {
		separator := strings.IndexByte(payload, ':')
		if separator <= 0 || separator > 1 && payload[0] == '0' {
			return false
		}
		for i := range separator {
			if payload[i] < '0' || payload[i] > '9' {
				return false
			}
		}
		length, err := strconv.Atoi(payload[:separator])
		if err != nil || length < 0 || length > len(payload)-separator-1 {
			return false
		}
		payload = payload[separator+1:]
		item := payload[:length]
		if len(item) < 2 || !ValidPrimitiveKind(PrimitiveKind(item[0])) || item[1] != '\x1e' {
			return false
		}
		payload = payload[length:]
	}
	return true
}

func expectedSimpleValueIdentity(typ SimpleValuePayloadType, canonical string) (string, bool) {
	primitive := typ.Primitive
	if typ.Variety == SimpleVarietyList {
		primitive = PrimitiveString
	}
	switch primitive {
	case PrimitiveDecimal:
		value, err := ParseDecimalValue(canonical)
		if err != nil {
			return "", false
		}
		canonical = value.CanonicalText()
	case PrimitiveDuration:
		value, err := ParseDurationValue(canonical)
		if err != nil {
			return "", false
		}
		canonical = durationIdentityCanonical(value)
	default:
	}
	return SimpleIdentityKey(primitive, canonical), true
}

// SimpleIdentityKey builds the comparable identity key for a primitive value.
func SimpleIdentityKey(kind PrimitiveKind, canonical string) string {
	return identityKey(byte(kind), canonical)
}

// UntypedSimpleIdentityKey builds the comparable identity key for a value whose
// primitive type is not known to the caller.
func UntypedSimpleIdentityKey(canonical string) string {
	return identityKey(0xff, canonical)
}

func identityKey(kind byte, canonical string) string {
	var b strings.Builder
	b.Grow(2 + len(canonical))
	b.WriteByte(kind)
	b.WriteByte('\x1e')
	b.WriteString(canonical)
	return b.String()
}

// BooleanCanonical returns the XML Schema canonical form for a boolean value.
func BooleanCanonical(v bool) string {
	if v {
		return booleanCanonicalTrue
	}
	return booleanCanonicalFalse
}

type simpleTypeGraphState uint8

const (
	simpleTypeGraphUnchecked simpleTypeGraphState = iota
	simpleTypeGraphChecking
	simpleTypeGraphChecked
)

// ValidateSimpleTypeGraphForSimpleTypes validates simple-type base/list/union
// topology from runtime records.
func ValidateSimpleTypeGraphForSimpleTypes(types []SimpleType) error {
	state := make([]simpleTypeGraphState, len(types))
	reachesList := make([]bool, len(types))
	stack := make([]simpleTypeGraphFrame, 0, min(len(types), 1_024))
	for root := range types {
		if state[root] != simpleTypeGraphUnchecked {
			continue
		}
		state[root] = simpleTypeGraphChecking
		stack = appendDFSFrame(stack, simpleTypeGraphFrame{id: SimpleTypeID(root), next: -1}, len(types))
		for len(stack) != 0 {
			last := len(stack) - 1
			frame := &stack[last]
			st := types[frame.id]
			if frame.next < 0 {
				if st.Base != NoSimpleType {
					if !validSimpleTypeGraphID(types, st.Base) {
						return errors.New("simple type graph references invalid type")
					}
					switch state[st.Base] {
					case simpleTypeGraphChecking:
						return errors.New("simple type graph contains cycle")
					case simpleTypeGraphUnchecked:
						state[st.Base] = simpleTypeGraphChecking
						stack = appendDFSFrame(stack, simpleTypeGraphFrame{id: st.Base, next: -1}, len(types))
						continue
					case simpleTypeGraphChecked:
					}
				}
				frame.next = 0
			}

			switch st.Variety {
			case SimpleVarietyAtomic:
				state[frame.id] = simpleTypeGraphChecked
				stack = stack[:last]
			case SimpleVarietyList:
				if !validSimpleTypeGraphID(types, st.ListItem) {
					return errors.New("simple type graph references invalid type")
				}
				switch state[st.ListItem] {
				case simpleTypeGraphChecking:
					return errors.New("simple type graph contains cycle")
				case simpleTypeGraphUnchecked:
					state[st.ListItem] = simpleTypeGraphChecking
					stack = appendDFSFrame(stack, simpleTypeGraphFrame{id: st.ListItem, next: -1}, len(types))
					continue
				case simpleTypeGraphChecked:
				}
				if reachesList[st.ListItem] {
					return errors.New("list simple type uses list item type")
				}
				reachesList[frame.id] = true
				state[frame.id] = simpleTypeGraphChecked
				stack = stack[:last]
			case SimpleVarietyUnion:
				if frame.next == len(st.Union) {
					state[frame.id] = simpleTypeGraphChecked
					stack = stack[:last]
					continue
				}
				member := st.Union[frame.next]
				if err := validateUnionGraphMember(types, member); err != nil {
					return err
				}
				switch state[member] {
				case simpleTypeGraphChecking:
					return errors.New("simple type graph contains cycle")
				case simpleTypeGraphUnchecked:
					state[member] = simpleTypeGraphChecking
					stack = appendDFSFrame(stack, simpleTypeGraphFrame{id: member, next: -1}, len(types))
					continue
				case simpleTypeGraphChecked:
				}
				reachesList[frame.id] = reachesList[frame.id] || reachesList[member]
				frame.next++
			default:
				return errors.New("simple type graph has invalid variety")
			}
		}
	}
	return nil
}

func validateUnionGraphMember(types []SimpleType, member SimpleTypeID) error {
	if !validSimpleTypeGraphID(types, member) {
		return errors.New("simple type graph references invalid type")
	}
	if types[member].Variety == SimpleVarietyUnion {
		return errors.New("union simple type references unflattened union member")
	}
	return nil
}

type simpleTypeGraphFrame struct {
	id   SimpleTypeID
	next int
}

func appendDFSFrame[T any](stack []T, frame T, limit int) []T {
	if len(stack) < cap(stack) {
		return append(stack, frame)
	}
	if len(stack) >= limit {
		panic("DFS stack exceeds graph size")
	}
	newCapacity := limit
	if cap(stack) <= limit/2 {
		newCapacity = max(1, cap(stack)*2)
	}
	grown := make([]T, len(stack), newCapacity)
	copy(grown, stack)
	return append(grown, frame)
}

func validSimpleTypeGraphID(types []SimpleType, id SimpleTypeID) bool {
	return ValidSimpleTypeID(id, len(types))
}
