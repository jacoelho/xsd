package runtime

import (
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/uriref"
)

// ErrSimpleValueMetadata reports invalid frozen simple-type metadata discovered
// while routing simple-value validation.
var ErrSimpleValueMetadata = errors.New("simple value metadata is invalid")

// SimpleValueType is the runtime-owned projection needed to route full
// simple-value validation.
type SimpleValueType struct {
	DecimalMinInclusive RawDecimalBound
	DecimalMaxInclusive RawDecimalBound
	UnionMembers        []SimpleTypeID
	StringFacets        StringFacetValues
	DecimalFacets       DecimalFacetValues
	LengthFacets        LengthFacetValues
	ListItem            SimpleTypeID
	Facets              FacetMask
	Variety             SimpleVariety
	Primitive           PrimitiveKind
	Builtin             BuiltinValidationKind
	Whitespace          WhitespaceMode
	Identity            SimpleIdentityKind
	Fast                SimpleFastKind
	RawBypass           SimpleValueBypassAction
}

// SimpleValueCallbacks supplies schema edge facts and read projections used by
// the runtime-owned simple-value dispatcher.
type SimpleValueCallbacks struct {
	Type              func(id SimpleTypeID) (SimpleValueType, bool)
	Facets            func(id SimpleTypeID) (SimpleValueFacets, bool)
	StringEnumeration func(id SimpleTypeID, canonical string) (bool, bool)
	ResolveQName      func(string) (ns, local string, ok bool)
	Notation          func(ns, local string) bool
	Unsupported       func(error) bool
}

type simpleValueMetadataReader interface {
	simpleValueType(id SimpleTypeID) (SimpleValueType, bool)
	simpleValueFacets(id SimpleTypeID) (SimpleValueFacets, bool)
	simpleValueStringEnumeration(id SimpleTypeID, canonical string) (bool, bool)
	simpleValueUnsupported(err error) bool
}

type simpleValueReader interface {
	simpleValueMetadataReader
	simpleValueNotationReader
}

type callbackSimpleValueMetadataReader struct {
	callbacks SimpleValueCallbacks
}

func (r callbackSimpleValueMetadataReader) simpleValueType(id SimpleTypeID) (SimpleValueType, bool) {
	if r.callbacks.Type == nil {
		return SimpleValueType{}, false
	}
	return r.callbacks.Type(id)
}

func (r callbackSimpleValueMetadataReader) simpleValueFacets(id SimpleTypeID) (SimpleValueFacets, bool) {
	if r.callbacks.Facets == nil {
		return SimpleValueFacets{}, false
	}
	return r.callbacks.Facets(id)
}

func (r callbackSimpleValueMetadataReader) simpleValueStringEnumeration(id SimpleTypeID, canonical string) (contains, valid bool) {
	if r.callbacks.StringEnumeration == nil {
		return false, false
	}
	return r.callbacks.StringEnumeration(id, canonical)
}

func (r callbackSimpleValueMetadataReader) simpleValueNotation(ns, local string) (declared, valid bool) {
	if r.callbacks.Notation == nil {
		return false, false
	}
	return r.callbacks.Notation(ns, local), true
}

func (r callbackSimpleValueMetadataReader) simpleValueUnsupported(err error) bool {
	return r.callbacks.Unsupported != nil && r.callbacks.Unsupported(err)
}

// LengthFacetValues is the runtime projection of length/minLength/maxLength
// facet values.
type LengthFacetValues struct {
	Length    FacetCardinalityValue
	MinLength FacetCardinalityValue
	MaxLength FacetCardinalityValue
}

// StringFacetValues is the runtime projection of string-based facets.
type StringFacetValues struct {
	patternSource  stringPatternSteps
	patternReads   *stringPatternStepRead
	HasEnumeration bool
}

func (f StringFacetValues) patternCount() int {
	if f.patternReads != nil {
		return int(f.patternReads.count)
	}
	return int(f.patternSource.count())
}

func (f StringFacetValues) validatePatterns(normalized string, scratch *StringPatternScratch) error {
	if f.patternReads != nil {
		return validateStringPatternStepReadsWithScratch(f.patternReads, normalized, scratch)
	}
	return validateStringPatternSteps(f.patternSource, normalized)
}

// SimpleValueFacetLiteral is the runtime read projection of a compiled facet
// literal. It carries only the value facts needed by validation.
type SimpleValueFacetLiteral struct {
	Canonical string
	Actual    PrimitiveActualValue
	Present   bool
}

// SimpleValueFacets is the runtime-owned read projection of simple-type facets
// needed by schema atomic fallback validation.
type SimpleValueFacets struct {
	Enumeration      []SimpleValueFacetLiteral
	StringFacets     StringFacetValues
	enumerationReads []simpleValueLiteralRead
	MinInclusive     SimpleValueFacetLiteral
	MaxInclusive     SimpleValueFacetLiteral
	MinExclusive     SimpleValueFacetLiteral
	MaxExclusive     SimpleValueFacetLiteral
	DecimalFacets    DecimalFacetValues
	LengthFacets     LengthFacetValues
	Facets           FacetMask
}

// SimpleValueFacetProjector projects immutable facet storage while pooling
// enumeration projections by exact source identity.
type SimpleValueFacetProjector struct {
	enumerations map[simpleValueEnumerationSource][]SimpleValueFacetLiteral
}

// simpleValueRouteRead is the compact, cache-hot subset used before full
// simple-value validation falls back to the immutable cold type record.
type simpleValueRouteRead struct {
	minInclusive RawDecimalBound
	maxInclusive RawDecimalBound
	listItem     SimpleTypeID
	facets       FacetMask
	variety      SimpleVariety
	primitive    PrimitiveKind
	builtin      BuiltinValidationKind
	whitespace   WhitespaceMode
	identity     SimpleIdentityKind
	fast         SimpleFastKind
	rawBypass    SimpleValueBypassAction
	availability simpleTypeAvailability
	present      bool
}

type simpleTypeAvailability uint8

const (
	simpleTypeAvailabilityInvalid simpleTypeAvailability = iota
	simpleTypeAvailabilityAvailable
	simpleTypeAvailabilityUnavailable
)

type simpleValueColdRead struct {
	union       []SimpleTypeID
	enumeration []simpleValueLiteralRead
	facets      simpleValueFacetRead
}

type simpleValueLiteralRead struct {
	canonical string
	actual    PrimitiveActualValue
}

type simpleValueEnumerationSource struct {
	first  *CompiledLiteral
	length int
}

type simpleValueEnumerationRead struct {
	first  *simpleValueLiteralRead
	length int
}

type simpleValueBoundReads [4]*simpleValueLiteralRead

