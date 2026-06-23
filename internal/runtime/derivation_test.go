package runtime

import "testing"

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

func TestEqualSimpleTypeDerivations(t *testing.T) {
	t.Parallel()

	base := SimpleTypeDerivation{
		Union:   []SimpleTypeID{1, 2},
		Base:    3,
		Variety: SimpleVarietyUnion,
	}
	tests := []struct {
		name string
		a    SimpleTypeDerivation
		b    SimpleTypeDerivation
		want bool
	}{
		{
			name: "equal",
			a:    base,
			b: SimpleTypeDerivation{
				Union:   []SimpleTypeID{1, 2},
				Base:    3,
				Variety: SimpleVarietyUnion,
			},
			want: true,
		},
		{
			name: "union differs",
			a:    base,
			b: SimpleTypeDerivation{
				Union:   []SimpleTypeID{2, 1},
				Base:    3,
				Variety: SimpleVarietyUnion,
			},
		},
		{
			name: "base differs",
			a:    base,
			b: SimpleTypeDerivation{
				Union:   []SimpleTypeID{1, 2},
				Base:    4,
				Variety: SimpleVarietyUnion,
			},
		},
		{
			name: "variety differs",
			a:    base,
			b: SimpleTypeDerivation{
				Union:   []SimpleTypeID{1, 2},
				Base:    3,
				Variety: SimpleVarietyAtomic,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := EqualSimpleTypeDerivations(tt.a, tt.b); got != tt.want {
				t.Fatalf("EqualSimpleTypeDerivations() = %v, want %v", got, tt.want)
			}
		})
	}
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
	if !EqualSimpleTypeDerivations(projection, SimpleTypeDerivation{
		Union:   []SimpleTypeID{1, 2},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}) {
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

	simple := []SimpleTypeDerivation{{
		Union:   []SimpleTypeID{1},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}}
	complexDerivations := []ComplexTypeDerivation{{
		Base:  ComplexRef(0),
		Kind:  DerivationKindExtension,
		Block: DerivationRestriction,
	}}
	read := NewTypeDerivationRead(0, simple, complexDerivations)
	simple[0].Union[0] = 9
	complexDerivations[0].Kind = DerivationKindRestriction

	if read.AnyTypeID() != 0 {
		t.Fatalf("AnyTypeID() = %d, want 0", read.AnyTypeID())
	}
	if read.SimpleTypeCount() != 1 || read.ComplexTypeCount() != 1 {
		t.Fatalf("counts = %d, %d; want 1, 1", read.SimpleTypeCount(), read.ComplexTypeCount())
	}
	if !ValidTypeID(SimpleRef(0), read.SimpleTypeCount(), read.ComplexTypeCount()) ||
		!ValidTypeID(ComplexRef(0), read.SimpleTypeCount(), read.ComplexTypeCount()) ||
		ValidTypeID(SimpleRef(1), read.SimpleTypeCount(), read.ComplexTypeCount()) ||
		ValidTypeID(ComplexRef(1), read.SimpleTypeCount(), read.ComplexTypeCount()) ||
		ValidTypeID(TypeID{}, read.SimpleTypeCount(), read.ComplexTypeCount()) {
		t.Fatal("ValidTypeID() did not match published graph bounds")
	}

	gotSimple, ok := read.SimpleTypeDerivation(0)
	if !ok || !EqualSimpleTypeDerivations(gotSimple, SimpleTypeDerivation{
		Union:   []SimpleTypeID{1},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}) {
		t.Fatalf("SimpleTypeDerivation() = %+v, %v; want cloned simple projection", gotSimple, ok)
	}
	gotSimple.Union[0] = 8
	again, ok := read.SimpleTypeDerivation(0)
	if !ok || again.Union[0] != 1 {
		t.Fatalf("SimpleTypeDerivation() aliased union slice: %+v, %v", again, ok)
	}
	gotComplex, ok := read.ComplexTypeDerivation(0)
	if !ok || gotComplex.Kind != DerivationKindExtension {
		t.Fatalf("ComplexTypeDerivation() = %+v, %v; want cloned complex projection", gotComplex, ok)
	}
	if got := RuntimeAnyTypeID(read, BuiltinIDs{AnyType: 9}); got != 0 {
		t.Fatalf("RuntimeAnyTypeID(published) = %d, want 0", got)
	}
	if got := RuntimeComplexTypeCount(read, []ComplexType{{}, {}}); got != 1 {
		t.Fatalf("RuntimeComplexTypeCount(published) = %d, want 1", got)
	}
	if got, ok := RuntimeSimpleTypeDerivation(read, []SimpleType{{Base: 9}}, 0); !ok || !EqualSimpleTypeDerivations(got, again) {
		t.Fatalf("RuntimeSimpleTypeDerivation(published) = %+v, %v; want published projection", got, ok)
	}
	if got, ok := RuntimeComplexTypeDerivation(read, []ComplexType{{Derivation: DerivationKindRestriction}}, 0); !ok || got.Kind != DerivationKindExtension {
		t.Fatalf("RuntimeComplexTypeDerivation(published) = %+v, %v; want published projection", got, ok)
	}

	emptyRead := TypeDerivationRead{}
	compileSimpleTypes := []SimpleType{{
		Union:   []SimpleTypeID{1},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}}
	compileComplexTypes := []ComplexType{{
		Base:       ComplexRef(0),
		Derivation: DerivationKindExtension,
		Block:      DerivationRestriction,
	}}
	if got := RuntimeAnyTypeID(emptyRead, BuiltinIDs{AnyType: 2}); got != 2 {
		t.Fatalf("RuntimeAnyTypeID(compile) = %d, want 2", got)
	}
	if got := RuntimeComplexTypeCount(emptyRead, compileComplexTypes); got != 1 {
		t.Fatalf("RuntimeComplexTypeCount(compile) = %d, want 1", got)
	}
	if got, ok := RuntimeSimpleTypeDerivation(emptyRead, compileSimpleTypes, 0); !ok || !EqualSimpleTypeDerivations(got, again) {
		t.Fatalf("RuntimeSimpleTypeDerivation(compile) = %+v, %v; want type-derived projection", got, ok)
	}
	if got, ok := RuntimeComplexTypeDerivation(emptyRead, compileComplexTypes, 0); !ok || got.Kind != DerivationKindExtension {
		t.Fatalf("RuntimeComplexTypeDerivation(compile) = %+v, %v; want type-derived projection", got, ok)
	}

	expectedSimple := []SimpleTypeDerivation{{
		Union:   []SimpleTypeID{1},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}}
	if !EqualSimpleTypeDerivationReadProjection(read, expectedSimple) {
		t.Fatal("EqualSimpleTypeDerivationReadProjection() rejected matching projection")
	}
	expectedSimple[0].Base = 1
	if EqualSimpleTypeDerivationReadProjection(read, expectedSimple) {
		t.Fatal("EqualSimpleTypeDerivationReadProjection() accepted mismatched projection")
	}

	complexTypes := []ComplexType{{
		Base:       ComplexRef(0),
		Derivation: DerivationKindExtension,
		Block:      DerivationRestriction,
	}}
	if !EqualComplexTypeDerivationReadProjection(read, complexTypes) {
		t.Fatal("EqualComplexTypeDerivationReadProjection() rejected matching projection")
	}
	complexTypes[0].Derivation = DerivationKindRestriction
	if EqualComplexTypeDerivationReadProjection(read, complexTypes) {
		t.Fatal("EqualComplexTypeDerivationReadProjection() accepted mismatched projection")
	}

	simpleTypes := []SimpleType{{
		Union:   []SimpleTypeID{1},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}}
	read = NewTypeDerivationReadForTypes(0, simpleTypes, []ComplexType{{
		Base:       ComplexRef(0),
		Derivation: DerivationKindExtension,
		Block:      DerivationRestriction,
	}})
	simpleTypes[0].Union[0] = 9
	if !EqualSimpleTypeDerivationReadProjectionForTypes(read, []SimpleType{{
		Union:   []SimpleTypeID{1},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}}) {
		t.Fatal("NewTypeDerivationReadForTypes() did not publish cloned simple-type projection")
	}
	if EqualSimpleTypeDerivationReadProjectionForTypes(read, simpleTypes) {
		t.Fatal("EqualSimpleTypeDerivationReadProjectionForTypes() accepted mismatched simple type")
	}
	if err := ValidateTypeDerivationReadProjection(read, 0, []SimpleType{{
		Union:   []SimpleTypeID{1},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}}, []ComplexType{{
		Base:       ComplexRef(0),
		Derivation: DerivationKindExtension,
		Block:      DerivationRestriction,
	}}); err != nil {
		t.Fatalf("ValidateTypeDerivationReadProjection() error = %v", err)
	}
	if err := ValidateTypeDerivationReadProjection(read, 1, simpleTypes, complexTypes); err == nil || err.Error() != "type derivation projection stores invalid anyType" {
		t.Fatalf("ValidateTypeDerivationReadProjection(anyType) error = %v, want anyType invariant", err)
	}
	if err := ValidateTypeDerivationReadProjection(NewTypeDerivationRead(0, nil, NewComplexTypeDerivationsForComplexTypes(complexTypes)), 0, simpleTypes, complexTypes); err == nil || err.Error() != "simple type derivation projection count does not match types" {
		t.Fatalf("ValidateTypeDerivationReadProjection(simple count) error = %v, want simple count invariant", err)
	}
	if err := ValidateTypeDerivationReadProjection(NewTypeDerivationRead(0, NewSimpleTypeDerivationsForSimpleTypes(simpleTypes), nil), 0, simpleTypes, complexTypes); err == nil || err.Error() != "complex type derivation projection count does not match types" {
		t.Fatalf("ValidateTypeDerivationReadProjection(complex count) error = %v, want complex count invariant", err)
	}
	if err := ValidateTypeDerivationReadProjection(read, 0, simpleTypes, []ComplexType{{
		Base:       ComplexRef(0),
		Derivation: DerivationKindRestriction,
		Block:      DerivationRestriction,
	}}); err == nil || err.Error() != "simple type derivation projection does not match type" {
		t.Fatalf("ValidateTypeDerivationReadProjection(simple mismatch) error = %v, want simple mismatch invariant", err)
	}
	if err := ValidateTypeDerivationReadProjection(read, 0, []SimpleType{{
		Union:   []SimpleTypeID{1},
		Base:    NoSimpleType,
		Variety: SimpleVarietyUnion,
	}}, complexTypes); err == nil || err.Error() != "complex type derivation projection does not match type" {
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
