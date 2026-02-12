package grouprefs

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/occurs"
)

func TestExpandGroupRefsAllGroupAsChoice(t *testing.T) {
	ref := model.QName{Namespace: "urn:test", Local: "G"}
	leaf := &model.ElementDecl{
		Name:      model.QName{Namespace: "urn:test", Local: "item"},
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
	}
	groups := map[model.QName]*model.ModelGroup{
		ref: {
			Kind:      model.AllGroup,
			MinOccurs: occurs.OccursFromInt(1),
			MaxOccurs: occurs.OccursFromInt(1),
			Particles: []model.Particle{leaf},
		},
	}

	got, err := ExpandGroupRefs(&model.GroupRef{
		RefQName:  ref,
		MinOccurs: occurs.OccursFromInt(0),
		MaxOccurs: occurs.OccursFromInt(2),
	}, ExpandGroupRefsOptions{
		Lookup:       func(gr *model.GroupRef) *model.ModelGroup { return groups[gr.RefQName] },
		AllGroupMode: AllGroupAsChoice,
		LeafClone:    LeafClone,
	})
	if err != nil {
		t.Fatalf("ExpandGroupRefs error = %v", err)
	}

	group, ok := got.(*model.ModelGroup)
	if !ok || group == nil {
		t.Fatalf("ExpandGroupRefs type = %T, want *model.ModelGroup", got)
	}
	if group.Kind != model.Choice {
		t.Fatalf("expanded kind = %v, want %v", group.Kind, model.Choice)
	}
	if !group.MinOccurs.IsZero() {
		t.Fatalf("expanded minOccurs = %s, want 0", group.MinOccurs)
	}
	if !group.MaxOccurs.GreaterThanInt(1) {
		t.Fatalf("expanded maxOccurs = %s, want >1", group.MaxOccurs)
	}
	child, ok := group.Particles[0].(*model.ElementDecl)
	if !ok || child == nil {
		t.Fatalf("expanded child type = %T, want *model.ElementDecl", group.Particles[0])
	}
	if child == leaf {
		t.Fatalf("expanded child should be cloned when LeafClone is set")
	}
}

func TestExpandGroupRefsReuseLeaves(t *testing.T) {
	leaf := &model.ElementDecl{
		Name:      model.QName{Namespace: "urn:test", Local: "item"},
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
	}
	root := &model.ModelGroup{
		Kind:      model.Sequence,
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
		Particles: []model.Particle{leaf},
	}

	got, err := ExpandGroupRefs(root, ExpandGroupRefsOptions{LeafClone: LeafReuse})
	if err != nil {
		t.Fatalf("ExpandGroupRefs error = %v", err)
	}
	group, ok := got.(*model.ModelGroup)
	if !ok || group == nil {
		t.Fatalf("ExpandGroupRefs type = %T, want *model.ModelGroup", got)
	}
	if group == root {
		t.Fatalf("ExpandGroupRefs should clone model-group nodes")
	}
	child, ok := group.Particles[0].(*model.ElementDecl)
	if !ok || child == nil {
		t.Fatalf("expanded child type = %T, want *model.ElementDecl", group.Particles[0])
	}
	if child != leaf {
		t.Fatalf("expanded child pointer changed with LeafReuse")
	}
}

func TestExpandGroupRefsUsesConfiguredErrors(t *testing.T) {
	cycleSentinel := errors.New("cycle")
	missingSentinel := errors.New("missing")

	cycleQName := model.QName{Namespace: "urn:test", Local: "A"}
	cycleGroup := &model.ModelGroup{
		Kind: model.Sequence,
		Particles: []model.Particle{
			&model.GroupRef{
				RefQName:  cycleQName,
				MinOccurs: occurs.OccursFromInt(1),
				MaxOccurs: occurs.OccursFromInt(1),
			},
		},
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
	}

	_, cycleErr := ExpandGroupRefs(&model.GroupRef{
		RefQName:  cycleQName,
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
	}, ExpandGroupRefsOptions{
		Lookup: func(gr *model.GroupRef) *model.ModelGroup {
			if gr.RefQName == cycleQName {
				return cycleGroup
			}
			return nil
		},
		CycleError: func(ref model.QName) error {
			if ref != cycleQName {
				t.Fatalf("cycle ref = %s, want %s", ref, cycleQName)
			}
			return cycleSentinel
		},
	})
	if !errors.Is(cycleErr, cycleSentinel) {
		t.Fatalf("cycle error = %v, want %v", cycleErr, cycleSentinel)
	}

	missingQName := model.QName{Namespace: "urn:test", Local: "Missing"}
	_, missingErr := ExpandGroupRefs(&model.GroupRef{
		RefQName:  missingQName,
		MinOccurs: occurs.OccursFromInt(1),
		MaxOccurs: occurs.OccursFromInt(1),
	}, ExpandGroupRefsOptions{
		MissingError: func(ref model.QName) error {
			if ref != missingQName {
				t.Fatalf("missing ref = %s, want %s", ref, missingQName)
			}
			return missingSentinel
		},
	})
	if !errors.Is(missingErr, missingSentinel) {
		t.Fatalf("missing error = %v, want %v", missingErr, missingSentinel)
	}
	if strings.TrimSpace(missingErr.Error()) == "" {
		t.Fatalf("missing error should be non-empty")
	}
}
