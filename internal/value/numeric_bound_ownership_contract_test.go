package value_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

type numericByteCase struct {
	input   string
	message string
	facet   bool
}

func TestAdmittedNumericBoundNormalization(t *testing.T) {
	for _, test := range []struct {
		name, lower, upper string
		builtin            value.BuiltinKind
		cases              []numericByteCase
	}{
		{name: "padded-negative-integer", lower: "-00012347", upper: "-00012345", builtin: value.BuiltinInteger,
			cases: []numericByteCase{{input: "-12347"}, {input: "-00012346"}, {input: "-12345"},
				{input: "-12348", message: "minInclusive facet failed"}, {input: "-12344", message: "maxInclusive facet failed"}, {input: "-12346"}}},
		{name: "signed-zero-integer", lower: "-0000", upper: "+0001", builtin: value.BuiltinInteger,
			cases: []numericByteCase{{input: "-0000"}, {input: "+0001"}, {input: "-1", message: "minInclusive facet failed"},
				{input: "2", message: "maxInclusive facet failed"}, {input: "1.0", message: "invalid integer"}, {input: "0"}}},
		{name: "padded-decimal", lower: "0", upper: "+" + strings.Repeat("0", 1024) + "1.5000",
			cases: []numericByteCase{{input: "1.5"}, {input: "+0001.5000"}, {input: "1.5001", message: "maxInclusive facet failed"}, {input: "1.5"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			if completeErr := builder.Complete(id, value.TypeSpec{
				Variety: value.Atomic, Primitive: value.PrimitiveDecimal, Builtin: test.builtin,
				Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.NoType,
				Facets: value.FacetSpec{
					MinInclusive: value.BoundFacet{LiteralSpec: value.LiteralSpec{Lexical: test.lower, Type: value.NoType}, Present: true},
					MaxInclusive: value.BoundFacet{LiteralSpec: value.LiteralSpec{Lexical: test.upper, Type: value.NoType}, Present: true},
				},
			}); completeErr != nil {
				t.Fatal(completeErr)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			assertNumericByteCases(t, program, id, test.cases)
		})
	}
}

func TestBorrowedNumericInheritedAndExclusiveBounds(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	base, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	spec := value.TypeSpec{
		Variety: value.Atomic, Primitive: value.PrimitiveDecimal,
		Whitespace: value.WhitespaceCollapse, WhitespacePresent: true, Base: value.NoType, ListItem: value.NoType,
		Facets: value.FacetSpec{
			MinInclusive: value.BoundFacet{LiteralSpec: value.LiteralSpec{Lexical: "1", Type: value.NoType}, Present: true},
			MaxInclusive: value.BoundFacet{LiteralSpec: value.LiteralSpec{Lexical: "10", Type: value.NoType}, Present: true},
		},
	}
	if completeErr := builder.Complete(base, spec); completeErr != nil {
		t.Fatal(completeErr)
	}
	derived, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	spec.Base = base
	spec.Facets.MinInclusive.Lexical = "2"
	spec.Facets.MaxInclusive.Lexical = "8"
	if completeErr := builder.Complete(derived, spec); completeErr != nil {
		t.Fatal(completeErr)
	}
	exclusive, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	spec.Base = value.NoType
	spec.Facets = value.FacetSpec{MinExclusive: value.BoundFacet{LiteralSpec: value.LiteralSpec{Lexical: "1", Type: value.NoType}, Present: true}}
	if completeErr := builder.Complete(exclusive, spec); completeErr != nil {
		t.Fatal(completeErr)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	assertNumericByteCases(t, program, derived, []numericByteCase{
		{input: "2"}, {input: "8"}, {input: "1.5", message: "lower bound facet failed", facet: true},
		{input: "9", message: "upper bound facet failed", facet: true}, {input: "2"},
	})
	assertNumericByteCases(t, program, exclusive, []numericByteCase{
		{input: "1", message: "lower bound facet failed", facet: true}, {input: "2"},
	})
}

func assertNumericByteCases(tb testing.TB, program *value.Program, id value.TypeID, cases []numericByteCase) {
	tb.Helper()
	var scratch value.Scratch
	for _, test := range cases {
		got, err := program.ValidateBytes(id, []byte(test.input), value.Resolver{}, 0, 1<<20, &scratch)
		if test.message != "" {
			if err == nil || err.Error() != test.message || errors.Is(err, value.ErrFacet) != test.facet {
				tb.Fatalf("ValidateBytes(%q) = %v, want %q (facet=%t)", test.input, err, test.message, test.facet)
			}
			continue
		}
		if err != nil || got.Type() != id || got.SelectedType() != value.NoType || got.CanonicalText() != "" || got.IdentityKey() != "" || got.IDs() != "" || got.IDRefs() != "" || got.HasQualifiedNames() {
			tb.Fatalf("ValidateBytes(%q) = %#v, %v", test.input, got, err)
		}
	}
}

func TestPresentZeroNumericBoundAndRecovery(t *testing.T) {
	builder := value.NewBuilder(value.BuilderOptions{})
	spec := value.TypeSpec{
		Variety: value.Atomic, Primitive: value.PrimitiveDecimal,
		Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
		Base: value.NoType, ListItem: value.NoType,
	}
	unbounded, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	if err = builder.Complete(unbounded, spec); err != nil {
		t.Fatal(err)
	}
	bounded, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	spec.Facets.MaxInclusive = value.BoundFacet{
		Lexical: "-000.000", Type: value.NoType, Present: true,
	}
	if err = builder.Complete(bounded, spec); err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	assertNumericByteCases(t, program, bounded, []numericByteCase{
		{input: "-.5"}, {input: "-0"},
		{input: ".0001", message: "maxInclusive facet failed"}, {input: "0"},
	})
	assertNumericByteCases(t, program, unbounded, []numericByteCase{{input: ".0001"}})
}
