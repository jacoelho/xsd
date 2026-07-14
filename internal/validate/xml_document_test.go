package validate

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

type emptyXMLDocument = xmlDocument[struct{}]

func TestXMLDocumentStatePrepareStartRollsBackNamespaces(t *testing.T) {
	var doc emptyXMLDocument
	values := stream.NewCache()
	_, err := prepareXMLStartForTest(&doc, testXMLStart(
		xml.Name{Space: "missing", Local: "root"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:test"),
	), &values, 0, 2, 3)
	requireCode(t, err, xsderrors.CodeValidationXML)
	if _, ok := doc.LookupNamespace("p"); ok {
		t.Fatal("failed start retained namespace binding")
	}
	if doc.Depth() != 0 {
		t.Fatalf("depth = %d, want 0", doc.Depth())
	}

	_, err = prepareXMLStartForTest(&doc, testXMLStart(xml.Name{Space: "p", Local: "root"}), &values, 0, 4, 5)
	if !strings.Contains(err.Error(), "unbound namespace prefix p") {
		t.Fatalf("PrepareStart() error = %v", err)
	}
}

func TestXMLDocumentStateRejectsDuplicateExpandedAttributes(t *testing.T) {
	var doc emptyXMLDocument
	values := stream.NewCache()
	_, err := prepareXMLStartForTest(&doc, testXMLStart(
		xml.Name{Local: "root"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:test"),
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "q"}, "urn:test"),
		testXMLAttr(xml.Name{Space: "p", Local: "id"}, ""),
		testXMLAttr(xml.Name{Space: "q", Local: "id"}, ""),
	), &values, 0, 2, 3)
	requireCode(t, err, xsderrors.CodeValidationXML)
	if !strings.Contains(err.Error(), "duplicate attribute {urn:test}id") {
		t.Fatalf("PrepareStart() error = %v", err)
	}
	if _, ok := doc.LookupNamespace("p"); ok {
		t.Fatal("duplicate attribute retained namespace bindings")
	}
}

func TestXMLDocumentStateEnforcesDepthLimit(t *testing.T) {
	values := stream.NewCache()
	var doc emptyXMLDocument
	start, err := prepareXMLStartForTest(&doc, testXMLStart(xml.Name{Local: "root"}), &values, 0, 2, 3)
	if err != nil {
		t.Fatalf("PrepareStart() error = %v", err)
	}
	doc.CommitStart(start, false, struct{}{})

	_, err = prepareXMLStartForTest(&doc, testXMLStart(xml.Name{Local: "child"}), &values, 1, 4, 5)
	requireCode(t, err, xsderrors.CodeValidationLimit)
	if !strings.Contains(err.Error(), "instance depth limit exceeded") {
		t.Fatalf("PrepareStart() error = %v", err)
	}
}