// simpleValueFacetRead contains only facet state read during instance
// validation. Enumeration literals use a separate compact projection and do
// not retain the compiler's literal table.
type simpleValueFacetRead struct {
	bounds         simpleValueBoundReads
	patterns       *stringPatternStepRead
	length         uint32
	minLength      uint32
	maxLength      uint32
	totalDigits    uint32
	fractionDigits uint32
	present        FacetMask
}

func newSimpleValueFacetRead(
	f FacetSet,
	boundPool []simpleValueLiteralRead,
	boundIndexes map[*CompiledLiteral]uint32,
	patterns *stringPatternStepRead,
) simpleValueFacetRead {
	var bounds simpleValueBoundReads
	for i, literal := range f.bounds {
		if literal == nil {
			continue
		}
		index, ok := boundIndexes[literal]
		if !ok || !ValidUint32Index(index, len(boundPool)) {
			panic("simple value bound is missing from read pool")
		}
		bounds[i] = &boundPool[index]
	}
	return simpleValueFacetRead{
		bounds:         bounds,
		patterns:       patterns,
		length:         f.Length,
		minLength:      f.MinLength,
		maxLength:      f.MaxLength,
		totalDigits:    f.TotalDigits,
		fractionDigits: f.FractionDigits,
		present:        f.Present,
	}
}

func (f simpleValueFacetRead) bound(flag FacetMask) (simpleValueLiteralRead, bool) {
	if f.present&flag == 0 {
		return simpleValueLiteralRead{}, false
	}
	index, ok := boundFacetIndex(flag)
	if !ok || f.bounds[index] == nil {
		return simpleValueLiteralRead{}, false
	}
	return *f.bounds[index], true
}

func (f simpleValueFacetRead) literal(flag FacetMask) SimpleValueFacetLiteral {
	lit, present := f.bound(flag)
	if !present {
		return SimpleValueFacetLiteral{}
	}
	return SimpleValueFacetLiteral{Canonical: lit.canonical, Actual: lit.actual, Present: true}
}

func newSimpleValueLiteralRead(lit CompiledLiteral) simpleValueLiteralRead {
	return simpleValueLiteralRead{canonical: lit.Canonical, actual: lit.Actual}
}

func (f simpleValueFacetRead) lengthValues() LengthFacetValues {
	return LengthFacetValues{
		Length:    facetCardinalityValue(f.present, FacetLength, f.length),
		MinLength: facetCardinalityValue(f.present, FacetMinLength, f.minLength),
		MaxLength: facetCardinalityValue(f.present, FacetMaxLength, f.maxLength),
	}
}

func (f simpleValueFacetRead) decimalValues() DecimalFacetValues {
	return DecimalFacetValues{
		MinInclusive:   f.decimalBound(FacetMinInclusive),
		MaxInclusive:   f.decimalBound(FacetMaxInclusive),
		MinExclusive:   f.decimalBound(FacetMinExclusive),
		MaxExclusive:   f.decimalBound(FacetMaxExclusive),
		TotalDigits:    facetCardinalityValue(f.present, FacetTotalDigits, f.totalDigits),
		FractionDigits: facetCardinalityValue(f.present, FacetFractionDigits, f.fractionDigits),
		Facets:         f.present,
	}
}

func (f simpleValueFacetRead) decimalBound(flag FacetMask) DecimalFacetValue {
	lit, present := f.bound(flag)
	if !present {
		return DecimalFacetValue{}
	}
	return DecimalFacetValue{
		Value:   lit.actual.Decimal,
		Present: lit.actual.Valid && lit.actual.Kind == PrimitiveDecimal,
	}
}

type simpleTypeColdReadTable struct {
	index      []uint32
	values     []simpleValueColdRead
	boundReads []simpleValueLiteralRead
}

func newSimpleTypeColdReadTable(types []SimpleType) *simpleTypeColdReadTable {
	count := simpleTypeColdReadCount(types)
	table := simpleTypeColdReadTable{
		index:  make([]uint32, len(types)),
		values: make([]simpleValueColdRead, 0, count),
	}
	boundIndexes := simpleValueBoundReadIndexes(types)
	table.boundReads = make([]simpleValueLiteralRead, len(boundIndexes))
	populateSimpleValueBoundReads(table.boundReads, boundIndexes)
	fillSimpleValueColdReadIndex(table.index)
	enumerationPool := newSimpleValueEnumerationReadPool(types)
	patternPool := newStringPatternReadPoolForSimpleTypes(types)
	populateSimpleTypeColdReads(&table, types, boundIndexes, enumerationPool, patternPool)
	return &table
}

func simpleTypeColdReadCount(types []SimpleType) int {
	count := 0
	for i := range types {
		if simpleTypeNeedsColdRead(types[i]) {
			count++
		}
	}
	return count
}

func populateSimpleValueBoundReads(reads []simpleValueLiteralRead, indexes map[*CompiledLiteral]uint32) {
	for source, index := range indexes {
		reads[index] = newSimpleValueLiteralRead(*source)
	}
}

func fillSimpleValueColdReadIndex(index []uint32) {
	for i := range index {
		index[i] = invalidID
	}
}

func populateSimpleTypeColdReads(
	table *simpleTypeColdReadTable,
	types []SimpleType,
	boundIndexes map[*CompiledLiteral]uint32,
	enumerationPool map[simpleValueEnumerationSource][]simpleValueLiteralRead,
	patternPool map[*stringPatternStep]*stringPatternStepRead,
) {
	for i := range types {
		if !simpleTypeNeedsColdRead(types[i]) {
			continue
		}
		if len(table.values) >= int(invalidID) {
			panic("too many simple value cold reads")
		}
		table.index[i] = uint32(len(table.values)) //nolint:gosec // guarded against the invalidID sentinel above.
		table.values = append(table.values, newSimpleTypeColdRead(types[i], table.boundReads, boundIndexes, enumerationPool, patternPool))
	}
}

func newSimpleTypeColdRead(
	typ SimpleType,
	boundReads []simpleValueLiteralRead,
	boundIndexes map[*CompiledLiteral]uint32,
	enumerationPool map[simpleValueEnumerationSource][]simpleValueLiteralRead,
	patternPool map[*stringPatternStep]*stringPatternStepRead,
) simpleValueColdRead {
	var enumeration []simpleValueLiteralRead
	if source, ok := simpleValueEnumerationSourceForLiterals(typ.Facets.Enumeration); ok {
		enumeration = enumerationPool[source]
	}
	var patterns *stringPatternStepRead
	if source := typ.Facets.patterns.tail; source != nil {
		patterns = patternPool[source]
	}
	return simpleValueColdRead{
		union:       slices.Clone(typ.Union),
		enumeration: enumeration,
		facets:      newSimpleValueFacetRead(typ.Facets, boundReads, boundIndexes, patterns),
	}
}

