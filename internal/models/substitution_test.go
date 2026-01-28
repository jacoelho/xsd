package models

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestExpandSubstitutionSequence(t *testing.T) {
	head := elem("head", 1, 1)
	head.IsReference = true
	member1 := elem("m1", 1, 1)
	member2 := elem("m2", 1, 1)
	tail := elem("tail", 1, 1)
	group := sequence(head, tail)

	glu, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}

	expanded, err := ExpandSubstitution(glu, nil, func(h *types.ElementDecl) []*types.ElementDecl {
		if h == head {
			return []*types.ElementDecl{member1, member2}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ExpandSubstitution: %v", err)
	}
	if len(expanded.Positions) != 4 {
		t.Fatalf("positions = %d, want 4", len(expanded.Positions))
	}
	if expanded.Positions[0].Element != head || expanded.Positions[1].Element != member1 || expanded.Positions[2].Element != member2 || expanded.Positions[3].Element != tail {
		t.Fatalf("position order mismatch")
	}

	if got := bitsetPositions(expanded.Bitsets, expanded.First); len(got) != 3 {
		t.Fatalf("firstPos = %v, want 3 entries", got)
	}
	if got := bitsetPositions(expanded.Bitsets, expanded.Last); len(got) != 1 || got[0] != 3 {
		t.Fatalf("lastPos = %v, want [3]", got)
	}
	for i := range 3 {
		if got := bitsetPositions(expanded.Bitsets, expanded.Follow[i]); len(got) != 1 || got[0] != 3 {
			t.Fatalf("follow[%d] = %v, want [3]", i, got)
		}
	}
}

func TestExpandSubstitutionBlockSubstitution(t *testing.T) {
	head := elem("head", 1, 1)
	head.IsReference = true
	head.Block = head.Block.Add(types.DerivationSubstitution)
	member := elem("m1", 1, 1)
	group := sequence(head)

	glu, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}

	expanded, err := ExpandSubstitution(glu, nil, func(h *types.ElementDecl) []*types.ElementDecl {
		if h == head {
			return []*types.ElementDecl{member}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ExpandSubstitution: %v", err)
	}
	if len(expanded.Positions) != 1 {
		t.Fatalf("positions = %d, want 1", len(expanded.Positions))
	}
	if expanded.Positions[0].Element != head {
		t.Fatalf("expected head to remain")
	}
}

func TestExpandSubstitutionAbstractHead(t *testing.T) {
	head := elem("head", 1, 1)
	head.IsReference = true
	head.Abstract = true
	member := elem("m1", 1, 1)
	group := sequence(head)

	glu, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}

	expanded, err := ExpandSubstitution(glu, nil, func(h *types.ElementDecl) []*types.ElementDecl {
		if h == head {
			return []*types.ElementDecl{member}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ExpandSubstitution: %v", err)
	}
	if len(expanded.Positions) != 1 {
		t.Fatalf("positions = %d, want 1", len(expanded.Positions))
	}
	if expanded.Positions[0].Element != member {
		t.Fatalf("expected member to replace abstract head")
	}
}

func TestExpandSubstitutionBlockExtension(t *testing.T) {
	baseType := &types.ComplexType{QName: types.QName{Local: "HeadType"}}
	head := elem("head", 1, 1)
	head.IsReference = true
	head.Type = baseType
	head.Abstract = true
	head.Block = head.Block.Add(types.DerivationExtension)

	memberType := &types.ComplexType{QName: types.QName{Local: "MemberType"}}
	memberType.ResolvedBase = baseType
	memberType.DerivationMethod = types.DerivationExtension
	member := elem("m1", 1, 1)
	member.Type = memberType

	group := sequence(head)
	glu, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}

	_, err = ExpandSubstitution(glu, nil, func(h *types.ElementDecl) []*types.ElementDecl {
		if h == head {
			return []*types.ElementDecl{member}
		}
		return nil
	})
	if err == nil {
		t.Fatalf("expected error for blocked substitution")
	}
}

func TestExpandSubstitutionAbstractBlocked(t *testing.T) {
	head := elem("head", 1, 1)
	head.IsReference = true
	head.Abstract = true
	head.Block = head.Block.Add(types.DerivationSubstitution)
	group := sequence(head)

	glu, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}

	_, err = ExpandSubstitution(glu, nil, func(h *types.ElementDecl) []*types.ElementDecl {
		return nil
	})
	if err == nil {
		t.Fatalf("expected error for abstract head with substitution blocked")
	}
}

func TestExpandSubstitutionAbstractNoMembers(t *testing.T) {
	head := elem("head", 1, 1)
	head.IsReference = true
	head.Abstract = true
	group := sequence(head)

	glu, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}

	expanded, err := ExpandSubstitution(glu, nil, func(h *types.ElementDecl) []*types.ElementDecl {
		return nil
	})
	if err != nil {
		t.Fatalf("ExpandSubstitution: %v", err)
	}
	if len(expanded.Positions) != 0 {
		t.Fatalf("expected no positions after substitution expansion, got %d", len(expanded.Positions))
	}
	if expanded.Nullable {
		t.Fatalf("expected non-nullable model for required abstract head")
	}
}
