package schema

import (
	"slices"
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func testValue(t *testing.T, id valuepkg.TypeID, lexical string, resolver valuepkg.Resolver, needs valuepkg.Needs) valuepkg.Value {
	t.Helper()
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	p, err := b.Seal()
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	v, err := p.Validate(id, lexical, resolver, needs, valuepkg.EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil)
	if err != nil {
		t.Fatalf("Validate(%d, %q) error = %v", id, lexical, err)
	}
	return v
}

func testBuiltinValue(t *testing.T, kind valuepkg.PrimitiveKind, lexical string) valuepkg.Value {
	t.Helper()
	return testValue(t, valuepkg.BuiltinType(kind), lexical, valuepkg.Resolver{}, valuepkg.NeedCanonical|valuepkg.NeedIdentity)
}

func valueIdentityKey(kind valuepkg.PrimitiveKind, lexical string) string {
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	p, err := b.Seal()
	if err != nil {
		panic(err)
	}
	v, err := p.Validate(valuepkg.BuiltinType(kind), lexical, valuepkg.Resolver{}, valuepkg.NeedIdentity, valuepkg.EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil)
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

func fixedValueConstraintIdentity(lexical, canonical string, typ SimpleTypeID, identity string) constraintIdentity {
	return constraintIdentity{
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
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	p, err := b.Seal()
	if err != nil {
		panic(err)
	}
	v, err := p.Validate(valuepkg.BuiltinType(kind), lexical, resolver, needs, valuepkg.EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil)
	if err != nil {
		panic(err)
	}
	return v
}

func TestValueConstraintRecordProjections(t *testing.T) {
	t.Parallel()

	value := testQNameValue(t, "urn:test")
	vc := &valueConstraint{
		ResolvedNames: []resolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}},
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

	identity := newValueConstraintIdentity(vc)
	identity.ResolvedNames[0].Local = "other"
	if vc.ResolvedNames[0].Local != "item" {
		t.Fatalf("NewValueConstraintIdentity aliased resolved names: %#v", vc.ResolvedNames)
	}
	if !identity.Present || identity.Lexical != "p:item" || identity.Canonical != "p:item" || !identity.Value.Equal(value) {
		t.Fatalf("NewValueConstraintIdentity() = %+v; want projected value", identity)
	}
	if identity := newValueConstraintIdentity(nil); identity.Present || len(identity.ResolvedNames) != 0 {
		t.Fatalf("NewValueConstraintIdentity(nil) = %+v; want zero", identity)
	}

	validation := newValueConstraintValidation(vc)
	if validation.Lexical != "p:item" || validation.Canonical != "p:item" || !validation.Value.Equal(value) || !validation.HasResolvedNames {
		t.Fatalf("NewValueConstraintValidation() = %+v; want projected value", validation)
	}
	if validation := newValueConstraintValidation(nil); validation != (valueConstraintValidation{}) {
		t.Fatalf("NewValueConstraintValidation(nil) = %+v; want zero", validation)
	}
}

func TestValueConstraintRead(t *testing.T) {
	t.Parallel()

	value := testBuiltinValue(t, valuepkg.PrimitiveDecimal, "1")
	vc, ok := newValueConstraintReadFromConstraint(&valueConstraint{Lexical: "01", Canonical: "1", Value: value})
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
		entries     []resolvedValueName
		resolve     []string
		wantResolve []resolvedValueName
	}{
		{
			name: "empty",
		},
		{
			name:        "valid prefixed",
			entries:     []resolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}},
			resolve:     []string{"p:item"},
			wantResolve: []resolvedValueName{{NS: "urn:test", Local: "item"}},
		},
		{
			name:        "valid unprefixed",
			entries:     []resolvedValueName{{Lexical: "item", NS: "", Local: "item"}},
			resolve:     []string{"item"},
			wantResolve: []resolvedValueName{{NS: "", Local: "item"}},
		},
		{
			name:        "duplicate same resolution",
			entries:     []resolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}, {Lexical: "p:item", NS: "urn:test", Local: "item"}},
			resolve:     []string{"p:item"},
			wantResolve: []resolvedValueName{{NS: "urn:test", Local: "item"}},
		},
		{
			name:       "duplicate conflicting resolution",
			entries:    []resolvedValueName{{Lexical: "p:item", NS: "urn:a", Local: "item"}, {Lexical: "p:item", NS: "urn:b", Local: "item"}},
			wantNewErr: "resolved name proof is not deterministic",
		},
		{
			name:       "local mismatch",
			entries:    []resolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "other"}},
			wantNewErr: "resolved name proof is not deterministic",
		},
		{
			name:       "invalid lexical QName",
			entries:    []resolvedValueName{{Lexical: "p:item:extra", NS: "urn:test", Local: "item"}},
			wantNewErr: "resolved name proof is not deterministic",
		},
		{
			name:    "resolve missing lexical",
			entries: []resolvedValueName{{Lexical: "p:item", NS: "urn:test", Local: "item"}},
			resolve: []string{"p:other", "p:item"},
			wantResolve: []resolvedValueName{
				{},
				{NS: "urn:test", Local: "item"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			replay, err := newValueConstraintNameReplay(tt.entries)
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
		base    constraintIdentity
		derived constraintIdentity
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
			name: "untyped compares canonical text",
			base: constraintIdentity{
				Lexical:   "a  b",
				Canonical: "a b",
				Value:     valuepkg.NewUntypedValue("a b"),
				Present:   true,
			},
			derived: constraintIdentity{
				Lexical:   "a b",
				Canonical: "a b",
				Value:     valuepkg.NewUntypedValue("a b"),
				Present:   true,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := fixedValueConstraintEqual(tt.base, tt.derived); got != tt.want {
				t.Fatalf("FixedValueConstraintEqual() = %v, want %v", got, tt.want)
			}
		})
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
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
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
	return notationConstraintTypes{build: &schemaBuild{valueBuilder: b, SimpleTypes: make([]simpleType, int(enumerated)+1)}, enumerated: enumerated}
}

func TestValidateValueConstraintShape(t *testing.T) {
	t.Parallel()
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
	stringID := valuepkg.BuiltinType(valuepkg.PrimitiveString)
	unionID := completeConstraintFixtureType(t, b, valuepkg.TypeSpec{Base: valuepkg.NoType, ListItem: valuepkg.NoType,
		Variety: valuepkg.Union, Union: []valuepkg.TypeID{stringID}})
	shared := completeConstraintFixtureType(t, b, valuepkg.TypeSpec{Base: valuepkg.NoType, ListItem: valuepkg.NoType, Variety: valuepkg.Union, Union: []valuepkg.TypeID{unionID, unionID}})
	rt := &schemaBuild{valueBuilder: b}
	cycle, cycleID := compileOwnerCallbackCycleFixture(t)
	needs := valuepkg.NeedCanonical | valuepkg.NeedIdentity
	valid := valueConstraintValidation{Lexical: "value", Canonical: "value", Value: testBuiltinValue(t, valuepkg.PrimitiveString, "value")}
	unionValue, err := b.Validate(unionID, "value", valuepkg.Resolver{}, needs, nil)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		rt       *schemaBuild
		vc       valueConstraintValidation
		expected SimpleTypeID
		wantErr  string
	}{
		{"valid atomic owner", rt, valid, stringID, ""},
		{"canonical mismatch", rt, valueConstraintValidation{Lexical: "value", Canonical: "value", Value: testBuiltinValue(t, valuepkg.PrimitiveString, "other")}, stringID, "canonical value mismatch"},
		{"valid mixed lexical", nil, valueConstraintValidation{Lexical: "text", Canonical: "text", Value: valuepkg.NewUntypedValue("text")}, NoSimpleType, ""},
		{"mixed rejects typed value", nil, valueConstraintValidation{Lexical: "text", Canonical: "text", Value: testBuiltinValue(t, valuepkg.PrimitiveString, "text")}, NoSimpleType, "mixed value constraint is not untyped lexical text"},
		{"mixed rejects identity payload", nil, valueConstraintValidation{Lexical: "text", Canonical: "text", Value: testBuiltinValue(t, valuepkg.PrimitiveString, "text"), HasResolvedNames: true}, NoSimpleType, "mixed value constraint is not untyped lexical text"},
		{"invalid actual type", rt, valueConstraintValidation{Lexical: "value", Canonical: "value", Value: testBuiltinValue(t, valuepkg.PrimitiveBoolean, "true")}, stringID, "value type does not match owner type"},
		{"invalid expected type", rt, valid, 999, "value type does not match owner type"},
		{"union owner rejects member actual type", rt, valid, unionID, "value type does not match owner type"},
		{"shared union rejects member actual type", rt, valid, shared, "value type does not match owner type"},
		{"union owner stores owner type", rt, valueConstraintValidation{Lexical: "value", Canonical: "value", Value: unionValue}, unionID, ""},
		{"cyclic union owner rejected", cycle, valid, cycleID, "value type does not match owner type"},
		{"nil runtime rejected", nil, valid, stringID, "value type does not match owner type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateValueConstraintShape(tt.rt, tt.vc, tt.expected)
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

func TestElementValueConstraintType(t *testing.T) {
	t.Parallel()

	const (
		emptyID contentModelID = 0
		seqID   contentModelID = 1
	)
	one := occurrence{Min: 1, Max: 1}
	rt := &schemaBuild{
		Models: []contentModelNode{
			{Kind: modelEmpty},
			{Kind: modelSequence, Occurs: one, Particles: []contentParticle{elementParticle(0, one)}},
		},
		ComplexTypes: []complexTypeMetadata{
			{Content: emptyID, TextType: 1, ContentKind: contentSimple},
			{Content: emptyID, TextType: NoSimpleType, ContentKind: contentMixed},
			{Content: seqID, TextType: NoSimpleType, ContentKind: contentMixed},
			{Content: emptyID, TextType: NoSimpleType, ContentKind: contentElementOnly},
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
			got, err := elementValueConstraintType(tt.rt, analysis, tt.typ)
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
	b := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
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
	builder := valuepkg.NewBuilder(valuepkg.BuilderOptions{MaxPatternWork: 1 << 30, MaxTotalPatternWork: 1 << 62})
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
	if shapeErr := validateValueConstraintShape(rt, valueConstraintValidation{Lexical: "owner-value", Canonical: "owner-value", Value: cached}, outer); shapeErr != nil {
		t.Fatalf("exact outer-owner cache rejected: %v", shapeErr)
	}
}
