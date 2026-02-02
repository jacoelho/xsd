package parser

import "testing"

func TestResolveQNameMatchesElementQName(t *testing.T) {
	doc, root := parseSchemaDoc(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
  xmlns="urn:default"
  targetNamespace="urn:default"/>`)
	schema := NewSchema()
	if err := parseSchemaAttributes(doc, root, schema); err != nil {
		t.Fatalf("parseSchemaAttributes error = %v", err)
	}

	gotType, err := resolveQName(doc, "RootType", root, schema)
	if err != nil {
		t.Fatalf("resolveQName error = %v", err)
	}
	gotElem, err := resolveElementQName(doc, "RootType", root, schema)
	if err != nil {
		t.Fatalf("resolveElementQName error = %v", err)
	}
	if gotType != gotElem {
		t.Fatalf("resolveQName = %s, resolveElementQName = %s", gotType, gotElem)
	}
}
