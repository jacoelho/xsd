package grammar

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

func TestAutomatonValidate(t *testing.T) {
	automaton := buildTestAutomaton(t)

	tests := []struct {
		name     string
		children []string
		valid    bool
	}{
		// valid cases
		{"<a><a>", []string{"a", "a"}, true},                 // seq1: a, Seq2: a
		{"<a><a><b>", []string{"a", "a", "b"}, true},         // seq1: a, Seq2: a,b
		{"<a><b><a>", []string{"a", "b", "a"}, true},         // seq1: a,b, Seq2: a
		{"<a><b><a><b>", []string{"a", "b", "a", "b"}, true}, // seq1: a,b, Seq2: a,b
		{"<a><a><a>", []string{"a", "a", "a"}, true},         // seq1: a,a, Seq2: a (each seq can have 1-2 a's)
		{"<a><a><a><a>", []string{"a", "a", "a", "a"}, true}, // seq1: a,a, Seq2: a,a

		// invalid cases
		{"<a>", []string{"a"}, false},                                 // only 1 sequence, need 2
		{"<a><a><a><a><a>", []string{"a", "a", "a", "a", "a"}, false}, // too many
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, children := makeElements(t, tt.children)
			err := automaton.Validate(doc, children, nil)

			if tt.valid {
				if err != nil {
					t.Errorf("Expected valid, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected invalid, got valid")
				}
			}
		})
	}
}

func TestAutomatonValidateEndStateOutOfBounds(t *testing.T) {
	automaton := buildTestAutomaton(t)
	state := &validationState{currentState: len(automaton.accepting)}

	if err := automaton.validateEndState(state, 0); err == nil {
		t.Fatalf("expected error for out-of-bounds state")
	}
}

func buildTestAutomaton(t *testing.T) *Automaton {
	t.Helper()

	elemA := &types.ElementDecl{Name: types.QName{Local: "a"}}
	elemB := &types.ElementDecl{Name: types.QName{Local: "b"}}
	compiledA := &CompiledElement{QName: elemA.Name, Original: elemA}
	compiledB := &CompiledElement{QName: elemB.Name, Original: elemB}

	group := func() *ParticleAdapter {
		return &ParticleAdapter{
			Kind:      ParticleGroup,
			MinOccurs: types.OccursFromInt(1),
			MaxOccurs: types.OccursFromInt(1),
			GroupKind: types.Sequence,
			Children: []*ParticleAdapter{
				{
					Kind:      ParticleElement,
					MinOccurs: types.OccursFromInt(1),
					MaxOccurs: types.OccursFromInt(2),
					Element:   compiledA,
					Original:  elemA,
				},
				{
					Kind:      ParticleElement,
					MinOccurs: types.OccursFromInt(0),
					MaxOccurs: types.OccursFromInt(1),
					Element:   compiledB,
					Original:  elemB,
				},
			},
		}
	}

	builder := NewBuilder([]*ParticleAdapter{group(), group()}, "", types.FormUnqualified)
	automaton, err := builder.Build()
	if err != nil {
		t.Fatalf("build automaton: %v", err)
	}
	return automaton
}

func makeElements(t *testing.T, names []string) (*xsdxml.Document, []xsdxml.NodeID) {
	t.Helper()

	var sb strings.Builder
	sb.WriteString("<root>")
	for _, name := range names {
		sb.WriteString("<")
		sb.WriteString(name)
		sb.WriteString("/>")
	}
	sb.WriteString("</root>")

	doc, err := xsdxml.Parse(strings.NewReader(sb.String()))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root := doc.Root()
	children := doc.Children(root)
	return doc, children
}
