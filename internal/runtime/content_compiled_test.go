package runtime

import (
	"math"
	"testing"
)

func TestContentFrameForTypeUsesCompiledModelInitialState(t *testing.T) {
	t.Parallel()

	typ := ComplexRef(1)
	modelID := ContentModelID(2)
	rt := compiledContentRuntimeStub{
		contentModels: map[TypeID]ContentModelID{
			typ: modelID,
		},
		models: map[ContentModelID]CompiledModel{
			modelID: {Start: 7, AllBitLen: 130},
		},
	}
	got := ContentFrameForType(rt, typ)
	state := got.ContentState()
	if state.model != modelID || state.state != 7 || got.AllBitLen() != 130 {
		t.Fatalf("ContentFrameForType() = %+v, want model %v state 7 bitLen 130", got, modelID)
	}

	got = ContentFrameForType(compiledContentRuntimeStub{}, typ)
	if got.ContentState().HasModel() {
		t.Fatalf("ContentFrameForType(no model) = %+v, want no model", got)
	}
	got = ContentFrameForType(compiledContentRuntimeStub{
		contentModels: map[TypeID]ContentModelID{
			typ: modelID,
		},
	}, typ)
	state = got.ContentState()
	if state.model != modelID || state.state != 0 || got.AllBitLen() != 0 {
		t.Fatalf("ContentFrameForType(missing compiled model) = %+v, want model %v with zero state", got, modelID)
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

func TestAdvanceCompiledContentAnyReturnsGlobalElement(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	elem := ElementID(2)
	rt := compiledContentRuntimeStub{
		models:         map[ContentModelID]CompiledModel{0: {Kind: CompiledModelAny}},
		globalElements: map[QName]ElementID{name: elem},
	}
	st := ContentState{model: 0, present: true}
	match, matched, valid := AdvanceCompiledContent(rt, &st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &ContentScratch{})
	if !valid || !matched || match.Element != elem || match.Skip || match.StrictMissing {
		t.Fatalf("AdvanceCompiledContent(any) = %+v/%v/%v, want global element", match, matched, valid)
	}
}

func TestAdvanceCompiledContentAllMarksScratchAndCompletes(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	elem := ElementID(2)
	model := ContentModelID(3)
	rt := compiledContentRuntimeStub{
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
	}
	scratch := NewContentScratch(make([]uint64, 1), 0, 1)
	st := ContentState{model: model, present: true}
	match, matched, valid := AdvanceCompiledContent(rt, &st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &scratch)
	if !valid || !matched || match.Element != elem {
		t.Fatalf("AdvanceCompiledContent(all) = %+v/%v/%v, want element", match, matched, valid)
	}
	seen, ok := scratch.AllSeen(0)
	if !ok || !seen {
		t.Fatalf("AllSeen(0) = %v/%v, want true/true", seen, ok)
	}
	complete, ok := CompleteCompiledContent(rt, st, &scratch)
	if !ok || !complete {
		t.Fatalf("CompleteCompiledContent(all) = %v/%v, want complete", complete, ok)
	}
}

func TestAdvanceCompiledContentIndexedSubstitutionReturnsMember(t *testing.T) {
	t.Parallel()

	head := ElementID(1)
	member := ElementID(2)
	headName := QName{Local: 1}
	memberName := QName{Local: 2}
	model := ContentModelID(3)
	rt := compiledContentRuntimeStub{
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
	}
	st := ContentState{model: model, present: true}
	match, matched, valid := AdvanceCompiledContent(rt, &st, ContentInput{
		Name: RuntimeName{Known: true, Name: memberName, Local: "member"},
	}, &ContentScratch{})
	if !valid || !matched || match.Element != member || st.state != 1 {
		t.Fatalf("AdvanceCompiledContent(indexed substitution) = %+v/%v/%v state %d, want member state 1", match, matched, valid, st.state)
	}
}

func TestAdvanceCompiledContentIndexedPreservesWildcardBeforeElement(t *testing.T) {
	t.Parallel()

	elem := ElementID(1)
	wildcard := WildcardID(2)
	name := QName{Local: 1}
	model := ContentModelID(3)
	rt := compiledContentRuntimeStub{
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
	}
	st := ContentState{model: model, present: true}
	match, matched, valid := AdvanceCompiledContent(rt, &st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &ContentScratch{})
	if !valid || !matched || !match.Skip || match.Element != NoElement || st.state != 1 {
		t.Fatalf("AdvanceCompiledContent(indexed order) = %+v/%v/%v state %d, want wildcard skip state 1", match, matched, valid, st.state)
	}
}

func TestAdvanceCompiledContentWildcardProcessContents(t *testing.T) {
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
			rt := compiledContentRuntimeStub{
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
				wildcards: map[WildcardID]Wildcard{wildcard: {Mode: WildcardAny, Process: tt.process}},
			}
			if tt.global {
				rt.globalElements = map[QName]ElementID{name: elem}
			}
			st := ContentState{model: model, present: true}
			match, matched, valid := AdvanceCompiledContent(rt, &st, ContentInput{
				Name:       RuntimeName{Known: true, Name: name, Local: "e"},
				HasXSIType: tt.xsiType,
			}, &ContentScratch{})
			if !valid || matched != tt.wantMatch || match != tt.want {
				t.Fatalf("AdvanceCompiledContent(wildcard) = %+v/%v/%v, want %+v/%v/true", match, matched, valid, tt.want, tt.wantMatch)
			}
			if match.StrictMissing != tt.wantStrict {
				t.Fatalf("StrictMissing = %v, want %v", match.StrictMissing, tt.wantStrict)
			}
		})
	}
}

