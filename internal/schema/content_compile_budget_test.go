package schema

import (
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestCheckContentModelsUPAChargesSharedModelForEachRoot(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	rt := compiledModelRuntimeStub{models: map[ContentModelID]ContentModel{
		0: {Kind: ModelChoice, Occurs: one, Particles: []Particle{ModelParticle(2, one)}},
		1: {Kind: ModelChoice, Occurs: one, Particles: []Particle{ModelParticle(2, one)}},
		2: {Kind: ModelChoice, Occurs: one, Particles: []Particle{ModelParticle(3, one)}},
		3: {Kind: ModelEmpty},
	}}
	// Root traversal charges are 6+6+4+2: each of roots 0 and 1 visits
	// model 2 and then model 3, root 2 visits model 3, and root 3 is empty.
	// All direct choice checks have one particle and therefore no pairwise work.
	for _, tt := range []struct {
		name  string
		limit int
		err   bool
	}{
		{name: "exact", limit: 18},
		{name: "one short", limit: 17, err: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			work := newWorkBudget(contentModelWorkBudget, tt.limit)
			analysis := mustContentModelAnalysis(t, rt, work.spend)
			err := CheckContentModelsUPA(nil, rt, 4, &work, analysis)
			if tt.err {
				expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
				return
			}
			if err != nil {
				t.Fatalf("CheckContentModelsUPA() error = %v", err)
			}
		})
	}
}

func TestDFAFollowStateLookupChargesCacheHits(t *testing.T) {
	t.Parallel()

	newBuilder := func(limit int) *dfaBuilder {
		work := newWorkBudget(contentModelWorkBudget, limit)
		b := &dfaBuilder{
			c:         &contentModelCompiler{work: &work},
			follow:    map[int][]dfaEntry{0: {{Pos: 0}}},
			states:    make(map[string]uint32),
			positions: []Particle{ElementParticle(1, Occurrence{Min: 1, Max: 1})},
			limit:     1,
		}
		return b
	}

	// Normalization charges one follow-map entry plus one entry admission (3).
	// Each row edge charges itself plus one follow lookup (3), so one lookup
	// costs 6 and a repeated lookup costs 9; the second case must fail at 8.
	t.Run("first lookup charges normalized admission once", func(t *testing.T) {
		t.Parallel()
		b := newBuilder(6)
		if err := b.normalizeFollows(); err != nil {
			t.Fatalf("normalizeFollows() error = %v", err)
		}
		if _, err := b.row([]dfaEntry{{Pos: 0}}); err != nil {
			t.Fatalf("row() error = %v", err)
		}
	})

	t.Run("repeated lookup charges the hit", func(t *testing.T) {
		t.Parallel()
		b := newBuilder(8)
		if err := b.normalizeFollows(); err != nil {
			t.Fatalf("normalizeFollows() error = %v", err)
		}
		if _, err := b.row([]dfaEntry{{Pos: 0}}); err != nil {
			t.Fatalf("first row() error = %v", err)
		}
		_, err := b.row([]dfaEntry{{Pos: 0}})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	})
}
