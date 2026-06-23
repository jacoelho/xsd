package runtime

import (
	"strings"
	"testing"
)

func TestCompiledModelKindValidity(t *testing.T) {
	t.Parallel()

	for _, kind := range []CompiledModelKind{
		CompiledModelEmpty,
		CompiledModelAny,
		CompiledModelAll,
		CompiledModelDFA,
	} {
		if !ValidCompiledModelKind(kind) {
			t.Fatalf("ValidCompiledModelKind(%d) = false", kind)
		}
	}
	if ValidCompiledModelKind(CompiledModelKind(99)) {
		t.Fatal("invalid compiled model kind was accepted")
	}
}

func TestDFARowIndexIsEnabled(t *testing.T) {
	t.Parallel()

	if (DFARowIndex{}).IsEnabled() {
		t.Fatal("zero-value row index is enabled")
	}
	if !(DFARowIndex{Enabled: true}).IsEnabled() {
		t.Fatal("enabled row index reported disabled")
	}
}

func TestEqualCompiledModels(t *testing.T) {
	t.Parallel()

	row := CompiledModelRow{
		Edges: []CompiledModelEdge{{Particle: ElementParticle(1, Occurrence{Min: 1, Max: 1}), To: 1}},
		Index: DFARowIndex{
			NameToEdge:    map[QName]uint32{QName{Local: 1}: 0},
			WildcardEdges: []uint32{1},
			Enabled:       true,
		},
	}
	model := CompiledModel{
		Rows:      []CompiledModelRow{row},
		All:       []CompiledAllTerm{{Particle: ElementParticle(1, Occurrence{Min: 1, Max: 1}), Required: true}},
		Source:    1,
		Start:     0,
		AllBitLen: 1,
		Kind:      CompiledModelDFA,
		Mixed:     true,
		Empty:     false,
	}
	if !EqualCompiledModels(model, model) {
		t.Fatal("EqualCompiledModels() rejected identical models")
	}
	drifted := model
	drifted.Rows = []CompiledModelRow{row}
	drifted.Rows[0].Index.NameToEdge = map[QName]uint32{QName{Local: 1}: 1}
	if EqualCompiledModels(model, drifted) {
		t.Fatal("EqualCompiledModels() accepted row-index drift")
	}
}

func TestCompiledCountingException(t *testing.T) {
	t.Parallel()

	p := ElementParticle(1, Occurrence{Min: 1, Max: 1})
	row := CompiledModelRow{
		Counted:       true,
		CountParticle: p,
		Min:           2,
		Max:           2,
	}
	loop := CompiledModelEdge{Particle: p, To: 0}
	exit := CompiledModelEdge{Particle: p, To: 1}
	if !CompiledCountingException(0, row, loop, exit) {
		t.Fatal("CompiledCountingException() rejected loop/exit pair")
	}
	row.Unbounded = true
	if CompiledCountingException(0, row, loop, exit) {
		t.Fatal("CompiledCountingException() accepted unbounded row")
	}
}

func TestIndexCompiledModelRows(t *testing.T) {
	t.Parallel()

	const head = ElementID(1)
	headName := QName{Local: 1}
	subName := QName{Local: 10}
	elementNames := map[ElementID]QName{head: headName}
	one := Occurrence{Min: 1, Max: 1}
	edges := []CompiledModelEdge{
		{Particle: ElementParticle(head, one), To: 1},
		{Particle: WildcardParticle(1, one), To: 1},
	}
	for id := ElementID(2); len(edges) < CompiledDFARowIndexMinEdges; id++ {
		elementNames[id] = QName{Local: LocalNameID(id)}
		edges = append(edges, CompiledModelEdge{Particle: ElementParticle(id, one), To: 1})
	}
	rt := dfaRowIndexRuntimeStub{
		elementNames: elementNames,
		substitutionByName: map[ElementID]map[QName]ElementID{
			head: {subName: ElementID(2)},
		},
	}
	model := CompiledModel{
		Kind: CompiledModelDFA,
		Rows: []CompiledModelRow{
			{Edges: edges},
			{Edges: edges[:CompiledDFARowIndexMinEdges-1]},
		},
	}
	if err := IndexCompiledModelRows(rt, &model); err != nil {
		t.Fatalf("IndexCompiledModelRows() error = %v", err)
	}
	if !model.Rows[0].Index.IsEnabled() {
		t.Fatal("wide row was not indexed")
	}
	if got, ok := model.Rows[0].Index.NameToEdge[headName]; !ok || got != 0 {
		t.Fatalf("head index = %d, %v; want 0, true", got, ok)
	}
	if got, ok := model.Rows[0].Index.NameToEdge[subName]; !ok || got != 0 {
		t.Fatalf("substitution index = %d, %v; want 0, true", got, ok)
	}
	if got := model.Rows[0].Index.WildcardEdges; len(got) != 1 || got[0] != 1 {
		t.Fatalf("wildcard index = %v, want [1]", got)
	}
	if model.Rows[1].Index.IsEnabled() {
		t.Fatal("narrow row was indexed")
	}
}

