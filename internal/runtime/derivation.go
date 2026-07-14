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

// TypeDerivationRead is the freeze-published type-derivation index used by
// validation-time derivation traversal. Simple union edges are owned once by
// simpleTypes and shared with published simple-value validation.
type TypeDerivationRead struct {
	index *typeDerivationIndex
}

type typeDerivationIndex struct {
	simpleTypes       *simpleTypeColdReadTable
	simpleIn          []uint32
	simpleOut         []uint32
	complexIn         []uint32
	complexOut        []uint32
	complexExtensions []uint32
	complexRestricts  []uint32
	complexSimpleBase []SimpleTypeID
	complexSimpleMask []DerivationMask
	anyType           ComplexTypeID
}

func newTypeDerivationReadForTypes(
	anyType ComplexTypeID,
	simpleTypes []SimpleType,
	complexTypes []ComplexType,
	reads *simpleTypeColdReadTable,
) (TypeDerivationRead, error) {
	if !ValidComplexTypeID(anyType, len(complexTypes)) {
		return TypeDerivationRead{}, errors.New("type derivation projection stores invalid anyType")
	}
	if reads == nil || len(reads.index) != len(simpleTypes) {
		return TypeDerivationRead{}, errors.New("type derivation simple type reads do not match types")
	}
	for i := range simpleTypes {
		members, ok := reads.unionMembers(SimpleTypeID(i))
		if !ok || !slices.Equal(members, simpleTypes[i].Union) {
			return TypeDerivationRead{}, errors.New("type derivation union reads do not match types")
		}
	}
	index := &typeDerivationIndex{
		simpleTypes: reads,
		anyType:     anyType,
	}
	if err := buildTypeDerivationIndex(index, simpleTypes, complexTypes); err != nil {
		return TypeDerivationRead{}, err
	}
	return TypeDerivationRead{index: index}, nil
}

func buildTypeDerivationIndex(r *typeDerivationIndex, simpleTypes []SimpleType, complexTypes []ComplexType) error {
	simpleParents := make([]int, len(simpleTypes))
	for i := range simpleParents {
		simpleParents[i] = -1
		base := simpleTypes[i].Base
		if base == NoSimpleType {
			continue
		}
		if !ValidSimpleTypeID(base, len(simpleTypes)) || int(base) == i {
			return errors.New("simple type derivation graph references invalid base")
		}
		simpleParents[i] = int(base)
	}
	var simpleOK bool
	r.simpleIn, r.simpleOut, _, simpleOK = buildDerivationForest(simpleParents)
	if !simpleOK {
		return errors.New("simple type derivation graph contains a cycle")
	}

	complexParents := make([]int, len(complexTypes))
	for i := range complexParents {
		complexParents[i] = -1
		if base, ok := complexTypes[i].Base.Complex(); ok {
			if !ValidComplexTypeID(base, len(complexTypes)) || int(base) == i {
				return errors.New("complex type derivation graph references invalid base")
			}
			complexParents[i] = int(base)
		} else if base, ok := complexTypes[i].Base.Simple(); ok && !ValidSimpleTypeID(base, len(simpleTypes)) {
			return errors.New("complex type derivation graph references invalid simple base")
		}
	}
	var order []int
	var complexOK bool
	r.complexIn, r.complexOut, order, complexOK = buildDerivationForest(complexParents)
	if !complexOK {
		return errors.New("complex type derivation graph contains a cycle")
	}
	r.complexExtensions = make([]uint32, len(complexTypes))
	r.complexRestricts = make([]uint32, len(complexTypes))
	r.complexSimpleBase = make([]SimpleTypeID, len(complexTypes))
	r.complexSimpleMask = make([]DerivationMask, len(complexTypes))
	for i := range r.complexSimpleBase {
		r.complexSimpleBase[i] = NoSimpleType
	}
	for _, i := range order {
		ct := complexTypes[i]
		if parent := complexParents[i]; parent >= 0 {
			r.complexExtensions[i] = r.complexExtensions[parent]
			r.complexRestricts[i] = r.complexRestricts[parent]
			r.complexSimpleBase[i] = r.complexSimpleBase[parent]
			r.complexSimpleMask[i] = r.complexSimpleMask[parent]
		} else if base, ok := ct.Base.Simple(); ok {
			r.complexSimpleBase[i] = base
		}
		switch ct.Derivation {
		case DerivationKindExtension:
			r.complexExtensions[i]++
			r.complexSimpleMask[i] |= DerivationExtension
		case DerivationKindRestriction:
			r.complexRestricts[i]++
			r.complexSimpleMask[i] |= DerivationRestriction
		case DerivationKindNone:
		default:
			return errors.New("complex type derivation graph stores invalid derivation kind")
		}
	}
	return nil
}

