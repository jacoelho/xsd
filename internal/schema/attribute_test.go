package schema

import (
	"maps"
	"slices"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func TestAttributeWildcardDerivationValidity(t *testing.T) {
	t.Parallel()

	for _, kind := range []AttributeWildcardDerivation{
		AttributeWildcardNone,
		AttributeWildcardRestriction,
		AttributeWildcardExtension,
	} {
		if !ValidAttributeWildcardDerivation(kind) {
			t.Fatalf("ValidAttributeWildcardDerivation(%d) = false", kind)
		}
	}
	if ValidAttributeWildcardDerivation(AttributeWildcardDerivation(99)) {
		t.Fatal("invalid attribute wildcard derivation was accepted")
	}
}

func TestAttributeUseRead(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	fixed := newValueConstraintRead("01", "1", builtinValueForTest(t, "integer", "01"))
	def := newValueConstraintRead("02", "2", builtinValueForTest(t, "integer", "02"))
	use := newTestAttributeUseRead(attributeUseReadShape{
		Name:                 name,
		Type:                 7,
		Label:                "a",
		Fixed:                fixed,
		Default:              def,
		Required:             true,
		HasFixed:             true,
		HasDefault:           true,
		FixedFromDeclaration: true,
	})
	if use.Name() != name {
		t.Fatalf("Name() = %v, want %v", use.Name(), name)
	}
	if use.TypeID() != 7 {
		t.Fatalf("TypeID() = %d, want 7", use.TypeID())
	}
	if use.Label() != "a" {
		t.Fatalf("Label() = %q, want a", use.Label())
	}
	if !use.Required() {
		t.Fatal("Required() = false, want true")
	}
	if !use.FixedUsesValueSpace() {
		t.Fatal("FixedUsesValueSpace() = false, want true")
	}
	if !use.CanValidateFixedStringFast() {
		t.Fatal("CanValidateFixedStringFast() = false, want true")
	}
	if newAttributeUseReadForSimpleTypes(attributeUseReadShape{Type: 7, HasFixed: true}, nil).CanValidateFixedStringFast() {
		t.Fatal("CanValidateFixedStringFast() = true without simple-value read, want false")
	}
	if got, ok := use.FixedValue(); !ok || got != fixed {
		t.Fatalf("FixedValue() = %+v, %v; want fixed, true", got, ok)
	}
	if got, ok := use.AbsentValueConstraint(); !ok || got != fixed {
		t.Fatalf("AbsentValueConstraint() = %+v, %v; want fixed, true", got, ok)
	}

	defaulted := newTestAttributeUseRead(attributeUseReadShape{Default: def, HasDefault: true})
	if got, ok := defaulted.AbsentValueConstraint(); !ok || got != def {
		t.Fatalf("AbsentValueConstraint(defaulted) = %+v, %v; want default, true", got, ok)
	}

	var zero AttributeUseRead
	if got, ok := zero.FixedValue(); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("zero FixedValue() = %+v, %v; want zero, false", got, ok)
	}
	if got, ok := zero.AbsentValueConstraint(); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("zero AbsentValueConstraint() = %+v, %v; want zero, false", got, ok)
	}
}

func TestAttributeDeclRead(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	fixed := newValueConstraintRead("01", "1", builtinValueForTest(t, "integer", "01"))
	decl := newAttributeDeclRead(attributeDeclReadShape{
		Name:     name,
		Type:     7,
		Fixed:    fixed,
		HasFixed: true,
	})
	if decl.Name() != name {
		t.Fatalf("Name() = %v, want %v", decl.Name(), name)
	}
	if decl.TypeID() != 7 {
		t.Fatalf("TypeID() = %d, want 7", decl.TypeID())
	}
	if got, ok := decl.FixedValue(); !ok || got != fixed {
		t.Fatalf("FixedValue() = %+v, %v; want fixed, true", got, ok)
	}

	var zero AttributeDeclRead
	if got, ok := zero.FixedValue(); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("zero FixedValue() = %+v, %v; want zero, false", got, ok)
	}
}

