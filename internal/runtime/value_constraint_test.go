package runtime

import (
	"errors"
	"strings"
	"testing"
)

type valueConstraintRuntimeStub map[SimpleTypeID]ValueConstraintSimpleType

func (s valueConstraintRuntimeStub) ValueConstraintSimpleType(id SimpleTypeID) (ValueConstraintSimpleType, bool) {
	st, ok := s[id]
	return st, ok
}

func fixedValueConstraintIdentity(lexical, canonical string, typ SimpleTypeID, identity string) ValueConstraintIdentity {
	return ValueConstraintIdentity{
		Lexical:   lexical,
		Canonical: canonical,
		Value: SimpleValue{
			Canonical: canonical,
			Identity:  identity,
			Type:      typ,
		},
		Present: true,
	}
}

func TestValueConstraintRecordProjections(t *testing.T) {
	t.Parallel()

	value := SimpleValue{Canonical: "p:item", Identity: "qname:p:item", Type: 7}
	vc := &ValueConstraint{
		ResolvedNames: []ResolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}},
		Lexical:       "p:item",
		Canonical:     "p:item",
		Value:         value,
	}

	read, ok := NewValueConstraintReadFromConstraint(vc)
	if !ok || read.LexicalText() != "p:item" || read.CanonicalText() != "p:item" || read.SimpleValue() != value {
		t.Fatalf("NewValueConstraintReadFromConstraint() = %+v, %v; want projected value", read, ok)
	}
	if read, ok := NewValueConstraintReadFromConstraint(nil); ok || read != (ValueConstraintRead{}) {
		t.Fatalf("NewValueConstraintReadFromConstraint(nil) = %+v, %v; want zero, false", read, ok)
	}

	identity := NewValueConstraintIdentity(vc)
	identity.ResolvedNames[0].Local = "other"
	if vc.ResolvedNames[0].Local != "item" {
		t.Fatalf("NewValueConstraintIdentity aliased resolved names: %#v", vc.ResolvedNames)
	}
	if !identity.Present || identity.Lexical != "p:item" || identity.Canonical != "p:item" || identity.Value != value {
		t.Fatalf("NewValueConstraintIdentity() = %+v; want projected value", identity)
	}
	if identity := NewValueConstraintIdentity(nil); identity.Present || len(identity.ResolvedNames) != 0 {
		t.Fatalf("NewValueConstraintIdentity(nil) = %+v; want zero", identity)
	}

	validation := NewValueConstraintValidation(vc)
	if validation.Lexical != "p:item" || validation.Canonical != "p:item" || validation.Value != value || !validation.HasResolvedNames {
		t.Fatalf("NewValueConstraintValidation() = %+v; want projected value", validation)
	}
	if validation := NewValueConstraintValidation(nil); validation != (ValueConstraintValidation{}) {
		t.Fatalf("NewValueConstraintValidation(nil) = %+v; want zero", validation)
	}
}

func TestValueConstraintRead(t *testing.T) {
	t.Parallel()

	value := SimpleValue{Canonical: "1", Type: 7, Identity: "decimal:1"}
	vc := NewValueConstraintRead("01", "1", value)
	if vc.LexicalText() != "01" {
		t.Fatalf("LexicalText() = %q, want 01", vc.LexicalText())
	}
	if vc.CanonicalText() != "1" {
		t.Fatalf("CanonicalText() = %q, want 1", vc.CanonicalText())
	}
	if got := vc.SimpleValue(); got != value {
		t.Fatalf("SimpleValue() = %+v, want %+v", got, value)
	}
}

func TestAbsentValueConstraint(t *testing.T) {
	t.Parallel()

	fixed := NewValueConstraintRead("fixed", "fixed", SimpleValue{Canonical: "fixed", Type: 1})
	def := NewValueConstraintRead("default", "default", SimpleValue{Canonical: "default", Type: 1})
	if got, ok := AbsentValueConstraint(fixed, true, def, true); !ok || got.CanonicalText() != "fixed" {
		t.Fatalf("AbsentValueConstraint(fixed/default) = %q, %v; want fixed, true", got.CanonicalText(), ok)
	}
	if got, ok := AbsentValueConstraint(ValueConstraintRead{}, false, def, true); !ok || got.CanonicalText() != "default" {
		t.Fatalf("AbsentValueConstraint(default) = %q, %v; want default, true", got.CanonicalText(), ok)
	}
	if got, ok := AbsentValueConstraint(ValueConstraintRead{}, false, ValueConstraintRead{}, false); ok || got != (ValueConstraintRead{}) {
		t.Fatalf("AbsentValueConstraint(absent) = %+v, %v; want zero, false", got, ok)
	}
}

