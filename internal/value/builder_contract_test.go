package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestBuilderSealChargesFacetLiteralEvaluationWork(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{
		MaxConstructionWork: 1,
		MaxStorageBytes:     1024,
	})
	if _, err := builder.Add(value.TypeSpec{
		Variety:           value.Atomic,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespacePreserve,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.NoType,
		Facets: value.FacetSpec{Enumeration: []value.LiteralSpec{
			{Lexical: "x", Type: value.NoType},
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Seal(); !errors.Is(err, value.ErrLimit) {
		t.Fatalf("Seal() error = %v, want ErrLimit", err)
	}
}

func TestBuilderReserveChargesTypeStorage(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{MaxStorageBytes: 1})
	if _, err := builder.Reserve(); !errors.Is(err, value.ErrLimit) {
		t.Fatalf("Reserve() error = %v, want ErrLimit", err)
	}
}
