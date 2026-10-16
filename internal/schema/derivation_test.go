package schema

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

type derivationFixture struct {
	simple  []SimpleTypeDerivation
	complex []ComplexTypeDerivation
	anyType ComplexTypeID
}

func unboundedTypeDerivationWork(_ int) error { return nil }

func typeDerivationMaskForTest(rt *schemaBuild, derived, base TypeID) (DerivationMask, bool) {
	mask, ok, err := deriveTypeMask(rt, derived, base, unboundedTypeDerivationWork)
	if err != nil {
		return 0, false
	}
	return mask, ok
}

func substitutionDerivationAllowedForTest(rt *schemaBuild, derived, base TypeID, block DerivationMask) bool {
	mask, ok := typeDerivationMaskForTest(rt, derived, base)
	if !ok || mask&block != 0 {
		return false
	}
	typeBlocks, err := substitutionTypeBlocks(rt, derived, base, unboundedTypeDerivationWork)
	return err == nil && mask&typeBlocks == 0
}

func newTypeDerivationReadForTest(
	t *testing.T,
	simpleTypes []SimpleType,
	complexTypes []ComplexType,
) typeDerivationRead {
	t.Helper()
	if len(complexTypes) == 0 {
		complexTypes = []ComplexType{{Derivation: DerivationKindNone}}
	}
	read, err := newTypeDerivationReadForTypes(0, simpleTypes, complexTypes)
	if err != nil {
		t.Fatalf("newTypeDerivationReadForTypes() error = %v", err)
	}
	return read
}

func newDerivationRuntimeStub(simpleTypes []SimpleType, complexTypes []ComplexType) derivationFixture {
	stub := derivationFixture{
		simple:  make([]SimpleTypeDerivation, len(simpleTypes)),
		complex: make([]ComplexTypeDerivation, len(complexTypes)),
	}
	for i := range simpleTypes {
		stub.simple[i] = newSimpleTypeDerivationForSimpleType(simpleTypes[i])
	}
	for i := range complexTypes {
		stub.complex[i] = newComplexTypeDerivationForComplexType(complexTypes[i])
	}
	return stub
}

