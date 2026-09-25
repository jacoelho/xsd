package schema

import (
	"math"
	"slices"
	"strconv"
	"testing"
	"unsafe"
)

func TestElementFrameUsesCompiledModelInitialState(t *testing.T) {
	t.Parallel()

	typ := ComplexRef(1)
	modelID := ContentModelID(2)
	rt := publishedContentSchema(contentSchemaFixture{
		contentModels: map[TypeID]ContentModelID{
			typ: modelID,
		},
		models: map[ContentModelID]CompiledModel{
			modelID: {Start: 7, AllBitLen: 130},
		},
	})
	read, ok := rt.ElementFrame(typ, NoElement)
	if !ok {
		t.Fatal("ElementFrame() failed for valid complex type")
	}
	state := read.Content.ContentState()
	if state.model != modelID || state.state != 7 || read.Content.AllBitLen() != 130 {
		t.Fatalf("ElementFrame().Content = %+v, want model %v state 7 bitLen 130", read.Content, modelID)
	}

	read, ok = publishedContentSchema(contentSchemaFixture{
		contentModels: map[TypeID]ContentModelID{typ: NoContentModel},
	}).ElementFrame(typ, NoElement)
	if !ok {
		t.Fatal("ElementFrame() failed for valid complex type without content model")
	}
	if read.Content.ContentState().HasModel() {
		t.Fatalf("ElementFrame(no model) = %+v, want no model", read.Content)
	}
}

func TestContentScratchAllBitInvariants(t *testing.T) {
	t.Parallel()

	scratch := ContentScratch{}
	if _, ok := scratch.AllSeen(0); ok {
		t.Fatal("AllSeen accepted zero-length bitset")
	}
	if scratch.SetAllSeen(0) {
		t.Fatal("SetAllSeen accepted zero-length bitset")
	}

	scratch = NewContentScratch(nil, 0, 1)
	if _, ok := scratch.AllSeen(0); ok {
		t.Fatal("AllSeen accepted missing bit storage")
	}
	if scratch.SetAllSeen(0) {
		t.Fatal("SetAllSeen accepted missing bit storage")
	}
}

func TestCompiledModelReadProjectionUsesIsolatedFlatPools(t *testing.T) {
	t.Parallel()

	models := []CompiledModel{
		{},
		{Rows: []CompiledModelRow{{}}},
		{
			Rows: []CompiledModelRow{
				{Edges: []CompiledModelEdge{{To: 1}}},
				{Edges: []CompiledModelEdge{{To: 2}}},
			},
			All: []CompiledAllTerm{{Required: true}},
		},
		{
			Rows: []CompiledModelRow{{Edges: []CompiledModelEdge{{To: 3}}}},
			All:  []CompiledAllTerm{{Required: false}},
		},
	}

	reads := testCompiledModelReads(models)
	if reads[0].Rows != nil || reads[0].All != nil {
		t.Fatalf("empty model projection = rows %#v all %#v, want nil slices", reads[0].Rows, reads[0].All)
	}
	if reads[1].Rows == nil || reads[1].Rows[0].Edges != nil {
		t.Fatalf("empty row projection = rows %#v, want non-nil rows and nil edges", reads[1].Rows)
	}
	for i := 2; i < len(reads); i++ {
		if cap(reads[i].Rows) != len(reads[i].Rows) || cap(reads[i].All) != len(reads[i].All) {
			t.Fatalf("model %d projection capacities = rows %d/%d all %d/%d", i, len(reads[i].Rows), cap(reads[i].Rows), len(reads[i].All), cap(reads[i].All))
		}
		for j := range reads[i].Rows {
			if cap(reads[i].Rows[j].Edges) != len(reads[i].Rows[j].Edges) {
				t.Fatalf("model %d row %d edge capacity = %d/%d", i, j, len(reads[i].Rows[j].Edges), cap(reads[i].Rows[j].Edges))
			}
		}
	}

	reads[2].Rows[0].Edges[0].To = 99
	reads[2].Rows[1].Edges[0].To = 98
	reads[2].All[0].Required = false
	if reads[3].Rows[0].Edges[0].To != 3 || reads[3].All[0].Required {
		t.Fatalf("projection pools overlap: model 3 = %+v", reads[3])
	}
	if models[2].Rows[0].Edges[0].To != 1 || !models[2].All[0].Required {
		t.Fatalf("projection aliases source: model 2 = %+v", models[2])
	}
}

