package models

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestComputeReachable(t *testing.T) {
	group := sequence(elem("a", 1, 1), elem("b", 1, 1))
	glu, err := BuildGlushkov(group)
	if err != nil {
		t.Fatalf("BuildGlushkov: %v", err)
	}
	size := len(glu.Positions)
	state := newBitset(size)

	reachable := computeReachable(state, size, glu)
	if reachable.key() != glu.firstRaw.key() {
		t.Fatalf("reachable from empty state = %q, want %q", reachable.key(), glu.firstRaw.key())
	}

	state.set(0)
	reachable = computeReachable(state, size, glu)
	if reachable.key() != glu.followRaw[0].key() {
		t.Fatalf("reachable from state[0] = %q, want %q", reachable.key(), glu.followRaw[0].key())
	}
}

func TestScanReachablePositionsErrors(t *testing.T) {
	size := 2
	reachable := newBitset(size)
	reachable.set(0)
	reachable.set(1)

	_, _, _, err := scanReachablePositions(reachable, []runtime.PosMatcher{
		{Kind: runtime.PosExact, Sym: 1, Elem: 10},
		{Kind: runtime.PosExact, Sym: 1, Elem: 20},
	}, size)
	if err == nil || !strings.Contains(err.Error(), "maps to multiple elements") {
		t.Fatalf("expected symbol conflict error, got %v", err)
	}

	_, _, _, err = scanReachablePositions(reachable, []runtime.PosMatcher{
		{Kind: runtime.PosMatchKind(99)},
		{Kind: runtime.PosExact, Sym: 2, Elem: 21},
	}, size)
	if err == nil || !strings.Contains(err.Error(), "unknown matcher kind") {
		t.Fatalf("expected unknown kind error, got %v", err)
	}
}
