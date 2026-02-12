package loadmerge

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
)

func TestCloneSchemaDeepIsolation(t *testing.T) {
	t.Parallel()

	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string">
      <xs:minLength value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="tns:Code"/>
</xs:schema>`

	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	cloned, err := CloneSchemaDeep(parsed)
	if err != nil {
		t.Fatalf("CloneSchemaDeep() error = %v", err)
	}

	codeName := model.QName{Namespace: "urn:test", Local: "Code"}
	rootName := model.QName{Namespace: "urn:test", Local: "root"}

	origType, ok := parsed.TypeDefs[codeName].(*model.SimpleType)
	if !ok || origType == nil {
		t.Fatalf("expected original simple type %s", codeName)
	}
	clonedType, ok := cloned.TypeDefs[codeName].(*model.SimpleType)
	if !ok || clonedType == nil {
		t.Fatalf("expected cloned simple type %s", codeName)
	}
	if origType == clonedType {
		t.Fatal("expected deep-cloned simple type pointer")
	}

	origElem := parsed.ElementDecls[rootName]
	clonedElem := cloned.ElementDecls[rootName]
	if origElem == nil || clonedElem == nil {
		t.Fatalf("expected original and cloned element %s", rootName)
	}
	if origElem == clonedElem {
		t.Fatal("expected deep-cloned element pointer")
	}

	origType.Restriction.Base = model.QName{Namespace: model.XSDNamespace, Local: "int"}
	if clonedType.Restriction == nil {
		t.Fatal("expected cloned restriction")
	}
	if clonedType.Restriction.Base.Local != "string" {
		t.Fatalf("cloned restriction base = %s, want string", clonedType.Restriction.Base.Local)
	}
}