func TestAdvanceCompiledContentInvalidParticleReferenceIsInvalidState(t *testing.T) {
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

			rt := compiledContentRuntimeStub{
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
			}
			st := ContentState{model: model, present: true}
			match, matched, valid := AdvanceCompiledContent(rt, &st, ContentInput{
				Name: RuntimeName{Known: true, Name: childName, Local: "child"},
			}, &ContentScratch{})
			if valid || matched {
				t.Fatalf("AdvanceCompiledContent() = %+v/%v/%v, want invalid state", match, matched, valid)
			}
		})
	}
}

func TestCompleteCompiledContentInvalidDFAStateIsInvalid(t *testing.T) {
	t.Parallel()

	model := ContentModelID(4)
	rt := compiledContentRuntimeStub{
		models: map[ContentModelID]CompiledModel{
			model: {
				Kind: CompiledModelDFA,
				Rows: []CompiledModelRow{{Accept: true}},
			},
		},
	}
	complete, ok := CompleteCompiledContent(rt, ContentState{model: model, state: 1, present: true}, &ContentScratch{})
	if ok || complete {
		t.Fatalf("CompleteCompiledContent() = %v/%v, want invalid", complete, ok)
	}
}

func TestAdvanceCompiledContentCountSaturatesAtUint32Max(t *testing.T) {
	t.Parallel()

	name := QName{Local: 1}
	elem := ElementID(2)
	model := ContentModelID(3)
	particle := ElementParticle(elem, Occurrence{Min: 2, Unbounded: true})
	rt := compiledContentRuntimeStub{
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
	}
	st := ContentState{model: model, count: math.MaxUint32, present: true}
	match, matched, valid := AdvanceCompiledContent(rt, &st, ContentInput{
		Name: RuntimeName{Known: true, Name: name, Local: "e"},
	}, &ContentScratch{})
	if !valid || !matched || match.Element != elem {
		t.Fatalf("AdvanceCompiledContent() = %+v/%v/%v, want matched valid transition", match, matched, valid)
	}
	if st.count != math.MaxUint32 {
		t.Fatalf("Count = %d, want saturation at %d", st.count, uint32(math.MaxUint32))
	}
	complete, ok := CompleteCompiledContent(rt, st, &ContentScratch{})
	if !ok || !complete {
		t.Fatalf("CompleteCompiledContent() after saturated count = %v/%v, want complete", complete, ok)
	}
}

type compiledContentRuntimeStub struct {
	contentModels      map[TypeID]ContentModelID
	models             map[ContentModelID]CompiledModel
	globalElements     map[QName]ElementID
	elementNames       map[ElementID]QName
	wildcards          map[WildcardID]Wildcard
	substitutionLookup map[ElementID]map[QName]ElementID
}

func (s compiledContentRuntimeStub) ContentModelForType(id TypeID) ContentModelID {
	if s.contentModels == nil {
		return NoContentModel
	}
	return s.contentModels[id]
}

func (s compiledContentRuntimeStub) CompiledContentModelView(id ContentModelID) (CompiledModelView, bool) {
	model, ok := s.models[id]
	if !ok {
		return CompiledModelView{}, false
	}
	return NewCompiledModelView(&model), true
}

func (s compiledContentRuntimeStub) GlobalElement(name QName) (ElementID, bool) {
	id, ok := s.globalElements[name]
	return id, ok
}

func (s compiledContentRuntimeStub) ElementName(id ElementID) (QName, bool) {
	name, ok := s.elementNames[id]
	return name, ok
}

func (s compiledContentRuntimeStub) WildcardView(id WildcardID) (WildcardView, bool) {
	w, ok := s.wildcards[id]
	if !ok {
		return WildcardView{}, false
	}
	return NewWildcardView(nil, &w), true
}

func (s compiledContentRuntimeStub) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	members := s.substitutionLookup[id]
	if members == nil {
		return NoElement, false
	}
	member, ok := members[name]
	return member, ok
}