func TestIndexCompiledModelRowsKeepsLinearScanForDuplicateNames(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	duplicateName := QName{Local: 1}
	elementNames := map[ElementID]QName{
		1: duplicateName,
		2: duplicateName,
	}
	var edges []CompiledModelEdge
	for id := ElementID(1); len(edges) < CompiledDFARowIndexMinEdges; id++ {
		if _, ok := elementNames[id]; !ok {
			elementNames[id] = QName{Local: LocalNameID(id)}
		}
		edges = append(edges, CompiledModelEdge{Particle: ElementParticle(id, one), To: 1})
	}
	model := CompiledModel{
		Kind: CompiledModelDFA,
		Rows: []CompiledModelRow{{Edges: edges}},
	}
	if err := IndexCompiledModelRows(dfaRowIndexRuntimeStub{elementNames: elementNames}, &model); err != nil {
		t.Fatalf("IndexCompiledModelRows() error = %v", err)
	}
	if model.Rows[0].Index.IsEnabled() {
		t.Fatal("ambiguous row was indexed")
	}
}

func TestIndexCompiledModelRowsRejectsInvalidElement(t *testing.T) {
	t.Parallel()

	one := Occurrence{Min: 1, Max: 1}
	edges := make([]CompiledModelEdge, CompiledDFARowIndexMinEdges)
	for i := range edges {
		edges[i] = CompiledModelEdge{Particle: ElementParticle(ElementID(i+1), one), To: 1}
	}
	model := CompiledModel{
		Kind: CompiledModelDFA,
		Rows: []CompiledModelRow{{Edges: edges}},
	}
	err := IndexCompiledModelRows(dfaRowIndexRuntimeStub{}, &model)
	if err == nil || !strings.Contains(err.Error(), "compiled content model index references invalid element") {
		t.Fatalf("IndexCompiledModelRows() error = %v", err)
	}
}

type dfaRowIndexRuntimeStub struct {
	elementNames       map[ElementID]QName
	substitutionIDs    map[ElementID][]ElementID
	substitutionByName map[ElementID]map[QName]ElementID
	contentModels      map[ContentModelID]ContentModel
	wildcards          map[WildcardID]Wildcard
}

func (s dfaRowIndexRuntimeStub) ContentModel(id ContentModelID) (ContentModel, bool) {
	model, ok := s.contentModels[id]
	return model, ok
}

func (s dfaRowIndexRuntimeStub) ElementName(id ElementID) (QName, bool) {
	name, ok := s.elementNames[id]
	return name, ok
}

func (s dfaRowIndexRuntimeStub) Wildcard(id WildcardID) (Wildcard, bool) {
	w, ok := s.wildcards[id]
	return w, ok
}

func (s dfaRowIndexRuntimeStub) ForEachSubstitutionMember(id ElementID, fn func(ElementID) bool) {
	for _, member := range s.substitutionIDs[id] {
		if !fn(member) {
			return
		}
	}
}

func (s dfaRowIndexRuntimeStub) SubstitutionMemberByName(id ElementID, name QName) (ElementID, bool) {
	members := s.substitutionByName[id]
	if members == nil {
		return NoElement, false
	}
	member, ok := members[name]
	return member, ok
}

func (s dfaRowIndexRuntimeStub) SubstitutionMembersByName(id ElementID) map[QName]ElementID {
	return s.substitutionByName[id]
}

