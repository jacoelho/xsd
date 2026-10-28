package schema

import (
	"math"
	"runtime"
	"testing"
	"unsafe"
)

func TestNextContentQNameBounds(t *testing.T) {
	t.Parallel()
	build := contentContractBuild(t, []QName{{Namespace: 3, Local: math.MaxUint32 - 4}, {Namespace: 3, Local: math.MaxUint32 - 1}})
	rt := contentContractSchema(build, testCompiledModelReads(build.CompiledModels))
	for _, test := range []struct {
		name  string
		input RuntimeName
		want  ElementID
	}{
		{"first", RuntimeName{Known: true, Name: QName{Namespace: 3, Local: math.MaxUint32 - 4}}, 0},
		{"last", RuntimeName{Known: true, Name: QName{Namespace: 3, Local: math.MaxUint32 - 1}}, 1},
		{"different namespace", RuntimeName{Known: true, Name: QName{Namespace: 4, Local: math.MaxUint32 - 1}}, NoElement},
		{"unknown namespace", RuntimeName{NS: "urn:unknown", Known: false, Name: QName{Namespace: 3, Local: math.MaxUint32 - 1}}, NoElement},
		{"below", RuntimeName{Known: true, Name: QName{Namespace: 3, Local: 0}}, NoElement},
		{"hole", RuntimeName{Known: true, Name: QName{Namespace: 3, Local: math.MaxUint32 - 2}}, NoElement},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			st := ContentState{model: 0, present: true}
			transition, status := rt.NextContent(st, ContentInput{Name: test.input}, &ContentScratch{})
			if test.want == NoElement {
				if status != ContentTransitionNoMatch || transition.IsPlanned() {
					t.Fatalf("missing name = %+v/%v, want no match", transition, status)
				}
				return
			}
			kind, element := transition.Match()
			if status != ContentTransitionMatched || kind != ContentMatchDeclared || element != test.want {
				t.Fatalf("named match = %v/%v/%v, want declared %v", status, kind, element, test.want)
			}
			if !transition.Commit(&st, &ContentScratch{}) || st.state != 1 {
				t.Fatalf("commit = %+v, want state 1", st)
			}
		})
	}
	t.Run("above span", func(t *testing.T) {
		t.Parallel()
		lower := contentContractBuild(t, []QName{{Local: 10}, {Local: 11}})
		runtime := contentContractSchema(lower, testCompiledModelReads(lower.CompiledModels))
		transition, status := runtime.NextContent(ContentState{model: 0, present: true}, ContentInput{Name: RuntimeName{Known: true, Name: QName{Local: 12}}}, &ContentScratch{})
		if status != ContentTransitionNoMatch || transition.IsPlanned() {
			t.Fatalf("above-span name = %+v/%v, want no match", transition, status)
		}
	})
}

func TestNextContentExhaustedNamedCandidateContinuesToWildcard(t *testing.T) {
	t.Parallel()
	for _, named := range []int{2, 7} {
		t.Run(map[int]string{2: "three edges", 7: "eight edges"}[named], func(t *testing.T) {
			t.Parallel()
			build := contentContractBuild(t, contentContractNames(named))
			row := &build.CompiledModels[0].Rows[0]
			row.Counted, row.Min, row.Max = true, 1, 1
			row.CountParticle = row.Edges[0].Particle
			row.Edges[0].To = 0
			row.Edges = append(row.Edges, CompiledModelEdge{Particle: WildcardParticle(0, Occurrence{Min: 1, Max: 1}), To: 1})
			if err := IndexCompiledModelsRows(build, build.CompiledModels, unlimitedContentModelWork); err != nil {
				t.Fatal(err)
			}
			rt := contentContractSchema(build, testCompiledModelReads(build.CompiledModels))
			rt.program.Wildcards = []WildcardView{newWildcardView(nil, &Wildcard{Mode: WildcardAny, Process: ProcessSkip})}
			st := ContentState{model: 0, count: 1, present: true}
			transition, status := rt.NextContent(st, ContentInput{Name: RuntimeName{Known: true, Name: QName{Local: 1}}}, &ContentScratch{})
			kind, _ := transition.Match()
			if status != ContentTransitionMatched || kind != ContentMatchSkip || !transition.Commit(&st, &ContentScratch{}) || st.state != 1 {
				t.Fatalf("exhausted named candidate = %+v/%v/%v, want wildcard exit", st, status, kind)
			}
		})
	}
}