func newTestAttributeUseRead(shape attributeUseReadShape) AttributeUseRead {
	return newAttributeUseReadForSimpleTypes(shape, testAttributeSimpleTypes())
}

type testAttributeUseSetReadShape struct {
	Index            map[QName]uint32
	Uses             []attributeUseReadShape
	Required         []uint32
	ValueConstraints []uint32
	Wildcard         WildcardID
}

func newTestAttributeUseSetRead(shape testAttributeUseSetReadShape) AttributeUseSetRead {
	uses := make([]AttributeUseRead, len(shape.Uses))
	for i := range shape.Uses {
		uses[i] = newTestAttributeUseRead(shape.Uses[i])
	}
	read := AttributeUseSetRead{
		index:            maps.Clone(shape.Index),
		uses:             uses,
		required:         slices.Clone(shape.Required),
		valueConstraints: slices.Clone(shape.ValueConstraints),
		wildcard:         shape.Wildcard,
	}
	read.singleUse = attributeUseSetReadHasSingleUse(read)
	return read
}

func testAttributeSimpleTypes() []SimpleType {
	types := make([]SimpleType, 8)
	types[7] = SimpleType{
		ValueSpec: valuepkg.TypeSpec{
			Variety:    SimpleVarietyAtomic,
			Primitive:  PrimitiveString,
			Whitespace: WhitespacePreserve,
		},
	}
	return types
}

func TestAttributeUseSetRead(t *testing.T) {
	t.Parallel()

	firstName := QName{Local: 1}
	secondName := QName{Local: 2}
	firstUse := attributeUseReadShape{Name: firstName, Label: "first", Required: true}
	secondUse := attributeUseReadShape{Name: secondName, Label: "second"}
	index := map[QName]uint32{
		firstName:  0,
		secondName: 1,
	}
	uses := []attributeUseReadShape{firstUse, secondUse}
	required := []uint32{0}
	valueConstraints := []uint32{1}

	set := newTestAttributeUseSetRead(testAttributeUseSetReadShape{
		Index:            index,
		Uses:             uses,
		Required:         required,
		ValueConstraints: valueConstraints,
		Wildcard:         7,
	})

	index[firstName] = 99
	uses[0] = attributeUseReadShape{}
	required[0] = 99
	valueConstraints[0] = 99

	if set.UseCount() != 2 {
		t.Fatalf("UseCount() = %d, want 2", set.UseCount())
	}
	if set.Wildcard() != 7 {
		t.Fatalf("Wildcard() = %d, want 7", set.Wildcard())
	}
	got, slot, found := set.DeclaredUse(firstName)
	if !found || slot != 0 || got.Name() != firstName || !got.Required() {
		t.Fatalf("DeclaredUse(first) = %+v slot %d %v, want first slot 0", got, slot, found)
	}
	if got, slot, ok := set.DeclaredUse(QName{Local: 99}); ok || slot != -1 || got != (AttributeUseRead{}) {
		t.Fatalf("DeclaredUse(missing) = %+v slot %d %v, want zero -1 false", got, slot, ok)
	}

	requiredSlots := set.RequiredSlots()
	requiredSlot, ok := requiredSlots.At(0)
	if requiredSlots.Len() != 1 || !ok || requiredSlot != 0 {
		t.Fatalf("RequiredSlots() = len %d slot %d, %v; want 1, 0, true", requiredSlots.Len(), requiredSlot, ok)
	}
	requiredUse, ok := set.UseAt(int(requiredSlot))
	if !ok || requiredUse.Name() != firstName {
		t.Fatalf("UseAt(required) = %v, %v; want first, true", requiredUse.Name(), ok)
	}

	valueConstraintSlots := set.ValueConstraintSlots()
	valueConstraintSlot, ok := valueConstraintSlots.At(0)
	if valueConstraintSlots.Len() != 1 || !ok || valueConstraintSlot != 1 {
		t.Fatalf("ValueConstraintSlots() = len %d slot %d, %v; want 1, 1, true", valueConstraintSlots.Len(), valueConstraintSlot, ok)
	}
	valueConstraintUse, ok := set.UseAt(int(valueConstraintSlot))
	if !ok || valueConstraintUse.Name() != secondName {
		t.Fatalf("UseAt(value constraint) = %v, %v; want second, true", valueConstraintUse.Name(), ok)
	}
}

