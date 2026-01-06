package xml

import (
	"strings"
	"testing"
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
	if root == nil {
		t.Fatal("DocumentElement() returned nil")
	}

	if root.LocalName() != "root" {
		t.Errorf("root LocalName() = %v, want %v", root.LocalName(), "root")
	}

	if root.NamespaceURI() != "http://example.com" {
		t.Errorf("root NamespaceURI() = %v, want %v", root.NamespaceURI(), "http://example.com")
	}

	children := root.Children()
	if len(children) != 2 {
		t.Fatalf("root has %d children, want 2", len(children))
	}

	child := children[0]
	if child.LocalName() != "child" {
		t.Errorf("child LocalName() = %v, want %v", child.LocalName(), "child")
	}

	if !child.HasAttribute("attr") {
		t.Error("child should have 'attr' attribute")
	}

	if got := child.GetAttribute("attr"); got != "value" {
		t.Errorf("GetAttribute(attr) = %v, want %v", got, "value")
	}

	if got := child.TextContent(); got != "text content" {
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
	attrs := root.Attributes()

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
	content := root.TextContent()

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