func newSimpleValueEnumerationReadPool(types []SimpleType) map[simpleValueEnumerationSource][]simpleValueLiteralRead {
	pool, literalCount := collectSimpleValueEnumerationSources(types)
	if literalCount == 0 {
		return nil
	}
	populateSimpleValueEnumerationReads(types, pool, literalCount)
	return pool
}

func collectSimpleValueEnumerationSources(types []SimpleType) (map[simpleValueEnumerationSource][]simpleValueLiteralRead, int) {
	var pool map[simpleValueEnumerationSource][]simpleValueLiteralRead
	literalCount := 0
	for i := range types {
		if !simpleTypeNeedsColdRead(types[i]) {
			continue
		}
		literals := types[i].Facets.Enumeration
		source, present := simpleValueEnumerationSourceForLiterals(literals)
		if !present {
			continue
		}
		if _, exists := pool[source]; exists {
			continue
		}
		if pool == nil {
			pool = make(map[simpleValueEnumerationSource][]simpleValueLiteralRead)
		}
		pool[source] = nil
		literalCount = addSimpleValueEnumerationReadCount(literalCount, len(literals))
	}
	return pool, literalCount
}

func populateSimpleValueEnumerationReads(
	types []SimpleType,
	pool map[simpleValueEnumerationSource][]simpleValueLiteralRead,
	literalCount int,
) {
	literalReads := make([]simpleValueLiteralRead, literalCount)
	offset := 0
	for i := range types {
		literals := types[i].Facets.Enumeration
		source, present := simpleValueEnumerationSourceForLiterals(literals)
		reads, exists := pool[source]
		if !present || !exists || reads != nil {
			continue
		}
		end := offset + len(literals)
		reads = literalReads[offset:end:end]
		offset = end
		for i := range literals {
			reads[i] = newSimpleValueLiteralRead(literals[i])
		}
		pool[source] = reads
	}
}

func addSimpleValueEnumerationReadCount(total, count int) int {
	if count > math.MaxInt-total {
		panic("simple value enumeration read projection size exceeds int capacity")
	}
	return total + count
}

func simpleValueEnumerationSourceForLiterals(in []CompiledLiteral) (simpleValueEnumerationSource, bool) {
	if len(in) == 0 {
		return simpleValueEnumerationSource{}, false
	}
	return simpleValueEnumerationSource{first: &in[0], length: len(in)}, true
}

func simpleValueEnumerationReadForLiterals(in []simpleValueLiteralRead) (simpleValueEnumerationRead, bool) {
	if len(in) == 0 {
		return simpleValueEnumerationRead{}, false
	}
	return simpleValueEnumerationRead{first: &in[0], length: len(in)}, true
}

func simpleValueBoundReadIndexes(types []SimpleType) map[*CompiledLiteral]uint32 {
	var indexes map[*CompiledLiteral]uint32
	for i := range types {
		if !simpleTypeNeedsColdRead(types[i]) {
			continue
		}
		for _, literal := range types[i].Facets.bounds {
			indexes = addSimpleValueBoundReadIndex(indexes, literal)
		}
	}
	return indexes
}

func addSimpleValueBoundReadIndex(indexes map[*CompiledLiteral]uint32, literal *CompiledLiteral) map[*CompiledLiteral]uint32 {
	if literal == nil {
		return indexes
	}
	if _, exists := indexes[literal]; exists {
		return indexes
	}
	if indexes == nil {
		indexes = make(map[*CompiledLiteral]uint32)
	}
	if len(indexes) >= int(invalidID) {
		panic("too many simple value bound reads")
	}
	indexes[literal] = uint32(len(indexes)) //nolint:gosec // guarded against the invalidID sentinel above.
	return indexes
}

func simpleTypeNeedsColdRead(st SimpleType) bool {
	return !st.Missing && (len(st.Union) != 0 || st.Facets.Present != 0)
}

func (t *simpleTypeColdReadTable) read(id SimpleTypeID) (*simpleValueColdRead, bool) {
	if t == nil {
		return nil, false
	}
	if !ValidSimpleTypeID(id, len(t.index)) {
		return nil, false
	}
	idx := t.index[id]
	if idx == invalidID {
		return nil, true
	}
	if !ValidUint32Index(idx, len(t.values)) {
		return nil, false
	}
	return &t.values[idx], true
}

func (t *simpleTypeColdReadTable) unionMembers(id SimpleTypeID) ([]SimpleTypeID, bool) {
	read, ok := t.read(id)
	if !ok {
		return nil, false
	}
	if read == nil {
		return nil, true
	}
	return read.union, true
}

func newSimpleValueRouteReadsForSimpleTypes(types []SimpleType) []simpleValueRouteRead {
	reads := make([]simpleValueRouteRead, len(types))
	for i := range types {
		reads[i] = newSimpleValueRouteReadForSimpleType(types[i])
	}
	availability := simpleTypeAvailabilities(types)
	for i := range reads {
		reads[i].availability = availability[i]
	}
	return reads
}

func newSimpleValueRouteReadForSimpleType(st SimpleType) simpleValueRouteRead {
	if st.Missing {
		return simpleValueRouteRead{availability: simpleTypeAvailabilityUnavailable}
	}
	read := simpleValueRouteRead{
		minInclusive: rawDecimalBoundFacet(st.Facets, FacetMinInclusive),
		maxInclusive: rawDecimalBoundFacet(st.Facets, FacetMaxInclusive),
		listItem:     st.ListItem,
		facets:       st.Facets.Present,
		variety:      st.Variety,
		primitive:    st.Primitive,
		builtin:      st.Builtin,
		whitespace:   st.Whitespace,
		identity:     st.Identity,
		fast:         st.Fast,
		availability: simpleTypeAvailabilityAvailable,
		present:      true,
	}
	typ := read.simpleValueType()
	read.rawBypass = SimpleValueBypass(simpleValueAtomicBypassShape(&typ, 0))
	return read
}

func simpleTypeAvailabilities(types []SimpleType) []simpleTypeAvailability {
	audit := simpleTypeAvailabilityAudit{
		types:        types,
		availability: make([]simpleTypeAvailability, len(types)),
		state:        make([]simpleTypeGraphState, len(types)),
		stack:        make([]simpleTypeAvailabilityFrame, 0, min(len(types), 1_024)),
	}
	for root := range types {
		if audit.state[root] != simpleTypeGraphUnchecked {
			continue
		}
		audit.push(SimpleTypeID(root))
		for len(audit.stack) != 0 {
			audit.advance()
		}
	}
	return audit.availability
}

