package schema

import (
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

	names, err := NewNameTable(8, []string{EmptyNamespaceURI}, nil)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	name, err := NewNameInterner(&names).InternQName("", "a")
	if err != nil {
		t.Fatal(err)
	}
	fixedValue := testQNameValue(t, "urn:fixed")
	defaultValue := testQNameValue(t, "urn:default")
	fixed := &ValueConstraint{
		Lexical: "p:item", Canonical: "{urn:fixed}item", Value: fixedValue,
		ResolvedNames: []ResolvedValueName{{Lexical: "p:item", NS: "urn:fixed", Local: "item"}},
		qnameContext:  &valueConstraintQNameContext{bindings: []valueConstraintQNameBinding{{prefix: "p", namespace: "urn:fixed", bound: true}}},
	}
	def := &ValueConstraint{
		Lexical: "p:item", Canonical: "{urn:default}item", Value: defaultValue,
		ResolvedNames: []ResolvedValueName{{Lexical: "p:item", NS: "urn:default", Local: "item"}},
		qnameContext:  &valueConstraintQNameContext{bindings: []valueConstraintQNameBinding{{prefix: "p", namespace: "urn:default", bound: true}}},
	}
	// Both pointers exercise read precedence; schema admission rejects that shape.
	use := newAttributeUseReadForSimpleTypes(&names, AttributeUse{
		Name: name, Type: 7, Fixed: fixed, Default: def, Required: true, FixedFromDeclaration: true,
	}, testAttributeSimpleTypes())
	defaulted := newAttributeUseReadForSimpleTypes(&names, AttributeUse{Name: name, Type: 7, Default: def}, testAttributeSimpleTypes())
	fixed.qnameContext.bindings[0].namespace = "urn:changed"
	def.qnameContext.bindings[0].bound = false
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
	if use.CanValidateFixedStringFast() {
		t.Fatal("CanValidateFixedStringFast() = true for QName, want false")
	}
	for range 2 {
		fixedRead, hasFixed := use.FixedValue()
		if !hasFixed || fixedRead.ApplicationText() != "p:item" || fixedRead.CanonicalText() != "{urn:fixed}item" || fixedRead.Value().CanonicalText() != "{urn:fixed}item" || !fixedRead.Value().Equal(fixedValue) {
			t.Fatalf("FixedValue() = %+v, %v; want p:item, {urn:fixed}item, typed QName, true", fixedRead, hasFixed)
		}
		if resolved, ok := fixedRead.ResolveQName("p:item"); !ok || resolved != (valuepkg.ExpandedName{Namespace: "urn:fixed", Local: "item"}) {
			t.Fatalf("fixed QName = %v, %v; want {urn:fixed}item, true", resolved, ok)
		}
		if got, ok := use.AbsentValueConstraint(); !ok || got != fixedRead {
			t.Fatalf("AbsentValueConstraint() = %+v, %v; want fixed, true", got, ok)
		}
		defaultRead, hasDefault := defaulted.AbsentValueConstraint()
		if !hasDefault || defaultRead.ApplicationText() != "p:item" || defaultRead.CanonicalText() != "{urn:default}item" || defaultRead.Value().CanonicalText() != "{urn:default}item" || !defaultRead.Value().Equal(defaultValue) {
			t.Fatalf("AbsentValueConstraint(defaulted) = %+v, %v; want p:item, {urn:default}item, typed QName, true", defaultRead, hasDefault)
		}
		if resolved, ok := defaultRead.ResolveQName("p:item"); !ok || resolved != (valuepkg.ExpandedName{Namespace: "urn:default", Local: "item"}) {
			t.Fatalf("default QName = %v, %v; want {urn:default}item, true", resolved, ok)
		}
	}
	stringUse := AttributeUse{Name: name, Type: 8, Fixed: &ValueConstraint{
		Lexical: "fixed", Canonical: "fixed", Value: builtinValueForTest(t, "string", "fixed"),
	}}
	if !newAttributeUseReadForSimpleTypes(&names, stringUse, testAttributeSimpleTypes()).CanValidateFixedStringFast() {
		t.Fatal("CanValidateFixedStringFast() = false, want true")
	}
	if newAttributeUseReadForSimpleTypes(&names, stringUse, nil).CanValidateFixedStringFast() {
		t.Fatal("CanValidateFixedStringFast() = true without simple-value read, want false")
	}
	absent := newAttributeUseReadForSimpleTypes(&names, AttributeUse{Name: name, Type: 8}, testAttributeSimpleTypes())
	if absent.CanValidateFixedStringFast() {
		t.Fatal("CanValidateFixedStringFast() = true without fixed value, want false")
	}
	if got, ok := absent.FixedValue(); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("absent FixedValue() = %+v, %v; want zero, false", got, ok)
	}
	if got, ok := absent.AbsentValueConstraint(); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("absent AbsentValueConstraint() = %+v, %v; want zero, false", got, ok)
	}
	empty := newAttributeUseReadForSimpleTypes(&names, AttributeUse{Fixed: &ValueConstraint{}}, nil)
	if got, ok := empty.FixedValue(); !ok || got != (ValueConstraintRead{}) {
		t.Fatalf("present-empty FixedValue() = %+v, %v; want zero, true", got, ok)
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

	names, err := NewNameTable(8, []string{EmptyNamespaceURI}, nil)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	name, err := NewNameInterner(&names).InternQName("", "a")
	if err != nil {
		t.Fatal(err)
	}
	value := testQNameValue(t, "urn:fixed")
	fixed := &ValueConstraint{
		Lexical: "p:item", Canonical: "{urn:fixed}item", Value: value,
		ResolvedNames: []ResolvedValueName{{Lexical: "p:item", NS: "urn:fixed", Local: "item"}},
		qnameContext:  &valueConstraintQNameContext{bindings: []valueConstraintQNameBinding{{prefix: "p", namespace: "urn:fixed", bound: true}}},
	}
	decl := newAttributeDeclRead(AttributeDecl{Name: name, Type: 7, Fixed: fixed})
	defaultOnly := newAttributeDeclRead(AttributeDecl{Name: name, Type: 7, Default: fixed})
	fixed.qnameContext.bindings[0].namespace = "urn:changed"
	if decl.Name() != name {
		t.Fatalf("Name() = %v, want %v", decl.Name(), name)
	}
	if decl.TypeID() != 7 {
		t.Fatalf("TypeID() = %d, want 7", decl.TypeID())
	}
	for range 2 {
		got, present := decl.FixedValue()
		if !present || got.ApplicationText() != "p:item" || got.CanonicalText() != "{urn:fixed}item" || got.Value().CanonicalText() != "{urn:fixed}item" || !got.Value().Equal(value) {
			t.Fatalf("FixedValue() = %+v, %v; want p:item, {urn:fixed}item, typed QName, true", got, present)
		}
		if resolved, ok := got.ResolveQName("p:item"); !ok || resolved != (valuepkg.ExpandedName{Namespace: "urn:fixed", Local: "item"}) {
			t.Fatalf("declaration QName = %v, %v; want {urn:fixed}item, true", resolved, ok)
		}
	}
	if got, ok := defaultOnly.FixedValue(); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("default declaration FixedValue() = %+v, %v; want zero, false", got, ok)
	}
	empty := newAttributeDeclRead(AttributeDecl{Fixed: &ValueConstraint{}})
	if got, ok := empty.FixedValue(); !ok || got != (ValueConstraintRead{}) {
		t.Fatalf("present-empty declaration FixedValue() = %+v, %v; want zero, true", got, ok)
	}

	var zero AttributeDeclRead
	if got, ok := zero.FixedValue(); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("zero FixedValue() = %+v, %v; want zero, false", got, ok)
	}
}

