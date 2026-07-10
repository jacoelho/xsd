package runtime

import (
	"math"
	"testing"
)

func TestContentFrameForTypeUsesCompiledModelInitialState(t *testing.T) {
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
	got := rt.ContentFrame(typ)
	state := got.ContentState()
	if state.model != modelID || state.state != 7 || got.AllBitLen() != 130 {
		t.Fatalf("ContentFrame() = %+v, want model %v state 7 bitLen 130", got, modelID)
	}

	got = publishedContentSchema(contentSchemaFixture{
		contentModels: map[TypeID]ContentModelID{typ: NoContentModel},
	}).ContentFrame(typ)
	if got.ContentState().HasModel() {
		t.Fatalf("ContentFrame(no model) = %+v, want no model", got)
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

func TestEqualCompiledModelViewProjection(t *testing.T) {
	t.Parallel()

	model := CompiledModel{
		Source:    1,
		Kind:      CompiledModelAll,
		Start:     2,
		AllBitLen: 1,
		All: []CompiledAllTerm{{
			Particle: ElementParticle(3, Occurrence{Min: 1, Max: 1}),
			Required: true,
		}},
	}
	view := NewCompiledModelView(&model)
	if !EqualCompiledModelViewProjection(view, &model) {
		t.Fatal("EqualCompiledModelViewProjection() = false, want true")
	}

	changed := model
	changed.Kind = CompiledModelEmpty
	changed.All = nil
	if EqualCompiledModelViewProjection(view, &changed) {
		t.Fatal("EqualCompiledModelViewProjection() = true for different model")
	}
}

func TestCompiledModelViewProjectionTable(t *testing.T) {
	t.Parallel()

	models := []CompiledModel{
		{
			Source:    1,
			Kind:      CompiledModelAll,
			Start:     2,
			AllBitLen: 1,
			All: []CompiledAllTerm{{
				Particle: ElementParticle(3, Occurrence{Min: 1, Max: 1}),
				Required: true,
			}},
		},
		{Source: 4, Kind: CompiledModelEmpty, Empty: true},
	}
	views := NewCompiledModelViews(models)
	if !EqualCompiledModelViewProjectionTable(views, models) {
		t.Fatalf("NewCompiledModelViews() = %#v, want projection for %#v", views, models)
	}
	if got, ok := CompiledModelViewByID(views, 1); !ok || !EqualCompiledModelViews(got, views[1]) {
		t.Fatalf("CompiledModelViewByID() = %#v, %v; want view 1, true", got, ok)
	}
	if got, ok := CompiledModelViewByID(views, ContentModelID(99)); ok || !EqualCompiledModelViews(got, CompiledModelView{}) {
		t.Fatalf("CompiledModelViewByID(invalid) = %#v, %v; want zero, false", got, ok)
	}
	if EqualCompiledModelViewProjectionTable(views[:1], models) {
		t.Fatal("EqualCompiledModelViewProjectionTable() accepted mismatched table length")
	}

	changed := append([]CompiledModel(nil), models...)
	changed[0].Kind = CompiledModelEmpty
	changed[0].All = nil
	if EqualCompiledModelViewProjectionTable(views, changed) {
		t.Fatal("EqualCompiledModelViewProjectionTable() accepted mismatched model")
	}
	if err := ValidateCompiledModelViewProjectionTable(NewCompiledModelViews(models), models); err != nil {
		t.Fatalf("ValidateCompiledModelViewProjectionTable() error = %v", err)
	}
	if err := ValidateCompiledModelViewProjectionTable(views[:1], models); err == nil || err.Error() != "compiled model view projection count does not match compiled models" {
		t.Fatalf("ValidateCompiledModelViewProjectionTable(short) error = %v, want count invariant", err)
	}
	if err := ValidateCompiledModelViewProjectionTable(views, changed); err == nil || err.Error() != "compiled model view projection does not match compiled model" {
		t.Fatalf("ValidateCompiledModelViewProjectionTable(changed) error = %v, want mismatch invariant", err)
	}
}

func TestAdvanceContentAnyReturnsGlobalElement(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	elem := ElementID(2)
	rt := publishedContentSchema(contentSchemaFixture{
		models:         map[ContentModelID]CompiledModel{0: {Kind: CompiledModelAny}},
		globalElements: map[QName]ElementID{name: elem},
	})
	st := ContentState{model: 0, present: true}
	match, status := rt.AdvanceContent(&st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &ContentScratch{})
	if status != ContentAdvanceMatched || match.Element != elem || match.Skip || match.StrictMissing {
		t.Fatalf("AdvanceContent(any) = %+v/%v, want global element", match, status)
	}
}

func TestAdvanceContentAllMarksScratchAndCompletes(t *testing.T) {
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
	match, status := rt.AdvanceContent(&st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &scratch)
	if status != ContentAdvanceMatched || match.Element != elem {
		t.Fatalf("AdvanceContent(all) = %+v/%v, want element", match, status)
	}
	seen, ok := scratch.AllSeen(0)
	if !ok || !seen {
		t.Fatalf("AllSeen(0) = %v/%v, want true/true", seen, ok)
	}
	if status := rt.CompleteContent(st, &scratch); status != ContentCompletionComplete {
		t.Fatalf("CompleteContent(all) = %v, want complete", status)
	}
}

func TestAdvanceContentIndexedSubstitutionReturnsMember(t *testing.T) {
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
						Index: DFARowIndex{Enabled: true, NameToEdge: map[QName]uint32{memberName: 0}},
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
	match, status := rt.AdvanceContent(&st, ContentInput{
		Name: RuntimeName{Known: true, Name: memberName, Local: "member"},
	}, &ContentScratch{})
	if status != ContentAdvanceMatched || match.Element != member || st.state != 1 {
		t.Fatalf("AdvanceContent(indexed substitution) = %+v/%v state %d, want member state 1", match, status, st.state)
	}
}

func TestAdvanceContentIndexedPreservesWildcardBeforeElement(t *testing.T) {
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
						Index: DFARowIndex{
							Enabled:       true,
							NameToEdge:    map[QName]uint32{name: 1},
							WildcardEdges: []uint32{0},
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
	match, status := rt.AdvanceContent(&st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &ContentScratch{})
	if status != ContentAdvanceMatched || !match.Skip || match.Element != NoElement || st.state != 1 {
		t.Fatalf("AdvanceContent(indexed order) = %+v/%v state %d, want wildcard skip state 1", match, status, st.state)
	}
}

func TestAdvanceContentWildcardProcessContents(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	elem := ElementID(2)
	model := ContentModelID(3)
	tests := []struct {
		name       string
		process    ProcessContents
		xsiType    bool
		global     bool
		want       ContentMatch
		wantMatch  bool
		wantStrict bool
	}{
		{
			name:       "strict missing",
			process:    ProcessStrict,
			want:       ContentMatch{Element: NoElement, StrictMissing: true},
			wantMatch:  true,
			wantStrict: true,
		},
		{
			name:      "strict missing with xsi type",
			process:   ProcessStrict,
			xsiType:   true,
			want:      NoContentMatch(),
			wantMatch: true,
		},
		{
			name:      "lax declared",
			process:   ProcessLax,
			global:    true,
			want:      ContentMatch{Element: elem},
			wantMatch: true,
		},
		{
			name:      "skip",
			process:   ProcessSkip,
			want:      ContentMatch{Element: NoElement, Skip: true},
			wantMatch: true,
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
			match, status := rt.AdvanceContent(&st, ContentInput{
				Name:       RuntimeName{Known: true, Name: name, Local: "e"},
				HasXSIType: tt.xsiType,
			}, &ContentScratch{})
			wantStatus := ContentAdvanceNoMatch
			if tt.wantMatch {
				wantStatus = ContentAdvanceMatched
			}
			if status != wantStatus || match != tt.want {
				t.Fatalf("AdvanceContent(wildcard) = %+v/%v, want %+v/%v", match, status, tt.want, wantStatus)
			}
			if match.StrictMissing != tt.wantStrict {
				t.Fatalf("StrictMissing = %v, want %v", match.StrictMissing, tt.wantStrict)
			}
		})
	}
}

func TestAdvanceContentInvalidParticleReferenceIsInvalidState(t *testing.T) {
	t.Parallel()

	model := ContentModelID(4)
	child := ElementID(3)
	childName := QName{Namespace: EmptyNamespaceID, Local: 1}
	tests := []struct {
		name  string
		index DFARowIndex
	}{
		{name: "linear"},
		{
			name: "indexed",
			index: DFARowIndex{
				NameToEdge: map[QName]uint32{childName: 0},
				Enabled:    true,
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
								Index: tc.index,
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
			match, status := rt.AdvanceContent(&st, ContentInput{
				Name: RuntimeName{Known: true, Name: childName, Local: "child"},
			}, &ContentScratch{})
			if status != ContentAdvanceInvalid {
				t.Fatalf("AdvanceContent() = %+v/%v, want invalid state", match, status)
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

func TestAdvanceContentCountSaturatesAtUint32Max(t *testing.T) {
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
	match, status := rt.AdvanceContent(&st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &ContentScratch{})
	if status != ContentAdvanceMatched || match.Element != elem {
		t.Fatalf("AdvanceContent() = %+v/%v, want matched valid transition", match, status)
	}
	if st.count != math.MaxUint32 {
		t.Fatalf("Count = %d, want saturation at %d", st.count, uint32(math.MaxUint32))
	}
	if status := rt.CompleteContent(st, &ContentScratch{}); status != ContentCompletionComplete {
		t.Fatalf("CompleteContent() after saturated count = %v, want complete", status)
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
	for id, name := range s.elementNames {
		elementNames[id] = name
	}

	maxWildcard := -1
	for id := range s.wildcards {
		if int(id) > maxWildcard {
			maxWildcard = int(id)
		}
	}
	wildcards := make([]WildcardView, maxWildcard+1)
	for id, wildcard := range s.wildcards {
		wildcards[id] = NewWildcardView(nil, &wildcard)
	}

	return &Schema{runtime: schemaRuntime{
		GlobalElements:     s.globalElements,
		SubstitutionLookup: s.substitutionLookup,
		ComplexTypes:       complexTypes,
		CompiledModels:     NewBorrowedCompiledModelViews(models),
		ElementNames:       elementNames,
		Wildcards:          wildcards,
	}}
}
