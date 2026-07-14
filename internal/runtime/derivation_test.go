package runtime

import (
	"slices"
	"strings"
	"testing"
)

type derivationRuntimeStub struct {
	simple  []SimpleTypeDerivation
	complex []ComplexTypeDerivation
	anyType ComplexTypeID
}

func (s derivationRuntimeStub) AnyTypeID() ComplexTypeID {
	return s.anyType
}

func (s derivationRuntimeStub) ComplexTypeCount() int {
	return len(s.complex)
}

func (s derivationRuntimeStub) SimpleTypeCount() int {
	return len(s.simple)
}

func (s derivationRuntimeStub) SimpleTypeDerivation(id SimpleTypeID) (SimpleTypeDerivation, bool) {
	if !ValidUint32Index(uint32(id), len(s.simple)) {
		return SimpleTypeDerivation{}, false
	}
	return s.simple[id], true
}

func (s derivationRuntimeStub) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	if !ValidUint32Index(uint32(id), len(s.complex)) {
		return ComplexTypeDerivation{}, false
	}
	return s.complex[id], true
}

func newTypeDerivationReadForTest(
	t *testing.T,
	simpleTypes []SimpleType,
	complexTypes []ComplexType,
) TypeDerivationRead {
	t.Helper()
	if len(complexTypes) == 0 {
		complexTypes = []ComplexType{{Derivation: DerivationKindNone}}
	}
	cold := newSimpleTypeColdReadTable(simpleTypes)
	read, err := newTypeDerivationReadForTypes(0, simpleTypes, complexTypes, cold)
	if err != nil {
		t.Fatalf("newTypeDerivationReadForTypes() error = %v", err)
	}
	return read
}

func newDerivationRuntimeStub(simpleTypes []SimpleType, complexTypes []ComplexType) derivationRuntimeStub {
	stub := derivationRuntimeStub{
		simple:  make([]SimpleTypeDerivation, len(simpleTypes)),
		complex: make([]ComplexTypeDerivation, len(complexTypes)),
	}
	for i := range simpleTypes {
		stub.simple[i] = NewSimpleTypeDerivationForSimpleType(simpleTypes[i])
	}
	for i := range complexTypes {
		stub.complex[i] = NewComplexTypeDerivationForComplexType(complexTypes[i])
	}
	return stub
}