var compiledModelReadAllocationSink []compiledModelRead

func TestCompiledModelReadProjectionAllocationCountIsConstant(t *testing.T) {
	models := make([]CompiledModel, 10_000)
	for i := range models {
		models[i].Rows = []CompiledModelRow{{Edges: []CompiledModelEdge{{To: uint32(i)}}}}
		models[i].All = []CompiledAllTerm{{Required: true}}
	}

	allocs := testing.AllocsPerRun(3, func() {
		compiledModelReadAllocationSink = testCompiledModelReads(models)
	})
	if allocs > 4 {
		t.Fatalf("newCompiledModelReads() allocations = %v, want at most 4 flat tables", allocs)
	}
}

func TestAddCompiledModelReadCountRejectsOverflow(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("addCompiledModelReadCount() accepted overflowing count")
		}
	}()
	addCompiledModelReadCount(math.MaxInt, 1)
}

func TestNextContentAnyReturnsGlobalElement(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	elem := ElementID(2)
	rt := publishedContentSchema(contentSchemaFixture{
		models:         map[ContentModelID]CompiledModel{0: {Kind: CompiledModelAny}},
		globalElements: map[QName]ElementID{name: elem},
	})
	st := ContentState{model: 0, present: true}
	transition, status := rt.NextContent(st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &ContentScratch{})
	kind, element := transition.Match()
	if status != ContentTransitionMatched || kind != ContentMatchDeclared || element != elem {
		t.Fatalf("NextContent(any) = %v/%v/%v, want declared element", kind, element, status)
	}
}

func TestNextContentAllDefersScratchUntilCommit(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	elem := ElementID(2)
	model := ContentModelID(3)
	rt := publishedContentSchema(contentSchemaFixture{
		models: map[ContentModelID]CompiledModel{
			model: {
				Kind:      CompiledModelAll,
				AllBitLen: 1,
				All: []CompiledAllTerm{
					{Particle: ElementParticle(elem, Occurrence{Min: 1, Max: 1}), Required: true},
				},
			},
		},
		elementNames: map[ElementID]QName{elem: name},
	})
	scratch := NewContentScratch(make([]uint64, 1), 0, 1)
	st := ContentState{model: model, present: true}
	transition, status := rt.NextContent(st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &scratch)
	kind, element := transition.Match()
	if status != ContentTransitionMatched || kind != ContentMatchDeclared || element != elem {
		t.Fatalf("NextContent(all) = %v/%v/%v, want declared element", kind, element, status)
	}
	seen, ok := scratch.AllSeen(0)
	if !ok || seen {
		t.Fatalf("AllSeen(0) before commit = %v/%v, want false/true", seen, ok)
	}
	if !transition.Commit(&st, &scratch) {
		t.Fatal("ContentTransition.Commit() rejected current all-group state")
	}
	seen, ok = scratch.AllSeen(0)
	if !ok || !seen {
		t.Fatalf("AllSeen(0) after commit = %v/%v, want true/true", seen, ok)
	}
	if status := rt.CompleteContent(st, &scratch); status != ContentCompletionComplete {
		t.Fatalf("CompleteContent(all) = %v, want complete", status)
	}
}

func TestContentTransitionRejectsStaleAllStateWithoutMutation(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	elem := ElementID(2)
	model := ContentModelID(3)
	rt := publishedContentSchema(contentSchemaFixture{
		models: map[ContentModelID]CompiledModel{
			model: {
				Kind:      CompiledModelAll,
				AllBitLen: 1,
				All: []CompiledAllTerm{
					{Particle: ElementParticle(elem, Occurrence{Min: 1, Max: 1}), Required: true},
				},
			},
		},
		elementNames: map[ElementID]QName{elem: name},
	})
	bits := make([]uint64, 1)
	scratch := NewContentScratch(bits, 0, 1)
	state := ContentState{model: model, present: true}
	transition, status := rt.NextContent(state, ContentInput{Name: RuntimeName{Known: true, Name: name}}, &scratch)
	if status != ContentTransitionMatched {
		t.Fatalf("NextContent(all) status = %v, want matched", status)
	}
	state.state++
	wantState := state
	wantBits := slices.Clone(bits)
	if transition.Commit(&state, &scratch) {
		t.Fatal("ContentTransition.Commit() accepted stale state")
	}
	if state != wantState || !slices.Equal(bits, wantBits) {
		t.Fatalf("stale commit mutated state: state=%+v bits=%v, want %+v/%v", state, bits, wantState, wantBits)
	}
}

