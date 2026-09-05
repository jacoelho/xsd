package runtime

import (
	"errors"
	"slices"

	"github.com/jacoelho/xsd/internal/lex"
)

// ValueConstraint is a prevalidated default or fixed value constraint.
// A nil *ValueConstraint means absent. Once attached to a runtime component,
// the value is immutable; declarations and uses may share the same pointer.
type ValueConstraint struct {
	ResolvedNames []ResolvedValueName
	Lexical       string
	Canonical     string
	Value         SimpleValue
}

// ValueConstraintValidation is the runtime projection needed to validate cached
// value-constraint shape and owner type references.
type ValueConstraintValidation struct {
	Lexical          string
	Canonical        string
	Value            SimpleValue
	HasResolvedNames bool
}

// ValueConstraintRead exposes prevalidated default/fixed value data to
// validation without exposing compiler-owned pointer storage.
type ValueConstraintRead struct {
	lexical   string
	canonical string
	value     SimpleValue
}

// newValueConstraintRead returns the immutable validation read projection for
// one prevalidated value constraint.
func newValueConstraintRead(lexical, canonical string, value SimpleValue) ValueConstraintRead {
	return ValueConstraintRead{
		lexical:   lexical,
		canonical: canonical,
		value:     value,
	}
}

// newValueConstraintReadFromConstraint returns the immutable validation read
// projection for one prevalidated value constraint.
func newValueConstraintReadFromConstraint(vc *ValueConstraint) (ValueConstraintRead, bool) {
	if vc == nil {
		return ValueConstraintRead{}, false
	}
	return newValueConstraintRead(vc.Lexical, vc.Canonical, vc.Value), true
}

// equalValueConstraintReads reports whether two value-constraint read
// projections expose the same validation-facing value.
func equalValueConstraintReads(a, b ValueConstraintRead) bool {
	return a.lexical == b.lexical &&
		a.canonical == b.canonical &&
		a.value == b.value
}

// LexicalText returns the source lexical value.
func (v ValueConstraintRead) LexicalText() string {
	return v.lexical
}

// CanonicalText returns the canonical text used for fixed-value comparison.
func (v ValueConstraintRead) CanonicalText() string {
	return v.canonical
}

// SimpleValue returns the cached simple value.
func (v ValueConstraintRead) SimpleValue() SimpleValue {
	return v.value
}

// FixedAttributeComparison identifies the equality relation for a fixed
// attribute constraint.
type FixedAttributeComparison uint8

const (
	// FixedAttributeComparisonInvalid is not a valid equality relation.
	FixedAttributeComparisonInvalid FixedAttributeComparison = iota
	// FixedAttributeComparisonLexical compares canonical lexical values.
	FixedAttributeComparisonLexical
	// FixedAttributeComparisonValueSpace compares datatype identity values.
	FixedAttributeComparisonValueSpace
)

// FixedAttributeValueEqual compares an attribute value with a fixed
// constraint. valid is false for invalid comparison modes or when a
// value-space comparison lacks its precomputed identity projection.
func FixedAttributeValueEqual(actual SimpleValue, fixed ValueConstraintRead, comparison FixedAttributeComparison) (equal, valid bool) {
	switch comparison {
	case FixedAttributeComparisonLexical:
		return actual.Canonical == fixed.canonical, true
	case FixedAttributeComparisonValueSpace:
	case FixedAttributeComparisonInvalid:
		return false, false
	default:
		valid := false
		return false, valid
	}
	if actual.Identity == "" || fixed.value.Identity == "" {
		return false, false
	}
	return actual.Identity == fixed.value.Identity, true
}

// ElementValueConstraints exposes prevalidated default/fixed values attached to
// one element declaration.
type ElementValueConstraints struct {
	fixed        ValueConstraintRead
	defaultValue ValueConstraintRead
	owner        TypeID
	hasFixed     bool
	hasDefault   bool
}