func TestNextContentDuplicateNameKeepsCountedLoopAndExit(t *testing.T) {
	t.Parallel()
	for _, width := range []int{3, 8, 17} {
		t.Run(map[int]string{3: "narrow", 8: "wide", 17: "seventeen edges"}[width], func(t *testing.T) {
			t.Parallel()
			build := contentContractBuild(t, contentContractNames(width-1))
			row := &build.CompiledModels[0].Rows[0]
			row.Counted, row.Min, row.Max = true, 2, 2
			row.CountParticle = row.Edges[0].Particle
			loop := row.Edges[0]
			loop.To = 0
			row.Edges = append([]CompiledModelEdge{loop}, row.Edges...)
			row.index = dfaRowIndex{}
			if err := IndexCompiledModelsRows(build, build.CompiledModels, unlimitedContentModelWork); err != nil {
				t.Fatal(err)
			}
			reads := testCompiledModelReads(build.CompiledModels)
			rt := contentContractSchema(build, reads)
			st := ContentState{model: 0, present: true}
			for _, want := range []ContentState{{model: 0, count: 1, present: true}, {model: 0, count: 2, present: true}, {model: 0, state: 1, present: true}} {
				transition, status := rt.NextContent(st, ContentInput{Name: RuntimeName{Known: true, Name: QName{Local: 1}}}, &ContentScratch{})
				if status != ContentTransitionMatched || !transition.Commit(&st, &ContentScratch{}) || st != want {
					t.Fatalf("counted transition = %+v/%v, want %+v", st, status, want)
				}
			}
		})
	}
}

func contentContractNames(count int) []QName {
	names := make([]QName, count)
	for i := range names {
		local, ok := NewUint32Index(i + 1)
		if !ok {
			panic("test local-name ID exceeds uint32")
		}
		names[i] = QName{Local: LocalNameID(local)}
	}
	return names
}

func contentContractBuild(t *testing.T, names []QName) *schemaBuild {
	t.Helper()
	build := &schemaBuild{Elements: make([]ElementDecl, len(names))}
	edges := make([]CompiledModelEdge, len(names))
	for i, name := range names {
		build.Elements[i] = ElementDecl{Name: name}
		edges[i] = CompiledModelEdge{Particle: ElementParticle(ElementID(i), Occurrence{Min: 1, Max: 1}), To: 1}
	}
	build.CompiledModels = []CompiledModel{{Kind: CompiledModelDFA, Rows: []CompiledModelRow{{Edges: edges}, {Accept: true}}}}
	if err := IndexCompiledModelsRows(build, build.CompiledModels, unlimitedContentModelWork); err != nil {
		t.Fatal(err)
	}
	return build
}

func contentContractSchema(build *schemaBuild, reads []compiledModelRead) *Schema {
	return &Schema{program: schemaProgram{CompiledModels: reads, Elements: newElementReadTable(build.Elements), Substitutions: build.Substitutions}}
}

func TestNextContentWarmNamedDispatchAllocatesNothing(t *testing.T) {
	for _, width := range []uint32{4, 8, 16} {
		t.Run(map[uint32]string{4: "four edges", 8: "eight edges", 16: "sixteen edges"}[width], func(t *testing.T) {
			names := make(map[ElementID]QName, width)
			edges := make([]CompiledModelEdge, width)
			for i := range width {
				id := ElementID(i + 1)
				names[id] = QName{Local: LocalNameID(i + 1)}
				edges[i] = CompiledModelEdge{Particle: ElementParticle(id, Occurrence{Min: 1, Max: 1}), To: 1}
			}
			build := (dfaRowIndexFixture{elementNames: names}).build()
			models := []CompiledModel{{Kind: CompiledModelDFA, Rows: []CompiledModelRow{{Edges: edges}, {Accept: true}}}}
			if err := IndexCompiledModelsRows(build, models, unlimitedContentModelWork); err != nil {
				t.Fatal(err)
			}
			rt := publishedContentSchema(contentSchemaFixture{models: map[ContentModelID]CompiledModel{0: models[0]}, elementNames: names})
			initial := ContentState{model: 0, present: true}
			input := ContentInput{Name: RuntimeName{Known: true, Name: QName{Local: LocalNameID(width)}}}
			var scratch ContentScratch
			transition, status := rt.NextContent(initial, input, &scratch)
			check := func() {
				t.Helper()
				kind, element := transition.Match()
				if status != ContentTransitionMatched || kind != ContentMatchDeclared || element != ElementID(width) {
					t.Fatalf("named dispatch = %v/%v/%v, want declaration %d", status, kind, element, width)
				}
				next := initial
				if !transition.Commit(&next, &scratch) || next.state != 1 || rt.CompleteContent(next, &scratch) != ContentCompletionComplete {
					t.Fatalf("named dispatch did not commit and complete: %+v", next)
				}
			}
			check()
			allocs := testing.AllocsPerRun(100, func() {
				transition, status = rt.NextContent(initial, input, &scratch)
			})
			check()
			if allocs != 0 {
				t.Fatalf("warm NextContent allocations = %v, want 0", allocs)
			}
		})
	}
}

func TestCompiledModelRowReadStorageCeiling(t *testing.T) {
	t.Parallel()
	var ceiling uintptr
	switch {
	case runtime.GOARCH == "arm64":
		ceiling = 80
	case runtime.GOARCH == "386" && runtime.Compiler == "gc":
		ceiling = 52
	default:
		t.Skip("row storage ceiling is specified for arm64 and gc/386")
	}
	size := unsafe.Sizeof(compiledModelRowRead{})
	if size > ceiling {
		t.Fatalf("compiled content row occupies %d bytes, ceiling %d", size, ceiling)
	}
	t.Logf("compiled content row occupies %d bytes", size)
}
