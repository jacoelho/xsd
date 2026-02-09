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

	gotType, err := resolveQNameWithPolicy(doc, "RootType", root, schema, useDefaultNamespace)
	if err != nil {
		t.Fatalf("resolveQName error = %v", err)
	}
	gotElem, err := resolveQNameWithPolicy(doc, "RootType", root, schema, useDefaultNamespace)
	if err != nil {
		t.Fatalf("resolveQName (element) error = %v", err)
	}
	if gotType != gotElem {
		t.Fatalf("resolveQNameWithPolicy(type) = %s, resolveQNameWithPolicy(element) = %s", gotType, gotElem)
	}
}
