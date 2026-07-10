package runtime

import (
	"errors"
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
	Type                     func(id SimpleTypeID) (SimpleValueType, bool)
	Facets                   func(id SimpleTypeID) (SimpleValueFacets, bool)
	ForEachStringEnumeration func(id SimpleTypeID, yield func(string) bool)
	StringEnumeration        func(id SimpleTypeID, canonical string) (bool, bool)
	ResolveQName             func(string) (ns, local string, ok bool)
	Notation                 func(ns, local string) bool
	Unsupported              func(error) bool
}

// LengthFacetValues is the runtime projection of length/minLength/maxLength
// facet values.
type LengthFacetValues struct {
	Length    FacetCardinalityValue
	MinLength FacetCardinalityValue
	MaxLength FacetCardinalityValue
}

// StringFacetValues is the runtime projection of string-based pattern and
// canonical enumeration facets.
type StringFacetValues struct {
	Patterns             []StringPatternGroup
	CanonicalEnumeration []string
	HasEnumeration       bool
}

// SimpleValueFacetLiteral is the runtime read projection of a compiled facet
// literal. It carries only the value facts needed by validation.
type SimpleValueFacetLiteral struct {
	Lexical   string
	Canonical string
	Actual    PrimitiveActualValue
	Present   bool
}

// SimpleValueFacets is the runtime-owned read projection of simple-type facets
// needed by schema atomic fallback validation.
type SimpleValueFacets struct {
	MinInclusive  SimpleValueFacetLiteral
	MaxInclusive  SimpleValueFacetLiteral
	MinExclusive  SimpleValueFacetLiteral
	MaxExclusive  SimpleValueFacetLiteral
	Enumeration   []SimpleValueFacetLiteral
	StringFacets  StringFacetValues
	DecimalFacets DecimalFacetValues
	LengthFacets  LengthFacetValues
	Facets        FacetMask
}

// SimpleValueTypeRead is the hot published simple-value projection for one
// simple type. Full facet literals are stored separately because most hot-path
// decisions only need these type facts.
type SimpleValueTypeRead struct {
	Type    SimpleValueType
	Present bool
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
	enumeration []SimpleValueFacetLiteral
	facets      FacetSet
}

type simpleValueColdReadTable struct {
	index  []uint32
	values []simpleValueColdRead
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
	for i := range table.index {
		table.index[i] = invalidID
	}
	for i := range types {
		if !simpleValueTypeNeedsColdRead(types[i]) {
			continue
		}
		if len(table.values) >= int(invalidID) {
			panic("too many simple value cold reads")
		}
		table.index[i] = uint32(len(table.values)) //nolint:gosec // guarded against the invalidID sentinel above.
		table.values = append(table.values, simpleValueColdRead{
			union:       types[i].Union,
			enumeration: simpleValueFacetLiterals(types[i].Facets.Enumeration),
			facets:      types[i].Facets,
		})
	}
	return table
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
	typ.StringFacets = StringFacetValues{Patterns: cold.facets.Patterns, HasEnumeration: len(cold.facets.Enumeration) != 0}
	typ.DecimalFacets = decimalFacetValues(cold.facets)
	typ.LengthFacets = lengthFacetValues(cold.facets)
	return typ
}

func simpleValueFacetsForColdRead(cold *simpleValueColdRead) SimpleValueFacets {
	if cold == nil {
		return SimpleValueFacets{}
	}
	f := cold.facets
	return SimpleValueFacets{
		MinInclusive:  simpleValueBoundFacetLiteral(f, FacetMinInclusive),
		MaxInclusive:  simpleValueBoundFacetLiteral(f, FacetMaxInclusive),
		MinExclusive:  simpleValueBoundFacetLiteral(f, FacetMinExclusive),
		MaxExclusive:  simpleValueBoundFacetLiteral(f, FacetMaxExclusive),
		Enumeration:   cold.enumeration,
		StringFacets:  StringFacetValues{Patterns: f.Patterns, HasEnumeration: len(f.Enumeration) != 0},
		DecimalFacets: decimalFacetValues(f),
		LengthFacets:  lengthFacetValues(f),
		Facets:        f.Present,
	}
}

