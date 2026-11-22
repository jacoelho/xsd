package schema

import (
	"errors"
	"testing"
)

func largeAllModel(first ElementID) compiledModel {
	terms := make([]compiledAllTerm, compiledDFARowIndexMinEdges)
	for i := range terms {
		terms[i].Particle = contentParticle{Kind: particleElement, Element: first + ElementID(i)}
	}
	return compiledModel{Kind: compiledModelAll, All: terms}
}

func allTermNames(models ...compiledModel) map[ElementID]QName {
	names := make(map[ElementID]QName)
	for _, model := range models {
		for _, term := range model.All {
			names[term.Particle.Element] = QName{Local: localNameID(term.Particle.Element)}
		}
	}
	return names
}

func TestAllContentIndexBoundsSubstitutionWork(t *testing.T) {
	t.Parallel()
	const head = ElementID(1)
	// substituted's head expands to extra names; plain has none.
	substituted := largeAllModel(1)
	plain := largeAllModel(31)
	source := dfaRowIndexFixture{
		elementNames: allTermNames(substituted, plain),
		substitutionByName: map[ElementID]map[QName]ElementID{
			head: {{Local: 20}: 2, {Local: 21}: 3},
		},
	}
	exhausted := errors.New("content work exhausted")
	index := func(model compiledModel) (compiledModelRead, error) {
		read := testProjectedModel(model)
		remaining := 1 + compiledDFARowIndexMinEdges
		err := newDFARowIndexAnalysis(source.build(), func(steps int) error {
			remaining -= steps
			if remaining < 0 {
				return exhausted
			}
			return nil
		}, testDependencyWork()).indexModel(&read)
		return read, err
	}
	if read, err := index(plain); err != nil || read.allNames == nil {
		t.Fatalf("plain index = %v, installed %v; want it to fit the budget", err, read.allNames != nil)
	}
	read, err := index(substituted)
	if !errors.Is(err, exhausted) {
		t.Fatalf("indexModel() = %v, want work exhaustion", err)
	}
	if read.allNames != nil {
		t.Fatal("failed index construction installed lookup data on the published model")
	}
}

func TestAllContentIndexMapsEachTermOfAnIndexableModel(t *testing.T) {
	t.Parallel()
	// unindexable repeats one element at two positions, so its names cannot
	// be indexed although it is large enough.
	unindexable := largeAllModel(41)
	unindexable.All[1].Particle.Element = unindexable.All[0].Particle.Element
	small := compiledModel{Kind: compiledModelAll, All: largeAllModel(61).All[:compiledDFARowIndexMinEdges-1]}
	for _, tt := range []struct {
		name    string
		model   compiledModel
		indexed bool
	}{
		{name: "large", model: largeAllModel(1), indexed: true},
		{name: "below the index threshold", model: small},
		{name: "repeated element", model: unindexable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			names := allTermNames(tt.model)
			read := testProjectedModel(tt.model)
			if err := newDFARowIndexAnalysis((dfaRowIndexFixture{elementNames: names}).build(), unlimitedContentModelWork, testDependencyWork()).indexModel(&read); err != nil {
				t.Fatalf("indexModel() error = %v", err)
			}
			if !tt.indexed {
				if read.allNames != nil {
					t.Fatal("model carries an all-group index it cannot own")
				}
				return
			}
			if len(read.allNames) != len(tt.model.All) {
				t.Fatalf("index has %d names, want %d", len(read.allNames), len(tt.model.All))
			}
			for pos, term := range tt.model.All {
				if read.allNames[names[term.Particle.Element]] != uint32(pos) {
					t.Fatalf("index does not map term %d to its position", pos)
				}
			}
		})
	}
}
