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
	simpleParents, err := simpleDerivationParents(simpleTypes)
	if err != nil {
		return err
	}
	simpleForest, simpleOK := buildDerivationForest(simpleParents)
	if !simpleOK {
		return errors.New("simple type derivation graph contains a cycle")
	}
	r.simpleIn = simpleForest.in
	r.simpleOut = simpleForest.out
	complexParents, err := complexDerivationParents(simpleTypes, complexTypes)
	if err != nil {
		return err
	}
	complexForest, complexOK := buildDerivationForest(complexParents)
	if !complexOK {
		return errors.New("complex type derivation graph contains a cycle")
	}
	r.complexIn = complexForest.in
	r.complexOut = complexForest.out
	initializeComplexDerivationIndex(r, len(complexTypes))
	return populateComplexDerivationIndex(r, complexTypes, complexParents, complexForest.order)
}

func simpleDerivationParents(types []SimpleType) ([]int, error) {
	parents := make([]int, len(types))
	for i := range parents {
		parents[i] = -1
		base := types[i].Base
		if base == NoSimpleType {
			continue
		}
		if !ValidSimpleTypeID(base, len(types)) || int(base) == i {
			return nil, errors.New("simple type derivation graph references invalid base")
		}
		parents[i] = int(base)
	}
	return parents, nil
}

func complexDerivationParents(simpleTypes []SimpleType, complexTypes []ComplexType) ([]int, error) {
	parents := make([]int, len(complexTypes))
	for i := range parents {
		parents[i] = -1
		base, ok := complexTypes[i].Base.Complex()
		if ok {
			if !ValidComplexTypeID(base, len(complexTypes)) || int(base) == i {
				return nil, errors.New("complex type derivation graph references invalid base")
			}
			parents[i] = int(base)
			continue
		}
		if base, ok := complexTypes[i].Base.Simple(); ok && !ValidSimpleTypeID(base, len(simpleTypes)) {
			return nil, errors.New("complex type derivation graph references invalid simple base")
		}
	}
	return parents, nil
}

func initializeComplexDerivationIndex(r *typeDerivationIndex, count int) {
	r.complexExtensions = make([]uint32, count)
	r.complexRestricts = make([]uint32, count)
	r.complexSimpleBase = make([]SimpleTypeID, count)
	r.complexSimpleMask = make([]DerivationMask, count)
	for i := range r.complexSimpleBase {
		r.complexSimpleBase[i] = NoSimpleType
	}
}

func populateComplexDerivationIndex(
	r *typeDerivationIndex,
	types []ComplexType,
	parents, order []int,
) error {
	for _, i := range order {
		ct := types[i]
		if parent := parents[i]; parent >= 0 {
			r.complexExtensions[i] = r.complexExtensions[parent]
			r.complexRestricts[i] = r.complexRestricts[parent]
			r.complexSimpleBase[i] = r.complexSimpleBase[parent]
			r.complexSimpleMask[i] = r.complexSimpleMask[parent]
		} else if base, ok := ct.Base.Simple(); ok {
			r.complexSimpleBase[i] = base
		}
		if err := applyComplexDerivationKind(r, i, ct.Derivation); err != nil {
			return errors.New("complex type derivation graph stores invalid derivation kind")
		}
	}
	return nil
}

func applyComplexDerivationKind(r *typeDerivationIndex, index int, kind DerivationKind) error {
	switch kind {
	case DerivationKindExtension:
		r.complexExtensions[index]++
		r.complexSimpleMask[index] |= DerivationExtension
	case DerivationKindRestriction:
		r.complexRestricts[index]++
		r.complexSimpleMask[index] |= DerivationRestriction
	case DerivationKindNone:
	default:
		return errors.New("invalid derivation kind")
	}
	return nil
}

type derivationForestFrame struct {
	node  int
	child int
}

type derivationForest struct {
	in    []uint32
	out   []uint32
	order []int
}