func TestNextContentIndexedSubstitutionReturnsMember(t *testing.T) {
	t.Parallel()

	head := ElementID(1)
	member := ElementID(2)
	headName := QName{Local: 1}
	memberName := QName{Local: 2}
	model := ContentModelID(3)
	rt := publishedContentSchema(contentSchemaFixture{
		models: map[ContentModelID]CompiledModel{
			model: {
				Kind: CompiledModelDFA,
				Rows: []CompiledModelRow{
					{
						index: dfaRowIndex{nameToEdge: map[QName]uint32{memberName: 0}},
						Edges: []CompiledModelEdge{{
							Particle: ElementParticle(head, Occurrence{Min: 1, Max: 1}),
							To:       1,
						}},
					},
					{Accept: true},
				},
			},
		},
		elementNames: map[ElementID]QName{head: headName},
		substitutionLookup: map[ElementID]map[QName]ElementID{
			head: {memberName: member},
		},
	})
	st := ContentState{model: model, present: true}
	transition, status := rt.NextContent(st, ContentInput{
		Name: RuntimeName{Known: true, Name: memberName, Local: "member"},
	}, &ContentScratch{})
	kind, element := transition.Match()
	if status != ContentTransitionMatched || kind != ContentMatchDeclared || element != member || st.state != 0 {
		t.Fatalf("NextContent(indexed substitution) = %v/%v/%v state %d, want declared member and unchanged state 0", kind, element, status, st.state)
	}
	if !transition.Commit(&st, &ContentScratch{}) || st.state != 1 {
		t.Fatalf("ContentTransition.Commit() state = %d, want 1", st.state)
	}
}

func TestNextContentIndexedPreservesWildcardBeforeElement(t *testing.T) {
	t.Parallel()

	elem := ElementID(1)
	wildcard := WildcardID(2)
	name := QName{Local: 1}
	model := ContentModelID(3)
	rt := publishedContentSchema(contentSchemaFixture{
		models: map[ContentModelID]CompiledModel{
			model: {
				Kind: CompiledModelDFA,
				Rows: []CompiledModelRow{
					{
						index: dfaRowIndex{
							nameToEdge:    map[QName]uint32{name: 1},
							wildcardEdges: []uint32{0},
						},
						Edges: []CompiledModelEdge{
							{
								Particle: WildcardParticle(wildcard, Occurrence{Min: 1, Max: 1}),
								To:       1,
							},
							{
								Particle: ElementParticle(elem, Occurrence{Min: 1, Max: 1}),
								To:       2,
							},
						},
					},
					{Accept: true},
					{Accept: true},
				},
			},
		},
		elementNames: map[ElementID]QName{elem: name},
		wildcards:    map[WildcardID]Wildcard{wildcard: {Mode: WildcardAny, Process: ProcessSkip}},
	})
	st := ContentState{model: model, present: true}
	transition, status := rt.NextContent(st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &ContentScratch{})
	kind, element := transition.Match()
	if status != ContentTransitionMatched || kind != ContentMatchSkip || element != NoElement || st.state != 0 {
		t.Fatalf("NextContent(indexed order) = %v/%v/%v state %d, want wildcard skip and unchanged state 0", kind, element, status, st.state)
	}
	if !transition.Commit(&st, &ContentScratch{}) || st.state != 1 {
		t.Fatalf("ContentTransition.Commit() state = %d, want 1", st.state)
	}
}