func TestEqualComplexTypeDerivations(t *testing.T) {
	t.Parallel()

	base := ComplexTypeDerivation{
		Base:  ComplexRef(1),
		Kind:  DerivationKindExtension,
		Block: DerivationRestriction,
	}
	tests := []struct {
		name string
		a    ComplexTypeDerivation
		b    ComplexTypeDerivation
		want bool
	}{
		{
			name: "equal",
			a:    base,
			b: ComplexTypeDerivation{
				Base:  ComplexRef(1),
				Kind:  DerivationKindExtension,
				Block: DerivationRestriction,
			},
			want: true,
		},
		{
			name: "base differs",
			a:    base,
			b: ComplexTypeDerivation{
				Base:  ComplexRef(2),
				Kind:  DerivationKindExtension,
				Block: DerivationRestriction,
			},
		},
		{
			name: "kind differs",
			a:    base,
			b: ComplexTypeDerivation{
				Base:  ComplexRef(1),
				Kind:  DerivationKindRestriction,
				Block: DerivationRestriction,
			},
		},
		{
			name: "block differs",
			a:    base,
			b: ComplexTypeDerivation{
				Base: ComplexRef(1),
				Kind: DerivationKindExtension,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := EqualComplexTypeDerivations(tt.a, tt.b); got != tt.want {
				t.Fatalf("EqualComplexTypeDerivations() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComplexTypeDerivationForComplexType(t *testing.T) {
	t.Parallel()

	ct := ComplexType{
		Base:       ComplexRef(1),
		Derivation: DerivationKindExtension,
		Block:      DerivationRestriction,
	}
	projection := NewComplexTypeDerivationForComplexType(ct)
	if projection != (ComplexTypeDerivation{
		Base:  ComplexRef(1),
		Kind:  DerivationKindExtension,
		Block: DerivationRestriction,
	}) {
		t.Fatalf("NewComplexTypeDerivationForComplexType() = %+v, want projected complex type facts", projection)
	}
	if !EqualComplexTypeDerivationForComplexType(projection, ct) {
		t.Fatal("EqualComplexTypeDerivationForComplexType() = false, want true")
	}
	if EqualComplexTypeDerivationForComplexType(projection, ComplexType{
		Base:       ComplexRef(2),
		Derivation: DerivationKindExtension,
		Block:      DerivationRestriction,
	}) {
		t.Fatal("EqualComplexTypeDerivationForComplexType() accepted wrong base")
	}
	if EqualComplexTypeDerivationForComplexType(projection, ComplexType{
		Base:       ComplexRef(1),
		Derivation: DerivationKindRestriction,
		Block:      DerivationRestriction,
	}) {
		t.Fatal("EqualComplexTypeDerivationForComplexType() accepted wrong derivation kind")
	}
	if EqualComplexTypeDerivationForComplexType(projection, ComplexType{
		Base:       ComplexRef(1),
		Derivation: DerivationKindExtension,
	}) {
		t.Fatal("EqualComplexTypeDerivationForComplexType() accepted wrong block")
	}
}

func TestSimpleTypeDerivationForSimpleType(t *testing.T) {
	t.Parallel()

	st := SimpleType{
		Union:   []SimpleTypeID{1, 2},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}
	projection := NewSimpleTypeDerivationForSimpleType(st)
	if projection.Base != NoSimpleType ||
		projection.Variety != SimpleVarietyUnion ||
		!slices.Equal(projection.Union, []SimpleTypeID{1, 2}) {
		t.Fatalf("NewSimpleTypeDerivationForSimpleType() = %+v, want projected simple type facts", projection)
	}
	st.Union[0] = 9
	if projection.Union[0] != 1 {
		t.Fatalf("NewSimpleTypeDerivationForSimpleType() aliased union slice: %+v", projection)
	}
	if !EqualSimpleTypeDerivationForSimpleType(projection, SimpleType{
		Union:   []SimpleTypeID{1, 2},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}) {
		t.Fatal("EqualSimpleTypeDerivationForSimpleType() = false, want true")
	}
	if EqualSimpleTypeDerivationForSimpleType(projection, SimpleType{
		Union:   []SimpleTypeID{1, 2},
		Base:    3,
		Variety: SimpleVarietyUnion,
	}) {
		t.Fatal("EqualSimpleTypeDerivationForSimpleType() accepted wrong base")
	}
}

func TestTypeDerivationRead(t *testing.T) {
	t.Parallel()

	simpleTypes := []SimpleType{{Base: NoSimpleType, Variety: SimpleVarietyAtomic}}
	complexTypes := []ComplexType{{
		Derivation: DerivationKindNone,
		Block:      DerivationRestriction,
	}}
	read := newTypeDerivationReadForTest(t, simpleTypes, complexTypes)

	if read.AnyTypeID() != 0 {
		t.Fatalf("AnyTypeID() = %d, want 0", read.AnyTypeID())
	}
	if read.SimpleTypeCount() != 1 || read.ComplexTypeCount() != 1 {
		t.Fatalf("counts = %d, %d; want 1, 1", read.SimpleTypeCount(), read.ComplexTypeCount())
	}
	if !validTypeID(SimpleRef(0), read.SimpleTypeCount(), read.ComplexTypeCount()) ||
		!validTypeID(ComplexRef(0), read.SimpleTypeCount(), read.ComplexTypeCount()) ||
		validTypeID(SimpleRef(1), read.SimpleTypeCount(), read.ComplexTypeCount()) ||
		validTypeID(ComplexRef(1), read.SimpleTypeCount(), read.ComplexTypeCount()) ||
		validTypeID(TypeID{}, read.SimpleTypeCount(), read.ComplexTypeCount()) {
		t.Fatal("type ID validation did not match published graph bounds")
	}
	if read.simpleTypeTable() == nil {
		t.Fatal("type derivation read is not bound to the published simple-type table")
	}

	changedSimpleTypes := []SimpleType{{
		Union:   []SimpleTypeID{0},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}}
	changedComplexTypes := []ComplexType{{
		Derivation: DerivationKindRestriction,
		Block:      DerivationRestriction,
	}}

	if err := ValidateTypeDerivationReadProjection(read, 0, simpleTypes, complexTypes); err != nil {
		t.Fatalf("ValidateTypeDerivationReadProjection() error = %v", err)
	}
	if err := ValidateTypeDerivationReadProjection(read, 1, simpleTypes, complexTypes); err == nil || err.Error() != "type derivation projection stores invalid anyType" {
		t.Fatalf("ValidateTypeDerivationReadProjection(anyType) error = %v, want anyType invariant", err)
	}
	wrongSimpleCount := read
	wrongSimpleIndex := *read.index
	wrongSimpleIndex.simpleIn = nil
	wrongSimpleCount.index = &wrongSimpleIndex
	if err := ValidateTypeDerivationReadProjection(wrongSimpleCount, 0, simpleTypes, complexTypes); err == nil || err.Error() != "simple type derivation projection count does not match types" {
		t.Fatalf("ValidateTypeDerivationReadProjection(simple count) error = %v, want simple count invariant", err)
	}
	wrongComplexCount := read
	wrongComplexIndex := *read.index
	wrongComplexIndex.complexIn = nil
	wrongComplexCount.index = &wrongComplexIndex
	if err := ValidateTypeDerivationReadProjection(wrongComplexCount, 0, simpleTypes, complexTypes); err == nil || err.Error() != "complex type derivation projection count does not match types" {
		t.Fatalf("ValidateTypeDerivationReadProjection(complex count) error = %v, want complex count invariant", err)
	}
	if err := ValidateTypeDerivationReadProjection(read, 0, changedSimpleTypes, complexTypes); err == nil || err.Error() != "type derivation union reads do not match types" {
		t.Fatalf("ValidateTypeDerivationReadProjection(simple mismatch) error = %v, want simple mismatch invariant", err)
	}
	if err := ValidateTypeDerivationReadProjection(read, 0, simpleTypes, changedComplexTypes); err == nil || err.Error() != "type derivation index does not match type graph" {
		t.Fatalf("ValidateTypeDerivationReadProjection(complex mismatch) error = %v, want complex mismatch invariant", err)
	}
}

func TestTypeDerivationMaskSimpleTypeRestrictionAndUnionBase(t *testing.T) {
	t.Parallel()

	rt := derivationRuntimeStub{
		simple: []SimpleTypeDerivation{
			{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
			{Base: 0, Variety: SimpleVarietyAtomic},
			{Base: NoSimpleType, Variety: SimpleVarietyUnion, Union: []SimpleTypeID{0}},
		},
		complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}},
	}
	mask, ok := TypeDerivationMask(rt, SimpleRef(1), SimpleRef(0))
	if !ok || mask != DerivationRestriction {
		t.Fatalf("simple restriction mask = %08b, %v; want restriction, true", mask, ok)
	}
	mask, ok = TypeDerivationMask(rt, SimpleRef(1), SimpleRef(2))
	if !ok || mask != DerivationRestriction {
		t.Fatalf("simple union-base mask = %08b, %v; want restriction, true", mask, ok)
	}
}

func TestTypeDerivationMaskComplexTypeChain(t *testing.T) {
	t.Parallel()

	rt := derivationRuntimeStub{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(0), Kind: DerivationKindExtension},
			{Base: ComplexRef(1), Kind: DerivationKindRestriction},
		},
	}
	mask, ok := TypeDerivationMask(rt, ComplexRef(2), ComplexRef(0))
	want := DerivationExtension | DerivationRestriction
	if !ok || mask != want {
		t.Fatalf("complex chain mask = %08b, %v; want %08b, true", mask, ok, want)
	}
	mask, ok = TypeDerivationMask(rt, ComplexRef(2), ComplexRef(1))
	if !ok || mask != DerivationRestriction {
		t.Fatalf("complex parent mask = %08b, %v; want restriction, true", mask, ok)
	}
}

func TestTypeDerivationMaskComplexSimpleBase(t *testing.T) {
	t.Parallel()

	rt := derivationRuntimeStub{
		anyType: 0,
		simple: []SimpleTypeDerivation{
			{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
		},
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: SimpleRef(0), Kind: DerivationKindExtension},
		},
	}
	mask, ok := TypeDerivationMask(rt, ComplexRef(1), SimpleRef(0))
	if !ok || mask != DerivationExtension {
		t.Fatalf("complex simple-base mask = %08b, %v; want extension, true", mask, ok)
	}
	mask, ok = TypeDerivationMask(rt, ComplexRef(1), ComplexRef(0))
	want := DerivationExtension | DerivationRestriction
	if !ok || mask != want {
		t.Fatalf("complex anyType mask = %08b, %v; want %08b, true", mask, ok, want)
	}
}

func TestTypeDerivationMaskRejectsInvalidOrCyclicGraph(t *testing.T) {
	t.Parallel()

	rt := derivationRuntimeStub{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(1), Kind: DerivationKindExtension},
		},
	}
	if mask, ok := TypeDerivationMask(rt, ComplexRef(4), ComplexRef(0)); ok {
		t.Fatalf("invalid complex type derived with mask %08b", mask)
	}
	if mask, ok := TypeDerivationMask(rt, ComplexRef(1), ComplexRef(0)); ok {
		t.Fatalf("cyclic complex type derived with mask %08b", mask)
	}
	if mask, ok := TypeDerivationMask(rt, ComplexRef(1), SimpleRef(0)); ok || mask != 0 {
		t.Fatalf("cyclic complex type derived from simple type with mask %08b, %v", mask, ok)
	}
}

