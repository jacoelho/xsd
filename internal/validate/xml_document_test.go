package validate

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

type emptyXMLDocument struct {
	xmlDocument[struct{}]

	reader         xmlstream.Reader
	input          testXMLInput
	lexicalEnds    []xmlstream.LexicalName
	pendingLexical xmlstream.LexicalName
	ready          bool
	endReady       bool
}

// commitEndState advances only the semantic test document. Production end
// commits perform this transition together with the Reader commit.
func (d *xmlDocument[P]) commitEndState() error {
	if d.Depth() == 0 {
		return xsderrors.InternalInvariant("cannot commit XML end element with no open element", nil)
	}

	i := len(d.elements) - 1
	d.elements[i] = xmlDocumentElement[P]{}
	d.elements = d.elements[:i]
	return nil
}

type testXMLInput struct {
	data   []byte
	offset int
}

func (r *testXMLInput) Read(dst []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(dst, r.data[r.offset:])
	r.offset += n
	return n, nil
}

func (d *emptyXMLDocument) ensureReader(start testXMLStartTag) error {
	d.input.offset = 0
	d.input.data = appendTestXMLStartBytes(d.input.data[:0], start)
	if d.ready {
		return nil
	}
	if len(d.input.data) < xmlstream.XMLDeclarationPrefixLen {
		// Reader.Reset peeks six bytes to classify the XML prolog. Keep the
		// synthetic input long enough without introducing a visible token.
		d.input.data = append(d.input.data, []byte("<!--x-->")...)
	}
	if err := d.reader.Reset(&d.input, xmlstream.Config{}); err != nil {
		return err
	}
	d.ready = true
	return nil
}

func (d *emptyXMLDocument) PrepareStart(start testXMLStartTag, _ *struct{}, maxDepth, line, col int) (preparedXMLStart, error) {
	if maxDepth > 0 && d.Depth()+1 > maxDepth {
		return preparedXMLStart{}, validation(d.context(line, col), xsderrors.CodeValidationLimit, "instance depth limit exceeded")
	}
	if err := d.ensureReader(start); err != nil {
		return preparedXMLStart{}, err
	}
	_, err := d.reader.Next()
	if err != nil {
		return preparedXMLStart{}, err
	}
	prepared, err := d.xmlDocument.PrepareStart(&d.reader, Limits{InstanceDepth: maxDepth}, line, col)
	if err == nil {
		d.pendingLexical = xmlstream.Lexical(start.name)
	}
	return prepared, err
}

// CommitStart records lexical names only for the synthetic reader used by
// these tests; production document frames retain expanded names and handles.
func (d *emptyXMLDocument) CommitStart(start preparedXMLStart, payload struct{}) {
	d.xmlDocument.CommitStart(start, payload)
	d.rememberLexicalStart(start)
}

func (d *emptyXMLDocument) CommitExpandedStart(start preparedXMLStart, payload struct{}) {
	d.xmlDocument.CommitExpandedStart(start, payload)
	d.rememberLexicalStart(start)
}

func (d *emptyXMLDocument) rememberLexicalStart(start preparedXMLStart) {
	lexical := d.pendingLexical
	if lexical.Local == "" {
		lexical = xmlstream.Lexical(start.name)
	}
	d.lexicalEnds = append(d.lexicalEnds, lexical)
	d.pendingLexical = xmlstream.LexicalName{}
}

func (d *emptyXMLDocument) rollbackStart(checkpoint xmlDocumentCheckpoint) {
	d.xmlDocument.rollbackStart(checkpoint)
	if checkpoint.depth < len(d.lexicalEnds) {
		clear(d.lexicalEnds[checkpoint.depth:])
		d.lexicalEnds = d.lexicalEnds[:checkpoint.depth]
	}
	d.pendingLexical = xmlstream.LexicalName{}
}

func (d *emptyXMLDocument) ValidateEnd(end xmlstream.EndElement, line, col int) error {
	if !d.ready {
		return d.xmlDocument.ValidateEnd(&d.reader, line, col)
	}
	d.input.offset = 0
	d.input.data = appendTestXMLEndBytes(d.input.data[:0], end)
	_, err := d.reader.Next()
	if err != nil {
		return err
	}
	if err := d.xmlDocument.ValidateEnd(&d.reader, line, col); err != nil {
		return err
	}
	d.endReady = true
	return nil
}

func (d *emptyXMLDocument) LookupNamespace(prefix string) (string, bool) {
	if !d.ready {
		return "", false
	}
	return d.reader.Lookup(prefix)
}

