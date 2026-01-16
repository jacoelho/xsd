package xsdxml

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
