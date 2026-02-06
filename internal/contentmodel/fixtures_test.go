package contentmodel

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
)

type modelFixture struct {
	particle  types.Particle
	name      string
	wantError string
	matchers  []runtime.PosMatcher
	wantPos   int
	limits    Limits
	wantKind  runtime.ModelKind
	nullable  bool
}

func TestContentModelFixtures(t *testing.T) {
	fixtures := []modelFixture{
		{
			name:     "sequence-choice-optional",
			particle: sequence(elem("a", 1, 1), choice(1, 1, elem("b", 1, 1), elem("c", 1, 1)), elem("d", 0, 1)),
			matchers: []runtime.PosMatcher{
				{Kind: runtime.PosExact, Sym: 1, Elem: 11},
				{Kind: runtime.PosExact, Sym: 2, Elem: 12},
				{Kind: runtime.PosExact, Sym: 3, Elem: 13},
				{Kind: runtime.PosExact, Sym: 4, Elem: 14},
			},
			limits:   Limits{MaxDFAStates: 64},
			wantKind: runtime.ModelDFA,
			wantPos:  4,
		},
		{
			name:     "mixed-uses-same-particle-model",
			particle: sequence(elem("a", 1, 1)),
			matchers: []runtime.PosMatcher{{Kind: runtime.PosExact, Sym: 1, Elem: 11}},
			limits:   Limits{MaxDFAStates: 16},
			wantKind: runtime.ModelDFA,
			wantPos:  1,
		},
		{
			name:      "all-group-rejected",
			particle:  &types.ModelGroup{Kind: types.AllGroup, MinOccurs: types.OccursFromInt(1), MaxOccurs: types.OccursFromInt(1)},
			wantError: "all group",
		},
		{
			name:     "empty-content",
			particle: nil,
			wantKind: runtime.ModelNone,
			nullable: true,
		},
		{
			name:     "nfa-fallback",
			particle: sequence(elem("a", 1, 1), elem("b", 1, 1)),
			matchers: []runtime.PosMatcher{{Kind: runtime.PosExact, Sym: 1, Elem: 11}, {Kind: runtime.PosExact, Sym: 2, Elem: 12}},
			limits:   Limits{MaxDFAStates: 1},
			wantKind: runtime.ModelNFA,
			wantPos:  2,
		},
	}

	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			glu, err := BuildGlushkov(fx.particle)
			if fx.wantError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", fx.wantError)
				}
				if !strings.Contains(err.Error(), fx.wantError) {
					t.Fatalf("error = %q, want contains %q", err.Error(), fx.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildGlushkov: %v", err)
			}
			if fx.wantPos > 0 && len(glu.Positions) != fx.wantPos {
				t.Fatalf("positions = %d, want %d", len(glu.Positions), fx.wantPos)
			}
			if fx.nullable != glu.Nullable {
				t.Fatalf("nullable = %v, want %v", glu.Nullable, fx.nullable)
			}

			model, err := Compile(glu, fx.matchers, fx.limits)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if model.Kind != fx.wantKind {
				t.Fatalf("model kind = %v, want %v", model.Kind, fx.wantKind)
			}
		})
	}
}