type simpleTypeAvailabilityAudit struct {
	types        []SimpleType
	availability []simpleTypeAvailability
	state        []simpleTypeGraphState
	stack        []simpleTypeAvailabilityFrame
}

func (a *simpleTypeAvailabilityAudit) advance() {
	last := len(a.stack) - 1
	frame := &a.stack[last]
	typ := a.types[frame.id]
	if typ.Missing {
		a.availability[frame.id] = simpleTypeAvailabilityUnavailable
		a.complete(last, frame.id)
		return
	}
	if !validSimpleTypeAvailabilityShape(typ) {
		a.complete(last, frame.id)
		return
	}
	dependency, ok := simpleTypeAvailabilityDependency(typ, frame.next)
	if !ok {
		a.completeAvailable(last, frame.id)
		return
	}
	a.advanceDependency(last, frame, dependency)
}

func (a *simpleTypeAvailabilityAudit) advanceDependency(
	last int,
	frame *simpleTypeAvailabilityFrame,
	dependency SimpleTypeID,
) {
	if !ValidSimpleTypeID(dependency, len(a.types)) {
		a.complete(last, frame.id)
		return
	}
	switch a.state[dependency] {
	case simpleTypeGraphUnchecked:
		a.push(dependency)
		return
	case simpleTypeGraphChecking:
		a.complete(last, frame.id)
		return
	case simpleTypeGraphChecked:
	}
	if a.availability[dependency] == simpleTypeAvailabilityInvalid {
		a.complete(last, frame.id)
		return
	}
	if a.availability[dependency] == simpleTypeAvailabilityUnavailable {
		a.availability[frame.id] = simpleTypeAvailabilityUnavailable
	}
	frame.next++
}

func (a *simpleTypeAvailabilityAudit) push(id SimpleTypeID) {
	a.state[id] = simpleTypeGraphChecking
	a.stack = appendDFSFrame(a.stack, simpleTypeAvailabilityFrame{id: id}, len(a.types))
}

func (a *simpleTypeAvailabilityAudit) completeAvailable(last int, id SimpleTypeID) {
	if a.availability[id] == simpleTypeAvailabilityInvalid {
		a.availability[id] = simpleTypeAvailabilityAvailable
	}
	a.complete(last, id)
}

func (a *simpleTypeAvailabilityAudit) complete(last int, id SimpleTypeID) {
	a.state[id] = simpleTypeGraphChecked
	a.stack = a.stack[:last]
}

func validSimpleTypeAvailabilityShape(st SimpleType) bool {
	return st.Variety == SimpleVarietyAtomic || st.Variety == SimpleVarietyList || st.Variety == SimpleVarietyUnion
}

func simpleTypeAvailabilityDependency(st SimpleType, index int) (SimpleTypeID, bool) {
	if st.Base != NoSimpleType {
		if index == 0 {
			return st.Base, true
		}
		index--
	}
	switch st.Variety {
	case SimpleVarietyAtomic:
	case SimpleVarietyList:
		if index == 0 {
			return st.ListItem, true
		}
	case SimpleVarietyUnion:
		if index < len(st.Union) {
			return st.Union[index], true
		}
	}
	return NoSimpleType, false
}

type simpleTypeAvailabilityFrame struct {
	id   SimpleTypeID
	next int
}

func (r simpleValueRouteRead) simpleValueType() SimpleValueType {
	return SimpleValueType{
		DecimalMinInclusive: r.minInclusive,
		DecimalMaxInclusive: r.maxInclusive,
		ListItem:            r.listItem,
		Facets:              r.facets,
		Variety:             r.variety,
		Primitive:           r.primitive,
		Builtin:             r.builtin,
		Whitespace:          r.whitespace,
		Identity:            r.identity,
		Fast:                r.fast,
		RawBypass:           r.rawBypass,
	}
}

func simpleValueTypeForRouteAndCold(route *simpleValueRouteRead, cold *simpleValueColdRead) SimpleValueType {
	typ := route.simpleValueType()
	if cold == nil {
		return typ
	}
	typ.UnionMembers = cold.union
	typ.StringFacets = StringFacetValues{patternReads: cold.facets.patterns, HasEnumeration: len(cold.enumeration) != 0}
	typ.DecimalFacets = cold.facets.decimalValues()
	typ.LengthFacets = cold.facets.lengthValues()
	return typ
}

func simpleValueFacetsForColdRead(cold *simpleValueColdRead) SimpleValueFacets {
	if cold == nil {
		return SimpleValueFacets{}
	}
	f := cold.facets
	return SimpleValueFacets{
		MinInclusive:     f.literal(FacetMinInclusive),
		MaxInclusive:     f.literal(FacetMaxInclusive),
		MinExclusive:     f.literal(FacetMinExclusive),
		MaxExclusive:     f.literal(FacetMaxExclusive),
		StringFacets:     StringFacetValues{patternReads: f.patterns, HasEnumeration: len(cold.enumeration) != 0},
		DecimalFacets:    f.decimalValues(),
		LengthFacets:     f.lengthValues(),
		Facets:           f.present,
		enumerationReads: cold.enumeration,
	}
}

func simpleValueRouteReadByID(reads []simpleValueRouteRead, id SimpleTypeID) (*simpleValueRouteRead, bool) {
	read, ok := simpleValueRouteSlotByID(reads, id)
	if !ok || !read.present {
		return nil, false
	}
	return read, true
}

func simpleValueRouteSlotByID(reads []simpleValueRouteRead, id SimpleTypeID) (*simpleValueRouteRead, bool) {
	if !ValidSimpleTypeID(id, len(reads)) {
		return nil, false
	}
	return &reads[id], true
}

// newSimpleValueQNameResolverNeedsForSimpleTypes precomputes whether each
// published simple type can require lexical QName namespace resolution.
func newSimpleValueQNameResolverNeedsForSimpleTypes(types []SimpleType) []bool {
	audit := simpleValueQNameAudit{
		types: types,
		out:   make([]bool, len(types)),
		state: make([]simpleValueQNameState, len(types)),
		stack: make([]simpleValueQNameFrame, 0, min(len(types), 1_024)),
	}
	for root := range types {
		if audit.state[root] != simpleValueQNameUnchecked {
			continue
		}
		audit.push(SimpleTypeID(root))
		for len(audit.stack) != 0 {
			audit.advance()
		}
	}
	return audit.out
}

type simpleValueQNameAudit struct {
	types []SimpleType
	out   []bool
	state []simpleValueQNameState
	stack []simpleValueQNameFrame
}