func simpleValueRouteReadByID(reads []simpleValueRouteRead, id SimpleTypeID) (*simpleValueRouteRead, bool) {
	if !ValidSimpleTypeID(id, len(reads)) || !reads[id].present {
		return nil, false
	}
	return &reads[id], true
}

// SimpleValueFacetReadTable stores full facet projections only for simple
// types that have facet data.
type SimpleValueFacetReadTable struct {
	Index  []uint32
	Values []SimpleValueFacets
}

// NewSimpleValueCallbacksForTypeReads returns runtime callbacks backed by the
// published split simple-value projections.
func NewSimpleValueCallbacksForTypeReads(
	types []SimpleValueTypeRead,
	facets SimpleValueFacetReadTable,
	notation func(ns, local string) bool,
	resolve func(string) (ns, local string, ok bool),
	unsupported func(error) bool,
) SimpleValueCallbacks {
	return SimpleValueCallbacks{
		Type:                     simpleValueReadTypeFromTypeReads(types),
		Facets:                   simpleValueReadFacetsFromTable(types, facets),
		ForEachStringEnumeration: simpleValueReadStringEnumerationFromTable(types, facets),
		StringEnumeration:        simpleValueReadStringEnumerationContainsFromTable(types, facets),
		ResolveQName:             resolve,
		Notation:                 notation,
		Unsupported:              unsupported,
	}
}

// NewSimpleValueCallbacksForTypeReadsAndSimpleTypes returns runtime callbacks
// backed by hot type reads and cold facet/enumeration reads borrowed from
// immutable published simple types.
func NewSimpleValueCallbacksForTypeReadsAndSimpleTypes(
	typeReads []SimpleValueTypeRead,
	simpleTypes []SimpleType,
	notation func(ns, local string) bool,
	resolve func(string) (ns, local string, ok bool),
	unsupported func(error) bool,
) SimpleValueCallbacks {
	return SimpleValueCallbacks{
		Type:                     simpleValueReadTypeFromTypeReads(typeReads),
		Facets:                   simpleValueReadFacetsFromSimpleTypes(simpleTypes),
		ForEachStringEnumeration: simpleValueReadStringEnumerationFromSimpleTypes(simpleTypes),
		StringEnumeration:        simpleValueReadStringEnumerationContainsFromSimpleTypes(simpleTypes),
		ResolveQName:             resolve,
		Notation:                 notation,
		Unsupported:              unsupported,
	}
}

// NewSimpleValueCallbacksForSimpleTypes returns runtime callbacks backed by
// immutable published simple types.
func NewSimpleValueCallbacksForSimpleTypes(
	types []SimpleType,
	notation func(ns, local string) bool,
	resolve func(string) (ns, local string, ok bool),
	unsupported func(error) bool,
) SimpleValueCallbacks {
	return SimpleValueCallbacks{
		Type:                     simpleValueReadTypeFromSimpleTypes(types),
		Facets:                   simpleValueReadFacetsFromSimpleTypes(types),
		ForEachStringEnumeration: simpleValueReadStringEnumerationFromSimpleTypes(types),
		StringEnumeration:        simpleValueReadStringEnumerationContainsFromSimpleTypes(types),
		ResolveQName:             resolve,
		Notation:                 notation,
		Unsupported:              unsupported,
	}
}

// NewRawSimpleValueCallbacksForTypeReads returns raw simple-value callbacks
// backed by the published hot simple-value type projections.
func NewRawSimpleValueCallbacksForTypeReads(types []SimpleValueTypeRead) RawSimpleValueCallbacks {
	return RawSimpleValueCallbacks{
		Type:               rawSimpleValueReadTypeFromTypeReads(types),
		ForEachUnionMember: simpleValueReadUnionMembersFromTypeReads(types),
	}
}

