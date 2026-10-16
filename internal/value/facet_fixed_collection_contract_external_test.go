package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"
)

func TestCollectionFacetsCannotBeFixedAndCompletionCanRetry(t *testing.T) {
	t.Parallel()
	pattern, err := xsdregex.Compile("x", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name    string
		facets  value.FacetSpec
		message string
	}{
		{
			name:    "pattern",
			facets:  value.FacetSpec{Patterns: [][]*value.Pattern{{pattern}}, Fixed: value.FacetPattern},
			message: "pattern facet failed",
		},
		{
			name:    "enumeration",
			facets:  value.FacetSpec{Enumeration: []value.LiteralSpec{{Lexical: "x", Type: value.NoType}}, Fixed: value.FacetEnumeration},
			message: "enumeration facet failed",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			builder := value.NewBuilder(value.BuilderOptions{})
			id, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			spec := atomicFacetType(value.PrimitiveString, test.facets)
			err = builder.Complete(id, spec)
			if !errors.Is(err, value.ErrFacet) || err.Error() != "pattern and enumeration facets cannot be fixed" {
				t.Fatalf("Complete() = %v; want fixed collection facet rejection", err)
			}
			if _, available := builder.InputRequirements(id); available {
				t.Fatal("failed completion exposed type metadata")
			}
			spec.Facets.Fixed = 0
			if completionErr := builder.Complete(id, spec); completionErr != nil {
				t.Fatalf("Complete() retry = %v", completionErr)
			}
			program, err := builder.Seal()
			if err != nil {
				t.Fatal(err)
			}
			if _, validationErr := program.Validate(id, "x", value.Resolver{}, 0, 16<<20, nil); validationErr != nil {
				t.Fatalf("Validate(x) = %v", validationErr)
			}
			_, err = program.Validate(id, "y", value.Resolver{}, 0, 16<<20, nil)
			if !errors.Is(err, value.ErrFacet) || err.Error() != test.message {
				t.Fatalf("Validate(y) = %v; want %q", err, test.message)
			}
		})
	}
}

func TestFixedWhitespaceRejectsStrengtheningAndCompletionCanRetry(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{})
	baseID, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	base := atomicFacetType(value.PrimitiveString, value.FacetSpec{Fixed: value.FacetWhiteSpace})
	base.Whitespace = value.WhitespacePreserve
	if completionErr := builder.Complete(baseID, base); completionErr != nil {
		t.Fatal(completionErr)
	}
	derivedID, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	derived := atomicFacetType(value.PrimitiveString, value.FacetSpec{})
	derived.Base = baseID
	err = builder.Complete(derivedID, derived)
	if !errors.Is(err, value.ErrFacet) || err.Error() != "fixed whiteSpace facet cannot change" {
		t.Fatalf("Complete() = %v; want fixed whiteSpace rejection", err)
	}
	if _, available := builder.InputRequirements(derivedID); available {
		t.Fatal("failed completion exposed type metadata")
	}
	derived.Whitespace = value.WhitespacePreserve
	if completionErr := builder.Complete(derivedID, derived); completionErr != nil {
		t.Fatalf("Complete() retry = %v", completionErr)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	const lexical = " \ta\n "
	result, err := program.Validate(derivedID, lexical, value.Resolver{}, value.NeedCanonical, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.CanonicalText() != lexical || result.Type() != derivedID || result.SelectedType() != value.NoType {
		t.Fatalf("Validate() = canonical %q, type %d, selection %d; want %q, %d, NoType", result.CanonicalText(), result.Type(), result.SelectedType(), lexical, derivedID)
	}
}
