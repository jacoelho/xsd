package value

import (
	"errors"
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/xsdregex"
)

// Pattern is the compiled XSD regular-expression implementation. Syntax and
// matching resource policy belong to internal/xsdregex; value owns only facet
// grouping and inheritance.
type Pattern = xsdregex.Pattern

// CardinalityFacet is the source and effective form of an optional
// non-negative facet value.
type CardinalityFacet = FacetCardinalityValue

// LiteralSpec is a source lexical value used by a bound or enumeration facet.
// Type selects the value space that owns the literal. NoType means the
// containing type. Resolver is used once while completing the type and is not
// retained in the sealed program.
type LiteralSpec struct {
	Resolver Resolver
	Lexical  string
	Type     TypeID
}

// BoundFacet is an optional ordered facet literal.
type BoundFacet struct {
	LiteralSpec

	Present bool
}

// FacetSpec is the temporary source form of a type's facets. Patterns in one
// group are ORed; groups inherited from a base type are ANDed.
type FacetSpec struct {
	Patterns       [][]*Pattern
	Enumeration    []LiteralSpec
	MinInclusive   BoundFacet
	MaxExclusive   BoundFacet
	MinExclusive   BoundFacet
	MaxInclusive   BoundFacet
	MinLength      CardinalityFacet
	FractionDigits CardinalityFacet
	TotalDigits    CardinalityFacet
	MaxLength      CardinalityFacet
	Length         CardinalityFacet
	Present        FacetMask
	Fixed          FacetMask
}

// TypeSpec is temporary source metadata consumed by Builder.Complete.
type TypeSpec struct {
	Union             []TypeID
	Facets            FacetSpec
	Base              TypeID
	ListItem          TypeID
	Variety           Variety
	Primitive         PrimitiveKind
	Whitespace        WhitespaceMode
	WhitespacePresent bool
	Builtin           BuiltinKind
	Identity          IdentityKind
}

// BuilderOptions bounds construction. Zero fields use package defaults.
// MaxTypes includes the fixed builtin records.
type BuilderOptions struct {
	MaxDepth            uint16
	MaxTypes            uint32
	MaxStorageBytes     uint64
	MaxConstructionWork uint64
}

const (
	defaultValueDepth       = 1024
	defaultValueTypes       = 65536
	defaultValueStorage     = 64 << 20
	defaultConstructionWork = 16 << 20
	// typeStorageBase includes one completed type record and its scalar raw
	// evaluation plan; no per-plan heap allocation is admitted at runtime.
	typeStorageBase = uint64(512)
)

// Builder completes dependency-ordered type definitions. It is single-owner and is discarded
// after Seal; no source specs or resolver closures enter Program.
type Builder struct {
	program             *Program
	maxStorage          uint64
	storageUsed         uint64
	maxConstructionWork uint64
	prepaidSlots        int
	maxTypes            uint32
}

// NewBuilder creates a value-program builder. All XSD simple builtins occupy
// fixed IDs below BuiltinTypeCount; user types are allocated afterwards.
func NewBuilder(options BuilderOptions) *Builder {
	depth := options.MaxDepth
	if depth == 0 {
		depth = defaultValueDepth
	}
	types := options.MaxTypes
	if types == 0 {
		types = defaultValueTypes
	}
	storage := options.MaxStorageBytes
	if storage == 0 {
		storage = defaultValueStorage
	}
	work := options.MaxConstructionWork
	if work == 0 {
		work = defaultConstructionWork
	}
	return &Builder{
		program:             &Program{maxDepth: depth},
		maxTypes:            types,
		maxStorage:          storage,
		maxConstructionWork: work,
	}
}

// Reserve allocates one stable user-type ID. The slot remains incomplete until
// Complete succeeds; builtins occupy the fixed prefix below this ID.
func (b *Builder) Reserve() (TypeID, error) {
	if b == nil || b.program == nil || b.program.sealed {
		return NoType, ErrMetadata
	}
	if uint64(BuiltinTypeCount)+uint64(len(b.program.types))+1 > uint64(b.maxTypes) || len(b.program.types) > maxUserTypeIndex {
		return NoType, ErrLimit
	}
	if b.prepaidSlots == 0 {
		if b.maxStorage < typeStorageBase || b.storageUsed > b.maxStorage-typeStorageBase {
			return NoType, ErrLimit
		}
		b.storageUsed += typeStorageBase
	} else {
		b.prepaidSlots--
	}
	id := userTypeID(len(b.program.types))
	b.program.types = append(b.program.types, typeDef{})
	b.program.complete = append(b.program.complete, completionReserved)
	return id, nil
}