// NewRawSimpleValueCallbacksForSimpleTypes returns raw simple-value callbacks
// backed by immutable published simple types.
func NewRawSimpleValueCallbacksForSimpleTypes(types []SimpleType) RawSimpleValueCallbacks {
	return RawSimpleValueCallbacks{
		Type:                     rawSimpleValueReadTypeFromSimpleTypes(types),
		ForEachUnionMember:       simpleValueReadUnionMembersFromSimpleTypes(types),
		ForEachStringEnumeration: simpleValueReadStringEnumerationFromSimpleTypes(types),
	}
}

func simpleValueReadTypeFromTypeReads(types []SimpleValueTypeRead) func(SimpleTypeID) (SimpleValueType, bool) {
	return func(id SimpleTypeID) (SimpleValueType, bool) {
		read, ok := simpleValueTypeReadByID(types, id)
		if !ok {
			return SimpleValueType{}, false
		}
		return read.Type, true
	}
}

func simpleValueReadTypeFromSimpleTypes(types []SimpleType) func(SimpleTypeID) (SimpleValueType, bool) {
	return func(id SimpleTypeID) (SimpleValueType, bool) {
		st, ok := UsableSimpleType(types, id)
		if !ok {
			return SimpleValueType{}, false
		}
		return SimpleValueTypeForSimpleType(*st), true
	}
}

func simpleValueReadFacetsFromTable(types []SimpleValueTypeRead, facets SimpleValueFacetReadTable) func(SimpleTypeID) (SimpleValueFacets, bool) {
	return func(id SimpleTypeID) (SimpleValueFacets, bool) {
		if _, ok := simpleValueTypeReadByID(types, id); !ok {
			return SimpleValueFacets{}, false
		}
		return facets.Facets(id)
	}
}

func simpleValueReadFacetsFromSimpleTypes(types []SimpleType) func(SimpleTypeID) (SimpleValueFacets, bool) {
	return func(id SimpleTypeID) (SimpleValueFacets, bool) {
		st, ok := UsableSimpleType(types, id)
		if !ok {
			return SimpleValueFacets{}, false
		}
		return SimpleValueFacetsForFacetSet(st.Facets), true
	}
}

func rawSimpleValueReadTypeFromTypeReads(types []SimpleValueTypeRead) func(SimpleTypeID) (RawSimpleValueType, bool) {
	return func(id SimpleTypeID) (RawSimpleValueType, bool) {
		read, ok := simpleValueTypeReadByID(types, id)
		if !ok {
			return RawSimpleValueType{}, false
		}
		return rawSimpleValueTypeForSimpleValueType(read.Type), true
	}
}

func rawSimpleValueReadTypeFromSimpleTypes(types []SimpleType) func(SimpleTypeID) (RawSimpleValueType, bool) {
	return func(id SimpleTypeID) (RawSimpleValueType, bool) {
		st, ok := UsableSimpleType(types, id)
		if !ok {
			return RawSimpleValueType{}, false
		}
		return rawSimpleValueTypeForPublishedSimpleType(*st), true
	}
}

func rawSimpleValueTypeForSimpleValueType(typ SimpleValueType) RawSimpleValueType {
	return RawSimpleValueType{
		DecimalMinInclusive: typ.DecimalMinInclusive,
		DecimalMaxInclusive: typ.DecimalMaxInclusive,
		StringPatterns:      typ.StringFacets.Patterns,
		ListItem:            typ.ListItem,
		Facets:              typ.Facets,
		Variety:             typ.Variety,
		Primitive:           typ.Primitive,
		Builtin:             typ.Builtin,
		Whitespace:          typ.Whitespace,
		Identity:            typ.Identity,
		Fast:                typ.Fast,
	}
}

func rawSimpleValueTypeForPublishedSimpleType(st SimpleType) RawSimpleValueType {
	return RawSimpleValueType{
		DecimalMinInclusive: rawDecimalBoundFacet(st.Facets, FacetMinInclusive),
		DecimalMaxInclusive: rawDecimalBoundFacet(st.Facets, FacetMaxInclusive),
		StringPatterns:      st.Facets.Patterns,
		ListItem:            st.ListItem,
		Facets:              st.Facets.Present,
		Variety:             st.Variety,
		Primitive:           st.Primitive,
		Builtin:             st.Builtin,
		Whitespace:          st.Whitespace,
		Identity:            st.Identity,
		Fast:                st.Fast,
	}
}