func buildDerivationForest(parents []int) (derivationForest, bool) {
	firstChild, nextSibling, ok := derivationForestChildren(parents)
	if !ok {
		return derivationForest{}, false
	}
	audit := derivationForestAudit{
		firstChild:  firstChild,
		nextSibling: nextSibling,
		in:          make([]uint32, len(parents)),
		out:         make([]uint32, len(parents)),
		state:       make([]uint8, len(parents)),
		order:       make([]int, 0, len(parents)),
		stack:       make([]derivationForestFrame, 0, min(len(parents), 1_024)),
	}
	for i, parent := range parents {
		if parent < 0 && audit.state[i] == 0 && !audit.visit(i) {
			return derivationForest{}, false
		}
	}
	if slices.Contains(audit.state, uint8(0)) {
		return derivationForest{}, false
	}
	return derivationForest{in: audit.in, out: audit.out, order: audit.order}, true
}

func derivationForestChildren(parents []int) (firstChild, nextSibling []int, valid bool) {
	firstChild = make([]int, len(parents))
	nextSibling = make([]int, len(parents))
	for i := range firstChild {
		firstChild[i] = -1
		nextSibling[i] = -1
	}
	for child, parent := range parents {
		if parent < 0 {
			continue
		}
		if parent >= len(parents) {
			return nil, nil, false
		}
		nextSibling[child] = firstChild[parent]
		firstChild[parent] = child
	}
	return firstChild, nextSibling, true
}

type derivationForestAudit struct {
	firstChild  []int
	nextSibling []int
	in          []uint32
	out         []uint32
	state       []uint8
	order       []int
	stack       []derivationForestFrame
	clock       uint32
}

func (a *derivationForestAudit) visit(root int) bool {
	a.push(root)
	for len(a.stack) != 0 {
		last := len(a.stack) - 1
		frame := &a.stack[last]
		if frame.child < 0 {
			a.complete(last, frame.node)
			continue
		}
		child := frame.child
		frame.child = a.nextSibling[child]
		if a.state[child] != 0 {
			return false
		}
		a.push(child)
	}
	return true
}

func (a *derivationForestAudit) push(node int) {
	a.state[node] = 1
	a.in[node] = a.clock
	a.clock++
	a.order = append(a.order, node)
	a.stack = append(a.stack, derivationForestFrame{node: node, child: a.firstChild[node]})
}

func (a *derivationForestAudit) complete(last, node int) {
	a.out[node] = a.clock
	a.state[node] = 2
	a.stack = a.stack[:last]
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
	mask, ok, err := deriveTypeMask(rt, derived, base, unboundedTypeDerivationWork)
	if err != nil {
		return 0, false
	}
	return mask, ok
}

func unboundedTypeDerivationWork(int) error { return nil }

func deriveTypeMask[T TypeDerivationRuntime](
	rt T,
	derived, base TypeID,
	work func(int) error,
) (DerivationMask, bool, error) {
	if err := work(1); err != nil {
		return 0, false, err
	}
	if derived == base {
		return 0, true, nil
	}
	if base == ComplexRef(rt.AnyTypeID()) {
		if id, ok := derived.Complex(); ok {
			return complexAnyTypeDerivationMask(rt, id, work)
		}
		return DerivationRestriction, true, nil
	}
	if derivedID, ok := derived.Complex(); ok {
		return complexDerivedTypeDerivationMask(rt, derivedID, base, work)
	}
	if derivedID, ok := derived.Simple(); ok {
		return simpleDerivedTypeDerivationMask(rt, derivedID, base, work)
	}
	return 0, false, nil
}

func complexDerivedTypeDerivationMask[T TypeDerivationRuntime](
	rt T,
	derived ComplexTypeID,
	base TypeID,
	work func(int) error,
) (DerivationMask, bool, error) {
	if baseID, ok := base.Simple(); ok {
		return complexSimpleTypeDerivationMask(rt, derived, baseID, work)
	}
	if baseID, ok := base.Complex(); ok {
		return complexTypeDerivationMask(rt, derived, baseID, work)
	}
	return 0, false, nil
}

func simpleDerivedTypeDerivationMask[T TypeDerivationRuntime](
	rt T,
	derived SimpleTypeID,
	base TypeID,
	work func(int) error,
) (DerivationMask, bool, error) {
	baseID, ok := base.Simple()
	if !ok {
		return 0, false, nil
	}
	return simpleTypeDerivationMaskOf(rt, derived, baseID, work)
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
	pair := typeDerivationPair{derived: derived, base: base}
	if result, ok := lookupTypeDerivationMemo(scratch, pair); ok {
		return result.mask, result.found
	}
	mask, found := r.deriveUncached(derived, base, scratch)
	storeTypeDerivationMemo(scratch, pair, typeDerivationResult{mask: mask, found: found})
	return mask, found
}

