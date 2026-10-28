package schema

import (
	"errors"
	"slices"
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

func testQNameValue(t *testing.T, namespace string) valuepkg.Value {
	t.Helper()
	const lexical = "p:item"
	return testValue(t, valuepkg.BuiltinType(valuepkg.PrimitiveQName), lexical, valuepkg.Resolver{
		QName: func(got string) (valuepkg.ExpandedName, bool) {
			if got != lexical {
				return valuepkg.ExpandedName{}, false
			}
			return valuepkg.ExpandedName{Namespace: namespace, Local: "item"}, true
		},
	}, valuepkg.NeedCanonical|valuepkg.NeedIdentity)
}

func testIdentityValue(t *testing.T, builtin, lexical string) valuepkg.Value {
	t.Helper()
	id, ok := valuepkg.BuiltinTypeID(builtin)
	if !ok {
		t.Fatalf("builtin %s missing", builtin)
	}
	p, err := valuepkg.NewBuilder(valuepkg.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	v, err := p.Validate(id, lexical, valuepkg.Resolver{}, valuepkg.NeedCanonical|valuepkg.NeedIdentity, 16<<20, nil)
	if err != nil {
		t.Fatalf("Validate(%s) error = %v", builtin, err)
	}
	return v
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

	value := testQNameValue(t, "urn:test")
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

func TestFixedAttributeValueEqualUsesValueSpace(t *testing.T) {
	t.Parallel()

	fixedValue := testBuiltinValue(t, valuepkg.PrimitiveDuration, "P1Y", valuepkg.NeedCanonical|valuepkg.NeedIdentity)
	fixed := newValueConstraintRead("P1Y", "P1Y", fixedValue)
	actual := testBuiltinValue(t, valuepkg.PrimitiveDuration, "P12M", valuepkg.NeedCanonical|valuepkg.NeedIdentity)
	if equal, valid := FixedAttributeValueEqual(actual, fixed); !equal || !valid {
		t.Fatalf("declaration-owned equality = %v, %v, want true, true", equal, valid)
	}
	actual = testBuiltinValue(t, valuepkg.PrimitiveDuration, "P12M", valuepkg.NeedCanonical)
	if equal, valid := FixedAttributeValueEqual(actual, fixed); equal || valid {
		t.Fatalf("missing identity equality = %v, %v, want false, false", equal, valid)
	}
}

func completeConstraintFixtureType(t *testing.T, b *valuepkg.Builder, spec valuepkg.TypeSpec) SimpleTypeID {
	t.Helper()
	id, err := b.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Complete(id, spec); err != nil {
		t.Fatalf("Complete constraint fixture: %v", err)
	}
	return id
}

type notationConstraintTypes struct {
	build      *schemaBuild
	enumerated SimpleTypeID
}

func notationConstraintFixture(t *testing.T) notationConstraintTypes {
	t.Helper()
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	bare := valuepkg.BuiltinType(valuepkg.PrimitiveNotation)
	enumerated := completeConstraintFixtureType(t, b, valuepkg.TypeSpec{
		Base: bare, ListItem: valuepkg.NoType, Primitive: valuepkg.PrimitiveNotation,
		Facets: valuepkg.FacetSpec{Present: valuepkg.FacetEnumeration, Enumeration: []valuepkg.LiteralSpec{{
			Type: valuepkg.NoType, Lexical: "p:item",
			Resolver: valuepkg.Resolver{
				QName: func(string) (valuepkg.ExpandedName, bool) {
					return valuepkg.ExpandedName{Namespace: "urn:test", Local: "item"}, true
				},
				Notation: func(string, string) bool { return true },
			},
		}}},
	})
	return notationConstraintTypes{build: &schemaBuild{valueBuilder: b, SimpleTypes: make([]SimpleType, int(enumerated)+1)}, enumerated: enumerated}
}

func TestValidateValueConstraintShape(t *testing.T) {
	t.Parallel()
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	stringID := valuepkg.BuiltinType(valuepkg.PrimitiveString)
	unionID := completeConstraintFixtureType(t, b, valuepkg.TypeSpec{Base: valuepkg.NoType, ListItem: valuepkg.NoType,
		Variety: valuepkg.Union, Union: []valuepkg.TypeID{stringID}})
	shared := completeConstraintFixtureType(t, b, valuepkg.TypeSpec{Base: valuepkg.NoType, ListItem: valuepkg.NoType, Variety: valuepkg.Union, Union: []valuepkg.TypeID{unionID, unionID}})
	rt := &schemaBuild{valueBuilder: b}
	cycle, cycleID := compileOwnerCallbackCycleFixture(t)
	needs := valuepkg.NeedCanonical | valuepkg.NeedIdentity
	valid := ValueConstraintValidation{Lexical: "value", Canonical: "value", Value: testBuiltinValue(t, valuepkg.PrimitiveString, "value", needs)}
	unionValue, err := b.Validate(unionID, "value", valuepkg.Resolver{}, needs, nil)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		rt       *schemaBuild
		vc       ValueConstraintValidation
		expected SimpleTypeID
		wantErr  string
	}{
		{"valid atomic owner", rt, valid, stringID, ""},
		{"canonical mismatch", rt, ValueConstraintValidation{Lexical: "value", Canonical: "value", Value: testBuiltinValue(t, valuepkg.PrimitiveString, "other", needs)}, stringID, "canonical value mismatch"},
		{"valid mixed lexical", nil, ValueConstraintValidation{Lexical: "text", Canonical: "text", Value: valuepkg.NewUntypedValue("text")}, NoSimpleType, ""},
		{"mixed rejects typed value", nil, ValueConstraintValidation{Lexical: "text", Canonical: "text", Value: testBuiltinValue(t, valuepkg.PrimitiveString, "text", needs)}, NoSimpleType, "mixed value constraint is not untyped lexical text"},
		{"mixed rejects identity payload", nil, ValueConstraintValidation{Lexical: "text", Canonical: "text", Value: testBuiltinValue(t, valuepkg.PrimitiveString, "text", needs), HasResolvedNames: true}, NoSimpleType, "mixed value constraint is not untyped lexical text"},
		{"invalid actual type", rt, ValueConstraintValidation{Lexical: "value", Canonical: "value", Value: testBuiltinValue(t, valuepkg.PrimitiveBoolean, "true", needs)}, stringID, "value type does not match owner type"},
		{"invalid expected type", rt, valid, 999, "value type does not match owner type"},
		{"union owner rejects member actual type", rt, valid, unionID, "value type does not match owner type"},
		{"shared union rejects member actual type", rt, valid, shared, "value type does not match owner type"},
		{"union owner stores owner type", rt, ValueConstraintValidation{Lexical: "value", Canonical: "value", Value: unionValue}, unionID, ""},
		{"cyclic union owner rejected", cycle, valid, cycleID, "value type does not match owner type"},
		{"nil runtime rejected", nil, valid, stringID, "value type does not match owner type"},
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
		Value:     testQNameValue(t, "urn:test"),
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

	validValue := testIdentityValue(t, "ID", "id")
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
			replayed: testIdentityValue(t, "ID", "other"),
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "type mismatch",
			replayed: testIdentityValue(t, "ID", "different"),
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "identity mismatch",
			replayed: testIdentityValue(t, "IDREF", "id"),
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "IDs mismatch",
			replayed: testIdentityValue(t, "ID", "other"),
			wantErr:  "cached value does not match replayed validation",
		},
		{
			name:     "IDRefs mismatch",
			replayed: testIdentityValue(t, "IDREF", "other"),
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

func TestElementValueConstraintType(t *testing.T) {
	t.Parallel()

	const (
		emptyID ContentModelID = 0
		seqID   ContentModelID = 1
	)
	one := Occurrence{Min: 1, Max: 1}
	rt := &schemaBuild{
		Models: []ContentModel{
			{Kind: ModelEmpty},
			{Kind: ModelSequence, Occurs: one, Particles: []Particle{ElementParticle(0, one)}},
		},
		ComplexTypes: []ComplexType{
			{Content: emptyID, TextType: 1, ContentKind: ContentSimple},
			{Content: emptyID, TextType: NoSimpleType, ContentKind: ContentMixed},
			{Content: seqID, TextType: NoSimpleType, ContentKind: ContentMixed},
			{Content: emptyID, TextType: NoSimpleType, ContentKind: ContentElementOnly},
		},
	}
	tests := []struct {
		rt      *schemaBuild
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

func compileOwnerCallbackCycleFixture(t *testing.T) (*schemaBuild, SimpleTypeID) {
	t.Helper()
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	a, err := b.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	c, err := b.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	var nestedErr error
	parentErr := b.Complete(a, valuepkg.TypeSpec{
		Base:      valuepkg.BuiltinType(valuepkg.PrimitiveQName),
		ListItem:  valuepkg.NoType,
		Primitive: valuepkg.PrimitiveQName,
		Facets: valuepkg.FacetSpec{
			Present: valuepkg.FacetEnumeration,
			Enumeration: []valuepkg.LiteralSpec{{
				Type:    valuepkg.NoType,
				Lexical: "p:item",
				Resolver: valuepkg.Resolver{QName: func(string) (valuepkg.ExpandedName, bool) {
					nestedErr = b.Complete(c, valuepkg.TypeSpec{
						Base: valuepkg.NoType, ListItem: valuepkg.NoType,
						Variety: valuepkg.Union, Union: []valuepkg.TypeID{a},
					})
					return valuepkg.ExpandedName{}, false
				}},
			}},
		},
	})
	if parentErr == nil || nestedErr != nil {
		t.Fatalf("parent failure = %v, nested completion = %v", parentErr, nestedErr)
	}
	if _, available := b.TypeView(a); available {
		t.Fatal("failed parent remains available")
	}
	if _, available := b.TypeView(c); !available {
		t.Fatal("successful nested type was lost after parent failure")
	}
	if err := b.Complete(a, valuepkg.TypeSpec{
		Base: valuepkg.NoType, ListItem: valuepkg.NoType,
		Variety: valuepkg.Union, Union: []valuepkg.TypeID{c},
	}); err != nil {
		t.Fatalf("retry parent = %v", err)
	}
	for id, member := range map[valuepkg.TypeID]valuepkg.TypeID{a: c, c: a} {
		view, available := b.TypeView(id)
		if !available || view.Variety != valuepkg.Union || !slices.Equal(view.Union, []valuepkg.TypeID{member}) {
			t.Fatalf("cyclic type %d = %+v, %v", id, view, available)
		}
	}
	return &schemaBuild{valueBuilder: b}, a
}

func TestValueConstraintNestedSharedUnionKeepsExactOwner(t *testing.T) {
	t.Parallel()
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{})
	stringID := valuepkg.BuiltinType(valuepkg.PrimitiveString)
	makeUnion := func(members ...valuepkg.TypeID) valuepkg.TypeID {
		return completeConstraintFixtureType(t, builder, valuepkg.TypeSpec{Base: valuepkg.NoType, ListItem: valuepkg.NoType, Variety: valuepkg.Union, Union: members})
	}
	shared := makeUnion(stringID)
	left := makeUnion(shared)
	right := makeUnion(shared)
	outer := makeUnion(left, right)
	cached, err := builder.Validate(outer, "owner-value", valuepkg.Resolver{}, valuepkg.NeedCanonical|valuepkg.NeedIdentity, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cached.Type() != outer || cached.CanonicalText() != "owner-value" {
		t.Fatalf("cached owner/canonical = %d/%q; want %d/owner-value", cached.Type(), cached.CanonicalText(), outer)
	}
	rt := &schemaBuild{valueBuilder: builder}
	if shapeErr := ValidateValueConstraintShape(rt, ValueConstraintValidation{Lexical: "owner-value", Canonical: "owner-value", Value: cached}, outer); shapeErr != nil {
		t.Fatalf("exact outer-owner cache rejected: %v", shapeErr)
	}
}
