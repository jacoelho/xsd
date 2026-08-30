package validate

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlns"
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

func TestXMLDocumentPathRejectsInvalidMode(t *testing.T) {
	t.Parallel()

	for _, mode := range []xmlPathMode{xmlPathInvalid, xmlPathMode(99)} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("PathString() accepted mode %d", mode)
				}
			}()
			doc := emptyXMLDocument{
				elements: []xmlDocumentElement[struct{}]{{
					name:       xml.Name{Local: "root"},
					pathLength: len("/root"),
					pathMode:   mode,
				}},
			}
			doc.PathString()
		}()
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
	doc.CommitStart(start, struct{}{})

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
	doc.CommitStart(start, struct{}{})
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
	doc.CommitStart(start, struct{}{})
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
	doc.CommitStart(start, struct{}{})
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

func TestXMLDocumentPathAtDepthUsesCanonicalCache(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	values := stream.NewCache()
	commitDocumentStart(t, &doc, &values, "root")
	commitDocumentStart(t, &doc, &values, "child")
	commitDocumentStart(t, &doc, &values, "grandchild")

	if got := doc.PathStringAtDepth(1); got != "/root" {
		t.Fatalf("root path = %q", got)
	}
	if doc.pathTextDepth != 1 {
		t.Fatalf("cached depth = %d, want 1", doc.pathTextDepth)
	}
	if got := doc.PathStringAtDepth(2); got != "/root/child" {
		t.Fatalf("child path = %q", got)
	}
	if got := doc.PathString(); got != "/root/child/grandchild" {
		t.Fatalf("current path = %q", got)
	}
	wantCache := doc.pathText
	if got := doc.PathStringAtDepth(1); got != "/root" {
		t.Fatalf("cached root path = %q", got)
	}
	if doc.pathText != wantCache || doc.pathTextDepth != 3 {
		t.Fatalf("ancestor projection changed cache: text=%q depth=%d", doc.pathText, doc.pathTextDepth)
	}

	if err := doc.CommitEnd(); err != nil {
		t.Fatal(err)
	}
	if doc.pathText != "/root/child" || doc.pathTextDepth != 2 {
		t.Fatalf("pop cache: text=%q depth=%d", doc.pathText, doc.pathTextDepth)
	}
}

func TestXMLDocumentRetainedPathsSharePrefixesAndSurvivePop(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	values := stream.NewCache()
	root := strings.Repeat("r", 255)
	commitDocumentStart(t, &doc, &values, root)
	commitDocumentStart(t, &doc, &values, "first")
	first := doc.retainPathAtDepth(2)
	if doc.pathText != "" {
		t.Fatalf("retaining path materialized active cache %q", doc.pathText)
	}
	if len(doc.retainedPaths.nodes) != 1 || doc.elements[0].pathRef.node != 1 {
		t.Fatalf("first retained path state: nodes=%d root=%+v", len(doc.retainedPaths.nodes), doc.elements[0].pathRef)
	}
	if err := doc.CommitEnd(); err != nil {
		t.Fatal(err)
	}
	commitDocumentStart(t, &doc, &values, "second")
	second := doc.retainPathAtDepth(2)
	if len(doc.retainedPaths.nodes) != 2 || doc.retainedPaths.nodes[1].parent != doc.elements[0].pathRef {
		t.Fatalf("sibling retained path did not share root: %+v", doc.retainedPaths.nodes)
	}
	if got := first.String(); got != "/"+root+"/first" {
		t.Fatalf("first retained path = %q", got)
	}
	if got := second.String(); got != "/"+root+"/second" {
		t.Fatalf("second retained path = %q", got)
	}
}

func TestXMLDocumentRetainedPathReferencesEverySegmentBoundary(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "root"}}, struct{}{})
	doc.CommitExpandedStart(preparedXMLStart{name: xml.Name{Space: "urn:test", Local: "middle"}}, struct{}{})
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "leaf"}}, struct{}{})
	deepest := doc.retainPathAtDepth(3)
	if got, want := deepest.String(), "/root/{urn:test}middle/leaf"; got != want {
		t.Fatalf("retained path = %q, want %q", got, want)
	}
	if got := len(doc.retainedPaths.nodes); got != 1 {
		t.Fatalf("retained nodes = %d, want 1", got)
	}
	for i := 1; i < len(doc.elements); i++ {
		previous, current := doc.elements[i-1].pathRef, doc.elements[i].pathRef
		if current.node != 1 || current.end <= previous.end {
			t.Fatalf("element %d path reference = %+v after %+v", i, current, previous)
		}
	}
	for depth, want := range []string{"/root", "/root/{urn:test}middle", "/root/{urn:test}middle/leaf"} {
		if got := doc.retainPathAtDepth(depth + 1).String(); got != want {
			t.Fatalf("retained path at depth %d = %q, want %q", depth+1, got, want)
		}
	}
	if len(doc.retainedPaths.nodes) != 1 || len(doc.retainedPaths.namespaces) != 1 {
		t.Fatal("retaining annotated ancestors grew path storage")
	}
	if doc.pathText != "" {
		t.Fatalf("retaining path materialized active cache %q", doc.pathText)
	}
}