func (a *simpleValueQNameAudit) advance() {
	last := len(a.stack) - 1
	frame := &a.stack[last]
	typ := a.types[frame.id]
	switch typ.Variety {
	case SimpleVarietyAtomic:
		a.advanceAtomic(last, frame.id, typ)
	case SimpleVarietyList:
		a.advanceList(last, frame.id, typ)
	case SimpleVarietyUnion:
		a.advanceUnion(last, frame, typ)
	default:
		a.complete(last, frame.id)
	}
}

func (a *simpleValueQNameAudit) advanceAtomic(last int, id SimpleTypeID, typ SimpleType) {
	a.out[id] = !typ.Missing && (typ.Primitive == PrimitiveQName || typ.Primitive == PrimitiveNotation)
	a.complete(last, id)
}

func (a *simpleValueQNameAudit) advanceList(last int, id SimpleTypeID, typ SimpleType) {
	if typ.Missing || !ValidSimpleTypeID(typ.ListItem, len(a.types)) || a.state[typ.ListItem] == simpleValueQNameChecking {
		a.complete(last, id)
		return
	}
	if a.state[typ.ListItem] == simpleValueQNameUnchecked {
		a.push(typ.ListItem)
		return
	}
	a.out[id] = a.out[typ.ListItem]
	a.complete(last, id)
}

func (a *simpleValueQNameAudit) advanceUnion(last int, frame *simpleValueQNameFrame, typ SimpleType) {
	if typ.Missing {
		a.complete(last, frame.id)
		return
	}
	if a.visitUnionMembers(frame, typ.Union) {
		return
	}
	a.complete(last, frame.id)
}

func (a *simpleValueQNameAudit) visitUnionMembers(frame *simpleValueQNameFrame, members []SimpleTypeID) bool {
	for frame.next < len(members) {
		member := members[frame.next]
		if !ValidSimpleTypeID(member, len(a.types)) || a.state[member] == simpleValueQNameChecking {
			frame.next++
			continue
		}
		if a.state[member] == simpleValueQNameUnchecked {
			a.push(member)
			return true
		}
		frame.next++
		if a.out[member] {
			a.out[frame.id] = true
			break
		}
	}
	return false
}

func (a *simpleValueQNameAudit) push(id SimpleTypeID) {
	a.state[id] = simpleValueQNameChecking
	a.stack = appendDFSFrame(a.stack, simpleValueQNameFrame{id: id}, len(a.types))
}

func (a *simpleValueQNameAudit) complete(last int, id SimpleTypeID) {
	a.state[id] = simpleValueQNameChecked
	a.stack = a.stack[:last]
}

type simpleValueQNameState uint8

const (
	simpleValueQNameUnchecked simpleValueQNameState = iota
	simpleValueQNameChecking
	simpleValueQNameChecked
)

type simpleValueQNameFrame struct {
	id   SimpleTypeID
	next int
}

// SimpleValueTypeForSimpleType returns validation type facts borrowed from a
// simple type record.
func SimpleValueTypeForSimpleType(st SimpleType) SimpleValueType {
	typ := SimpleValueType{
		DecimalMinInclusive: rawDecimalBoundFacet(st.Facets, FacetMinInclusive),
		DecimalMaxInclusive: rawDecimalBoundFacet(st.Facets, FacetMaxInclusive),
		DecimalFacets:       decimalFacetValues(st.Facets),
		LengthFacets:        lengthFacetValues(st.Facets),
		StringFacets: StringFacetValues{
			patternSource:  st.Facets.patterns,
			HasEnumeration: len(st.Facets.Enumeration) != 0,
		},
		UnionMembers: st.Union,
		ListItem:     st.ListItem,
		Facets:       st.Facets.Present,
		Variety:      st.Variety,
		Primitive:    st.Primitive,
		Builtin:      st.Builtin,
		Whitespace:   st.Whitespace,
		Identity:     st.Identity,
		Fast:         st.Fast,
	}
	typ.RawBypass = SimpleValueBypass(simpleValueAtomicBypassShape(&typ, 0))
	return typ
}

// Project returns validation facet facts for f.
func (p *SimpleValueFacetProjector) Project(f FacetSet) SimpleValueFacets {
	enumeration := p.enumeration(f.Enumeration)
	return simpleValueFacetsForFacetSet(f, enumeration)
}

func (p *SimpleValueFacetProjector) enumeration(source []CompiledLiteral) []SimpleValueFacetLiteral {
	key, present := simpleValueEnumerationSourceForLiterals(source)
	if !present {
		return nil
	}
	if projection, ok := p.enumerations[key]; ok {
		return projection
	}
	projection := newSimpleValueFacetLiterals(source)
	if p.enumerations == nil {
		p.enumerations = make(map[simpleValueEnumerationSource][]SimpleValueFacetLiteral)
	}
	p.enumerations[key] = projection
	return projection
}

func simpleValueFacetsForFacetSet(f FacetSet, enumeration []SimpleValueFacetLiteral) SimpleValueFacets {
	if len(enumeration) != len(f.Enumeration) {
		panic("simple value facet enumeration projection length does not match source")
	}
	return SimpleValueFacets{
		MinInclusive: simpleValueBoundFacetLiteral(f, FacetMinInclusive),
		MaxInclusive: simpleValueBoundFacetLiteral(f, FacetMaxInclusive),
		MinExclusive: simpleValueBoundFacetLiteral(f, FacetMinExclusive),
		MaxExclusive: simpleValueBoundFacetLiteral(f, FacetMaxExclusive),
		Enumeration:  enumeration,
		StringFacets: StringFacetValues{
			patternSource: f.patterns,
		},
		DecimalFacets: decimalFacetValues(f),
		LengthFacets:  lengthFacetValues(f),
		Facets:        f.Present,
	}
}

func simpleValueBoundFacetLiteral(f FacetSet, flag FacetMask) SimpleValueFacetLiteral {
	lit, present := BoundFacet(f, flag)
	if !present {
		return SimpleValueFacetLiteral{}
	}
	return SimpleValueFacetLiteral{
		Canonical: lit.Canonical,
		Actual:    lit.Actual,
		Present:   true,
	}
}

func newSimpleValueFacetLiterals(in []CompiledLiteral) []SimpleValueFacetLiteral {
	out := make([]SimpleValueFacetLiteral, len(in))
	for i := range in {
		out[i] = SimpleValueFacetLiteral{Canonical: in[i].Canonical, Actual: in[i].Actual, Present: true}
	}
	return slices.Clip(out)
}