func testAttributeSimpleTypes() []SimpleType {
	types := make([]SimpleType, 9)
	types[7] = SimpleType{ValueSpec: valuepkg.TypeSpec{
		Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveQName, Whitespace: valuepkg.WhitespaceCollapse,
	}}
	types[8] = SimpleType{ValueSpec: valuepkg.TypeSpec{
		Variety: valuepkg.Atomic, Primitive: valuepkg.PrimitiveString, Whitespace: valuepkg.WhitespacePreserve,
	}}
	return types
}

func TestAttributeUseSetRead(t *testing.T) {
	t.Parallel()

	names, firstName, secondName := attributeUseNameTable(t)
	firstUse := AttributeUse{Name: firstName, Required: true}
	secondUse := AttributeUse{Name: secondName, Default: &ValueConstraint{}}
	index := map[QName]uint32{
		firstName:  0,
		secondName: 1,
	}
	uses := []AttributeUse{firstUse, secondUse}

	set := newAttributeUseSetReads(&names, []AttributeUseSet{{
		Index:    index,
		Uses:     uses,
		Wildcard: 7,
	}}, testAttributeSimpleTypes())[0]

	index[firstName] = 99
	uses[0] = AttributeUse{}
	uses[1].Default = nil

	if set.UseCount() != 2 {
		t.Fatalf("UseCount() = %d, want 2", set.UseCount())
	}
	if set.Wildcard() != 7 {
		t.Fatalf("Wildcard() = %d, want 7", set.Wildcard())
	}
	if first, ok := set.UseAt(0); !ok || first.Label() != "first" {
		t.Fatalf("UseAt(0).Label() = %q, %v; want first, true", first.Label(), ok)
	}
	if second, ok := set.UseAt(1); !ok || second.Label() != "second" {
		t.Fatalf("UseAt(1).Label() = %q, %v; want second, true", second.Label(), ok)
	}
	for _, slot := range []int{-1, 2} {
		if got, ok := set.UseAt(slot); ok || got != (AttributeUseRead{}) {
			t.Fatalf("UseAt(%d) = %+v, %v; want zero, false", slot, got, ok)
		}
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
	if got, ok := valueConstraintUse.AbsentValueConstraint(); !ok || got != (ValueConstraintRead{}) {
		t.Fatalf("present-empty default = %+v, %v; want zero, true", got, ok)
	}
	for _, slot := range []int{-1, 1} {
		if got, ok := requiredSlots.At(slot); ok || got != 0 {
			t.Fatalf("RequiredSlots().At(%d) = %d, %v; want zero, false", slot, got, ok)
		}
		if got, ok := valueConstraintSlots.At(slot); ok || got != 0 {
			t.Fatalf("ValueConstraintSlots().At(%d) = %d, %v; want zero, false", slot, got, ok)
		}
	}
}

func TestAttributeUseSetReadDerivesOrderedSlots(t *testing.T) {
	t.Parallel()
	names, first, second := attributeUseNameTable(t)
	interner := NewNameInterner(&names)
	third, err := interner.InternQName("", "third")
	if err != nil {
		t.Fatal(err)
	}
	fourth, err := interner.InternQName("", "fourth")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name                  string
		uses                  []AttributeUse
		required, constraints []uint32
	}{
		{name: "absent"},
		{name: "optional", uses: []AttributeUse{{Name: first}}},
		{name: "required only", uses: []AttributeUse{{Name: first, Required: true}}, required: []uint32{0}},
		{name: "empty default only", uses: []AttributeUse{{Name: first, Default: &ValueConstraint{}}}, constraints: []uint32{0}},
		{name: "mixed", uses: []AttributeUse{
			{Name: first},
			{Name: second, Required: true},
			{Name: third, Default: &ValueConstraint{}},
			{Name: fourth, Required: true, Fixed: &ValueConstraint{Canonical: "fixed"}},
		}, required: []uint32{1, 3}, constraints: []uint32{2, 3}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			read := newAttributeUseSetReads(&names, []AttributeUseSet{{Uses: test.uses}}, testAttributeSimpleTypes())[0]
			for _, slots := range []struct {
				name string
				got  AttributeUseSlots
				want []uint32
			}{{"required", read.RequiredSlots(), test.required}, {"constraints", read.ValueConstraintSlots(), test.constraints}} {
				if slots.got.Len() != len(slots.want) {
					t.Fatalf("%s length = %d, want %d", slots.name, slots.got.Len(), len(slots.want))
				}
				for i, want := range slots.want {
					if got, ok := slots.got.At(i); !ok || got != want {
						t.Fatalf("%s slot %d = %d, %v; want %d", slots.name, i, got, ok, want)
					}
				}
			}
		})
	}
}

