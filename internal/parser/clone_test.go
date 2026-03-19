package parser

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestCloneSchemaIsolation(t *testing.T) {
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

	parsed, err := Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	cloned := CloneSchema(parsed)

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

	parsed.NamespaceDecls["tns"] = "urn:mutated"
	if cloned.NamespaceDecls["tns"] != "urn:test" {
		t.Fatalf("cloned namespace decl = %q, want urn:test", cloned.NamespaceDecls["tns"])
	}
}

func TestCloneSchemaForMergeCopiesMetadataAndSlices(t *testing.T) {
	t.Parallel()

	schema := NewSchema()
	schema.Location = "schema.xsd"
	schema.TargetNamespace = "urn:test"
	schema.NamespaceDecls["tns"] = "urn:test"
	schema.ImportContexts[schema.Location] = ImportContext{
		Imports: map[model.NamespaceURI]bool{
			"urn:dep": true,
		},
		TargetNamespace: schema.TargetNamespace,
	}

	head := model.QName{Namespace: schema.TargetNamespace, Local: "head"}
	member := model.QName{Namespace: schema.TargetNamespace, Local: "member"}
	schema.SubstitutionGroups[head] = []model.QName{member}

	cloned := CloneSchemaForMerge(schema)

	schema.NamespaceDecls["tns"] = "urn:changed"
	ctx := schema.ImportContexts[schema.Location]
	ctx.Imports["urn:other"] = true
	schema.ImportContexts[schema.Location] = ctx
	schema.SubstitutionGroups[head][0] = model.QName{Namespace: schema.TargetNamespace, Local: "other"}

	if cloned.NamespaceDecls["tns"] != "urn:test" {
		t.Fatalf("cloned namespace decl = %q, want urn:test", cloned.NamespaceDecls["tns"])
	}
	if cloned.ImportContexts[schema.Location].Imports["urn:other"] {
		t.Fatal("cloned import context unexpectedly shared imports map")
	}
	if got := cloned.SubstitutionGroups[head][0]; got != member {
		t.Fatalf("cloned substitution group member = %v, want %v", got, member)
	}
}