func lengthFacetValues(f FacetSet) LengthFacetValues {
	return LengthFacetValues{
		Length:    facetCardinalityValue(f.Present, FacetLength, f.Length),
		MinLength: facetCardinalityValue(f.Present, FacetMinLength, f.MinLength),
		MaxLength: facetCardinalityValue(f.Present, FacetMaxLength, f.MaxLength),
	}
}

func decimalFacetValues(f FacetSet) DecimalFacetValues {
	return DecimalFacetValues{
		MinInclusive:   decimalBoundFacetValue(f, FacetMinInclusive),
		MaxInclusive:   decimalBoundFacetValue(f, FacetMaxInclusive),
		MinExclusive:   decimalBoundFacetValue(f, FacetMinExclusive),
		MaxExclusive:   decimalBoundFacetValue(f, FacetMaxExclusive),
		TotalDigits:    facetCardinalityValue(f.Present, FacetTotalDigits, f.TotalDigits),
		FractionDigits: facetCardinalityValue(f.Present, FacetFractionDigits, f.FractionDigits),
		Facets:         f.Present,
	}
}

func decimalBoundFacetValue(f FacetSet, flag FacetMask) DecimalFacetValue {
	lit, present := BoundFacet(f, flag)
	if !present {
		return DecimalFacetValue{}
	}
	return DecimalFacetValue{
		Value:   lit.Actual.Decimal,
		Present: lit.Actual.Valid && lit.Actual.Kind == PrimitiveDecimal,
	}
}

func facetCardinalityValue(present, flag FacetMask, value uint32) FacetCardinalityValue {
	if present&flag == 0 {
		return FacetCardinalityValue{}
	}
	return FacetCardinalityValue{Value: value, Present: true}
}

func rawDecimalBoundFacet(f FacetSet, flag FacetMask) RawDecimalBound {
	lit, present := BoundFacet(f, flag)
	if !present {
		return RawDecimalBound{}
	}
	return lit.Actual.Decimal.RawBound()
}

// AtomicSimpleValueResult is the runtime fallback validation result needed by
// runtime-owned atomic simple-value projection.
type AtomicSimpleValueResult struct {
	Canonical         string
	IdentityCanonical string
}

// ValidateSimpleValue validates lexical text using runtime-owned normalization,
// list splitting, route, list-recursion, union-recursion, primitive parsing,
// and facet execution policy.
func ValidateSimpleValue(cb SimpleValueCallbacks, id SimpleTypeID, lexical string, needs SimpleValueNeed) (SimpleValue, error) {
	return validateSimpleValueWithReader(callbackSimpleValueMetadataReader{callbacks: cb}, id, lexical, cb.ResolveQName, needs, nil)
}

func validateSimpleValueWithReader[R simpleValueReader](reader R, id SimpleTypeID, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed, scratch *StringPatternScratch) (SimpleValue, error) {
	validator := simpleValueValidator[R]{reader: reader, resolve: resolve, needs: needs, scratch: scratch}
	return validator.validate(id, lexical)
}

type simpleValueValidator[R simpleValueReader] struct {
	reader  R
	resolve ResolveQNameParts
	scratch *StringPatternScratch
	needs   SimpleValueNeed
}

