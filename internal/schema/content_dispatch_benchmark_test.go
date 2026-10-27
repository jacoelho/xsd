package schema

import (
	"fmt"
	"testing"
)

type contentDispatchBenchmarkCase struct {
	name     string
	width    uint32
	position string
	shape    string
}

func BenchmarkContentDispatch(b *testing.B) {
	cases := []contentDispatchBenchmarkCase{
		{"dense/1/first", 1, "first", "dense"},
		{"dense/2/last", 2, "last", "dense"},
		{"dense/4/first", 4, "first", "dense"},
		{"dense/4/last", 4, "last", "dense"},
		{"dense/4/missing", 4, "missing", "dense"},
		{"dense/8/last", 8, "last", "dense"},
		{"dense/16/last", 16, "last", "dense"},
		{"dense/200/last", 200, "last", "dense"},
		{"sparse/4/last", 4, "last", "sparse"},
		{"fullQName/4/last", 4, "last", "fullQName"},
		{"substitution/4/member-last", 4, "member-last", "substitution"},
	}
	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			setup := contentDispatchBenchmarkFixture(b, test)
			rt, initial, input, wantElement := setup.rt, setup.initial, setup.input, setup.wantElement
			original := initial
			var scratch ContentScratch
			check := func(transition ContentTransition, status ContentTransitionStatus) {
				b.Helper()
				if initial != original {
					b.Fatalf("planning mutated initial state: %v", initial)
				}
				if test.position == "missing" {
					if status != ContentTransitionNoMatch || transition != (ContentTransition{}) {
						b.Fatalf("missing transition = %+v/%v, want zero/no match", transition, status)
					}
					return
				}
				kind, element := transition.Match()
				if status != ContentTransitionMatched || kind != ContentMatchDeclared || element != wantElement {
					b.Fatalf("transition = %v/%v/%v, want matched/declared/%v", status, kind, element, wantElement)
				}
				next := initial
				if !transition.Commit(&next, &scratch) || next.state != 1 || next.model != initial.model {
					b.Fatalf("committed state = %+v, want destination1", next)
				}
				if got := rt.CompleteContent(next, &scratch); got != ContentCompletionComplete {
					b.Fatalf("completion = %v, want complete", got)
				}
				if initial != original {
					b.Fatal("commit changed the uncommitted initial state")
				}
			}
			transition, status := rt.NextContent(initial, input, &scratch)
			check(transition, status)
			b.ReportAllocs()
			for b.Loop() {
				transition, status = rt.NextContent(initial, input, &scratch)
			}
			check(transition, status)
			b.ReportMetric(float64(test.width), "row_edges")
			expanded := test.width
			if test.shape == "substitution" {
				expanded++
			}
			b.ReportMetric(float64(expanded), "expanded_names")
		})
	}
}

type contentDispatchBenchmarkSetup struct {
	rt          *Schema
	initial     ContentState
	input       ContentInput
	wantElement ElementID
	wantKind    ContentMatchKind
}

