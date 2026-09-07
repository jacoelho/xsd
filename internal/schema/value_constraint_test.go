package schema

import (
	"errors"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func testValue(t *testing.T, id valuepkg.TypeID, lexical string, resolver valuepkg.Resolver, needs valuepkg.Needs) valuepkg.Value {
	t.Helper()
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	p, err := b.Seal()
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	v, err := p.Validate(id, lexical, resolver, needs, 16<<20, nil)
	if err != nil {
		t.Fatalf("Validate(%d, %q) error = %v", id, lexical, err)
	}
	return v
}

func testBuiltinValue(t *testing.T, kind valuepkg.PrimitiveKind, lexical string, needs valuepkg.Needs) valuepkg.Value {
	t.Helper()
	return testValue(t, valuepkg.BuiltinType(kind), lexical, valuepkg.Resolver{}, needs)
}

func valueIdentityKey(kind valuepkg.PrimitiveKind, lexical string) string {
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	p, err := b.Seal()
	if err != nil {
		panic(err)
	}
	v, err := p.Validate(valuepkg.BuiltinType(kind), lexical, valuepkg.Resolver{}, valuepkg.NeedIdentity, 16<<20, nil)
	if err != nil {
		panic(err)
	}
	return v.IdentityKey()
}

func testQNameValue(t *testing.T, lexical, namespace, local string, needs valuepkg.Needs) valuepkg.Value {
	t.Helper()
	return testValue(t, valuepkg.BuiltinType(valuepkg.PrimitiveQName), lexical, valuepkg.Resolver{
		QName: func(got string) (valuepkg.ExpandedName, bool) {
			if got != lexical {
				return valuepkg.ExpandedName{}, false
			}
			return valuepkg.ExpandedName{Namespace: namespace, Local: local}, true
		},
	}, needs)
}

func testIdentityValue(t *testing.T, kind valuepkg.IdentityKind, lexical string) valuepkg.Value {
	t.Helper()
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	id, err := b.Add(valuepkg.TypeSpec{
		Variety:           valuepkg.Atomic,
		Primitive:         valuepkg.PrimitiveString,
		Whitespace:        valuepkg.WhitespaceCollapse,
		WhitespacePresent: true,
		Base:              valuepkg.NoType,
		ListItem:          valuepkg.NoType,
		Identity:          kind,
	})
	if err != nil {
		t.Fatalf("Add(identity type) error = %v", err)
	}
	p, err := b.Seal()
	if err != nil {
		t.Fatalf("Seal(identity type) error = %v", err)
	}
	v, err := p.Validate(id, lexical, valuepkg.Resolver{}, valuepkg.NeedCanonical|valuepkg.NeedIdentity, 16<<20, nil)
	if err != nil {
		t.Fatalf("Validate(identity type) error = %v", err)
	}
	return v
}

type valueConstraintRuntimeStub map[SimpleTypeID]ValueConstraintSimpleType

func (s valueConstraintRuntimeStub) ValueConstraintSimpleType(id SimpleTypeID) (ValueConstraintSimpleType, bool) {
	st, ok := s[id]
	return st, ok
}

func fixedValueConstraintIdentity(lexical, canonical string, typ SimpleTypeID, identity string) ValueConstraintIdentity {
	return ValueConstraintIdentity{
		Lexical:   lexical,
		Canonical: canonical,
		Value:     testConstraintValue(lexical, canonical, typ, identity),
		Present:   true,
	}
}

func testConstraintValue(lexical, canonical string, typ SimpleTypeID, identity string) valuepkg.Value {
	if typ == valuepkg.NoType {
		return valuepkg.NewUntypedValue(canonical)
	}
	needs := valuepkg.NeedCanonical | valuepkg.NeedIdentity
	kind := valuepkg.PrimitiveString
	if identity != "" && identity[0] != 0xff {
		kind = valuepkg.PrimitiveKind(identity[0])
	}
	var resolver valuepkg.Resolver
	if kind == valuepkg.PrimitiveQName || kind == valuepkg.PrimitiveNotation {
		resolver = valuepkg.Resolver{QName: func(string) (valuepkg.ExpandedName, bool) {
			return valuepkg.ExpandedName{Namespace: "urn:test", Local: "item"}, true
		}}
	}
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	p, err := b.Seal()
	if err != nil {
		panic(err)
	}
	v, err := p.Validate(valuepkg.BuiltinType(kind), lexical, resolver, needs, 16<<20, nil)
	if err != nil {
		panic(err)
	}
	return v
}

func TestValueConstraintRecordProjections(t *testing.T) {
	t.Parallel()

	value := testQNameValue(t, "p:item", "urn:test", "item", valuepkg.NeedCanonical|valuepkg.NeedIdentity)
	vc := &ValueConstraint{
		ResolvedNames: []ResolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}},
		Lexical:       "p:item",
		Canonical:     "p:item",
		Value:         value,
	}

	read, ok := newValueConstraintReadFromConstraint(vc)
	if !ok || read.ApplicationText() != "p:item" || read.CanonicalText() != "p:item" || !read.Value().Equal(value) {
		t.Fatalf("NewValueConstraintReadFromConstraint() = %+v, %v; want projected value", read, ok)
	}
	if read, ok := newValueConstraintReadFromConstraint(nil); ok || read != (ValueConstraintRead{}) {
		t.Fatalf("NewValueConstraintReadFromConstraint(nil) = %+v, %v; want zero, false", read, ok)
	}

	identity := NewValueConstraintIdentity(vc)
	identity.ResolvedNames[0].Local = "other"
	if vc.ResolvedNames[0].Local != "item" {
		t.Fatalf("NewValueConstraintIdentity aliased resolved names: %#v", vc.ResolvedNames)
	}
	if !identity.Present || identity.Lexical != "p:item" || identity.Canonical != "p:item" || !identity.Value.Equal(value) {
		t.Fatalf("NewValueConstraintIdentity() = %+v; want projected value", identity)
	}
	if identity := NewValueConstraintIdentity(nil); identity.Present || len(identity.ResolvedNames) != 0 {
		t.Fatalf("NewValueConstraintIdentity(nil) = %+v; want zero", identity)
	}

	validation := NewValueConstraintValidation(vc)
	if validation.Lexical != "p:item" || validation.Canonical != "p:item" || !validation.Value.Equal(value) || !validation.HasResolvedNames {
		t.Fatalf("NewValueConstraintValidation() = %+v; want projected value", validation)
	}
	if validation := NewValueConstraintValidation(nil); validation != (ValueConstraintValidation{}) {
		t.Fatalf("NewValueConstraintValidation(nil) = %+v; want zero", validation)
	}
}