// ReserveCapacity admits storage for user-type slots and grows the construction
// slices before records are reserved. The charge is a bounded accounting unit
// for each slot, not an exact Go allocator byte measurement; slices.Grow may
// retain implementation-defined capacity slack.
func (b *Builder) ReserveCapacity(userTypes int) error {
	if b == nil || b.program == nil || b.program.sealed || userTypes < 0 {
		return ErrMetadata
	}
	if b.maxTypes < uint32(BuiltinTypeCount) || uint64(userTypes) > uint64(b.maxTypes)-uint64(BuiltinTypeCount) {
		return ErrLimit
	}
	if userTypes <= len(b.program.types) {
		return nil
	}
	remaining := userTypes - len(b.program.types)
	if remaining <= b.prepaidSlots {
		return nil
	}
	additional := remaining - b.prepaidSlots
	charge, ok := typeSlotStorage(additional)
	if !ok || charge > b.maxStorage || b.storageUsed > b.maxStorage-charge {
		return ErrLimit
	}
	b.program.types = slices.Grow(b.program.types, remaining)
	b.program.complete = slices.Grow(b.program.complete, remaining)
	b.storageUsed += charge
	b.prepaidSlots += additional
	return nil
}

// Complete validates and installs one reserved type. Referenced user types
// must already be visible, allowing literals to be validated before sealing.
func (b *Builder) Complete(id TypeID, spec TypeSpec) error {
	i, err := b.prepareType(id, spec)
	if err != nil {
		return err
	}
	charge, err := b.completeStorage(spec)
	if err != nil {
		return err
	}
	d := b.program.normalize(spec)
	d.dependencyDepth = b.program.types[i].dependencyDepth
	b.program.types[i] = d
	// Make the current record visible while compiling self-typed facet
	// literals. Facets are not installed until compileFacets returns.
	b.program.complete[i] = completionInProgress
	b.storageUsed += charge
	defer func() {
		if b.program.complete[i] != completionCommitted {
			b.program.types[i] = typeDef{}
			b.program.complete[i] = completionReserved
			b.storageUsed -= charge
		}
	}()
	if err := b.program.compileFacets(id, spec.Facets, b.maxConstructionWork); err != nil {
		return err
	}
	b.program.types[i].needsQName = b.program.typeNeedsQName(id, nil, 0)
	b.program.types[i].needsQNameKnown = true
	b.program.complete[i] = completionCommitted
	return nil
}

func (b *Builder) prepareType(id TypeID, spec TypeSpec) (int, error) {
	i, err := b.reservedTypeIndex(id)
	if err != nil {
		return 0, err
	}
	if shapeErr := validateTypeSpecShape(spec); shapeErr != nil {
		return 0, shapeErr
	}
	if !validVarietyRefs(spec) {
		return 0, ErrMetadata
	}
	coreDepth, err := b.admitCoreDependencies(spec)
	if err != nil {
		return 0, err
	}
	var checkedDepth uint16
	if err := b.recordFacetDependencyDepth(id, spec, &checkedDepth); err != nil {
		return 0, err
	}
	b.program.types[i].dependencyDepth = coreDepth
	return int(i), nil
}

func (b *Builder) reservedTypeIndex(id TypeID) (TypeID, error) {
	if b == nil || b.program == nil || b.program.sealed || id < BuiltinTypeCount {
		return 0, ErrMetadata
	}
	i := id - BuiltinTypeCount
	if uint64(i) >= uint64(len(b.program.types)) || b.program.complete[i] != completionReserved {
		return 0, ErrMetadata
	}
	return i, nil
}