func TestXMLDocumentRetainedExpandedPath(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	space := strings.Repeat("u", 256)
	doc.CommitExpandedStart(preparedXMLStart{name: xml.Name{Space: space, Local: "root"}}, struct{}{})
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "child"}}, struct{}{})
	path := doc.retainPathAtDepth(2)
	if got := path.String(); got != "/{"+space+"}root/child" {
		t.Fatalf("retained expanded path = %q", got)
	}
	if len(doc.retainedPaths.nodes) != 1 || len(doc.retainedPaths.namespaces) != 1 {
		t.Fatalf("retained expanded path state: nodes=%d namespaces=%d", len(doc.retainedPaths.nodes), len(doc.retainedPaths.namespaces))
	}
}

func TestXMLDocumentRetainedExpandedSiblingsShareLargeNamespace(t *testing.T) {
	t.Parallel()

	const siblings = 64
	var doc emptyXMLDocument
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "root"}}, struct{}{})
	space := strings.Repeat("u", 256)
	var first retainedPath
	for sibling := range siblings {
		doc.CommitExpandedStart(preparedXMLStart{name: xml.Name{Space: space, Local: "child"}}, struct{}{})
		path := doc.retainPathAtDepth(2)
		if sibling == 0 {
			first = path
		}
		if err := doc.CommitEnd(); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := len(doc.retainedPaths.nodes), siblings; got != want {
		t.Fatalf("retained expanded sibling nodes = %d, want %d", got, want)
	}
	if got := len(doc.retainedPaths.namespaces); got != siblings {
		t.Fatalf("retained sibling namespace headers = %d, want %d", got, siblings)
	}
	for i, node := range doc.retainedPaths.nodes {
		if strings.Contains(node.text, space) || strings.IndexByte(node.text, 0) < 0 {
			t.Fatalf("expanded sibling node %d retained rendered namespace: %+v", i, node)
		}
	}
	if got, want := first.String(), "/root/{"+space+"}child"; got != want {
		t.Fatalf("first retained expanded sibling path = %q, want %q", got, want)
	}
}

func TestXMLDocumentRetainedExpandedCommonPrefixUsesBoundaryReference(t *testing.T) {
	t.Parallel()

	const (
		commonDepth = 62
		siblings    = 64
	)
	var doc emptyXMLDocument
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "root"}}, struct{}{})
	var prefix strings.Builder
	prefix.WriteString("/root")
	for level := range commonDepth {
		namespace := fmt.Sprintf("urn:%02d", level)
		doc.CommitExpandedStart(preparedXMLStart{name: xml.Name{Space: namespace, Local: "n"}}, struct{}{})
		fmt.Fprintf(&prefix, "/{%s}n", namespace)
	}

	var first, second retainedPath
	for sibling := range siblings {
		doc.CommitExpandedStart(preparedXMLStart{name: xml.Name{Space: "urn:leaf", Local: "n"}}, struct{}{})
		path := doc.retainPathAtDepth(doc.Depth())
		switch sibling {
		case 0:
			first = path
		case 1:
			second = path
		}
		if err := doc.CommitEnd(); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := len(doc.retainedPaths.nodes), siblings; got != want {
		t.Fatalf("retained nodes = %d, want %d", got, want)
	}
	if got, want := len(doc.retainedPaths.namespaces), commonDepth+siblings; got != want {
		t.Fatalf("retained namespace headers = %d, want %d", got, want)
	}
	common := doc.elements[commonDepth].pathRef
	if common.node != 1 || common.end <= 0 || common.end >= len(doc.retainedPaths.nodes[0].text) {
		t.Fatalf("common-prefix reference = %+v, first text bytes = %d", common, len(doc.retainedPaths.nodes[0].text))
	}
	if parent := doc.retainedPaths.nodes[1].parent; parent != common {
		t.Fatalf("second sibling parent = %+v, want %+v", parent, common)
	}
	want := prefix.String() + "/{urn:leaf}n"
	if got := first.String(); got != want {
		t.Fatalf("first retained path = %q, want %q", got, want)
	}
	if got := second.String(); got != want {
		t.Fatalf("second retained path = %q, want %q", got, want)
	}
}