func TestValueConstraintRead(t *testing.T) {
	t.Parallel()

	value := testBuiltinValue(t, valuepkg.PrimitiveDecimal, "1", valuepkg.NeedCanonical|valuepkg.NeedIdentity)
	vc, ok := newValueConstraintReadFromConstraint(&ValueConstraint{Lexical: "01", Canonical: "1", Value: value})
	if !ok || vc.ApplicationText() != "1" {
		t.Fatalf("ApplicationText() = %q, present %v; want 1, true", vc.ApplicationText(), ok)
	}
	if vc.CanonicalText() != "1" {
		t.Fatalf("CanonicalText() = %q, want 1", vc.CanonicalText())
	}
	if got := vc.Value(); !got.Equal(value) {
		t.Fatalf("Value() = %+v, want %+v", got, value)
	}
}

func TestElementValueConstraintsRead(t *testing.T) {
	t.Parallel()

	owner := SimpleRef(5)
	fixed := newValueConstraintRead("01", "1", testBuiltinValue(t, valuepkg.PrimitiveString, "1", valuepkg.NeedCanonical|valuepkg.NeedIdentity))
	constraints := newElementValueConstraints(owner, fixed, true, ValueConstraintRead{}, false)
	if constraints.OwnerType() != owner {
		t.Fatalf("OwnerType() = %v, want %v", constraints.OwnerType(), owner)
	}
	if !constraints.HasAny() {
		t.Fatal("HasAny() = false, want true")
	}
	if got, ok := constraints.FixedValue(); !ok || got.CanonicalText() != "1" {
		t.Fatalf("FixedValue() = %q, %v; want 1, true", got.CanonicalText(), ok)
	}
	if got, ok := constraints.DefaultValueConstraint(); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("DefaultValueConstraint() = %+v, %v; want zero, false", got, ok)
	}
}