// Admit all core references before checking depth: a later malformed member
// takes metadata-error priority over an earlier deep member.
func (b *Builder) admitCoreDependencies(spec TypeSpec) (uint16, error) {
	depth, err := b.coreReferenceDepth(spec.Base)
	if err != nil {
		return 0, err
	}
	itemDepth, err := b.coreReferenceDepth(spec.ListItem)
	if err != nil {
		return 0, err
	}
	depth = max(depth, itemDepth)
	for _, member := range spec.Union {
		if member == NoType {
			return 0, ErrMetadata
		}
		memberDepth, err := b.coreReferenceDepth(member)
		if err != nil {
			return 0, err
		}
		depth = max(depth, memberDepth)
	}
	if depth >= b.program.maxDepth {
		return 0, ErrLimit
	}
	return depth, nil
}

func (b *Builder) coreReferenceDepth(ref TypeID) (uint16, error) {
	if ref == NoType || ref < BuiltinTypeCount {
		return 0, nil
	}
	t, ok := b.program.typeDef(ref)
	if !ok {
		return 0, ErrMetadata
	}
	// Visible depths are below maxDepth, so adding one fits uint16.
	return t.dependencyDepth + 1, nil
}

func (b *Builder) completeStorage(spec TypeSpec) (uint64, error) {
	storage, ok := typeSpecStorage(spec)
	if !ok || storage > b.maxStorage {
		return 0, ErrLimit
	}
	if storage < typeStorageBase {
		return 0, ErrMetadata
	}
	storage -= typeStorageBase
	if b.storageUsed > b.maxStorage-storage {
		return 0, ErrLimit
	}
	return storage, nil
}

// Seal closes the incremental builder and returns its immutable value program.
func (b *Builder) Seal() (*Program, error) {
	if b == nil || b.program == nil || b.program.sealed {
		return nil, ErrMetadata
	}
	if b.maxTypes < uint32(BuiltinTypeCount) {
		return nil, ErrLimit
	}
	if err := b.verifyComplete(); err != nil {
		return nil, err
	}
	b.program.complete = nil
	b.program.sealed = true
	return b.program, nil
}

func (b *Builder) verifyComplete() error {
	inProgress := false
	for i, state := range b.program.complete {
		if state == completionReserved {
			return fmt.Errorf(msgTypeMetadata, userTypeID(i), ErrMetadata)
		}
		inProgress = inProgress || state == completionInProgress
	}
	if inProgress {
		return ErrMetadata
	}
	return nil
}