func TestElementValueConstraintsRead(t *testing.T) {
	t.Parallel()

	owner := SimpleRef(5)
	fixed := NewValueConstraintRead("01", "1", SimpleValue{Canonical: "1", Type: 5})
	constraints := NewElementValueConstraints(owner, fixed, true, ValueConstraintRead{}, false)
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

	def := &ValueConstraint{
		Lexical:   "abc",
		Canonical: "abc",
		Value:     SimpleValue{Canonical: "abc", Type: 6},
	}
	decls := []ElementDecl{
		{Type: owner, Fixed: &ValueConstraint{
			Lexical:   "01",
			Canonical: "1",
			Value:     SimpleValue{Canonical: "1", Type: 5},
		}},
		{Type: SimpleRef(6), Default: def},
	}
	reads := NewElementValueConstraintReadsForDecls(decls)
	if !EqualElementValueConstraintReadProjectionForDecls(reads, decls) {
		t.Fatalf("NewElementValueConstraintReadsForDecls() = %+v, want projection for declarations", reads)
	}
	if err := ValidateElementValueConstraintReadProjectionForDecls(reads, decls); err != nil {
		t.Fatalf("ValidateElementValueConstraintReadProjectionForDecls() error = %v", err)
	}
	if err := ValidateElementValueConstraintReadProjectionForDecls(reads[:1], decls); err == nil || err.Error() != "element value read projection count does not match declarations" {
		t.Fatalf("ValidateElementValueConstraintReadProjectionForDecls(short) error = %v, want count invariant", err)
	}
	decls[0].Fixed.Canonical = "2"
	if EqualElementValueConstraintReadProjectionForDecls(reads, decls) {
		t.Fatal("EqualElementValueConstraintReadProjectionForDecls() accepted fixed-value drift")
	}
	if err := ValidateElementValueConstraintReadProjectionForDecls(reads, decls); err == nil || err.Error() != "element value read projection does not match declaration" {
		t.Fatalf("ValidateElementValueConstraintReadProjectionForDecls(changed) error = %v, want mismatch invariant", err)
	}
}

