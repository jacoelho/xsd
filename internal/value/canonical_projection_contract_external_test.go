package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestCanonicalProjectionRequestsAndOwnership(t *testing.T) {
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

func TestCanonicalProjectionPreservesAdmittedIntegerOverride(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	spec := atomicFacetType(value.PrimitiveDouble, value.FacetSpec{})
	spec.Builtin = value.BuiltinInteger
	id := completeCanonicalProjectionType(t, builder, spec)
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	// Builder admission permits this combination; preserving it is part of the
	// projection refactor even though schema compilation does not produce it.
	for _, needs := range []value.Needs{0, value.NeedCanonical, value.NeedIdentity, value.NeedCanonical | value.NeedIdentity} {
		got, err := program.Validate(id, "1", value.Resolver{}, needs, 16<<20, nil)
		if err != nil {
			t.Fatal(err)
		}
		assertRequestedCanonicalProjection(t, got, needs, "0", "\x04\x1e0")
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
			if _, err := program.Validate(id, lexical, value.Resolver{}, needs, 16<<20, nil); !errors.Is(err, value.ErrFacet) || err.Error() != "enumeration facet failed" {
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
