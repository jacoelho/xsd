package schema

import (
	"errors"
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
			{Edges: edges[:compiledDFARowIndexMinEdges-1]},
		},
	}
	if err := newDFARowIndexAnalysis(rt, unlimitedContentModelWork).indexModel(&model); err != nil {
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
	rt := dfaRowIndexRuntimeStub{
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
	err := IndexCompiledModelsRows(rt, models, func(steps int) error {
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
	if err := newDFARowIndexAnalysis(dfaRowIndexRuntimeStub{elementNames: elementNames}, unlimitedContentModelWork).indexModel(&model); err != nil {
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
	err := newDFARowIndexAnalysis(dfaRowIndexRuntimeStub{}, unlimitedContentModelWork).indexModel(&model)
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

func (s dfaRowIndexRuntimeStub) ForEachSubstitutionEntry(id ElementID, fn func(QName, ElementID) bool) {
	for name, member := range s.substitutionByName[id] {
		if !fn(name, member) {
			return
		}
	}
}
