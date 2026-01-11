package grammar

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

type testAllGroupElement struct {
	qname             types.QName
	optional          bool
	allowSubstitution bool
}

func (e testAllGroupElement) ElementQName() types.QName {
	return e.qname
}

func (e testAllGroupElement) ElementDecl() *CompiledElement {
	return nil
}

func (e testAllGroupElement) IsOptional() bool {
	return e.optional
}

func (e testAllGroupElement) AllowsSubstitution() bool {
	return e.allowSubstitution
}

func TestAutomatonStreamValidator(t *testing.T) {
	automaton := buildTestAutomaton(t)

	tests := []struct {
		name     string
		children []string
		valid    bool
	}{
		{"<a><a>", []string{"a", "a"}, true},
		{"<a><a><b>", []string{"a", "a", "b"}, true},
		{"<a><b><a>", []string{"a", "b", "a"}, true},
		{"<a><b><a><b>", []string{"a", "b", "a", "b"}, true},
		{"<a><a><a>", []string{"a", "a", "a"}, true},
		{"<a><a><a><a>", []string{"a", "a", "a", "a"}, true},
		{"<a>", []string{"a"}, false},
		{"<a><a><a><a><a>", []string{"a", "a", "a", "a", "a"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := automaton.NewStreamValidator(nil, nil)
			var err error
			for _, name := range tt.children {
				_, err = validator.Feed(types.QName{Local: name})
				if err != nil {
					break
				}
			}
			if err == nil {
				err = validator.Close()
			}

			if tt.valid {
				if err != nil {
					t.Fatalf("expected valid, got error: %v", err)
				}
			} else if err == nil {
				t.Fatalf("expected invalid, got valid")
			}
		})
	}
}

func TestAutomatonStreamValidatorWildcardMatch(t *testing.T) {
	wildcard := &types.AnyElement{
		Namespace:       types.NSCAny,
		ProcessContents: types.Skip,
	}
	particle := &ParticleAdapter{
		Kind:      ParticleWildcard,
		MinOccurs: 1,
		MaxOccurs: 1,
		Wildcard:  wildcard,
		Original:  wildcard,
	}

	builder := NewBuilder([]*ParticleAdapter{particle}, "", types.FormUnqualified)
	automaton, err := builder.Build()
	if err != nil {
		t.Fatalf("build automaton: %v", err)
	}

	validator := automaton.NewStreamValidator(nil, []*types.AnyElement{wildcard})
	match, err := validator.Feed(types.QName{Local: "a"})
	if err != nil {
		t.Fatalf("feed error: %v", err)
	}
	if !match.IsWildcard {
		t.Fatalf("expected wildcard match")
	}
	if match.ProcessContents != types.Skip {
		t.Fatalf("expected processContents=%v, got %v", types.Skip, match.ProcessContents)
	}
	if err := validator.Close(); err != nil {
		t.Fatalf("close error: %v", err)
	}
}

func TestAllGroupStreamValidator(t *testing.T) {
	elements := []AllGroupElementInfo{
		testAllGroupElement{qname: types.QName{Local: "a"}},
		testAllGroupElement{qname: types.QName{Local: "b"}, optional: true},
		testAllGroupElement{qname: types.QName{Local: "c"}},
	}
	validator := NewAllGroupValidator(elements, false, 1)

	tests := []struct {
		name     string
		children []string
		valid    bool
	}{
		{"required only", []string{"a", "c"}, true},
		{"any order", []string{"c", "b", "a"}, true},
		{"duplicate", []string{"a", "a"}, false},
		{"missing required", []string{"b"}, false},
		{"not allowed", []string{"d"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream := validator.NewStreamValidator(nil)
			var err error
			for _, name := range tt.children {
				_, err = stream.Feed(types.QName{Local: name})
				if err != nil {
					break
				}
			}
			if err == nil {
				err = stream.Close()
			}

			if tt.valid {
				if err != nil {
					t.Fatalf("expected valid, got error: %v", err)
				}
			} else if err == nil {
				t.Fatalf("expected invalid, got valid")
			}
		})
	}
}