func TestComplexTypeDerivationForComplexType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  ComplexType
		want ComplexTypeDerivation
	}{
		{
			name: "equal",
			typ:  ComplexType{Base: ComplexRef(1), Derivation: DerivationKindExtension, Block: DerivationRestriction},
			want: ComplexTypeDerivation{Base: ComplexRef(1), Kind: DerivationKindExtension, Block: DerivationRestriction},
		},
		{
			name: "base differs",
			typ:  ComplexType{Base: ComplexRef(2), Derivation: DerivationKindExtension, Block: DerivationRestriction},
			want: ComplexTypeDerivation{Base: ComplexRef(2), Kind: DerivationKindExtension, Block: DerivationRestriction},
		},
		{
			name: "kind differs",
			typ:  ComplexType{Base: ComplexRef(1), Derivation: DerivationKindRestriction, Block: DerivationRestriction},
			want: ComplexTypeDerivation{Base: ComplexRef(1), Kind: DerivationKindRestriction, Block: DerivationRestriction},
		},
		{
			name: "block differs",
			typ:  ComplexType{Base: ComplexRef(1), Derivation: DerivationKindExtension},
			want: ComplexTypeDerivation{Base: ComplexRef(1), Kind: DerivationKindExtension},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := newComplexTypeDerivationForComplexType(tt.typ); got != tt.want {
				t.Fatalf("newComplexTypeDerivationForComplexType() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSimpleTypeDerivationForSimpleType(t *testing.T) {
	t.Parallel()

	st := testSimpleType(valuepkg.Union, NoSimpleType, 1, 2)
	projection := newSimpleTypeDerivationForSimpleType(st)
	if projection.Base != NoSimpleType ||
		projection.Variety != valuepkg.Union ||
		!slices.Equal(projection.Union, []SimpleTypeID{1, 2}) {
		t.Fatalf("NewSimpleTypeDerivationForSimpleType() = %+v, want projected simple type facts", projection)
	}
	st.ValueSpec.Union[0] = 9
	if projection.Union[0] != 1 {
		t.Fatalf("NewSimpleTypeDerivationForSimpleType() aliased union slice: %+v", projection)
	}
	other := newSimpleTypeDerivationForSimpleType(testSimpleType(valuepkg.Union, 3, 1, 2))
	if other.Base != 3 || other.Variety != valuepkg.Union || !slices.Equal(other.Union, []SimpleTypeID{1, 2}) {
		t.Fatalf("newSimpleTypeDerivationForSimpleType() = %+v, want base 3, union variety and members [1 2]", other)
	}
}

func TestTypeDerivationRead(t *testing.T) {
	t.Parallel()

	simpleTypes := []SimpleType{testSimpleType(valuepkg.Atomic, NoSimpleType)}
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
	var scratch TypeDerivationScratch
	if mask, valid := read.derivation(SimpleRef(0), SimpleRef(0), &scratch); !valid || mask != 0 {
		t.Fatalf("published self derivation = %v, %v; want 0, true", mask, valid)
	}
	if mask, valid := read.derivation(SimpleRef(1), SimpleRef(0), &scratch); valid || mask != 0 {
		t.Fatalf("missing published derivation = %v, %v; want 0, false", mask, valid)
	}
}

func TestTypeDerivationMaskSimpleTypeRestrictionAndUnionBase(t *testing.T) {
	t.Parallel()

	rt := derivationFixture{
		simple: []SimpleTypeDerivation{
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
			{Base: 0, Variety: valuepkg.Atomic},
			{Base: NoSimpleType, Variety: valuepkg.Union, Union: []SimpleTypeID{0}},
		},
		complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}},
	}
	mask, ok := typeDerivationMaskForTest(rt.build(), SimpleRef(1), SimpleRef(0))
	if !ok || mask != DerivationRestriction {
		t.Fatalf("simple restriction mask = %08b, %v; want restriction, true", mask, ok)
	}
	mask, ok = typeDerivationMaskForTest(rt.build(), SimpleRef(1), SimpleRef(2))
	if !ok || mask != DerivationRestriction {
		t.Fatalf("simple union-base mask = %08b, %v; want restriction, true", mask, ok)
	}
}

func TestTypeDerivationMaskComplexTypeChain(t *testing.T) {
	t.Parallel()

	rt := derivationFixture{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(0), Kind: DerivationKindExtension},
			{Base: ComplexRef(1), Kind: DerivationKindRestriction},
		},
	}
	mask, ok := typeDerivationMaskForTest(rt.build(), ComplexRef(2), ComplexRef(0))
	want := DerivationExtension | DerivationRestriction
	if !ok || mask != want {
		t.Fatalf("complex chain mask = %08b, %v; want %08b, true", mask, ok, want)
	}
	mask, ok = typeDerivationMaskForTest(rt.build(), ComplexRef(2), ComplexRef(1))
	if !ok || mask != DerivationRestriction {
		t.Fatalf("complex parent mask = %08b, %v; want restriction, true", mask, ok)
	}
}

func TestTypeDerivationMaskComplexSimpleBase(t *testing.T) {
	t.Parallel()

	rt := derivationFixture{
		anyType: 0,
		simple: []SimpleTypeDerivation{
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
		},
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: SimpleRef(0), Kind: DerivationKindExtension},
		},
	}
	mask, ok := typeDerivationMaskForTest(rt.build(), ComplexRef(1), SimpleRef(0))
	if !ok || mask != DerivationExtension {
		t.Fatalf("complex simple-base mask = %08b, %v; want extension, true", mask, ok)
	}
	mask, ok = typeDerivationMaskForTest(rt.build(), ComplexRef(1), ComplexRef(0))
	want := DerivationExtension | DerivationRestriction
	if !ok || mask != want {
		t.Fatalf("complex anyType mask = %08b, %v; want %08b, true", mask, ok, want)
	}
}

func TestTypeDerivationMaskRejectsInvalidOrCyclicGraph(t *testing.T) {
	t.Parallel()

	rt := derivationFixture{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(1), Kind: DerivationKindExtension},
		},
	}
	if mask, ok := typeDerivationMaskForTest(rt.build(), ComplexRef(4), ComplexRef(0)); ok {
		t.Fatalf("invalid complex type derived with mask %08b", mask)
	}
	if mask, ok := typeDerivationMaskForTest(rt.build(), ComplexRef(1), ComplexRef(0)); ok {
		t.Fatalf("cyclic complex type derived with mask %08b", mask)
	}
	if mask, ok := typeDerivationMaskForTest(rt.build(), ComplexRef(1), SimpleRef(0)); ok || mask != 0 {
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
	rt := derivationFixture{complex: complexTypes}
	mask, ok := typeDerivationMaskForTest(rt.build(), ComplexRef(0), SimpleRef(0))
	if !ok || mask != DerivationExtension {
		t.Fatalf("deep complex simple-base mask = %08b, %v; want extension, true", mask, ok)
	}
}

