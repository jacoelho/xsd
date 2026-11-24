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

func TestCompileRootContentModelChargesOnlyAutomatonTablesWhenDirectCompilationDoesNotApply(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	for _, test := range []struct {
		name         string
		root, nested modelKind
	}{
		{name: "sequence of six elements and a choice of two", root: modelSequence, nested: modelChoice},
		{name: "choice of six elements and a sequence of two", root: modelChoice, nested: modelSequence},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			rt := compiledModelRuntimeFixture(t, test.root)
			elements := rt.models[0].Particles
			rt.models[0] = contentModelNode{Kind: test.root, Occurs: one, Particles: append(elements[:6:6], modelParticle(1, one))}
			rt.models[1] = contentModelNode{Kind: test.nested, Occurs: one, Particles: elements[6:]}
			compile := func(limit int) (compiledModel, error) {
				work := newWorkBudget(contentModelWorkBudget, 1_000_000)
				retained := newWorkBudget(dependencyWorkBudget, limit)
				analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
				cc := newContentModelCompiler(rt.build(), 32, &work, &retained, analysis)
				return cc.compileRootContentModel(0)
			}
			compiled, err := compile(1_000_000)
			if err != nil {
				t.Fatalf("compileRootContentModel() error = %v", err)
			}
			want := 2 + len(compiled.Rows)
			for _, row := range compiled.Rows {
				want += len(row.Edges)
			}
			if _, err = compile(want); err != nil {
				t.Fatalf("compileRootContentModel() at its table's %d records error = %v", want, err)
			}
			_, err = compile(want - 1)
			expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
		})
	}
}

func TestCompileDirectModelDeclinesWithinThreeStepsPerParticleAndNoRetainedSteps(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	optional := occurrence{Min: 0, Max: 1}
	rt := compiledModelRuntimeFixture(t, modelSequence)
	elements := rt.models[0].Particles
	particles := make([]contentParticle, 0, 7)
	for _, element := range elements[:6] {
		particles = append(particles, elementParticle(element.Element, optional))
	}
	particles = append(particles, modelParticle(1, one))
	rt.models[0] = contentModelNode{Kind: modelSequence, Occurs: one, Particles: particles}
	rt.models[1] = contentModelNode{Kind: modelChoice, Occurs: one, Particles: elements[6:]}
	build := rt.build()
	work := newWorkBudget(contentModelWorkBudget, 3*len(particles))
	retained := newWorkBudget(dependencyWorkBudget, 0)
	analysisWork := newWorkBudget(contentModelWorkBudget, 1_000_000)
	analysis := mustContentModelAnalysis(t, build, analysisWork.spend)
	cc := newContentModelCompiler(build, 32, &work, &retained, analysis)
	_, ok, err := cc.compileDirectModel(build.Models[0])
	if err != nil || ok {
		t.Fatalf("compileDirectModel() = (ok %v, error %v), want a decline within %d steps", ok, err, 3*len(particles))
	}
}

func TestDirectBuildRejectsWhatItsAdmissionDidNotCount(t *testing.T) {
	t.Parallel()

	rt := compiledModelRuntimeFixture(t, modelSequence)
	model := rt.build().Models[0]
	for _, test := range []struct {
		name         string
		walkSteps    int
		admittedRows int
	}{
		{name: "walk longer than admission", walkSteps: 0, admittedRows: 9},
		{name: "more rows than admission", walkSteps: 1_000_000, admittedRows: 8},
	} {
		for _, kind := range []struct {
			name string
			kind modelKind
		}{{"sequence", modelSequence}, {"choice", modelChoice}} {
			build := rt.build()
			work := newWorkBudget(contentModelWorkBudget, 1_000_000)
			retained := newWorkBudget(dependencyWorkBudget, 1_000_000)
			analysis := mustContentModelAnalysis(t, build, work.spend)
			cc := newContentModelCompiler(build, 32, &work, &retained, analysis)
			walk := cc
			walkBudget := newWorkBudget(contentModelWorkBudget, test.walkSteps)
			walk.work = &walkBudget
			var err error
			if kind.kind == modelSequence {
				_, err = cc.compileDirectSequenceModel(model, &walk, test.admittedRows)
			} else {
				_, err = cc.compileDirectChoiceModel(model, &walk, test.admittedRows)
			}
			t.Run(test.name+" in a "+kind.name, func(t *testing.T) {
				t.Parallel()
				expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			})
		}
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

func wildcardOverlapPassWithin(t *testing.T, models map[contentModelID]contentModelNode, limit int) (int, error) {
	t.Helper()
	rt := compiledModelFixture{models: models}
	work := newWorkBudget(contentModelWorkBudget, limit)
	analysis := mustContentModelAnalysis(t, rt.build(), work.spend)
	c := newContentModelCompiler(rt.build(), 0, &work, testDependencyWork(), analysis)
	overlap, err := c.sequenceHasWildcardEquivalentOverlap(models[0])
	if overlap {
		t.Fatal("sequenceHasWildcardEquivalentOverlap() = true for a sequence without wildcards")
	}
	return work.limit - work.remaining, err
}

func TestWildcardOverlapPassChargesNothingForARepeatedElementMember(t *testing.T) {
	t.Parallel()

	const following = 30
	one := occurrence{Min: 1, Max: 1}
	optional := occurrence{Min: 0, Max: 1}
	sequence := func(first occurrence) map[contentModelID]contentModelNode {
		particles := []contentParticle{elementParticle(0, first)}
		for i := range following {
			particles = append(particles, elementParticle(ElementID(1+i), optional))
		}
		return map[contentModelID]contentModelNode{0: {Kind: modelSequence, Occurs: one, Particles: particles}}
	}
	single, err := wildcardOverlapPassWithin(t, sequence(one), 1_000_000)
	if err != nil {
		t.Fatalf("sequenceHasWildcardEquivalentOverlap() error = %v", err)
	}
	expectChargeLimit(t, single, func(limit int) (int, error) {
		return wildcardOverlapPassWithin(t, sequence(occurrence{Min: 1, Max: 2}), limit)
	})
}

func TestWildcardOverlapPassChargesAGroupsElementCandidatesOncePerModel(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	twice := occurrence{Min: 1, Max: 2}
	pass := func(references, width int) int {
		choice := make([]contentParticle, width)
		for i := range choice {
			choice[i] = elementParticle(ElementID(i), one)
		}
		var sequence []contentParticle
		for range references {
			sequence = append(sequence, modelParticle(1, twice))
		}
		sequence = append(sequence, elementParticle(1_000, one))
		steps, err := wildcardOverlapPassWithin(t, map[contentModelID]contentModelNode{
			0: {Kind: modelSequence, Occurs: one, Particles: sequence},
			1: {Kind: modelChoice, Occurs: twice, Particles: choice},
		}, 1_000_000)
		if err != nil {
			t.Fatalf("sequenceHasWildcardEquivalentOverlap() error = %v", err)
		}
		return steps
	}
	once := pass(1, 40) - pass(1, 1)
	if many := pass(8, 40) - pass(8, 1); many != once {
		t.Fatalf("39 more choice members cost %d steps across 8 references, want %d as across 1", many, once)
	}
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
