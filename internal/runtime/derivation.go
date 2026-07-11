package runtime

import (
	"errors"
	"slices"
)

// SimpleTypeDerivation is the graph metadata needed to test simple-type derivation.
type SimpleTypeDerivation struct {
	Union   []SimpleTypeID
	Base    SimpleTypeID
	Variety SimpleVariety
}

// NewSimpleTypeDerivationForSimpleType returns the runtime derivation
// projection for one simple type.
func NewSimpleTypeDerivationForSimpleType(st SimpleType) SimpleTypeDerivation {
	return CloneSimpleTypeDerivation(SimpleTypeDerivation{
		Union:   st.Union,
		Base:    st.Base,
		Variety: st.Variety,
	})
}

// EqualSimpleTypeDerivationForSimpleType reports whether projection exposes
// the runtime derivation facts for st.
func EqualSimpleTypeDerivationForSimpleType(projection SimpleTypeDerivation, st SimpleType) bool {
	return projection.Base == st.Base &&
		projection.Variety == st.Variety &&
		slices.Equal(projection.Union, st.Union)
}

// ComplexTypeDerivation is the graph metadata needed to test complex-type derivation.
type ComplexTypeDerivation struct {
	Base  TypeID
	Kind  DerivationKind
	Block DerivationMask
}

// NewComplexTypeDerivationForComplexType returns the runtime derivation
// projection for one complex type.
func NewComplexTypeDerivationForComplexType(ct ComplexType) ComplexTypeDerivation {
	return ComplexTypeDerivation{
		Base:  ct.Base,
		Kind:  ct.Derivation,
		Block: ct.Block,
	}
}

// EqualComplexTypeDerivations reports whether two complex-type derivation
// projections expose the same runtime derivation graph node.
func EqualComplexTypeDerivations(a, b ComplexTypeDerivation) bool {
	return a == b
}

// EqualComplexTypeDerivationForComplexType reports whether projection exposes
// the runtime derivation facts for ct.
func EqualComplexTypeDerivationForComplexType(projection ComplexTypeDerivation, ct ComplexType) bool {
	return EqualComplexTypeDerivations(projection, NewComplexTypeDerivationForComplexType(ct))
}

// TypeDerivationRead is the freeze-published type-derivation graph used by
// validation-time derivation traversal.
type TypeDerivationRead struct {
	simple  []SimpleTypeDerivation
	complex []ComplexTypeDerivation
	anyType ComplexTypeID
}

// NewBorrowedTypeDerivationReadForTypes returns a derivation graph that borrows
// union-member slices from compiler state transferred to an immutable schema.
func NewBorrowedTypeDerivationReadForTypes(anyType ComplexTypeID, simpleTypes []SimpleType, complexTypes []ComplexType) TypeDerivationRead {
	simple := make([]SimpleTypeDerivation, len(simpleTypes))
	for i, st := range simpleTypes {
		simple[i] = SimpleTypeDerivation{
			Union:   st.Union,
			Base:    st.Base,
			Variety: st.Variety,
		}
	}
	complexDerivations := make([]ComplexTypeDerivation, len(complexTypes))
	for i, ct := range complexTypes {
		complexDerivations[i] = NewComplexTypeDerivationForComplexType(ct)
	}
	return TypeDerivationRead{
		simple:  simple,
		complex: complexDerivations,
		anyType: anyType,
	}
}

// AnyTypeID returns the complex type ID of xs:anyType.
func (r TypeDerivationRead) AnyTypeID() ComplexTypeID {
	return r.anyType
}

// SimpleTypeCount returns the number of simple-type derivation nodes.
func (r TypeDerivationRead) SimpleTypeCount() int {
	return len(r.simple)
}

// ComplexTypeCount returns the number of complex-type derivation nodes.
func (r TypeDerivationRead) ComplexTypeCount() int {
	return len(r.complex)
}

func (r TypeDerivationRead) simpleTypeDerivation(id SimpleTypeID) (SimpleTypeDerivation, bool) {
	if !ValidSimpleTypeID(id, len(r.simple)) {
		return SimpleTypeDerivation{}, false
	}
	return r.simple[id], true
}

