package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"
)

func TestCompositeValueIdentityProjections(t *testing.T) {
	for _, tc := range []struct {
		name, member string
		id, ref      string
	}{
		{name: "ID union", member: "ID", id: "item"},
		{name: "IDREF union", member: "IDREF", ref: "item"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builder := value.NewBuilder(value.BuilderOptions{})
			member, _ := value.BuiltinTypeID(tc.member)
			integer, _ := value.BuiltinTypeID("int")
			union, err := builder.Reserve()
			if err != nil {
				t.Fatalf("Reserve() error = %v", err)
			}
			err = builder.Complete(union, value.TypeSpec{Variety: value.Union, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{member, integer}})
			if err != nil {
				t.Fatal(err)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
				got, err := program.Validate(union, "item", value.Resolver{}, needs, 16<<20, nil)
				if err != nil {
					t.Fatal(err)
				}
				if got.IDs() != tc.id || got.IDRefs() != tc.ref {
					t.Errorf("needs %d: IDs=%q, IDRefs=%q; want %q, %q", needs, got.IDs(), got.IDRefs(), tc.id, tc.ref)
				}
			}
		})
	}
}

func TestListIdentityUsesSharedFraming(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	id, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(id, value.TypeSpec{Variety: value.List, Base: value.NoType, ListItem: value.BuiltinType(value.PrimitiveAnyURI)})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := program.Validate(id, "urn:first urn:second", value.Resolver{}, value.NeedIdentity, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := value.ListIdentityKey([]string{value.PrimitiveIdentityKey(value.PrimitiveAnyURI, "urn:first"), value.PrimitiveIdentityKey(value.PrimitiveAnyURI, "urn:second")})
	if got.IdentityKey() != want {
		t.Fatalf("list identity=%q, want same framing as XSI list %q", got.IdentityKey(), want)
	}
}

func TestListOfUnionCarriesSelectedIDREFs(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	idref, _ := value.BuiltinTypeID("IDREF")
	integer, _ := value.BuiltinTypeID("int")
	member, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(member, value.TypeSpec{
		Variety: value.Union,
		Base:    value.NoType, ListItem: value.NoType,
		Union: []value.TypeID{integer, idref},
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err := builder.Reserve()
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	err = builder.Complete(list, value.TypeSpec{
		Variety: value.List, Base: value.NoType, ListItem: member,
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
		got, err := program.Validate(list, "one 2 three", value.Resolver{}, needs, 16<<20, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got.IDRefs() != "one three" {
			t.Fatalf("needs %d: IDRefs=%q, want %q", needs, got.IDRefs(), "one three")
		}
	}
}

func TestIdentityPayloadsSurviveLaterEvaluation(t *testing.T) {
	pattern, err := xsdregex.Compile("[a-z]+", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	builder := value.NewBuilder(value.BuilderOptions{})
	item := completeCanonicalProjectionType(t, builder, value.TypeSpec{
		Variety: value.Atomic, Primitive: value.PrimitiveString,
		Base: value.BuiltinType(value.PrimitiveString), ListItem: value.NoType,
		Facets: value.FacetSpec{Patterns: [][]*value.Pattern{{pattern}}},
	})
	list := completeCanonicalProjectionType(t, builder, value.TypeSpec{
		Variety: value.List, Base: value.NoType, ListItem: item,
	})
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	var scratch value.Scratch
	raw := []byte("first")
	primitive, err := program.ValidateBytes(item, raw, value.Resolver{}, value.NeedIdentity, 16<<20, &scratch)
	clear(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte("first last")
	aggregate, err := program.ValidateBytes(list, raw, value.Resolver{}, value.NeedIdentity, 16<<20, &scratch)
	clear(raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(list, "a much longer pair", value.Resolver{}, value.NeedIdentity, 16<<20, &scratch); err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(item, "123", value.Resolver{}, value.NeedIdentity, 16<<20, &scratch); !errors.Is(err, value.ErrFacet) {
		t.Fatalf("invalid patterned value error = %v", err)
	}
	scratch.Reset(0)
	for _, test := range []struct {
		name, lexical, identity string
		typ                     value.TypeID
		retained                value.Value
	}{
		{"primitive", "first", "\x00\x1efirst", item, primitive},
		{"list", "first last", "\x00\x1e7:\x00\x1efirst6:\x00\x1elast", list, aggregate},
	} {
		fresh, err := program.Validate(test.typ, test.lexical, value.Resolver{}, value.NeedIdentity, 16<<20, &scratch)
		if err != nil {
			t.Fatal(err)
		}
		if !test.retained.HasIdentity() || test.retained.IdentityKey() != test.identity || !test.retained.Equal(fresh) {
			t.Fatalf("%s identity after later evaluation = %q", test.name, test.retained.IdentityKey())
		}
	}
}

func TestEmptyListIdentityThroughUnions(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	stringType := value.BuiltinType(value.PrimitiveString)
	list, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err = builder.Complete(list, value.TypeSpec{Variety: value.List, Base: value.NoType, ListItem: stringType}); err != nil {
		t.Fatal(err)
	}
	inner, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err = builder.Complete(inner, value.TypeSpec{Variety: value.Union, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{list, stringType}}); err != nil {
		t.Fatal(err)
	}
	outer, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err = builder.Complete(outer, value.TypeSpec{Variety: value.Union, Base: value.NoType, ListItem: value.NoType, Union: []value.TypeID{inner, stringType}}); err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct {
		name             string
		typeID, selected value.TypeID
	}{
		{name: "list", typeID: list, selected: value.NoType},
		{name: "union", typeID: inner, selected: list},
		{name: "nested union", typeID: outer, selected: inner},
	} {
		t.Run(route.name, func(t *testing.T) {
			for _, needs := range []value.Needs{value.NeedIdentity, value.NeedCanonical | value.NeedIdentity} {
				atomic, err := program.Validate(stringType, "", value.Resolver{}, needs, 16<<20, nil)
				if err != nil {
					t.Fatal(err)
				}
				for _, bytes := range []bool{false, true} {
					var got value.Value
					if bytes {
						got, err = program.ValidateBytes(route.typeID, []byte{}, value.Resolver{}, needs, 16<<20, nil)
					} else {
						got, err = program.Validate(route.typeID, "", value.Resolver{}, needs, 16<<20, nil)
					}
					if err != nil {
						t.Fatalf("needs %d, bytes %t: %v", needs, bytes, err)
					}
					if !got.HasIdentity() || got.Type() != route.typeID || got.SelectedType() != route.selected || got.IdentityKey() != "\x00\x1e" || got.CanonicalText() != "" {
						t.Fatalf("needs %d, bytes %t: type=%d, selected=%d, identity=%q, canonical=%q", needs, bytes, got.Type(), got.SelectedType(), got.IdentityKey(), got.CanonicalText())
					}
					if got.Equal(atomic) {
						t.Fatalf("needs %d, bytes %t: empty list equals empty string", needs, bytes)
					}
				}
			}
		})
	}
}

func TestCanonicalProjectionRequestsAndOwnership(t *testing.T) {
	if (value.Value{}).HasIdentity() {
		t.Fatal("zero value has an identity projection")
	}
	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name                         string
		primitive                    value.PrimitiveKind
		lexical, canonical, identity string
	}{
		{"float exponent", value.PrimitiveFloat, "1e2", "1.0E2", "\x03\x1e1.0E2"},
		{"float precision", value.PrimitiveFloat, "16777217", "1.6777216E7", "\x03\x1e1.6777216E7"},
		{"float maximum", value.PrimitiveFloat, "3.4028235e38", "3.4028235E38", "\x03\x1e3.4028235E38"},
		{"float subnormal", value.PrimitiveFloat, "1e-45", "1.0E-45", "\x03\x1e1.0E-45"},
		{"float positive overflow", value.PrimitiveFloat, "1e39", "INF", "\x03\x1eINF"},
		{"float negative overflow", value.PrimitiveFloat, "-1e39", "-INF", "\x03\x1e-INF"},
		{"float positive underflow", value.PrimitiveFloat, "1e-50", "0.0E0", "\x03\x1e0.0E0"},
		{"float negative underflow", value.PrimitiveFloat, "-1e-50", "0.0E0", "\x03\x1e0.0E0"},
		{"double precision", value.PrimitiveDouble, "16777217", "1.6777217E7", "\x04\x1e1.6777217E7"},
		{"negative zero", value.PrimitiveDouble, "-0", "0.0E0", "\x04\x1e0.0E0"},
		{"infinity", value.PrimitiveDouble, "INF", "INF", "\x04\x1eINF"},
		{"not a number", value.PrimitiveDouble, "NaN", "NaN", "\x04\x1eNaN"},
		{"date", value.PrimitiveDate, "2000-02-29+00:00", "2000-02-29Z", "\x08\x1e2000-02-29Z"},
		{"dateTime", value.PrimitiveDateTime, "2000-02-29T00:30:00.1200+01:00", "2000-02-28T23:30:00.12Z", "\x06\x1e2000-02-28T23:30:00.12Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, needs := range []value.Needs{0, value.NeedCanonical, value.NeedIdentity, value.NeedCanonical | value.NeedIdentity} {
				for _, bytes := range []bool{false, true} {
					id := value.BuiltinType(test.primitive)
					var got value.Value
					if bytes {
						raw := []byte(test.lexical)
						got, err = program.ValidateBytes(id, raw, value.Resolver{}, needs, 16<<20, nil)
						clear(raw)
					} else {
						got, err = program.Validate(id, test.lexical, value.Resolver{}, needs, 16<<20, nil)
					}
					if err != nil {
						t.Fatalf("needs %d, bytes %t: %v", needs, bytes, err)
					}
					if got.Type() != id || got.SelectedType() != value.NoType {
						t.Fatalf("type=%d selected=%d; want %d and NoType", got.Type(), got.SelectedType(), id)
					}
					assertRequestedCanonicalProjection(t, got, needs, test.canonical, test.identity)
				}
			}
		})
	}
}

func TestCanonicalProjectionClearsRejectedUnionMember(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	first := completeCanonicalProjectionType(t, builder, atomicFacetType(value.PrimitiveFloat, boundFacet("2")))
	stringType := value.BuiltinType(value.PrimitiveString)
	union := completeCanonicalProjectionType(t, builder, value.TypeSpec{
		Variety: value.Union, Base: value.NoType, ListItem: value.NoType,
		Union: []value.TypeID{first, stringType},
	})
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, needs := range []value.Needs{0, value.NeedCanonical, value.NeedIdentity, value.NeedCanonical | value.NeedIdentity} {
		for _, bytes := range []bool{false, true} {
			var got value.Value
			if bytes {
				got, err = program.ValidateBytes(union, []byte("1"), value.Resolver{}, needs, 16<<20, nil)
			} else {
				got, err = program.Validate(union, "1", value.Resolver{}, needs, 16<<20, nil)
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Type() != union || got.SelectedType() != stringType {
				t.Fatalf("type=%d selected=%d; want union=%d, string=%d", got.Type(), got.SelectedType(), union, stringType)
			}
			assertRequestedCanonicalProjection(t, got, needs, "1", "\x00\x1e1")
		}
	}
}

func TestCanonicalProjectionRetainsFloatListEnumeration(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	id := completeCanonicalProjectionType(t, builder, value.TypeSpec{
		Variety: value.List, Base: value.NoType, ListItem: value.BuiltinType(value.PrimitiveFloat),
		Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
		Facets: value.FacetSpec{Enumeration: []value.LiteralSpec{{Lexical: "1 2", Type: value.NoType}}},
	})
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, needs := range []value.Needs{0, value.NeedCanonical, value.NeedIdentity, value.NeedCanonical | value.NeedIdentity} {
		got, err := program.Validate(id, "1e0 2e0", value.Resolver{}, needs, 16<<20, nil)
		if err != nil {
			t.Fatal(err)
		}
		assertRequestedCanonicalProjection(t, got, needs, "1.0E0 2.0E0", "\x00\x1e7:\x03\x1e1.0E07:\x03\x1e2.0E0")
		for _, lexical := range []string{"1e0", "1e0 3e0"} {
			if _, err := program.Validate(id, lexical, value.Resolver{}, needs, 16<<20, nil); !errors.Is(err, value.ErrFacet) {
				t.Fatalf("Validate(%q) error=%v; want enumeration facet failed", lexical, err)
			}
		}
	}
}

func completeCanonicalProjectionType(t *testing.T, builder *value.Builder, spec value.TypeSpec) value.TypeID {
	t.Helper()
	id, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.Complete(id, spec); err != nil {
		t.Fatal(err)
	}
	return id
}

func assertRequestedCanonicalProjection(t *testing.T, got value.Value, needs value.Needs, canonical, identity string) {
	t.Helper()
	if got.HasIdentity() != needs.Has(value.NeedIdentity) {
		t.Fatalf("needs %d: identity availability = %t", needs, got.HasIdentity())
	}
	if !needs.Has(value.NeedCanonical) {
		canonical = ""
	}
	if !needs.Has(value.NeedIdentity) {
		identity = ""
	}
	if got.CanonicalText() != canonical || got.IdentityKey() != identity {
		t.Fatalf("needs %d: canonical=%q, identity=%q; want %q, %q", needs, got.CanonicalText(), got.IdentityKey(), canonical, identity)
	}
}

func TestDecimalAndIntegerProjectionRequests(t *testing.T) {
	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		builtin, lexical, canonical, identity string
	}{
		{"integer", "-00012", "-12", "\x02\x1e-12.0"},
		{"decimal", "-00012", "-12.0", "\x02\x1e-12.0"},
		{"integer", "-000", "0", "\x02\x1e0.0"},
		{"decimal", "-000.000", "0.0", "\x02\x1e0.0"},
		{"decimal", "-00012.3400", "-12.34", "\x02\x1e-12.34"},
	} {
		t.Run(test.builtin+"/"+test.lexical, func(t *testing.T) {
			id, ok := value.BuiltinTypeID(test.builtin)
			if !ok {
				t.Fatalf("missing builtin %s", test.builtin)
			}
			for _, needs := range []value.Needs{0, value.NeedCanonical, value.NeedIdentity, value.NeedCanonical | value.NeedIdentity} {
				for _, bytes := range []bool{false, true} {
					var got value.Value
					if bytes {
						raw := []byte(test.lexical)
						got, err = program.ValidateBytes(id, raw, value.Resolver{}, needs, 16<<20, nil)
						clear(raw)
					} else {
						got, err = program.Validate(id, test.lexical, value.Resolver{}, needs, 16<<20, nil)
					}
					if err != nil {
						t.Fatalf("needs %d, bytes %t: %v", needs, bytes, err)
					}
					if got.Type() != id || got.SelectedType() != value.NoType {
						t.Fatalf("needs %d, bytes %t: type/selection=%d/%d, want %d/NoType", needs, bytes, got.Type(), got.SelectedType(), id)
					}
					assertRequestedCanonicalProjection(t, got, needs, test.canonical, test.identity)
					if got.IDs() != "" || got.IDRefs() != "" {
						t.Fatalf("needs %d, bytes %t: unsolicited document identity %q/%q", needs, bytes, got.IDs(), got.IDRefs())
					}
				}
			}
		})
	}
}