func TestEqualElementValueConstraints(t *testing.T) {
	t.Parallel()

	fixed := NewValueConstraintRead("01", "1", SimpleValue{Canonical: "1", Type: 5})
	def := NewValueConstraintRead("abc", "abc", SimpleValue{Canonical: "abc", Type: 6})
	base := NewElementValueConstraints(SimpleRef(5), fixed, true, def, true)
	tests := []struct {
		name string
		a    ElementValueConstraints
		b    ElementValueConstraints
		want bool
	}{
		{
			name: "equal",
			a:    base,
			b:    NewElementValueConstraints(SimpleRef(5), fixed, true, def, true),
			want: true,
		},
		{
			name: "owner mismatch",
			a:    base,
			b:    NewElementValueConstraints(SimpleRef(6), fixed, true, def, true),
		},
		{
			name: "fixed presence mismatch",
			a:    base,
			b:    NewElementValueConstraints(SimpleRef(5), fixed, false, def, true),
		},
		{
			name: "fixed value mismatch",
			a:    base,
			b:    NewElementValueConstraints(SimpleRef(5), NewValueConstraintRead("02", "2", SimpleValue{Canonical: "2", Type: 5}), true, def, true),
		},
		{
			name: "default presence mismatch",
			a:    base,
			b:    NewElementValueConstraints(SimpleRef(5), fixed, true, def, false),
		},
		{
			name: "default value mismatch",
			a:    base,
			b:    NewElementValueConstraints(SimpleRef(5), fixed, true, NewValueConstraintRead("def", "def", SimpleValue{Canonical: "def", Type: 6}), true),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := EqualElementValueConstraints(tt.a, tt.b); got != tt.want {
				t.Fatalf("EqualElementValueConstraints() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestElementValueConstraintReadProjectionHelpers(t *testing.T) {
	t.Parallel()

	fixed := NewValueConstraintRead("01", "1", SimpleValue{Canonical: "1", Type: 5})
	def := NewValueConstraintRead("abc", "abc", SimpleValue{Canonical: "abc", Type: 6})
	shapes := []ElementValueConstraintReadShape{
		{Owner: SimpleRef(5), Fixed: fixed, HasFixed: true, Default: def, HasDefault: true},
		{Owner: ComplexRef(2)},
	}

	reads := NewElementValueConstraintReads(shapes)
	if !EqualElementValueConstraintReadProjection(reads, shapes) {
		t.Fatalf("NewElementValueConstraintReads() = %#v, want projection for %#v", reads, shapes)
	}
	if got, declared, ok := ElementValueConstraintsByID(reads, 0); !ok || !declared || got.OwnerType() != SimpleRef(5) {
		t.Fatalf("ElementValueConstraintsByID() = %+v, %v, %v; want first read, true, true", got, declared, ok)
	}
	if got, declared, ok := ElementValueConstraintsByID(reads, NoElement); !ok || declared || got != (ElementValueConstraints{}) {
		t.Fatalf("ElementValueConstraintsByID(no element) = %+v, %v, %v; want zero, false, true", got, declared, ok)
	}
	if got, declared, ok := ElementValueConstraintsByID(reads, ElementID(99)); ok || declared || got != (ElementValueConstraints{}) {
		t.Fatalf("ElementValueConstraintsByID(invalid) = %+v, %v, %v; want zero, false, false", got, declared, ok)
	}
	if EqualElementValueConstraintReadProjection(reads[:1], shapes) {
		t.Fatal("EqualElementValueConstraintReadProjection() accepted mismatched table length")
	}

	tests := []struct {
		name   string
		mutate func([]ElementValueConstraints)
	}{
		{"owner mismatch", func(reads []ElementValueConstraints) { reads[0].owner = SimpleRef(9) }},
		{"fixed presence mismatch", func(reads []ElementValueConstraints) { reads[0].hasFixed = false }},
		{"fixed value mismatch", func(reads []ElementValueConstraints) {
			reads[0].fixed = NewValueConstraintRead("02", "2", SimpleValue{Canonical: "2", Type: 5})
		}},
		{"default presence mismatch", func(reads []ElementValueConstraints) { reads[0].hasDefault = false }},
		{"default value mismatch", func(reads []ElementValueConstraints) {
			reads[0].defaultValue = NewValueConstraintRead("def", "def", SimpleValue{Canonical: "def", Type: 6})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := NewElementValueConstraintReads(shapes)
			tt.mutate(got)
			if EqualElementValueConstraintReadProjection(got, shapes) {
				t.Fatal("EqualElementValueConstraintReadProjection() accepted mismatched projection")
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
				ns, local, ok := replay.ResolveQName(lexical)
				want := tt.wantResolve[i]
				wantOK := want.NS != "" || want.Local != ""
				if ns != want.NS || local != want.Local || ok != wantOK {
					t.Fatalf("ResolveQName(%q) = %q, %q, %v; want %q, %q, %v", lexical, ns, local, ok, want.NS, want.Local, wantOK)
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

	decimalFive := SimpleIdentityKey(PrimitiveDecimal, "5")
	stringTrue := SimpleIdentityKey(PrimitiveString, "true")
	booleanTrue := SimpleIdentityKey(PrimitiveBoolean, "true")
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
				Value:     SimpleValue{Canonical: "a b", Type: NoSimpleType},
				Present:   true,
			},
			derived: ValueConstraintIdentity{
				Lexical:   "a b",
				Canonical: "a b",
				Value:     SimpleValue{Canonical: "a b", Type: NoSimpleType},
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

	fixed := NewValueConstraintRead(
		"P1Y",
		"P1Y",
		SimpleValue{Canonical: "P1Y", Identity: "duration:12-months", Type: 1},
	)
	actual := SimpleValue{Canonical: "P12M", Identity: "duration:12-months", Type: 1}
	if equal, valid := FixedAttributeValueEqual(actual, fixed, false); equal || !valid {
		t.Fatalf("use-owned equality = %v, %v, want false, true", equal, valid)
	}
	if equal, valid := FixedAttributeValueEqual(actual, fixed, true); !equal || !valid {
		t.Fatalf("declaration-owned equality = %v, %v, want true, true", equal, valid)
	}
	actual.Identity = ""
	if equal, valid := FixedAttributeValueEqual(actual, fixed, true); equal || valid {
		t.Fatalf("missing identity equality = %v, %v, want false, false", equal, valid)
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
		Value:     SimpleValue{Canonical: "value", Type: 1},
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
			vc:       ValueConstraintValidation{Lexical: "value", Canonical: "value", Value: SimpleValue{Canonical: "other", Type: 1}},
			expected: 1,
			wantErr:  "canonical value mismatch",
		},
		{
			name: "valid mixed lexical",
			vc: ValueConstraintValidation{
				Lexical:   "text",
				Canonical: "text",
				Value:     SimpleValue{Canonical: "text", Type: NoSimpleType},
			},
			expected: NoSimpleType,
		},
		{
			name: "mixed rejects typed value",
			vc: ValueConstraintValidation{
				Lexical:   "text",
				Canonical: "text",
				Value:     SimpleValue{Canonical: "text", Type: 1},
			},
			expected: NoSimpleType,
			wantErr:  "mixed value constraint is not untyped lexical text",
		},
		{
			name: "mixed rejects identity payload",
			vc: ValueConstraintValidation{
				Lexical:          "text",
				Canonical:        "text",
				Value:            SimpleValue{Canonical: "text", Type: NoSimpleType, Identity: "id"},
				HasResolvedNames: true,
			},
			expected: NoSimpleType,
			wantErr:  "mixed value constraint is not untyped lexical text",
		},
		{
			name:     "invalid actual type",
			rt:       rt,
			vc:       ValueConstraintValidation{Lexical: "value", Canonical: "value", Value: SimpleValue{Canonical: "value", Type: 99}},
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
			name:     "actual type cannot be union owner",
			rt:       rt,
			vc:       ValueConstraintValidation{Lexical: "value", Canonical: "value", Value: SimpleValue{Canonical: "value", Type: 2}},
			expected: 2,
			wantErr:  "stores union owner as simple value type",
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

	cached := ValueConstraintValidation{
		Lexical:   "p:item",
		Canonical: FormatExpandedName("urn:test", "item"),
		Value: SimpleValue{
			Canonical: FormatExpandedName("urn:test", "item"),
			Identity:  SimpleIdentityKey(PrimitiveQName, FormatExpandedName("urn:test", "item")),
			Type:      1,
		},
	}
	names := []ResolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}}
	validating := func(id SimpleTypeID, lexical string, resolve ValueConstraintQNameResolver, needs SimpleValueNeed) (SimpleValue, error) {
		if id != 1 || lexical != "p:item" || needs != SimpleNeedCanonical|SimpleNeedIdentity {
			return SimpleValue{}, errors.New("unexpected validator args")
		}
		ns, local, ok := resolve(lexical)
		if !ok || ns != "urn:test" || local != "item" {
			return SimpleValue{}, errors.New("unexpected resolved QName")
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
			validate: func(SimpleTypeID, string, ValueConstraintQNameResolver, SimpleValueNeed) (SimpleValue, error) {
				return SimpleValue{}, errors.New("invalid")
			},
			wantErr: "lexical value no longer validates against owner type",
		},
		{
			name:   "unused proof",
			cached: cached,
			names:  append(append([]ResolvedValueName(nil), names...), ResolvedValueName{Lexical: "p:other", NS: "urn:test", Local: "other"}),
			validate: func(SimpleTypeID, string, ValueConstraintQNameResolver, SimpleValueNeed) (SimpleValue, error) {
				return cached.Value, nil
			},
			wantErr: "resolved name proof was not fully consumed",
		},
		{
			name:   "cached value mismatch",
			cached: cached,
			names:  names,
			validate: func(_ SimpleTypeID, lexical string, resolve ValueConstraintQNameResolver, _ SimpleValueNeed) (SimpleValue, error) {
				resolve(lexical)
				return SimpleValue{Canonical: "other", Type: 1}, nil
			},
			wantErr: "cached value does not match replayed validation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateValueConstraintReplay(tt.cached, 1, tt.names, tt.validate)
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

	valid := ValueConstraintValidation{
		Value: SimpleValue{
			Canonical: "abc",
			IDs:       "id",
			IDRefs:    "ref",
			Identity:  "identity",
			Type:      1,
		},
	}
	tests := []struct {
		name     string
		wantErr  string
		replayed SimpleValue
	}{
		{
			name:     "match",
			replayed: valid.Value,
		},
		{
			name:     "canonical mismatch",
			replayed: SimpleValue{Canonical: "other", IDs: "id", IDRefs: "ref", Identity: "identity", Type: 1},
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "type mismatch",
			replayed: SimpleValue{Canonical: "abc", IDs: "id", IDRefs: "ref", Identity: "identity", Type: 2},
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "identity mismatch",
			replayed: SimpleValue{Canonical: "abc", IDs: "id", IDRefs: "ref", Identity: "other", Type: 1},
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "IDs mismatch",
			replayed: SimpleValue{Canonical: "abc", IDs: "other", IDRefs: "ref", Identity: "identity", Type: 1},
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "IDRefs mismatch",
			replayed: SimpleValue{Canonical: "abc", IDs: "id", IDRefs: "other", Identity: "identity", Type: 1},
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
	complex map[ComplexTypeID]ValueConstraintComplexType
	testParticleRuntime
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
		testParticleRuntime: testParticleRuntime{
			models: []ContentModel{
				{Kind: ModelEmpty},
				{
					Kind:      ModelSequence,
					Occurs:    one,
					Particles: []Particle{ElementParticle(0, one)},
				},
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

			got, err := ElementValueConstraintType(tt.rt, tt.typ)
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