func TestXMLDocumentRetainedPathBeyondInlineChain(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	for range 40 {
		doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "n"}}, struct{}{})
		_ = doc.retainPathAtDepth(doc.Depth())
	}
	if got, want := doc.retainPathAtDepth(40).String(), strings.Repeat("/n", 40); got != want {
		t.Fatalf("retained deep path = %q, want %q", got, want)
	}
	if len(doc.retainedPaths.nodes) != 40 {
		t.Fatalf("retained deep path nodes = %d, want 40", len(doc.retainedPaths.nodes))
	}
}

func TestXMLDocumentRetainedDisjointPathsAreBoundedByBytes(t *testing.T) {
	t.Parallel()

	const (
		branches = 64
		depth    = 128
	)
	var doc emptyXMLDocument
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "root"}}, struct{}{})
	var first retainedPath
	for branch := range branches {
		for range depth {
			doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "node"}}, struct{}{})
		}
		path := doc.retainPathAtDepth(depth + 1)
		if branch == 0 {
			first = path
		}
		for range depth {
			if err := doc.CommitEnd(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got, want := len(doc.retainedPaths.nodes), branches; got != want {
		t.Fatalf("retained disjoint path nodes = %d, want %d", got, want)
	}
	if got, want := first.String(), "/root"+strings.Repeat("/node", depth); got != want {
		t.Fatalf("first retained disjoint path = %q, want %q", got, want)
	}
}

func TestXMLDocumentRetainedDisjointExpandedPathsAreBoundedByBytes(t *testing.T) {
	t.Parallel()

	const (
		branches = 64
		depth    = 128
	)
	var doc emptyXMLDocument
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "root"}}, struct{}{})
	var first retainedPath
	for branch := range branches {
		for range depth {
			doc.CommitExpandedStart(preparedXMLStart{name: xml.Name{Space: "u", Local: "n"}}, struct{}{})
		}
		path := doc.retainPathAtDepth(depth + 1)
		if branch == 0 {
			first = path
		}
		for range depth {
			if err := doc.CommitEnd(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got, want := len(doc.retainedPaths.nodes), branches; got != want {
		t.Fatalf("retained disjoint expanded path nodes = %d, want %d", got, want)
	}
	if got := len(doc.retainedPaths.namespaces); got != branches {
		t.Fatalf("retained disjoint namespace headers = %d, want %d", got, branches)
	}
	if got, want := first.String(), "/root"+strings.Repeat("/{u}n", depth); got != want {
		t.Fatalf("first retained disjoint expanded path = %q, want %q", got, want)
	}
}

func TestXMLDocumentStartRollbackRemovesRetainedPathNodes(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	root := strings.Repeat("r", 255)
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: root}}, struct{}{})
	checkpoint := doc.startCheckpoint()
	doc.CommitExpandedStart(preparedXMLStart{name: xml.Name{Space: "urn:test", Local: "child"}}, struct{}{})
	_ = doc.retainPathAtDepth(2)
	if len(doc.retainedPaths.nodes) != 1 || len(doc.retainedPaths.namespaces) != 1 || doc.elements[0].pathRef.node == 0 {
		t.Fatal("test setup did not retain child path")
	}
	if err := doc.rollbackStart(checkpoint, xmlns.Frame{}); err != nil {
		t.Fatal(err)
	}
	if len(doc.retainedPaths.nodes) != 0 || len(doc.retainedPaths.namespaces) != 0 || doc.elements[0].pathRef != (documentPathRef{}) {
		t.Fatalf(
			"rollback retained path state: nodes=%d names=%d root=%+v",
			len(doc.retainedPaths.nodes),
			len(doc.retainedPaths.namespaces),
			doc.elements[0].pathRef,
		)
	}
	if got := doc.retainPathAtDepth(1).String(); got != "/"+root {
		t.Fatalf("root path after rollback = %q", got)
	}
}

func TestXMLDocumentResetDropsOversizedPathNamespaceState(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	for i := range maxRetainedSliceCap + 1 {
		doc.CommitExpandedStart(preparedXMLStart{
			name: xml.Name{Space: fmt.Sprintf("urn:%d", i), Local: "n"},
		}, struct{}{})
	}
	_ = doc.retainPathAtDepth(doc.Depth())
	if len(doc.retainedPaths.namespaces) <= maxRetainedSliceCap {
		t.Fatal("test setup did not grow retained path namespace state past the reset limit")
	}
	if len(doc.retainedPaths.namespaceScratch) != 0 {
		t.Fatal("path namespace scratch retained keys after encoding")
	}

	doc.Reset(maxRetainedSliceCap)
	if cap(doc.retainedPaths.namespaces) != 0 || doc.retainedPaths.namespaceScratch != nil {
		t.Fatal("reset retained oversized path namespace state")
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
	doc.CommitStart(start, struct{}{})
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
