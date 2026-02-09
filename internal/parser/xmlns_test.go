package parser

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func TestParseSchemaAttributesRegistersXMLNS(t *testing.T) {
	doc, root := parseSchemaDoc(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
  xmlns="urn:default"
  xmlns:ex="urn:extra"/>`)
	schema := NewSchema()
	if err := parseSchemaAttributes(doc, root, schema); err != nil {
		t.Fatalf("parseSchemaAttributes error = %v", err)
	}
	if schema.NamespaceDecls[""] != "urn:default" {
		t.Fatalf("default namespace = %q, want %q", schema.NamespaceDecls[""], "urn:default")
	}
	if schema.NamespaceDecls["ex"] != "urn:extra" {
		t.Fatalf("ex namespace = %q, want %q", schema.NamespaceDecls["ex"], "urn:extra")
	}
}

func TestResolveQNameIgnoresForeignXMLNSLocalName(t *testing.T) {
	doc, root := parseSchemaDoc(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
  xmlns:foo="urn:foo"
  foo:xmlns="urn:bad">
  <xs:element name="root" type="Type"/>
</xs:schema>`)
	schema := NewSchema()
	if err := parseSchemaAttributes(doc, root, schema); err != nil {
		t.Fatalf("parseSchemaAttributes error = %v", err)
	}

	elem := xsdxml.InvalidNode
	for _, child := range doc.Children(root) {
		if doc.NamespaceURI(child) == xsdxml.XSDNamespace && doc.LocalName(child) == "element" {
			elem = child
			break
		}
	}
	if elem == xsdxml.InvalidNode {
		t.Fatalf("expected element node")
	}

	qname, err := resolveQNameWithPolicy(doc, "Type", elem, schema, useDefaultNamespace)
	if err != nil {
		t.Fatalf("resolveQName error = %v", err)
	}
	if qname.Namespace != types.NamespaceEmpty {
		t.Fatalf("qname namespace = %q, want empty", qname.Namespace)
	}
}
