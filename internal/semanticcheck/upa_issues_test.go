package semanticcheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestValidateUPA_DoesNotMutateOccurs(t *testing.T) {
	schema := &parser.Schema{
		TargetNamespace:    types.NamespaceURI("urn:test"),
		SubstitutionGroups: map[types.QName][]types.QName{},
	}

	elem := &types.ElementDecl{
		Name:      types.QName{Namespace: schema.TargetNamespace, Local: "item"},
		MinOccurs: types.OccursFromInt(0),
		MaxOccurs: types.OccursFromInt(2),
	}
	group := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
		Particles: []types.Particle{elem},
	}
	content := &types.ElementContent{Particle: group}

	if err := ValidateUPA(schema, content, schema.TargetNamespace); err != nil {
		t.Fatalf("ValidateUPA error = %v", err)
	}
	if elem.MinOcc().CmpInt(0) != 0 {
		t.Fatalf("element minOccurs mutated = %s, want 0", elem.MinOcc())
	}
	if elem.MaxOcc().CmpInt(2) != 0 {
		t.Fatalf("element maxOccurs mutated = %s, want 2", elem.MaxOcc())
	}
}

func TestValidateUPA_DuplicateElementRefs(t *testing.T) {
	schema := &parser.Schema{
		TargetNamespace:    types.NamespaceURI("urn:test"),
		SubstitutionGroups: map[types.QName][]types.QName{},
	}

	name := types.QName{Namespace: schema.TargetNamespace, Local: "a"}
	elem1 := &types.ElementDecl{
		Name:      name,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
	}
	elem2 := &types.ElementDecl{
		Name:      name,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
	}
	choice := &types.ModelGroup{
		Kind:      types.Choice,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
		Particles: []types.Particle{elem1, elem2},
	}
	content := &types.ElementContent{Particle: choice}

	if err := ValidateUPA(schema, content, schema.TargetNamespace); err == nil {
		t.Fatalf("expected UPA error for duplicate element references")
	}
}