func lookupTypeDerivationMemo(scratch *TypeDerivationScratch, pair typeDerivationPair) (typeDerivationResult, bool) {
	if scratch == nil || scratch.memo == nil {
		return typeDerivationResult{}, false
	}
	result, ok := scratch.memo[pair]
	return result, ok
}

func (r TypeDerivationRead) deriveUncached(
	derived, base TypeID,
	scratch *TypeDerivationScratch,
) (DerivationMask, bool) {
	index := r.index
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
		mask, found = r.complexSimpleDerivation(derivedComplex, baseSimple, scratch)
	case derivedIsSimple && baseIsSimple:
		mask, found = r.simpleDerivation(derivedSimple, baseSimple, scratch)
	}
	return mask, found
}

func (r TypeDerivationRead) complexSimpleDerivation(
	derived ComplexTypeID,
	base SimpleTypeID,
	scratch *TypeDerivationScratch,
) (DerivationMask, bool) {
	index := r.index
	if !ValidComplexTypeID(derived, len(index.complexSimpleBase)) {
		return 0, false
	}
	anchor := index.complexSimpleBase[derived]
	if anchor == NoSimpleType {
		return 0, false
	}
	simpleMask, found := r.simpleDerivation(anchor, base, scratch)
	if !found {
		return 0, false
	}
	return index.complexSimpleMask[derived] | simpleMask, true
}

func storeTypeDerivationMemo(
	scratch *TypeDerivationScratch,
	pair typeDerivationPair,
	result typeDerivationResult,
) {
	if scratch == nil {
		return
	}
	if scratch.memo == nil {
		scratch.memo = make(map[typeDerivationPair]typeDerivationResult)
	}
	const maxMemoEntries = 256
	if len(scratch.memo) < maxMemoEntries {
		scratch.memo[pair] = result
	}
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
	generation, stack := prepareSimpleUnionDerivationScratch(scratch, len(index.simpleIn), base)
	defer func() { scratch.unionStack = stack[:0] }()
	for len(stack) != 0 {
		last := len(stack) - 1
		candidate := stack[last]
		stack = stack[:last]
		members, found, ok := r.simpleUnionCandidate(derived, candidate, generation, scratch)
		if !ok {
			return 0, false
		}
		if found {
			return DerivationRestriction, true
		}
		stack = append(stack, members...)
	}
	return 0, false
}