func (d *emptyXMLDocument) CommitEnd() error {
	if !d.ready || d.Depth() == 0 {
		d.endReady = false
		err := d.commitEndState()
		if err == nil {
			d.popLexicalEnd()
		}
		return err
	}
	if !d.endReady {
		current := d.elements[len(d.elements)-1]
		lexical := xmlstream.Lexical(current.name)
		if len(d.lexicalEnds) != 0 {
			lexical = d.lexicalEnds[len(d.lexicalEnds)-1]
		}
		d.input.offset = 0
		d.input.data = appendTestXMLEndBytes(d.input.data[:0], xmlstream.EndElement{Name: xml.Name{Space: lexical.Prefix, Local: lexical.Local}})
		tok, err := d.reader.Next()
		if err != nil {
			return err
		}
		if tok.Kind != xmlstream.KindEnd {
			return errors.New("test XML input did not produce an end token")
		}
		if err := d.xmlDocument.ValidateEnd(&d.reader, 0, 0); err != nil {
			return err
		}
	}
	err := d.xmlDocument.CommitEnd(&d.reader)
	if err == nil {
		d.endReady = false
		d.popLexicalEnd()
	}
	return err
}

func (d *emptyXMLDocument) popLexicalEnd() {
	if len(d.lexicalEnds) == 0 {
		return
	}
	i := len(d.lexicalEnds) - 1
	d.lexicalEnds[i] = xmlstream.LexicalName{}
	d.lexicalEnds = d.lexicalEnds[:i]
}

func (d *emptyXMLDocument) Complete() error {
	if !d.ready {
		if d.Depth() == 0 {
			return validation(startContext{}, xsderrors.CodeValidationXML, "instance document has no root element")
		}
		return validation(d.context(0, 0), xsderrors.CodeValidationXML, "unclosed element")
	}
	return d.xmlDocument.Complete(&d.reader)
}

func (d *emptyXMLDocument) Reset(maxRetainedCap int) {
	d.xmlDocument.Reset(maxRetainedCap)
	d.reader.Detach()
	d.input = testXMLInput{}
	clear(d.lexicalEnds)
	d.lexicalEnds = d.lexicalEnds[:0]
	d.pendingLexical = xmlstream.LexicalName{}
	d.ready = false
	d.endReady = false
}

func testXMLStartBytes(start testXMLStartTag) []byte {
	var b strings.Builder
	b.WriteByte('<')
	testXMLName(&b, start.name)
	for _, attr := range start.attrs {
		b.WriteByte(' ')
		testXMLName(&b, attr.name)
		b.WriteString("=\"")
		if err := xml.EscapeText(&b, []byte(attr.value)); err != nil {
			panic(err)
		}
		b.WriteString("\"")
	}
	b.WriteByte('>')
	return []byte(b.String())
}

func appendTestXMLStartBytes(dst []byte, start testXMLStartTag) []byte {
	if len(start.attrs) != 0 {
		return append(dst, testXMLStartBytes(start)...)
	}
	dst = append(dst, '<')
	dst = appendTestXMLNameBytes(dst, start.name)
	return append(dst, '>')
}

func appendTestXMLEndBytes(dst []byte, end xmlstream.EndElement) []byte {
	dst = append(dst, '<', '/')
	dst = appendTestXMLNameBytes(dst, end.Name)
	return append(dst, '>')
}

func appendTestXMLNameBytes(dst []byte, name xml.Name) []byte {
	if name.Space != "" {
		dst = append(dst, name.Space...)
		dst = append(dst, ':')
	}
	return append(dst, name.Local...)
}

func testXMLName(b *strings.Builder, name xml.Name) {
	if name.Space != "" {
		b.WriteString(name.Space)
		b.WriteByte(':')
	}
	b.WriteString(name.Local)
}

func TestXMLDocumentStatePrepareStartRollsBackNamespaces(t *testing.T) {
	var doc emptyXMLDocument
	var values struct{}
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
	if !strings.Contains(err.Error(), `unbound namespace prefix "p"`) {
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
					name:     xml.Name{Local: "root"},
					pathMode: mode,
				}},
			}
			doc.PathString()
		}()
	}
}

func TestXMLDocumentStateRejectsDuplicateExpandedAttributes(t *testing.T) {
	var doc emptyXMLDocument
	var values struct{}
	_, err := prepareXMLStartForTest(&doc, testXMLStart(
		xml.Name{Local: "root"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:test"),
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "q"}, "urn:test"),
		testXMLAttr(xml.Name{Space: "p", Local: "id"}, ""),
		testXMLAttr(xml.Name{Space: "q", Local: "id"}, ""),
	), &values, 0, 2, 3)
	requireCode(t, err, xsderrors.CodeValidationXML)
	if !strings.Contains(err.Error(), `duplicate attribute "{urn:test}id"`) {
		t.Fatalf("PrepareStart() error = %v", err)
	}
	if _, ok := doc.LookupNamespace("p"); ok {
		t.Fatal("duplicate attribute retained namespace bindings")
	}
}