func (v simpleValueValidator[R]) validate(id SimpleTypeID, lexical string) (SimpleValue, error) {
	var typ SimpleValueType
	known := false
	if id != NoSimpleType {
		typ, known = v.reader.simpleValueType(id)
	}
	switch SimpleValueRoute(SimpleValueRouteShape{Type: id, Variety: typ.Variety, Known: known}) {
	case SimpleValueRouteUntyped:
		return SimpleValue{Canonical: lexical, Type: NoSimpleType}, nil
	case SimpleValueRouteAtomic:
		return v.validateAtomic(id, typ, lexical)
	case SimpleValueRouteList:
		return v.validateList(id, typ, lexical)
	case SimpleValueRouteUnion:
		return v.validateUnion(id, typ, lexical)
	case SimpleValueRouteMissing, SimpleValueRouteInvalid:
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	return SimpleValue{}, ErrSimpleValueMetadata
}

func (v simpleValueValidator[R]) validateAtomic(id SimpleTypeID, typ SimpleValueType, lexical string) (SimpleValue, error) {
	normalized := normalizeSimpleValueLexical(lexical, typ.Whitespace)
	bypass := SimpleValueBypass(simpleValueAtomicBypassShape(&typ, v.needs))
	if bypass == SimpleValueBypassNone {
		return v.validateAtomicWithoutBypass(id, typ, normalized)
	}
	if bypass == SimpleValueBypassValidateDecimal {
		return validateAtomicDecimalSimpleValue(id, typ, normalized)
	}
	return v.validateAtomicBypass(id, typ, normalized, bypass)
}

func (v simpleValueValidator[R]) validateAtomicBypass(
	id SimpleTypeID,
	typ SimpleValueType,
	normalized string,
	bypass SimpleValueBypassAction,
) (SimpleValue, error) {
	switch bypass {
	case SimpleValueBypassAcceptString:
		return unconstrainedStringSimpleValue(id, normalized, v.needs), nil
	case SimpleValueBypassValidateInt:
		return validatedAtomicSimpleValue(id, ValidateFastIntLexical(normalized))
	case SimpleValueBypassValidateStringPatterns, SimpleValueBypassValidateStringEnumeration:
		return validatedAtomicSimpleValue(id, v.validateStringFacets(id, typ, normalized, normalized))
	case SimpleValueBypassValidateAnyURI:
		_, err := uriref.Check(normalized)
		return validatedAtomicSimpleValue(id, err)
	case SimpleValueBypassValidateHexBinary:
		return validatedAtomicSimpleValue(id, ValidateHexBinaryLexical(normalized))
	case SimpleValueBypassValidateBase64Binary:
		return validatedAtomicSimpleValue(id, ValidateBase64BinaryLexical(normalized))
	case SimpleValueBypassValidateFloat:
		return validatedAtomicSimpleValue(id, ValidateFloatLexical(normalized, simpleValueFloatBits(typ.Primitive)))
	case SimpleValueBypassValidateDuration:
		return validatedAtomicSimpleValue(id, ValidateDurationLexical(normalized))
	case SimpleValueBypassValidateBoolean:
		return validatedAtomicSimpleValue(id, ValidateBooleanLexical(normalized))
	case SimpleValueBypassValidateTemporal:
		return validatedAtomicSimpleValue(id, ValidateTemporalLexical(typ.Primitive, normalized))
	case SimpleValueBypassValidateDate:
		return validatedAtomicSimpleValue(id, validateDateLexical(normalized))
	case SimpleValueBypassNone, SimpleValueBypassValidateDecimal:
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	return SimpleValue{}, ErrSimpleValueMetadata
}

func validatedAtomicSimpleValue(id SimpleTypeID, err error) (SimpleValue, error) {
	if err != nil {
		return SimpleValue{}, err
	}
	return SimpleValue{Type: id}, nil
}

func (v simpleValueValidator[R]) validateAtomicWithoutBypass(
	id SimpleTypeID,
	typ SimpleValueType,
	normalized string,
) (SimpleValue, error) {
	if value, ok, err := validateAtomicStringSimpleValueFallback(id, typ, normalized, v.needs); ok {
		return value, err
	}
	return v.validateAtomicFallback(id, typ, normalized)
}

func validateAtomicDecimalSimpleValue(id SimpleTypeID, typ SimpleValueType, normalized string) (SimpleValue, error) {
	handled, err := ValidateFastDecimalLexical(RawDecimalFastPathShape{
		MinInclusive: typ.DecimalMinInclusive,
		MaxInclusive: typ.DecimalMaxInclusive,
		Facets:       typ.Facets,
	}, normalized)
	if err != nil {
		return SimpleValue{}, err
	}
	if handled {
		return SimpleValue{Type: id}, nil
	}
	decimal, err := ParseDecimalValue(normalized)
	if err != nil {
		return SimpleValue{}, err
	}
	return validatedAtomicSimpleValue(id, ValidateDecimalFacets(typ.DecimalFacets, decimal))
}

func unconstrainedStringSimpleValue(id SimpleTypeID, normalized string, needs SimpleValueNeed) SimpleValue {
	value := SimpleValue{Type: id}
	if needs.Has(SimpleNeedCanonical) || needs.Has(SimpleNeedIdentity) {
		value.Canonical = normalized
	}
	if needs.Has(SimpleNeedIdentity) {
		value.Identity = SimpleIdentityKey(PrimitiveString, normalized)
	}
	return value
}

func validateAtomicStringSimpleValueFallback(id SimpleTypeID, typ SimpleValueType, normalized string, needs SimpleValueNeed) (SimpleValue, bool, error) {
	if typ.Primitive != PrimitiveString || typ.Facets != 0 {
		return SimpleValue{}, false, nil
	}
	if err := validateRuntimeAtomicBuiltin(typ, normalized); err != nil {
		return SimpleValue{}, true, err
	}
	canon := ""
	if needs.Has(SimpleNeedCanonical) || needs.Has(SimpleNeedIdentity) || typ.Identity != SimpleIdentityNone {
		canon = normalized
	}
	value := SimpleValue{Canonical: canon, Type: id}
	if needs.Has(SimpleNeedIdentity) {
		value.Identity = SimpleIdentityKey(PrimitiveString, normalized)
	}
	switch typ.Identity {
	case SimpleIdentityID:
		value.IDs = canon
	case SimpleIdentityIDREF:
		value.IDRefs = canon
	case SimpleIdentityNone, SimpleIdentityIDREFList:
	}
	return value, true, nil
}

func (v simpleValueValidator[R]) validateAtomicFallback(id SimpleTypeID, typ SimpleValueType, normalized string) (SimpleValue, error) {
	if err := validateRuntimeAtomicBuiltin(typ, normalized); err != nil {
		return SimpleValue{}, err
	}
	if err := validateRuntimeAtomicLengthFacets(typ, normalized); err != nil {
		return SimpleValue{}, err
	}
	facets, ok := v.reader.simpleValueFacets(id)
	if !ok {
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	result, err := validateAtomicSimpleValueFallbackWithReaderAndScratch(v.reader, AtomicSimpleValueInput{
		Type:         typ,
		Facets:       facets,
		ResolveQName: v.resolve,
		Normalized:   normalized,
		Needs: SimpleValuePrimitiveNeeds(PrimitiveValueNeedShape{
			Facets:    typ.Facets,
			Primitive: typ.Primitive,
			Builtin:   typ.Builtin,
			Identity:  typ.Identity,
			Needs:     v.needs,
		}),
		Present: true,
	}, v.scratch)
	if err != nil {
		return SimpleValue{}, err
	}
	return AtomicSimpleValue(AtomicSimpleValueProjection{
		Canonical:         result.Canonical,
		IdentityCanonical: result.IdentityCanonical,
		Type:              id,
		Primitive:         typ.Primitive,
		Identity:          typ.Identity,
		Needs:             v.needs,
	}), nil
}

func validateRuntimeAtomicBuiltin(typ SimpleValueType, normalized string) error {
	if !SimpleValueBuiltinDerivedRuntimeOwned(typ.Builtin) {
		return nil
	}
	if typ.Builtin == BuiltinValidationInteger {
		return ValidateIntegerLexical(normalized)
	}
	return ValidateBuiltinDerived(BuiltinDerivedInput{
		Kind: typ.Builtin,
		Norm: normalized,
	})
}

func validateRuntimeAtomicLengthFacets(typ SimpleValueType, normalized string) error {
	if !SimpleValueAtomicLengthFacets(AtomicLengthFacetShape{Facets: typ.Facets, Primitive: typ.Primitive, Builtin: typ.Builtin}) {
		return nil
	}
	length, err := PrimitiveLength(typ.Primitive, normalized)
	if err != nil {
		return err
	}
	return validateSimpleValueLengthFacets(typ, length)
}

func simpleValueAtomicBypassShape(typ *SimpleValueType, needs SimpleValueNeed) SimpleValueBypassShape {
	return SimpleValueBypassShape{
		Facets:    typ.Facets,
		Variety:   typ.Variety,
		Primitive: typ.Primitive,
		Builtin:   typ.Builtin,
		Identity:  typ.Identity,
		Fast:      typ.Fast,
		Needs:     needs,
	}
}

func simpleValueFloatBits(kind PrimitiveKind) int {
	if kind == PrimitiveFloat {
		return 32
	}
	return 64
}

func (v simpleValueValidator[R]) validateList(id SimpleTypeID, typ SimpleValueType, lexical string) (SimpleValue, error) {
	needPlan := SimpleValueListNeeds(ListSimpleValueNeedShape{
		Facets:   typ.Facets,
		Identity: typ.Identity,
		Needs:    v.needs,
	})
	var values listSimpleValueAccumulator
	if err := v.collectListItems(typ.ListItem, lexical, needPlan, &values); err != nil {
		return SimpleValue{}, err
	}
	canonical, normalized := values.strings(needPlan)
	if err := v.validateListFacets(id, typ, canonical, normalized, values.count); err != nil {
		return SimpleValue{}, err
	}
	return ListSimpleValue(ListSimpleValueProjection{
		Canonical:    canonical,
		ItemIDRefs:   values.refs.String(),
		ItemIdentity: values.identity.String(),
		Type:         id,
		Needs:        v.needs,
	}), nil
}

type listSimpleValueAccumulator struct {
	canonical  strings.Builder
	normalized strings.Builder
	refs       strings.Builder
	identity   strings.Builder
	count      uint32
}

func (v simpleValueValidator[R]) collectListItems(
	itemType SimpleTypeID,
	lexical string,
	plan ListSimpleValueNeedPlan,
	values *listSimpleValueAccumulator,
) error {
	itemValidator := v
	itemValidator.needs = plan.ItemNeeds
	for item := range lex.XMLFieldsSeq(lexical) {
		itemValue, err := itemValidator.validate(itemType, item)
		if err != nil {
			return err
		}
		if !values.append(item, itemValue, v.needs, plan) {
			return ErrSimpleValueMetadata
		}
	}
	return nil
}

func (a *listSimpleValueAccumulator) append(
	normalized string,
	value SimpleValue,
	needs SimpleValueNeed,
	plan ListSimpleValueNeedPlan,
) bool {
	if plan.NeedStrings {
		if a.count > 0 {
			a.canonical.WriteByte(' ')
			a.normalized.WriteByte(' ')
		}
		a.canonical.WriteString(value.Canonical)
		a.normalized.WriteString(normalized)
	}
	AppendSimpleValueIDRefs(&a.refs, value)
	if needs.Has(SimpleNeedIdentity) && !AppendSimpleValueListIdentity(&a.identity, value) {
		return false
	}
	a.count++
	return true
}

func (a *listSimpleValueAccumulator) strings(plan ListSimpleValueNeedPlan) (canonical, normalized string) {
	if !plan.NeedStrings {
		return "", ""
	}
	return a.canonical.String(), a.normalized.String()
}

func (v simpleValueValidator[R]) validateListFacets(
	id SimpleTypeID,
	typ SimpleValueType,
	canonical, normalized string,
	count uint32,
) error {
	facetPlan := SimpleValueListFacetPlan(typ.Facets)
	if facetPlan.ValidateLength {
		if err := validateSimpleValueLengthFacets(typ, count); err != nil {
			return err
		}
	}
	if facetPlan.ValidateLexical {
		if err := v.validateStringFacets(id, typ, normalized, canonical); err != nil {
			return err
		}
	}
	return nil
}

// ValidateLengthFacets validates length/minLength/maxLength against a computed
// length.
func ValidateLengthFacets(facets LengthFacetValues, length uint32) error {
	if facets.Length.Present && length != facets.Length.Value {
		return errors.New("length facet failed")
	}
	if facets.MinLength.Present && length < facets.MinLength.Value {
		return errors.New("minLength facet failed")
	}
	if facets.MaxLength.Present && length > facets.MaxLength.Value {
		return errors.New("maxLength facet failed")
	}
	return nil
}

func validateSimpleValueLengthFacets(typ SimpleValueType, length uint32) error {
	if typ.Facets&FacetLength != 0 && !typ.LengthFacets.Length.Present {
		return ErrSimpleValueMetadata
	}
	if typ.Facets&FacetMinLength != 0 && !typ.LengthFacets.MinLength.Present {
		return ErrSimpleValueMetadata
	}
	if typ.Facets&FacetMaxLength != 0 && !typ.LengthFacets.MaxLength.Present {
		return ErrSimpleValueMetadata
	}
	return ValidateLengthFacets(typ.LengthFacets, length)
}

func (v simpleValueValidator[R]) validateUnion(id SimpleTypeID, typ SimpleValueType, lexical string) (SimpleValue, error) {
	if len(typ.UnionMembers) == 0 {
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	normalized := normalizeSimpleValueLexical(lexical, typ.Whitespace)
	memberNeeds := SimpleValueUnionMemberNeeds(UnionSimpleValueNeedShape{
		Facets:   typ.Facets,
		Identity: typ.Identity,
		Needs:    v.needs,
	})
	match, err := v.matchUnion(id, typ, normalized, memberNeeds)
	if err != nil {
		return SimpleValue{}, err
	}
	if match.matched {
		return match.value, nil
	}
	if match.unsupported != nil {
		return SimpleValue{}, match.unsupported
	}
	return SimpleValue{}, errors.New("value does not match any union member")
}

type unionSimpleValueMatch struct {
	unsupported error
	value       SimpleValue
	matched     bool
}

func (v simpleValueValidator[R]) matchUnion(
	id SimpleTypeID,
	typ SimpleValueType,
	normalized string,
	memberNeeds SimpleValueNeed,
) (unionSimpleValueMatch, error) {
	var unsupported error
	memberValidator := v
	memberValidator.needs = memberNeeds
	for _, member := range typ.UnionMembers {
		value, err := memberValidator.validate(member, normalized)
		if err == nil {
			facetErr := v.validateUnionFacets(id, typ, normalized, value.Canonical)
			return unionSimpleValueMatch{value: value, unsupported: unsupported, matched: true}, facetErr
		}
		if unsupported == nil && v.reader.simpleValueUnsupported(err) {
			unsupported = err
		}
	}
	return unionSimpleValueMatch{unsupported: unsupported}, nil
}

func (v simpleValueValidator[R]) validateUnionFacets(
	id SimpleTypeID,
	typ SimpleValueType,
	normalized, canonical string,
) error {
	if !SimpleValueUnionFacetValidation(typ.Facets) {
		return nil
	}
	return v.validateStringFacets(id, typ, normalized, canonical)
}

func (v simpleValueValidator[R]) validateStringFacets(id SimpleTypeID, typ SimpleValueType, normalized, canonical string) error {
	if typ.Facets&FacetPattern != 0 && typ.StringFacets.patternCount() == 0 {
		return ErrSimpleValueMetadata
	}
	if typ.Facets&FacetEnumeration != 0 && !typ.StringFacets.HasEnumeration {
		return ErrSimpleValueMetadata
	}
	if err := typ.StringFacets.validatePatterns(normalized, v.scratch); err != nil {
		return err
	}
	if typ.StringFacets.HasEnumeration {
		matched, ok := v.reader.simpleValueStringEnumeration(id, canonical)
		if !ok {
			return ErrSimpleValueMetadata
		}
		if matched {
			return nil
		}
		return errors.New("enumeration facet failed")
	}
	return nil
}

func normalizeSimpleValueLexical(lexical string, mode WhitespaceMode) string {
	switch mode {
	case WhitespacePreserve:
		return lexical
	case WhitespaceReplace:
		return lex.ReplaceXMLWhitespace(lexical)
	case WhitespaceCollapse:
		return lex.CollapseXMLWhitespace(lexical)
	default:
	}
	return lex.CollapseXMLWhitespace(lexical)
}