// ComplexTypeDerivation returns graph metadata for complex-type derivation
// traversal.
func (r TypeDerivationRead) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	if !ValidComplexTypeID(id, len(r.complex)) {
		return ComplexTypeDerivation{}, false
	}
	return r.complex[id], true
}

type typeDerivationReadRuntime struct {
	read TypeDerivationRead
}

func (r typeDerivationReadRuntime) AnyTypeID() ComplexTypeID {
	return r.read.AnyTypeID()
}

func (r typeDerivationReadRuntime) SimpleTypeCount() int {
	return r.read.SimpleTypeCount()
}

func (r typeDerivationReadRuntime) ComplexTypeCount() int {
	return r.read.ComplexTypeCount()
}

func (r typeDerivationReadRuntime) SimpleTypeDerivation(id SimpleTypeID) (SimpleTypeDerivation, bool) {
	return r.read.simpleTypeDerivation(id)
}

func (r typeDerivationReadRuntime) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	return r.read.ComplexTypeDerivation(id)
}

// EqualSimpleTypeDerivationReadProjectionForTypes reports whether read exposes
// the simple-type derivation graph for simpleTypes.
func EqualSimpleTypeDerivationReadProjectionForTypes(read TypeDerivationRead, simpleTypes []SimpleType) bool {
	if read.SimpleTypeCount() != len(simpleTypes) {
		return false
	}
	for i := range simpleTypes {
		if !EqualSimpleTypeDerivationForSimpleType(read.simple[i], simpleTypes[i]) {
			return false
		}
	}
	return true
}

// EqualComplexTypeDerivationReadProjection reports whether read exposes the
// complex-type derivation graph for complexTypes.
func EqualComplexTypeDerivationReadProjection(read TypeDerivationRead, complexTypes []ComplexType) bool {
	if read.ComplexTypeCount() != len(complexTypes) {
		return false
	}
	for i := range complexTypes {
		if !EqualComplexTypeDerivationForComplexType(read.complex[i], complexTypes[i]) {
			return false
		}
	}
	return true
}

// ValidateTypeDerivationReadProjection validates type-derivation read metadata
// against frozen simple and complex type records.
func ValidateTypeDerivationReadProjection(read TypeDerivationRead, anyType ComplexTypeID, simpleTypes []SimpleType, complexTypes []ComplexType) error {
	if read.AnyTypeID() != anyType {
		return errors.New("type derivation projection stores invalid anyType")
	}
	if read.SimpleTypeCount() != len(simpleTypes) {
		return errors.New("simple type derivation projection count does not match types")
	}
	if read.ComplexTypeCount() != len(complexTypes) {
		return errors.New("complex type derivation projection count does not match types")
	}
	if !EqualSimpleTypeDerivationReadProjectionForTypes(read, simpleTypes) {
		return errors.New("simple type derivation projection does not match type")
	}
	if !EqualComplexTypeDerivationReadProjection(read, complexTypes) {
		return errors.New("complex type derivation projection does not match type")
	}
	return nil
}

// TypeDerivationRuntime supplies runtime type-derivation graph metadata.
type TypeDerivationRuntime interface {
	AnyTypeID() ComplexTypeID
	SimpleTypeCount() int
	ComplexTypeCount() int
	SimpleTypeDerivation(id SimpleTypeID) (SimpleTypeDerivation, bool)
	ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool)
}

// TypeDerivationMask reports the derivation steps used by derived to derive from base.
func TypeDerivationMask[T TypeDerivationRuntime](rt T, derived, base TypeID) (DerivationMask, bool) {
	if derived == base {
		return 0, true
	}
	if base == ComplexRef(rt.AnyTypeID()) {
		if id, ok := derived.Complex(); ok {
			return complexAnyTypeDerivationMask(rt, id)
		}
		return DerivationRestriction, true
	}
	if derivedID, ok := derived.Complex(); ok {
		if baseID, ok := base.Simple(); ok {
			return complexSimpleTypeDerivationMask(rt, derivedID, baseID)
		}
		if baseID, ok := base.Complex(); ok {
			return complexTypeDerivationMask(rt, derivedID, baseID)
		}
		return 0, false
	}
	if derivedID, ok := derived.Simple(); ok {
		if baseID, ok := base.Simple(); ok {
			return simpleTypeDerivationMaskOf(rt, derivedID, baseID)
		}
	}
	return 0, false
}

