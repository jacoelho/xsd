package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestValidateSchemaInput(t *testing.T) {
	if err := validateSchemaInput(nil); err == nil {
		t.Fatalf("expected nil schema to error")
	}

	schema := parser.NewSchema()
	if err := validateSchemaInput(schema); err != nil {
		t.Fatalf("unexpected error for empty schema: %v", err)
	}

	schema.ElementDecls[types.QName{Local: "root"}] = &types.ElementDecl{}
	if err := validateSchemaInput(schema); err == nil {
		t.Fatalf("expected missing global declaration order error")
	}
}