func (b *Builder) recordFacetDependencyDepth(id TypeID, spec TypeSpec, depth *uint16) error {
	for _, bound := range []BoundFacet{spec.Facets.MinInclusive, spec.Facets.MaxInclusive, spec.Facets.MinExclusive, spec.Facets.MaxExclusive} {
		if bound.Present {
			if err := b.recordDependencyDepth(id, bound.Type, depth); err != nil {
				return err
			}
		}
	}
	for _, literal := range spec.Facets.Enumeration {
		if err := b.recordDependencyDepth(id, literal.Type, depth); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) recordDependencyDepth(id, ref TypeID, depth *uint16) error {
	if ref == NoType || ref < BuiltinTypeCount || ref == id {
		return nil
	}
	t, ok := b.program.typeDef(ref)
	if !ok {
		return ErrMetadata
	}
	if t.dependencyDepth >= b.program.maxDepth-1 {
		return ErrLimit
	}
	candidate := t.dependencyDepth + 1
	if candidate > *depth {
		*depth = candidate
	}
	return nil
}

type completionState uint8

const (
	completionReserved completionState = iota
	completionInProgress
	completionCommitted
)

// Program is immutable after construction. It stores normalized user records;
// builtin records are returned by pure builtinTypeDef lookups.
type Program struct {
	types    []typeDef
	complete []completionState
	maxDepth uint16
	sealed   bool
}

const maxUserTypeIndex = int(^TypeID(0) - BuiltinTypeCount)

func userTypeID(index int) TypeID {
	// The builder checks the index against maxUserTypeIndex before every call.
	//nolint:gosec // the checked index is narrowed to the stable TypeID range.
	return BuiltinTypeCount + TypeID(index)
}

func typeSpecStorage(spec TypeSpec) (uint64, bool) {
	// This is a conservative admission estimate for copied source metadata and
	// the immutable facet records produced by Seal. It intentionally charges
	// lexical and pattern bytes twice because both source and sealed records
	// may retain them during construction.
	estimate := storageEstimate{total: 512}
	if !estimate.addCount(uint64(len(spec.Union)), 4) {
		return 0, false
	}
	if !estimate.addLiterals(spec.Facets.Enumeration) {
		return 0, false
	}
	for _, bound := range []BoundFacet{
		spec.Facets.MinInclusive,
		spec.Facets.MaxInclusive,
		spec.Facets.MinExclusive,
		spec.Facets.MaxExclusive,
	} {
		if bound.Present && !estimate.addCount(uint64(len(bound.Lexical)), 2) {
			return 0, false
		}
	}
	if !estimate.addPatterns(spec.Facets.Patterns) {
		return 0, false
	}
	return estimate.total, true
}

func typeSlotStorage(slots int) (uint64, bool) {
	if slots < 0 {
		return 0, false
	}
	count := uint64(slots)
	if count != 0 && typeStorageBase > ^uint64(0)/count {
		return 0, false
	}
	return count * typeStorageBase, true
}

type storageEstimate struct {
	total uint64
}

func (e *storageEstimate) add(n uint64) bool {
	if e.total > ^uint64(0)-n {
		return false
	}
	e.total += n
	return true
}

func (e *storageEstimate) addCount(count, size uint64) bool {
	if count != 0 && size > ^uint64(0)/count {
		return false
	}
	return e.add(count * size)
}

func (e *storageEstimate) addLiterals(literals []LiteralSpec) bool {
	for _, literal := range literals {
		if !e.add(32) || !e.addCount(uint64(len(literal.Lexical)), 2) {
			return false
		}
	}
	return true
}

func (e *storageEstimate) addPatterns(groups [][]*Pattern) bool {
	for _, group := range groups {
		if !e.addPatternGroup(group) {
			return false
		}
	}
	return true
}

func (e *storageEstimate) addPatternGroup(group []*Pattern) bool {
	if !e.add(16) || !e.addCount(uint64(len(group)), 8) {
		return false
	}
	for _, pattern := range group {
		if pattern != nil && (!e.add(32) || !e.addCount(uint64(len(pattern.Source())), 2)) {
			return false
		}
	}
	return true
}

type typeDef struct {
	union           []TypeID
	facets          facetProgram
	base            TypeID
	listItem        TypeID
	variety         Variety
	primitive       PrimitiveKind
	whitespace      WhitespaceMode
	builtin         BuiltinKind
	identity        IdentityKind
	dependencyDepth uint16
	needsQName      bool
	needsQNameKnown bool
}

type facetProgram struct {
	lower          []boundValue
	patterns       [][]*Pattern
	enumGroups     [][]parsedValue
	upper          []boundValue
	raw            rawEvaluationPlan
	minLength      CardinalityFacet
	fractionDigits CardinalityFacet
	totalDigits    CardinalityFacet
	maxLength      CardinalityFacet
	length         CardinalityFacet
	enumItemLimit  uint32
	enumBase64     bool
	present        FacetMask
	fixed          FacetMask
}

type boundValue struct {
	value         atomicValue
	exclusive     bool
	timeDayOffset int8
}

func (p *Program) typeDef(id TypeID) (*typeDef, bool) {
	if id < BuiltinTypeCount {
		return builtinTypeDef(id)
	}
	i := id - BuiltinTypeCount
	if uint64(i) >= uint64(len(p.types)) {
		return nil, false
	}
	if len(p.complete) != 0 && p.complete[i] == completionReserved {
		return nil, false
	}
	return &p.types[i], true
}

func (p *Program) normalize(spec TypeSpec) typeDef {
	d := typeDef{
		variety:    spec.Variety,
		primitive:  spec.Primitive,
		whitespace: spec.Whitespace,
		builtin:    spec.Builtin,
		identity:   spec.Identity,
		base:       spec.Base,
		listItem:   spec.ListItem,
		union:      append([]TypeID(nil), spec.Union...),
	}
	if !spec.WhitespacePresent {
		d.whitespace = p.inheritedWhitespace(spec.Base, d.whitespace)
	}
	if d.identity == IdentityNone {
		d.identity = p.inheritedIdentity(spec, d.identity)
	}
	return d
}

func (p *Program) inheritedWhitespace(base TypeID, fallback WhitespaceMode) WhitespaceMode {
	if parent, ok := p.typeDef(base); ok {
		return parent.whitespace
	}
	return fallback
}

func (p *Program) inheritedIdentity(spec TypeSpec, fallback IdentityKind) IdentityKind {
	if parent, ok := p.typeDef(spec.Base); ok && parent.identity != IdentityNone {
		return parent.identity
	}
	if spec.Variety == List {
		if item, ok := p.typeDef(spec.ListItem); ok && item.identity == IdentityIDREF {
			return IdentityIDREFList
		}
	}
	return fallback
}

func (p *Program) compileFacets(id TypeID, source FacetSpec, workLimit uint64) error {
	budget, err := newEvaluationBudget(workLimit)
	if err != nil {
		return err
	}
	t := &p.types[id-BuiltinTypeCount]
	mask, missing := facetMask(source)
	own := newFacetProgram(source, mask)
	if presenceErr := validateFacetPresence(source, mask, missing); presenceErr != nil {
		return presenceErr
	}
	if sourceErr := validateFacetSource(*t, source, own, missing); sourceErr != nil {
		return sourceErr
	}
	if batchErr := p.compileFacetBatch(id, t, source, &own, &budget); batchErr != nil {
		return batchErr
	}
	t = &p.types[id-BuiltinTypeCount]
	facets := own
	if t.base != NoType {
		base, ok := p.typeDef(t.base)
		if !ok {
			return ErrMetadata
		}
		facets, err = deriveFacetProgram(*t, *base, own)
		if err != nil {
			return err
		}
	}
	t.facets = facets
	if err := validateEffectiveFacetShape(t.facets); err != nil {
		return facetFailure(err.Error())
	}
	t.facets.raw = rawEvaluationPlan{}
	prepareRawDecimalFastPath(t)
	prepareRawIntegerFastPath(t)
	prepareRawValuePlans(p, t)
	return nil
}

func (p *Program) compileFacetBatch(id TypeID, t *typeDef, source FacetSpec, own *facetProgram, budget *evaluationBudget) error {
	if err := p.compileBounds(id, t, source, own, budget); err != nil {
		return err
	}
	return p.compileEnumeration(id, source, own, budget)
}

func newFacetProgram(source FacetSpec, mask FacetMask) facetProgram {
	return facetProgram{
		present:        mask,
		fixed:          source.Fixed,
		length:         source.Length,
		minLength:      source.MinLength,
		maxLength:      source.MaxLength,
		totalDigits:    source.TotalDigits,
		fractionDigits: source.FractionDigits,
		patterns:       clonePatterns(source.Patterns),
	}
}

func (p *Program) compileBounds(id TypeID, t *typeDef, source FacetSpec, own *facetProgram, budget *evaluationBudget) error {
	entries := []struct {
		src       BoundFacet
		lower     bool
		exclusive bool
	}{
		{src: source.MinInclusive, lower: true},
		{src: source.MinExclusive, lower: true, exclusive: true},
		{src: source.MaxInclusive},
		{src: source.MaxExclusive, exclusive: true},
	}
	for _, entry := range entries {
		if !entry.src.Present {
			continue
		}
		bound, err := p.compileBound(id, t, entry.src, budget)
		if err != nil {
			return err
		}
		bound.exclusive = entry.exclusive
		if entry.lower {
			own.lower = append(own.lower, bound)
		} else {
			own.upper = append(own.upper, bound)
		}
	}
	return nil
}

func (p *Program) compileBound(id TypeID, t *typeDef, source BoundFacet, budget *evaluationBudget) (boundValue, error) {
	literalType := source.Type
	if literalType == NoType {
		literalType = id
	}
	var v parsedValue
	_, err := p.eval(literalType, source.Lexical, evalOptions{
		resolver:      source.Resolver,
		enforceFacets: false,
		work:          budget,
	}, &v)
	if err != nil {
		return boundValue{}, fmt.Errorf(msgBoundValue, source.Lexical, err)
	}
	if t.variety != Atomic || v.isList || v.atom.kind != t.primitive {
		return boundValue{}, fmt.Errorf(msgIncompatibleBound, source.Lexical)
	}
	offset, err := p.timeBoundDayOffset(t, literalType, source.Lexical, v.atom)
	if err != nil {
		return boundValue{}, err
	}
	return boundValue{value: v.atom, timeDayOffset: offset}, nil
}

func (p *Program) timeBoundDayOffset(spec *typeDef, literalType TypeID, lexical string, value atomicValue) (int8, error) {
	if spec.primitive != PrimitiveTime {
		return 0, nil
	}
	normalized := normalize(lexical, effectiveWhitespace(p, literalType))
	raw, err := ParseTimeRawValue(normalized)
	if err != nil {
		return 0, err
	}
	delta := raw.second - value.time.second
	if delta%daySeconds != 0 {
		return 0, ErrMetadata
	}
	offset := delta / daySeconds
	if offset < -128 || offset > 127 {
		return 0, ErrLimit
	}
	return int8(offset), nil
}

func (p *Program) compileEnumeration(id TypeID, source FacetSpec, own *facetProgram, budget *evaluationBudget) error {
	if len(source.Enumeration) == 0 {
		return nil
	}
	group := make([]parsedValue, 0, len(source.Enumeration))
	for _, entry := range source.Enumeration {
		literalType := entry.Type
		if literalType == NoType {
			literalType = id
		}
		// Enumeration literals retain typed list items for value-space
		// comparison and canonical text for failure diagnostics. Their
		// projections are construction data, not validation results. The
		// containing type's facets are skipped until its effective program is
		// installed; evalUnion and evalListField still enforce child facets.
		var v parsedValue
		_, err := p.eval(literalType, entry.Lexical, evalOptions{
			resolver:           entry.Resolver,
			needs:              NeedCanonical | NeedIdentity,
			retainAllListItems: true,
			enforceFacets:      false,
			work:               budget,
		}, &v)
		if err != nil {
			return fmt.Errorf(msgEnumerationValue, entry.Lexical, err)
		}
		group = append(group, v)
		if v.isList && v.count > own.enumItemLimit {
			own.enumItemLimit = v.count
		}
		own.enumBase64 = own.enumBase64 || parsedValueContainsBase64(&v)
	}
	own.enumGroups = append(own.enumGroups, group)
	return nil
}

func mergeFacetPrograms(base, own facetProgram) facetProgram {
	if own.present == 0 {
		return base
	}
	out := base
	out.length = overrideCardinality(base.length, own.length)
	out.minLength = overrideCardinality(base.minLength, own.minLength)
	out.maxLength = overrideCardinality(base.maxLength, own.maxLength)
	out.totalDigits = overrideCardinality(base.totalDigits, own.totalDigits)
	out.fractionDigits = overrideCardinality(base.fractionDigits, own.fractionDigits)
	out.present |= own.present
	out.fixed |= own.fixed
	out.lower = append(append([]boundValue(nil), base.lower...), own.lower...)
	out.upper = append(append([]boundValue(nil), base.upper...), own.upper...)
	out.enumGroups = append(append([][]parsedValue(nil), base.enumGroups...), own.enumGroups...)
	out.enumItemLimit = max(base.enumItemLimit, own.enumItemLimit)
	out.enumBase64 = base.enumBase64 || own.enumBase64
	out.patterns = append(clonePatterns(base.patterns), own.patterns...)
	return out
}

func overrideCardinality(base, own CardinalityFacet) CardinalityFacet {
	if own.Present {
		return own
	}
	return base
}

func facetMask(f FacetSpec) (mask, missing FacetMask) {
	// whiteSpace is stored on TypeSpec rather than as a numeric value. A fixed
	// whiteSpace declaration therefore has no source value to infer presence
	// from, but its fixed bit still identifies the facet family.
	mask = f.Present | (f.Fixed & FacetWhiteSpace)
	var supplied FacetMask
	for _, entry := range []struct {
		flag    FacetMask
		present bool
	}{
		{FacetLength, f.Length.Present},
		{FacetMinLength, f.MinLength.Present},
		{FacetMaxLength, f.MaxLength.Present},
		{FacetTotalDigits, f.TotalDigits.Present},
		{FacetFractionDigits, f.FractionDigits.Present},
		{FacetMinInclusive, f.MinInclusive.Present},
		{FacetMaxInclusive, f.MaxInclusive.Present},
		{FacetMinExclusive, f.MinExclusive.Present},
		{FacetMaxExclusive, f.MaxExclusive.Present},
		{FacetEnumeration, len(f.Enumeration) != 0},
		{FacetPattern, len(f.Patterns) != 0},
	} {
		if entry.present {
			supplied |= entry.flag
		}
	}
	mask |= supplied
	return mask, mask &^ (supplied | FacetWhiteSpace)
}

func validateFacetPresence(f FacetSpec, mask, missing FacetMask) error {
	if f.Fixed&^mask != 0 || missing&(FacetEnumeration|FacetPattern) != 0 {
		return ErrMetadata
	}
	if !validPatternGroups(f.Patterns) || missing&orderedFacetMask != 0 {
		return ErrMetadata
	}
	return nil
}

func validPatternGroups(groups [][]*Pattern) bool {
	for _, group := range groups {
		if len(group) == 0 {
			return false
		}
		for _, pattern := range group {
			if pattern == nil {
				return false
			}
		}
	}
	return true
}

func validateFacetShape(spec typeDef, f facetProgram) error {
	if invalidBoundKinds(f) {
		return errors.New(msgBoundKindsConflict)
	}
	if message := facetVarietyError(spec, f); message != "" {
		return errors.New(message)
	}
	if f.totalDigits.Present && f.totalDigits.Value == 0 {
		return errors.New(msgTotalDigitsPositive)
	}
	if facetCardinalityViolation(f) != nil {
		return errors.New(msgFacetCardinality)
	}
	return nil
}

func invalidBoundKinds(f facetProgram) bool {
	return f.present&(FacetMinInclusive|FacetMinExclusive) == FacetMinInclusive|FacetMinExclusive || f.present&(FacetMaxInclusive|FacetMaxExclusive) == FacetMaxInclusive|FacetMaxExclusive
}

func facetVarietyError(spec typeDef, f facetProgram) string {
	unsupported := f.present &^ allowedFacetMask(spec.variety, spec.primitive)
	if unsupported&orderedFacetMask != 0 {
		return msgOrderedAtomicRequired
	}
	if unsupported&(FacetTotalDigits|FacetFractionDigits) != 0 {
		return msgAtomicDecimalRequired
	}
	if unsupported&lengthFacetMask != 0 && spec.variety == Atomic {
		return msgLengthCapableRequired
	}
	if unsupported&lengthFacetMask != 0 && spec.variety == Union {
		return msgUnionLengthForbidden
	}
	return ""
}

func validateEffectiveFacetShape(f facetProgram) error {
	for _, lower := range f.lower {
		for _, upper := range f.upper {
			if !validBoundPair(lower, upper) {
				return errors.New(msgInvalidOrderedBounds)
			}
		}
	}
	return nil
}

func validBoundPair(lower, upper boundValue) bool {
	relation, ok := compareAtomic(&lower.value, &upper.value)
	if !ok || relation == OrderedFacetGreater || relation == OrderedFacetEqual && lower.exclusive != upper.exclusive {
		return false
	}
	// Schema consistency permits empty value spaces and does not prohibit
	// incomparable endpoints; instance acceptance still checks each bound.
	return true
}

func validateTypeSpecShape(spec TypeSpec) error {
	if !spec.Whitespace.valid() || spec.Variety > Union || spec.Identity > IdentityIDREFList || spec.Builtin > BuiltinEntity {
		return ErrMetadata
	}
	if spec.Variety == Atomic && !ValidPrimitiveKind(spec.Primitive) {
		return ErrMetadata
	}
	return nil
}

func validVarietyRefs(spec TypeSpec) bool {
	if spec.Variety == List && spec.ListItem == NoType || spec.Variety == Union && len(spec.Union) == 0 {
		return false
	}
	if spec.Variety == Atomic && (spec.ListItem != NoType || len(spec.Union) != 0) {
		return false
	}
	if spec.Variety == List && len(spec.Union) != 0 {
		return false
	}
	return spec.Variety != Union || spec.ListItem == NoType
}

func clonePatterns(in [][]*Pattern) [][]*Pattern {
	out := make([][]*Pattern, len(in))
	for i := range in {
		out[i] = append([]*Pattern(nil), in[i]...)
	}
	return out
}
