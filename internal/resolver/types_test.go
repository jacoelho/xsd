package resolver

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestValidateTypeReferenceFromTypeNil(t *testing.T) {
	schema := &parser.Schema{}
	if err := validateTypeReferenceFromType(schema, nil, types.NamespaceURI("")); err != nil {
		t.Fatalf("expected nil error for nil type, got %v", err)
	}
}