func TestTypeDerivationMaskHandlesDeepComplexSimpleChain(t *testing.T) {
	t.Parallel()

	const depth = 10_000
	complexTypes := make([]ComplexTypeDerivation, depth)
	for i := range depth - 1 {
		complexTypes[i] = ComplexTypeDerivation{
			Base: ComplexRef(ComplexTypeID(i + 1)),
			Kind: DerivationKindExtension,
		}
	}
	complexTypes[depth-1] = ComplexTypeDerivation{Base: SimpleRef(0), Kind: DerivationKindExtension}
	rt := derivationRuntimeStub{complex: complexTypes}
	mask, ok := TypeDerivationMask(rt, ComplexRef(0), SimpleRef(0))
	if !ok || mask != DerivationExtension {
		t.Fatalf("deep complex simple-base mask = %08b, %v; want extension, true", mask, ok)
	}
}

func TestTypeDerivationMaskHandlesDeepComplexSimpleBaseChain(t *testing.T) {
	t.Parallel()

	const depth = 10_000
	simpleTypes := make([]SimpleTypeDerivation, depth)
	for i := range depth - 1 {
		simpleTypes[i] = SimpleTypeDerivation{Base: SimpleTypeID(i + 1), Variety: SimpleVarietyAtomic}
	}
	simpleTypes[depth-1] = SimpleTypeDerivation{Base: NoSimpleType, Variety: SimpleVarietyAtomic}
	rt := derivationRuntimeStub{
		simple:  simpleTypes,
		complex: []ComplexTypeDerivation{{Base: SimpleRef(0), Kind: DerivationKindExtension}},
	}
	mask, ok := TypeDerivationMask(rt, ComplexRef(0), SimpleRef(depth-1))
	want := DerivationExtension | DerivationRestriction
	if !ok || mask != want {
		t.Fatalf("deep complex simple-base chain mask = %08b, %v; want %08b, true", mask, ok, want)
	}
}

