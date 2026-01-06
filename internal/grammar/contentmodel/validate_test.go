package contentmodel

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// TestRepeatingSequenceWithOptionalElement tests the addB176 case:
// Sequence with minOccurs=2, maxOccurs=2 containing:
//   - element a (minOccurs=1, maxOccurs=2)
//   - element b (minOccurs=0 - optional)
//
// XML: <Root><a/><a/><b/></Root>
// Expected: valid (Sequence1: <a/>, Sequence2: <a/><b/>)
func TestRepeatingSequenceWithOptionalElement(t *testing.T) {
	// Create the content model:
	// (a{1,2}, b{0,1}){2,2}
	elemA := &types.ElementDecl{Name: types.QName{Local: "a"}}
	elemB := &types.ElementDecl{Name: types.QName{Local: "b"}}

	particles := []*ParticleAdapter{
		{
			Kind:      1, // ParticleGroup
			MinOccurs: 2,
			MaxOccurs: 2,
			GroupKind: types.Sequence,
			Children: []*ParticleAdapter{
				{
					Kind:      0, // ParticleElement
					MinOccurs: 1,
					MaxOccurs: 2,
					Original:  elemA,
				},
				{
					Kind:      0, // ParticleElement
					MinOccurs: 0,
					MaxOccurs: 1,
					Original:  elemB,
				},
			},
		},
	}

	builder := NewBuilder(particles, nil, "", false)
	automaton, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build automaton: %v", err)
	}

	// Print automaton info for debugging
	t.Logf("Symbols: %d", len(automaton.symbols))
	for i, sym := range automaton.symbols {
		t.Logf("  Symbol %d: Kind=%d, QName=%s", i, sym.Kind, sym.QName.Local)
	}
	t.Logf("States: %d", len(automaton.trans))
	t.Logf("Counters:")
	for i, c := range automaton.counting {
		if c != nil {
			t.Logf("  State %d: Min=%d, Max=%d, SymIdx=%d, IsGroup=%v, GroupID=%d, CompletionSymbols=%v",
				i, c.Min, c.Max, c.SymbolIdx, c.IsGroupCounter, c.GroupID, c.GroupCompletionSymbols)
		}
	}
	t.Logf("Accepting states:")
	for i, acc := range automaton.accepting {
		if acc {
			t.Logf("  State %d is accepting", i)
		}
	}
	t.Logf("Transitions:")
	for stateIdx, row := range automaton.trans {
		for symIdx, next := range row {
			if next >= 0 {
				t.Logf("  State %d --[sym %d]--> State %d", stateIdx, symIdx, next)
			}
		}
	}

	tests := []struct {
		name     string
		children []string
		valid    bool
	}{
		// Valid cases
		{"<a><a>", []string{"a", "a"}, true},                 // Seq1: a, Seq2: a
		{"<a><a><b>", []string{"a", "a", "b"}, true},         // Seq1: a, Seq2: a,b
		{"<a><b><a>", []string{"a", "b", "a"}, true},         // Seq1: a,b, Seq2: a
		{"<a><b><a><b>", []string{"a", "b", "a", "b"}, true}, // Seq1: a,b, Seq2: a,b
		{"<a><a><a>", []string{"a", "a", "a"}, true},         // Seq1: a,a, Seq2: a (each seq can have 1-2 a's)
		{"<a><a><a><a>", []string{"a", "a", "a", "a"}, true}, // Seq1: a,a, Seq2: a,a

		// Invalid cases
		{"<a>", []string{"a"}, false},                                 // Only 1 sequence, need 2
		{"<a><a><a><a><a>", []string{"a", "a", "a", "a", "a"}, false}, // Too many
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			children := makeElements(tt.children)
			err := automaton.Validate(children, nil)

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

// mockElement implements xml.Element for testing
type mockElement struct {
	localName    string
	namespaceURI string
}

func (m *mockElement) NodeType() xml.NodeType                 { return xml.ElementNode }
func (m *mockElement) NodeName() string                       { return m.localName }
func (m *mockElement) NodeValue() string                      { return "" }
func (m *mockElement) LocalName() string                      { return m.localName }
func (m *mockElement) NamespaceURI() string                   { return m.namespaceURI }
func (m *mockElement) Prefix() string                         { return "" }
func (m *mockElement) GetAttribute(name string) string        { return "" }
func (m *mockElement) GetAttributeNS(ns, local string) string { return "" }
func (m *mockElement) HasAttribute(name string) bool          { return false }
func (m *mockElement) HasAttributeNS(ns, local string) bool   { return false }
func (m *mockElement) Attributes() []xml.Attr                 { return nil }
func (m *mockElement) Children() []xml.Element                { return nil }
func (m *mockElement) Parent() xml.Element                    { return nil }
func (m *mockElement) TextContent() string                    { return "" }
func (m *mockElement) DirectTextContent() string              { return "" }

func makeElements(names []string) []xml.Element {
	elements := make([]xml.Element, len(names))
	for i, name := range names {
		elements[i] = &mockElement{localName: name}
	}
	return elements
}