func TestAttributeUseSetReadRejectsInvalidDeclaredUseLookup(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	use := attributeUseReadShape{Name: name}
	if _, _, ok := newTestAttributeUseSetRead(testAttributeUseSetReadShape{
		Uses: []attributeUseReadShape{use},
	}).DeclaredUse(name); ok {
		t.Fatal("DeclaredUse() succeeded without index entry")
	}
	if _, _, ok := newTestAttributeUseSetRead(testAttributeUseSetReadShape{
		Index: map[QName]uint32{name: 99},
		Uses:  []attributeUseReadShape{use},
	}).DeclaredUse(name); ok {
		t.Fatal("DeclaredUse() succeeded with invalid index slot")
	}
	if _, _, ok := newTestAttributeUseSetRead(testAttributeUseSetReadShape{
		Index: map[QName]uint32{name: 0},
		Uses:  []attributeUseReadShape{{Name: NoQName()}},
	}).DeclaredUse(name); ok {
		t.Fatal("DeclaredUse() succeeded with stale index name")
	}
}

func TestAttributeUseSetReadReturnsUsesByValue(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	set := newTestAttributeUseSetRead(testAttributeUseSetReadShape{
		Index: map[QName]uint32{name: 0},
		Uses:  []attributeUseReadShape{{Name: name}},
	})

	declared, _, ok := set.DeclaredUse(name)
	if !ok {
		t.Fatal("DeclaredUse() failed")
	}
	declared.name = NoQName()
	if declared.Name() != NoQName() {
		t.Fatalf("mutated declared use name = %v; want no name", declared.Name())
	}

	stored, ok := set.UseAt(0)
	if !ok || stored.Name() != name {
		t.Fatalf("UseAt(0) = %v, %v; want %v, true", stored.Name(), ok, name)
	}
	stored.name = NoQName()
	if stored.Name() != NoQName() {
		t.Fatalf("mutated stored use name = %v; want no name", stored.Name())
	}

	again, _, ok := set.DeclaredUse(name)
	if !ok || again.Name() != name {
		t.Fatalf("DeclaredUse() after mutating returned values = %v, %v; want %v, true", again.Name(), ok, name)
	}
}