type derivationForestFrame struct {
	node  int
	child int
}

func buildDerivationForest(parents []int) (in, out []uint32, order []int, ok bool) {
	firstChild := make([]int, len(parents))
	nextSibling := make([]int, len(parents))
	for i := range firstChild {
		firstChild[i] = -1
		nextSibling[i] = -1
	}
	for child, parent := range parents {
		if parent < 0 {
			continue
		}
		if parent >= len(parents) {
			return nil, nil, nil, false
		}
		nextSibling[child] = firstChild[parent]
		firstChild[parent] = child
	}
	in = make([]uint32, len(parents))
	out = make([]uint32, len(parents))
	state := make([]uint8, len(parents))
	order = make([]int, 0, len(parents))
	stack := make([]derivationForestFrame, 0, min(len(parents), 1_024))
	var clock uint32
	visit := func(root int) bool {
		state[root] = 1
		in[root] = clock
		clock++
		order = append(order, root)
		stack = append(stack, derivationForestFrame{node: root, child: firstChild[root]})
		for len(stack) != 0 {
			last := len(stack) - 1
			frame := &stack[last]
			if frame.child < 0 {
				out[frame.node] = clock
				state[frame.node] = 2
				stack = stack[:last]
				continue
			}
			child := frame.child
			frame.child = nextSibling[child]
			if state[child] != 0 {
				return false
			}
			state[child] = 1
			in[child] = clock
			clock++
			order = append(order, child)
			stack = append(stack, derivationForestFrame{node: child, child: firstChild[child]})
		}
		return true
	}
	for i, parent := range parents {
		if parent < 0 && state[i] == 0 && !visit(i) {
			return nil, nil, nil, false
		}
	}
	if slices.Contains(state, uint8(0)) {
		return nil, nil, nil, false
	}
	return in, out, order, true
}

// AnyTypeID returns the complex type ID of xs:anyType.
func (r TypeDerivationRead) AnyTypeID() ComplexTypeID {
	if r.index == nil {
		return 0
	}
	return r.index.anyType
}

// SimpleTypeCount returns the number of simple-type derivation nodes.
func (r TypeDerivationRead) SimpleTypeCount() int {
	if r.index == nil {
		return 0
	}
	return len(r.index.simpleIn)
}

// ComplexTypeCount returns the number of complex-type derivation nodes.
func (r TypeDerivationRead) ComplexTypeCount() int {
	if r.index == nil {
		return 0
	}
	return len(r.index.complexIn)
}

