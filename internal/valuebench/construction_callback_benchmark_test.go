package valuebench

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func BenchmarkReentrantQNameConstruction(b *testing.B) {
	childSpec := value.TypeSpec{Variety: value.Atomic, Primitive: value.PrimitiveString, Base: value.NoType, ListItem: value.NoType}
	build := func() (*value.Program, value.TypeID, value.TypeID, error) {
		builder := value.NewBuilder(value.BuilderOptions{})
		if err := builder.ReserveCapacity(2); err != nil {
			return nil, value.NoType, value.NoType, err
		}
		parent, err := builder.Reserve()
		if err != nil {
			return nil, value.NoType, value.NoType, err
		}
		child, err := builder.Reserve()
		if err != nil {
			return nil, value.NoType, value.NoType, err
		}
		var callbackErr error
		parentSpec := value.TypeSpec{
			Variety: value.Atomic, Primitive: value.PrimitiveQName, Base: value.NoType, ListItem: value.NoType,
			Facets: value.FacetSpec{Enumeration: []value.LiteralSpec{{Type: value.NoType, Lexical: "p:x", Resolver: value.Resolver{
				QName: func(string) (value.ExpandedName, bool) {
					callbackErr = builder.Complete(child, childSpec)
					return value.ExpandedName{Namespace: "urn:construction", Local: "x"}, callbackErr == nil
				},
			}}}},
		}
		completionErr := builder.Complete(parent, parentSpec)
		if callbackErr != nil {
			return nil, parent, child, callbackErr
		}
		if completionErr != nil {
			return nil, parent, child, completionErr
		}
		program, err := builder.Seal()
		return program, parent, child, err
	}
	check := func(program *value.Program, parent, child value.TypeID) {
		resolver := value.Resolver{QName: func(lexical string) (value.ExpandedName, bool) {
			switch lexical {
			case "p:x":
				return value.ExpandedName{Namespace: "urn:construction", Local: "x"}, true
			case "p:y":
				return value.ExpandedName{Namespace: "urn:construction", Local: "y"}, true
			}
			return value.ExpandedName{}, false
		}}
		if _, err := program.Validate(parent, "p:x", resolver, 0, 16<<20, nil); err != nil {
			b.Fatal(err)
		}
		_, err := program.Validate(parent, "p:y", resolver, 0, 16<<20, nil)
		if !errors.Is(err, value.ErrFacet) || err.Error() != "enumeration facet failed" {
			b.Fatalf("Validate(p:y) = %v; want enumeration rejection", err)
		}
		got, err := program.Validate(child, "child", value.Resolver{}, value.NeedCanonical, 16<<20, nil)
		if err != nil || got.Type() != child || got.CanonicalText() != "child" {
			b.Fatalf("child validation = %v, %v", got, err)
		}
	}
	program, parent, child, err := build()
	if err != nil {
		b.Fatal(err)
	}
	check(program, parent, child)
	b.ReportAllocs()
	for b.Loop() {
		program, parent, child, err = build()
		if err != nil {
			b.Fatal(err)
		}
	}
	check(program, parent, child)
}