func simpleValueReadUnionMembersFromTypeReads(types []SimpleValueTypeRead) func(SimpleTypeID, func(SimpleTypeID) bool) {
	return func(id SimpleTypeID, yield func(SimpleTypeID) bool) {
		read, ok := simpleValueTypeReadByID(types, id)
		if !ok {
			return
		}
		for _, member := range read.Type.UnionMembers {
			if !yield(member) {
				return
			}
		}
	}
}

func simpleValueReadUnionMembersFromSimpleTypes(types []SimpleType) func(SimpleTypeID, func(SimpleTypeID) bool) {
	return func(id SimpleTypeID, yield func(SimpleTypeID) bool) {
		st, ok := UsableSimpleType(types, id)
		if !ok {
			return
		}
		for _, member := range st.Union {
			if !yield(member) {
				return
			}
		}
	}
}

func simpleValueReadStringEnumerationFromTable(types []SimpleValueTypeRead, facets SimpleValueFacetReadTable) func(SimpleTypeID, func(string) bool) {
	return func(id SimpleTypeID, yield func(string) bool) {
		if _, ok := simpleValueTypeReadByID(types, id); !ok {
			return
		}
		f, ok := facets.Facets(id)
		if !ok {
			return
		}
		for _, lit := range f.Enumeration {
			if !yield(lit.Canonical) {
				return
			}
		}
	}
}

func simpleValueReadStringEnumerationFromSimpleTypes(types []SimpleType) func(SimpleTypeID, func(string) bool) {
	return func(id SimpleTypeID, yield func(string) bool) {
		st, ok := UsableSimpleType(types, id)
		if !ok {
			return
		}
		for _, lit := range st.Facets.Enumeration {
			if !yield(lit.Canonical) {
				return
			}
		}
	}
}

func simpleValueReadStringEnumerationContainsFromTable(types []SimpleValueTypeRead, facets SimpleValueFacetReadTable) func(SimpleTypeID, string) (bool, bool) {
	return func(id SimpleTypeID, canonical string) (bool, bool) {
		if _, ok := simpleValueTypeReadByID(types, id); !ok {
			return false, false
		}
		f, ok := facets.Facets(id)
		if !ok {
			return false, false
		}
		for _, lit := range f.Enumeration {
			if lit.Canonical == canonical {
				return true, true
			}
		}
		return false, true
	}
}

func simpleValueReadStringEnumerationContainsFromSimpleTypes(types []SimpleType) func(SimpleTypeID, string) (bool, bool) {
	return func(id SimpleTypeID, canonical string) (bool, bool) {
		st, ok := UsableSimpleType(types, id)
		if !ok {
			return false, false
		}
		for _, lit := range st.Facets.Enumeration {
			if lit.Canonical == canonical {
				return true, true
			}
		}
		return false, true
	}
}

func simpleValueTypeReadByID(types []SimpleValueTypeRead, id SimpleTypeID) (*SimpleValueTypeRead, bool) {
	if !ValidSimpleTypeID(id, len(types)) {
		return nil, false
	}
	read := &types[id]
	return read, read.Present
}

// Facets returns the full facet projection for id. Missing table entries mean
// the simple type has no full facet projection to apply.
func (t SimpleValueFacetReadTable) Facets(id SimpleTypeID) (SimpleValueFacets, bool) {
	if !ValidSimpleTypeID(id, len(t.Index)) {
		return SimpleValueFacets{}, false
	}
	idx := t.Index[id]
	if idx == invalidID {
		return SimpleValueFacets{}, true
	}
	if !ValidUint32Index(idx, len(t.Values)) {
		return SimpleValueFacets{}, false
	}
	return t.Values[idx], true
}

