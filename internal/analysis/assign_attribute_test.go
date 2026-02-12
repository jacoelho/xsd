package analysis

import (
	"testing"

	model "github.com/jacoelho/xsd/internal/types"
)

func TestVisitAttributeDeclsAssignIDs(t *testing.T) {
	b := &builder{
		registry: newRegistry(),
		nextAttr: 1,
	}
	attr := &model.AttributeDecl{
		Name: model.QName{Local: "attr"},
	}

	if err := b.visitAttributeDecls([]*model.AttributeDecl{attr}); err != nil {
		t.Fatalf("visitAttributeDecls: %v", err)
	}
	if len(b.registry.localAttributes) != 0 {
		t.Fatalf("expected no local attribute IDs assigned")
	}

	if err := b.visitAttributeDeclsWithIDs([]*model.AttributeDecl{attr}); err != nil {
		t.Fatalf("visitAttributeDeclsWithIDs: %v", err)
	}
	if len(b.registry.localAttributes) != 1 {
		t.Fatalf("expected local attribute IDs to be assigned")
	}
}