// elementValueConstraintShape is the runtime-read projection source for
// one element declaration's value constraints.
type elementValueConstraintShape struct {
	Fixed      ValueConstraintRead
	Default    ValueConstraintRead
	Owner      TypeID
	HasFixed   bool
	HasDefault bool
}

// newElementValueConstraints returns the immutable validation read projection
// for an element declaration's value constraints.
func newElementValueConstraints(owner TypeID, fixed ValueConstraintRead, hasFixed bool, def ValueConstraintRead, hasDefault bool) ElementValueConstraints {
	return ElementValueConstraints{
		owner:        owner,
		fixed:        fixed,
		defaultValue: def,
		hasFixed:     hasFixed,
		hasDefault:   hasDefault,
	}
}

// equalElementValueConstraints reports whether two element value-constraint
// projections expose the same validation-facing constraints.
func equalElementValueConstraints(a, b ElementValueConstraints) bool {
	if a.owner != b.owner || a.hasFixed != b.hasFixed || a.hasDefault != b.hasDefault {
		return false
	}
	if a.hasFixed && !equalValueConstraintReads(a.fixed, b.fixed) {
		return false
	}
	return !a.hasDefault || equalValueConstraintReads(a.defaultValue, b.defaultValue)
}

// OwnerType returns the declaration type that validated the cached value.
func (c ElementValueConstraints) OwnerType() TypeID {
	return c.owner
}

// HasAny reports whether either default or fixed is present.
func (c ElementValueConstraints) HasAny() bool {
	return c.hasFixed || c.hasDefault
}

// FixedValue returns the fixed value, if present.
func (c ElementValueConstraints) FixedValue() (ValueConstraintRead, bool) {
	return c.fixed, c.hasFixed
}

// DefaultValueConstraint returns the default value, if present.
func (c ElementValueConstraints) DefaultValueConstraint() (ValueConstraintRead, bool) {
	return c.defaultValue, c.hasDefault
}

func elementValueConstraintReadShape(decl ElementDecl) elementValueConstraintShape {
	fixed, hasFixed := newValueConstraintReadFromConstraint(decl.Fixed)
	def, hasDefault := newValueConstraintReadFromConstraint(decl.Default)
	return elementValueConstraintShape{
		Owner:      decl.Type,
		Fixed:      fixed,
		Default:    def,
		HasFixed:   hasFixed,
		HasDefault: hasDefault,
	}
}

// ResolvedValueName records one QName resolution proof entry captured while
// validating a value constraint.
type ResolvedValueName struct {
	Lexical string
	NS      string
	Local   string
}

// ValueConstraintNameReplay replays QName resolution proofs captured while
// validating a value constraint.
type ValueConstraintNameReplay struct {
	entries []ResolvedValueName
	used    []bool
}

// ValueConstraintQNameResolver resolves one lexical QName while replaying a
// value-constraint validation proof.
type ValueConstraintQNameResolver func(string) (ns, local string, ok bool)

// ValueConstraintSimpleValidator revalidates value-constraint lexical text
// against an owner simple type.
type ValueConstraintSimpleValidator func(SimpleTypeID, string, ValueConstraintQNameResolver, SimpleValueNeed) (SimpleValue, error)

// NewValueConstraintNameReplay validates resolved QName proof entries and
// returns replay state for datatype validation.
func NewValueConstraintNameReplay(entries []ResolvedValueName) (ValueConstraintNameReplay, error) {
	seen := make(map[string]ResolvedValueName, len(entries))
	for _, entry := range entries {
		parts := resolvedValueNameLexicalParts(entry.Lexical)
		if !parts.Valid || entry.Local != parts.Local || !lex.IsNCName(entry.Local) {
			return ValueConstraintNameReplay{}, errors.New("resolved name proof is not deterministic")
		}
		prev, ok := seen[entry.Lexical]
		if !ok {
			seen[entry.Lexical] = entry
			continue
		}
		if prev.NS != entry.NS || prev.Local != entry.Local {
			return ValueConstraintNameReplay{}, errors.New("resolved name proof is not deterministic")
		}
	}
	return ValueConstraintNameReplay{
		entries: entries,
		used:    make([]bool, len(entries)),
	}, nil
}