// NewSimpleValueQNameResolverNeedsForTypeReads precomputes whether each
// published simple type can require lexical QName namespace resolution.
func NewSimpleValueQNameResolverNeedsForTypeReads(types []SimpleValueTypeRead) []bool {
	out := make([]bool, len(types))
	state := make([]uint8, len(types))
	for i := range types {
		out[i] = simpleValueTypeReadNeedsQNameResolverCached(types, SimpleTypeID(i), out, state)
	}
	return out
}

func simpleValueTypeReadNeedsQNameResolverCached(types []SimpleValueTypeRead, id SimpleTypeID, out []bool, state []uint8) bool {
	read, ok := simpleValueTypeReadByID(types, id)
	if !ok {
		return false
	}
	idx := int(id)
	switch state[idx] {
	case 2:
		return out[idx]
	case 1:
		return false
	}
	state[idx] = 1
	typ := read.Type
	var needs bool
	switch typ.Variety {
	case SimpleVarietyAtomic:
		needs = typ.Primitive == PrimitiveQName || typ.Primitive == PrimitiveNotation
	case SimpleVarietyList:
		needs = simpleValueTypeReadNeedsQNameResolverCached(types, typ.ListItem, out, state)
	case SimpleVarietyUnion:
		for _, member := range typ.UnionMembers {
			if simpleValueTypeReadNeedsQNameResolverCached(types, member, out, state) {
				needs = true
				break
			}
		}
	}
	out[idx] = needs
	state[idx] = 2
	return needs
}

// NewSimpleValueQNameResolverNeedsForSimpleTypes precomputes whether each
// published simple type can require lexical QName namespace resolution.
func NewSimpleValueQNameResolverNeedsForSimpleTypes(types []SimpleType) []bool {
	out := make([]bool, len(types))
	state := make([]uint8, len(types))
	for i := range types {
		out[i] = simpleTypeNeedsQNameResolverCached(types, SimpleTypeID(i), out, state)
	}
	return out
}

func simpleTypeNeedsQNameResolverCached(types []SimpleType, id SimpleTypeID, out []bool, state []uint8) bool {
	if !ValidSimpleTypeID(id, len(types)) {
		return false
	}
	idx := int(id)
	switch state[idx] {
	case 2:
		return out[idx]
	case 1:
		return false
	}
	typ := types[idx]
	if typ.Missing {
		state[idx] = 2
		return false
	}
	state[idx] = 1
	var needs bool
	switch typ.Variety {
	case SimpleVarietyAtomic:
		needs = typ.Primitive == PrimitiveQName || typ.Primitive == PrimitiveNotation
	case SimpleVarietyList:
		needs = simpleTypeNeedsQNameResolverCached(types, typ.ListItem, out, state)
	case SimpleVarietyUnion:
		for _, member := range typ.Union {
			if simpleTypeNeedsQNameResolverCached(types, member, out, state) {
				needs = true
				break
			}
		}
	}
	out[idx] = needs
	state[idx] = 2
	return needs
}

// SimpleTypeNeedsQNameResolver reports whether validating id can require
// lexical QName namespace resolution from immutable simple-type records.
func SimpleTypeNeedsQNameResolver(types []SimpleType, id SimpleTypeID) bool {
	return simpleTypeNeedsQNameResolver(types, id, 0)
}

func simpleTypeNeedsQNameResolver(types []SimpleType, id SimpleTypeID, depth int) bool {
	if !ValidSimpleTypeID(id, len(types)) || depth > len(types) {
		return false
	}
	typ := types[id]
	if typ.Missing {
		return false
	}
	switch typ.Variety {
	case SimpleVarietyAtomic:
		return typ.Primitive == PrimitiveQName || typ.Primitive == PrimitiveNotation
	case SimpleVarietyList:
		return simpleTypeNeedsQNameResolver(types, typ.ListItem, depth+1)
	case SimpleVarietyUnion:
		for _, member := range typ.Union {
			if simpleTypeNeedsQNameResolver(types, member, depth+1) {
				return true
			}
		}
	}
	return false
}

