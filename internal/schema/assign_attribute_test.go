package schema

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestVisitAttributeDeclsAssignIDs(t *testing.T) {
	b := &builder{
		registry: newRegistry(),
		nextAttr: 1,
	}
	attr := &types.AttributeDecl{
		Name: types.QName{Local: "attr"},
	}

	if err := b.visitAttributeDecls([]*types.AttributeDecl{attr}); err != nil {
		t.Fatalf("visitAttributeDecls: %v", err)
	}
	if len(b.registry.LocalAttributes) != 0 {
		t.Fatalf("expected no local attribute IDs assigned")
	}

	if err := b.visitAttributeDeclsWithIDs([]*types.AttributeDecl{attr}); err != nil {
		t.Fatalf("visitAttributeDeclsWithIDs: %v", err)
	}
	if len(b.registry.LocalAttributes) != 1 {
		t.Fatalf("expected local attribute IDs to be assigned")
	}
}