func TestTypeDerivationMaskSimpleUnionBranchingAndCycles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		simple  []SimpleTypeDerivation
		derived SimpleTypeID
		base    SimpleTypeID
		want    bool
	}{
		{
			name: "later union member",
			simple: []SimpleTypeDerivation{
				{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
				{Base: 0, Variety: SimpleVarietyAtomic},
				{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
				{Base: NoSimpleType, Variety: SimpleVarietyUnion, Union: []SimpleTypeID{2, 0}},
			},
			derived: 1,
			base:    3,
			want:    true,
		},
		{
			name: "base cycle",
			simple: []SimpleTypeDerivation{
				{Base: 1, Variety: SimpleVarietyAtomic},
				{Base: 0, Variety: SimpleVarietyAtomic},
				{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
			},
			derived: 0,
			base:    2,
		},
		{
			name: "union cycle",
			simple: []SimpleTypeDerivation{
				{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
				{Base: NoSimpleType, Variety: SimpleVarietyUnion, Union: []SimpleTypeID{1}},
			},
			derived: 0,
			base:    1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mask, ok := TypeDerivationMask(derivationRuntimeStub{simple: tt.simple}, SimpleRef(tt.derived), SimpleRef(tt.base))
			if ok != tt.want || ok && mask != DerivationRestriction || !ok && mask != 0 {
				t.Fatalf("TypeDerivationMask() = %08b, %v; want restriction=%v", mask, ok, tt.want)
			}
		})
	}
}

func TestIndexedTypeDerivationMatchesGraphQueries(t *testing.T) {
	simple := []SimpleType{
		{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
		{Base: 0, Variety: SimpleVarietyAtomic},
		{Base: 0, Variety: SimpleVarietyUnion, Union: []SimpleTypeID{0}},
		{Base: 2, Variety: SimpleVarietyAtomic},
		{Base: 0, Variety: SimpleVarietyUnion, Union: []SimpleTypeID{1, 2}},
	}
	complexTypes := []ComplexType{
		{Derivation: DerivationKindNone},
		{Base: ComplexRef(0), Derivation: DerivationKindExtension},
		{Base: ComplexRef(1), Derivation: DerivationKindRestriction},
		{Base: SimpleRef(1), Derivation: DerivationKindExtension},
		{Base: ComplexRef(3), Derivation: DerivationKindRestriction},
	}
	read := newTypeDerivationReadForTest(t, simple, complexTypes)
	stub := newDerivationRuntimeStub(simple, complexTypes)
	var scratch TypeDerivationScratch
	var types []TypeID
	for i := range simple {
		types = append(types, SimpleRef(SimpleTypeID(i)))
	}
	for i := range complexTypes {
		types = append(types, ComplexRef(ComplexTypeID(i)))
	}
	for _, derived := range types {
		for _, base := range types {
			wantMask, wantOK := TypeDerivationMask(stub, derived, base)
			gotMask, gotOK := read.derivation(derived, base, &scratch)
			if gotMask != wantMask || gotOK != wantOK {
				t.Fatalf("derivation(%v, %v) = %08b, %v; want %08b, %v", derived, base, gotMask, gotOK, wantMask, wantOK)
			}
		}
	}
}

func TestTypeDerivationIndexRejectsCyclesWithoutTraversal(t *testing.T) {
	simple := []SimpleType{
		{Base: 1, Variety: SimpleVarietyAtomic},
		{Base: 0, Variety: SimpleVarietyAtomic},
	}
	_, err := newTypeDerivationReadForTypes(
		0,
		simple,
		[]ComplexType{{Derivation: DerivationKindNone}},
		newSimpleTypeColdReadTable(simple),
	)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("newTypeDerivationReadForTypes() error = %v, want cycle rejection", err)
	}
}

func TestTypeDerivationProjectionAuditRejectsIndexCorruption(t *testing.T) {
	simple := []SimpleType{{Base: NoSimpleType, Variety: SimpleVarietyAtomic}}
	complexTypes := []ComplexType{
		{Derivation: DerivationKindNone},
		{Base: ComplexRef(0), Derivation: DerivationKindExtension},
	}
	tests := []struct {
		name   string
		mutate func(*TypeDerivationRead)
	}{
		{"missing index", func(read *TypeDerivationRead) { read.index = nil }},
		{"simple type table", func(read *TypeDerivationRead) { read.index.simpleTypes = nil }},
		{"simple interval", func(read *TypeDerivationRead) { read.index.simpleOut[0]++ }},
		{"complex interval", func(read *TypeDerivationRead) { read.index.complexIn[1] = 0 }},
		{"extension prefix", func(read *TypeDerivationRead) { read.index.complexExtensions[1] = 0 }},
		{"simple anchor", func(read *TypeDerivationRead) { read.index.complexSimpleBase[1] = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			read := newTypeDerivationReadForTest(t, simple, complexTypes)
			test.mutate(&read)
			if err := ValidateTypeDerivationReadProjection(read, 0, simple, complexTypes); err == nil {
				t.Fatal("ValidateTypeDerivationReadProjection() accepted index corruption")
			}
		})
	}
}

func TestIndexedTypeDerivationReusesBoundedScratch(t *testing.T) {
	simple := []SimpleType{
		{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
		{Base: 0, Variety: SimpleVarietyAtomic},
		{Base: NoSimpleType, Variety: SimpleVarietyUnion, Union: []SimpleTypeID{0}},
	}
	read := newTypeDerivationReadForTest(t, simple, nil)
	var scratch TypeDerivationScratch
	if mask, ok := read.derivation(SimpleRef(1), SimpleRef(2), &scratch); !ok || mask != DerivationRestriction {
		t.Fatalf("union derivation = %08b, %v", mask, ok)
	}
	allocs := testing.AllocsPerRun(100, func() {
		if _, ok := read.derivation(SimpleRef(1), SimpleRef(2), &scratch); !ok {
			t.Fatal("memoized union derivation failed")
		}
	})
	if allocs != 0 {
		t.Fatalf("memoized union derivation allocations = %v, want 0", allocs)
	}

	scratch.unionStack = make([]SimpleTypeID, 0, 5_000)
	scratch.unionSeen = make([]uint32, 5_000)
	scratch.Reset(4_096)
	if scratch.unionStack != nil || scratch.unionSeen != nil || scratch.unionGeneration != 0 {
		t.Fatalf("Reset() retained oversized scratch: %+v", scratch)
	}
}

func TestTypeDerivationScratchIsBoundToPublishedGraph(t *testing.T) {
	derived := SimpleRef(1)
	base := SimpleRef(0)
	withDerivation := newTypeDerivationReadForTest(t, []SimpleType{
		{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
		{Base: 0, Variety: SimpleVarietyAtomic},
	}, nil)
	withoutDerivation := newTypeDerivationReadForTest(t, []SimpleType{
		{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
		{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
	}, nil)

	var scratch TypeDerivationScratch
	if mask, ok := withDerivation.derivation(derived, base, &scratch); !ok || mask != DerivationRestriction {
		t.Fatalf("first graph derivation = %08b, %v; want restriction", mask, ok)
	}
	if mask, ok := withoutDerivation.derivation(derived, base, &scratch); ok || mask != 0 {
		t.Fatalf("second graph reused first graph memo: %08b, %v", mask, ok)
	}
}

func TestTypeDerivationScratchIsBoundToCompleteIndex(t *testing.T) {
	simpleTypes := []SimpleType{{Base: NoSimpleType, Variety: SimpleVarietyAtomic}}
	cold := newSimpleTypeColdReadTable(simpleTypes)
	extension, err := newTypeDerivationReadForTypes(0, simpleTypes, []ComplexType{
		{Derivation: DerivationKindNone},
		{Base: ComplexRef(0), Derivation: DerivationKindExtension},
	}, cold)
	if err != nil {
		t.Fatal(err)
	}
	restriction, err := newTypeDerivationReadForTypes(0, simpleTypes, []ComplexType{
		{Derivation: DerivationKindNone},
		{Base: ComplexRef(0), Derivation: DerivationKindRestriction},
	}, cold)
	if err != nil {
		t.Fatal(err)
	}

	var scratch TypeDerivationScratch
	if mask, ok := extension.derivation(ComplexRef(1), ComplexRef(0), &scratch); !ok || mask != DerivationExtension {
		t.Fatalf("extension graph derivation = %08b, %v; want extension", mask, ok)
	}
	if mask, ok := restriction.derivation(ComplexRef(1), ComplexRef(0), &scratch); !ok || mask != DerivationRestriction {
		t.Fatalf("restriction graph reused extension memo = %08b, %v", mask, ok)
	}
}

func TestSubstitutionDerivationAllowedAppliesElementAndTypeBlocks(t *testing.T) {
	t.Parallel()

	rt := derivationRuntimeStub{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(0), Kind: DerivationKindExtension, Block: DerivationRestriction},
			{Base: ComplexRef(1), Kind: DerivationKindRestriction},
		},
	}
	if SubstitutionDerivationAllowed(rt, ComplexRef(1), ComplexRef(0), DerivationExtension) {
		t.Fatal("element block allowed extension substitution")
	}
	if SubstitutionDerivationAllowed(rt, ComplexRef(2), ComplexRef(0), 0) {
		t.Fatal("intermediate type block allowed restriction substitution")
	}
	rt.complex[1].Block = 0
	if !SubstitutionDerivationAllowed(rt, ComplexRef(2), ComplexRef(0), 0) {
		t.Fatal("unblocked derivation was rejected")
	}
}
