package source

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/model"
)

func TestIncludeCycleMergesSchemas(t *testing.T) {
	testFS := fstest.MapFS{
		"a.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:cycle"
           targetNamespace="urn:cycle"
           elementFormDefault="qualified">
  <xs:include schemaLocation="b.xsd"/>
  <xs:element name="a" type="xs:string"/>
</xs:schema>`),
		},
		"b.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:cycle"
           targetNamespace="urn:cycle"
           elementFormDefault="qualified">
  <xs:include schemaLocation="a.xsd"/>
  <xs:element name="b" type="xs:string"/>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: testFS})
	schemaA, err := loader.Load("a.xsd")
	if err != nil {
		t.Fatalf("Load(a.xsd) error = %v", err)
	}

	if schemaA.ElementDecls[model.QName{Namespace: "urn:cycle", Local: "a"}] == nil {
		t.Fatalf("expected element a in schemaA")
	}
	if schemaA.ElementDecls[model.QName{Namespace: "urn:cycle", Local: "b"}] == nil {
		t.Fatalf("expected element b in schemaA")
	}

	schemaB, err := loader.Load("b.xsd")
	if err != nil {
		t.Fatalf("Load(b.xsd) error = %v", err)
	}
	if schemaB.ElementDecls[model.QName{Namespace: "urn:cycle", Local: "a"}] == nil {
		t.Fatalf("expected element a in schemaB")
	}
	if schemaB.ElementDecls[model.QName{Namespace: "urn:cycle", Local: "b"}] == nil {
		t.Fatalf("expected element b in schemaB")
	}
}