func prepareSimpleUnionDerivationScratch(
	scratch *TypeDerivationScratch,
	count int,
	base SimpleTypeID,
) (uint32, []SimpleTypeID) {
	if len(scratch.unionSeen) != count {
		scratch.unionSeen = make([]uint32, count)
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
	return generation, stack
}

func (r TypeDerivationRead) simpleUnionCandidate(
	derived, candidate SimpleTypeID,
	generation uint32,
	scratch *TypeDerivationScratch,
) (stack []SimpleTypeID, found, valid bool) {
	index := r.index
	if !ValidSimpleTypeID(candidate, len(index.simpleIn)) {
		return nil, false, false
	}
	if scratch.unionSeen[candidate] == generation {
		return nil, false, true
	}
	scratch.unionSeen[candidate] = generation
	if index.simpleIn[candidate] <= index.simpleIn[derived] && index.simpleIn[derived] < index.simpleOut[candidate] {
		return nil, true, true
	}
	members, ok := index.simpleTypes.unionMembers(candidate)
	return members, false, ok
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
	blocks := baseSubstitutionTypeBlock(rt, base)
	current, ok := derived.Complex()
	if !ok {
		return blocks
	}
	for range rt.ComplexTypeCount() {
		block, parent, done := nextSubstitutionTypeBlock(rt, current, base)
		if done {
			return blocks
		}
		blocks |= block
		current = parent
	}
	return blocks
}

func baseSubstitutionTypeBlock(rt TypeDerivationRuntime, base TypeID) DerivationMask {
	baseID, ok := base.Complex()
	if !ok {
		return 0
	}
	baseType, ok := rt.ComplexTypeDerivation(baseID)
	if !ok {
		return 0
	}
	return baseType.Block
}

func nextSubstitutionTypeBlock(
	rt TypeDerivationRuntime,
	current ComplexTypeID,
	base TypeID,
) (DerivationMask, ComplexTypeID, bool) {
	typ, ok := rt.ComplexTypeDerivation(current)
	if !ok || typ.Base == base {
		return 0, 0, true
	}
	parent, ok := typ.Base.Complex()
	if !ok {
		return 0, 0, true
	}
	parentType, ok := rt.ComplexTypeDerivation(parent)
	if !ok {
		return 0, 0, true
	}
	return parentType.Block, parent, false
}

func derivationKindMask(kind DerivationKind) DerivationMask {
	switch kind {
	case DerivationKindExtension:
		return DerivationExtension
	case DerivationKindRestriction:
		return DerivationRestriction
	case DerivationKindNone:
		return 0
	default:
	}
	return 0
}

func complexSimpleTypeDerivationMask[T TypeDerivationRuntime](
	rt T,
	derived ComplexTypeID,
	base SimpleTypeID,
	work func(int) error,
) (DerivationMask, bool, error) {
	var mask DerivationMask
	for range rt.ComplexTypeCount() {
		step, err := nextComplexSimpleDerivation(rt, derived, base, work)
		if err != nil {
			return 0, false, err
		}
		if !step.valid {
			return 0, false, nil
		}
		mask |= step.mask
		if step.done {
			return mask, true, nil
		}
		derived = step.next
	}
	return 0, false, nil
}

type complexSimpleDerivationStep struct {
	next  ComplexTypeID
	mask  DerivationMask
	done  bool
	valid bool
}

func nextComplexSimpleDerivation[T TypeDerivationRuntime](
	rt T,
	derived ComplexTypeID,
	base SimpleTypeID,
	work func(int) error,
) (complexSimpleDerivationStep, error) {
	if err := work(1); err != nil {
		return complexSimpleDerivationStep{}, err
	}
	typ, ok := rt.ComplexTypeDerivation(derived)
	if !ok {
		return complexSimpleDerivationStep{}, nil
	}
	step := complexSimpleDerivationStep{mask: derivationKindMask(typ.Kind), valid: true}
	if simpleBase, simpleOK := typ.Base.Simple(); simpleOK {
		simpleMask, found, err := simpleTypeDerivationMaskOf(rt, simpleBase, base, work)
		if err != nil || !found {
			return complexSimpleDerivationStep{}, err
		}
		step.mask |= simpleMask
		step.done = true
		return step, nil
	}
	next, ok := typ.Base.Complex()
	if !ok {
		return complexSimpleDerivationStep{}, nil
	}
	step.next = next
	return step, nil
}

func complexAnyTypeDerivationMask[T TypeDerivationRuntime](
	rt T,
	derived ComplexTypeID,
	work func(int) error,
) (DerivationMask, bool, error) {
	var mask DerivationMask
	for range rt.ComplexTypeCount() {
		step, err := nextComplexAnyTypeDerivation(rt, derived, work)
		if err != nil {
			return 0, false, err
		}
		if !step.valid {
			return 0, false, nil
		}
		mask |= step.mask
		if step.done {
			return mask, true, nil
		}
		derived = step.next
	}
	return 0, false, nil
}

type complexAnyTypeDerivationStep struct {
	next  ComplexTypeID
	mask  DerivationMask
	done  bool
	valid bool
}

func nextComplexAnyTypeDerivation[T TypeDerivationRuntime](
	rt T,
	derived ComplexTypeID,
	work func(int) error,
) (complexAnyTypeDerivationStep, error) {
	if err := work(1); err != nil {
		return complexAnyTypeDerivationStep{}, err
	}
	if derived == rt.AnyTypeID() {
		return complexAnyTypeDerivationStep{done: true, valid: true}, nil
	}
	typ, ok := rt.ComplexTypeDerivation(derived)
	if !ok {
		return complexAnyTypeDerivationStep{}, nil
	}
	step := complexAnyTypeDerivationStep{mask: derivationKindMask(typ.Kind), valid: true}
	if typ.Base.IsSimple() {
		step.mask |= DerivationRestriction
		step.done = true
		return step, nil
	}
	parent, ok := typ.Base.Complex()
	if !ok {
		return complexAnyTypeDerivationStep{}, nil
	}
	step.next = parent
	return step, nil
}

func simpleTypeDerivationMaskOf[T TypeDerivationRuntime](
	rt T,
	derived, base SimpleTypeID,
	work func(int) error,
) (DerivationMask, bool, error) {
	if derived == base {
		return 0, true, nil
	}
	if err := work(2); err != nil {
		return 0, false, err
	}
	st, ok := rt.SimpleTypeDerivation(derived)
	if !ok {
		return 0, false, nil
	}
	baseType, ok := rt.SimpleTypeDerivation(base)
	if !ok {
		return 0, false, nil
	}
	if baseType.Variety != SimpleVarietyUnion {
		return simpleTypeBaseChainDerivationMask(rt, derived, base, st, work)
	}
	return simpleTypeUnionDerivationMask(rt, derived, base, work)
}

func simpleTypeBaseChainDerivationMask[T TypeDerivationRuntime](
	rt T,
	derived, base SimpleTypeID,
	st SimpleTypeDerivation,
	work func(int) error,
) (DerivationMask, bool, error) {
	cycle := simpleTypeBaseChainCycle{anchor: derived, power: 1}
	for {
		step, err := nextSimpleBaseDerivation(rt, derived, base, st, work, &cycle)
		if err != nil || !step.valid {
			return 0, false, err
		}
		if step.found {
			return DerivationRestriction, true, nil
		}
		derived = step.id
		st = step.typ
	}
}

type simpleBaseDerivationStep struct {
	typ   SimpleTypeDerivation
	id    SimpleTypeID
	found bool
	valid bool
}

func nextSimpleBaseDerivation[T TypeDerivationRuntime](
	rt T,
	derived, base SimpleTypeID,
	typ SimpleTypeDerivation,
	work func(int) error,
	cycle *simpleTypeBaseChainCycle,
) (simpleBaseDerivationStep, error) {
	if typ.Base == NoSimpleType || typ.Base == derived {
		return simpleBaseDerivationStep{}, nil
	}
	next := typ.Base
	if next == base {
		return simpleBaseDerivationStep{id: next, found: true, valid: true}, nil
	}
	if cycle.repeats(next) {
		return simpleBaseDerivationStep{}, nil
	}
	if err := work(1); err != nil {
		return simpleBaseDerivationStep{}, err
	}
	nextType, ok := rt.SimpleTypeDerivation(next)
	if !ok {
		return simpleBaseDerivationStep{}, nil
	}
	return simpleBaseDerivationStep{id: next, typ: nextType, valid: true}, nil
}

type simpleTypeBaseChainCycle struct {
	anchor   SimpleTypeID
	power    uint64
	distance uint64
}

func (c *simpleTypeBaseChainCycle) repeats(current SimpleTypeID) bool {
	c.distance++
	if current == c.anchor {
		return true
	}
	if c.distance == c.power {
		c.anchor = current
		c.power *= 2
		c.distance = 0
	}
	return false
}

type simpleTypeDerivationFrame struct {
	derived SimpleTypeID
	base    SimpleTypeID
	next    int
	entered bool
}

func simpleTypeUnionDerivationMask[T TypeDerivationRuntime](
	rt T,
	derived, base SimpleTypeID,
	work func(int) error,
) (DerivationMask, bool, error) {
	limit := simpleTypeDerivationPairLimit(rt.SimpleTypeCount())
	if err := work(min(limit, 1_024) + 2); err != nil {
		return 0, false, err
	}
	search := simpleTypeUnionDerivationSearch[T]{
		rt:    rt,
		work:  work,
		limit: limit,
		stack: make([]simpleTypeDerivationFrame, 0, min(limit, 1_024)),
		seen:  make(map[[2]SimpleTypeID]bool),
	}
	search.push(derived, base)
	for len(search.stack) != 0 {
		found, err := search.advance()
		if err != nil {
			return 0, false, err
		}
		if found {
			return DerivationRestriction, true, nil
		}
	}
	return 0, false, nil
}

type simpleTypeUnionDerivationSearch[T TypeDerivationRuntime] struct {
	rt    T
	work  func(int) error
	seen  map[[2]SimpleTypeID]bool
	stack []simpleTypeDerivationFrame
	limit int
}

func (s *simpleTypeUnionDerivationSearch[T]) advance() (bool, error) {
	if err := s.work(3); err != nil {
		return false, err
	}
	last := len(s.stack) - 1
	frame := &s.stack[last]
	if frame.derived == frame.base {
		return true, nil
	}
	derived, base, ok := s.frameTypes(last, frame)
	if !ok {
		return false, nil
	}
	if handled, err := s.enter(last, frame); handled || err != nil {
		return false, err
	}
	memberCount := unionMemberCount(base)
	if frame.next < memberCount {
		return s.visitMember(frame, base.Union[frame.next])
	}
	if frame.next == memberCount {
		frame.next++
		found, continued := s.visitBase(frame, derived)
		if found || continued {
			return found, nil
		}
	}
	s.pop(last)
	return false, nil
}

func (s *simpleTypeUnionDerivationSearch[T]) frameTypes(
	last int,
	frame *simpleTypeDerivationFrame,
) (derived, base SimpleTypeDerivation, valid bool) {
	derived, ok := s.rt.SimpleTypeDerivation(frame.derived)
	if !ok {
		s.pop(last)
		return SimpleTypeDerivation{}, SimpleTypeDerivation{}, false
	}
	base, ok = s.rt.SimpleTypeDerivation(frame.base)
	if !ok {
		s.pop(last)
		return SimpleTypeDerivation{}, SimpleTypeDerivation{}, false
	}
	return derived, base, true
}

func (s *simpleTypeUnionDerivationSearch[T]) enter(
	last int,
	frame *simpleTypeDerivationFrame,
) (bool, error) {
	if frame.entered {
		return false, nil
	}
	pair := [2]SimpleTypeID{frame.derived, frame.base}
	if s.seen[pair] {
		s.pop(last)
		return true, nil
	}
	if err := s.work(1); err != nil {
		return false, err
	}
	s.seen[pair] = true
	frame.entered = true
	return false, nil
}

func unionMemberCount(typ SimpleTypeDerivation) int {
	if typ.Variety != SimpleVarietyUnion {
		return 0
	}
	return len(typ.Union)
}

func (s *simpleTypeUnionDerivationSearch[T]) visitMember(
	frame *simpleTypeDerivationFrame,
	member SimpleTypeID,
) (bool, error) {
	if err := s.work(1); err != nil {
		return false, err
	}
	frame.next++
	if frame.derived == member {
		return true, nil
	}
	if s.seen[[2]SimpleTypeID{frame.derived, member}] {
		return false, nil
	}
	s.push(frame.derived, member)
	return false, nil
}

func (s *simpleTypeUnionDerivationSearch[T]) visitBase(
	frame *simpleTypeDerivationFrame,
	derived SimpleTypeDerivation,
) (found, handled bool) {
	if derived.Base == NoSimpleType || derived.Base == frame.derived {
		return false, false
	}
	if derived.Base == frame.base {
		return true, true
	}
	if s.seen[[2]SimpleTypeID{derived.Base, frame.base}] {
		return false, true
	}
	s.push(derived.Base, frame.base)
	return false, true
}

func (s *simpleTypeUnionDerivationSearch[T]) push(derived, base SimpleTypeID) {
	s.stack = appendDFSFrame(s.stack, simpleTypeDerivationFrame{derived: derived, base: base}, s.limit)
}

func (s *simpleTypeUnionDerivationSearch[T]) pop(last int) {
	s.stack = s.stack[:last]
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

func complexTypeDerivationMask[T TypeDerivationRuntime](
	rt T,
	derived, base ComplexTypeID,
	work func(int) error,
) (DerivationMask, bool, error) {
	var mask DerivationMask
	for range rt.ComplexTypeCount() {
		step, ok, err := complexParentDerivation(rt, derived, work)
		if err != nil {
			return 0, false, err
		}
		if !ok {
			return 0, false, nil
		}
		mask |= step.mask
		if step.parent == base {
			return mask, true, nil
		}
		derived = step.parent
	}
	return 0, false, nil
}

type complexParentDerivationStep struct {
	parent ComplexTypeID
	mask   DerivationMask
}

func complexParentDerivation[T TypeDerivationRuntime](
	rt T,
	derived ComplexTypeID,
	work func(int) error,
) (complexParentDerivationStep, bool, error) {
	if err := work(1); err != nil {
		return complexParentDerivationStep{}, false, err
	}
	typ, ok := rt.ComplexTypeDerivation(derived)
	if !ok {
		return complexParentDerivationStep{}, false, nil
	}
	parent, ok := typ.Base.Complex()
	if !ok {
		return complexParentDerivationStep{}, false, nil
	}
	return complexParentDerivationStep{parent: parent, mask: derivationKindMask(typ.Kind)}, true, nil
}
