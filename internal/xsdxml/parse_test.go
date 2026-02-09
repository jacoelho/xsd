package xsdxml

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/xmllex"
)

func TestParse(t *testing.T) {
	xmlData := `<root xmlns="http://example.com">
		<child attr="value">text content</child>
		<child2>more text</child2>
	</root>`

	doc, err := Parse(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root := doc.DocumentElement()
	if root == InvalidNode {
		t.Fatal("DocumentElement() returned invalid node")
	}

	if doc.LocalName(root) != "root" {
		t.Errorf("root LocalName() = %v, want %v", doc.LocalName(root), "root")
	}

	if doc.NamespaceURI(root) != "http://example.com" {
		t.Errorf("root NamespaceURI() = %v, want %v", doc.NamespaceURI(root), "http://example.com")
	}

	children := doc.Children(root)
	if len(children) != 2 {
		t.Fatalf("root has %d children, want 2", len(children))
	}

	child := children[0]
	if doc.LocalName(child) != "child" {
		t.Errorf("child LocalName() = %v, want %v", doc.LocalName(child), "child")
	}

	if !doc.HasAttribute(child, "attr") {
		t.Error("child should have 'attr' attribute")
	}

	if got := doc.GetAttribute(child, "attr"); got != "value" {
		t.Errorf("GetAttribute(attr) = %v, want %v", got, "value")
	}

	if got := doc.TextContent(child); got != "text content" {
		t.Errorf("TextContent() = %v, want %v", got, "text content")
	}
}

func TestElement_Attributes(t *testing.T) {
	xmlData := `<root attr1="val1" attr2="val2" xmlns:ns="http://ns.com" ns:attr3="val3"></root>`

	doc, err := Parse(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root := doc.DocumentElement()
	attrs := doc.Attributes(root)

	if len(attrs) < 2 {
		t.Errorf("Attributes() returned %d attributes, want at least 2", len(attrs))
	}

	foundAttr1 := false
	for _, attr := range attrs {
		if attr.LocalName() == "attr1" && attr.Value() == "val1" {
			foundAttr1 = true
			break
		}
	}

	if !foundAttr1 {
		t.Error("attr1 with value val1 not found in attributes")
	}
}

func TestParseNamespaceDeclarations(t *testing.T) {
	xmlData := `<root xmlns="urn:default" xmlns:p="urn:prefix"><p:child/></root>`

	doc, err := Parse(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root := doc.DocumentElement()
	attrs := doc.Attributes(root)
	foundDefault := false
	foundPrefix := false
	for _, attr := range attrs {
		if attr.NamespaceURI() != XMLNSNamespace {
			continue
		}
		if attr.LocalName() == "xmlns" && attr.Value() == "urn:default" {
			foundDefault = true
		}
		if attr.LocalName() == "p" && attr.Value() == "urn:prefix" {
			foundPrefix = true
		}
	}
	if !foundDefault || !foundPrefix {
		t.Fatalf("xmlns attrs: default=%v prefix=%v, want true, true", foundDefault, foundPrefix)
	}
}

func TestParseRejectsReservedNamespacePrefixes(t *testing.T) {
	cases := []string{
		`<root xmlns:xml="urn:wrong"><child/></root>`,
		`<root xmlns:xmlns="urn:wrong"><child/></root>`,
	}
	for _, xmlData := range cases {
		if _, err := Parse(strings.NewReader(xmlData)); err == nil {
			t.Fatalf("expected error for reserved namespace declaration: %s", xmlData)
		}
	}
}

func TestParseNamespaceUndeclareRedeclare(t *testing.T) {
	xmlData := `<root xmlns="a"><child xmlns=""><grand xmlns="b"/></child></root>`

	doc, err := Parse(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root := doc.DocumentElement()
	children := doc.Children(root)
	if len(children) != 1 {
		t.Fatalf("root children = %d, want 1", len(children))
	}
	child := children[0]
	grandChildren := doc.Children(child)
	if len(grandChildren) != 1 {
		t.Fatalf("child children = %d, want 1", len(grandChildren))
	}
	grand := grandChildren[0]

	rootXMLNS := ""
	for _, attr := range doc.Attributes(root) {
		if attr.NamespaceURI() == XMLNSNamespace && attr.LocalName() == "xmlns" {
			rootXMLNS = attr.Value()
		}
	}
	if rootXMLNS != "a" {
		t.Fatalf("root xmlns = %q, want a", rootXMLNS)
	}

	childXMLNS := ""
	for _, attr := range doc.Attributes(child) {
		if attr.NamespaceURI() == XMLNSNamespace && attr.LocalName() == "xmlns" {
			childXMLNS = attr.Value()
		}
	}
	if childXMLNS != "" {
		t.Fatalf("child xmlns = %q, want empty", childXMLNS)
	}

	grandXMLNS := ""
	for _, attr := range doc.Attributes(grand) {
		if attr.NamespaceURI() == XMLNSNamespace && attr.LocalName() == "xmlns" {
			grandXMLNS = attr.Value()
		}
	}
	if grandXMLNS != "b" {
		t.Fatalf("grand xmlns = %q, want b", grandXMLNS)
	}
}

type errReader struct {
	err error
}

func (r errReader) Read(p []byte) (int, error) {
	return 0, r.err
}

func TestParseIntoWrapsReadError(t *testing.T) {
	sentinel := errors.New("read failure")
	doc := &Document{root: InvalidNode}

	err := ParseIntoWithOptions(errReader{err: sentinel}, doc)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want wrapped %v", err, sentinel)
	}
}

func TestTextContent(t *testing.T) {
	xmlData := `<root>
		text 1
		<child>child text</child>
		text 2
	</root>`

	doc, err := Parse(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root := doc.DocumentElement()
	content := doc.TextContent(root)

	if !strings.Contains(content, "text 1") {
		t.Errorf("TextContent() = %q, should contain 'text 1'", content)
	}

	if !strings.Contains(content, "child text") {
		t.Errorf("TextContent() = %q, should contain 'child text'", content)
	}

	if !strings.Contains(content, "text 2") {
		t.Errorf("TextContent() = %q, should contain 'text 2'", content)
	}
}

func TestParseIntoResetsOnError(t *testing.T) {
	doc := &Document{}
	if err := ParseIntoWithOptions(strings.NewReader("<root><child/></root>"), doc); err != nil {
		t.Fatalf("ParseIntoWithOptions() error = %v", err)
	}
	if doc.DocumentElement() == InvalidNode {
		t.Fatalf("expected document element after successful parse")
	}

	if err := ParseIntoWithOptions(strings.NewReader("<root>"), doc); err == nil {
		t.Fatalf("expected parse error for malformed XML")
	}
	if doc.DocumentElement() != InvalidNode {
		t.Fatalf("expected document to be reset after parse error")
	}
	if len(doc.nodes) != 0 || len(doc.attrs) != 0 || len(doc.children) != 0 {
		t.Fatalf("expected document arenas to be cleared after error")
	}
}

func TestTextContentOrder(t *testing.T) {
	xmlData := `<root>a <child>b</child> c</root>`

	doc, err := Parse(strings.NewReader(xmlData))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	got := doc.TextContent(doc.DocumentElement())
	if got != "a b c" {
		t.Errorf("TextContent() = %q, want %q", got, "a b c")
	}
}

func TestDocumentOutOfBoundsNodeID(t *testing.T) {
	doc, err := Parse(strings.NewReader(`<root><child/></root>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	badID := NodeID(len(doc.nodes))
	if doc.Parent(badID) != InvalidNode {
		t.Fatalf("Parent out-of-bounds should return InvalidNode")
	}
	if doc.NamespaceURI(badID) != "" {
		t.Fatalf("NamespaceURI out-of-bounds should be empty")
	}
	if doc.LocalName(badID) != "" {
		t.Fatalf("LocalName out-of-bounds should be empty")
	}
	if attrs := doc.Attributes(badID); attrs != nil {
		t.Fatalf("Attributes out-of-bounds should be nil")
	}
	if children := doc.Children(badID); children != nil {
		t.Fatalf("Children out-of-bounds should be nil")
	}
	if doc.DirectTextContentBytes(badID) != nil {
		t.Fatalf("DirectTextContentBytes out-of-bounds should be nil")
	}
	if doc.TextContent(badID) != "" {
		t.Fatalf("TextContent out-of-bounds should be empty")
	}
}

func TestParseRejectsNonXMLWhitespaceOutsideRoot(t *testing.T) {
	xmlData := "\u00a0<root/>"
	if _, err := Parse(strings.NewReader(xmlData)); err == nil {
		t.Fatal("Parse() should reject non-XML whitespace outside root")
	}
}

func TestParseRejectsBOMAfterRoot(t *testing.T) {
	xmlData := "<root/>\uFEFF"
	if _, err := Parse(strings.NewReader(xmlData)); err == nil {
		t.Fatal("Parse() should reject BOM outside the document start")
	}
}

func TestParseRejectsBOMAfterNonContentTokens(t *testing.T) {
	cases := []string{
		"<?xml version=\"1.0\"?>\ufeff<root/>",
		"<!--c-->\ufeff<root/>",
		"<?pi?>\ufeff<root/>",
	}
	for _, xmlData := range cases {
		if _, err := Parse(strings.NewReader(xmlData)); err == nil {
			t.Fatalf("expected parse error for BOM after non-content token: %q", xmlData)
		}
	}
}

func TestIsIgnorableOutsideRoot(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		allowBOM bool
		want     bool
	}{
		{"empty", nil, true, true},
		{"whitespace", []byte(" \t\r\n"), true, true},
		{"bom only allowed", []byte{0xef, 0xbb, 0xbf}, true, true},
		{"bom only disallowed", []byte{0xef, 0xbb, 0xbf}, false, false},
		{"bom and whitespace", []byte{0xef, 0xbb, 0xbf, ' ', '\n'}, true, true},
		{"whitespace then bom", []byte{' ', 0xef, 0xbb, 0xbf}, true, false},
		{"non-whitespace", []byte("x"), true, false},
		{"bom and non-whitespace", []byte{0xef, 0xbb, 0xbf, 'x'}, true, false},
		{"invalid utf8", []byte{0xff}, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xmllex.IsIgnorableOutsideRoot(tt.data, tt.allowBOM); got != tt.want {
				t.Fatalf("IsIgnorableOutsideRoot() = %v, want %v", got, tt.want)
			}
		})
	}
}
