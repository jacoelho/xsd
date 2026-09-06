package value_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestBuiltinValueContracts(t *testing.T) {
	t.Parallel()
	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	resolver := value.Resolver{
		QName: func(s string) (string, string, bool) {
			return "urn:test", "item", s == "p:item"
		},
		Notation: func(ns, local string) bool { return ns == "urn:test" && local == "item" },
	}
	for _, tc := range []struct {
		typ, lexical string
		valid        bool
	}{
		{"anySimpleType", "", true},
		{"string", "a b", true},
		{"string", "", true},
		{"normalizedString", "a\tb", true},
		{"token", "  a\tb  ", true},
		{"language", "en-GB", true},
		{"language", "en GB", false},
		{"Name", "p:item", true},
		{"Name", "7item", false},
		{"NCName", "item", true},
		{"NCName", "p:item", false},
		{"boolean", " 1 ", true},
		{"boolean", "True", false},
		{"decimal", "+123.00", true},
		{"decimal", "1e2", false},
		{"integer", "+123", true},
		{"integer", "123.0", false},
		{"nonPositiveInteger", "0", true},
		{"nonPositiveInteger", "1", false},
		{"negativeInteger", "-1", true},
		{"negativeInteger", "0", false},
		{"nonNegativeInteger", "0", true},
		{"nonNegativeInteger", "-1", false},
		{"positiveInteger", "1", true},
		{"positiveInteger", "0", false},
		{"long", "-9223372036854775808", true},
		{"long", "9223372036854775808", false},
		{"int", "2147483647", true},
		{"int", "2147483648", false},
		{"short", "-32768", true},
		{"short", "-32769", false},
		{"byte", "127", true},
		{"byte", "128", false},
		{"unsignedLong", "18446744073709551615", true},
		{"unsignedLong", "18446744073709551616", false},
		{"unsignedInt", "4294967295", true},
		{"unsignedInt", "4294967296", false},
		{"unsignedShort", "65535", true},
		{"unsignedShort", "65536", false},
		{"unsignedByte", "255", true},
		{"unsignedByte", "256", false},
		{"float", "INF", true},
		{"float", "1e2", true},
		{"double", "NaN", true},
		{"double", "nan", false},
		{"duration", "P1Y2M3DT4H5M6.7S", true},
		{"duration", "P", false},
		{"date", "2024-02-29Z", true},
		{"date", "2023-02-29Z", false},
		{"dateTime", "2024-02-29T12:34:56Z", true},
		{"dateTime", "2024-02-29", false},
		{"time", "12:34:56+01:00", true},
		{"time", "25:00:00", false},
		{"gYearMonth", "2024-02", true},
		{"gYear", "2024", true},
		{"gMonthDay", "--02-29", true},
		{"gDay", "---31", true},
		{"gMonth", "--02", true},
		{"anyURI", "../a", true},
		{"hexBinary", "00aF", true},
		{"hexBinary", "0", false},
		{"base64Binary", "YQ==", true},
		{"base64Binary", "YQ=", false},
		{"QName", "p:item", true},
		{"QName", "q:item", false},
		{"NOTATION", "p:item", true},
		{"ID", "item", true},
		{"ID", "two words", false},
		{"IDREF", "item", true},
		{"IDREFS", "one two", true},
		{"IDREFS", "", false},
		{"NMTOKEN", "7item", true},
		{"NMTOKEN", "two words", false},
		{"NMTOKENS", "one two", true},
		{"NMTOKENS", "", false},
		{"ENTITY", "entity", false},
		{"ENTITIES", "one two", false},
		{"xml:lang", "", true},
		{"xml:lang", "en-GB", true},
		{"xml:space", "preserve", true},
		{"xml:space", "invalid", false},
	} {
		t.Run(tc.typ+"/"+tc.lexical, func(t *testing.T) {
			t.Parallel()
			id, ok := value.BuiltinTypeID(tc.typ)
			if !ok {
				t.Fatalf("missing builtin %q", tc.typ)
			}
			for _, needs := range []value.Needs{0, value.NeedCanonical | value.NeedIdentity} {
				_, err := program.Validate(id, tc.lexical, resolver, needs, nil)
				if (err == nil) != tc.valid {
					t.Errorf("Validate(%q, needs=%d) = %v, valid = %v", tc.lexical, needs, err, tc.valid)
				}
			}
		})
	}
}

func TestForwardBaseRetainsFacets(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{})
	derived, err := builder.Add(value.TypeSpec{
		Variety: value.Atomic, Primitive: value.PrimitiveDecimal,
		Base: value.BuiltinTypeCount + 1, ListItem: value.NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = builder.Add(value.TypeSpec{
		Variety: value.Atomic, Primitive: value.PrimitiveDecimal,
		Base: value.BuiltinType(value.PrimitiveDecimal), ListItem: value.NoType,
		Facets: value.FacetSpec{MinInclusive: value.BoundFacet{
			Present: true, LiteralSpec: value.LiteralSpec{Lexical: "10", Type: value.NoType},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := program.Validate(derived, "9", value.Resolver{}, 0, nil); err == nil {
		t.Fatal("derived type accepted a value below its forward-referenced base bound")
	}
	if _, err := program.Validate(derived, "10", value.Resolver{}, 0, nil); err != nil {
		t.Fatal(err)
	}
}

func TestValueEqualityUsesValueSpace(t *testing.T) {
	t.Parallel()
	program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		leftType, left, rightType, right string
		equal                            bool
	}{
		{"decimal", "1", "integer", "+01", true},
		{"decimal", "1.0", "decimal", "1.00", true},
		{"string", "", "string", "", true},
		{"string", "1", "decimal", "1", false},
		{"boolean", "1", "boolean", "true", true},
		{"boolean", "1", "string", "true", false},
		{"duration", "P1D", "duration", "PT24H", true},
	} {
		t.Run(tc.leftType+"/"+tc.rightType+"/"+tc.left, func(t *testing.T) {
			t.Parallel()
			leftID, _ := value.BuiltinTypeID(tc.leftType)
			rightID, _ := value.BuiltinTypeID(tc.rightType)
			left, err := program.Validate(leftID, tc.left, value.Resolver{}, value.NeedIdentity, nil)
			if err != nil {
				t.Fatal(err)
			}
			right, err := program.Validate(rightID, tc.right, value.Resolver{}, value.NeedIdentity, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := left.Equal(right); got != tc.equal {
				t.Fatalf("Equal = %v, want %v", got, tc.equal)
			}
		})
	}
}
