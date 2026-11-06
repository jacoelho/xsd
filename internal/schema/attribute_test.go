package schema

import (
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestAttributeWildcardDerivationValidity(t *testing.T) {
	t.Parallel()

	for _, kind := range []attributeWildcardDerivation{
		attributeWildcardNone,
		attributeWildcardRestriction,
		attributeWildcardExtension,
	} {
		if !validAttributeWildcardDerivation(kind) {
			t.Fatalf("ValidAttributeWildcardDerivation(%d) = false", kind)
		}
	}
	if validAttributeWildcardDerivation(attributeWildcardDerivation(99)) {
		t.Fatal("invalid attribute wildcard derivation was accepted")
	}
}

func TestAttributeUseRead(t *testing.T) {
	t.Parallel()

	names, err := newNameTable(8, []string{EmptyNamespaceURI}, nil)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	name, err := newNameInterner(&names).InternQName("", "a")
	if err != nil {
		t.Fatal(err)
	}
	fixedValue := testQNameValue(t, "urn:fixed")
	defaultValue := testQNameValue(t, "urn:default")
	fixed := &valueConstraint{
		Lexical: "p:item", Canonical: "{urn:fixed}item", Value: fixedValue,
		ResolvedNames: []resolvedValueName{{Lexical: "p:item", NS: "urn:fixed", Local: "item"}},
		qnameContext:  &valueConstraintQNameContext{bindings: []valueConstraintQNameBinding{{prefix: "p", namespace: "urn:fixed", bound: true}}},
	}
	def := &valueConstraint{
		Lexical: "p:item", Canonical: "{urn:default}item", Value: defaultValue,
		ResolvedNames: []resolvedValueName{{Lexical: "p:item", NS: "urn:default", Local: "item"}},
		qnameContext:  &valueConstraintQNameContext{bindings: []valueConstraintQNameBinding{{prefix: "p", namespace: "urn:default", bound: true}}},
	}
	// Both pointers exercise read precedence; schema admission rejects that shape.
	use := newAttributeUseRead(&names, attributeUse{
		Name: name, Type: 7, Fixed: fixed, Default: def, Required: true,
	})
	defaulted := newAttributeUseRead(&names, attributeUse{Name: name, Type: 7, Default: def})
	fixed.qnameContext.bindings[0].namespace = "urn:changed"
	def.qnameContext.bindings[0].bound = false
	if use.Name() != name {
		t.Fatalf("Name() = %v, want %v", use.Name(), name)
	}
	if use.TypeID() != 7 {
		t.Fatalf("TypeID() = %d, want 7", use.TypeID())
	}
	if use.Label() != `"a"` {
		t.Fatalf("Label() = %q, want a", use.Label())
	}
	if !use.Required() {
		t.Fatal("Required() = false, want true")
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
	absent := newAttributeUseRead(&names, attributeUse{Name: name, Type: 8})
	if got, ok := absent.FixedValue(); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("absent FixedValue() = %+v, %v; want zero, false", got, ok)
	}
	if got, ok := absent.AbsentValueConstraint(); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("absent AbsentValueConstraint() = %+v, %v; want zero, false", got, ok)
	}
	empty := newAttributeUseRead(&names, attributeUse{Fixed: &valueConstraint{}})
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

	names, err := newNameTable(8, []string{EmptyNamespaceURI}, nil)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	name, err := newNameInterner(&names).InternQName("", "a")
	if err != nil {
		t.Fatal(err)
	}
	value := testQNameValue(t, "urn:fixed")
	fixed := &valueConstraint{
		Lexical: "p:item", Canonical: "{urn:fixed}item", Value: value,
		ResolvedNames: []resolvedValueName{{Lexical: "p:item", NS: "urn:fixed", Local: "item"}},
		qnameContext:  &valueConstraintQNameContext{bindings: []valueConstraintQNameBinding{{prefix: "p", namespace: "urn:fixed", bound: true}}},
	}
	decl := newAttributeDeclRead(attributeDecl{Name: name, Type: 7, Fixed: fixed})
	defaultOnly := newAttributeDeclRead(attributeDecl{Name: name, Type: 7, Default: fixed})
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
	empty := newAttributeDeclRead(attributeDecl{Fixed: &valueConstraint{}})
	if got, ok := empty.FixedValue(); !ok || got != (ValueConstraintRead{}) {
		t.Fatalf("present-empty declaration FixedValue() = %+v, %v; want zero, true", got, ok)
	}

	var zero AttributeDeclRead
	if got, ok := zero.FixedValue(); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("zero FixedValue() = %+v, %v; want zero, false", got, ok)
	}
}