func TestNextContentWildcardProcessContents(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	elem := ElementID(2)
	model := ContentModelID(3)
	tests := []struct {
		name        string
		process     ProcessContents
		xsiType     bool
		global      bool
		wantKind    ContentMatchKind
		wantElement ElementID
	}{
		{
			name:        "strict missing",
			process:     ProcessStrict,
			wantKind:    ContentMatchStrictMissing,
			wantElement: NoElement,
		},
		{
			name:        "strict missing with xsi type",
			process:     ProcessStrict,
			xsiType:     true,
			wantKind:    ContentMatchAssessUndeclared,
			wantElement: NoElement,
		},
		{
			name:        "lax declared",
			process:     ProcessLax,
			global:      true,
			wantKind:    ContentMatchDeclared,
			wantElement: elem,
		},
		{
			name:        "lax undeclared",
			process:     ProcessLax,
			wantKind:    ContentMatchAssessUndeclared,
			wantElement: NoElement,
		},
		{
			name:        "skip",
			process:     ProcessSkip,
			wantKind:    ContentMatchSkip,
			wantElement: NoElement,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			wildcard := WildcardID(4)
			var globalElements map[QName]ElementID
			if tt.global {
				globalElements = map[QName]ElementID{name: elem}
			}
			rt := publishedContentSchema(contentSchemaFixture{
				models: map[ContentModelID]CompiledModel{
					model: {
						Kind: CompiledModelDFA,
						Rows: []CompiledModelRow{
							{
								Edges: []CompiledModelEdge{{
									Particle: WildcardParticle(wildcard, Occurrence{Min: 1, Max: 1}),
									To:       1,
								}},
							},
							{Accept: true},
						},
					},
				},
				globalElements: globalElements,
				wildcards:      map[WildcardID]Wildcard{wildcard: {Mode: WildcardAny, Process: tt.process}},
			})
			st := ContentState{model: model, present: true}
			transition, status := rt.NextContent(st, ContentInput{
				Name:       RuntimeName{Known: true, Name: name, Local: "e"},
				HasXSIType: tt.xsiType,
			}, &ContentScratch{})
			kind, element := transition.Match()
			if status != ContentTransitionMatched || kind != tt.wantKind || element != tt.wantElement {
				t.Fatalf("NextContent(wildcard) = %v/%v/%v, want %v/%v/%v", kind, element, status, tt.wantKind, tt.wantElement, ContentTransitionMatched)
			}
		})
	}
}

func TestNextContentInvalidParticleReferenceIsInvalidState(t *testing.T) {
	t.Parallel()

	model := ContentModelID(4)
	child := ElementID(3)
	childName := QName{Namespace: EmptyNamespaceID, Local: 1}
	tests := []struct {
		name  string
		index dfaRowIndex
	}{
		{name: "linear"},
		{
			name: "indexed",
			index: dfaRowIndex{
				nameToEdge: map[QName]uint32{childName: 0},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rt := publishedContentSchema(contentSchemaFixture{
				models: map[ContentModelID]CompiledModel{
					model: {
						Kind: CompiledModelDFA,
						Rows: []CompiledModelRow{
							{
								index: tc.index,
								Edges: []CompiledModelEdge{{
									Particle: ElementParticle(child, Occurrence{Min: 1, Max: 1}),
									To:       1,
								}},
							},
							{Accept: true},
						},
					},
				},
			})
			st := ContentState{model: model, present: true}
			transition, status := rt.NextContent(st, ContentInput{
				Name: RuntimeName{Known: true, Name: childName, Local: "child"},
			}, &ContentScratch{})
			if status != ContentTransitionInvalid {
				kind, element := transition.Match()
				t.Fatalf("NextContent() = %v/%v/%v, want invalid state", kind, element, status)
			}
		})
	}
}

func TestCompleteContentInvalidDFAStateIsInvalid(t *testing.T) {
	t.Parallel()

	model := ContentModelID(4)
	rt := publishedContentSchema(contentSchemaFixture{
		models: map[ContentModelID]CompiledModel{
			model: {
				Kind: CompiledModelDFA,
				Rows: []CompiledModelRow{{Accept: true}},
			},
		},
	})
	status := rt.CompleteContent(ContentState{model: model, state: 1, present: true}, &ContentScratch{})
	if status != ContentCompletionInvalid {
		t.Fatalf("CompleteContent() = %v, want invalid", status)
	}
}

func TestNextContentCountSaturatesAtUint32Max(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	elem := ElementID(2)
	model := ContentModelID(3)
	particle := ElementParticle(elem, Occurrence{Min: 2, Unbounded: true})
	rt := publishedContentSchema(contentSchemaFixture{
		models: map[ContentModelID]CompiledModel{
			model: {
				Kind: CompiledModelDFA,
				Rows: []CompiledModelRow{{
					Accept:        true,
					Counted:       true,
					Unbounded:     true,
					Min:           2,
					CountParticle: particle,
					Edges: []CompiledModelEdge{{
						Particle: particle,
						To:       0,
					}},
				}},
			},
		},
		elementNames: map[ElementID]QName{elem: name},
	})
	st := ContentState{model: model, count: math.MaxUint32, present: true}
	transition, status := rt.NextContent(st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &ContentScratch{})
	kind, element := transition.Match()
	if status != ContentTransitionMatched || kind != ContentMatchDeclared || element != elem {
		t.Fatalf("NextContent() = %v/%v/%v, want matched valid transition", kind, element, status)
	}
	if !transition.Commit(&st, &ContentScratch{}) {
		t.Fatal("ContentTransition.Commit() rejected current counted state")
	}
	if st.count != math.MaxUint32 {
		t.Fatalf("Count = %d, want saturation at %d", st.count, uint32(math.MaxUint32))
	}
	if status := rt.CompleteContent(st, &ContentScratch{}); status != ContentCompletionComplete {
		t.Fatalf("CompleteContent() after saturated count = %v, want complete", status)
	}
}

