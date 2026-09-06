package schema

import (
	"errors"
	"testing"
)

func TestAllContentIndexBoundsSubstitutionWork(t *testing.T) {
	t.Parallel()
	const head = ElementID(1)
	terms := make([]compiledAllTermRead, compiledDFARowIndexMinEdges)
	names := make(map[ElementID]QName, len(terms))
	for i := range terms {
		id := ElementID(i + 1)
		names[id] = QName{Local: LocalNameID(i + 1)}
		terms[i].Particle = compiledParticleRead{Kind: ParticleElement, Element: id}
	}
	source := dfaRowIndexRuntimeStub{
		elementNames: names,
		substitutionByName: map[ElementID]map[QName]ElementID{
			head: {{Local: 20}: 2, {Local: 21}: 3},
		},
	}
	models := []compiledModelRead{{Kind: CompiledModelAll, All: terms}}
	exhausted := errors.New("content work exhausted")
	remaining := len(terms)
	err := indexAllContentModels(models, source, func(steps int) error {
		remaining -= steps
		if remaining < 0 {
			return exhausted
		}
		return nil
	})
	if !errors.Is(err, exhausted) {
		t.Fatalf("indexAllContentModels() = %v, want work exhaustion", err)
	}
	if models[0].allNames != nil {
		t.Fatal("failed index construction installed partial lookup data")
	}
}
