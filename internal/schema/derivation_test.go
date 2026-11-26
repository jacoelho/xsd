package schema

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

type derivationFixture struct {
	simple  map[SimpleTypeID]simpleTypeDerivation
	complex []complexTypeDerivation
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
	simpleTypes []simpleTypeDerivation,
	complexTypes []complexTypeMetadata,
) typeDerivationRead {
	t.Helper()
	if len(complexTypes) == 0 {
		complexTypes = []complexTypeMetadata{{Derivation: derivationKindNone}}
	}
	read, err := newTypeDerivationRead(0, simpleTypes, complexTypes)
	if err != nil {
		t.Fatalf("newTypeDerivationRead() error = %v", err)
	}
	return read
}

func TestComplexTypeDerivationForComplexType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		typ  complexTypeMetadata
		want complexTypeDerivation
	}{
		{
			name: "equal",
			typ:  complexTypeMetadata{Base: ComplexRef(1), Derivation: derivationKindExtension, Block: DerivationRestriction},
			want: complexTypeDerivation{Base: ComplexRef(1), Kind: derivationKindExtension, Block: DerivationRestriction},
		},
		{
			name: "base differs",
			typ:  complexTypeMetadata{Base: ComplexRef(2), Derivation: derivationKindExtension, Block: DerivationRestriction},
			want: complexTypeDerivation{Base: ComplexRef(2), Kind: derivationKindExtension, Block: DerivationRestriction},
		},
		{
			name: "kind differs",
			typ:  complexTypeMetadata{Base: ComplexRef(1), Derivation: derivationKindRestriction, Block: DerivationRestriction},
			want: complexTypeDerivation{Base: ComplexRef(1), Kind: derivationKindRestriction, Block: DerivationRestriction},
		},
		{
			name: "block differs",
			typ:  complexTypeMetadata{Base: ComplexRef(1), Derivation: derivationKindExtension},
			want: complexTypeDerivation{Base: ComplexRef(1), Kind: derivationKindExtension},
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

func TestSimpleTypeDerivationFromView(t *testing.T) {
	t.Parallel()

	view := valuepkg.TypeView{Variety: valuepkg.Union, Base: 3, Union: []SimpleTypeID{1, 2}}
	got := newSimpleTypeDerivation(view, true)
	if got.Base != 3 || got.Variety != valuepkg.Union || !got.UnionHasFacets || !slices.Equal(got.Union, []SimpleTypeID{1, 2}) {
		t.Fatalf("newSimpleTypeDerivation() = %+v, want base 3, union variety, facets and members [1 2]", got)
	}
}

func TestTypeDerivationRead(t *testing.T) {
	t.Parallel()

	simpleTypes := []simpleTypeDerivation{testSimpleType(valuepkg.Atomic, NoSimpleType)}
	complexTypes := []complexTypeMetadata{{
		Derivation: derivationKindNone,
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

	root, restriction, union := userSimpleType(0), userSimpleType(1), userSimpleType(2)
	rt := derivationFixture{
		simple: map[SimpleTypeID]simpleTypeDerivation{
			root:        {Base: NoSimpleType, Variety: valuepkg.Atomic},
			restriction: {Base: root, Variety: valuepkg.Atomic},
			union:       {Base: NoSimpleType, Variety: valuepkg.Union, Union: []SimpleTypeID{root}},
		},
		complex: []complexTypeDerivation{{Kind: derivationKindNone}},
	}
	mask, ok := typeDerivationMaskForTest(rt.build(), SimpleRef(restriction), SimpleRef(root))
	if !ok || mask != DerivationRestriction {
		t.Fatalf("simple restriction mask = %08b, %v; want restriction, true", mask, ok)
	}
	mask, ok = typeDerivationMaskForTest(rt.build(), SimpleRef(restriction), SimpleRef(union))
	if !ok || mask != DerivationRestriction {
		t.Fatalf("simple union-base mask = %08b, %v; want restriction, true", mask, ok)
	}
}

func TestTypeDerivationMaskComplexTypeChain(t *testing.T) {
	t.Parallel()

	rt := derivationFixture{
		anyType: 0,
		complex: []complexTypeDerivation{
			{Kind: derivationKindNone},
			{Base: ComplexRef(0), Kind: derivationKindExtension},
			{Base: ComplexRef(1), Kind: derivationKindRestriction},
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

	root := userSimpleType(0)
	rt := derivationFixture{
		anyType: 0,
		simple: map[SimpleTypeID]simpleTypeDerivation{
			root: {Base: NoSimpleType, Variety: valuepkg.Atomic},
		},
		complex: []complexTypeDerivation{
			{Kind: derivationKindNone},
			{Base: SimpleRef(root), Kind: derivationKindExtension},
		},
	}
	mask, ok := typeDerivationMaskForTest(rt.build(), ComplexRef(1), SimpleRef(root))
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
		complex: []complexTypeDerivation{
			{Kind: derivationKindNone},
			{Base: ComplexRef(1), Kind: derivationKindExtension},
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
	complexTypes := make([]complexTypeDerivation, depth)
	for i := range depth - 1 {
		complexTypes[i] = complexTypeDerivation{
			Base: ComplexRef(ComplexTypeID(i + 1)),
			Kind: derivationKindExtension,
		}
	}
	complexTypes[depth-1] = complexTypeDerivation{Base: SimpleRef(0), Kind: derivationKindExtension}
	rt := derivationFixture{complex: complexTypes}
	mask, ok := typeDerivationMaskForTest(rt.build(), ComplexRef(0), SimpleRef(0))
	if !ok || mask != DerivationExtension {
		t.Fatalf("deep complex simple-base mask = %08b, %v; want extension, true", mask, ok)
	}
}

func TestTypeDerivationMaskHandlesDeepComplexSimpleBaseChain(t *testing.T) {
	t.Parallel()

	const depth = 10_000
	simpleTypes := map[SimpleTypeID]simpleTypeDerivation{userSimpleType(0): {Base: NoSimpleType, Variety: valuepkg.Atomic}}
	for i := SimpleTypeID(1); i < depth; i++ {
		simpleTypes[userSimpleType(i)] = simpleTypeDerivation{Base: userSimpleType(i - 1), Variety: valuepkg.Atomic}
	}
	rt := derivationFixture{
		simple:  simpleTypes,
		complex: []complexTypeDerivation{{Base: SimpleRef(userSimpleType(depth - 1)), Kind: derivationKindExtension}},
	}
	mask, ok := typeDerivationMaskForTest(rt.build(), ComplexRef(0), SimpleRef(userSimpleType(0)))
	want := DerivationExtension | DerivationRestriction
	if !ok || mask != want {
		t.Fatalf("deep complex simple-base chain mask = %08b, %v; want %08b, true", mask, ok, want)
	}
}

func TestTypeDerivationMaskFindsRestrictionOfALaterUnionMember(t *testing.T) {
	t.Parallel()

	root, restriction, other, union := userSimpleType(0), userSimpleType(1), userSimpleType(2), userSimpleType(3)
	rt := derivationFixture{
		simple: map[SimpleTypeID]simpleTypeDerivation{
			root:        {Base: NoSimpleType, Variety: valuepkg.Atomic},
			restriction: {Base: root, Variety: valuepkg.Atomic},
			other:       {Base: NoSimpleType, Variety: valuepkg.Atomic},
			union:       {Base: NoSimpleType, Variety: valuepkg.Union, Union: []SimpleTypeID{other, root}},
		},
	}
	mask, ok := typeDerivationMaskForTest(rt.build(), SimpleRef(restriction), SimpleRef(union))
	if !ok || mask != DerivationRestriction {
		t.Fatalf("typeDerivationMaskForTest() = %08b, %v; want restriction, true", mask, ok)
	}
}

func TestTypeDerivationIndexReportsCorruptTypeTablesAsBuildDefects(t *testing.T) {
	t.Parallel()
	type tables struct {
		anyType ComplexTypeID
		simple  []simpleTypeDerivation
		complex []complexTypeMetadata
	}
	intact := func() tables {
		return tables{
			simple: []simpleTypeDerivation{
				testSimpleType(valuepkg.Atomic, NoSimpleType),
				testSimpleType(valuepkg.Atomic, 0),
			},
			complex: []complexTypeMetadata{
				{Derivation: derivationKindNone},
				{Base: ComplexRef(0), Derivation: derivationKindExtension},
				{Base: SimpleRef(1), Derivation: derivationKindExtension},
			},
		}
	}
	tests := []struct {
		name    string
		corrupt func(*tables)
		want    error
	}{
		{"anyType is the first ID past the complex type table", func(tb *tables) { tb.anyType = 3 }, errDerivationAnyTypeInvalid},
		{"simple base is the first ID past the simple type table", func(tb *tables) { tb.simple[1].Base = 2 }, errSimpleDerivationBaseInvalid},
		{"simple type is its own base", func(tb *tables) { tb.simple[1].Base = 1 }, errSimpleDerivationCycle},
		{"simple derivation cycle", func(tb *tables) { tb.simple[0].Base = 1 }, errSimpleDerivationCycle},
		{"complex base is the first ID past the complex type table", func(tb *tables) { tb.complex[1].Base = ComplexRef(3) }, errComplexDerivationBaseInvalid},
		{"complex type is its own base", func(tb *tables) { tb.complex[1].Base = ComplexRef(1) }, errComplexDerivationCycle},
		{"complex derivation cycle", func(tb *tables) { tb.complex[0].Base = ComplexRef(1) }, errComplexDerivationCycle},
		{"complex simple base is the first ID past the simple type table", func(tb *tables) { tb.complex[2].Base = SimpleRef(2) }, errComplexDerivationSimpleInvalid},
		{"complex derivation kind out of range", func(tb *tables) { tb.complex[1].Derivation = derivationKind(200) }, errDerivationKindInvalid},
	}
	base := intact()
	if _, err := newTypeDerivationRead(base.anyType, base.simple, base.complex); err != nil {
		t.Fatalf("intact tables: newTypeDerivationRead() error = %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tb := intact()
			tt.corrupt(&tb)
			if _, err := newTypeDerivationRead(tb.anyType, tb.simple, tb.complex); !errors.Is(err, tt.want) {
				t.Fatalf("newTypeDerivationRead() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestIndexedTypeDerivationReusesBoundedScratch(t *testing.T) {
	simple := []simpleTypeDerivation{
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
	withDerivation := newTypeDerivationReadForTest(t, []simpleTypeDerivation{
		testSimpleType(valuepkg.Atomic, NoSimpleType),
		testSimpleType(valuepkg.Atomic, 0),
	}, nil)
	withoutDerivation := newTypeDerivationReadForTest(t, []simpleTypeDerivation{
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
	simpleTypes := []simpleTypeDerivation{testSimpleType(valuepkg.Atomic, NoSimpleType)}
	extension, err := newTypeDerivationRead(0, simpleTypes, []complexTypeMetadata{
		{Derivation: derivationKindNone},
		{Base: ComplexRef(0), Derivation: derivationKindExtension},
	})
	if err != nil {
		t.Fatal(err)
	}
	restriction, err := newTypeDerivationRead(0, simpleTypes, []complexTypeMetadata{
		{Derivation: derivationKindNone},
		{Base: ComplexRef(0), Derivation: derivationKindRestriction},
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
		complex: []complexTypeDerivation{
			{Kind: derivationKindNone},
			{Base: ComplexRef(0), Kind: derivationKindExtension, Block: DerivationRestriction},
			{Base: ComplexRef(1), Kind: derivationKindRestriction},
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
			simple := []simpleType{{}}
			complexTypes := []complexTypeMetadata{
				{Derivation: derivationKindNone},
				{Base: ComplexRef(0), Derivation: derivationKindExtension},
				{Base: ComplexRef(1), Derivation: derivationKindExtension},
				{Base: ComplexRef(2), Derivation: derivationKindRestriction},
				{Base: SimpleRef(0), Derivation: derivationKindExtension},
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

// userSimpleType returns the ID a fresh value.Builder issues to the n-th type
// it reserves: the first ID after the builtins.
func userSimpleType(n SimpleTypeID) SimpleTypeID {
	return valuepkg.BuiltinTypeCount + n
}

// fixtureSimpleTypes completes, in ID order, the user simple types
// derivations describes through a value.Builder, the owner of simple-type
// structure. A root atomic type, one whose Base is NoSimpleType, restricts
// xs:string; a root union has the base xs:anySimpleType. A user ID below the
// last one derivations describes and absent from it is the unavailable-type
// sentinel. It panics when the builder rejects a description or completes a
// different derivation, since the build is then not the graph the test
// describes.
func fixtureSimpleTypes(derivations map[SimpleTypeID]simpleTypeDerivation) ([]simpleType, *valuepkg.Builder) {
	var count SimpleTypeID
	for id := range derivations {
		if id < valuepkg.BuiltinTypeCount {
			panic(fmt.Sprintf("simple derivation fixture %d is a builtin ID", id))
		}
		count = max(count, id-valuepkg.BuiltinTypeCount+1)
	}
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxDepth: math.MaxUint16, MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	types := make([]simpleType, valuepkg.BuiltinTypeCount, valuepkg.BuiltinTypeCount+count)
	for i := range count {
		id, err := builder.Reserve()
		if err != nil || id != userSimpleType(i) {
			panic(fmt.Sprintf("Reserve() = %d, %v; want %d", id, err, userSimpleType(i)))
		}
		want, described := derivations[id]
		if !described {
			if err = builder.Complete(id, missingValueTypeSpec()); err != nil {
				panic(fmt.Sprintf("Complete(%d) missing type: %v", id, err))
			}
			types = append(types, simpleType{Missing: true})
			continue
		}
		if err = builder.Complete(id, fixtureSimpleTypeSpec(want)); err != nil {
			panic(fmt.Sprintf("Complete(%d) %+v: %v", id, want, err))
		}
		got, err := valueSimpleTypeDerivation(builder, id)
		if err != nil {
			panic(fmt.Sprintf("derivation of %d: %v", id, err))
		}
		if want.Base == NoSimpleType {
			want.Base = got.Base
		}
		if got.Base != want.Base || got.Variety != want.Variety || got.UnionHasFacets != want.UnionHasFacets || !slices.Equal(got.Union, want.Union) {
			panic(fmt.Sprintf("completed %d as %+v, want %+v", id, got, want))
		}
		types = append(types, simpleType{})
	}
	return types, builder
}

// The simple types of restrictionFixtureTypes.
const (
	fixtureRoot SimpleTypeID = valuepkg.BuiltinTypeCount + iota
	fixtureRestriction
	fixtureUnrelated
)

// restrictionFixtureTypes returns a root atomic type, a restriction of it and
// an unrelated root.
func restrictionFixtureTypes() map[SimpleTypeID]simpleTypeDerivation {
	return map[SimpleTypeID]simpleTypeDerivation{
		fixtureRoot:        {Base: NoSimpleType, Variety: valuepkg.Atomic},
		fixtureRestriction: {Base: fixtureRoot, Variety: valuepkg.Atomic},
		fixtureUnrelated:   {Base: NoSimpleType, Variety: valuepkg.Atomic},
	}
}

func fixtureSimpleTypeSpec(d simpleTypeDerivation) valuepkg.TypeSpec {
	spec := valuepkg.TypeSpec{
		Variety:   d.Variety,
		Primitive: valuepkg.PrimitiveString,
		Base:      d.Base,
		ListItem:  valuepkg.NoType,
	}
	switch d.Variety {
	case valuepkg.Atomic:
		spec.Whitespace = valuepkg.WhitespacePreserve
		if spec.Base == NoSimpleType {
			spec.Base = valuepkg.BuiltinType(valuepkg.PrimitiveString)
		}
	case valuepkg.Union:
		spec.Whitespace, spec.WhitespacePresent = valuepkg.WhitespaceCollapse, true
		spec.Union = d.Union
		if spec.Base == NoSimpleType {
			spec.Base = valuepkg.AnySimpleTypeID
		}
		if d.UnionHasFacets {
			spec.Facets.Enumeration = []valuepkg.LiteralSpec{{Lexical: "x", Type: valuepkg.NoType}}
		}
	case valuepkg.List:
		panic("list fixture types are not supported")
	}
	return spec
}

func fixtureComplexTypes(in []complexTypeDerivation) []complexTypeMetadata {
	out := make([]complexTypeMetadata, len(in))
	for i, typ := range in {
		out[i] = complexTypeMetadata{Base: typ.Base, Derivation: typ.Kind, Block: typ.Block}
	}
	return out
}

func (f derivationFixture) build() *schemaBuild {
	simpleTypes, builder := fixtureSimpleTypes(f.simple)
	return &schemaBuild{
		SimpleTypes:  simpleTypes,
		valueBuilder: builder,
		ComplexTypes: fixtureComplexTypes(f.complex),
		AnyType:      f.anyType,
	}
}

func TestMarkSimpleTypeSeenTreatsAnIDBeyondTheSetAsUnmarkable(t *testing.T) {
	for _, id := range []SimpleTypeID{64, 1 << 31, 1<<31 + 1, ^SimpleTypeID(0)} {
		seen := make([]uint64, 1)
		if markSimpleTypeSeen(seen, id) {
			t.Errorf("markSimpleTypeSeen(%d) = true, want false for an ID beyond the set", id)
		}
		if seen[0] != 0 {
			t.Errorf("markSimpleTypeSeen(%d) set %#x, want nothing", id, seen[0])
		}
	}
}

func TestMarkSimpleTypeSeenMarksAnIDOnce(t *testing.T) {
	seen := make([]uint64, 2)
	if !markSimpleTypeSeen(seen, 127) {
		t.Fatal("first markSimpleTypeSeen(127) = false, want true")
	}
	if markSimpleTypeSeen(seen, 127) {
		t.Fatal("second markSimpleTypeSeen(127) = true, want false")
	}
	if seen[0] != 0 || seen[1] != 1<<63 {
		t.Fatalf("seen = %#x, want only bit 63 of word 1", seen)
	}
}

// buildWithUncompletedSimpleType returns a build of restrictionFixtureTypes
// that also declares a simple type its value builder never completed.
func buildWithUncompletedSimpleType() (*schemaBuild, SimpleTypeID) {
	build := derivationFixture{simple: restrictionFixtureTypes()}.build()
	uncompleted := fixtureUnrelated + 1
	build.SimpleTypes = append(build.SimpleTypes, simpleType{})
	return build, uncompleted
}

func TestSimpleTypeDerivationReportsATypeWithoutAValueDefinitionAsABuildDefect(t *testing.T) {
	t.Parallel()

	build, uncompleted := buildWithUncompletedSimpleType()
	if _, found, err := build.SimpleTypeDerivation(uncompleted); found || !errors.Is(err, errSimpleDerivationNoValueType) {
		t.Fatalf("SimpleTypeDerivation(uncompleted) = %v, %v; want a missing value definition", found, err)
	}
	outside := uncompleted + 1
	if _, found, err := build.SimpleTypeDerivation(outside); found || !errors.Is(err, errSimpleDerivationTypeInvalid) {
		t.Fatalf("SimpleTypeDerivation(outside) = %v, %v; want an invalid type", found, err)
	}
	build.SimpleTypes[uncompleted].Missing = true
	if _, found, err := build.SimpleTypeDerivation(uncompleted); found || err != nil {
		t.Fatalf("SimpleTypeDerivation(missing) = %v, %v; want not found and no error", found, err)
	}
}

func TestSimpleDerivationWalksReportATypeWithoutAValueDefinitionAsABuildDefect(t *testing.T) {
	t.Parallel()

	walks := map[string]func(rt *schemaBuild, uncompleted SimpleTypeID) error{
		"derived without a value definition": func(rt *schemaBuild, uncompleted SimpleTypeID) error {
			_, _, err := deriveTypeMask(rt, SimpleRef(uncompleted), SimpleRef(fixtureRoot), unboundedTypeDerivationWork)
			return err
		},
		"base without a value definition": func(rt *schemaBuild, uncompleted SimpleTypeID) error {
			_, _, err := deriveTypeMask(rt, SimpleRef(fixtureRestriction), SimpleRef(uncompleted), unboundedTypeDerivationWork)
			return err
		},
		"union membership of a base without a value definition": func(rt *schemaBuild, uncompleted SimpleTypeID) error {
			_, err := new(unionMemberWalk).reaches(rt, fixtureRoot, uncompleted, unboundedTypeDerivationWork)
			return err
		},
		"union flattening of a member without a value definition": func(rt *schemaBuild, uncompleted SimpleTypeID) error {
			var members []SimpleTypeID
			_, _, err := rt.appendFlattenedUnionMember(&members, uncompleted, map[SimpleTypeID]struct{}{}, 1)
			return err
		},
		"attribute restriction to a type without a value definition": func(rt *schemaBuild, uncompleted SimpleTypeID) error {
			return validateAttributeUseRestriction(rt, attributeUse{Type: fixtureRoot}, attributeUse{Type: uncompleted})
		},
	}
	for name, walk := range walks {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			build, uncompleted := buildWithUncompletedSimpleType()
			if err := walk(build, uncompleted); !errors.Is(err, errSimpleDerivationNoValueType) {
				t.Fatalf("walk error = %v, want a missing value definition build defect", err)
			}
			build.SimpleTypes[uncompleted].Missing = true
			if err := walk(build, uncompleted); errors.Is(err, errSimpleDerivationNoValueType) || isBuildDefect(err) {
				t.Fatalf("walk error over a missing type = %v, want no build defect", err)
			}
		})
	}
}