func TestValidateAttributeWildcardProvenance(t *testing.T) {
	t.Parallel()

	const (
		baseID     WildcardID = 1
		declaredID WildcardID = 2
		unionID    WildcardID = 3
	)
	base := Wildcard{Mode: WildcardAny, Process: ProcessLax}
	declared := Wildcard{Mode: WildcardTargetNamespace, Namespaces: []NamespaceID{1}, Process: ProcessStrict}
	union, err := UnionWildcard(declared, base, declared.Process)
	if err != nil {
		t.Fatalf("UnionWildcard() error = %v", err)
	}
	rt := attributeWildcardRuntimeStub{
		baseID:     base,
		declaredID: declared,
		unionID:    union,
	}

	tests := []struct {
		name    string
		wantErr string
		state   AttributeWildcardState
	}{
		{
			name:  "none matches declared wildcard",
			state: attributeWildcardState(declaredID, NoWildcard, declaredID, AttributeWildcardNone),
		},
		{
			name:    "none rejects base provenance",
			state:   attributeWildcardState(declaredID, baseID, declaredID, AttributeWildcardNone),
			wantErr: "attribute wildcard does not match declared wildcard",
		},
		{
			name:  "restriction keeps declared subset",
			state: attributeWildcardState(declaredID, baseID, declaredID, AttributeWildcardRestriction),
		},
		{
			name:    "restriction rejects non-subset",
			state:   attributeWildcardState(baseID, declaredID, baseID, AttributeWildcardRestriction),
			wantErr: "attribute wildcard restriction does not match provenance",
		},
		{
			name:    "restriction rejects missing base",
			state:   attributeWildcardState(declaredID, NoWildcard, declaredID, AttributeWildcardRestriction),
			wantErr: "attribute wildcard restriction has no base wildcard",
		},
		{
			name:    "restriction rejects undeclared wildcard",
			state:   attributeWildcardState(declaredID, NoWildcard, NoWildcard, AttributeWildcardRestriction),
			wantErr: "attribute wildcard restriction stores undeclared wildcard",
		},
		{
			name:  "extension inherits base when no declared wildcard",
			state: attributeWildcardState(baseID, baseID, NoWildcard, AttributeWildcardExtension),
		},
		{
			name:  "extension uses declared wildcard when no base wildcard",
			state: attributeWildcardState(declaredID, NoWildcard, declaredID, AttributeWildcardExtension),
		},
		{
			name:  "extension validates stored union",
			state: attributeWildcardState(unionID, baseID, declaredID, AttributeWildcardExtension),
		},
		{
			name:    "extension rejects wrong union",
			state:   attributeWildcardState(declaredID, baseID, declaredID, AttributeWildcardExtension),
			wantErr: "attribute wildcard extension does not match provenance",
		},
		{
			name:    "invalid wildcard id",
			state:   attributeWildcardState(WildcardID(99), NoWildcard, NoWildcard, AttributeWildcardNone),
			wantErr: "attribute use set references invalid wildcard",
		},
		{
			name:    "invalid base id",
			state:   attributeWildcardState(NoWildcard, WildcardID(99), NoWildcard, AttributeWildcardNone),
			wantErr: "attribute use set references invalid base wildcard",
		},
		{
			name:    "invalid declared id",
			state:   attributeWildcardState(NoWildcard, NoWildcard, WildcardID(99), AttributeWildcardNone),
			wantErr: "attribute use set references invalid declared wildcard",
		},
		{
			name:    "invalid derivation",
			state:   attributeWildcardState(NoWildcard, NoWildcard, NoWildcard, AttributeWildcardDerivation(99)),
			wantErr: "attribute use set has invalid wildcard derivation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributeWildcardProvenance(rt, tt.state)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeWildcardProvenance() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributeWildcardProvenance() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAttributeWildcardDerivation(t *testing.T) {
	t.Parallel()

	const baseID WildcardID = 1
	rt := attributeWildcardRuntimeStub{
		baseID: {Mode: WildcardLocal, Process: ProcessStrict},
	}
	base := attributeWildcardState(baseID, NoWildcard, NoWildcard, AttributeWildcardNone)
	derived := attributeWildcardState(baseID, baseID, NoWildcard, AttributeWildcardExtension)
	if err := ValidateAttributeWildcardDerivation(rt, base, derived, AttributeWildcardExtension); err != nil {
		t.Fatalf("ValidateAttributeWildcardDerivation() error = %v", err)
	}
	if err := ValidateAttributeWildcardDerivation(rt, base, derived, AttributeWildcardRestriction); err == nil ||
		!strings.Contains(err.Error(), "attribute wildcard derivation does not match owning type") {
		t.Fatalf("ValidateAttributeWildcardDerivation() wrong derivation error = %v", err)
	}
	derived.Base = NoWildcard
	if err := ValidateAttributeWildcardDerivation(rt, base, derived, AttributeWildcardExtension); err == nil ||
		!strings.Contains(err.Error(), "attribute wildcard base does not match owning type") {
		t.Fatalf("ValidateAttributeWildcardDerivation() wrong base error = %v", err)
	}
}

func TestValidateAttributeUseSetRecord(t *testing.T) {
	t.Parallel()

	names, first, second := attributeUseNameTable(t)
	tests := []struct {
		name    string
		mutate  func(*AttributeUseSet)
		wantErr string
	}{
		{
			name: "valid",
		},
		{
			name: "index size drift",
			mutate: func(set *AttributeUseSet) {
				set.Index[QName{Namespace: EmptyNamespaceID, Local: 99}] = 2
			},
			wantErr: "attribute use set index size does not match uses",
		},
		{
			name: "invalid name",
			mutate: func(set *AttributeUseSet) {
				set.Uses[0].Name = QName{Namespace: 99, Local: 99}
			},
			wantErr: "attribute use references invalid name or type",
		},
		{
			name: "invalid type",
			mutate: func(set *AttributeUseSet) {
				set.Uses[0].Type = 99
			},
			wantErr: "attribute use references invalid name or type",
		},
		{
			name: "index slot drift",
			mutate: func(set *AttributeUseSet) {
				set.Index[first] = 1
			},
			wantErr: "attribute use index does not match use slice",
		},
		{
			name: "prohibited use",
			mutate: func(set *AttributeUseSet) {
				set.Uses[0].Prohibited = true
			},
			wantErr: "attribute use set stores prohibited use",
		},
		{
			name: "default and fixed",
			mutate: func(set *AttributeUseSet) {
				set.Uses[0].Default = &ValueConstraint{}
				set.Uses[0].Fixed = &ValueConstraint{}
			},
			wantErr: "attribute use stores both default and fixed value constraints",
		},
		{
			name: "declaration provenance without fixed value",
			mutate: func(set *AttributeUseSet) {
				set.Uses[0].FixedFromDeclaration = true
			},
			wantErr: "attribute use marks absent fixed value as declaration-owned",
		},
		{
			name: "ID value constraint",
			mutate: func(set *AttributeUseSet) {
				set.Uses[0].Type = 1
				set.Uses[0].Default = &ValueConstraint{}
			},
			wantErr: "ID-typed attribute use stores value constraint",
		},
		{
			name: "multiple ID attributes",
			mutate: func(set *AttributeUseSet) {
				set.Uses[0].Type = 1
				set.Uses[1].Type = 1
				set.Uses[1].Default = nil
				set.ValueConstraints = nil
			},
			wantErr: "attribute use set stores multiple ID attributes",
		},
		{
			name: "required slots drift",
			mutate: func(set *AttributeUseSet) {
				set.Required = nil
			},
			wantErr: "attribute use set required slots do not match uses",
		},
		{
			name: "value constraint slots drift",
			mutate: func(set *AttributeUseSet) {
				set.ValueConstraints = nil
			},
			wantErr: "attribute use set value constraint slots do not match uses",
		},
		{
			name: "required slot invalid",
			mutate: func(set *AttributeUseSet) {
				set.Required = []uint32{1}
			},
			wantErr: "attribute use set required slots do not match uses",
		},
		{
			name: "value constraint slot invalid",
			mutate: func(set *AttributeUseSet) {
				set.ValueConstraints = []uint32{0}
			},
			wantErr: "attribute use set value constraint slots do not match uses",
		},
	}
	rt := attributeUseSetRuntimeStub{
		identities: map[SimpleTypeID]SimpleIdentityKind{
			0: SimpleIdentityNone,
			1: SimpleIdentityID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			set := validAttributeUseSet(first, second)
			if tt.mutate != nil {
				tt.mutate(&set)
			}
			err := ValidateAttributeUseSetRecord(&names, rt, set)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeUseSetRecord() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributeUseSetRecord() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAttributeUseRestriction(t *testing.T) {
	t.Parallel()

	rt := derivationRuntimeStub{
		simple: []SimpleTypeDerivation{
			{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
			{Base: 0, Variety: SimpleVarietyAtomic},
			{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
		},
		complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}},
	}
	base := AttributeUseRestrictionValidation{Type: 0}
	derived := AttributeUseRestrictionValidation{Type: 1}
	tests := []struct {
		name    string
		wantErr string
		base    AttributeUseRestrictionValidation
		derived AttributeUseRestrictionValidation
	}{
		{
			name:    "derived type restricts base type",
			base:    base,
			derived: derived,
		},
		{
			name: "optional base can be prohibited",
			base: base,
			derived: AttributeUseRestrictionValidation{
				Type:       1,
				Prohibited: true,
			},
		},
		{
			name: "required base cannot be prohibited",
			base: AttributeUseRestrictionValidation{
				Type:     0,
				Required: true,
			},
			derived: AttributeUseRestrictionValidation{
				Type:       1,
				Prohibited: true,
			},
			wantErr: "required attribute cannot be prohibited by restriction",
		},
		{
			name: "required base cannot become optional",
			base: AttributeUseRestrictionValidation{
				Type:     0,
				Required: true,
			},
			derived: derived,
			wantErr: "required attribute cannot become optional by restriction",
		},
		{
			name: "derived type must derive from base type",
			base: base,
			derived: AttributeUseRestrictionValidation{
				Type: 2,
			},
			wantErr: "restricted attribute type is not derived from base",
		},
		{
			name: "fixed value preserved",
			base: AttributeUseRestrictionValidation{
				Type:  0,
				Fixed: valueConstraintIdentity(t, "decimal", "5.0"),
			},
			derived: AttributeUseRestrictionValidation{
				Type:  1,
				Fixed: valueConstraintIdentity(t, "decimal", "5"),
			},
		},
		{
			name: "fixed missing",
			base: AttributeUseRestrictionValidation{
				Type:  0,
				Fixed: valueConstraintIdentity(t, "string", "fixed"),
			},
			derived: derived,
			wantErr: "fixed attribute constraint must be preserved by restriction",
		},
		{
			name: "fixed value mismatch",
			base: AttributeUseRestrictionValidation{
				Type:  0,
				Fixed: valueConstraintIdentity(t, "string", "true"),
			},
			derived: AttributeUseRestrictionValidation{
				Type:  1,
				Fixed: valueConstraintIdentity(t, "boolean", "true"),
			},
			wantErr: "fixed attribute constraint must be preserved by restriction",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributeUseRestriction(rt, tt.base, tt.derived, unboundedTypeDerivationWork)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeUseRestriction() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributeUseRestriction() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

type attributeUseRestrictionRuntimeStub struct {
	attributeWildcardRuntimeStub
	derivationRuntimeStub
}

func TestValidateAttributeUseSetRestriction(t *testing.T) {
	t.Parallel()

	const (
		baseWildcard    WildcardID = 0
		derivedWildcard WildcardID = 1
	)
	first := QName{Namespace: 1, Local: 1}
	second := QName{Namespace: 1, Local: 2}
	third := QName{Namespace: 2, Local: 3}
	rt := attributeUseRestrictionRuntimeStub{
		simple: []SimpleTypeDerivation{
			{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
			{Base: 0, Variety: SimpleVarietyAtomic},
			{Base: NoSimpleType, Variety: SimpleVarietyAtomic},
		},
		complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}},
		attributeWildcardRuntimeStub: attributeWildcardRuntimeStub{
			baseWildcard:    {Mode: WildcardAny, Process: ProcessStrict},
			derivedWildcard: {Mode: WildcardList, Namespaces: []NamespaceID{third.Namespace}, Process: ProcessStrict},
		},
	}
	base := []AttributeUseRestrictionValidation{
		{Name: first, Type: 0, Required: true},
		{Name: second, Type: 0},
	}
	derived := []AttributeUseRestrictionValidation{
		{Name: first, Type: 1, Required: true},
		{Name: second, Type: 1},
	}
	withExtra := func(use AttributeUseRestrictionValidation) []AttributeUseRestrictionValidation {
		out := append([]AttributeUseRestrictionValidation(nil), derived...)
		return append(out, use)
	}
	baseState := AttributeWildcardState{Wildcard: baseWildcard}
	derivedState := AttributeWildcardState{
		Wildcard:   derivedWildcard,
		Base:       baseWildcard,
		Declared:   derivedWildcard,
		Derivation: AttributeWildcardRestriction,
	}
	tests := []struct {
		name         string
		wantErr      string
		derived      []AttributeUseRestrictionValidation
		baseState    AttributeWildcardState
		derivedState AttributeWildcardState
		binding      AttributeWildcardBinding
	}{
		{
			name:         "valid restriction without wildcard binding",
			derived:      derived,
			baseState:    baseState,
			derivedState: NoAttributeWildcardState(),
		},
		{
			name:         "valid restriction with wildcard binding",
			derived:      derived,
			baseState:    baseState,
			derivedState: derivedState,
			binding:      AttributeWildcardBound,
		},
		{
			name: "missing required base use",
			derived: []AttributeUseRestrictionValidation{
				{Name: second, Type: 1},
			},
			baseState:    baseState,
			derivedState: NoAttributeWildcardState(),
			wantErr:      "complex restriction omits required base attribute",
		},
		{
			name: "pairwise restriction must be valid",
			derived: []AttributeUseRestrictionValidation{
				{Name: first, Type: 2, Required: true},
				{Name: second, Type: 1},
			},
			baseState:    baseState,
			derivedState: NoAttributeWildcardState(),
			wantErr:      "complex restriction attribute use is invalid",
		},
		{
			name:         "derived-only use allowed by base wildcard",
			derived:      withExtra(AttributeUseRestrictionValidation{Name: third, Type: 1}),
			baseState:    baseState,
			derivedState: NoAttributeWildcardState(),
		},
		{
			name:         "derived-only use requires base wildcard",
			derived:      withExtra(AttributeUseRestrictionValidation{Name: third, Type: 1}),
			baseState:    NoAttributeWildcardState(),
			derivedState: NoAttributeWildcardState(),
			wantErr:      "complex restriction adds attribute outside base wildcard",
		},
		{
			name:         "implicit restriction cannot store wildcard provenance",
			derived:      derived,
			baseState:    baseState,
			derivedState: derivedState,
			wantErr:      "implicit complex type stores derived attribute wildcard provenance",
		},
		{
			name:         "explicit wildcard derivation is checked",
			derived:      derived,
			baseState:    baseState,
			derivedState: AttributeWildcardState{Wildcard: derivedWildcard, Base: baseWildcard, Declared: derivedWildcard},
			binding:      AttributeWildcardBound,
			wantErr:      "attribute wildcard derivation does not match owning type",
		},
		{
			name:         "invalid wildcard binding",
			derived:      derived,
			baseState:    baseState,
			derivedState: derivedState,
			binding:      AttributeWildcardBinding(99),
			wantErr:      "attribute wildcard binding is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributeUseSetRestriction(rt, AttributeUseRestrictionSet{Uses: base, Wildcard: tt.baseState}, AttributeUseRestrictionSet{Uses: tt.derived, Wildcard: tt.derivedState}, tt.binding, unboundedTypeDerivationWork)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeUseSetRestriction() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributeUseSetRestriction() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAttributeUseSetExtension(t *testing.T) {
	t.Parallel()

	qname, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{}).Validate(
		valuepkg.BuiltinType(valuepkg.PrimitiveQName), "p:item",
		valuepkg.Resolver{QName: func(string) (valuepkg.ExpandedName, bool) {
			return valuepkg.ExpandedName{Namespace: "urn:test", Local: "item"}, true
		}},
		valuepkg.NeedCanonical|valuepkg.NeedIdentity, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	value := ValueConstraintIdentity{
		ResolvedNames: []ResolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}},
		Lexical:       "p:item",
		Canonical:     "p:item",
		Value:         qname,
		Present:       true,
	}
	fixed := ValueConstraintIdentity{
		Lexical:   "fixed",
		Canonical: "fixed",
		Value:     builtinValueForTest(t, "string", "fixed"),
		Present:   true,
	}
	base := []AttributeUseExtensionValidation{
		{
			Default:  value,
			Name:     QName{Namespace: 1, Local: 1},
			Type:     1,
			Required: true,
		},
		{
			Fixed: fixed,
			Name:  QName{Namespace: 1, Local: 2},
			Type:  2,
		},
	}
	tests := []struct {
		name    string
		mutate  func([]AttributeUseExtensionValidation) []AttributeUseExtensionValidation
		wantErr string
	}{
		{
			name: "preserves base uses",
		},
		{
			name: "missing base use",
			mutate: func(uses []AttributeUseExtensionValidation) []AttributeUseExtensionValidation {
				return uses[:1]
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "type mismatch",
			mutate: func(uses []AttributeUseExtensionValidation) []AttributeUseExtensionValidation {
				uses[0].Type = 3
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "required mismatch",
			mutate: func(uses []AttributeUseExtensionValidation) []AttributeUseExtensionValidation {
				uses[0].Required = false
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "prohibited mismatch",
			mutate: func(uses []AttributeUseExtensionValidation) []AttributeUseExtensionValidation {
				uses[0].Prohibited = true
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default presence mismatch",
			mutate: func(uses []AttributeUseExtensionValidation) []AttributeUseExtensionValidation {
				uses[0].Default = ValueConstraintIdentity{}
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default canonical mismatch",
			mutate: func(uses []AttributeUseExtensionValidation) []AttributeUseExtensionValidation {
				uses[0].Default.Canonical = "other"
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default value mismatch",
			mutate: func(uses []AttributeUseExtensionValidation) []AttributeUseExtensionValidation {
				uses[0].Default.Value = builtinValueForTest(t, "string", "other")
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default resolved name mismatch",
			mutate: func(uses []AttributeUseExtensionValidation) []AttributeUseExtensionValidation {
				uses[0].Default.ResolvedNames[0].Local = "other"
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "fixed canonical mismatch",
			mutate: func(uses []AttributeUseExtensionValidation) []AttributeUseExtensionValidation {
				uses[1].Fixed.Canonical = "other"
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "fixed provenance mismatch",
			mutate: func(uses []AttributeUseExtensionValidation) []AttributeUseExtensionValidation {
				uses[1].FixedFromDeclaration = true
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			derived := cloneAttributeUseExtensionValidations(base)
			if tt.mutate != nil {
				derived = tt.mutate(derived)
			}
			err := ValidateAttributeUseSetExtension(base, derived)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeUseSetExtension() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributeUseSetExtension() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func cloneAttributeUseExtensionValidations(in []AttributeUseExtensionValidation) []AttributeUseExtensionValidation {
	out := append([]AttributeUseExtensionValidation(nil), in...)
	for i := range out {
		out[i].Default.ResolvedNames = append([]ResolvedValueName(nil), out[i].Default.ResolvedNames...)
		out[i].Fixed.ResolvedNames = append([]ResolvedValueName(nil), out[i].Fixed.ResolvedNames...)
	}
	return out
}

type attributeWildcardRuntimeStub map[WildcardID]Wildcard

type attributeUseSetRuntimeStub struct {
	wildcards  map[WildcardID]Wildcard
	identities map[SimpleTypeID]SimpleIdentityKind
}

func attributeWildcardState(wildcard, base, declared WildcardID, derivation AttributeWildcardDerivation) AttributeWildcardState {
	return AttributeWildcardState{
		Wildcard:   wildcard,
		Base:       base,
		Declared:   declared,
		Derivation: derivation,
	}
}

func (rt attributeWildcardRuntimeStub) Wildcard(id WildcardID) (Wildcard, bool) {
	w, ok := rt[id]
	return w, ok
}

func (rt attributeUseSetRuntimeStub) Wildcard(id WildcardID) (Wildcard, bool) {
	w, ok := rt.wildcards[id]
	return w, ok
}

func (rt attributeUseSetRuntimeStub) SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool) {
	identity, ok := rt.identities[id]
	return identity, ok
}

func attributeUseNameTable(t *testing.T) (names NameTable, first, second QName) {
	t.Helper()

	names, err := NewNameTable(8, []string{EmptyNamespaceURI}, []ExpandedName{{Local: "first"}, {Local: "second"}})
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	first, ok := names.LookupQName("", "first")
	if !ok {
		t.Fatal("missing first QName")
	}
	second, ok = names.LookupQName("", "second")
	if !ok {
		t.Fatal("missing second QName")
	}
	return names, first, second
}

func validAttributeUseSet(first, second QName) AttributeUseSet {
	return AttributeUseSet{
		Index: map[QName]uint32{
			first:  0,
			second: 1,
		},
		Uses: []AttributeUse{
			{Name: first, Type: 0, Required: true},
			{Name: second, Type: 0, Default: &ValueConstraint{}},
		},
		Required:         []uint32{0},
		ValueConstraints: []uint32{1},
		Wildcard:         NoWildcard,
		WildcardBase:     NoWildcard,
		WildcardDeclared: NoWildcard,
	}
}