// SubstitutionDerivationAllowed reports whether derived may substitute for base after block constraints.
func SubstitutionDerivationAllowed(rt TypeDerivationRuntime, derived, base TypeID, block DerivationMask) bool {
	mask, ok := TypeDerivationMask(rt, derived, base)
	if !ok {
		return false
	}
	if mask&block != 0 {
		return false
	}
	return mask&substitutionTypeBlocks(rt, derived, base) == 0
}

func substitutionTypeBlocks(rt TypeDerivationRuntime, derived, base TypeID) DerivationMask {
	var blocks DerivationMask
	if baseID, ok := base.Complex(); ok {
		if baseCT, ok := rt.ComplexTypeDerivation(baseID); ok {
			blocks |= baseCT.Block
		}
	}
	current, ok := derived.Complex()
	if !ok {
		return blocks
	}
	for range rt.ComplexTypeCount() {
		ct, ok := rt.ComplexTypeDerivation(current)
		if !ok {
			return blocks
		}
		if ct.Base == base {
			return blocks
		}
		parent, ok := ct.Base.Complex()
		if !ok {
			return blocks
		}
		parentCT, ok := rt.ComplexTypeDerivation(parent)
		if !ok {
			return blocks
		}
		blocks |= parentCT.Block
		current = parent
	}
	return blocks
}

func complexSimpleTypeDerivationMask[T TypeDerivationRuntime](rt T, derived ComplexTypeID, base SimpleTypeID) (DerivationMask, bool) {
	var mask DerivationMask
	for range rt.ComplexTypeCount() {
		ct, ok := rt.ComplexTypeDerivation(derived)
		if !ok {
			return 0, false
		}
		switch ct.Kind {
		case DerivationKindExtension:
			mask |= DerivationExtension
		case DerivationKindRestriction:
			mask |= DerivationRestriction
		case DerivationKindNone:
		}
		if baseSimple, simple := ct.Base.Simple(); simple {
			simpleMask, found := simpleTypeDerivationMaskOf(rt, baseSimple, base)
			if !found {
				return 0, false
			}
			return mask | simpleMask, true
		}
		baseComplex, ok := ct.Base.Complex()
		if !ok {
			return 0, false
		}
		derived = baseComplex
	}
	return 0, false
}

func complexAnyTypeDerivationMask[T TypeDerivationRuntime](rt T, derived ComplexTypeID) (DerivationMask, bool) {
	var mask DerivationMask
	for range rt.ComplexTypeCount() {
		if derived == rt.AnyTypeID() {
			return mask, true
		}
		ct, ok := rt.ComplexTypeDerivation(derived)
		if !ok {
			return 0, false
		}
		switch ct.Kind {
		case DerivationKindExtension:
			mask |= DerivationExtension
		case DerivationKindRestriction:
			mask |= DerivationRestriction
		case DerivationKindNone:
		}
		if ct.Base.Kind == TypeSimple {
			return mask | DerivationRestriction, true
		}
		parent, ok := ct.Base.Complex()
		if !ok {
			return 0, false
		}
		derived = parent
	}
	return 0, false
}

func simpleTypeDerivationMaskOf[T TypeDerivationRuntime](
	rt T,
	derived, base SimpleTypeID,
) (DerivationMask, bool) {
	if derived == base {
		return 0, true
	}
	st, ok := rt.SimpleTypeDerivation(derived)
	if !ok {
		return 0, false
	}
	baseType, ok := rt.SimpleTypeDerivation(base)
	if !ok {
		return 0, false
	}
	if baseType.Variety != SimpleVarietyUnion {
		return simpleTypeBaseChainDerivationMask(rt, derived, base, st)
	}
	return simpleTypeUnionDerivationMask(rt, derived, base)
}