func TestEqualElementValueConstraints(t *testing.T) {
	t.Parallel()

	fixed := newValueConstraintRead("01", "1", testBuiltinValue(t, valuepkg.PrimitiveString, "1", valuepkg.NeedCanonical|valuepkg.NeedIdentity))
	def := newValueConstraintRead("abc", "abc", testBuiltinValue(t, valuepkg.PrimitiveString, "abc", valuepkg.NeedCanonical|valuepkg.NeedIdentity))
	base := newElementValueConstraints(SimpleRef(5), fixed, true, def, true)
	tests := []struct {
		name string
		a    ElementValueConstraints
		b    ElementValueConstraints
		want bool
	}{
		{
			name: "equal",
			a:    base,
			b:    newElementValueConstraints(SimpleRef(5), fixed, true, def, true),
			want: true,
		},
		{
			name: "owner mismatch",
			a:    base,
			b:    newElementValueConstraints(SimpleRef(6), fixed, true, def, true),
		},
		{
			name: "fixed presence mismatch",
			a:    base,
			b:    newElementValueConstraints(SimpleRef(5), fixed, false, def, true),
		},
		{
			name: "fixed value mismatch",
			a:    base,
			b:    newElementValueConstraints(SimpleRef(5), newValueConstraintRead("02", "2", testBuiltinValue(t, valuepkg.PrimitiveString, "2", valuepkg.NeedCanonical|valuepkg.NeedIdentity)), true, def, true),
		},
		{
			name: "default presence mismatch",
			a:    base,
			b:    newElementValueConstraints(SimpleRef(5), fixed, true, def, false),
		},
		{
			name: "default value mismatch",
			a:    base,
			b:    newElementValueConstraints(SimpleRef(5), fixed, true, newValueConstraintRead("def", "def", testBuiltinValue(t, valuepkg.PrimitiveString, "def", valuepkg.NeedCanonical|valuepkg.NeedIdentity)), true),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := equalElementValueConstraints(tt.a, tt.b); got != tt.want {
				t.Fatalf("EqualElementValueConstraints() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValueConstraintNameReplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		wantNewErr  string
		wantEndErr  string
		entries     []ResolvedValueName
		resolve     []string
		wantResolve []ResolvedValueName
	}{
		{
			name: "empty",
		},
		{
			name:        "valid prefixed",
			entries:     []ResolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}},
			resolve:     []string{"p:item"},
			wantResolve: []ResolvedValueName{{NS: "urn:test", Local: "item"}},
		},
		{
			name:        "valid unprefixed",
			entries:     []ResolvedValueName{{Lexical: "item", NS: "", Local: "item"}},
			resolve:     []string{"item"},
			wantResolve: []ResolvedValueName{{NS: "", Local: "item"}},
		},
		{
			name:        "duplicate same resolution",
			entries:     []ResolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}, {Lexical: "p:item", NS: "urn:test", Local: "item"}},
			resolve:     []string{"p:item"},
			wantResolve: []ResolvedValueName{{NS: "urn:test", Local: "item"}},
			wantEndErr:  "resolved name proof was not fully consumed",
		},
		{
			name:       "duplicate conflicting resolution",
			entries:    []ResolvedValueName{{Lexical: "p:item", NS: "urn:a", Local: "item"}, {Lexical: "p:item", NS: "urn:b", Local: "item"}},
			wantNewErr: "resolved name proof is not deterministic",
		},
		{
			name:       "local mismatch",
			entries:    []ResolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "other"}},
			wantNewErr: "resolved name proof is not deterministic",
		},
		{
			name:       "invalid lexical QName",
			entries:    []ResolvedValueName{{Lexical: "p:item:extra", NS: "urn:test", Local: "item"}},
			wantNewErr: "resolved name proof is not deterministic",
		},
		{
			name:       "unconsumed entry",
			entries:    []ResolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}},
			wantEndErr: "resolved name proof was not fully consumed",
		},
		{
			name:    "resolve missing lexical",
			entries: []ResolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}},
			resolve: []string{"p:other", "p:item"},
			wantResolve: []ResolvedValueName{
				{},
				{NS: "urn:test", Local: "item"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			replay, err := NewValueConstraintNameReplay(tt.entries)
			if tt.wantNewErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantNewErr) {
					t.Fatalf("NewValueConstraintNameReplay() error = %v, want %q", err, tt.wantNewErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewValueConstraintNameReplay() error = %v", err)
			}
			for i, lexical := range tt.resolve {
				name, ok := replay.ResolveQName(lexical)
				want := tt.wantResolve[i]
				wantOK := want.NS != "" || want.Local != ""
				if name.Namespace != want.NS || name.Local != want.Local || ok != wantOK {
					t.Fatalf("ResolveQName(%q) = %q, %q, %v; want %q, %q, %v", lexical, name.Namespace, name.Local, ok, want.NS, want.Local, wantOK)
				}
			}
			err = replay.ValidateConsumed()
			if tt.wantEndErr == "" {
				if err != nil {
					t.Fatalf("ValidateConsumed() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantEndErr) {
				t.Fatalf("ValidateConsumed() error = %v, want %q", err, tt.wantEndErr)
			}
		})
	}
}