// ResolveQName replays one captured QName resolution.
func (r *ValueConstraintNameReplay) ResolveQName(lexical string) (namespace, local string, ok bool) {
	parts := resolvedValueNameLexicalParts(lexical)
	if !parts.Valid || parts.Prefixed && parts.Prefix == "" {
		return "", "", false
	}
	for i, resolved := range r.entries {
		if r.used[i] || resolved.Lexical != lexical {
			continue
		}
		if resolved.Local != parts.Local || !lex.IsNCName(resolved.Local) {
			return "", "", false
		}
		r.used[i] = true
		return resolved.NS, resolved.Local, true
	}
	return "", "", false
}

// ValidateConsumed validates that datatype replay consumed every captured name
// resolution proof entry.
func (r *ValueConstraintNameReplay) ValidateConsumed() error {
	for _, used := range r.used {
		if !used {
			return errors.New("resolved name proof was not fully consumed")
		}
	}
	return nil
}

func resolvedValueNameLexicalParts(lexical string) lex.QNameParts {
	trimmed := lex.TrimXMLWhitespaceString(lexical)
	if trimmed == "" {
		return lex.QNameParts{}
	}
	return lex.SplitQName(trimmed)
}

// ValueConstraintIdentity is the equality projection used when runtime rules
// must prove that an inherited value constraint was preserved unchanged.
type ValueConstraintIdentity struct {
	ResolvedNames []ResolvedValueName
	Lexical       string
	Canonical     string
	Value         SimpleValue
	Present       bool
}

// NewValueConstraintIdentity returns the equality projection for a
// prevalidated value constraint.
func NewValueConstraintIdentity(vc *ValueConstraint) ValueConstraintIdentity {
	if vc == nil {
		return ValueConstraintIdentity{}
	}
	return CloneValueConstraintIdentity(ValueConstraintIdentity{
		ResolvedNames: vc.ResolvedNames,
		Lexical:       vc.Lexical,
		Canonical:     vc.Canonical,
		Value:         vc.Value,
		Present:       true,
	})
}

// ValueConstraintIdentityEqual reports whether two value-constraint identity
// projections preserve the same validated constraint.
func ValueConstraintIdentityEqual(a, b ValueConstraintIdentity) bool {
	if a.Present != b.Present {
		return false
	}
	if !a.Present {
		return true
	}
	return a.Lexical == b.Lexical &&
		a.Canonical == b.Canonical &&
		a.Value == b.Value &&
		slices.Equal(a.ResolvedNames, b.ResolvedNames)
}

// FixedValueConstraintEqual reports whether two fixed value constraints carry
// the same actual value. Exact lexical preservation is checked separately by
// ValueConstraintIdentityEqual.
func FixedValueConstraintEqual(base, derived ValueConstraintIdentity) bool {
	if base.Present != derived.Present {
		return false
	}
	if !base.Present {
		return true
	}
	if base.Value.Type == NoSimpleType || derived.Value.Type == NoSimpleType {
		return base.Canonical == derived.Canonical
	}
	if base.Value.Identity != "" && derived.Value.Identity != "" {
		return base.Value.Identity == derived.Value.Identity
	}
	return base.Value == derived.Value
}

// NewValueConstraintValidation returns the runtime validation projection for a
// prevalidated value constraint.
func NewValueConstraintValidation(vc *ValueConstraint) ValueConstraintValidation {
	if vc == nil {
		return ValueConstraintValidation{}
	}
	return ValueConstraintValidation{
		Lexical:          vc.Lexical,
		Canonical:        vc.Canonical,
		Value:            vc.Value,
		HasResolvedNames: len(vc.ResolvedNames) != 0,
	}
}