func simpleTypeBaseChainDerivationMask[T TypeDerivationRuntime](
	rt T,
	derived, base SimpleTypeID,
	st SimpleTypeDerivation,
) (DerivationMask, bool) {
	anchor := derived
	power, distance := uint64(1), uint64(0)
	for {
		if st.Base == NoSimpleType || st.Base == derived {
			return 0, false
		}
		derived = st.Base
		if derived == base {
			return DerivationRestriction, true
		}
		distance++
		if derived == anchor {
			return 0, false
		}
		if distance == power {
			anchor = derived
			power *= 2
			distance = 0
		}
		var ok bool
		st, ok = rt.SimpleTypeDerivation(derived)
		if !ok {
			return 0, false
		}
	}
}

type simpleTypeDerivationFrame struct {
	derived SimpleTypeID
	base    SimpleTypeID
	next    int
	entered bool
}

func simpleTypeUnionDerivationMask[T TypeDerivationRuntime](rt T, derived, base SimpleTypeID) (DerivationMask, bool) {
	limit := simpleTypeDerivationPairLimit(rt.SimpleTypeCount())
	stack := make([]simpleTypeDerivationFrame, 0, min(limit, 1_024))
	stack = appendDFSFrame(stack, simpleTypeDerivationFrame{derived: derived, base: base}, limit)
	seen := make(map[[2]SimpleTypeID]bool)
	for len(stack) != 0 {
		last := len(stack) - 1
		frame := &stack[last]
		if frame.derived == frame.base {
			return DerivationRestriction, true
		}
		st, ok := rt.SimpleTypeDerivation(frame.derived)
		if !ok {
			stack = stack[:last]
			continue
		}
		baseType, ok := rt.SimpleTypeDerivation(frame.base)
		if !ok {
			stack = stack[:last]
			continue
		}
		if !frame.entered {
			pair := [2]SimpleTypeID{frame.derived, frame.base}
			if seen[pair] {
				stack = stack[:last]
				continue
			}
			seen[pair] = true
			frame.entered = true
		}
		memberCount := 0
		if baseType.Variety == SimpleVarietyUnion {
			memberCount = len(baseType.Union)
		}
		if frame.next < memberCount {
			member := baseType.Union[frame.next]
			frame.next++
			if frame.derived == member {
				return DerivationRestriction, true
			}
			if seen[[2]SimpleTypeID{frame.derived, member}] {
				continue
			}
			stack = appendDFSFrame(stack, simpleTypeDerivationFrame{derived: frame.derived, base: member}, limit)
			continue
		}
		if frame.next == memberCount {
			frame.next++
			if st.Base != NoSimpleType && st.Base != frame.derived {
				if st.Base == frame.base {
					return DerivationRestriction, true
				}
				if seen[[2]SimpleTypeID{st.Base, frame.base}] {
					continue
				}
				stack = appendDFSFrame(stack, simpleTypeDerivationFrame{derived: st.Base, base: frame.base}, limit)
				continue
			}
		}
		stack = stack[:last]
	}
	return 0, false
}

func simpleTypeDerivationPairLimit(count int) int {
	if count <= 0 {
		return int(^uint(0) >> 1)
	}
	maxInt := int(^uint(0) >> 1)
	if count > maxInt/count {
		return maxInt
	}
	return count * count
}

func complexTypeDerivationMask[T TypeDerivationRuntime](rt T, derived, base ComplexTypeID) (DerivationMask, bool) {
	var mask DerivationMask
	for range rt.ComplexTypeCount() {
		ct, ok := rt.ComplexTypeDerivation(derived)
		if !ok {
			return 0, false
		}
		parent, ok := ct.Base.Complex()
		if !ok {
			return 0, false
		}
		switch ct.Kind {
		case DerivationKindExtension:
			mask |= DerivationExtension
		case DerivationKindRestriction:
			mask |= DerivationRestriction
		case DerivationKindNone:
		}
		if parent == base {
			return mask, true
		}
		derived = parent
	}
	return 0, false
}