func TestFixedValueConstraintEqual(t *testing.T) {
	t.Parallel()

	decimalFive := valueIdentityKey(valuepkg.PrimitiveDecimal, "5")
	stringTrue := valueIdentityKey(valuepkg.PrimitiveString, "true")
	booleanTrue := valueIdentityKey(valuepkg.PrimitiveBoolean, "true")
	tests := []struct {
		name    string
		base    ValueConstraintIdentity
		derived ValueConstraintIdentity
		want    bool
	}{
		{
			name: "both absent",
			want: true,
		},
		{
			name: "derived missing",
			base: fixedValueConstraintIdentity("5", "5", 1, decimalFive),
		},
		{
			name:    "typed identity match ignores lexical form",
			base:    fixedValueConstraintIdentity("5.0", "5.0", 1, decimalFive),
			derived: fixedValueConstraintIdentity("5", "5", 2, decimalFive),
			want:    true,
		},
		{
			name:    "same canonical text different identity",
			base:    fixedValueConstraintIdentity("true", "true", 1, stringTrue),
			derived: fixedValueConstraintIdentity("true", "true", 2, booleanTrue),
		},
		{
			name: "mixed lexical fallback",
			base: ValueConstraintIdentity{
				Lexical:   "a  b",
				Canonical: "a b",
				Value:     valuepkg.NewUntypedValue("a b"),
				Present:   true,
			},
			derived: ValueConstraintIdentity{
				Lexical:   "a b",
				Canonical: "a b",
				Value:     valuepkg.NewUntypedValue("a b"),
				Present:   true,
			},
			want: true,
		},
		{
			name:    "typed fallback uses full simple value",
			base:    fixedValueConstraintIdentity("a  b", "a b", 1, ""),
			derived: fixedValueConstraintIdentity("a b", "a b", 1, ""),
			want:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := FixedValueConstraintEqual(tt.base, tt.derived); got != tt.want {
				t.Fatalf("FixedValueConstraintEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFixedAttributeValueEqualDistinguishesConstraintOwner(t *testing.T) {
	t.Parallel()

	fixedValue := testBuiltinValue(t, valuepkg.PrimitiveDuration, "P1Y", valuepkg.NeedCanonical|valuepkg.NeedIdentity)
	fixed := newValueConstraintRead("P1Y", "P1Y", fixedValue)
	actual := testBuiltinValue(t, valuepkg.PrimitiveDuration, "P12M", valuepkg.NeedCanonical|valuepkg.NeedIdentity)
	if equal, valid := FixedAttributeValueEqual(actual, fixed, FixedAttributeComparisonLexical); equal || !valid {
		t.Fatalf("use-owned equality = %v, %v, want false, true", equal, valid)
	}
	if equal, valid := FixedAttributeValueEqual(actual, fixed, FixedAttributeComparisonValueSpace); !equal || !valid {
		t.Fatalf("declaration-owned equality = %v, %v, want true, true", equal, valid)
	}
	actual = testBuiltinValue(t, valuepkg.PrimitiveDuration, "P12M", valuepkg.NeedCanonical)
	if equal, valid := FixedAttributeValueEqual(actual, fixed, FixedAttributeComparisonValueSpace); equal || valid {
		t.Fatalf("missing identity equality = %v, %v, want false, false", equal, valid)
	}
	if equal, valid := FixedAttributeValueEqual(actual, fixed, FixedAttributeComparisonInvalid); equal || valid {
		t.Fatalf("invalid comparison equality = %v, %v, want false, false", equal, valid)
	}
}

func TestSimpleTypeUsesBareNotation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rt   valueConstraintRuntimeStub
		name string
		id   SimpleTypeID
		want bool
	}{
		{
			name: "atomic bare notation",
			rt: valueConstraintRuntimeStub{
				1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveNotation},
			},
			id:   1,
			want: true,
		},
		{
			name: "enumerated notation",
			rt: valueConstraintRuntimeStub{
				1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveNotation, HasEnumeration: true},
			},
			id: 1,
		},
		{
			name: "list item bare notation",
			rt: valueConstraintRuntimeStub{
				1: {Variety: SimpleVarietyList, ListItem: 2},
				2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveNotation},
			},
			id:   1,
			want: true,
		},
		{
			name: "union member bare notation",
			rt: valueConstraintRuntimeStub{
				1: {Variety: SimpleVarietyUnion, Union: []SimpleTypeID{2, 3}},
				2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveString},
				3: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveNotation},
			},
			id:   1,
			want: true,
		},
		{
			name: "cycle terminates",
			rt: valueConstraintRuntimeStub{
				1: {Variety: SimpleVarietyUnion, Union: []SimpleTypeID{1}},
			},
			id: 1,
		},
		{
			name: "missing type",
			rt:   valueConstraintRuntimeStub{},
			id:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SimpleTypeUsesBareNotation(tt.rt, tt.id)
			if got != tt.want {
				t.Fatalf("SimpleTypeUsesBareNotation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateValueConstraintShape(t *testing.T) {
	t.Parallel()

	rt := valueConstraintRuntimeStub{
		1: {Variety: SimpleVarietyAtomic},
		2: {Variety: SimpleVarietyUnion, Union: []SimpleTypeID{1}},
		3: {Variety: SimpleVarietyUnion, Union: []SimpleTypeID{4}},
		4: {Variety: SimpleVarietyUnion, Union: []SimpleTypeID{3}},
	}
	valid := ValueConstraintValidation{
		Lexical:   "value",
		Canonical: "value",
		Value:     testBuiltinValue(t, valuepkg.PrimitiveString, "value", valuepkg.NeedCanonical|valuepkg.NeedIdentity),
	}
	tests := []struct {
		rt       ValueConstraintRuntime
		name     string
		wantErr  string
		vc       ValueConstraintValidation
		expected SimpleTypeID
	}{
		{
			name:     "valid atomic owner",
			rt:       rt,
			vc:       valid,
			expected: 1,
		},
		{
			name:     "canonical mismatch",
			rt:       rt,
			vc:       ValueConstraintValidation{Lexical: "value", Canonical: "value", Value: testBuiltinValue(t, valuepkg.PrimitiveString, "other", valuepkg.NeedCanonical|valuepkg.NeedIdentity)},
			expected: 1,
			wantErr:  "canonical value mismatch",
		},
		{
			name: "valid mixed lexical",
			vc: ValueConstraintValidation{
				Lexical:   "text",
				Canonical: "text",
				Value:     valuepkg.NewUntypedValue("text"),
			},
			expected: NoSimpleType,
		},
		{
			name: "mixed rejects typed value",
			vc: ValueConstraintValidation{
				Lexical:   "text",
				Canonical: "text",
				Value:     testBuiltinValue(t, valuepkg.PrimitiveString, "text", valuepkg.NeedCanonical|valuepkg.NeedIdentity),
			},
			expected: NoSimpleType,
			wantErr:  "mixed value constraint is not untyped lexical text",
		},
		{
			name: "mixed rejects identity payload",
			vc: ValueConstraintValidation{
				Lexical:          "text",
				Canonical:        "text",
				Value:            testBuiltinValue(t, valuepkg.PrimitiveString, "text", valuepkg.NeedCanonical|valuepkg.NeedIdentity),
				HasResolvedNames: true,
			},
			expected: NoSimpleType,
			wantErr:  "mixed value constraint is not untyped lexical text",
		},
		{
			name:     "invalid actual type",
			rt:       rt,
			vc:       ValueConstraintValidation{Lexical: "value", Canonical: "value", Value: testBuiltinValue(t, valuepkg.PrimitiveBoolean, "true", valuepkg.NeedCanonical|valuepkg.NeedIdentity)},
			expected: 1,
			wantErr:  "value type does not match owner type",
		},
		{
			name:     "invalid expected type",
			rt:       rt,
			vc:       valid,
			expected: 99,
			wantErr:  "value type does not match owner type",
		},
		{
			name:     "union owner accepts member actual type",
			rt:       rt,
			vc:       valid,
			expected: 2,
		},
		{
			name:     "union owner stores owner type",
			rt:       rt,
			vc:       ValueConstraintValidation{Lexical: "value", Canonical: "value", Value: testValue(t, valuepkg.TypeID(2), "value", valuepkg.Resolver{}, valuepkg.NeedCanonical|valuepkg.NeedIdentity)},
			expected: 2,
		},
		{
			name:     "cyclic union owner rejected",
			rt:       rt,
			vc:       valid,
			expected: 3,
			wantErr:  "value type does not match owner type",
		},
		{
			name:     "nil runtime rejected",
			rt:       nil,
			vc:       valid,
			expected: 1,
			wantErr:  "value type does not match owner type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateValueConstraintShape(tt.rt, tt.vc, tt.expected)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateValueConstraintShape() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateValueConstraintShape() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateValueConstraintReplay(t *testing.T) {
	t.Parallel()

	qnameType := valuepkg.BuiltinType(valuepkg.PrimitiveQName)
	cached := ValueConstraintValidation{
		Lexical:   "p:item",
		Canonical: FormatExpandedName("urn:test", "item"),
		Value:     testQNameValue(t, "p:item", "urn:test", "item", valuepkg.NeedCanonical|valuepkg.NeedIdentity),
	}
	names := []ResolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}}
	validating := func(id SimpleTypeID, lexical string, resolve valuepkg.QNameResolver, needs valuepkg.Needs) (valuepkg.Value, error) {
		if id != qnameType || lexical != "p:item" || needs != valuepkg.NeedCanonical|valuepkg.NeedIdentity {
			return valuepkg.Value{}, errors.New("unexpected validator args")
		}
		name, ok := resolve(lexical)
		if !ok || name.Namespace != "urn:test" || name.Local != "item" {
			return valuepkg.Value{}, errors.New("unexpected resolved QName")
		}
		return cached.Value, nil
	}
	tests := []struct {
		validate ValueConstraintSimpleValidator
		name     string
		wantErr  string
		names    []ResolvedValueName
		cached   ValueConstraintValidation
	}{
		{
			name:     "match",
			cached:   cached,
			names:    names,
			validate: validating,
		},
		{
			name:    "missing validator",
			cached:  cached,
			names:   names,
			wantErr: "missing value constraint validator",
		},
		{
			name:     "invalid proof",
			cached:   cached,
			names:    []ResolvedValueName{{Lexical: "p:item:extra", NS: "urn:test", Local: "item"}},
			validate: validating,
			wantErr:  "resolved name proof is not deterministic",
		},
		{
			name:   "lexical no longer validates",
			cached: cached,
			names:  names,
			validate: func(SimpleTypeID, string, valuepkg.QNameResolver, valuepkg.Needs) (valuepkg.Value, error) {
				return valuepkg.Value{}, errors.New("invalid")
			},
			wantErr: "lexical value no longer validates against owner type",
		},
		{
			name:   "unused proof",
			cached: cached,
			names:  append(append([]ResolvedValueName(nil), names...), ResolvedValueName{Lexical: "p:other", NS: "urn:test", Local: "other"}),
			validate: func(SimpleTypeID, string, valuepkg.QNameResolver, valuepkg.Needs) (valuepkg.Value, error) {
				return cached.Value, nil
			},
			wantErr: "resolved name proof was not fully consumed",
		},
		{
			name:   "cached value mismatch",
			cached: cached,
			names:  names,
			validate: func(_ SimpleTypeID, lexical string, resolve valuepkg.QNameResolver, _ valuepkg.Needs) (valuepkg.Value, error) {
				resolve(lexical)
				return testBuiltinValue(t, valuepkg.PrimitiveString, "other", valuepkg.NeedCanonical|valuepkg.NeedIdentity), nil
			},
			wantErr: "cached value does not match replayed validation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateValueConstraintReplay(tt.cached, qnameType, tt.names, tt.validate)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateValueConstraintReplay() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateValueConstraintReplay() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateValueConstraintReplayResult(t *testing.T) {
	t.Parallel()

	validValue := testIdentityValue(t, valuepkg.IdentityID, "id")
	valid := ValueConstraintValidation{
		Value: validValue,
	}
	tests := []struct {
		name     string
		wantErr  string
		replayed valuepkg.Value
	}{
		{
			name:     "match",
			replayed: validValue,
		},
		{
			name:     "canonical mismatch",
			replayed: testIdentityValue(t, valuepkg.IdentityID, "other"),
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "type mismatch",
			replayed: testIdentityValue(t, valuepkg.IdentityID, "different"),
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "identity mismatch",
			replayed: testIdentityValue(t, valuepkg.IdentityIDREF, "id"),
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "IDs mismatch",
			replayed: testIdentityValue(t, valuepkg.IdentityID, "other"),
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "IDRefs mismatch",
			replayed: testIdentityValue(t, valuepkg.IdentityIDREF, "other"),
			wantErr:  "cached value does not match replayed validation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateValueConstraintReplayResult(valid, tt.replayed)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateValueConstraintReplayResult() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateValueConstraintReplayResult() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

type elementValueConstraintRuntimeStub struct {
	testParticleRuntime

	complex map[ComplexTypeID]ValueConstraintComplexType
}

func (s elementValueConstraintRuntimeStub) ValueConstraintComplexType(id ComplexTypeID) (ValueConstraintComplexType, bool) {
	ct, ok := s.complex[id]
	return ct, ok
}

func TestElementValueConstraintType(t *testing.T) {
	t.Parallel()

	const (
		emptyID ContentModelID = 0
		seqID   ContentModelID = 1
	)
	one := Occurrence{Min: 1, Max: 1}
	rt := elementValueConstraintRuntimeStub{
		models: []ContentModel{
			{Kind: ModelEmpty},
			{
				Kind:      ModelSequence,
				Occurs:    one,
				Particles: []Particle{ElementParticle(0, one)},
			},
		},
		complex: map[ComplexTypeID]ValueConstraintComplexType{
			0: {Content: emptyID, TextType: 1, ContentKind: ContentSimple},
			1: {Content: emptyID, TextType: NoSimpleType, ContentKind: ContentMixed},
			2: {Content: seqID, TextType: NoSimpleType, ContentKind: ContentMixed},
			3: {Content: emptyID, TextType: NoSimpleType, ContentKind: ContentElementOnly},
		},
	}
	tests := []struct {
		rt      ElementValueConstraintRuntime
		name    string
		wantErr string
		typ     TypeID
		want    SimpleTypeID
	}{
		{
			name: "simple owner",
			rt:   rt,
			typ:  SimpleRef(1),
			want: 1,
		},
		{
			name: "complex simple-content owner",
			rt:   rt,
			typ:  ComplexRef(0),
			want: 1,
		},
		{
			name: "mixed emptiable owner is lexical text",
			rt:   rt,
			typ:  ComplexRef(1),
			want: NoSimpleType,
		},
		{
			name:    "mixed non-emptiable owner rejected",
			rt:      rt,
			typ:     ComplexRef(2),
			want:    NoSimpleType,
			wantErr: "element value constraint requires simple content",
		},
		{
			name:    "element-only owner rejected",
			rt:      rt,
			typ:     ComplexRef(3),
			want:    NoSimpleType,
			wantErr: "element value constraint requires simple content",
		},
		{
			name:    "invalid complex owner",
			rt:      rt,
			typ:     ComplexRef(99),
			want:    NoSimpleType,
			wantErr: "element value constraint references invalid type",
		},
		{
			name:    "invalid type",
			rt:      rt,
			typ:     TypeID{},
			want:    NoSimpleType,
			wantErr: "element value constraint references invalid type",
		},
		{
			name:    "nil runtime",
			rt:      nil,
			typ:     ComplexRef(0),
			want:    NoSimpleType,
			wantErr: "element value constraint references invalid type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			analysis := unlimitedContentModelAnalysis(rt)
			got, err := ElementValueConstraintType(tt.rt, analysis, tt.typ)
			if got != tt.want {
				t.Fatalf("ElementValueConstraintType() type = %d, want %d", got, tt.want)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ElementValueConstraintType() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ElementValueConstraintType() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