func TestXMLDocumentStateEnforcesDepthLimit(t *testing.T) {
	var values struct{}
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

func TestXMLDocumentStateReportsMultipleRootsBeforeNamespaceErrors(t *testing.T) {
	var doc emptyXMLDocument
	var values struct{}
	start, err := prepareXMLStartForTest(&doc, testXMLStart(xml.Name{Local: "a"}), &values, 0, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	doc.CommitStart(start, struct{}{})
	if endErr := doc.ValidateEnd(xmlstream.EndElement{Name: xml.Name{Local: "a"}}, 1, 4); endErr != nil {
		t.Fatal(endErr)
	}
	if commitErr := doc.CommitEnd(); commitErr != nil {
		t.Fatal(commitErr)
	}

	_, err = prepareXMLStartForTest(&doc, testXMLStart(xml.Name{Space: "p", Local: "b"}), &values, 0, 1, 5)
	if !strings.Contains(err.Error(), "multiple root elements") {
		t.Fatalf("PrepareStart() error = %v, want multiple-root error before namespace admission", err)
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
	var values struct{}
	start, err := prepareXMLStartForTest(&doc, testXMLStart(
		xml.Name{Space: "p", Local: "root"},
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "p"}, "urn:test"),
		testXMLAttr(xml.Name{Space: vocab.XMLNSPrefix, Local: "q"}, "urn:test"),
	), &values, 0, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	doc.CommitStart(start, struct{}{})
	err = doc.ValidateEnd(xmlstream.EndElement{Name: xml.Name{Space: "q", Local: "root"}}, 4, 5)
	if !strings.Contains(err.Error(), `end element "q:root" does not match start element "p:root"`) {
		t.Fatalf("ValidateEnd() error = %v", err)
	}
	if doc.Depth() != 1 {
		t.Fatalf("mismatched end changed depth to %d", doc.Depth())
	}
}

func TestXMLDocumentStateCompleteRejectsMissingAndPendingUnclosedRoot(t *testing.T) {
	var doc emptyXMLDocument
	requireCode(t, streamError(startContext{}, doc.Complete(), Limits{}), xsderrors.CodeValidationXML)

	var values struct{}
	start, err := prepareXMLStartForTest(&doc, testXMLStart(xml.Name{Local: "root"}), &values, 0, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	doc.CommitStart(start, struct{}{})
	err = streamError(startContext{}, doc.Complete(), Limits{})
	requireCode(t, err, xsderrors.CodeInternalInvariant)
	if !strings.Contains(err.Error(), "end token must be matched and committed before advancing") {
		t.Fatalf("Complete() error = %v, want pending end-token error", err)
	}
}

func TestXMLDocumentStatePathsStayLazyAndRecoverAcrossTransitions(t *testing.T) {
	var doc emptyXMLDocument
	var values struct{}
	commitDocumentStart(t, &doc, &values, "root")
	commitDocumentStart(t, &doc, &values, "child")
	if err := doc.ValidateEnd(xmlstream.EndElement{Name: xml.Name{Local: "child"}}, 2, 1); err != nil {
		t.Fatal(err)
	}
	if err := doc.CommitEnd(); err != nil {
		t.Fatal(err)
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
	if doc.Depth() != 0 {
		t.Fatalf("Reset() retained document state: %+v", doc)
	}
	if cap(doc.elements) != capacity {
		t.Fatalf("Reset() capacity = %d, want %d", cap(doc.elements), capacity)
	}
}

func TestXMLDocumentPathAtDepthProjectsAncestors(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	var values struct{}
	commitDocumentStart(t, &doc, &values, "root")
	commitDocumentStart(t, &doc, &values, "child")
	commitDocumentStart(t, &doc, &values, "grandchild")

	if got := doc.PathStringAtDepth(1); got != "/root" {
		t.Fatalf("root path = %q", got)
	}
	if got := doc.PathStringAtDepth(2); got != "/root/child" {
		t.Fatalf("child path = %q", got)
	}
	if got := doc.PathString(); got != "/root/child/grandchild" {
		t.Fatalf("current path = %q", got)
	}
	if got := doc.PathStringAtDepth(1); got != "/root" {
		t.Fatalf("cached root path = %q", got)
	}

	if err := doc.CommitEnd(); err != nil {
		t.Fatal(err)
	}
	if got := doc.PathString(); got != "/root/child" {
		t.Fatalf("path after pop = %q", got)
	}
}

func TestXMLDocumentRetainedPathsSharePrefixesAndSurvivePop(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	var values struct{}
	root := strings.Repeat("r", 100)
	commitDocumentStart(t, &doc, &values, root)
	commitDocumentStart(t, &doc, &values, "first")
	first := doc.retainPathAtDepth(2)
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
	if got := doc.retainedPathString(first); got != "/"+root+"/first" {
		t.Fatalf("first retained path = %q", got)
	}
	if got := doc.retainedPathString(second); got != "/"+root+"/second" {
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
	if got, want := doc.retainedPathString(deepest), "/root/{urn:test}middle/leaf"; got != want {
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
		if got := doc.retainedPathString(doc.retainPathAtDepth(depth + 1)); got != want {
			t.Fatalf("retained path at depth %d = %q, want %q", depth+1, got, want)
		}
	}
	if len(doc.retainedPaths.nodes) != 1 || len(doc.retainedPaths.namespaces) != 1 {
		t.Fatal("retaining annotated ancestors grew path storage")
	}
}

func TestXMLDocumentRetainedExpandedPath(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	space := strings.Repeat("u", 256)
	doc.CommitExpandedStart(preparedXMLStart{name: xml.Name{Space: space, Local: "root"}}, struct{}{})
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "child"}}, struct{}{})
	path := doc.retainPathAtDepth(2)
	if got := doc.retainedPathString(path); got != "/{"+strings.Repeat("u", 128)+"...}root/child" {
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
	if got, want := len(doc.retainedPaths.nodes), 2; got != want {
		t.Fatalf("retained expanded sibling nodes = %d, want %d", got, want)
	}
	if got, want := len(doc.retainedPaths.namespaces), 2; got != want {
		t.Fatalf("retained sibling namespace headers = %d, want %d", got, want)
	}
	for i, node := range doc.retainedPaths.nodes {
		if strings.Contains(node.text, space) || strings.IndexByte(node.text, 0) < 0 {
			t.Fatalf("expanded sibling node %d retained rendered namespace: %+v", i, node)
		}
	}
	if got, want := doc.retainedPathString(first), "/root/{"+strings.Repeat("u", 128)+"...}child"; got != want {
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
	if got, want := len(doc.retainedPaths.nodes), 2; got != want {
		t.Fatalf("retained nodes = %d, want %d", got, want)
	}
	if got, want := len(doc.retainedPaths.namespaces), commonDepth+2; got != want {
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
	if got := doc.retainedPathString(first); got != want {
		t.Fatalf("first retained path = %q, want %q", got, want)
	}
	if got := doc.retainedPathString(second); got != want {
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
	if got, want := doc.retainedPathString(doc.retainPathAtDepth(40)), strings.Repeat("/n", 40); got != want {
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
	if got, want := doc.retainedPathString(first), "/root"+strings.Repeat("/node", depth); got != want {
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
	if got, want := doc.retainedPathString(first), "/root"+strings.Repeat("/{u}n", depth); got != want {
		t.Fatalf("first retained disjoint expanded path = %q, want %q", got, want)
	}
}

func TestXMLDocumentRetainedSiblingPathsShareNodesOnlyWhenEqual(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "root"}}, struct{}{})
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "parent"}}, struct{}{})
	type sibling struct {
		name  xml.Name
		want  string
		nodes int
	}
	siblings := []sibling{
		{xml.Name{Local: "a"}, "/root/parent/a", 1},
		{xml.Name{Local: "a"}, "/root/parent/a", 2},
		{xml.Name{Local: "a"}, "/root/parent/a", 2},
		{xml.Name{Local: "ab"}, "/root/parent/ab", 3},
		{xml.Name{Space: "urn:x", Local: "a"}, "/root/parent/{urn:x}a", 4},
		{xml.Name{Space: "urn:x", Local: "a"}, "/root/parent/{urn:x}a", 4},
		{xml.Name{Space: "urn:y", Local: "a"}, "/root/parent/{urn:y}a", 5},
		{xml.Name{Local: "a"}, "/root/parent/a", 6},
	}
	var paths []retainedPath
	for i, sibling := range siblings {
		start := preparedXMLStart{name: sibling.name}
		if sibling.name.Space != "" {
			doc.CommitExpandedStart(start, struct{}{})
		} else {
			doc.CommitStart(start, struct{}{})
		}
		paths = append(paths, doc.retainPathAtDepth(3))
		if err := doc.CommitEnd(); err != nil {
			t.Fatal(err)
		}
		if got := len(doc.retainedPaths.nodes); got != sibling.nodes {
			t.Fatalf("after sibling %d retained nodes = %d, want %d", i, got, sibling.nodes)
		}
	}
	if got, want := len(doc.retainedPaths.namespaces), 2; got != want {
		t.Fatalf("retained namespace headers = %d, want %d", got, want)
	}
	if err := doc.CommitEnd(); err != nil {
		t.Fatal(err)
	}
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "other"}}, struct{}{})
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "a"}}, struct{}{})
	paths = append(paths, doc.retainPathAtDepth(3))
	siblings = append(siblings, sibling{want: "/root/other/a"})
	for i, path := range paths {
		if got := doc.retainedPathString(path); got != siblings[i].want {
			t.Fatalf("retained path %d = %q, want %q", i, got, siblings[i].want)
		}
	}
}