// ValueConstraintSimpleType is the simple-type projection needed for cached
// value-constraint owner matching.
type ValueConstraintSimpleType struct {
	Union          []SimpleTypeID
	ListItem       SimpleTypeID
	Variety        SimpleVariety
	Primitive      PrimitiveKind
	HasEnumeration bool
}

// NewValueConstraintSimpleTypeForSimpleType projects a runtime simple type
// into the shape needed for cached value-constraint owner matching.
func NewValueConstraintSimpleTypeForSimpleType(st SimpleType) ValueConstraintSimpleType {
	return CloneValueConstraintSimpleType(ValueConstraintSimpleType{
		Union:          st.Union,
		ListItem:       st.ListItem,
		Variety:        st.Variety,
		Primitive:      st.Primitive,
		HasEnumeration: len(st.Facets.Enumeration) != 0,
	})
}

// ValueConstraintComplexType is the complex-type projection needed to
// determine the owner type for an element value constraint.
type ValueConstraintComplexType struct {
	Content     ContentModelID
	TextType    SimpleTypeID
	ContentKind ContentKind
}

// NewValueConstraintComplexTypeForComplexType projects a runtime complex type
// into the shape needed to determine an element value-constraint owner.
func NewValueConstraintComplexTypeForComplexType(ct ComplexType) ValueConstraintComplexType {
	return ValueConstraintComplexType{
		Content:     ct.Content,
		TextType:    ct.TextType,
		ContentKind: ct.ContentKind,
	}
}

// ValueConstraintRuntime supplies simple-type metadata needed for value
// constraint shape validation.
type ValueConstraintRuntime interface {
	ValueConstraintSimpleType(id SimpleTypeID) (ValueConstraintSimpleType, bool)
}

// ElementValueConstraintRuntime supplies runtime metadata needed to determine
// the owner type for an element value constraint.
type ElementValueConstraintRuntime interface {
	ContentModelRuntime
	ValueConstraintComplexType(id ComplexTypeID) (ValueConstraintComplexType, bool)
}

// ElementValueConstraintType returns the simple type that must own an element's
// value constraint. NoSimpleType means the constraint is allowed only as mixed
// lexical text.
func ElementValueConstraintType(
	rt ElementValueConstraintRuntime,
	analysis *ContentModelAnalysis,
	typ TypeID,
) (SimpleTypeID, error) {
	if id, ok := typ.Simple(); ok {
		return id, nil
	}
	id, ok := typ.Complex()
	if !ok || rt == nil {
		return NoSimpleType, errors.New("element value constraint references invalid type")
	}
	ct, ok := rt.ValueConstraintComplexType(id)
	if !ok {
		return NoSimpleType, errors.New("element value constraint references invalid type")
	}
	switch {
	case ct.ContentKind.Simple():
		return ct.TextType, nil
	case ct.ContentKind.Mixed():
		return mixedElementValueConstraintType(analysis, ct.Content)
	default:
		return NoSimpleType, errors.New("element value constraint requires simple content")
	}
}

func mixedElementValueConstraintType(analysis *ContentModelAnalysis, content ContentModelID) (SimpleTypeID, error) {
	if analysis == nil {
		return NoSimpleType, errors.New("content model analysis is nil")
	}
	emptiable, err := analysis.ModelEmptiable(content)
	if err != nil {
		return NoSimpleType, err
	}
	if !emptiable {
		return NoSimpleType, errors.New("element value constraint requires simple content")
	}
	return NoSimpleType, nil
}

