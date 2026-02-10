package source

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/model"
)

func TestLoadKeepsTypeReferencesSymbolic(t *testing.T) {
	fsys := fstest.MapFS{
		"main.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:element name="v" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:extension base="tns:BaseType"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`)},
	}

	loader := NewLoader(Config{FS: fsys})
	sch, err := loader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	derivedQName := model.QName{Namespace: "urn:test", Local: "DerivedType"}
	derived, ok := sch.TypeDefs[derivedQName].(*model.ComplexType)
	if !ok || derived == nil {
		t.Fatalf("expected complex type %s", derivedQName)
	}
	if derived.ResolvedBase != nil {
		t.Fatal("source phase should keep complexType base unresolved")
	}
}

func TestLoadKeepsIdentityFieldTypesUnresolved(t *testing.T) {
	fsys := fstest.MapFS{
		"main.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="number" type="xs:integer"/>
    </xs:complexType>
    <xs:key name="rootKey">
      <xs:selector xpath="."/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`)},
	}

	loader := NewLoader(Config{FS: fsys})
	sch, err := loader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	root := sch.ElementDecls[model.QName{Namespace: "urn:test", Local: "root"}]
	if root == nil || len(root.Constraints) == 0 || len(root.Constraints[0].Fields) == 0 {
		t.Fatal("expected root key field")
	}
	if root.Constraints[0].Fields[0].ResolvedType != nil {
		t.Fatal("source phase should keep key field type unresolved")
	}
}
