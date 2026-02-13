package semanticcheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestValidateUPA_DoesNotMutateOccurs(t *testing.T) {
	schema := &parser.Schema{
		TargetNamespace:    model.NamespaceURI("urn:test"),
		SubstitutionGroups: map[model.QName][]model.QName{},
	}

	elem := &model.ElementDecl{
		Name:      model.QName{Namespace: schema.TargetNamespace, Local: "item"},
		MinOccurs: occurs.OccursFromInt(0),
		MaxOccurs: occurs.OccursFromInt(2),
	}
	group := &model.ModelGroup{
		Kind:      model.Sequence,
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
		Particles: []model.Particle{elem},
	}
	content := &model.ElementContent{Particle: group}

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
		TargetNamespace:    model.NamespaceURI("urn:test"),
		SubstitutionGroups: map[model.QName][]model.QName{},
	}

	name := model.QName{Namespace: schema.TargetNamespace, Local: "a"}
	elem1 := &model.ElementDecl{
		Name:      name,
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
	}
	elem2 := &model.ElementDecl{
		Name:      name,
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
	}
	choice := &model.ModelGroup{
		Kind:      model.Choice,
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
		Particles: []model.Particle{elem1, elem2},
	}
	content := &model.ElementContent{Particle: choice}

	if err := ValidateUPA(schema, content, schema.TargetNamespace); err == nil {
		t.Fatalf("expected UPA error for duplicate element references")
	}
}
