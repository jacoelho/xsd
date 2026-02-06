package semanticcheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestValidateSequenceRestrictionSkipsOptionalBase(t *testing.T) {
	baseChildren := []types.Particle{
		makeElement("a", 1),
		makeElement("b", 0),
		makeElement("c", 1),
	}
	restrictionChildren := []types.Particle{
		makeElement("a", 1),
		makeElement("c", 1),
	}

	if err := validateSequenceRestriction(&parser.Schema{}, baseChildren, restrictionChildren); err != nil {
		t.Fatalf("expected sequence restriction to succeed, got %v", err)
	}
}

func TestValidateSequenceRestrictionMissingRequiredBase(t *testing.T) {
	baseChildren := []types.Particle{
		makeElement("a", 1),
		makeElement("b", 1),
	}
	restrictionChildren := []types.Particle{
		makeElement("a", 1),
	}

	if err := validateSequenceRestriction(&parser.Schema{}, baseChildren, restrictionChildren); err == nil {
		t.Fatal("expected sequence restriction to fail for missing required base particle")
	}
}

func TestValidateChoiceRestrictionMatchesBase(t *testing.T) {
	baseChildren := []types.Particle{
		makeElement("a", 1),
		makeElement("b", 1),
	}
	restrictionChildren := []types.Particle{
		makeElement("b", 1),
	}

	if err := validateChoiceRestriction(&parser.Schema{}, baseChildren, restrictionChildren); err != nil {
		t.Fatalf("expected choice restriction to succeed, got %v", err)
	}
}

func TestValidateSingleWildcardGroupRestriction(t *testing.T) {
	baseMG := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
		Particles: []types.Particle{
			&types.AnyElement{
				MinOccurs: types.OccursFromInt(1),
				MaxOccurs: types.OccursFromInt(1),
				Namespace: types.NSCAny,
			},
		},
	}
	restrictionMG := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
		Particles: []types.Particle{
			makeElement("a", 1),
		},
	}

	if err := validateSingleWildcardGroupRestriction(&parser.Schema{}, baseMG, restrictionMG); err != nil {
		t.Fatalf("expected wildcard group restriction to succeed, got %v", err)
	}
}

func makeElement(local string, minOccurs int) *types.ElementDecl {
	return &types.ElementDecl{
		Name:      types.QName{Local: local},
		MinOccurs: types.OccursFromInt(minOccurs),
		MaxOccurs: types.OccursFromInt(1),
	}
}