func TestTypeDerivationMaskHandlesDeepComplexSimpleBaseChain(t *testing.T) {
	t.Parallel()

	const depth = 10_000
	simpleTypes := make([]SimpleTypeDerivation, depth)
	for i := range depth - 1 {
		simpleTypes[i] = SimpleTypeDerivation{Base: SimpleTypeID(i + 1), Variety: valuepkg.Atomic}
	}
	simpleTypes[depth-1] = SimpleTypeDerivation{Base: NoSimpleType, Variety: valuepkg.Atomic}
	rt := derivationFixture{
		simple:  simpleTypes,
		complex: []ComplexTypeDerivation{{Base: SimpleRef(0), Kind: DerivationKindExtension}},
	}
	mask, ok := typeDerivationMaskForTest(rt.build(), ComplexRef(0), SimpleRef(depth-1))
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
				{Base: NoSimpleType, Variety: valuepkg.Atomic},
				{Base: 0, Variety: valuepkg.Atomic},
				{Base: NoSimpleType, Variety: valuepkg.Atomic},
				{Base: NoSimpleType, Variety: valuepkg.Union, Union: []SimpleTypeID{2, 0}},
			},
			derived: 1,
			base:    3,
			want:    true,
		},
		{
			name: "base cycle",
			simple: []SimpleTypeDerivation{
				{Base: 1, Variety: valuepkg.Atomic},
				{Base: 0, Variety: valuepkg.Atomic},
				{Base: NoSimpleType, Variety: valuepkg.Atomic},
			},
			derived: 0,
			base:    2,
		},
		{
			name: "union cycle",
			simple: []SimpleTypeDerivation{
				{Base: NoSimpleType, Variety: valuepkg.Atomic},
				{Base: NoSimpleType, Variety: valuepkg.Union, Union: []SimpleTypeID{1}},
			},
			derived: 0,
			base:    1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mask, ok := typeDerivationMaskForTest((derivationFixture{simple: tt.simple}).build(), SimpleRef(tt.derived), SimpleRef(tt.base))
			if ok != tt.want || ok && mask != DerivationRestriction || !ok && mask != 0 {
				t.Fatalf("typeDerivationMaskForTest() = %08b, %v; want restriction=%v", mask, ok, tt.want)
			}
		})
	}
}

func TestIndexedTypeDerivationMatchesGraphQueries(t *testing.T) {
	simple := []SimpleType{
		testSimpleType(valuepkg.Atomic, NoSimpleType),
		testSimpleType(valuepkg.Atomic, 0),
		testSimpleType(valuepkg.Union, 0, 0),
		testSimpleType(valuepkg.Atomic, 2),
		testSimpleType(valuepkg.Union, 0, 1, 2),
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
			wantMask, wantOK := typeDerivationMaskForTest(stub.build(), derived, base)
			gotMask, gotOK := read.derivation(derived, base, &scratch)
			if gotMask != wantMask || gotOK != wantOK {
				t.Fatalf("derivation(%v, %v) = %08b, %v; want %08b, %v", derived, base, gotMask, gotOK, wantMask, wantOK)
			}
		}
	}
}

func TestTypeDerivationIndexRejectsCyclesWithoutTraversal(t *testing.T) {
	simple := []SimpleType{
		testSimpleType(valuepkg.Atomic, 1),
		testSimpleType(valuepkg.Atomic, 0),
	}
	_, err := newTypeDerivationReadForTypes(
		0,
		simple,
		[]ComplexType{{Derivation: DerivationKindNone}},
	)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("newTypeDerivationReadForTypes() error = %v, want cycle rejection", err)
	}
}