func contentDispatchBenchmarkFixture(b *testing.B, test contentDispatchBenchmarkCase) contentDispatchBenchmarkSetup {
	b.Helper()
	names := make(map[ElementID]QName, test.width+2)
	edges := make([]CompiledModelEdge, test.width)
	for i := range edges {
		id := ElementID(i + 1)
		name := QName{Local: LocalNameID(i + 1)}
		if test.shape == "sparse" {
			name.Local = LocalNameID(1 + 4096*i)
		}
		if test.shape == "wildcard-name" || test.shape == "wildcard-skip" {
			name.Namespace = 1
		}
		if test.shape == "fullQName" {
			name = QName{Namespace: NamespaceID(i + 1), Local: 1}
		}
		names[id] = name
		edges[i] = CompiledModelEdge{Particle: ElementParticle(id, Occurrence{Min: 1, Max: 1}), To: 1}
	}
	// Keep the ordinary span table present even when every row head has no members.
	extra := ElementID(test.width + 1)
	if test.shape == "substitution" {
		extra++
	}
	names[extra] = QName{Local: LocalNameID(test.width + 2)}
	if test.shape == "sparse" {
		names[extra] = QName{Local: LocalNameID(1 + 4096*test.width)}
	}
	substitutions := map[ElementID]map[QName]ElementID{extra: {}}
	selected := ElementID(test.width)
	switch test.position {
	case "first":
		selected = 1
	case "middle":
		selected = ElementID(test.width/2 + 1)
	case "last", "member-last", "missing":
	default:
		b.Fatalf("unknown benchmark position %q", test.position)
	}
	want := selected
	inputName := names[selected]
	if test.shape == "substitution" {
		member := ElementID(test.width + 1)
		names[member] = QName{Local: 77}
		substitutions[selected] = map[QName]ElementID{names[member]: member}
		inputName, want = names[member], member
	}
	if test.position == "missing" {
		inputName, want = names[extra], NoElement
	}
	wildcards := map[WildcardID]Wildcard{}
	wantKind := ContentMatchDeclared
	initial := ContentState{model: 0, present: true}
	row := CompiledModelRow{Edges: edges}
	switch test.shape {
	case "wildcard-name", "wildcard-skip":
		wildcards[0] = Wildcard{Mode: WildcardLocal, Process: ProcessSkip}
		row.Edges = append([]CompiledModelEdge{{Particle: WildcardParticle(0, Occurrence{Min: 1, Max: 1}), To: 1}}, edges...)
		if test.shape == "wildcard-skip" {
			inputName, want, wantKind = QName{}, NoElement, ContentMatchSkip
		}
	case "counted":
		wildcards[0] = Wildcard{Mode: WildcardAny, Process: ProcessSkip}
		row.Counted, row.Min, row.Max, row.CountParticle = true, 1, 1, edges[0].Particle
		row.Edges[0].To = 0
		row.Edges = append(row.Edges, CompiledModelEdge{Particle: WildcardParticle(0, Occurrence{Min: 1, Max: 1}), To: 1})
		initial.count = 1
		inputName, want, wantKind = names[1], NoElement, ContentMatchSkip
	}
	build := (dfaRowIndexFixture{elementNames: names, substitutionByName: substitutions}).build()
	models := []CompiledModel{{Kind: CompiledModelDFA, Rows: []CompiledModelRow{row, {Accept: true}}}}
	if err := IndexCompiledModelsRows(build, models, unlimitedContentModelWork); err != nil {
		b.Fatal(err)
	}
	rt := publishedContentSchema(contentSchemaFixture{models: map[ContentModelID]CompiledModel{0: models[0]}, elementNames: names, substitutionLookup: substitutions, wildcards: wildcards})
	input := ContentInput{Name: RuntimeName{Known: true, Name: inputName, Local: fmt.Sprintf("fixture-%d", inputName.Local)}}
	if test.shape == "wildcard-name" {
		input.Name.NS = "urn:fixture"
	}
	if test.shape == "wildcard-skip" {
		input.Name.Known = false
	}
	return contentDispatchBenchmarkSetup{rt: rt, initial: initial, input: input, wantElement: want, wantKind: wantKind}
}