func (r TypeDerivationRead) simpleTypeTable() *simpleTypeColdReadTable {
	if r.index == nil {
		return nil
	}
	return r.index.simpleTypes
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
	expected, err := newTypeDerivationReadForTypes(anyType, simpleTypes, complexTypes, read.simpleTypeTable())
	if err != nil {
		return err
	}
	if read.index == nil || expected.index == nil ||
		!slices.Equal(read.index.simpleIn, expected.index.simpleIn) ||
		!slices.Equal(read.index.simpleOut, expected.index.simpleOut) ||
		!slices.Equal(read.index.complexIn, expected.index.complexIn) ||
		!slices.Equal(read.index.complexOut, expected.index.complexOut) ||
		!slices.Equal(read.index.complexExtensions, expected.index.complexExtensions) ||
		!slices.Equal(read.index.complexRestricts, expected.index.complexRestricts) ||
		!slices.Equal(read.index.complexSimpleBase, expected.index.complexSimpleBase) ||
		!slices.Equal(read.index.complexSimpleMask, expected.index.complexSimpleMask) {
		return errors.New("type derivation index does not match type graph")
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

type typeDerivationPair struct {
	derived TypeID
	base    TypeID
}

type typeDerivationResult struct {
	mask  DerivationMask
	found bool
}

// TypeDerivationScratch owns reusable state for union derivation queries.
// It is document-local and must not be shared concurrently.
type TypeDerivationScratch struct {
	owner           *typeDerivationIndex
	memo            map[typeDerivationPair]typeDerivationResult
	unionSeen       []uint32
	unionStack      []SimpleTypeID
	unionGeneration uint32
}

// Reset clears document-local derivation state while retaining bounded storage.
func (s *TypeDerivationScratch) Reset(maxRetainedEntries int) {
	if s == nil {
		return
	}
	if cap(s.unionStack) > maxRetainedEntries {
		s.unionStack = nil
	} else {
		s.unionStack = s.unionStack[:0]
	}
	if len(s.unionSeen) > maxRetainedEntries {
		s.unionSeen = nil
		s.unionGeneration = 0
	}
	if len(s.memo) > maxRetainedEntries {
		s.memo = nil
	} else {
		clear(s.memo)
	}
}

func (s *TypeDerivationScratch) bind(owner *typeDerivationIndex) {
	if s.owner == owner {
		return
	}
	s.owner = owner
	s.memo = nil
	s.unionSeen = nil
	s.unionStack = nil
	s.unionGeneration = 0
}

func (r TypeDerivationRead) derivation(derived, base TypeID, scratch *TypeDerivationScratch) (DerivationMask, bool) {
	if scratch != nil {
		scratch.bind(r.index)
	}
	index := r.index
	if index == nil {
		return 0, false
	}
	if derived == base {
		return 0, true
	}
	if scratch != nil && scratch.memo != nil {
		if result, ok := scratch.memo[typeDerivationPair{derived: derived, base: base}]; ok {
			return result.mask, result.found
		}
	}
	var mask DerivationMask
	var found bool
	derivedComplex, derivedIsComplex := derived.Complex()
	baseComplex, baseIsComplex := base.Complex()
	derivedSimple, derivedIsSimple := derived.Simple()
	baseSimple, baseIsSimple := base.Simple()
	switch {
	case base == ComplexRef(index.anyType):
		if derivedIsComplex {
			mask, found = r.complexAnyTypeDerivation(derivedComplex)
		} else if derivedIsSimple {
			mask, found = DerivationRestriction, true
		}
	case derivedIsComplex && baseIsComplex:
		mask, found = r.complexDerivation(derivedComplex, baseComplex)
	case derivedIsComplex && baseIsSimple:
		if ValidComplexTypeID(derivedComplex, len(index.complexSimpleBase)) {
			anchor := index.complexSimpleBase[derivedComplex]
			if anchor != NoSimpleType {
				var simpleMask DerivationMask
				simpleMask, found = r.simpleDerivation(anchor, baseSimple, scratch)
				if found {
					mask = index.complexSimpleMask[derivedComplex] | simpleMask
				}
			}
		}
	case derivedIsSimple && baseIsSimple:
		mask, found = r.simpleDerivation(derivedSimple, baseSimple, scratch)
	}
	if scratch != nil {
		if scratch.memo == nil {
			scratch.memo = make(map[typeDerivationPair]typeDerivationResult)
		}
		const maxMemoEntries = 256
		if len(scratch.memo) < maxMemoEntries {
			scratch.memo[typeDerivationPair{derived: derived, base: base}] = typeDerivationResult{mask: mask, found: found}
		}
	}
	return mask, found
}

func (r TypeDerivationRead) simpleDerivation(derived, base SimpleTypeID, scratch *TypeDerivationScratch) (DerivationMask, bool) {
	index := r.index
	if index == nil {
		return 0, false
	}
	if derived == base {
		return 0, true
	}
	if !ValidSimpleTypeID(derived, len(index.simpleIn)) || !ValidSimpleTypeID(base, len(index.simpleIn)) {
		return 0, false
	}
	if index.simpleIn[base] <= index.simpleIn[derived] && index.simpleIn[derived] < index.simpleOut[base] {
		return DerivationRestriction, true
	}
	members, ok := index.simpleTypes.unionMembers(base)
	if !ok || len(members) == 0 {
		return 0, false
	}
	if scratch == nil {
		var local TypeDerivationScratch
		scratch = &local
	}
	return r.simpleUnionDerivation(derived, base, scratch)
}

func (r TypeDerivationRead) simpleUnionDerivation(derived, base SimpleTypeID, scratch *TypeDerivationScratch) (DerivationMask, bool) {
	index := r.index
	if index == nil {
		return 0, false
	}
	if len(scratch.unionSeen) != len(index.simpleIn) {
		scratch.unionSeen = make([]uint32, len(index.simpleIn))
		scratch.unionGeneration = 0
	}
	scratch.unionGeneration++
	if scratch.unionGeneration == 0 {
		clear(scratch.unionSeen)
		scratch.unionGeneration = 1
	}
	generation := scratch.unionGeneration
	stack := scratch.unionStack[:0]
	stack = append(stack, base)
	defer func() { scratch.unionStack = stack[:0] }()
	for len(stack) != 0 {
		last := len(stack) - 1
		candidate := stack[last]
		stack = stack[:last]
		if !ValidSimpleTypeID(candidate, len(index.simpleIn)) {
			return 0, false
		}
		if scratch.unionSeen[candidate] == generation {
			continue
		}
		scratch.unionSeen[candidate] = generation
		if index.simpleIn[candidate] <= index.simpleIn[derived] && index.simpleIn[derived] < index.simpleOut[candidate] {
			return DerivationRestriction, true
		}
		members, ok := index.simpleTypes.unionMembers(candidate)
		if !ok {
			return 0, false
		}
		if len(members) != 0 {
			stack = append(stack, members...)
		}
	}
	return 0, false
}

func (r TypeDerivationRead) complexDerivation(derived, base ComplexTypeID) (DerivationMask, bool) {
	index := r.index
	if index == nil || !ValidComplexTypeID(derived, len(index.complexIn)) || !ValidComplexTypeID(base, len(index.complexIn)) ||
		index.complexIn[base] > index.complexIn[derived] || index.complexIn[derived] >= index.complexOut[base] {
		return 0, false
	}
	var mask DerivationMask
	if index.complexExtensions[derived] > index.complexExtensions[base] {
		mask |= DerivationExtension
	}
	if index.complexRestricts[derived] > index.complexRestricts[base] {
		mask |= DerivationRestriction
	}
	return mask, true
}

func (r TypeDerivationRead) complexAnyTypeDerivation(derived ComplexTypeID) (DerivationMask, bool) {
	index := r.index
	if index == nil || !ValidComplexTypeID(derived, len(index.complexIn)) || !ValidComplexTypeID(index.anyType, len(index.complexIn)) {
		return 0, false
	}
	if derived == index.anyType {
		return 0, true
	}
	if mask, ok := r.complexDerivation(derived, index.anyType); ok {
		return mask, true
	}
	if index.complexSimpleBase[derived] != NoSimpleType {
		return index.complexSimpleMask[derived] | DerivationRestriction, true
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
	if derived == base {
		return 0
	}
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
		if ct.Base.IsSimple() {
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