func TestAttributeUseSetReadRejectsInvalidDeclaredUseLookup(t *testing.T) {
	t.Parallel()

	names, name, _ := attributeUseNameTable(t)
	use := AttributeUse{Name: name}
	if got, slot, ok := newAttributeUseSetReads(&names, []AttributeUseSet{{
		Uses: []AttributeUse{use},
	}}, testAttributeSimpleTypes())[0].DeclaredUse(name); ok || slot != -1 || got != (AttributeUseRead{}) {
		t.Fatal("DeclaredUse() succeeded without index entry")
	}
	if got, slot, ok := newAttributeUseSetReads(&names, []AttributeUseSet{{
		Index: map[QName]uint32{name: 99},
		Uses:  []AttributeUse{use},
	}}, testAttributeSimpleTypes())[0].DeclaredUse(name); ok || slot != -1 || got != (AttributeUseRead{}) {
		t.Fatal("DeclaredUse() succeeded with invalid index slot")
	}
	if got, slot, ok := newAttributeUseSetReads(&names, []AttributeUseSet{{
		Index: map[QName]uint32{name: 0},
		Uses:  []AttributeUse{{Name: NoQName()}},
	}}, testAttributeSimpleTypes())[0].DeclaredUse(name); ok || slot != -1 || got != (AttributeUseRead{}) {
		t.Fatal("DeclaredUse() succeeded with stale index name")
	}
}