// NewSimpleValueTypeReadsForSimpleTypes returns immutable hot simple-value
// projections for frozen simple types.
func NewSimpleValueTypeReadsForSimpleTypes(simpleTypes []SimpleType) []SimpleValueTypeRead {
	out := make([]SimpleValueTypeRead, len(simpleTypes))
	for i, st := range simpleTypes {
		out[i] = NewSimpleValueTypeReadForSimpleType(st)
	}
	return out
}

// NewSimpleValueTypeReadForSimpleType returns the immutable hot simple-value
// type projection for one frozen simple type.
func NewSimpleValueTypeReadForSimpleType(st SimpleType) SimpleValueTypeRead {
	if st.Missing {
		return SimpleValueTypeRead{}
	}
	return SimpleValueTypeRead{
		Type:    simpleValueTypeForSimpleType(st, st.Facets),
		Present: true,
	}
}

// NewSimpleValueFacetReadTableForSimpleTypes returns compact full-facet
// projections for frozen simple types.
func NewSimpleValueFacetReadTableForSimpleTypes(simpleTypes []SimpleType) SimpleValueFacetReadTable {
	count := 0
	for _, st := range simpleTypes {
		if !st.Missing && st.Facets.Present != 0 {
			count++
		}
	}
	table := SimpleValueFacetReadTable{
		Index:  make([]uint32, len(simpleTypes)),
		Values: make([]SimpleValueFacets, 0, count),
	}
	for i := range table.Index {
		table.Index[i] = invalidID
	}
	for i, st := range simpleTypes {
		if st.Missing || st.Facets.Present == 0 {
			continue
		}
		if len(table.Values) >= int(invalidID) {
			panic("too many simple value facet reads")
		}
		table.Index[i] = uint32(len(table.Values)) //nolint:gosec // guarded against the invalidID sentinel above.
		table.Values = append(table.Values, SimpleValueFacetsForFacetSet(st.Facets))
	}
	return table
}

// NewSimpleValuePayloadTypeForType projects a published simple-value type into
// the shape needed to validate cached simple-value identity payloads.
func NewSimpleValuePayloadTypeForType(typ SimpleValueType, ok bool) (SimpleValuePayloadType, bool) {
	if !ok {
		return SimpleValuePayloadType{}, false
	}
	return SimpleValuePayloadType{
		Primitive: typ.Primitive,
		Variety:   typ.Variety,
		Identity:  typ.Identity,
	}, true
}