func BenchmarkContentDispatchPolicy(b *testing.B) {
	cases := []contentDispatchBenchmarkCase{
		{"dense/7/last", 7, "last", "dense"},
		{"dense/8/first", 8, "first", "dense"},
		{"dense/15/last", 15, "last", "dense"},
		{"dense/17/last", 17, "last", "dense"},
		{"dense/16/first", 16, "first", "dense"},
		{"dense/16/middle", 16, "middle", "dense"},
		{"dense/16/missing", 16, "missing", "dense"},
		{"dense/16/fixed-mixed", 16, "fixed-mixed", "dense"},
		{"dense/200/first", 200, "first", "dense"},
		{"dense/200/middle", 200, "middle", "dense"},
		{"dense/200/missing", 200, "missing", "dense"},
		{"dense/200/fixed-mixed", 200, "fixed-mixed", "dense"},
		{"sparse/16/last", 16, "last", "sparse"},
		{"fullQName/16/last", 16, "last", "fullQName"},
		{"substitution/16/member77", 16, "member-last", "substitution"},
		{"wildcard/16/name-last", 16, "last", "wildcard-name"},
		{"wildcard/16/skip-first", 16, "first", "wildcard-skip"},
		{"counted/16/exhausted-to-wildcard", 16, "first", "counted"},
	}
	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			fixture := test
			if fixture.position == "fixed-mixed" {
				fixture.position = "first"
			}
			setup := contentDispatchBenchmarkFixture(b, fixture)
			inputs := []ContentInput{setup.input}
			wants := []contentDispatchBenchmarkWant{{ContentTransitionMatched, setup.wantKind, setup.wantElement}}
			if test.position == "missing" {
				wants[0] = contentDispatchBenchmarkWant{ContentTransitionNoMatch, ContentMatchInvalid, NoElement}
			}
			if test.position == "fixed-mixed" {
				inputs, wants = contentDispatchBenchmarkMixed(test.width)
			}
			var scratch ContentScratch
			for i, input := range inputs {
				transition, status := setup.rt.NextContent(setup.initial, input, &scratch)
				checkContentDispatchBenchmark(b, setup, &scratch, transition, status, wants[i])
			}
			index, last := 0, 0
			var transition ContentTransition
			var status ContentTransitionStatus
			b.ReportAllocs()
			for b.Loop() {
				last = index
				transition, status = setup.rt.NextContent(setup.initial, inputs[index], &scratch)
				index++
				if index == len(inputs) {
					index = 0
				}
			}
			checkContentDispatchBenchmark(b, setup, &scratch, transition, status, wants[last])
			for i, input := range inputs {
				transition, status = setup.rt.NextContent(setup.initial, input, &scratch)
				checkContentDispatchBenchmark(b, setup, &scratch, transition, status, wants[i])
			}
			edges, expanded := test.width, test.width
			switch test.shape {
			case "substitution":
				expanded++
			case "wildcard-name", "wildcard-skip", "counted":
				edges++
			}
			b.ReportMetric(float64(edges), "row_edges")
			b.ReportMetric(float64(expanded), "expanded_names")
		})
	}
}

type contentDispatchBenchmarkWant struct {
	status  ContentTransitionStatus
	kind    ContentMatchKind
	element ElementID
}

func contentDispatchBenchmarkMixed(width uint32) ([]ContentInput, []contentDispatchBenchmarkWant) {
	positions := [4]uint32{1, width/2 + 1, width, width + 2}
	inputs := make([]ContentInput, 64)
	wants := make([]contentDispatchBenchmarkWant, 64)
	for i := range inputs {
		position := positions[i%4]
		inputs[i] = ContentInput{Name: RuntimeName{Known: true, Name: QName{Local: LocalNameID(position)}, Local: fmt.Sprintf("fixture-%d", position)}}
		wants[i] = contentDispatchBenchmarkWant{ContentTransitionMatched, ContentMatchDeclared, ElementID(position)}
		if i%4 == 3 {
			wants[i] = contentDispatchBenchmarkWant{ContentTransitionNoMatch, ContentMatchInvalid, NoElement}
		}
	}
	return inputs, wants
}

func checkContentDispatchBenchmark(b *testing.B, setup contentDispatchBenchmarkSetup, scratch *ContentScratch, transition ContentTransition, status ContentTransitionStatus, want contentDispatchBenchmarkWant) {
	b.Helper()
	initial := setup.initial
	if status != want.status {
		b.Fatalf("status = %v, want %v", status, want.status)
	}
	if want.status == ContentTransitionNoMatch {
		if transition != (ContentTransition{}) {
			b.Fatalf("missing transition = %+v, want zero", transition)
		}
		return
	}
	kind, element := transition.Match()
	if kind != want.kind || element != want.element {
		b.Fatalf("match = %v/%v, want %v/%v", kind, element, want.kind, want.element)
	}
	next := initial
	if !transition.Commit(&next, scratch) || next.model != initial.model || next.state != 1 || next.count != 0 {
		b.Fatalf("commit = %+v, want model0/state1/count0", next)
	}
	if setup.rt.CompleteContent(next, scratch) != ContentCompletionComplete || setup.initial != initial {
		b.Fatal("commit changed initial state or destination is incomplete")
	}
}
