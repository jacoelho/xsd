package runtime

import (
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
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
	simpleValueNotation(ns, local string) (bool, bool)
	simpleValueUnsupported(err error) bool
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

func (r callbackSimpleValueMetadataReader) simpleValueStringEnumeration(id SimpleTypeID, canonical string) (bool, bool) {
	if r.callbacks.StringEnumeration == nil {
		return false, false
	}
	return r.callbacks.StringEnumeration(id, canonical)
}

func (r callbackSimpleValueMetadataReader) simpleValueNotation(ns, local string) (bool, bool) {
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

func (f StringFacetValues) validatePatterns(normalized string) error {
	if f.patternReads != nil {
		return validateStringPatternStepReads(f.patternReads, normalized)
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
	Enumeration   []SimpleValueFacetLiteral
	StringFacets  StringFacetValues
	enumeration   []simpleValueLiteralRead
	MinInclusive  SimpleValueFacetLiteral
	MaxInclusive  SimpleValueFacetLiteral
	MinExclusive  SimpleValueFacetLiteral
	MaxExclusive  SimpleValueFacetLiteral
	DecimalFacets DecimalFacetValues
	LengthFacets  LengthFacetValues
	Facets        FacetMask
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
	present      bool
}

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
	return lit.literal(present)
}

func newSimpleValueLiteralRead(lit CompiledLiteral) simpleValueLiteralRead {
	return simpleValueLiteralRead{canonical: lit.Canonical, actual: lit.Actual}
}

func (r simpleValueLiteralRead) literal(present bool) SimpleValueFacetLiteral {
	if !present {
		return SimpleValueFacetLiteral{}
	}
	return SimpleValueFacetLiteral{Canonical: r.canonical, Actual: r.actual, Present: true}
}

func (f simpleValueFacetRead) lengthValues() LengthFacetValues {
	return LengthFacetValues{
		Length:    facetCardinalityValue(f.length, f.present&FacetLength != 0),
		MinLength: facetCardinalityValue(f.minLength, f.present&FacetMinLength != 0),
		MaxLength: facetCardinalityValue(f.maxLength, f.present&FacetMaxLength != 0),
	}
}

func (f simpleValueFacetRead) decimalValues() DecimalFacetValues {
	return DecimalFacetValues{
		MinInclusive:   f.decimalBound(FacetMinInclusive),
		MaxInclusive:   f.decimalBound(FacetMaxInclusive),
		MinExclusive:   f.decimalBound(FacetMinExclusive),
		MaxExclusive:   f.decimalBound(FacetMaxExclusive),
		TotalDigits:    facetCardinalityValue(f.totalDigits, f.present&FacetTotalDigits != 0),
		FractionDigits: facetCardinalityValue(f.fractionDigits, f.present&FacetFractionDigits != 0),
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

type simpleValueColdReadTable struct {
	index      []uint32
	values     []simpleValueColdRead
	boundReads []simpleValueLiteralRead
}

func newSimpleValueColdReadTable(types []SimpleType) simpleValueColdReadTable {
	count := 0
	for i := range types {
		if simpleValueTypeNeedsColdRead(types[i]) {
			count++
		}
	}
	table := simpleValueColdReadTable{
		index:  make([]uint32, len(types)),
		values: make([]simpleValueColdRead, 0, count),
	}
	boundIndexes := simpleValueBoundReadIndexes(types)
	table.boundReads = make([]simpleValueLiteralRead, len(boundIndexes))
	for source, index := range boundIndexes {
		table.boundReads[index] = newSimpleValueLiteralRead(*source)
	}
	for i := range table.index {
		table.index[i] = invalidID
	}
	enumerationPool := newSimpleValueEnumerationReadPool(types)
	patternPool := newStringPatternReadPoolForSimpleTypes(types)
	for i := range types {
		if !simpleValueTypeNeedsColdRead(types[i]) {
			continue
		}
		if len(table.values) >= int(invalidID) {
			panic("too many simple value cold reads")
		}
		table.index[i] = uint32(len(table.values)) //nolint:gosec // guarded against the invalidID sentinel above.
		sourceEnumeration := types[i].Facets.Enumeration
		var enumeration []simpleValueLiteralRead
		if source, ok := simpleValueEnumerationSourceForLiterals(sourceEnumeration); ok {
			enumeration = enumerationPool[source]
		}
		var patterns *stringPatternStepRead
		if source := types[i].Facets.patterns.tail; source != nil {
			patterns = patternPool[source]
		}
		table.values = append(table.values, simpleValueColdRead{
			union:       types[i].Union,
			enumeration: enumeration,
			facets:      newSimpleValueFacetRead(types[i].Facets, table.boundReads, boundIndexes, patterns),
		})
	}
	return table
}

func newSimpleValueEnumerationReadPool(types []SimpleType) map[simpleValueEnumerationSource][]simpleValueLiteralRead {
	var pool map[simpleValueEnumerationSource][]simpleValueLiteralRead
	literalCount := 0
	for i := range types {
		if !simpleValueTypeNeedsColdRead(types[i]) {
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
	if literalCount == 0 {
		return nil
	}

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
	return pool
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
		if !simpleValueTypeNeedsColdRead(types[i]) {
			continue
		}
		for _, literal := range types[i].Facets.bounds {
			if literal == nil {
				continue
			}
			if _, exists := indexes[literal]; exists {
				continue
			}
			if indexes == nil {
				indexes = make(map[*CompiledLiteral]uint32)
			}
			if len(indexes) >= int(invalidID) {
				panic("too many simple value bound reads")
			}
			indexes[literal] = uint32(len(indexes)) //nolint:gosec // guarded against the invalidID sentinel above.
		}
	}
	return indexes
}

func simpleValueTypeNeedsColdRead(st SimpleType) bool {
	return !st.Missing && (len(st.Union) != 0 || st.Facets.Present != 0)
}

func (t simpleValueColdReadTable) read(id SimpleTypeID) (*simpleValueColdRead, bool) {
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

func newSimpleValueRouteReadsForSimpleTypes(types []SimpleType) []simpleValueRouteRead {
	reads := make([]simpleValueRouteRead, len(types))
	for i := range types {
		reads[i] = newSimpleValueRouteReadForSimpleType(types[i])
	}
	return reads
}

func newSimpleValueRouteReadForSimpleType(st SimpleType) simpleValueRouteRead {
	if st.Missing {
		return simpleValueRouteRead{}
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
		present:      true,
	}
	typ := read.simpleValueType()
	read.rawBypass = SimpleValueBypass(simpleValueAtomicBypassShape(&typ, 0))
	return read
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
		MinInclusive:  f.literal(FacetMinInclusive),
		MaxInclusive:  f.literal(FacetMaxInclusive),
		MinExclusive:  f.literal(FacetMinExclusive),
		MaxExclusive:  f.literal(FacetMaxExclusive),
		StringFacets:  StringFacetValues{patternReads: f.patterns, HasEnumeration: len(cold.enumeration) != 0},
		DecimalFacets: f.decimalValues(),
		LengthFacets:  f.lengthValues(),
		Facets:        f.present,
		enumeration:   cold.enumeration,
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
	out := make([]bool, len(types))
	state := make([]simpleValueQNameState, len(types))
	stack := make([]simpleValueQNameFrame, 0, min(len(types), 1_024))
	for root := range types {
		if state[root] != simpleValueQNameUnchecked {
			continue
		}
		state[root] = simpleValueQNameChecking
		stack = appendDFSFrame(stack, simpleValueQNameFrame{id: SimpleTypeID(root)}, len(types))
		for len(stack) != 0 {
			last := len(stack) - 1
			frame := &stack[last]
			typ := types[frame.id]
			switch typ.Variety {
			case SimpleVarietyAtomic:
				out[frame.id] = !typ.Missing && (typ.Primitive == PrimitiveQName || typ.Primitive == PrimitiveNotation)
				state[frame.id] = simpleValueQNameChecked
				stack = stack[:last]
			case SimpleVarietyList:
				if typ.Missing || !ValidSimpleTypeID(typ.ListItem, len(types)) || state[typ.ListItem] == simpleValueQNameChecking {
					state[frame.id] = simpleValueQNameChecked
					stack = stack[:last]
					continue
				}
				if state[typ.ListItem] == simpleValueQNameUnchecked {
					state[typ.ListItem] = simpleValueQNameChecking
					stack = appendDFSFrame(stack, simpleValueQNameFrame{id: typ.ListItem}, len(types))
					continue
				}
				out[frame.id] = out[typ.ListItem]
				state[frame.id] = simpleValueQNameChecked
				stack = stack[:last]
			case SimpleVarietyUnion:
				if typ.Missing {
					state[frame.id] = simpleValueQNameChecked
					stack = stack[:last]
					continue
				}
				pushed := false
				for frame.next < len(typ.Union) {
					member := typ.Union[frame.next]
					if !ValidSimpleTypeID(member, len(types)) || state[member] == simpleValueQNameChecking {
						frame.next++
						continue
					}
					if state[member] == simpleValueQNameUnchecked {
						state[member] = simpleValueQNameChecking
						stack = appendDFSFrame(stack, simpleValueQNameFrame{id: member}, len(types))
						pushed = true
						break
					}
					frame.next++
					if out[member] {
						out[frame.id] = true
						break
					}
				}
				if pushed {
					continue
				}
				state[frame.id] = simpleValueQNameChecked
				stack = stack[:last]
			default:
				state[frame.id] = simpleValueQNameChecked
				stack = stack[:last]
			}
		}
	}
	return out
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
	return simpleValueFacetLiteral(lit, present)
}

func simpleValueFacetLiteral(lit CompiledLiteral, present bool) SimpleValueFacetLiteral {
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
		out[i] = simpleValueFacetLiteral(in[i], true)
	}
	return slices.Clip(out)
}

func lengthFacetValues(f FacetSet) LengthFacetValues {
	return LengthFacetValues{
		Length:    facetCardinalityValue(f.Length, f.Present&FacetLength != 0),
		MinLength: facetCardinalityValue(f.MinLength, f.Present&FacetMinLength != 0),
		MaxLength: facetCardinalityValue(f.MaxLength, f.Present&FacetMaxLength != 0),
	}
}

func decimalFacetValues(f FacetSet) DecimalFacetValues {
	return DecimalFacetValues{
		MinInclusive:   decimalBoundFacetValue(f, FacetMinInclusive),
		MaxInclusive:   decimalBoundFacetValue(f, FacetMaxInclusive),
		MinExclusive:   decimalBoundFacetValue(f, FacetMinExclusive),
		MaxExclusive:   decimalBoundFacetValue(f, FacetMaxExclusive),
		TotalDigits:    facetCardinalityValue(f.TotalDigits, f.Present&FacetTotalDigits != 0),
		FractionDigits: facetCardinalityValue(f.FractionDigits, f.Present&FacetFractionDigits != 0),
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

func facetCardinalityValue(v uint32, present bool) FacetCardinalityValue {
	if !present {
		return FacetCardinalityValue{}
	}
	return FacetCardinalityValue{Value: v, Present: true}
}

func rawDecimalBoundFacet(f FacetSet, flag FacetMask) RawDecimalBound {
	lit, present := BoundFacet(f, flag)
	return rawDecimalBound(lit, present)
}

func rawDecimalBound(lit CompiledLiteral, present bool) RawDecimalBound {
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
	return validateSimpleValue(callbackSimpleValueMetadataReader{callbacks: cb}, id, lexical, cb.ResolveQName, needs)
}

func validateSimpleValue[R simpleValueMetadataReader](reader R, id SimpleTypeID, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	var typ SimpleValueType
	known := false
	if id != NoSimpleType {
		typ, known = reader.simpleValueType(id)
	}
	switch SimpleValueRoute(SimpleValueRouteShape{Type: id, Variety: typ.Variety, Known: known}) {
	case SimpleValueRouteUntyped:
		return SimpleValue{Canonical: lexical, Type: NoSimpleType}, nil
	case SimpleValueRouteAtomic:
		return validateAtomicSimpleValue(reader, id, typ, lexical, resolve, needs)
	case SimpleValueRouteList:
		return validateListSimpleValue(reader, id, typ, lexical, resolve, needs)
	case SimpleValueRouteUnion:
		return validateUnionSimpleValue(reader, id, typ, lexical, resolve, needs)
	case SimpleValueRouteMissing, SimpleValueRouteInvalid:
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	return SimpleValue{}, ErrSimpleValueMetadata
}

func validateSimpleValueRouteReadFast(reads []simpleValueRouteRead, notations map[ExpandedName]bool, id SimpleTypeID, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, bool, error) {
	if id == NoSimpleType {
		return SimpleValue{Canonical: lexical, Type: NoSimpleType}, true, nil
	}
	read, ok := simpleValueRouteReadByID(reads, id)
	if !ok {
		return SimpleValue{}, true, ErrSimpleValueMetadata
	}
	switch SimpleValueRoute(SimpleValueRouteShape{Type: id, Variety: read.variety, Known: true}) {
	case SimpleValueRouteAtomic:
		return validateAtomicSimpleValueRouteReadFast(notations, id, read, lexical, resolve, needs)
	case SimpleValueRouteList, SimpleValueRouteUnion:
		return SimpleValue{}, false, nil
	case SimpleValueRouteUntyped:
		return SimpleValue{Canonical: lexical, Type: NoSimpleType}, true, nil
	case SimpleValueRouteMissing, SimpleValueRouteInvalid:
		return SimpleValue{}, true, ErrSimpleValueMetadata
	}
	return SimpleValue{}, true, ErrSimpleValueMetadata
}

func validateAtomicSimpleValueRouteReadFast(
	notations map[ExpandedName]bool,
	id SimpleTypeID,
	typ *simpleValueRouteRead,
	lexical string,
	resolve ResolveQNameParts,
	needs SimpleValueNeed,
) (SimpleValue, bool, error) {
	normalized := normalizeSimpleValueLexical(lexical, typ.whitespace)
	if value, handled, err := validatePublishedQNameRoute(notations, id, typ, normalized, resolve, needs); handled {
		return value, true, err
	}
	action := SimpleValueBypass(SimpleValueBypassShape{
		Facets:    typ.facets,
		Variety:   typ.variety,
		Primitive: typ.primitive,
		Builtin:   typ.builtin,
		Identity:  typ.identity,
		Fast:      typ.fast,
		Needs:     needs,
	})
	switch action {
	case SimpleValueBypassAcceptString:
		return unconstrainedStringSimpleValue(id, normalized, needs), true, nil
	case SimpleValueBypassValidateInt:
		if err := ValidateFastIntLexical(normalized); err != nil {
			return SimpleValue{}, true, err
		}
		return SimpleValue{Type: id}, true, nil
	case SimpleValueBypassValidateAnyURI:
		if err := ValidateAnyURILexical(normalized); err != nil {
			return SimpleValue{}, true, err
		}
		return SimpleValue{Type: id}, true, nil
	case SimpleValueBypassValidateHexBinary:
		if err := ValidateHexBinaryLexical(normalized); err != nil {
			return SimpleValue{}, true, err
		}
		return SimpleValue{Type: id}, true, nil
	case SimpleValueBypassValidateBase64Binary:
		if err := ValidateBase64BinaryLexical(normalized); err != nil {
			return SimpleValue{}, true, err
		}
		return SimpleValue{Type: id}, true, nil
	case SimpleValueBypassValidateFloat:
		if err := ValidateFloatLexical(normalized, simpleValueFloatBits(typ.primitive)); err != nil {
			return SimpleValue{}, true, err
		}
		return SimpleValue{Type: id}, true, nil
	case SimpleValueBypassValidateDuration:
		if err := ValidateDurationLexical(normalized); err != nil {
			return SimpleValue{}, true, err
		}
		return SimpleValue{Type: id}, true, nil
	case SimpleValueBypassValidateBoolean:
		if err := ValidateBooleanLexical(normalized); err != nil {
			return SimpleValue{}, true, err
		}
		return SimpleValue{Type: id}, true, nil
	case SimpleValueBypassValidateTemporal:
		if err := ValidateTemporalLexical(typ.primitive, normalized); err != nil {
			return SimpleValue{}, true, err
		}
		return SimpleValue{Type: id}, true, nil
	case SimpleValueBypassValidateDate:
		if err := validateDateLexical(normalized); err != nil {
			return SimpleValue{}, true, err
		}
		return SimpleValue{Type: id}, true, nil
	case SimpleValueBypassValidateDecimal:
		handled, err := ValidateFastDecimalLexical(RawDecimalFastPathShape{
			MinInclusive: typ.minInclusive,
			MaxInclusive: typ.maxInclusive,
			Facets:       typ.facets,
		}, normalized)
		if err != nil {
			return SimpleValue{}, true, err
		}
		if handled {
			return SimpleValue{Type: id}, true, nil
		}
		return SimpleValue{}, false, nil
	case SimpleValueBypassNone:
		full := typ.simpleValueType()
		if value, ok, err := validateAtomicStringSimpleValueFallback(id, full, normalized, needs); ok {
			return value, true, err
		}
		return SimpleValue{}, false, nil
	case SimpleValueBypassValidateStringPatterns, SimpleValueBypassValidateStringEnumeration:
		return SimpleValue{}, false, nil
	}
	return SimpleValue{}, true, ErrSimpleValueMetadata
}

func validatePublishedQNameRoute(notations map[ExpandedName]bool, id SimpleTypeID, typ *simpleValueRouteRead, normalized string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, bool, error) {
	if typ.facets != 0 || typ.builtin != BuiltinValidationNone ||
		(typ.primitive != PrimitiveQName && typ.primitive != PrimitiveNotation) {
		return SimpleValue{}, false, nil
	}
	primitiveNeeds := SimpleValuePrimitiveNeeds(PrimitiveValueNeedShape{Primitive: typ.primitive, Identity: typ.identity, Needs: needs})
	var canonical string
	var err error
	if typ.primitive == PrimitiveQName {
		canonical, err = validateQNamePrimitive(normalized, resolve, primitiveNeeds)
	} else {
		canonical, err = validateNotationPrimitive(publishedSimpleValueNotations(notations), normalized, resolve, primitiveNeeds)
	}
	if err != nil {
		return SimpleValue{}, true, err
	}
	return AtomicSimpleValue(AtomicSimpleValueProjection{Canonical: canonical, Type: id, Primitive: typ.primitive, Identity: typ.identity, Needs: needs}), true, nil
}

type publishedSimpleValueNotations map[ExpandedName]bool

func (n publishedSimpleValueNotations) simpleValueNotation(ns, local string) (bool, bool) {
	return n[ExpandedName{Namespace: ns, Local: local}], true
}

func validateAtomicSimpleValue[R simpleValueMetadataReader](reader R, id SimpleTypeID, typ SimpleValueType, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	normalized := normalizeSimpleValueLexical(lexical, typ.Whitespace)
	switch SimpleValueBypass(simpleValueAtomicBypassShape(&typ, needs)) {
	case SimpleValueBypassAcceptString:
		return unconstrainedStringSimpleValue(id, normalized, needs), nil
	case SimpleValueBypassValidateInt:
		if err := ValidateFastIntLexical(normalized); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassValidateStringPatterns, SimpleValueBypassValidateStringEnumeration:
		if err := validateSimpleValueStringFacets(reader, id, typ, normalized, normalized); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassValidateAnyURI:
		if err := ValidateAnyURILexical(normalized); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassValidateHexBinary:
		if err := ValidateHexBinaryLexical(normalized); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassValidateBase64Binary:
		if err := ValidateBase64BinaryLexical(normalized); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassValidateFloat:
		if err := ValidateFloatLexical(normalized, simpleValueFloatBits(typ.Primitive)); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassValidateDuration:
		if err := ValidateDurationLexical(normalized); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassValidateBoolean:
		if err := ValidateBooleanLexical(normalized); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassValidateTemporal:
		if err := ValidateTemporalLexical(typ.Primitive, normalized); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassValidateDate:
		if err := validateDateLexical(normalized); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	case SimpleValueBypassNone:
		if value, ok, err := validateAtomicStringSimpleValueFallback(id, typ, normalized, needs); ok {
			return value, err
		}
		return validateAtomicSimpleValueFallback(reader, id, typ, normalized, resolve, needs)
	case SimpleValueBypassValidateDecimal:
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
		dec, err := ParseDecimalValue(normalized)
		if err != nil {
			return SimpleValue{}, err
		}
		if err := ValidateDecimalFacets(typ.DecimalFacets, dec); err != nil {
			return SimpleValue{}, err
		}
		return SimpleValue{Type: id}, nil
	}
	return SimpleValue{}, ErrSimpleValueMetadata
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

func validateAtomicSimpleValueFallback[R simpleValueMetadataReader](reader R, id SimpleTypeID, typ SimpleValueType, normalized string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	if err := validateRuntimeAtomicBuiltin(typ, normalized); err != nil {
		return SimpleValue{}, err
	}
	if err := validateRuntimeAtomicLengthFacets(typ, normalized); err != nil {
		return SimpleValue{}, err
	}
	facets, ok := reader.simpleValueFacets(id)
	if !ok {
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	result, err := validateAtomicSimpleValueFallbackWithReader(reader, AtomicSimpleValueInput{
		Type:         typ,
		Facets:       facets,
		ResolveQName: resolve,
		Normalized:   normalized,
		Needs: SimpleValuePrimitiveNeeds(PrimitiveValueNeedShape{
			Facets:    typ.Facets,
			Primitive: typ.Primitive,
			Builtin:   typ.Builtin,
			Identity:  typ.Identity,
			Needs:     needs,
		}),
		Present: true,
	})
	if err != nil {
		return SimpleValue{}, err
	}
	return AtomicSimpleValue(AtomicSimpleValueProjection{
		Canonical:         result.Canonical,
		IdentityCanonical: result.IdentityCanonical,
		Type:              id,
		Primitive:         typ.Primitive,
		Identity:          typ.Identity,
		Needs:             needs,
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

func validateListSimpleValue[R simpleValueMetadataReader](reader R, id SimpleTypeID, typ SimpleValueType, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	needPlan := SimpleValueListNeeds(ListSimpleValueNeedShape{
		Facets:   typ.Facets,
		Identity: typ.Identity,
		Needs:    needs,
	})
	var canon strings.Builder
	var norm strings.Builder
	var refs strings.Builder
	var validateErr error
	count := uint32(0)
	forEachSimpleValueListItem(lexical, func(item string) bool {
		itemValue, err := validateSimpleValue(reader, typ.ListItem, item, resolve, needPlan.ItemNeeds)
		if err != nil {
			validateErr = err
			return false
		}
		if needPlan.NeedStrings {
			if count > 0 {
				canon.WriteByte(' ')
				norm.WriteByte(' ')
			}
			canon.WriteString(itemValue.Canonical)
			norm.WriteString(item)
		}
		AppendSimpleValueIDRefs(&refs, itemValue)
		count++
		return true
	})
	if validateErr != nil {
		return SimpleValue{}, validateErr
	}
	canonical := ""
	normalized := ""
	if needPlan.NeedStrings {
		canonical = canon.String()
		normalized = norm.String()
	}
	facetPlan := SimpleValueListFacetPlan(typ.Facets)
	if facetPlan.ValidateLength {
		if err := validateSimpleValueLengthFacets(typ, count); err != nil {
			return SimpleValue{}, err
		}
	}
	if facetPlan.ValidateLexical {
		if err := validateSimpleValueStringFacets(reader, id, typ, normalized, canonical); err != nil {
			return SimpleValue{}, err
		}
	}
	return ListSimpleValue(ListSimpleValueProjection{
		Canonical:  canonical,
		ItemIDRefs: refs.String(),
		Type:       id,
		Needs:      needs,
	}), nil
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

func validateUnionSimpleValue[R simpleValueMetadataReader](reader R, id SimpleTypeID, typ SimpleValueType, lexical string, resolve ResolveQNameParts, needs SimpleValueNeed) (SimpleValue, error) {
	if len(typ.UnionMembers) == 0 {
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	normalized := normalizeSimpleValueLexical(lexical, typ.Whitespace)
	memberNeeds := SimpleValueUnionMemberNeeds(UnionSimpleValueNeedShape{
		Facets:   typ.Facets,
		Identity: typ.Identity,
		Needs:    needs,
	})
	var matched bool
	var matchedValue SimpleValue
	var validateErr error
	var unsupportedErr error
	for _, member := range typ.UnionMembers {
		value, err := validateSimpleValue(reader, member, normalized, resolve, memberNeeds)
		if err == nil {
			if SimpleValueUnionFacetValidation(typ.Facets) {
				if facetErr := validateSimpleValueStringFacets(reader, id, typ, normalized, value.Canonical); facetErr != nil {
					validateErr = facetErr
					break
				}
			}
			matched = true
			matchedValue = value
			break
		}
		if unsupportedErr == nil && reader.simpleValueUnsupported(err) {
			unsupportedErr = err
		}
	}
	if validateErr != nil {
		return SimpleValue{}, validateErr
	}
	if matched {
		return matchedValue, nil
	}
	if unsupportedErr != nil {
		return SimpleValue{}, unsupportedErr
	}
	return SimpleValue{}, errors.New("value does not match any union member")
}

func validateSimpleValueStringFacets[R simpleValueMetadataReader](reader R, id SimpleTypeID, typ SimpleValueType, normalized, canonical string) error {
	if typ.Facets&FacetPattern != 0 && typ.StringFacets.patternCount() == 0 {
		return ErrSimpleValueMetadata
	}
	if typ.Facets&FacetEnumeration != 0 && !typ.StringFacets.HasEnumeration {
		return ErrSimpleValueMetadata
	}
	if err := typ.StringFacets.validatePatterns(normalized); err != nil {
		return err
	}
	if typ.StringFacets.HasEnumeration {
		matched, ok := reader.simpleValueStringEnumeration(id, canonical)
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
	default:
		return lex.CollapseXMLWhitespace(lexical)
	}
}

func forEachSimpleValueListItem(lexical string, yield func(string) bool) {
	start := -1
	for i := range len(lexical) {
		if lex.IsXMLWhitespaceByte(lexical[i]) {
			if start >= 0 {
				if !yield(lexical[start:i]) {
					return
				}
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		yield(lexical[start:])
	}
}
