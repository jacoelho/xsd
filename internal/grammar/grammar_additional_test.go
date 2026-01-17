package grammar

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

type substitutionMatcher struct {
	substitutes map[types.QName]types.QName
}

func (m substitutionMatcher) IsSubstitutable(actual, declared types.QName) bool {
	if m.substitutes == nil {
		return false
	}
	return m.substitutes[actual] == declared
}

func TestAllGroupValidator(t *testing.T) {
	elemA := &CompiledElement{QName: types.QName{Local: "a"}}
	elemB := &CompiledElement{QName: types.QName{Local: "b"}}
	elemHead := &CompiledElement{QName: types.QName{Local: "head"}}

	validator := NewAllGroupValidator([]AllGroupElementInfo{
		&AllGroupElement{Element: elemA, Optional: false},
		&AllGroupElement{Element: elemB, Optional: true},
		&AllGroupElement{Element: elemHead, Optional: false, AllowSubstitution: true},
	}, false, types.OccursFromInt(1))

	doc, children := makeXMLChildren(t, `<root><a/><b/><sub/></root>`)
	err := validator.Validate(doc, children, substitutionMatcher{
		substitutes: map[types.QName]types.QName{
			{Local: "sub"}: {Local: "head"},
		},
	})
	if err != nil {
		t.Fatalf("expected valid all group, got %v", err)
	}

	doc, children = makeXMLChildren(t, `<root><a/><a/></root>`)
	if err := validator.Validate(doc, children, nil); err == nil {
		t.Fatalf("expected duplicate element error")
	}

	doc, children = makeXMLChildren(t, `<root><a/></root>`)
	if err := validator.Validate(doc, children, nil); err == nil {
		t.Fatalf("expected missing required element error")
	}

	doc, children = makeXMLChildren(t, `<root><c/></root>`)
	if err := validator.Validate(doc, children, nil); err == nil {
		t.Fatalf("expected unexpected element error")
	}

	empty := NewAllGroupValidator(nil, false, types.OccursFromInt(0))
	doc, children = makeXMLChildren(t, `<root></root>`)
	if err := empty.Validate(doc, children, nil); err != nil {
		t.Fatalf("expected empty all group to allow empty content: %v", err)
	}
}

func TestCompiledContentHelpers(t *testing.T) {
	elem := &CompiledElement{
		QName:          types.QName{Local: "a"},
		EffectiveQName: types.QName{Local: "a", Namespace: "urn:test"},
	}
	groupElem := &AllGroupElement{Element: elem, Optional: true, AllowSubstitution: true}

	if got := groupElem.ElementQName(); got.Namespace != "urn:test" {
		t.Fatalf("expected effective QName to be used, got %s", got)
	}
	if groupElem.ElementDecl() != elem {
		t.Fatalf("expected ElementDecl to return element")
	}
	if !groupElem.IsOptional() || !groupElem.AllowsSubstitution() {
		t.Fatalf("expected optional and substitution flags to be true")
	}

	wildcard := &types.AnyElement{Namespace: types.NSCAny}
	model := &CompiledContentModel{
		Particles: []*CompiledParticle{
			{Kind: ParticleWildcard, Wildcard: wildcard},
		},
	}
	wildcards := model.Wildcards()
	if len(wildcards) != 1 || wildcards[0] != wildcard {
		t.Fatalf("expected wildcard collection to include the wildcard")
	}
}

func TestBitsetStringAndKey(t *testing.T) {
	bs := newBitset(64)
	bs.set(0)
	bs.set(63)
	if got := bs.String(); got != "8000000000000001" {
		t.Fatalf("unexpected bitset String: %s", got)
	}
	if got := bs.key(); len(got) != 8 {
		t.Fatalf("expected key length 8, got %d", len(got))
	}
	if got := newBitset(0).key(); got != "" {
		t.Fatalf("expected empty key for zero-sized bitset")
	}
}

func TestBuildAutomatonChoiceAndWildcard(t *testing.T) {
	elemA := &CompiledElement{QName: types.QName{Local: "a"}}

	choice := &CompiledParticle{
		Kind:      ParticleGroup,
		GroupKind: types.Choice,
		MinOccurs: types.OccursFromInt(1),
		MaxOccurs: types.OccursFromInt(2),
		Children: []*CompiledParticle{
			{
				Kind:      ParticleElement,
				MinOccurs: types.OccursFromInt(1),
				MaxOccurs: types.OccursFromInt(1),
				Element:   elemA,
			},
			{
				Kind:      ParticleWildcard,
				MinOccurs: types.OccursFromInt(0),
				MaxOccurs: types.OccursFromInt(1),
				Wildcard: &types.AnyElement{
					Namespace:     types.NSCList,
					NamespaceList: []types.NamespaceURI{"urn:a", "urn:b"},
				},
			},
		},
	}

	automaton, err := BuildAutomaton([]*CompiledParticle{choice}, "urn:test", types.FormQualified)
	if err != nil {
		t.Fatalf("build automaton: %v", err)
	}

	doc, children := makeXMLChildren(t, `<root xmlns:a="urn:a"><a:a/></root>`)
	if err := automaton.Validate(doc, children, nil); err != nil {
		t.Fatalf("expected wildcard match to validate, got %v", err)
	}

	doc, children = makeXMLChildren(t, `<root><a/></root>`)
	if err := automaton.Validate(doc, children, nil); err != nil {
		t.Fatalf("expected element match to validate, got %v", err)
	}

	stream := automaton.NewStreamValidator(nil, nil)
	if _, err := stream.Feed(types.QName{Local: "a"}); err != nil {
		t.Fatalf("stream feed: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("stream close: %v", err)
	}
	stream.Release()
}

func TestValidationErrorCodes(t *testing.T) {
	err := &ValidationError{Index: 1, Message: "boom", SubCode: ErrorCodeMissing}
	if err.Error() != "child 1: boom" {
		t.Fatalf("unexpected error string: %s", err.Error())
	}
	if err.FullCode() == "" {
		t.Fatalf("expected FullCode to be non-empty")
	}
}

func makeXMLChildren(t *testing.T, xmlStr string) (*xsdxml.Document, []xsdxml.NodeID) {
	t.Helper()

	doc, err := xsdxml.Parse(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatalf("parse xml: %v", err)
	}
	root := doc.Root()
	children := doc.Children(root)
	return doc, children
}
