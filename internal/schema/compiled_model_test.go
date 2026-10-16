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
		model CompiledModel
		want  ContentCompletionStatus
	}{
		{name: "empty", model: CompiledModel{Kind: CompiledModelEmpty}, want: ContentCompletionComplete},
		{name: "any", model: CompiledModel{Kind: CompiledModelAny}, want: ContentCompletionComplete},
		{name: "all", model: CompiledModel{Kind: CompiledModelAll, Empty: true}, want: ContentCompletionComplete},
		{name: "DFA", model: CompiledModel{Kind: CompiledModelDFA, Rows: []CompiledModelRow{{Accept: true}}}, want: ContentCompletionComplete},
		{name: "invalid", model: CompiledModel{Kind: CompiledModelKind(99)}, want: ContentCompletionInvalid},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rt := publishedContentSchema(contentSchemaFixture{models: map[ContentModelID]CompiledModel{0: tt.model}})
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
	for id := ElementID(2); len(edges) < compiledDFARowIndexMinEdges; id++ {
		elementNames[id] = QName{Local: LocalNameID(id)}
		edges = append(edges, CompiledModelEdge{Particle: ElementParticle(id, one), To: 1})
	}
	rt := dfaRowIndexFixture{
		elementNames: elementNames,
		substitutionByName: map[ElementID]map[QName]ElementID{
			head: {subName: ElementID(2)},
		},
	}
	model := CompiledModel{
		Kind: CompiledModelDFA,
		Rows: []CompiledModelRow{
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
	one := Occurrence{Min: 1, Max: 1}
	elementNames := map[ElementID]QName{head: {Local: 1}}
	edges := []CompiledModelEdge{{Particle: ElementParticle(head, one), To: 1}}
	for id := ElementID(2); len(edges) < compiledDFARowIndexMinEdges; id++ {
		elementNames[id] = QName{Local: LocalNameID(id)}
		edges = append(edges, CompiledModelEdge{Particle: ElementParticle(id, one), To: 1})
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
	models := []CompiledModel{{
		Kind: CompiledModelDFA,
		Rows: []CompiledModelRow{{Edges: edges}},
	}}
	budgetExceeded := errors.New("content work exceeded")
	remaining := 10 // model + row + eight edges; expansion must charge more.
	err := IndexCompiledModelsRows(rt.build(), models, func(steps int) error {
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

	one := Occurrence{Min: 1, Max: 1}
	duplicateName := QName{Local: 1}
	elementNames := map[ElementID]QName{
		1: duplicateName,
		2: duplicateName,
	}
	var edges []CompiledModelEdge
	for id := ElementID(1); len(edges) < compiledDFARowIndexMinEdges; id++ {
		if _, ok := elementNames[id]; !ok {
			elementNames[id] = QName{Local: LocalNameID(id)}
		}
		edges = append(edges, CompiledModelEdge{Particle: ElementParticle(id, one), To: 1})
	}
	model := CompiledModel{
		Kind: CompiledModelDFA,
		Rows: []CompiledModelRow{{Edges: edges}},
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

	one := Occurrence{Min: 1, Max: 1}
	edges := make([]CompiledModelEdge, compiledDFARowIndexMinEdges)
	for i := range edges {
		edges[i] = CompiledModelEdge{Particle: ElementParticle(ElementID(i+1), one), To: 1}
	}
	model := CompiledModel{
		Kind: CompiledModelDFA,
		Rows: []CompiledModelRow{{Edges: edges}},
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
	contentModels      map[ContentModelID]ContentModel
	wildcards          map[WildcardID]Wildcard
}

func TestIndexCompiledModelsRowsAdmissionFailureAllowsRetry(t *testing.T) {
	t.Parallel()
	budgetFailure := errors.New("index work unavailable")
	for _, tc := range []struct {
		name string
		work ContentModelWork
	}{
		{name: "missing work"},
		{name: "failed admission", work: func(int) error { return budgetFailure }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			one := Occurrence{Min: 1, Max: 1}
			elementNames := map[ElementID]QName{
				1: {Local: 1}, 2: {Local: 2}, 3: {Local: 3}, 4: {Local: 4},
				5: {Local: 5}, 6: {Local: 6}, 7: {Local: 7}, 8: {Local: 8},
			}
			edges := make([]CompiledModelEdge, 8)
			for i := range edges {
				edges[i] = CompiledModelEdge{Particle: ElementParticle(ElementID(i+1), one), To: 1}
			}
			build := (dfaRowIndexFixture{elementNames: elementNames}).build()
			models := []CompiledModel{{Kind: CompiledModelDFA, Rows: []CompiledModelRow{{Edges: edges}, {Accept: true}}}}
			want := []CompiledModel{{Kind: CompiledModelDFA, Rows: []CompiledModelRow{
				{Edges: append([]CompiledModelEdge(nil), edges...)}, {Accept: true},
			}}}
			err := IndexCompiledModelsRows(build, models, tc.work)
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
			if err := IndexCompiledModelsRows(build, models, unlimitedContentModelWork); err != nil {
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