func simpleValueTypeForSimpleType(st SimpleType, facets FacetSet) SimpleValueType {
	typ := SimpleValueType{
		DecimalMinInclusive: rawDecimalBoundFacet(facets, FacetMinInclusive),
		DecimalMaxInclusive: rawDecimalBoundFacet(facets, FacetMaxInclusive),
		DecimalFacets:       decimalFacetValues(facets),
		LengthFacets:        lengthFacetValues(facets),
		StringFacets:        stringFacetValues(facets),
		UnionMembers:        slices.Clone(st.Union),
		ListItem:            st.ListItem,
		Facets:              facets.Present,
		Variety:             st.Variety,
		Primitive:           st.Primitive,
		Builtin:             st.Builtin,
		Whitespace:          st.Whitespace,
		Identity:            st.Identity,
		Fast:                st.Fast,
	}
	typ.RawBypass = SimpleValueBypass(simpleValueAtomicBypassShape(&typ, 0))
	return typ
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
			Patterns:       st.Facets.Patterns,
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

// SimpleValueFacetsForFacetSet returns validation facet facts for a facet set.
func SimpleValueFacetsForFacetSet(f FacetSet) SimpleValueFacets {
	return SimpleValueFacets{
		MinInclusive: simpleValueBoundFacetLiteral(f, FacetMinInclusive),
		MaxInclusive: simpleValueBoundFacetLiteral(f, FacetMaxInclusive),
		MinExclusive: simpleValueBoundFacetLiteral(f, FacetMinExclusive),
		MaxExclusive: simpleValueBoundFacetLiteral(f, FacetMaxExclusive),
		Enumeration:  simpleValueFacetLiterals(f.Enumeration),
		StringFacets: StringFacetValues{
			Patterns:             CloneStringPatternGroups(f.Patterns),
			CanonicalEnumeration: canonicalEnumerationValues(f.Enumeration),
			HasEnumeration:       len(f.Enumeration) != 0,
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
		Lexical:   lit.Lexical,
		Canonical: lit.Canonical,
		Actual:    lit.Actual,
		Present:   true,
	}
}

func simpleValueFacetLiterals(in []CompiledLiteral) []SimpleValueFacetLiteral {
	out := make([]SimpleValueFacetLiteral, len(in))
	for i := range in {
		out[i] = simpleValueFacetLiteral(in[i], true)
	}
	return slices.Clip(out)
}

func canonicalEnumerationValues(in []CompiledLiteral) []string {
	out := make([]string, len(in))
	for i := range in {
		out[i] = in[i].Canonical
	}
	return slices.Clip(out)
}

func stringEnumerationContains(enumeration []string, canonical string) bool {
	return slices.Contains(enumeration, canonical)
}

func lengthFacetValues(f FacetSet) LengthFacetValues {
	return LengthFacetValues{
		Length:    facetCardinalityValue(f.Length, f.Present&FacetLength != 0),
		MinLength: facetCardinalityValue(f.MinLength, f.Present&FacetMinLength != 0),
		MaxLength: facetCardinalityValue(f.MaxLength, f.Present&FacetMaxLength != 0),
	}
}

func stringFacetValues(f FacetSet) StringFacetValues {
	return StringFacetValues{
		Patterns:             f.Patterns,
		CanonicalEnumeration: canonicalEnumerationValues(f.Enumeration),
		HasEnumeration:       len(f.Enumeration) != 0,
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
	return decimalFacetValue(lit, present)
}

func decimalFacetValue(lit CompiledLiteral, present bool) DecimalFacetValue {
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
	typ, known := simpleValueType(cb, id)
	switch SimpleValueRoute(SimpleValueRouteShape{Type: id, Variety: typ.Variety, Known: known}) {
	case SimpleValueRouteUntyped:
		return SimpleValue{Canonical: lexical, Type: NoSimpleType}, nil
	case SimpleValueRouteAtomic:
		return validateAtomicSimpleValue(cb, id, typ, lexical, needs)
	case SimpleValueRouteList:
		return validateListSimpleValue(cb, id, typ, lexical, needs)
	case SimpleValueRouteUnion:
		return validateUnionSimpleValue(cb, id, typ, lexical, needs)
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
		canonical, err = validatePublishedNotationPrimitive(normalized, notations, resolve, primitiveNeeds)
	}
	if err != nil {
		return SimpleValue{}, true, err
	}
	return AtomicSimpleValue(AtomicSimpleValueProjection{Canonical: canonical, Type: id, Primitive: typ.primitive, Identity: typ.identity, Needs: needs}), true, nil
}

func validatePublishedNotationPrimitive(normalized string, notations map[ExpandedName]bool, resolve ResolveQNameParts, needs PrimitiveValueNeed) (string, error) {
	if resolve == nil {
		if !lex.IsNCName(normalized) {
			return "", errors.New("invalid NOTATION")
		}
		if !notations[ExpandedName{Local: normalized}] {
			return "", errors.New("undeclared notation")
		}
		if !needs.Has(PrimitiveNeedCanonical) {
			return "", nil
		}
		return normalized, nil
	}
	ns, local, ok := resolve(normalized)
	if !ok {
		return "", errors.New("unresolved NOTATION")
	}
	if !notations[ExpandedName{Namespace: ns, Local: local}] {
		return "", errors.New("undeclared notation")
	}
	if !needs.Has(PrimitiveNeedCanonical) {
		return "", nil
	}
	return FormatExpandedName(ns, local), nil
}

func simpleValueType(cb SimpleValueCallbacks, id SimpleTypeID) (SimpleValueType, bool) {
	if id == NoSimpleType {
		return SimpleValueType{}, false
	}
	return cb.Type(id)
}

func simpleValueFacets(cb SimpleValueCallbacks, id SimpleTypeID) (SimpleValueFacets, bool) {
	if cb.Facets == nil {
		return SimpleValueFacets{}, false
	}
	return cb.Facets(id)
}

func validateAtomicSimpleValue(cb SimpleValueCallbacks, id SimpleTypeID, typ SimpleValueType, lexical string, needs SimpleValueNeed) (SimpleValue, error) {
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
		if err := validateSimpleValueStringFacets(cb, id, typ, normalized, normalized); err != nil {
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
		return validateAtomicSimpleValueFallback(cb, id, typ, normalized, needs)
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

func validateAtomicSimpleValueFallback(cb SimpleValueCallbacks, id SimpleTypeID, typ SimpleValueType, normalized string, needs SimpleValueNeed) (SimpleValue, error) {
	if err := validateRuntimeAtomicBuiltin(typ, normalized); err != nil {
		return SimpleValue{}, err
	}
	if err := validateRuntimeAtomicLengthFacets(typ, normalized); err != nil {
		return SimpleValue{}, err
	}
	facets, ok := simpleValueFacets(cb, id)
	if !ok {
		return SimpleValue{}, ErrSimpleValueMetadata
	}
	result, err := ValidateAtomicSimpleValueFallback(AtomicSimpleValueInput{
		Type:         typ,
		Facets:       facets,
		ResolveQName: cb.ResolveQName,
		Notation:     cb.Notation,
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

func validateListSimpleValue(cb SimpleValueCallbacks, id SimpleTypeID, typ SimpleValueType, lexical string, needs SimpleValueNeed) (SimpleValue, error) {
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
		itemValue, err := ValidateSimpleValue(cb, typ.ListItem, item, needPlan.ItemNeeds)
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
		if err := validateSimpleValueStringFacets(cb, id, typ, normalized, canonical); err != nil {
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

func validateUnionSimpleValue(cb SimpleValueCallbacks, id SimpleTypeID, typ SimpleValueType, lexical string, needs SimpleValueNeed) (SimpleValue, error) {
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
		value, err := ValidateSimpleValue(cb, member, normalized, memberNeeds)
		if err == nil {
			if SimpleValueUnionFacetValidation(typ.Facets) {
				if facetErr := validateSimpleValueStringFacets(cb, id, typ, normalized, value.Canonical); facetErr != nil {
					validateErr = facetErr
					break
				}
			}
			matched = true
			matchedValue = value
			break
		}
		if unsupportedErr == nil && cb.Unsupported(err) {
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

func validateSimpleValueStringFacets(cb SimpleValueCallbacks, id SimpleTypeID, typ SimpleValueType, normalized, canonical string) error {
	if typ.Facets&FacetPattern != 0 && len(typ.StringFacets.Patterns) == 0 {
		return ErrSimpleValueMetadata
	}
	if typ.Facets&FacetEnumeration != 0 && !typ.StringFacets.HasEnumeration {
		return ErrSimpleValueMetadata
	}
	if err := ValidateStringPatterns(typ.StringFacets.Patterns, normalized); err != nil {
		return err
	}
	if typ.StringFacets.HasEnumeration {
		if len(typ.StringFacets.CanonicalEnumeration) != 0 {
			if stringEnumerationContains(typ.StringFacets.CanonicalEnumeration, canonical) {
				return nil
			}
			return errors.New("enumeration facet failed")
		}
		if cb.StringEnumeration != nil {
			matched, ok := cb.StringEnumeration(id, canonical)
			if !ok {
				return ErrSimpleValueMetadata
			}
			if matched {
				return nil
			}
			return errors.New("enumeration facet failed")
		}
		return ValidateStringEnumeration(id, cb.ForEachStringEnumeration, canonical)
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