func TestIndexedTypeDerivationReusesBoundedScratch(t *testing.T) {
	simple := []SimpleType{
		testSimpleType(valuepkg.Atomic, NoSimpleType),
		testSimpleType(valuepkg.Atomic, 0),
		testSimpleType(valuepkg.Union, NoSimpleType, 0),
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
		testSimpleType(valuepkg.Atomic, NoSimpleType),
		testSimpleType(valuepkg.Atomic, 0),
	}, nil)
	withoutDerivation := newTypeDerivationReadForTest(t, []SimpleType{
		testSimpleType(valuepkg.Atomic, NoSimpleType),
		testSimpleType(valuepkg.Atomic, NoSimpleType),
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
	simpleTypes := []SimpleType{testSimpleType(valuepkg.Atomic, NoSimpleType)}
	extension, err := newTypeDerivationReadForTypes(0, simpleTypes, []ComplexType{
		{Derivation: DerivationKindNone},
		{Base: ComplexRef(0), Derivation: DerivationKindExtension},
	})
	if err != nil {
		t.Fatal(err)
	}
	restriction, err := newTypeDerivationReadForTypes(0, simpleTypes, []ComplexType{
		{Derivation: DerivationKindNone},
		{Base: ComplexRef(0), Derivation: DerivationKindRestriction},
	})
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

	rt := derivationFixture{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(0), Kind: DerivationKindExtension, Block: DerivationRestriction},
			{Base: ComplexRef(1), Kind: DerivationKindRestriction},
		},
	}
	if substitutionDerivationAllowedForTest(rt.build(), ComplexRef(1), ComplexRef(0), DerivationExtension) {
		t.Fatal("element block allowed extension substitution")
	}
	if substitutionDerivationAllowedForTest(rt.build(), ComplexRef(2), ComplexRef(0), 0) {
		t.Fatal("intermediate type block allowed restriction substitution")
	}
	rt.complex[1].Block = 0
	if !substitutionDerivationAllowedForTest(rt.build(), ComplexRef(2), ComplexRef(0), 0) {
		t.Fatal("unblocked derivation was rejected")
	}
}

func TestDerivationTraversalWorkFailureCanBeRetried(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		derived TypeID
		base    TypeID
		want    DerivationMask
	}{
		{name: "complex to simple", derived: ComplexRef(4), base: SimpleRef(0), want: DerivationExtension},
		{name: "complex to anyType", derived: ComplexRef(4), base: ComplexRef(0), want: DerivationExtension | DerivationRestriction},
		{name: "complex to complex", derived: ComplexRef(3), base: ComplexRef(1), want: DerivationExtension | DerivationRestriction},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			simple := []SimpleType{{ValueSpec: valuepkg.TypeSpec{Base: NoSimpleType, Variety: valuepkg.Atomic}}}
			complexTypes := []ComplexType{
				{Derivation: DerivationKindNone},
				{Base: ComplexRef(0), Derivation: DerivationKindExtension},
				{Base: ComplexRef(1), Derivation: DerivationKindExtension},
				{Base: ComplexRef(2), Derivation: DerivationKindRestriction},
				{Base: SimpleRef(0), Derivation: DerivationKindExtension},
			}
			build := &schemaBuild{SimpleTypes: slices.Clone(simple), ComplexTypes: slices.Clone(complexTypes), AnyType: 0}
			budget := newWorkBudget(dependencyWorkBudget, 1)
			mask, found, err := deriveTypeMask(build, tc.derived, tc.base, budget.spend)
			if mask != 0 || found {
				t.Fatalf("failed traversal published result: mask=%v found=%v", mask, found)
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaLimit {
				t.Fatalf("nested work failure diagnostic: %v", err)
			}
			if !reflect.DeepEqual(build.SimpleTypes, simple) || !slices.Equal(build.ComplexTypes, complexTypes) {
				t.Fatal("failed traversal changed canonical type records")
			}
			budget = newWorkBudget(dependencyWorkBudget, 16)
			mask, found, err = deriveTypeMask(build, tc.derived, tc.base, budget.spend)
			if err != nil || !found || mask != tc.want {
				t.Fatalf("retry: mask=%v found=%v err=%v, want mask=%v", mask, found, err, tc.want)
			}
			if !reflect.DeepEqual(build.SimpleTypes, simple) || !slices.Equal(build.ComplexTypes, complexTypes) {
				t.Fatal("retry changed canonical type records")
			}
		})
	}
}

func fixtureSimpleTypes(in []SimpleTypeDerivation) []SimpleType {
	out := make([]SimpleType, len(in))
	for i, typ := range in {
		out[i].ValueSpec = valuepkg.TypeSpec{Base: typ.Base, Variety: typ.Variety, Union: slices.Clone(typ.Union)}
	}
	return out
}

func fixtureComplexTypes(in []ComplexTypeDerivation) []ComplexType {
	out := make([]ComplexType, len(in))
	for i, typ := range in {
		out[i] = ComplexType{Base: typ.Base, Derivation: typ.Kind, Block: typ.Block}
	}
	return out
}

func (f derivationFixture) build() *schemaBuild {
	return &schemaBuild{SimpleTypes: fixtureSimpleTypes(f.simple), ComplexTypes: fixtureComplexTypes(f.complex), AnyType: f.anyType}
}
