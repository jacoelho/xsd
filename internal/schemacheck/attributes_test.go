package schemacheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestAttributeUseProhibitedDisallowsDefault(t *testing.T) {
	qname := types.QName{Namespace: "urn:test", Local: "a"}
	decl := &types.AttributeDecl{
		Name:       qname,
		Use:        types.Prohibited,
		HasDefault: true,
		Default:    "d",
	}
	schema := &parser.Schema{
		TargetNamespace: "urn:test",
		AttributeDecls: map[types.QName]*types.AttributeDecl{
			qname: decl,
		},
	}

	errs := ValidateStructure(schema)
	if len(errs) == 0 {
		t.Fatalf("expected schema validation errors")
	}
	if !strings.Contains(errs[0].Error(), "use='prohibited'") {
		t.Fatalf("expected prohibited-use error, got %v", errs[0])
	}
}

func TestAttributeUseProhibitedAllowsFixed(t *testing.T) {
	qname := types.QName{Namespace: "urn:test", Local: "a"}
	decl := &types.AttributeDecl{
		Name:     qname,
		Use:      types.Prohibited,
		HasFixed: true,
		Fixed:    "x",
	}
	schema := &parser.Schema{
		TargetNamespace: "urn:test",
		AttributeDecls: map[types.QName]*types.AttributeDecl{
			qname: decl,
		},
	}

	if errs := ValidateStructure(schema); len(errs) != 0 {
		t.Fatalf("expected schema to be valid, got %v", errs)
	}
}
