package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestBuilderCompleteChargesFacetLiteralEvaluationWork(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{
		MaxConstructionWork: 1,
		MaxStorageBytes:     1024,
	})
	reservedID, reserveErr := builder.Reserve()
	if reserveErr != nil {
		t.Fatalf("Reserve() error = %v", reserveErr)
	}
	if err := builder.Complete(reservedID, value.TypeSpec{
		Variety:           value.Atomic,
		Primitive:         value.PrimitiveString,
		Whitespace:        value.WhitespacePreserve,
		WhitespacePresent: true,
		Base:              value.NoType,
		ListItem:          value.NoType,
		Facets: value.FacetSpec{Enumeration: []value.LiteralSpec{
			{Lexical: "x", Type: value.NoType},
		}},
	}); !errors.Is(err, value.ErrLimit) {
		t.Fatalf("Complete() error = %v, want ErrLimit", err)
	}
}

func TestBuilderReserveChargesTypeStorage(t *testing.T) {
	t.Parallel()

	builder := value.NewBuilder(value.BuilderOptions{MaxStorageBytes: 1})
	if _, err := builder.Reserve(); !errors.Is(err, value.ErrLimit) {
		t.Fatalf("Reserve() error = %v, want ErrLimit", err)
	}
}
