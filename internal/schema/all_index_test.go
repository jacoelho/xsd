package schema

import (
	"errors"
	"slices"
	"testing"
)

func TestAllContentIndexBoundsSubstitutionWork(t *testing.T) {
	t.Parallel()
	const head = ElementID(1)
	allTerms := func(first ElementID) []CompiledAllTerm {
		terms := make([]CompiledAllTerm, compiledDFARowIndexMinEdges)
		for i := range terms {
			terms[i].Particle = Particle{Kind: ParticleElement, Element: first + ElementID(i)}
		}
		return terms
	}
	// substituted's head expands to extra names; plain has none.
	substituted := CompiledModel{Kind: CompiledModelAll, All: allTerms(1)}
	plain := CompiledModel{Kind: CompiledModelAll, All: allTerms(31)}
	names := make(map[ElementID]QName)
	for _, model := range []CompiledModel{substituted, plain} {
		for _, term := range model.All {
			names[term.Particle.Element] = QName{Local: LocalNameID(term.Particle.Element)}
		}
	}
	source := dfaRowIndexFixture{
		elementNames: names,
		substitutionByName: map[ElementID]map[QName]ElementID{
			head: {{Local: 20}: 2, {Local: 21}: 3},
		},
	}
	exhausted := errors.New("content work exhausted")
	index := func(models []CompiledModel, entered []ContentModelID) ([]compiledModelRead, error) {
		reads := make([]compiledModelRead, len(entered))
		remaining := compiledDFARowIndexMinEdges
		err := indexAllContentModels(models, entered, reads, source.build(), func(steps int) error {
			remaining -= steps
			if remaining < 0 {
				return exhausted
			}
			return nil
		})
		return reads, err
	}
	if reads, err := index([]CompiledModel{plain}, []ContentModelID{0}); err != nil || reads[0].allNames == nil {
		t.Fatalf("plain index = %v, installed %v; want it to fit the budget", err, reads[0].allNames != nil)
	}
	for _, tt := range []struct {
		name    string
		models  []CompiledModel
		entered []ContentModelID
	}{
		{name: "entered", models: []CompiledModel{substituted}, entered: []ContentModelID{0}},
		// Validation never enters the substituted model, but its index work
		// is still charged, so its exhaustion fails publication although the
		// entered model alone fits.
		{name: "unentered before entered", models: []CompiledModel{substituted, plain}, entered: []ContentModelID{1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reads, err := index(tt.models, tt.entered)
			if !errors.Is(err, exhausted) {
				t.Fatalf("indexAllContentModels() = %v, want work exhaustion", err)
			}
			for i := range reads {
				if reads[i].allNames != nil {
					t.Fatalf("failed index construction installed lookup data on published model %d", i)
				}
			}
		})
	}
}

func TestAllContentIndexStaysWithItsPublishedModel(t *testing.T) {
	t.Parallel()
	large := func(first ElementID) CompiledModel {
		terms := make([]CompiledAllTerm, compiledDFARowIndexMinEdges)
		for i := range terms {
			terms[i].Particle = Particle{Kind: ParticleElement, Element: first + ElementID(i)}
		}
		return CompiledModel{Kind: CompiledModelAll, All: terms}
	}
	// unindexable repeats one element at two positions, so its names cannot
	// be indexed although it is large enough.
	unindexable := large(41)
	unindexable.All[1].Particle.Element = unindexable.All[0].Particle.Element
	small := CompiledModel{Kind: CompiledModelAll, All: large(61).All[:compiledDFARowIndexMinEdges-1]}
	dfa := CompiledModel{Kind: CompiledModelDFA, Rows: []CompiledModelRow{{Accept: true}}}
	for _, tt := range []struct {
		name    string
		models  []CompiledModel
		entered []ContentModelID
		// indexed lists the entered models whose slots carry their own index.
		indexed []ContentModelID
	}{
		{
			// Unentered large all-groups lead and trail the entered ones, and
			// entered models that carry no index sit between indexed ones.
			name:    "mixed",
			models:  []CompiledModel{large(1), dfa, large(11), small, unindexable, large(21), large(31)},
			entered: []ContentModelID{1, 2, 3, 4, 5},
			indexed: []ContentModelID{2, 5},
		},
		{name: "entered last", models: []CompiledModel{large(1), large(11)}, entered: []ContentModelID{1}, indexed: []ContentModelID{1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			names := make(map[ElementID]QName)
			for _, model := range tt.models {
				for _, term := range model.All {
					names[term.Particle.Element] = QName{Local: LocalNameID(term.Particle.Element)}
				}
			}
			reads := make([]compiledModelRead, len(tt.entered))
			if err := indexAllContentModels(tt.models, tt.entered, reads, (dfaRowIndexFixture{elementNames: names}).build(), unlimitedContentModelWork); err != nil {
				t.Fatalf("indexAllContentModels() error = %v", err)
			}
			for slot, id := range tt.entered {
				model := tt.models[id]
				got := reads[slot].allNames
				if !slices.Contains(tt.indexed, id) {
					if got != nil {
						t.Fatalf("published slot %d (model %d) carries an all-group index it does not own", slot, id)
					}
					continue
				}
				if len(got) != len(model.All) {
					t.Fatalf("published slot %d (model %d) index has %d names, want its own %d", slot, id, len(got), len(model.All))
				}
				for pos, term := range model.All {
					if got[names[term.Particle.Element]] != uint32(pos) {
						t.Fatalf("published slot %d (model %d) does not map its term %d to its own position", slot, id, pos)
					}
				}
			}
		})
	}
}