// ValidateValueConstraintShape validates cached value-constraint metadata that
// is independent of datatype replay.
func ValidateValueConstraintShape(rt ValueConstraintRuntime, vc ValueConstraintValidation, expected SimpleTypeID) error {
	if vc.Value.Canonical != vc.Canonical {
		return errors.New("canonical value mismatch")
	}
	if expected == NoSimpleType {
		if vc.Value.Type != NoSimpleType ||
			vc.Canonical != vc.Lexical ||
			vc.Value.IDs != "" ||
			vc.Value.IDRefs != "" ||
			vc.Value.Identity != "" ||
			vc.HasResolvedNames {
			return errors.New("mixed value constraint is not untyped lexical text")
		}
		return nil
	}
	if rt == nil {
		return errors.New("value type does not match owner type")
	}
	actual, ok := rt.ValueConstraintSimpleType(vc.Value.Type)
	if !ok {
		return errors.New("value type does not match owner type")
	}
	if actual.Variety == SimpleVarietyUnion {
		return errors.New("stores union owner as simple value type")
	}
	if !valueConstraintTypeMatches(rt, expected, vc.Value.Type, make(map[SimpleTypeID]bool)) {
		return errors.New("value type does not match owner type")
	}
	return nil
}

// ValidateValueConstraintReplayResult validates that datatype replay reproduced
// the cached simple value.
func ValidateValueConstraintReplayResult(cached ValueConstraintValidation, replayed SimpleValue) error {
	if replayed != cached.Value {
		return errors.New("cached value does not match replayed validation")
	}
	return nil
}

// ValidateValueConstraintReplay replays the captured datatype validation proof
// for a cached value constraint and verifies that replay reproduces the cached
// value exactly.
func ValidateValueConstraintReplay(cached ValueConstraintValidation, expected SimpleTypeID, names []ResolvedValueName, validate ValueConstraintSimpleValidator) error {
	if validate == nil {
		return errors.New("missing value constraint validator")
	}
	replay, err := NewValueConstraintNameReplay(names)
	if err != nil {
		return err
	}
	value, err := validate(expected, cached.Lexical, replay.ResolveQName, SimpleNeedCanonical|SimpleNeedIdentity)
	if err != nil {
		return errors.New("lexical value no longer validates against owner type")
	}
	if err := replay.ValidateConsumed(); err != nil {
		return err
	}
	return ValidateValueConstraintReplayResult(cached, value)
}

// SimpleTypeUsesBareNotation reports whether a simple type graph contains
// xs:NOTATION without an enumeration facet.
func SimpleTypeUsesBareNotation(rt ValueConstraintRuntime, id SimpleTypeID) bool {
	return hasBareNotationUse(rt, id, make(map[SimpleTypeID]bool))
}

func hasBareNotationUse(rt ValueConstraintRuntime, id SimpleTypeID, seen map[SimpleTypeID]bool) bool {
	if rt == nil || id == NoSimpleType || seen[id] {
		return false
	}
	seen[id] = true
	st, ok := rt.ValueConstraintSimpleType(id)
	if !ok {
		return false
	}
	if st.Primitive == PrimitiveNotation && !st.HasEnumeration {
		return true
	}
	switch st.Variety {
	case SimpleVarietyList:
		return hasBareNotationUse(rt, st.ListItem, seen)
	case SimpleVarietyUnion:
		return unionUsesBareNotation(rt, st.Union, seen)
	case SimpleVarietyAtomic:
		return false
	default:
	}
	return false
}

func unionUsesBareNotation(rt ValueConstraintRuntime, members []SimpleTypeID, seen map[SimpleTypeID]bool) bool {
	for _, member := range members {
		if hasBareNotationUse(rt, member, seen) {
			return true
		}
	}
	return false
}

func valueConstraintTypeMatches(rt ValueConstraintRuntime, expected, actual SimpleTypeID, seen map[SimpleTypeID]bool) bool {
	if seen[expected] {
		return false
	}
	seen[expected] = true
	st, ok := rt.ValueConstraintSimpleType(expected)
	if !ok {
		return false
	}
	if st.Variety != SimpleVarietyUnion {
		return actual == expected
	}
	for _, member := range st.Union {
		if valueConstraintTypeMatches(rt, member, actual, seen) {
			return true
		}
	}
	return false
}
