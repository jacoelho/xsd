package schema

import (
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestCheckContentModelsUPAChargesSharedModelForEachRoot(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	rt := compiledModelFixture{models: map[contentModelID]contentModelNode{
		0: {Kind: modelChoice, Occurs: one, Particles: []contentParticle{modelParticle(2, one)}},
		1: {Kind: modelChoice, Occurs: one, Particles: []contentParticle{modelParticle(2, one)}},
		2: {Kind: modelChoice, Occurs: one, Particles: []contentParticle{modelParticle(3, one)}},
		3: {Kind: modelEmpty},
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
			analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
			upa := newContentModelUPACheck(nil, rt.build(), 4, &work, analysis)
			var err error
			for id := range contentModelID(4) {
				if err = upa.checkModel(id); err != nil {
					break
				}
			}
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

func upaCheckSteps(t *testing.T, models map[contentModelID]contentModelNode, roots []contentModelID) int {
	t.Helper()
	rt := compiledModelFixture{models: models}
	work := newWorkBudget(contentModelWorkBudget, 1_000_000)
	analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
	upa := newContentModelUPACheck(nil, rt.build(), len(models), &work, analysis)
	for _, id := range roots {
		if err := upa.checkModel(id); err != nil {
			t.Fatalf("checkModel(%d) error = %v", id, err)
		}
	}
	return work.limit - work.remaining
}

func TestCheckContentModelsUPAChargesNothingForContinuationsOfTheLastSequenceMember(t *testing.T) {
	t.Parallel()

	const roots, width = 6, 40
	one := occurrence{Min: 1, Max: 1}
	twice := occurrence{Min: 1, Max: 2}
	sequencesEndingIn := func(memberOccurs occurrence) map[contentModelID]contentModelNode {
		group := make([]contentParticle, width)
		for i := range group {
			group[i] = elementParticle(ElementID(i), memberOccurs)
		}
		models := map[contentModelID]contentModelNode{0: {Kind: modelChoice, Occurs: one, Particles: group}}
		for r := 1; r <= roots; r++ {
			models[contentModelID(r)] = contentModelNode{Kind: modelSequence, Occurs: one, Particles: []contentParticle{
				elementParticle(ElementID(width+r), one),
				modelParticle(0, one),
			}}
		}
		return models
	}
	ids := make([]contentModelID, 0, roots)
	for r := 1; r <= roots; r++ {
		ids = append(ids, contentModelID(r))
	}
	withoutContinuations := upaCheckSteps(t, sequencesEndingIn(one), ids)
	withContinuations := upaCheckSteps(t, sequencesEndingIn(twice), ids)
	if withContinuations != withoutContinuations {
		t.Fatalf("UPA steps with %d continuation candidates in the shared last group = %d, want %d as without them", width, withContinuations, withoutContinuations)
	}
}

func TestCheckContentModelsUPAChargesNothingAfterASplitIsDecided(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	optional := occurrence{Min: 0, Max: 1}
	twice := occurrence{Min: 1, Max: 2}
	splitRootWith := func(members int) map[contentModelID]contentModelNode {
		particles := []contentParticle{modelParticle(1, twice)}
		for i := range members {
			particles = append(particles, elementParticle(ElementID(10+i), optional))
		}
		return map[contentModelID]contentModelNode{
			0: {Kind: modelSequence, Occurs: one, Particles: particles},
			1: {Kind: modelChoice, Occurs: one, Particles: []contentParticle{elementParticle(0, one), elementParticle(1, optional)}},
		}
	}
	short := upaCheckSteps(t, splitRootWith(1), []contentModelID{0})
	long := upaCheckSteps(t, splitRootWith(30), []contentModelID{0})
	if long != short {
		t.Fatalf("UPA steps for a split root with 30 later members = %d, want %d as with one", long, short)
	}
}

func TestWildcardOverlapPassChargesNothingForElementCandidates(t *testing.T) {
	t.Parallel()

	const following = 30
	one := occurrence{Min: 1, Max: 1}
	optional := occurrence{Min: 0, Max: 1}
	passSteps := func(first occurrence) int {
		particles := []contentParticle{elementParticle(0, first)}
		for i := range following {
			particles = append(particles, elementParticle(ElementID(1+i), optional))
		}
		rt := compiledModelFixture{models: map[contentModelID]contentModelNode{0: {Kind: modelSequence, Occurs: one, Particles: particles}}}
		work := newWorkBudget(contentModelWorkBudget, 1_000_000)
		analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
		c := newContentModelCompiler(nil, rt.build(), 0, &work, analysis)
		model, _ := contentModelByID(c.rt.Models, 0)
		overlap, err := c.sequenceHasWildcardEquivalentOverlap(model)
		if err != nil || overlap {
			t.Fatalf("sequenceHasWildcardEquivalentOverlap() = %v, %v, want false, nil", overlap, err)
		}
		return work.limit - work.remaining
	}
	unit := passSteps(one)
	repeated := passSteps(occurrence{Min: 1, Max: 2})
	if repeated != unit {
		t.Fatalf("wildcard-pass steps with a repeated element candidate = %d, want %d as without a candidate", repeated, unit)
	}
}

func TestDFAFollowStateLookupChargesCacheHits(t *testing.T) {
	t.Parallel()

	newBuilder := func(limit int) *dfaBuilder {
		work := newWorkBudget(contentModelWorkBudget, limit)
		b := &dfaBuilder{
			c:         &contentModelCompiler{work: &work},
			follow:    followTableOf(map[int][]int{0: {0}}),
			states:    make(map[string]uint32),
			positions: []contentParticle{elementParticle(1, occurrence{Min: 1, Max: 1})},
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
		if _, err := b.row([]dfaStateEntry{{Pos: 0}}); err != nil {
			t.Fatalf("row() error = %v", err)
		}
	})

	t.Run("repeated lookup charges the hit", func(t *testing.T) {
		t.Parallel()
		b := newBuilder(8)
		if err := b.normalizeFollows(); err != nil {
			t.Fatalf("normalizeFollows() error = %v", err)
		}
		if _, err := b.row([]dfaStateEntry{{Pos: 0}}); err != nil {
			t.Fatalf("first row() error = %v", err)
		}
		_, err := b.row([]dfaStateEntry{{Pos: 0}})
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	})
}