func testAttributeUseSetReads(t *testing.T, names *nameTable, sets []attributeUseSet) []AttributeUseSetRead {
	t.Helper()
	reads, err := newAttributeUseSetReads(names, sets, valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62}))
	if err != nil {
		t.Fatal(err)
	}
	return reads
}

func TestAttributeUseSetReadRejectsUnavailableInputMetadata(t *testing.T) {
	t.Parallel()

	names, name, _ := attributeUseNameTable(t)
	values := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	reads, err := newAttributeUseSetReads(&names, []attributeUseSet{{
		Uses: []attributeUse{{Name: name, Type: valuepkg.BuiltinTypeCount}},
	}}, values)
	expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	if reads != nil {
		t.Fatal("unavailable input metadata produced attribute-use reads")
	}
}

func TestAttributeUseSetRead(t *testing.T) {
	t.Parallel()

	names, firstName, secondName := attributeUseNameTable(t)
	firstUse := attributeUse{Name: firstName, Required: true}
	secondUse := attributeUse{Name: secondName, Default: &valueConstraint{}}
	index := map[QName]uint32{
		firstName:  0,
		secondName: 1,
	}
	uses := []attributeUse{firstUse, secondUse}

	set := testAttributeUseSetReads(t, &names, []attributeUseSet{{
		Index:    index,
		Uses:     uses,
		Wildcard: 7,
	}})[0]

	index[firstName] = 99
	uses[0] = attributeUse{}
	uses[1].Default = nil

	if set.UseCount() != 2 {
		t.Fatalf("UseCount() = %d, want 2", set.UseCount())
	}
	if set.Wildcard() != 7 {
		t.Fatalf("Wildcard() = %d, want 7", set.Wildcard())
	}
	if first, ok := set.UseAt(0); !ok || first.Label() != `"first"` {
		t.Fatalf("UseAt(0).Label() = %q, %v; want first, true", first.Label(), ok)
	}
	if second, ok := set.UseAt(1); !ok || second.Label() != `"second"` {
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
	interner := newNameInterner(&names)
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
		uses                  []attributeUse
		required, constraints []uint32
	}{
		{name: "absent"},
		{name: "optional", uses: []attributeUse{{Name: first}}},
		{name: "required only", uses: []attributeUse{{Name: first, Required: true}}, required: []uint32{0}},
		{name: "empty default only", uses: []attributeUse{{Name: first, Default: &valueConstraint{}}}, constraints: []uint32{0}},
		{name: "mixed", uses: []attributeUse{
			{Name: first},
			{Name: second, Required: true},
			{Name: third, Default: &valueConstraint{}},
			{Name: fourth, Required: true, Fixed: &valueConstraint{Canonical: "fixed"}},
		}, required: []uint32{1, 3}, constraints: []uint32{2, 3}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			read := testAttributeUseSetReads(t, &names, []attributeUseSet{{Uses: test.uses}})[0]
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
	use := attributeUse{Name: name}
	if got, slot, ok := testAttributeUseSetReads(t, &names, []attributeUseSet{{
		Uses: []attributeUse{use},
	}})[0].DeclaredUse(name); ok || slot != -1 || got != (AttributeUseRead{}) {
		t.Fatal("DeclaredUse() succeeded without index entry")
	}
	if got, slot, ok := testAttributeUseSetReads(t, &names, []attributeUseSet{{
		Index: map[QName]uint32{name: 99},
		Uses:  []attributeUse{use},
	}})[0].DeclaredUse(name); ok || slot != -1 || got != (AttributeUseRead{}) {
		t.Fatal("DeclaredUse() succeeded with invalid index slot")
	}
	if got, slot, ok := testAttributeUseSetReads(t, &names, []attributeUseSet{{
		Index: map[QName]uint32{name: 0},
		Uses:  []attributeUse{{Name: noQName()}},
	}})[0].DeclaredUse(name); ok || slot != -1 || got != (AttributeUseRead{}) {
		t.Fatal("DeclaredUse() succeeded with stale index name")
	}
}

func TestAttributeUseSetReadReturnsUsesByValue(t *testing.T) {
	t.Parallel()

	names, name, _ := attributeUseNameTable(t)
	set := testAttributeUseSetReads(t, &names, []attributeUseSet{{
		Index: map[QName]uint32{name: 0},
		Uses:  []attributeUse{{Name: name}},
	}})[0]

	declared, _, ok := set.DeclaredUse(name)
	if !ok {
		t.Fatal("DeclaredUse() failed")
	}
	declared.name = noQName()
	if declared.Name() != noQName() {
		t.Fatalf("mutated declared use name = %v; want no name", declared.Name())
	}

	stored, ok := set.UseAt(0)
	if !ok || stored.Name() != name {
		t.Fatalf("UseAt(0) = %v, %v; want %v, true", stored.Name(), ok, name)
	}
	stored.name = noQName()
	if stored.Name() != noQName() {
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
	base := wildcardRuntime{Mode: WildcardAny, Process: ProcessLax}
	declared := wildcardRuntime{Mode: WildcardTargetNamespace, Namespaces: []namespaceID{1}, Process: processStrict}
	union, err := unionWildcard(declared, base, declared.Process)
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
		state   attributeUseSet
	}{
		{
			name:  "none matches declared wildcard",
			state: attributeWildcardUseSet(declaredID, NoWildcard, declaredID, attributeWildcardNone),
		},
		{
			name:    "none rejects base provenance",
			state:   attributeWildcardUseSet(declaredID, baseID, declaredID, attributeWildcardNone),
			wantErr: "attribute wildcard does not match declared wildcard",
		},
		{
			name:  "restriction keeps declared subset",
			state: attributeWildcardUseSet(declaredID, baseID, declaredID, attributeWildcardRestriction),
		},
		{
			name:    "restriction rejects non-subset",
			state:   attributeWildcardUseSet(baseID, declaredID, baseID, attributeWildcardRestriction),
			wantErr: "attribute wildcard restriction does not match provenance",
		},
		{
			name:    "restriction rejects missing base",
			state:   attributeWildcardUseSet(declaredID, NoWildcard, declaredID, attributeWildcardRestriction),
			wantErr: "attribute wildcard restriction has no base wildcard",
		},
		{
			name:    "restriction rejects undeclared wildcard",
			state:   attributeWildcardUseSet(declaredID, NoWildcard, NoWildcard, attributeWildcardRestriction),
			wantErr: "attribute wildcard restriction stores undeclared wildcard",
		},
		{
			name:  "extension inherits base when no declared wildcard",
			state: attributeWildcardUseSet(baseID, baseID, NoWildcard, attributeWildcardExtension),
		},
		{
			name:  "extension uses declared wildcard when no base wildcard",
			state: attributeWildcardUseSet(declaredID, NoWildcard, declaredID, attributeWildcardExtension),
		},
		{
			name:  "extension validates stored union",
			state: attributeWildcardUseSet(unionID, baseID, declaredID, attributeWildcardExtension),
		},
		{
			name:    "extension rejects wrong union",
			state:   attributeWildcardUseSet(declaredID, baseID, declaredID, attributeWildcardExtension),
			wantErr: "attribute wildcard extension does not match provenance",
		},
		{
			name:    "invalid wildcard id",
			state:   attributeWildcardUseSet(WildcardID(99), NoWildcard, NoWildcard, attributeWildcardNone),
			wantErr: "attribute use set references invalid wildcard",
		},
		{
			name:    "invalid base id",
			state:   attributeWildcardUseSet(NoWildcard, WildcardID(99), NoWildcard, attributeWildcardNone),
			wantErr: "attribute use set references invalid base wildcard",
		},
		{
			name:    "invalid declared id",
			state:   attributeWildcardUseSet(NoWildcard, NoWildcard, WildcardID(99), attributeWildcardNone),
			wantErr: "attribute use set references invalid declared wildcard",
		},
		{
			name:    "invalid derivation",
			state:   attributeWildcardUseSet(NoWildcard, NoWildcard, NoWildcard, attributeWildcardDerivation(99)),
			wantErr: "attribute use set has invalid wildcard derivation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAttributeWildcardProvenance(rt.build(), tt.state)
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
		baseID: {Mode: WildcardLocal, Process: processStrict},
	}
	base := attributeWildcardUseSet(baseID, NoWildcard, NoWildcard, attributeWildcardNone)
	derived := attributeWildcardUseSet(baseID, baseID, NoWildcard, attributeWildcardExtension)
	if err := validateAttributeWildcardDerivation(rt.build(), base, derived, attributeWildcardExtension); err != nil {
		t.Fatalf("ValidateAttributeWildcardDerivation() error = %v", err)
	}
	if err := validateAttributeWildcardDerivation(rt.build(), base, derived, attributeWildcardRestriction); err == nil ||
		!strings.Contains(err.Error(), "attribute wildcard derivation does not match owning type") {
		t.Fatalf("ValidateAttributeWildcardDerivation() wrong derivation error = %v", err)
	}
	derived.WildcardBase = NoWildcard
	if err := validateAttributeWildcardDerivation(rt.build(), base, derived, attributeWildcardExtension); err == nil ||
		!strings.Contains(err.Error(), "attribute wildcard base does not match owning type") {
		t.Fatalf("ValidateAttributeWildcardDerivation() wrong base error = %v", err)
	}
}

func TestValidateAttributeUseSetRecord(t *testing.T) {
	t.Parallel()

	names, first, second := attributeUseNameTable(t)
	rt, idTypes := attributeUseSetIDFixture(t)
	type attributeUseSetCase struct {
		name    string
		mutate  func(*attributeUseSet)
		wantErr string
	}
	tests := []attributeUseSetCase{
		{
			name: "valid",
		},
		{
			name: "index size drift",
			mutate: func(set *attributeUseSet) {
				set.Index[QName{Namespace: emptyNamespaceID, Local: 99}] = 2
			},
			wantErr: "attribute use set index size does not match uses",
		},
		{
			name: "invalid name",
			mutate: func(set *attributeUseSet) {
				set.Uses[0].Name = QName{Namespace: 99, Local: 99}
			},
			wantErr: "attribute use references invalid name or type",
		},
		{
			name: "invalid type",
			mutate: func(set *attributeUseSet) {
				set.Uses[0].Type = 99
			},
			wantErr: "attribute use references invalid name or type",
		},
		{
			name: "index slot drift",
			mutate: func(set *attributeUseSet) {
				set.Index[first] = 1
			},
			wantErr: "attribute use index does not match use slice",
		},
		{
			name: "default and fixed",
			mutate: func(set *attributeUseSet) {
				set.Uses[0].Default = &valueConstraint{}
				set.Uses[0].Fixed = &valueConstraint{}
			},
			wantErr: "attribute use stores both default and fixed value constraints",
		},
	}
	for _, carrier := range idTypes {
		tests = append(tests, attributeUseSetCase{
			name: carrier.name + " value constraint",
			mutate: func(set *attributeUseSet) {
				set.Uses[0].Type = carrier.id
				set.Uses[0].Default = &valueConstraint{}
			},
			wantErr: "ID-typed attribute use stores value constraint",
		}, attributeUseSetCase{
			name: "multiple " + carrier.name + " attributes",
			mutate: func(set *attributeUseSet) {
				set.Uses[0].Type = idTypes[0].id
				set.Uses[1].Type = carrier.id
				set.Uses[1].Default = nil
			},
			wantErr: "attribute use set stores multiple ID attributes",
		})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			set := validAttributeUseSet(first, second)
			if tt.mutate != nil {
				tt.mutate(&set)
			}
			err := validateAttributeUseSetRecord(&names, rt, set)
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
		simple: []simpleTypeDerivation{
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
			{Base: 0, Variety: valuepkg.Atomic},
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
		},
		complex: []complexTypeDerivation{{Kind: derivationKindNone}},
	}
	base := attributeUse{Type: 0}
	derived := attributeUse{Type: 1}
	tests := []struct {
		name    string
		wantErr string
		base    attributeUse
		derived attributeUse
	}{
		{
			name:    "derived type restricts base type",
			base:    base,
			derived: derived,
		},
		{
			name: "required base cannot become optional",
			base: attributeUse{
				Type:     0,
				Required: true,
			},
			derived: derived,
			wantErr: "required attribute cannot become optional by restriction",
		},
		{
			name: "derived type must derive from base type",
			base: base,
			derived: attributeUse{
				Type: 2,
			},
			wantErr: "restricted attribute type is not derived from base",
		},
		{
			name: "fixed value preserved",
			base: attributeUse{
				Type:  0,
				Fixed: attributeUseConstraintForTest(t, "decimal", "5.0"),
			},
			derived: attributeUse{
				Type:  1,
				Fixed: attributeUseConstraintForTest(t, "decimal", "5"),
			},
		},
		{
			name: "fixed missing",
			base: attributeUse{
				Type:  0,
				Fixed: attributeUseConstraintForTest(t, "string", "fixed"),
			},
			derived: derived,
			wantErr: "fixed attribute constraint must be preserved by restriction",
		},
		{
			name: "fixed value mismatch",
			base: attributeUse{
				Type:  0,
				Fixed: attributeUseConstraintForTest(t, "string", "true"),
			},
			derived: attributeUse{
				Type:  1,
				Fixed: attributeUseConstraintForTest(t, "boolean", "true"),
			},
			wantErr: "fixed attribute constraint must be preserved by restriction",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAttributeUseRestriction(rt.build(), tt.base, tt.derived)
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
		simple: []simpleTypeDerivation{
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
			{Base: 0, Variety: valuepkg.Atomic},
			{Base: NoSimpleType, Variety: valuepkg.Atomic},
		},
		complex: []complexTypeDerivation{{Kind: derivationKindNone}},
		attributeWildcardFixture: attributeWildcardFixture{
			baseWildcard:    {Mode: WildcardAny, Process: processStrict},
			derivedWildcard: {Mode: WildcardList, Namespaces: []namespaceID{third.Namespace}, Process: processStrict},
		},
	}
	base := []attributeUse{
		{Name: first, Type: 0, Required: true},
		{Name: second, Type: 0},
	}
	derived := []attributeUse{
		{Name: first, Type: 1, Required: true},
		{Name: second, Type: 1},
	}
	withExtra := func(use attributeUse) []attributeUse {
		out := append([]attributeUse(nil), derived...)
		return append(out, use)
	}
	baseState := attributeUseSet{Wildcard: baseWildcard}
	derivedState := attributeUseSet{
		Wildcard:         derivedWildcard,
		WildcardBase:     baseWildcard,
		WildcardDeclared: derivedWildcard,
		WildcardDerive:   attributeWildcardRestriction,
	}
	tests := []struct {
		name         string
		wantErr      string
		derived      []attributeUse
		baseState    attributeUseSet
		derivedState attributeUseSet
		binding      attributeWildcardBinding
	}{
		{
			name:         "valid restriction without wildcard binding",
			derived:      derived,
			baseState:    baseState,
			derivedState: attributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
		},
		{
			name:         "valid restriction with wildcard binding",
			derived:      derived,
			baseState:    baseState,
			derivedState: derivedState,
			binding:      attributeWildcardBound,
		},
		{
			name: "missing required base use",
			derived: []attributeUse{
				{Name: second, Type: 1},
			},
			baseState:    baseState,
			derivedState: attributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
			wantErr:      "complex restriction omits required base attribute",
		},
		{
			name: "pairwise restriction must be valid",
			derived: []attributeUse{
				{Name: first, Type: 2, Required: true},
				{Name: second, Type: 1},
			},
			baseState:    baseState,
			derivedState: attributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
			wantErr:      "complex restriction attribute use is invalid",
		},
		{
			name:         "derived-only use allowed by base wildcard",
			derived:      withExtra(attributeUse{Name: third, Type: 1}),
			baseState:    baseState,
			derivedState: attributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
		},
		{
			name:         "derived-only use requires base wildcard",
			derived:      withExtra(attributeUse{Name: third, Type: 1}),
			baseState:    attributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
			derivedState: attributeUseSet{Wildcard: NoWildcard, WildcardBase: NoWildcard, WildcardDeclared: NoWildcard},
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
			derivedState: attributeUseSet{Wildcard: derivedWildcard, WildcardBase: baseWildcard, WildcardDeclared: derivedWildcard},
			binding:      attributeWildcardBound,
			wantErr:      "attribute wildcard derivation does not match owning type",
		},
		{
			name:         "invalid wildcard binding",
			derived:      derived,
			baseState:    baseState,
			derivedState: derivedState,
			binding:      attributeWildcardBinding(99),
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
			err := validateAttributeUseSetRestriction(rt.build(), baseSet, derivedSet, tt.binding)
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

	qname, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62}).Validate(
		valuepkg.BuiltinType(valuepkg.PrimitiveQName), "p:item",
		valuepkg.Resolver{QName: func(string) (valuepkg.ExpandedName, bool) {
			return valuepkg.ExpandedName{Namespace: "urn:test", Local: "item"}, true
		}},
		valuepkg.NeedCanonical|valuepkg.NeedIdentity, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	value := &valueConstraint{
		ResolvedNames: []resolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}},
		Lexical:       "p:item",
		Canonical:     "p:item",
		Value:         qname,
	}
	fixed := &valueConstraint{
		Lexical:   "fixed",
		Canonical: "fixed",
		Value:     builtinValueForTest(t, "string", "fixed"),
	}
	base := []attributeUse{
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
		mutate  func([]attributeUse) []attributeUse
		wantErr string
	}{
		{
			name: "preserves base uses",
		},
		{
			name: "missing base use",
			mutate: func(uses []attributeUse) []attributeUse {
				return uses[:1]
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "type mismatch",
			mutate: func(uses []attributeUse) []attributeUse {
				uses[0].Type = 3
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "required mismatch",
			mutate: func(uses []attributeUse) []attributeUse {
				uses[0].Required = false
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default presence mismatch",
			mutate: func(uses []attributeUse) []attributeUse {
				uses[0].Default = nil
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default canonical mismatch",
			mutate: func(uses []attributeUse) []attributeUse {
				uses[0].Default.Canonical = "other"
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default value mismatch",
			mutate: func(uses []attributeUse) []attributeUse {
				uses[0].Default.Value = builtinValueForTest(t, "string", "other")
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "default resolved name mismatch",
			mutate: func(uses []attributeUse) []attributeUse {
				uses[0].Default.ResolvedNames[0].Local = "other"
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		},
		{
			name: "fixed canonical mismatch",
			mutate: func(uses []attributeUse) []attributeUse {
				uses[1].Fixed.Canonical = "other"
				return uses
			},
			wantErr: "complex extension does not preserve base attribute use",
		}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			derived := cloneAttributeUsesForTest(base)
			if tt.mutate != nil {
				derived = tt.mutate(derived)
			}
			err := validateAttributeUseSetExtension(attributeUseSet{Uses: base}, attributeUseSet{Uses: derived})
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

func cloneAttributeUsesForTest(in []attributeUse) []attributeUse {
	out := append([]attributeUse(nil), in...)
	for i := range out {
		if out[i].Default != nil {
			cloned := *out[i].Default
			cloned.ResolvedNames = append([]resolvedValueName(nil), cloned.ResolvedNames...)
			out[i].Default = &cloned
		}
		if out[i].Fixed != nil {
			cloned := *out[i].Fixed
			cloned.ResolvedNames = append([]resolvedValueName(nil), cloned.ResolvedNames...)
			out[i].Fixed = &cloned
		}
	}
	return out
}

func attributeUseConstraintForTest(tb testing.TB, typeName, lexical string) *valueConstraint {
	tb.Helper()
	validated := builtinValueForTest(tb, typeName, lexical)
	return &valueConstraint{Lexical: lexical, Canonical: validated.CanonicalText(), Value: validated}
}

type attributeWildcardFixture map[WildcardID]wildcardRuntime

type namedSimpleType struct {
	name string
	id   SimpleTypeID
}

// attributeUseSetIDFixture returns a value-backed build whose ID carriers
// cover I-016: xs:ID, a list of xs:ID, and a union with an xs:ID member.
func attributeUseSetIDFixture(t *testing.T) (*schemaBuild, []namedSimpleType) {
	t.Helper()
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	id, found := valuepkg.BuiltinTypeID("ID")
	if !found {
		t.Fatal("missing builtin ID")
	}
	list := completeConstraintFixtureType(t, b, valuepkg.TypeSpec{
		Base: valuepkg.AnySimpleTypeID, ListItem: id, Variety: valuepkg.List,
		Whitespace: valuepkg.WhitespaceCollapse, WhitespacePresent: true,
	})
	union := completeConstraintFixtureType(t, b, valuepkg.TypeSpec{
		Base: valuepkg.NoType, ListItem: valuepkg.NoType, Variety: valuepkg.Union,
		Union: []valuepkg.TypeID{valuepkg.BuiltinType(valuepkg.PrimitiveDecimal), id},
	})
	build := &schemaBuild{valueBuilder: b, SimpleTypes: make([]simpleType, int(max(list, union))+1)}
	return build, []namedSimpleType{{name: "ID", id: id}, {name: "list of ID", id: list}, {name: "union with ID", id: union}}
}

func attributeWildcardUseSet(wildcard, base, declared WildcardID, derivation attributeWildcardDerivation) attributeUseSet {
	return attributeUseSet{
		Wildcard:         wildcard,
		WildcardBase:     base,
		WildcardDeclared: declared,
		WildcardDerive:   derivation,
	}
}

func attributeUseNameTable(t *testing.T) (names nameTable, first, second QName) {
	t.Helper()

	names, err := newNameTable(8, []string{EmptyNamespaceURI}, []ExpandedName{{Local: "first"}, {Local: "second"}})
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

func validAttributeUseSet(first, second QName) attributeUseSet {
	return attributeUseSet{
		Index: map[QName]uint32{
			first:  0,
			second: 1,
		},
		Uses: []attributeUse{
			{Name: first, Type: 0, Required: true},
			{Name: second, Type: 0, Default: &valueConstraint{}},
		},
		Wildcard:         NoWildcard,
		WildcardBase:     NoWildcard,
		WildcardDeclared: NoWildcard,
	}
}

func (f attributeWildcardFixture) build() *schemaBuild {
	return &schemaBuild{Wildcards: fixtureMapSlice(f)}
}

func (f attributeUseRestrictionFixture) build() *schemaBuild {
	b := f.derivationFixture.build()
	b.Wildcards = fixtureMapSlice(f.attributeWildcardFixture)
	return b
}
