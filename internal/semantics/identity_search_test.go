package semantics

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestIdentitySearchDirectAndDescendantResolution(t *testing.T) {
	const schemaXML = `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="parent">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="leaf">
                <xs:complexType>
                  <xs:attribute name="descAttr" type="xs:int"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
            <xs:attribute name="directAttr" type="xs:string"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	root := schema.ElementDecls[model.QName{Namespace: "urn:test", Local: "root"}]
	if root == nil {
		t.Fatal("root element not found")
	}
	parentTest := runtime.NodeTest{
		Local:              "parent",
		Namespace:          "urn:test",
		NamespaceSpecified: true,
	}
	leafTest := runtime.NodeTest{
		Local:              "leaf",
		Namespace:          "urn:test",
		NamespaceSpecified: true,
	}
	directAttrTest := runtime.NodeTest{
		Local: "directAttr",
	}
	descAttrTest := runtime.NodeTest{
		Local: "descAttr",
	}

	parentDecl, err := findElementDecl(schema, root, parentTest)
	if err != nil {
		t.Fatalf("findElementDecl(parent) error = %v", err)
	}
	if parentDecl == nil || parentDecl.Name.Local != "parent" {
		t.Fatalf("findElementDecl(parent) = %v, want parent", parentDecl)
	}

	if _, err := findElementDecl(schema, root, leafTest); err == nil {
		t.Fatal("findElementDecl(leaf) unexpectedly succeeded for direct search")
	}

	leafDecl, err := findElementDeclDescendant(schema, root, leafTest)
	if err != nil {
		t.Fatalf("findElementDeclDescendant(leaf) error = %v", err)
	}
	if leafDecl == nil || leafDecl.Name.Local != "leaf" {
		t.Fatalf("findElementDeclDescendant(leaf) = %v, want leaf", leafDecl)
	}

	directAttrType, err := findAttributeType(schema, parentDecl, directAttrTest)
	if err != nil {
		t.Fatalf("findAttributeType(directAttr) error = %v", err)
	}
	if directAttrType == nil || directAttrType.Name().Local != "string" {
		t.Fatalf("findAttributeType(directAttr) = %v, want xs:string", directAttrType)
	}

	descAttrType, err := findAttributeTypeDescendant(schema, root, descAttrTest)
	if err != nil {
		t.Fatalf("findAttributeTypeDescendant(descAttr) error = %v", err)
	}
	if descAttrType == nil || descAttrType.Name().Local != "int" {
		t.Fatalf("findAttributeTypeDescendant(descAttr) = %v, want xs:int", descAttrType)
	}
}