func TestXMLDocumentStartRollbackForgetsSiblingPathNode(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "root"}}, struct{}{})
	_ = doc.retainPathAtDepth(1)
	checkpoint := doc.startCheckpoint()
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "a"}}, struct{}{})
	_ = doc.retainPathAtDepth(2)
	doc.rollbackStart(checkpoint)
	if doc.elements[0].childPathNode != 0 {
		t.Fatalf("rollback kept sibling path node %d", doc.elements[0].childPathNode)
	}
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "b"}}, struct{}{})
	if got, want := doc.retainedPathString(doc.retainPathAtDepth(2)), "/root/b"; got != want {
		t.Fatalf("path after rollback = %q, want %q", got, want)
	}
	if err := doc.CommitEnd(); err != nil {
		t.Fatal(err)
	}
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: "b"}}, struct{}{})
	if got, want := doc.retainedPathString(doc.retainPathAtDepth(2)), "/root/b"; got != want {
		t.Fatalf("sibling path after rollback = %q, want %q", got, want)
	}
	if got, want := len(doc.retainedPaths.nodes), 2; got != want {
		t.Fatalf("retained nodes = %d, want %d", got, want)
	}
}

func TestXMLDocumentStartRollbackRemovesRetainedPathNodes(t *testing.T) {
	t.Parallel()

	var doc emptyXMLDocument
	root := strings.Repeat("r", 100)
	doc.CommitStart(preparedXMLStart{name: xml.Name{Local: root}}, struct{}{})
	checkpoint := doc.startCheckpoint()
	doc.CommitExpandedStart(preparedXMLStart{name: xml.Name{Space: "urn:test", Local: "child"}}, struct{}{})
	_ = doc.retainPathAtDepth(2)
	if len(doc.retainedPaths.nodes) != 1 || len(doc.retainedPaths.namespaces) != 1 || doc.elements[0].pathRef.node == 0 {
		t.Fatal("test setup did not retain child path")
	}
	doc.rollbackStart(checkpoint)
	if len(doc.retainedPaths.nodes) != 0 || len(doc.retainedPaths.namespaces) != 0 || doc.elements[0].pathRef != (documentPathRef{}) {
		t.Fatalf(
			"rollback retained path state: nodes=%d names=%d root=%+v",
			len(doc.retainedPaths.nodes),
			len(doc.retainedPaths.namespaces),
			doc.elements[0].pathRef,
		)
	}
	if got := doc.retainedPathString(doc.retainPathAtDepth(1)); got != "/"+root {
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
	rt, err := compileSources([]source.Source{source.Bytes("schema.xsd", []byte(`
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

func commitDocumentStart(t *testing.T, doc *emptyXMLDocument, values *struct{}, local string) {
	t.Helper()
	start, err := prepareXMLStartForTest(doc, testXMLStart(xml.Name{Local: local}), values, 0, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	doc.CommitStart(start, struct{}{})
}

func prepareXMLStartForTest(
	doc *emptyXMLDocument,
	start testXMLStartTag,
	values *struct{},
	maxDepth, line, col int,
) (preparedXMLStart, error) {
	return doc.PrepareStart(start, values, maxDepth, line, col)
}

type testXMLStartTag struct {
	name  xml.Name
	attrs []testXMLAttribute
}

type testXMLAttribute struct {
	name  xml.Name
	value string
}

func testXMLStart(name xml.Name, attrs ...testXMLAttribute) testXMLStartTag {
	return testXMLStartTag{name: name, attrs: attrs}
}

func testXMLAttr(name xml.Name, value string) testXMLAttribute {
	return testXMLAttribute{name: name, value: value}
}
