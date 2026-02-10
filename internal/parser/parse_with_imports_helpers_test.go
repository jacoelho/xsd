package parser

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func TestParseSchemaAttributes(t *testing.T) {
	doc, root := parseSchemaDoc(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"/>`)
	schema := NewSchema()
	if err := parseSchemaAttributes(doc, root, schema); err != nil {
		t.Fatalf("parseSchemaAttributes: %v", err)
	}
	if schema.TargetNamespace != model.NamespaceURI("urn:test") {
		t.Fatalf("target namespace = %q, want %q", schema.TargetNamespace, "urn:test")
	}
}

func TestParseDirectives(t *testing.T) {
	doc, root := parseSchemaDoc(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:import namespace="urn:one" schemaLocation="a.xsd"/>
  <xs:include schemaLocation="b.xsd"/>
</xs:schema>`)
	schema := NewSchema()
	if err := parseSchemaAttributes(doc, root, schema); err != nil {
		t.Fatalf("parseSchemaAttributes: %v", err)
	}
	result := &ParseResult{Schema: schema}
	imported, err := parseDirectives(doc, root, schema, result)
	if err != nil {
		t.Fatalf("parseDirectives: %v", err)
	}
	if len(result.Imports) != 1 || len(result.Includes) != 1 || len(result.Directives) != 2 {
		t.Fatalf("imports=%d includes=%d directives=%d, want 1,1,2", len(result.Imports), len(result.Includes), len(result.Directives))
	}
	if result.Directives[0].Kind != DirectiveImport || result.Directives[1].Kind != DirectiveInclude {
		t.Fatalf("directive order = %v, want import then include", []DirectiveKind{result.Directives[0].Kind, result.Directives[1].Kind})
	}
	if !imported[model.NamespaceURI("urn:one")] {
		t.Fatalf("expected namespace urn:one to be recorded as imported")
	}
}

func TestParseComponents(t *testing.T) {
	doc, root := parseSchemaDoc(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
	schema := NewSchema()
	if err := parseSchemaAttributes(doc, root, schema); err != nil {
		t.Fatalf("parseSchemaAttributes: %v", err)
	}
	result := &ParseResult{Schema: schema}
	imported, err := parseDirectives(doc, root, schema, result)
	if err != nil {
		t.Fatalf("parseDirectives: %v", err)
	}
	applyImportedNamespaces(schema, imported)
	if err := parseComponents(doc, root, schema); err != nil {
		t.Fatalf("parseComponents: %v", err)
	}
	qname := model.QName{Namespace: model.NamespaceURI("urn:test"), Local: "root"}
	if _, ok := schema.ElementDecls[qname]; !ok {
		t.Fatalf("expected element %s to be parsed", qname)
	}
}

func parseSchemaDoc(t *testing.T, src string) (*xsdxml.Document, xsdxml.NodeID) {
	t.Helper()
	pool := xsdxml.NewDocumentPool()
	doc := pool.Acquire()
	t.Cleanup(func() {
		pool.Release(doc)
	})
	if err := xsdxml.ParseIntoWithOptions(strings.NewReader(src), doc); err != nil {
		t.Fatalf("parse XML: %v", err)
	}
	root := doc.DocumentElement()
	if root == xsdxml.InvalidNode {
		t.Fatalf("empty document")
	}
	return doc, root
}
