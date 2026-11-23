package schema

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestCompileRootContentModelChargesItsRecordRowsEdgesAndTerms(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		kind modelKind
		want int
	}{
		{name: "sequence of eight elements", kind: modelSequence, want: 2 + 9 + 8},
		{name: "choice of eight elements", kind: modelChoice, want: 2 + 9 + 8},
		{name: "all group of eight elements", kind: modelAll, want: 2 + 8},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rt := compiledModelRuntimeFixture(t, test.kind)
			compiles := func(limit int) bool {
				work := newWorkBudget(contentModelWorkBudget, 1_000_000)
				retained := newWorkBudget(dependencyWorkBudget, limit)
				analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
				cc := newContentModelCompiler(rt.build(), 32, &work, &retained, analysis)
				_, err := cc.compileRootContentModel(0)
				if err == nil {
					return true
				}
				if diagnostic, ok := errors.AsType[*xsderrors.Error](err); !ok || diagnostic.Code() != xsderrors.CodeSchemaLimit {
					t.Fatalf("compileRootContentModel() error = %v, want schema.limit", err)
				}
				return false
			}
			if !compiles(test.want) || compiles(test.want-1) {
				t.Fatalf("least MaxSchemaDependencySteps that compiles the model is not %d", test.want)
			}
		})
	}
}

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
			upa := newContentModelUPACheck(rt.build(), 4, &work, testDependencyWork(), analysis)
			var err error
			for id := range contentModelID(4) {
				if err = upa.checkModel(id, namedComplexTypeLabel(QName{})); err != nil {
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

func checkUPAWithin(t *testing.T, models map[contentModelID]contentModelNode, roots []contentModelID, limit int) (int, error) {
	t.Helper()
	rt := compiledModelFixture{models: models}
	work := newWorkBudget(contentModelWorkBudget, limit)
	analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
	upa := newContentModelUPACheck(rt.build(), len(models), &work, testDependencyWork(), analysis)
	for _, id := range roots {
		if err := upa.checkModel(id, namedComplexTypeLabel(QName{})); err != nil {
			return work.limit - work.remaining, err
		}
	}
	return work.limit - work.remaining, nil
}

func expectChargeLimit(t *testing.T, limit int, run func(limit int) (int, error)) {
	t.Helper()
	if _, err := run(limit); err != nil {
		t.Fatalf("check at limit %d error = %v, want none", limit, err)
	}
	_, err := run(limit - 1)
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
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
	withoutContinuations, err := checkUPAWithin(t, sequencesEndingIn(one), ids, 1_000_000)
	if err != nil {
		t.Fatalf("checkModel() error = %v", err)
	}
	expectChargeLimit(t, withoutContinuations, func(limit int) (int, error) {
		return checkUPAWithin(t, sequencesEndingIn(twice), ids, limit)
	})
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
	oneMember, err := checkUPAWithin(t, splitRootWith(1), []contentModelID{0}, 1_000_000)
	if err != nil {
		t.Fatalf("checkModel() error = %v", err)
	}
	expectChargeLimit(t, oneMember, func(limit int) (int, error) {
		return checkUPAWithin(t, splitRootWith(30), []contentModelID{0}, limit)
	})
}

func TestWildcardOverlapPassChargesNothingForElementCandidates(t *testing.T) {
	t.Parallel()

	const following = 30
	one := occurrence{Min: 1, Max: 1}
	optional := occurrence{Min: 0, Max: 1}
	pass := func(first occurrence, limit int) (int, error) {
		particles := []contentParticle{elementParticle(0, first)}
		for i := range following {
			particles = append(particles, elementParticle(ElementID(1+i), optional))
		}
		rt := compiledModelFixture{models: map[contentModelID]contentModelNode{0: {Kind: modelSequence, Occurs: one, Particles: particles}}}
		work := newWorkBudget(contentModelWorkBudget, limit)
		analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
		c := newContentModelCompiler(rt.build(), 0, &work, testDependencyWork(), analysis)
		overlap, err := c.sequenceHasWildcardEquivalentOverlap(contentModelNode{Kind: modelSequence, Occurs: one, Particles: particles})
		if overlap {
			t.Fatal("sequenceHasWildcardEquivalentOverlap() = true for a sequence without wildcards")
		}
		return work.limit - work.remaining, err
	}
	withoutCandidate, err := pass(one, 1_000_000)
	if err != nil {
		t.Fatalf("sequenceHasWildcardEquivalentOverlap() error = %v", err)
	}
	expectChargeLimit(t, withoutCandidate, func(limit int) (int, error) {
		return pass(occurrence{Min: 1, Max: 2}, limit)
	})
}

func TestDFAFollowStateLookupChargesCacheHits(t *testing.T) {
	t.Parallel()

	newBuilder := func(limit int) *dfaBuilder {
		work := newWorkBudget(contentModelWorkBudget, limit)
		b := &dfaBuilder{
			c:         &contentModelCompiler{work: &work, retained: testDependencyWork()},
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
