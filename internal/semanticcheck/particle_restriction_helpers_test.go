package semanticcheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
	parser "github.com/jacoelho/xsd/internal/parser"
)

func TestValidateSequenceRestrictionSkipsOptionalBase(t *testing.T) {
	baseChildren := []model.Particle{
		makeElement("a", 1),
		makeElement("b", 0),
		makeElement("c", 1),
	}
	restrictionChildren := []model.Particle{
		makeElement("a", 1),
		makeElement("c", 1),
	}

	if err := validateSequenceRestriction(&parser.Schema{}, baseChildren, restrictionChildren); err != nil {
		t.Fatalf("expected sequence restriction to succeed, got %v", err)
	}
}

func TestValidateSequenceRestrictionMissingRequiredBase(t *testing.T) {
	baseChildren := []model.Particle{
		makeElement("a", 1),
		makeElement("b", 1),
	}
	restrictionChildren := []model.Particle{
		makeElement("a", 1),
	}

	if err := validateSequenceRestriction(&parser.Schema{}, baseChildren, restrictionChildren); err == nil {
		t.Fatal("expected sequence restriction to fail for missing required base particle")
	}
}

func TestValidateChoiceRestrictionMatchesBase(t *testing.T) {
	baseChildren := []model.Particle{
		makeElement("a", 1),
		makeElement("b", 1),
	}
	restrictionChildren := []model.Particle{
		makeElement("b", 1),
	}

	if err := validateChoiceRestriction(&parser.Schema{}, baseChildren, restrictionChildren); err != nil {
		t.Fatalf("expected choice restriction to succeed, got %v", err)
	}
}

func TestValidateSingleWildcardGroupRestriction(t *testing.T) {
	baseMG := &model.ModelGroup{
		Kind:      model.Sequence,
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
		Particles: []model.Particle{
			&model.AnyElement{
				MinOccurs: occurs.OccursFromInt(1),
				MaxOccurs: occurs.OccursFromInt(1),
				Namespace: model.NSCAny,
			},
		},
	}
	restrictionMG := &model.ModelGroup{
		Kind:      model.Sequence,
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
		Particles: []model.Particle{
			makeElement("a", 1),
		},
	}

	if err := validateSingleWildcardGroupRestriction(&parser.Schema{}, baseMG, restrictionMG); err != nil {
		t.Fatalf("expected wildcard group restriction to succeed, got %v", err)
	}
}

func makeElement(local string, minOccurs int) *model.ElementDecl {
	return &model.ElementDecl{
		Name:      model.QName{Local: local},
		MinOccurs: occurs.OccursFromInt(minOccurs),
		MaxOccurs: occurs.OccursFromInt(1),
	}
}
