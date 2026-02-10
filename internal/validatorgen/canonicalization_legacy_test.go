package validatorgen

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestDefaultCanonicalizationMatchesGeneral(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:p="urn:ex"
           xmlns:tns="urn:ex"
           targetNamespace="urn:ex"
           elementFormDefault="qualified">
  <xs:simpleType name="QNameOrString">
    <xs:union memberTypes="xs:QName xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="QNameList">
    <xs:list itemType="xs:QName"/>
  </xs:simpleType>
</xs:schema>`

	sch, reg, err := parseAndAssign(schemaXML)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	comp := newCompiler(sch)
	comp.registry = reg
	comp.initRuntimeTypeIDs(reg)

	ctx := map[string]string{"p": "urn:ex"}
	cases := []struct {
		name     string
		typeName string
		lexical  string
	}{
		{name: "union qname picks qname member", typeName: "QNameOrString", lexical: "p:val"},
		{name: "list of qname canonicalizes items", typeName: "QNameList", lexical: "p:one p:two"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			typ := sch.TypeDefs[model.QName{Namespace: sch.TargetNamespace, Local: tc.typeName}]
			if typ == nil {
				t.Fatalf("type %s not found", tc.typeName)
			}
			normalized := comp.normalizeLexical(tc.lexical, typ)
			gotDefault, err := comp.canonicalizeNormalizedCore(tc.lexical, normalized, typ, ctx, canonicalizeDefault)
			if err != nil {
				t.Fatalf("canonicalize default: %v", err)
			}
			gotGeneral, err := comp.canonicalizeNormalizedCore(tc.lexical, normalized, typ, ctx, canonicalizeGeneral)
			if err != nil {
				t.Fatalf("canonicalize general: %v", err)
			}
			if !bytes.Equal(gotDefault, gotGeneral) {
				t.Fatalf("canonical mismatch: default=%q general=%q", gotDefault, gotGeneral)
			}
		})
	}
}
