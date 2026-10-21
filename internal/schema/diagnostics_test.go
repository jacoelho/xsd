package schema

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

func TestSchemaDTDDiagnosticPreservesCause(t *testing.T) {
	_, err := Compile(Options{}, []source.Source{source.Bytes("dtd.xsd", []byte("\n<!DOCTYPE xs:schema>"+`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`))})
	if !errors.Is(err, xmlstream.ErrUnsupportedDTD) {
		t.Fatalf("DTD cause lost: %v", err)
	}
	boundary, ok := errors.AsType[*xmlstream.Error](err)
	if !ok || boundary.Kind != xmlstream.ErrorUnsupported || boundary.Line != 2 || boundary.Column != 1 {
		t.Fatalf("stream cause = %+v, want unsupported at 2:1", boundary)
	}
}

func TestContentExpectationsDoNotConsumeState(t *testing.T) {
	for _, model := range []string{`<xs:sequence><xs:element name="a"/><xs:element name="b"/></xs:sequence>`, `<xs:all><xs:element name="a"/><xs:element name="b"/></xs:all>`} {
		rt, err := Compile(Options{}, []source.Source{source.Bytes("test.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType>`+model+`</xs:complexType></xs:element></xs:schema>`))})
		if err != nil {
			t.Fatal(err)
		}
		root, _ := rt.LookupQName("", "root")
		element, decl, ok := rt.RootElement(RuntimeName{Known: true, Name: root, Local: "root"})
		if !ok {
			t.Fatal("root missing")
		}
		frame, ok := rt.ElementFrame(decl.Type, element)
		if !ok {
			t.Fatal("frame missing")
		}
		state := frame.Content.ContentState()
		bits := make([]uint64, frame.Content.AllBitLen())
		scratch := NewContentScratch(bits, 0, len(bits))
		expected, valid := rt.ExpectedContent(state, &scratch, ExpectedNextChild)
		if !valid || expected.Len() == 0 {
			t.Fatal("expected children missing")
		}
		for _, word := range bits {
			if word != 0 {
				t.Fatal("diagnostic marked an all-group child seen")
			}
		}
		first, _ := rt.LookupQName("", "a")
		transition, status := rt.NextContent(state, ContentInput{Name: RuntimeName{Known: true, Name: first, Local: "a"}}, &scratch)
		if status != ContentTransitionMatched || !transition.Commit(&state, &scratch) {
			t.Fatal("diagnostic changed child admission")
		}
		after, valid := rt.ExpectedContent(state, &scratch, ExpectedNextChild)
		if !valid || after.Len() != 1 {
			t.Fatalf("after first child: %+v, valid=%v", after, valid)
		}
		item, _ := after.At(0)
		if item.Name.Local != "b" {
			t.Fatalf("next child=%+v", item)
		}
	}
}

func TestContentExpectationsBoundAlternativesAndTraversal(t *testing.T) {
	var model strings.Builder
	model.WriteString(`<xs:choice>`)
	for i := range 300 {
		model.WriteString(`<xs:element name="child`)
		model.WriteString(strconv.Itoa(i))
		model.WriteString(`"/>`)
	}
	model.WriteString(`</xs:choice>`)
	rt, err := Compile(Options{}, []source.Source{source.Bytes("test.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType>`+model.String()+`</xs:complexType></xs:element></xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := rt.LookupQName("", "root")
	element, decl, _ := rt.RootElement(RuntimeName{Known: true, Name: q, Local: "root"})
	frame, _ := rt.ElementFrame(decl.Type, element)
	state := frame.Content.ContentState()
	scratch := NewContentScratch(nil, 0, 0)
	expected, valid := rt.ExpectedContent(state, &scratch, ExpectedNextChild)
	if !valid || expected.Len() != 8 || !expected.More {
		t.Fatalf("expected bounded alternatives: len=%d more=%v valid=%v", expected.Len(), expected.More, valid)
	}
	if _, valid := rt.ExpectedContent(ContentState{}, &scratch, ExpectedNextChild); valid {
		t.Fatal("invalid state described as an empty expectation set")
	}
}

func TestContentExpectationsBoundSkippedAllTerms(t *testing.T) {
	var model strings.Builder
	model.WriteString(`<xs:all>`)
	for i := range 300 {
		model.WriteString(`<xs:element name="child`)
		model.WriteString(strconv.Itoa(i))
		model.WriteString(`"/>`)
	}
	model.WriteString(`</xs:all>`)
	rt, err := Compile(Options{}, []source.Source{source.Bytes("test.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType>`+model.String()+`</xs:complexType></xs:element></xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := rt.LookupQName("", "root")
	element, decl, _ := rt.RootElement(RuntimeName{Known: true, Name: q, Local: "root"})
	frame, _ := rt.ElementFrame(decl.Type, element)
	state := frame.Content.ContentState()
	bits := make([]uint64, frame.Content.AllBitLen())
	scratch := NewContentScratch(bits, 0, len(bits))
	for i := range 299 {
		if !scratch.SetAllSeen(i) {
			t.Fatal("test all-group bit unavailable")
		}
	}
	expected, valid := rt.ExpectedContent(state, &scratch, ExpectedNextChild)
	if !valid || expected.Len() != 0 || !expected.More {
		t.Fatalf("traversal was not bounded before the 300th term: len=%d more=%v valid=%v", expected.Len(), expected.More, valid)
	}
	if seen, valid := scratch.AllSeen(299); !valid || seen {
		t.Fatal("projection changed the unseen child")
	}
}