func TestContentTransitionRejectsNonMatchKinds(t *testing.T) {
	t.Parallel()

	state := ContentState{model: 1, present: true}
	transition := newContentTransition(state, state, contentMatch{})
	if transition.IsPlanned() {
		t.Fatal("newContentTransition(zero) planned a transition")
	}
	kind, element := transition.Match()
	if kind != ContentMatchInvalid || element != NoElement {
		t.Fatalf("rejected transition match = %v/%v, want invalid/no element", kind, element)
	}
}

func TestContentMatchRepresentationDoesNotGrow(t *testing.T) {
	t.Parallel()

	if got := unsafe.Sizeof(contentMatch{}); got > 8 {
		t.Fatalf("contentMatch size = %d, want at most 8", got)
	}
	wantTransition := uintptr(48)
	if strconv.IntSize == 64 {
		wantTransition = 56
	}
	if got := unsafe.Sizeof(ContentTransition{}); got > wantTransition {
		t.Fatalf("ContentTransition size = %d, want at most %d", got, wantTransition)
	}
}

type contentSchemaFixture struct {
	contentModels      map[TypeID]ContentModelID
	models             map[ContentModelID]CompiledModel
	globalElements     map[QName]ElementID
	elementNames       map[ElementID]QName
	wildcards          map[WildcardID]Wildcard
	substitutionLookup map[ElementID]map[QName]ElementID
}

func publishedContentSchema(s contentSchemaFixture) *Schema {
	maxComplex := -1
	for typ := range s.contentModels {
		if id, ok := typ.Complex(); ok && int(id) > maxComplex {
			maxComplex = int(id)
		}
	}
	complexTypes := make([]complexTypeRead, maxComplex+1)
	for typ, model := range s.contentModels {
		if id, ok := typ.Complex(); ok {
			complexTypes[id].contentModel = model
		}
	}

	maxModel := -1
	for id := range s.models {
		if int(id) > maxModel {
			maxModel = int(id)
		}
	}
	models := make([]CompiledModel, maxModel+1)
	for id, model := range s.models {
		models[id] = model
	}

	maxElement := -1
	for id := range s.elementNames {
		if int(id) > maxElement {
			maxElement = int(id)
		}
	}
	elementNames := make([]QName, maxElement+1)
	elements := make([]ElementDecl, maxElement+1)
	for id, name := range s.elementNames {
		elementNames[id] = name
		elements[id] = ElementDecl{Name: name}
	}

	maxWildcard := -1
	for id := range s.wildcards {
		if int(id) > maxWildcard {
			maxWildcard = int(id)
		}
	}
	wildcards := make([]WildcardView, maxWildcard+1)
	for id, wildcard := range s.wildcards {
		wildcards[id] = newWildcardView(nil, &wildcard)
	}

	return &Schema{program: schemaProgram{
		GlobalElements: s.globalElements,
		Substitutions:  testSubstitutionTable(s.substitutionLookup, len(elements)),
		ComplexTypes:   complexTypes,
		CompiledModels: testCompiledModelReads(models),
		Elements:       newElementReadTable(elements, nil),
		Wildcards:      wildcards,
	}}
}

func testSubstitutionTable(lookup map[ElementID]map[QName]ElementID, elementCount int) SubstitutionTable {
	if len(lookup) == 0 {
		return SubstitutionTable{}
	}
	table := SubstitutionTable{spans: make([]substitutionSpan, elementCount)}
	for head := range elementCount {
		members := lookup[ElementID(head)]
		table.spans[head] = substitutionSpan{start: len(table.entries), count: len(members)}
		for name, member := range members {
			table.entries = append(table.entries, substitutionEntry{name: name, member: member, effective: true})
		}
		slices.SortFunc(table.entries[table.spans[head].start:], compareSubstitutionEntry)
	}
	return table
}
