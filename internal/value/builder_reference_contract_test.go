package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestBuilderCoreReferenceAdmission(t *testing.T) {
	tests := []struct {
		name    string
		variety value.Variety
		late    string
	}{
		{name: "atomic self base", variety: value.Atomic, late: "self"},
		{name: "list self item", variety: value.List, late: "self"},
		{name: "atomic unreserved base", variety: value.Atomic, late: "unreserved"},
		{name: "list unreserved item", variety: value.List, late: "unreserved"},
		{name: "deep union then absent", variety: value.Union, late: "absent"},
		{name: "deep union then unreserved", variety: value.Union, late: "unreserved"},
		{name: "deep union then incomplete", variety: value.Union, late: "incomplete"},
		{name: "deep union then self", variety: value.Union, late: "self"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := value.NewBuilder(value.BuilderOptions{MaxDepth: 1})
			completed, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			atomic := value.TypeSpec{
				Variety: value.Atomic, Primitive: value.PrimitiveString,
				Base: value.NoType, ListItem: value.NoType,
			}
			if completionErr := builder.Complete(completed, atomic); completionErr != nil {
				t.Fatal(completionErr)
			}
			target, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			incomplete, err := builder.Reserve()
			if err != nil {
				t.Fatal(err)
			}
			ref := target
			switch test.late {
			case "absent":
				ref = value.NoType
			case "unreserved":
				ref = incomplete + 1
			case "incomplete":
				ref = incomplete
			}
			spec := value.TypeSpec{
				Variety: test.variety, Primitive: value.PrimitiveString,
				Base: value.NoType, ListItem: value.NoType,
			}
			switch test.variety {
			case value.Atomic:
				spec.Base = ref
			case value.List:
				spec.ListItem = ref
			case value.Union:
				spec.Union = []value.TypeID{completed, ref}
			}
			if err := builder.Complete(target, spec); !errors.Is(err, value.ErrMetadata) {
				t.Fatalf("Complete() error = %v, want ErrMetadata before any depth error", err)
			}
			if _, available := builder.InputRequirements(target); available {
				t.Fatal("rejected type became available")
			}
			if test.name == "atomic self base" {
				if err := builder.Complete(target, atomic); err != nil {
					t.Fatalf("Complete() retry error = %v", err)
				}
				got, err := builder.Validate(target, "x", value.Resolver{}, value.NeedCanonical, nil)
				if err != nil || got.CanonicalText() != "x" {
					t.Fatalf("Validate() after retry = %q, %v, want x, nil", got.CanonicalText(), err)
				}
			}
		})
	}
}

func TestBuilderCoreDepthPrecedesFacetReferenceAdmission(t *testing.T) {
	t.Parallel()
	builder := value.NewBuilder(value.BuilderOptions{MaxDepth: 1})
	base, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	decimal := atomicFacetType(value.PrimitiveDecimal, value.FacetSpec{})
	if completionErr := builder.Complete(base, decimal); completionErr != nil {
		t.Fatal(completionErr)
	}
	derived, err := builder.Reserve()
	if err != nil {
		t.Fatal(err)
	}
	decimal.Base = base
	decimal.Facets.MinInclusive = value.BoundFacet{
		Present: true, Type: derived + 1, Lexical: "1",
	}
	if completionErr := builder.Complete(derived, decimal); !errors.Is(completionErr, value.ErrLimit) {
		t.Fatalf("Complete = %v; want core depth limit before invalid facet reference", completionErr)
	}
	if _, available := builder.TypeView(derived); available {
		t.Fatal("rejected type exposed metadata")
	}
}
