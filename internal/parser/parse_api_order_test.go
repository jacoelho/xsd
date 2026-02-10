package parser

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestParseWithImportsPreservesDirectiveAndDeclarationOrder(t *testing.T) {
	result, err := ParseWithImportsOptions(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:import namespace="urn:one" schemaLocation="a.xsd"/>
  <xs:include schemaLocation="b.xsd"/>
  <xs:element name="root" type="xs:string"/>
  <xs:include schemaLocation="c.xsd"/>
  <xs:simpleType name="Code">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:include schemaLocation="d.xsd"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("ParseWithImportsOptions() error = %v", err)
	}

	if len(result.Directives) != 4 {
		t.Fatalf("Directives length = %d, want 4", len(result.Directives))
	}
	if result.Directives[0].Kind != DirectiveImport {
		t.Fatalf("directive[0] = %v, want import", result.Directives[0].Kind)
	}
	if result.Directives[1].Kind != DirectiveInclude {
		t.Fatalf("directive[1] = %v, want include", result.Directives[1].Kind)
	}
	if result.Directives[2].Kind != DirectiveInclude {
		t.Fatalf("directive[2] = %v, want include", result.Directives[2].Kind)
	}
	if result.Directives[3].Kind != DirectiveInclude {
		t.Fatalf("directive[3] = %v, want include", result.Directives[3].Kind)
	}

	if len(result.Includes) != 3 {
		t.Fatalf("Includes length = %d, want 3", len(result.Includes))
	}
	if result.Includes[0].SchemaLocation != "b.xsd" || result.Includes[0].DeclIndex != 0 || result.Includes[0].IncludeIndex != 0 {
		t.Fatalf("include[0] = %+v, want b.xsd@decl0/index0", result.Includes[0])
	}
	if result.Includes[1].SchemaLocation != "c.xsd" || result.Includes[1].DeclIndex != 1 || result.Includes[1].IncludeIndex != 1 {
		t.Fatalf("include[1] = %+v, want c.xsd@decl1/index1", result.Includes[1])
	}
	if result.Includes[2].SchemaLocation != "d.xsd" || result.Includes[2].DeclIndex != 2 || result.Includes[2].IncludeIndex != 2 {
		t.Fatalf("include[2] = %+v, want d.xsd@decl2/index2", result.Includes[2])
	}

	wantDecls := []GlobalDecl{
		{Kind: GlobalDeclElement, Name: model.QName{Namespace: "urn:test", Local: "root"}},
		{Kind: GlobalDeclType, Name: model.QName{Namespace: "urn:test", Local: "Code"}},
	}
	if len(result.Schema.GlobalDecls) != len(wantDecls) {
		t.Fatalf("GlobalDecls length = %d, want %d", len(result.Schema.GlobalDecls), len(wantDecls))
	}
	for i := range wantDecls {
		if result.Schema.GlobalDecls[i] != wantDecls[i] {
			t.Fatalf("GlobalDecls[%d] = %+v, want %+v", i, result.Schema.GlobalDecls[i], wantDecls[i])
		}
	}
}