func TestValidateCompiledModelRuntime(t *testing.T) {
	t.Parallel()

	const (
		sourceID ContentModelID = 1
		child    ElementID      = 1
		other    ElementID      = 2
	)
	childName := QName{Local: 1}
	otherName := QName{Local: 2}
	rt := dfaRowIndexRuntimeStub{
		elementNames: map[ElementID]QName{
			child: childName,
			other: otherName,
		},
	}
	one := Occurrence{Min: 1, Max: 1}
	optional := Occurrence{Min: 0, Max: 1}
	sourceEmpty := ContentModel{Kind: ModelEmpty}
	emptyModel := CompiledModel{Kind: CompiledModelEmpty, Source: sourceID, Empty: true}
	sourceAll := ContentModel{
		Kind:      ModelAll,
		Occurs:    one,
		Particles: []Particle{ElementParticle(child, one)},
	}
	allModel := CompiledModel{
		Kind:      CompiledModelAll,
		Source:    sourceID,
		AllBitLen: 1,
		All:       []CompiledAllTerm{{Particle: ElementParticle(child, one), Required: true}},
	}
	sourceDFA := ContentModel{
		Kind:      ModelSequence,
		Occurs:    one,
		Particles: []Particle{ElementParticle(child, one)},
	}
	dfaModel := CompiledModel{
		Kind:   CompiledModelDFA,
		Source: sourceID,
		Rows: []CompiledModelRow{
			{
				Edges: []CompiledModelEdge{{Particle: ElementParticle(child, one), To: 1}},
			},
			{Accept: true},
		},
	}

	tests := []struct {
		name    string
		wantErr string
		source  ContentModel
		model   CompiledModel
	}{
		{
			name:   "empty",
			source: sourceEmpty,
			model:  emptyModel,
		},
		{
			name:   "all",
			source: sourceAll,
			model:  allModel,
		},
		{
			name:   "DFA",
			source: sourceDFA,
			model:  dfaModel,
		},
		{
			name:    "source id drift",
			source:  sourceEmpty,
			model:   CompiledModel{Kind: CompiledModelEmpty, Source: NoContentModel, Empty: true},
			wantErr: "compiled content model source does not match model slot",
		},
		{
			name:    "empty stores active DFA row",
			source:  sourceEmpty,
			model:   CompiledModel{Kind: CompiledModelEmpty, Source: sourceID, Empty: true, Rows: []CompiledModelRow{{}}},
			wantErr: "compiled empty/any content model stores inactive fields",
		},
		{
			name:   "all bit length drift",
			source: sourceAll,
			model: CompiledModel{
				Kind:   CompiledModelAll,
				Source: sourceID,
				All:    []CompiledAllTerm{{Particle: ElementParticle(child, one), Required: true}},
			},
			wantErr: "compiled all content model bit length does not match terms",
		},
		{
			name:   "DFA edge target invalid",
			source: sourceDFA,
			model: CompiledModel{
				Kind:   CompiledModelDFA,
				Source: sourceID,
				Rows: []CompiledModelRow{
					{Edges: []CompiledModelEdge{{Particle: ElementParticle(child, one), To: 2}}},
					{Accept: true},
				},
			},
			wantErr: "compiled content model edge target is invalid",
		},
		{
			name:   "DFA row has overlapping particles",
			source: sourceDFA,
			model: CompiledModel{
				Kind:   CompiledModelDFA,
				Source: sourceID,
				Rows: []CompiledModelRow{
					{
						Edges: []CompiledModelEdge{
							{Particle: ElementParticle(child, one), To: 1},
							{Particle: ElementParticle(child, optional), To: 1},
						},
					},
					{Accept: true},
				},
			},
			wantErr: "compiled content model row has overlapping particles",
		},
		{
			name:   "DFA fixed counting exception",
			source: sourceDFA,
			model: CompiledModel{
				Kind:   CompiledModelDFA,
				Source: sourceID,
				Rows: []CompiledModelRow{
					{
						Edges: []CompiledModelEdge{
							{Particle: ElementParticle(child, one), To: 0},
							{Particle: ElementParticle(child, optional), To: 1},
						},
						Counted:       true,
						CountParticle: ElementParticle(child, one),
						Min:           2,
						Max:           2,
					},
					{Accept: true},
				},
			},
		},
		{
			name:   "DFA counted range invalid",
			source: sourceDFA,
			model: CompiledModel{
				Kind:   CompiledModelDFA,
				Source: sourceID,
				Rows: []CompiledModelRow{
					{
						Counted:       true,
						CountParticle: ElementParticle(child, one),
						Min:           2,
						Max:           1,
					},
				},
			},
			wantErr: "compiled content model counted state has invalid range",
		},
		{
			name:   "DFA inactive row index stores data",
			source: sourceDFA,
			model: CompiledModel{
				Kind:   CompiledModelDFA,
				Source: sourceID,
				Rows: []CompiledModelRow{
					{
						Edges: []CompiledModelEdge{{Particle: ElementParticle(child, one), To: 1}},
						Index: DFARowIndex{NameToEdge: map[QName]uint32{childName: 0}},
					},
					{Accept: true},
				},
			},
			wantErr: "compiled content model inactive row index stores data",
		},
		{
			name:   "DFA stale particle inactive field",
			source: sourceDFA,
			model: CompiledModel{
				Kind:   CompiledModelDFA,
				Source: sourceID,
				Rows: []CompiledModelRow{
					{
						Edges: []CompiledModelEdge{{
							Particle: Particle{Kind: ParticleElement, Element: child, Model: NoContentModel, Wildcard: WildcardID(0), Occurs: one},
							To:       1,
						}},
					},
					{Accept: true},
				},
			},
			wantErr: "particle stores wildcard ID for non-wildcard kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCompiledModelRuntime(nil, rt, sourceID, tt.source, tt.model)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateCompiledModelRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateCompiledModelRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCompiledModelsRuntime(t *testing.T) {
	t.Parallel()

	source := ContentModel{Kind: ModelEmpty}
	model := CompiledModel{Kind: CompiledModelEmpty, Source: 0, Empty: true}
	tests := []struct {
		name    string
		wantErr string
		sources []ContentModel
		models  []CompiledModel
	}{
		{
			name:    "valid table",
			sources: []ContentModel{source},
			models:  []CompiledModel{model},
		},
		{
			name:    "count mismatch",
			sources: []ContentModel{source},
			wantErr: "compiled content model count does not match model count",
		},
		{
			name:    "per-model validation",
			sources: []ContentModel{source},
			models:  []CompiledModel{{Kind: CompiledModelEmpty, Source: 1, Empty: true}},
			wantErr: "compiled content model source does not match model slot",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCompiledModelsRuntime(nil, dfaRowIndexRuntimeStub{}, tt.sources, tt.models)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateCompiledModelsRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateCompiledModelsRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDFARowIndex(t *testing.T) {
	t.Parallel()

	names, err := NewNameTable(16, []string{EmptyNamespaceURI, "urn:any"}, []ExpandedName{
		{Local: "head"},
		{Local: "sub"},
		{Local: "other"},
	})
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	headName, ok := names.LookupQName("", "head")
	if !ok {
		t.Fatal("head QName missing")
	}
	subName, ok := names.LookupQName("", "sub")
	if !ok {
		t.Fatal("sub QName missing")
	}
	otherName, ok := names.LookupQName("", "other")
	if !ok {
		t.Fatal("other QName missing")
	}
	const head = ElementID(1)
	const sub = ElementID(2)
	rt := dfaRowIndexRuntimeStub{
		elementNames: map[ElementID]QName{
			head: headName,
			sub:  subName,
		},
		substitutionByName: map[ElementID]map[QName]ElementID{
			head: {subName: sub},
		},
	}
	validRow := CompiledModelRow{
		Edges: []CompiledModelEdge{
			{Particle: ElementParticle(head, Occurrence{Min: 1, Max: 1}), To: 1},
			{Particle: WildcardParticle(WildcardID(1), Occurrence{Min: 1, Max: 1}), To: 1},
		},
		Index: DFARowIndex{
			NameToEdge: map[QName]uint32{
				headName: 0,
				subName:  0,
			},
			WildcardEdges: []uint32{1},
			Enabled:       true,
		},
	}
	if err := ValidateDFARowIndex(&names, rt, validRow); err != nil {
		t.Fatalf("ValidateDFARowIndex() error = %v", err)
	}

	tests := []struct {
		name string
		row  CompiledModelRow
	}{
		{
			name: "nil name index",
			row: CompiledModelRow{
				Edges: validRow.Edges,
				Index: DFARowIndex{Enabled: true},
			},
		},
		{
			name: "name points at wildcard",
			row: CompiledModelRow{
				Edges: validRow.Edges,
				Index: DFARowIndex{
					NameToEdge:    map[QName]uint32{headName: 1, subName: 0},
					WildcardEdges: []uint32{1},
					Enabled:       true,
				},
			},
		},
		{
			name: "name does not match element or substitution",
			row: CompiledModelRow{
				Edges: validRow.Edges,
				Index: DFARowIndex{
					NameToEdge:    map[QName]uint32{headName: 0, subName: 0, otherName: 0},
					WildcardEdges: []uint32{1},
					Enabled:       true,
				},
			},
		},
		{
			name: "substitution missing from index",
			row: CompiledModelRow{
				Edges: validRow.Edges,
				Index: DFARowIndex{
					NameToEdge:    map[QName]uint32{headName: 0},
					WildcardEdges: []uint32{1},
					Enabled:       true,
				},
			},
		},
		{
			name: "wildcard missing from index",
			row: CompiledModelRow{
				Edges: validRow.Edges,
				Index: DFARowIndex{
					NameToEdge: map[QName]uint32{headName: 0, subName: 0},
					Enabled:    true,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateDFARowIndex(&names, rt, tc.row); err == nil {
				t.Fatal("ValidateDFARowIndex() accepted invalid row")
			}
		})
	}
}
