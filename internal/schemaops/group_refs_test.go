package schemaops

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestExpandGroupRefsAllGroupAsChoice(t *testing.T) {
	ref := types.QName{Namespace: "urn:test", Local: "G"}
	leaf := &types.ElementDecl{
		Name:      types.QName{Namespace: "urn:test", Local: "item"},
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
	}
	groups := map[types.QName]*types.ModelGroup{
		ref: {
			Kind:      types.AllGroup,
			MinOccurs: types.OccursFromInt(1),
			MaxOccurs: types.OccursFromInt(1),
			Particles: []types.Particle{leaf},
		},
	}

	got, err := ExpandGroupRefs(&types.GroupRef{
		RefQName:  ref,
		MinOccurs: types.OccursFromInt(0),
		MaxOccurs: types.OccursFromInt(2),
	}, ExpandGroupRefsOptions{
		Lookup:       func(gr *types.GroupRef) *types.ModelGroup { return groups[gr.RefQName] },
		AllGroupMode: AllGroupAsChoice,
		LeafClone:    LeafClone,
	})
	if err != nil {
		t.Fatalf("ExpandGroupRefs error = %v", err)
	}

	group, ok := got.(*types.ModelGroup)
	if !ok || group == nil {
		t.Fatalf("ExpandGroupRefs type = %T, want *types.ModelGroup", got)
	}
	if group.Kind != types.Choice {
		t.Fatalf("expanded kind = %v, want %v", group.Kind, types.Choice)
	}
	if !group.MinOccurs.IsZero() {
		t.Fatalf("expanded minOccurs = %s, want 0", group.MinOccurs)
	}
	if !group.MaxOccurs.GreaterThanInt(1) {
		t.Fatalf("expanded maxOccurs = %s, want >1", group.MaxOccurs)
	}
	child, ok := group.Particles[0].(*types.ElementDecl)
	if !ok || child == nil {
		t.Fatalf("expanded child type = %T, want *types.ElementDecl", group.Particles[0])
	}
	if child == leaf {
		t.Fatalf("expanded child should be cloned when LeafClone is set")
	}
}

func TestExpandGroupRefsReuseLeaves(t *testing.T) {
	leaf := &types.ElementDecl{
		Name:      types.QName{Namespace: "urn:test", Local: "item"},
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
	}
	root := &types.ModelGroup{
		Kind:      types.Sequence,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
		Particles: []types.Particle{leaf},
	}

	got, err := ExpandGroupRefs(root, ExpandGroupRefsOptions{LeafClone: LeafReuse})
	if err != nil {
		t.Fatalf("ExpandGroupRefs error = %v", err)
	}
	group, ok := got.(*types.ModelGroup)
	if !ok || group == nil {
		t.Fatalf("ExpandGroupRefs type = %T, want *types.ModelGroup", got)
	}
	if group == root {
		t.Fatalf("ExpandGroupRefs should clone model-group nodes")
	}
	child, ok := group.Particles[0].(*types.ElementDecl)
	if !ok || child == nil {
		t.Fatalf("expanded child type = %T, want *types.ElementDecl", group.Particles[0])
	}
	if child != leaf {
		t.Fatalf("expanded child pointer changed with LeafReuse")
	}
}

func TestExpandGroupRefsUsesConfiguredErrors(t *testing.T) {
	cycleSentinel := errors.New("cycle")
	missingSentinel := errors.New("missing")

	cycleQName := types.QName{Namespace: "urn:test", Local: "A"}
	cycleGroup := &types.ModelGroup{
		Kind: types.Sequence,
		Particles: []types.Particle{
			&types.GroupRef{
				RefQName:  cycleQName,
				MinOccurs: types.OccursFromInt(1),
				MaxOccurs: types.OccursFromInt(1),
			},
		},
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
	}

	_, cycleErr := ExpandGroupRefs(&types.GroupRef{
		RefQName:  cycleQName,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
	}, ExpandGroupRefsOptions{
		Lookup: func(gr *types.GroupRef) *types.ModelGroup {
			if gr.RefQName == cycleQName {
				return cycleGroup
			}
			return nil
		},
		CycleError: func(ref types.QName) error {
			if ref != cycleQName {
				t.Fatalf("cycle ref = %s, want %s", ref, cycleQName)
			}
			return cycleSentinel
		},
	})
	if !errors.Is(cycleErr, cycleSentinel) {
		t.Fatalf("cycle error = %v, want %v", cycleErr, cycleSentinel)
	}

	missingQName := types.QName{Namespace: "urn:test", Local: "Missing"}
	_, missingErr := ExpandGroupRefs(&types.GroupRef{
		RefQName:  missingQName,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(1),
	}, ExpandGroupRefsOptions{
		MissingError: func(ref types.QName) error {
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
