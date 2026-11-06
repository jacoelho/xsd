package schema

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCompleteContentModelKinds(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		model compiledModel
		want  ContentCompletionStatus
	}{
		{name: "empty", model: compiledModel{Kind: compiledModelEmpty}, want: ContentCompletionComplete},
		{name: "any", model: compiledModel{Kind: compiledModelAny}, want: ContentCompletionComplete},
		{name: "all", model: compiledModel{Kind: compiledModelAll, Empty: true}, want: ContentCompletionComplete},
		{name: "DFA", model: compiledModel{Kind: compiledModelDFA, Rows: []compiledModelRow{{Accept: true}}}, want: ContentCompletionComplete},
		{name: "invalid", model: compiledModel{Kind: compiledModelKind(99)}, want: ContentCompletionInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rt := publishedContentSchema(contentSchemaFixture{models: map[contentModelID]compiledModel{0: tt.model}})
			got := rt.CompleteContent(ContentState{model: 0, present: true}, &ContentScratch{})
			if got != tt.want {
				t.Fatalf("CompleteContent(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestDFARowIndexEnabledState(t *testing.T) {
	t.Parallel()

	if (dfaRowIndex{}).enabled() {
		t.Fatal("zero-value row index is enabled")
	}
	if !(dfaRowIndex{nameToEdge: map[QName]uint32{}}).enabled() {
		t.Fatal("enabled row index reported disabled")
	}
}

func TestCompiledCountingException(t *testing.T) {
	t.Parallel()

	p := elementParticle(1, occurrence{Min: 1, Max: 1})
	row := compiledModelRow{
		Counted:       true,
		CountParticle: p,
		Min:           2,
		Max:           2,
	}
	loop := compiledModelEdge{Particle: p, To: 0}
	exit := compiledModelEdge{Particle: p, To: 1}
	if !compiledCountingException(0, row, loop, exit) {
		t.Fatal("CompiledCountingException() rejected loop/exit pair")
	}
	row.Unbounded = true
	if compiledCountingException(0, row, loop, exit) {
		t.Fatal("CompiledCountingException() accepted unbounded row")
	}
}

func TestIndexCompiledModelRows(t *testing.T) {
	t.Parallel()

	const head = ElementID(1)
	headName := QName{Local: 1}
	subName := QName{Local: 10}
	elementNames := map[ElementID]QName{head: headName}
	one := occurrence{Min: 1, Max: 1}
	edges := []compiledModelEdge{
		{Particle: elementParticle(head, one), To: 1},
		{Particle: wildcardParticle(1, one), To: 1},
	}
	for id := ElementID(2); len(edges) < compiledDFARowIndexMinEdges; id++ {
		elementNames[id] = QName{Local: localNameID(id)}
		edges = append(edges, compiledModelEdge{Particle: elementParticle(id, one), To: 1})
	}
	rt := dfaRowIndexFixture{
		elementNames: elementNames,
		substitutionByName: map[ElementID]map[QName]ElementID{
			head: {subName: ElementID(2)},
		},
	}
	model := compiledModel{
		Kind: compiledModelDFA,
		Rows: []compiledModelRow{
			{Edges: edges},
			{Edges: edges[:compiledDFARowIndexMinEdges-1]},
		},
	}
	if err := newDFARowIndexAnalysis(rt.build(), unlimitedContentModelWork).indexModel(&model); err != nil {
		t.Fatalf("IndexCompiledModelRows() error = %v", err)
	}
	if !model.Rows[0].index.enabled() {
		t.Fatal("wide row was not indexed")
	}
	if got, ok := model.Rows[0].index.nameToEdge[headName]; !ok || got != 0 {
		t.Fatalf("head index = %d, %v; want 0, true", got, ok)
	}
	if got, ok := model.Rows[0].index.nameToEdge[subName]; !ok || got != 0 {
		t.Fatalf("substitution index = %d, %v; want 0, true", got, ok)
	}
	if got := model.Rows[0].index.wildcardEdges; len(got) != 1 || got[0] != 1 {
		t.Fatalf("wildcard index = %v, want [1]", got)
	}
	if model.Rows[1].index.enabled() {
		t.Fatal("narrow row was indexed")
	}
}

func TestIndexCompiledModelsRowsBoundsSubstitutionExpansion(t *testing.T) {
	t.Parallel()

	const head = ElementID(1)
	one := occurrence{Min: 1, Max: 1}
	elementNames := map[ElementID]QName{head: {Local: 1}}
	edges := []compiledModelEdge{{Particle: elementParticle(head, one), To: 1}}
	for id := ElementID(2); len(edges) < compiledDFARowIndexMinEdges; id++ {
		elementNames[id] = QName{Local: localNameID(id)}
		edges = append(edges, compiledModelEdge{Particle: elementParticle(id, one), To: 1})
	}
	rt := dfaRowIndexFixture{
		elementNames: elementNames,
		substitutionByName: map[ElementID]map[QName]ElementID{
			head: {
				{Local: 20}: 2,
				{Local: 21}: 3,
			},
		},
	}
	models := []compiledModel{{
		Kind: compiledModelDFA,
		Rows: []compiledModelRow{{Edges: edges}},
	}}
	budgetExceeded := errors.New("content work exceeded")
	remaining := 10 // model + row + eight edges; expansion must charge more.
	err := indexCompiledModelsRows(rt.build(), models, func(steps int) error {
		remaining -= steps
		if remaining < 0 {
			return budgetExceeded
		}
		return nil
	})
	if !errors.Is(err, budgetExceeded) {
		t.Fatalf("IndexCompiledModelsRows() error = %v, want budget error", err)
	}
	if models[0].Rows[0].index.enabled() {
		t.Fatal("budget failure published a partial row index")
	}
}

func TestIndexCompiledModelRowsKeepsLinearScanForDuplicateNames(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	duplicateName := QName{Local: 1}
	elementNames := map[ElementID]QName{
		1: duplicateName,
		2: duplicateName,
	}
	var edges []compiledModelEdge
	for id := ElementID(1); len(edges) < compiledDFARowIndexMinEdges; id++ {
		if _, ok := elementNames[id]; !ok {
			elementNames[id] = QName{Local: localNameID(id)}
		}
		edges = append(edges, compiledModelEdge{Particle: elementParticle(id, one), To: 1})
	}
	model := compiledModel{
		Kind: compiledModelDFA,
		Rows: []compiledModelRow{{Edges: edges}},
	}
	if err := newDFARowIndexAnalysis((dfaRowIndexFixture{elementNames: elementNames}).build(), unlimitedContentModelWork).indexModel(&model); err != nil {
		t.Fatalf("IndexCompiledModelRows() error = %v", err)
	}
	if model.Rows[0].index.enabled() {
		t.Fatal("ambiguous row was indexed")
	}
}

func TestIndexCompiledModelRowsRejectsInvalidElement(t *testing.T) {
	t.Parallel()

	one := occurrence{Min: 1, Max: 1}
	edges := make([]compiledModelEdge, compiledDFARowIndexMinEdges)
	for i := range edges {
		edges[i] = compiledModelEdge{Particle: elementParticle(ElementID(i+1), one), To: 1}
	}
	model := compiledModel{
		Kind: compiledModelDFA,
		Rows: []compiledModelRow{{Edges: edges}},
	}
	err := newDFARowIndexAnalysis((dfaRowIndexFixture{}).build(), unlimitedContentModelWork).indexModel(&model)
	if err == nil || !strings.Contains(err.Error(), "compiled content model index references invalid element") {
		t.Fatalf("IndexCompiledModelRows() error = %v", err)
	}
}

type dfaRowIndexFixture struct {
	elementNames       map[ElementID]QName
	substitutionIDs    map[ElementID][]ElementID
	substitutionByName map[ElementID]map[QName]ElementID
	contentModels      map[contentModelID]contentModelNode
	wildcards          map[WildcardID]wildcardRuntime
}

func TestIndexCompiledModelsRowsAdmissionFailureAllowsRetry(t *testing.T) {
	t.Parallel()
	budgetFailure := errors.New("index work unavailable")
	for _, tc := range []struct {
		name string
		work contentModelWork
	}{
		{name: "missing work"},
		{name: "failed admission", work: func(int) error { return budgetFailure }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			one := occurrence{Min: 1, Max: 1}
			elementNames := map[ElementID]QName{
				1: {Local: 1}, 2: {Local: 2}, 3: {Local: 3}, 4: {Local: 4},
				5: {Local: 5}, 6: {Local: 6}, 7: {Local: 7}, 8: {Local: 8},
			}
			edges := make([]compiledModelEdge, 8)
			for i := range edges {
				edges[i] = compiledModelEdge{Particle: elementParticle(ElementID(i+1), one), To: 1}
			}
			build := (dfaRowIndexFixture{elementNames: elementNames}).build()
			models := []compiledModel{{Kind: compiledModelDFA, Rows: []compiledModelRow{{Edges: edges}, {Accept: true}}}}
			want := []compiledModel{{Kind: compiledModelDFA, Rows: []compiledModelRow{
				{Edges: append([]compiledModelEdge(nil), edges...)}, {Accept: true},
			}}}
			err := indexCompiledModelsRows(build, models, tc.work)
			if tc.work == nil {
				if err == nil {
					t.Fatalf("missing work error = %v, want required-budget error", err)
				}
			} else if !errors.Is(err, budgetFailure) {
				t.Fatalf("admission error = %v, want budget failure", err)
			}
			if !reflect.DeepEqual(models, want) {
				t.Fatalf("admission failure changed compiled models: got %#v, want %#v", models, want)
			}
			if err := indexCompiledModelsRows(build, models, unlimitedContentModelWork); err != nil {
				t.Fatalf("retry error = %v", err)
			}
			for _, entry := range []struct {
				name QName
				edge uint32
			}{
				{name: QName{Local: 1}, edge: 0}, {name: QName{Local: 8}, edge: 7},
			} {
				if got, ok := models[0].Rows[0].index.nameToEdge[entry.name]; !ok || got != entry.edge {
					t.Fatalf("retry index[%v] = %d, %v; want %d, true", entry.name, got, ok, entry.edge)
				}
			}
		})
	}
}

func fixtureMapSlice[K ~uint32, V any](in map[K]V) []V {
	count := 0
	for id := range in {
		count = max(count, int(id)+1)
	}
	out := make([]V, count)
	for id, v := range in {
		out[id] = v
	}
	return out
}

func (f dfaRowIndexFixture) build() *schemaBuild {
	b := &schemaBuild{Models: fixtureMapSlice(f.contentModels), Wildcards: fixtureMapSlice(f.wildcards), Elements: fixtureElements(fixtureMapSlice(f.elementNames), nil)}
	b.Substitutions = fixtureSubstitutions(b.Elements, f.substitutionIDs, f.substitutionByName)
	return b
}