func TestAttributeUseSetReadReturnsUsesByValue(t *testing.T) {
	t.Parallel()

	names, name, _ := attributeUseNameTable(t)
	set := newAttributeUseSetReads(&names, []AttributeUseSet{{
		Index: map[QName]uint32{name: 0},
		Uses:  []AttributeUse{{Name: name}},
	}}, testAttributeSimpleTypes())[0]

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
	rt := attributeWildcardFixture{
		baseID:     base,
		declaredID: declared,
		unionID:    union,
	}

	tests := []struct {
		name    string
		wantErr string
		state   AttributeUseSet
	}{
		{
			name:  "none matches declared wildcard",
			state: attributeWildcardUseSet(declaredID, NoWildcard, declaredID, AttributeWildcardNone),
		},
		{
			name:    "none rejects base provenance",
			state:   attributeWildcardUseSet(declaredID, baseID, declaredID, AttributeWildcardNone),
			wantErr: "attribute wildcard does not match declared wildcard",
		},
		{
			name:  "restriction keeps declared subset",
			state: attributeWildcardUseSet(declaredID, baseID, declaredID, AttributeWildcardRestriction),
		},
		{
			name:    "restriction rejects non-subset",
			state:   attributeWildcardUseSet(baseID, declaredID, baseID, AttributeWildcardRestriction),
			wantErr: "attribute wildcard restriction does not match provenance",
		},
		{
			name:    "restriction rejects missing base",
			state:   attributeWildcardUseSet(declaredID, NoWildcard, declaredID, AttributeWildcardRestriction),
			wantErr: "attribute wildcard restriction has no base wildcard",
		},
		{
			name:    "restriction rejects undeclared wildcard",
			state:   attributeWildcardUseSet(declaredID, NoWildcard, NoWildcard, AttributeWildcardRestriction),
			wantErr: "attribute wildcard restriction stores undeclared wildcard",
		},
		{
			name:  "extension inherits base when no declared wildcard",
			state: attributeWildcardUseSet(baseID, baseID, NoWildcard, AttributeWildcardExtension),
		},
		{
			name:  "extension uses declared wildcard when no base wildcard",
			state: attributeWildcardUseSet(declaredID, NoWildcard, declaredID, AttributeWildcardExtension),
		},
		{
			name:  "extension validates stored union",
			state: attributeWildcardUseSet(unionID, baseID, declaredID, AttributeWildcardExtension),
		},
		{
			name:    "extension rejects wrong union",
			state:   attributeWildcardUseSet(declaredID, baseID, declaredID, AttributeWildcardExtension),
			wantErr: "attribute wildcard extension does not match provenance",
		},
		{
			name:    "invalid wildcard id",
			state:   attributeWildcardUseSet(WildcardID(99), NoWildcard, NoWildcard, AttributeWildcardNone),
			wantErr: "attribute use set references invalid wildcard",
		},
		{
			name:    "invalid base id",
			state:   attributeWildcardUseSet(NoWildcard, WildcardID(99), NoWildcard, AttributeWildcardNone),
			wantErr: "attribute use set references invalid base wildcard",
		},
		{
			name:    "invalid declared id",
			state:   attributeWildcardUseSet(NoWildcard, NoWildcard, WildcardID(99), AttributeWildcardNone),
			wantErr: "attribute use set references invalid declared wildcard",
		},
		{
			name:    "invalid derivation",
			state:   attributeWildcardUseSet(NoWildcard, NoWildcard, NoWildcard, AttributeWildcardDerivation(99)),
			wantErr: "attribute use set has invalid wildcard derivation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributeWildcardProvenance(rt.build(), tt.state)
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
	rt := attributeWildcardFixture{
		baseID: {Mode: WildcardLocal, Process: ProcessStrict},
	}
	base := attributeWildcardUseSet(baseID, NoWildcard, NoWildcard, AttributeWildcardNone)
	derived := attributeWildcardUseSet(baseID, baseID, NoWildcard, AttributeWildcardExtension)
	if err := ValidateAttributeWildcardDerivation(rt.build(), base, derived, AttributeWildcardExtension); err != nil {
		t.Fatalf("ValidateAttributeWildcardDerivation() error = %v", err)
	}
	if err := ValidateAttributeWildcardDerivation(rt.build(), base, derived, AttributeWildcardRestriction); err == nil ||
		!strings.Contains(err.Error(), "attribute wildcard derivation does not match owning type") {
		t.Fatalf("ValidateAttributeWildcardDerivation() wrong derivation error = %v", err)
	}
	derived.WildcardBase = NoWildcard
	if err := ValidateAttributeWildcardDerivation(rt.build(), base, derived, AttributeWildcardExtension); err == nil ||
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
			},
			wantErr: "attribute use set stores multiple ID attributes",
		},
	}
	rt := attributeUseSetFixture{
		identities: map[SimpleTypeID]valuepkg.IdentityKind{
			0: valuepkg.IdentityNone,
			1: valuepkg.IdentityID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			set := validAttributeUseSet(first, second)
			if tt.mutate != nil {
				tt.mutate(&set)
			}
			err := ValidateAttributeUseSetRecord(&names, rt.build(), set)
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

	rt := derivationFixture{
		simple: []SimpleTypeDerivation{
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
			{Base: 0, Variety: valuepkg.Atomic},
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
		},
		complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}},
	}
	base := AttributeUse{Type: 0}
	derived := AttributeUse{Type: 1}
	tests := []struct {
		name    string
		wantErr string
		base    AttributeUse
		derived AttributeUse
	}{
		{
			name:    "derived type restricts base type",
			base:    base,
			derived: derived,
		},
		{
			name: "optional base can be prohibited",
			base: base,
			derived: AttributeUse{
				Type:       1,
				Prohibited: true,
			},
		},
		{
			name: "required base cannot be prohibited",
			base: AttributeUse{
				Type:     0,
				Required: true,
			},
			derived: AttributeUse{
				Type:       1,
				Prohibited: true,
			},
			wantErr: "required attribute cannot be prohibited by restriction",
		},
		{
			name: "required base cannot become optional",
			base: AttributeUse{
				Type:     0,
				Required: true,
			},
			derived: derived,
			wantErr: "required attribute cannot become optional by restriction",
		},
		{
			name: "derived type must derive from base type",
			base: base,
			derived: AttributeUse{
				Type: 2,
			},
			wantErr: "restricted attribute type is not derived from base",
		},
		{
			name: "fixed value preserved",
			base: AttributeUse{
				Type:  0,
				Fixed: attributeUseConstraintForTest(t, "decimal", "5.0"),
			},
			derived: AttributeUse{
				Type:  1,
				Fixed: attributeUseConstraintForTest(t, "decimal", "5"),
			},
		},
		{
			name: "fixed missing",
			base: AttributeUse{
				Type:  0,
				Fixed: attributeUseConstraintForTest(t, "string", "fixed"),
			},
			derived: derived,
			wantErr: "fixed attribute constraint must be preserved by restriction",
		},
		{
			name: "fixed value mismatch",
			base: AttributeUse{
				Type:  0,
				Fixed: attributeUseConstraintForTest(t, "string", "true"),
			},
			derived: AttributeUse{
				Type:  1,
				Fixed: attributeUseConstraintForTest(t, "boolean", "true"),
			},
			wantErr: "fixed attribute constraint must be preserved by restriction",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributeUseRestriction(rt.build(), tt.base, tt.derived, unboundedTypeDerivationWork)
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

type attributeUseRestrictionFixture struct {
	attributeWildcardFixture
	derivationFixture
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
	rt := attributeUseRestrictionFixture{
		simple: []SimpleTypeDerivation{
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
			{Base: 0, Variety: valuepkg.Atomic},
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
		},
		complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}},
		attributeWildcardFixture: attributeWildcardFixture{
			baseWildcard:    {Mode: WildcardAny, Process: ProcessStrict},
			derivedWildcard: {Mode: WildcardList, Namespaces: []NamespaceID{third.Namespace}, Process: ProcessStrict},
		},
	}
	base := []AttributeUse{
		{Name: first, Type: 0, Required: true},
		{Name: second, Type: 0},
	}
	derived := []AttributeUse{
		{Name: first, Type: 1, Required: true},
		{Name: second, Type: 1},
	}
	withExtra := func(use AttributeUse) []AttributeUse {
		out := append([]AttributeUse(nil), derived...)
		return append(out, use)
	}
	baseState := AttributeUseSet{Wildcard: baseWildcard}
	derivedState := AttributeUseSet{
		Wildcard:         derivedWildcard,
		WildcardBase:     baseWildcard,
		WildcardDeclared: derivedWildcard,
		WildcardDerive:   AttributeWildcardRestriction,
	}
	tests := []struct {
		name         string
		wantErr      string
		derived      []AttributeUse
		baseState    AttributeUseSet
		derivedState AttributeUseSet
		binding      AttributeWildcardBinding
	}{
		{
			name:         "valid restriction without wildcard binding",
			derived:      derived,
			baseState:    baseState,
			derivedState: AttributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
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
			derived: []AttributeUse{
				{Name: second, Type: 1},
			},
			baseState:    baseState,
			derivedState: AttributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
			wantErr:      "complex restriction omits required base attribute",
		},
		{
			name: "pairwise restriction must be valid",
			derived: []AttributeUse{
				{Name: first, Type: 2, Required: true},
				{Name: second, Type: 1},
			},
			baseState:    baseState,
			derivedState: AttributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
			wantErr:      "complex restriction attribute use is invalid",
		},
		{
			name:         "derived-only use allowed by base wildcard",
			derived:      withExtra(AttributeUse{Name: third, Type: 1}),
			baseState:    baseState,
			derivedState: AttributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
		},
		{
			name:         "derived-only use requires base wildcard",
			derived:      withExtra(AttributeUse{Name: third, Type: 1}),
			baseState:    AttributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
			derivedState: AttributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
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
			derivedState: AttributeUseSet{Wildcard: derivedWildcard, WildcardBase: baseWildcard, WildcardDeclared: derivedWildcard},
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

			baseSet := tt.baseState
			baseSet.Uses = base
			derivedSet := tt.derivedState
			derivedSet.Uses = tt.derived
			err := ValidateAttributeUseSetRestriction(rt.build(), baseSet, derivedSet, tt.binding, unboundedTypeDerivationWork)
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
	value := &ValueConstraint{
		ResolvedNames: []ResolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}},
		Lexical:       "p:item",
		Canonical:     "p:item",
		Value:         qname,
	}
	fixed := &ValueConstraint{
		Lexical:   "fixed",
		Canonical: "fixed",
		Value:     builtinValueForTest(t, "string", "fixed"),
	}
	base := []AttributeUse{
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
		mutate  func([]AttributeUse) []AttributeUse
		wantErr string
	}{
		{
			name: "preserves base uses",
		},
		{
			name: "missing base use",
			mutate: func(uses []AttributeUse) []AttributeUse {
				return uses[:1]
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "type mismatch",
			mutate: func(uses []AttributeUse) []AttributeUse {
				uses[0].Type = 3
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "required mismatch",
			mutate: func(uses []AttributeUse) []AttributeUse {
				uses[0].Required = false
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "prohibited mismatch",
			mutate: func(uses []AttributeUse) []AttributeUse {
				uses[0].Prohibited = true
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default presence mismatch",
			mutate: func(uses []AttributeUse) []AttributeUse {
				uses[0].Default = nil
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default canonical mismatch",
			mutate: func(uses []AttributeUse) []AttributeUse {
				uses[0].Default.Canonical = "other"
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default value mismatch",
			mutate: func(uses []AttributeUse) []AttributeUse {
				uses[0].Default.Value = builtinValueForTest(t, "string", "other")
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default resolved name mismatch",
			mutate: func(uses []AttributeUse) []AttributeUse {
				uses[0].Default.ResolvedNames[0].Local = "other"
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "fixed canonical mismatch",
			mutate: func(uses []AttributeUse) []AttributeUse {
				uses[1].Fixed.Canonical = "other"
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "fixed provenance mismatch",
			mutate: func(uses []AttributeUse) []AttributeUse {
				uses[1].FixedFromDeclaration = true
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			derived := cloneAttributeUsesForTest(base)
			if tt.mutate != nil {
				derived = tt.mutate(derived)
			}
			err := ValidateAttributeUseSetExtension(AttributeUseSet{Uses: base}, AttributeUseSet{Uses: derived})
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

func cloneAttributeUsesForTest(in []AttributeUse) []AttributeUse {
	out := append([]AttributeUse(nil), in...)
	for i := range out {
		if out[i].Default != nil {
			cloned := *out[i].Default
			cloned.ResolvedNames = append([]ResolvedValueName(nil), cloned.ResolvedNames...)
			out[i].Default = &cloned
		}
		if out[i].Fixed != nil {
			cloned := *out[i].Fixed
			cloned.ResolvedNames = append([]ResolvedValueName(nil), cloned.ResolvedNames...)
			out[i].Fixed = &cloned
		}
	}
	return out
}

func attributeUseConstraintForTest(tb testing.TB, typeName, lexical string) *ValueConstraint {
	tb.Helper()
	validated := builtinValueForTest(tb, typeName, lexical)
	return &ValueConstraint{Lexical: lexical, Canonical: validated.CanonicalText(), Value: validated}
}

type attributeWildcardFixture map[WildcardID]Wildcard

type attributeUseSetFixture struct {
	wildcards  map[WildcardID]Wildcard
	identities map[SimpleTypeID]valuepkg.IdentityKind
}

func attributeWildcardUseSet(wildcard, base, declared WildcardID, derivation AttributeWildcardDerivation) AttributeUseSet {
	return AttributeUseSet{
		Wildcard:         wildcard,
		WildcardBase:     base,
		WildcardDeclared: declared,
		WildcardDerive:   derivation,
	}
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
		Wildcard:         NoWildcard,
		WildcardBase:     NoWildcard,
		WildcardDeclared: NoWildcard,
	}
}

func (f attributeWildcardFixture) build() *schemaBuild {
	return &schemaBuild{Wildcards: fixtureMapSlice(f)}
}

func (f attributeUseSetFixture) build() *schemaBuild {
	b := &schemaBuild{Wildcards: fixtureMapSlice(f.wildcards), SimpleTypes: make([]SimpleType, len(fixtureMapSlice(f.identities)))}
	for id, identity := range f.identities {
		b.SimpleTypes[id].ValueSpec.Identity = identity
	}
	return b
}

func (f attributeUseRestrictionFixture) build() *schemaBuild {
	b := f.derivationFixture.build()
	b.Wildcards = fixtureMapSlice(f.attributeWildcardFixture)
	return b
}
