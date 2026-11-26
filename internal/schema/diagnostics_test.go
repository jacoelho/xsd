package schema

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSchemaDTDDiagnosticPreservesCause(t *testing.T) {
	_, err := compileSources(Options{}, []source.Source{source.Bytes("dtd.xsd", []byte("\n<!DOCTYPE xs:schema>"+`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`))})
	if feature, ok := xmlstream.UnsupportedFeatureOf(err); !ok || feature != xmlstream.UnsupportedDTD {
		t.Fatalf("DTD cause lost: %v", err)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Line() != 2 || diagnostic.Column() != 1 {
		t.Fatalf("diagnostic = %v, want located at 2:1", err)
	}
}

func TestContentExpectationsDoNotConsumeState(t *testing.T) {
	for _, model := range []string{`<xs:sequence><xs:element name="a"/><xs:element name="b"/></xs:sequence>`, `<xs:all><xs:element name="a"/><xs:element name="b"/></xs:all>`} {
		rt, err := compileSources(Options{}, []source.Source{source.Bytes("test.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType>`+model+`</xs:complexType></xs:element></xs:schema>`))})
		if err != nil {
			t.Fatal(err)
		}
		root, _ := rt.LookupQName("", "root")
		element, decl, ok := rt.GlobalElement(RuntimeName{known: true, name: root, local: "root"})
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
		transition, status := rt.NextContent(state, ContentInput{Name: RuntimeName{known: true, name: first, local: "a"}}, &scratch)
		if status != contentTransitionMatched || !transition.Commit(&state, &scratch) {
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
	rt, err := compileSources(Options{}, []source.Source{source.Bytes("test.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType>`+model.String()+`</xs:complexType></xs:element></xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := rt.LookupQName("", "root")
	element, decl, _ := rt.GlobalElement(RuntimeName{known: true, name: q, local: "root"})
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
	rt, err := compileSources(Options{}, []source.Source{source.Bytes("test.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType>`+model.String()+`</xs:complexType></xs:element></xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := rt.LookupQName("", "root")
	element, decl, _ := rt.GlobalElement(RuntimeName{known: true, name: q, local: "root"})
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

func TestContentExpectationsCopyOnlyTheDisplayedWildcardNamespaces(t *testing.T) {
	const count = 5000
	var namespaces strings.Builder
	for i := count - 1; i >= 0; i-- {
		namespaces.WriteString(" urn:")
		namespaces.WriteString(strconv.Itoa(100000 + i))
	}
	rt, err := compileSources(Options{}, []source.Source{source.Bytes("test.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:any namespace="`+namespaces.String()+`"/></xs:sequence></xs:complexType></xs:element></xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := rt.LookupQName("", "root")
	element, decl, _ := rt.GlobalElement(RuntimeName{known: true, name: q, local: "root"})
	frame, _ := rt.ElementFrame(decl.Type, element)
	state := frame.Content.ContentState()
	scratch := NewContentScratch(nil, 0, 0)

	expected, valid := rt.ExpectedContent(state, &scratch, ExpectedNextChild)
	wildcard, ok := expected.At(0)
	if !valid || expected.Len() != 1 || !ok || !wildcard.IsWildcard || !wildcard.MoreNamespaces {
		t.Fatalf("ExpectedContent() = %+v, valid %v; want one truncated wildcard", expected, valid)
	}
	want := []string{"urn:100000", "urn:100001", "urn:100002", "urn:100003", "urn:100004", "urn:100005", "urn:100006", "urn:100007"}
	if !slices.Equal(wildcard.Namespaces, want) || cap(wildcard.Namespaces) != len(want) {
		t.Fatalf("wildcard namespaces = %q (cap %d), want %q with no spare capacity", wildcard.Namespaces, cap(wildcard.Namespaces), want)
	}
	if allocs := testing.AllocsPerRun(10, func() { rt.ExpectedContent(state, &scratch, ExpectedNextChild) }); allocs != 1 {
		t.Fatalf("ExpectedContent() over %d namespaces allocated %v times, want 1 copy of the displayed namespaces", count, allocs)
	}
}

func TestContentExpectationsListAnAlternativeOnceHoweverManyParticlesRenderIt(t *testing.T) {
	t.Parallel()

	var projection contentExpectationProjection
	for range 8 {
		projection.add(ContentExpectation{IsWildcard: true, WildcardMode: WildcardAny})
	}
	projection.add(ContentExpectation{IsWildcard: true, WildcardMode: WildcardTargetNamespace, Namespaces: []string{"urn:t"}})
	projection.add(ContentExpectation{IsWildcard: true, WildcardMode: WildcardList, Namespaces: []string{"urn:t"}})
	projection.add(ContentExpectation{Name: ExpandedName{Local: "last"}})
	projection.add(ContentExpectation{Name: ExpandedName{Local: "last"}})

	got := projection.result()
	if got.Len() != 3 || got.More {
		t.Fatalf("result = %+v, want the any wildcard, the urn:t wildcard and last, each once, with nothing more", got)
	}
	if first, _ := got.At(0); !first.IsWildcard || first.WildcardMode != WildcardAny {
		t.Fatalf("first alternative = %+v, want the any wildcard", first)
	}
	if second, _ := got.At(1); !second.IsWildcard || !slices.Equal(second.Namespaces, []string{"urn:t"}) {
		t.Fatalf("second alternative = %+v, want the urn:t wildcard", second)
	}
	if third, _ := got.At(2); third.IsWildcard || third.Name.Local != "last" {
		t.Fatalf("third alternative = %+v, want element last", third)
	}
}

func TestSchemaStreamErrorMapsReaderMisuseToInternalInvariant(t *testing.T) {
	var reader xmlstream.Reader
	err := schemaStreamError(&reader, reader.Complete(), Limits{})
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Category() != xsderrors.CategoryInternal || diagnostic.Code() != xsderrors.CodeInternalInvariant {
		t.Fatalf("error = %v, want internal.invariant", err)
	}
}