func TestXMLDocumentStateStartErrorPrecedenceAndMultipleRoots(t *testing.T) {
	var doc emptyXMLDocument
	values := stream.NewCache()
	start, err := prepareXMLStartForTest(&doc, testXMLStart(xml.Name{Local: "a"}), &values, 0, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	doc.CommitStart(start, false, struct{}{})
	if endErr := doc.ValidateEnd(stream.EndElement{Name: xml.Name{Local: "a"}}, 1, 4); endErr != nil {
		t.Fatal(endErr)
	}
	if commitErr := doc.CommitEnd(); commitErr != nil {
		t.Fatal(commitErr)
	}

	_, err = prepareXMLStartForTest(&doc, testXMLStart(xml.Name{Space: "p", Local: "b"}), &values, 0, 1, 5)
	if !strings.Contains(err.Error(), "unbound namespace prefix p") {
		t.Fatalf("PrepareStart() error = %v, want namespace error before multiple roots", err)
	}

	_, err = prepareXMLStartForTest(&doc, testXMLStart(
		xml.Name{Space: "p", Local: "b"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:test"),
	), &values, 0, 1, 5)
	if !strings.Contains(err.Error(), "multiple root elements") {
		t.Fatalf("PrepareStart() error = %v", err)
	}
}

func TestXMLDocumentStateRequiresLexicallyMatchingEndTag(t *testing.T) {
	var doc emptyXMLDocument
	values := stream.NewCache()
	start, err := prepareXMLStartForTest(&doc, testXMLStart(
		xml.Name{Space: "p", Local: "root"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:test"),
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "q"}, "urn:test"),
	), &values, 0, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	doc.CommitStart(start, false, struct{}{})
	err = doc.ValidateEnd(stream.EndElement{Name: xml.Name{Space: "q", Local: "root"}}, 4, 5)
	if !strings.Contains(err.Error(), "end element </q:root> does not match start element <p:root>") {
		t.Fatalf("ValidateEnd() error = %v", err)
	}
	if doc.Depth() != 1 {
		t.Fatalf("mismatched end changed depth to %d", doc.Depth())
	}
}

func TestXMLDocumentStateCompleteRejectsMissingAndUnclosedRoot(t *testing.T) {
	var doc emptyXMLDocument
	requireCode(t, doc.Complete(), xsderrors.CodeValidationRoot)

	values := stream.NewCache()
	start, err := prepareXMLStartForTest(&doc, testXMLStart(xml.Name{Local: "root"}), &values, 0, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	doc.CommitStart(start, false, struct{}{})
	err = doc.Complete()
	requireCode(t, err, xsderrors.CodeValidationXML)
	if !strings.Contains(err.Error(), "unclosed element") {
		t.Fatalf("Complete() error = %v", err)
	}
}

func TestXMLDocumentStatePathsStayLazyAndRecoverAcrossTransitions(t *testing.T) {
	var doc emptyXMLDocument
	values := stream.NewCache()
	commitDocumentStart(t, &doc, &values, "root")
	commitDocumentStart(t, &doc, &values, "child")
	if doc.pathText != "" {
		t.Fatalf("successful starts materialized path %q", doc.pathText)
	}
	if err := doc.ValidateEnd(stream.EndElement{Name: xml.Name{Local: "child"}}, 2, 1); err != nil {
		t.Fatal(err)
	}
	if err := doc.CommitEnd(); err != nil {
		t.Fatal(err)
	}
	if doc.pathText != "" {
		t.Fatalf("successful end materialized path %q", doc.pathText)
	}
	commitDocumentStart(t, &doc, &values, "child")

	_, err := prepareXMLStartForTest(&doc, testXMLStart(
		xml.Name{Space: "missing", Local: "bad"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "temporary"}, "urn:temporary"),
	), &values, 0, 4, 5)
	if err == nil {
		t.Fatal("PrepareStart() succeeded")
	}
	if got := doc.PathString(); got != "/root/child" {
		t.Fatalf("path after failed prepare = %q", got)
	}
	if _, ok := doc.LookupNamespace("temporary"); ok {
		t.Fatal("failed prepare retained temporary namespace")
	}

	if err := doc.CommitEnd(); err != nil {
		t.Fatal(err)
	}
	if got := doc.PathString(); got != "/root" {
		t.Fatalf("path after pop = %q", got)
	}
	commitDocumentStart(t, &doc, &values, "sibling")
	if got := doc.PathString(); got != "/root/sibling" {
		t.Fatalf("sibling path = %q", got)
	}

	capacity := cap(doc.elements)
	doc.Reset(maxRetainedSliceCap)
	if doc.Depth() != 0 || doc.pathText != "" || doc.pathTextDepth != 0 {
		t.Fatalf("Reset() retained document state: %+v", doc)
	}
	if cap(doc.elements) != capacity {
		t.Fatalf("Reset() capacity = %d, want %d", cap(doc.elements), capacity)
	}
}

func TestXMLSyntaxDiagnosticParity(t *testing.T) {
	rt, err := compile.Compile(compile.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" type="xs:anyType"/>
</xs:schema>`))})
	if err != nil {
		t.Fatal(err)
	}

	tests := []string{
		`<a/><p:b/>`,
		`<a/><p:b xmlns:p="urn:test"/>`,
		`<a/><p:b xmlns:q="urn:test"/>`,
		`<a/><b p:id="1"/>`,
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			preflightErr := CheckXMLWellFormed(strings.NewReader(input), Options{})
			session, err := newSessionForTest(rt, Options{})
			if err != nil {
				t.Fatal(err)
			}
			validationErr := session.Validate(strings.NewReader(input))
			if preflightErr == nil || validationErr == nil {
				t.Fatalf("errors = preflight %v, validation %v", preflightErr, validationErr)
			}
			if preflightErr.Error() != validationErr.Error() {
				t.Fatalf("diagnostics differ:\npreflight: %v\nvalidation: %v", preflightErr, validationErr)
			}
		})
	}
}

func commitDocumentStart(t *testing.T, doc *emptyXMLDocument, values *stream.Cache, local string) {
	t.Helper()
	start, err := prepareXMLStartForTest(doc, testXMLStart(xml.Name{Local: local}), values, 0, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	doc.CommitStart(start, false, struct{}{})
}

func prepareXMLStartForTest(
	doc *emptyXMLDocument,
	start stream.StartElement,
	values *stream.Cache,
	maxDepth, line, col int,
) (preparedXMLStart, error) {
	return doc.PrepareStart(start, values, maxDepth, line, col)
}

func testXMLStart(name xml.Name, attrs ...stream.Attr) stream.StartElement {
	return stream.OwnedStartElement(name, attrs...)
}

func testXMLAttr(name xml.Name, value string) stream.Attr {
	return stream.OwnedAttr(name, value)
}
