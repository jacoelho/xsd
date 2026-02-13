package semanticresolve_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semanticcheck"
	"github.com/jacoelho/xsd/internal/semanticresolve"
)

func TestFacetValidationConsistentAcrossPhases(t *testing.T) {
	cases := []struct {
		name     string
		schema   string
		wantPass bool
	}{
		{
			name: "valid qname list length facets",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:p="urn:ex"
           xmlns:tns="urn:ex"
           targetNamespace="urn:ex"
           elementFormDefault="qualified">
  <xs:simpleType name="QNameEnum">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="p:val"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="QNameLen">
    <xs:restriction base="xs:QName">
      <xs:length value="1"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="TokenList">
    <xs:restriction base="xs:NMTOKENS">
      <xs:whiteSpace value="collapse"/>
      <xs:length value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="qenum" type="tns:QNameEnum" default="p:val"/>
  <xs:element name="qlen" type="tns:QNameLen" default="p:longname"/>
  <xs:element name="list" type="tns:TokenList" default="a b"/>
</xs:schema>`,
			wantPass: true,
		},
		{
			name: "invalid qname enum and list length",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:p="urn:ex"
           xmlns:tns="urn:ex"
           targetNamespace="urn:ex"
           elementFormDefault="qualified">
  <xs:simpleType name="QNameEnum">
    <xs:restriction base="xs:QName">
      <xs:enumeration value="p:val"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="QNameLen">
    <xs:restriction base="xs:QName">
      <xs:length value="1"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:simpleType name="TokenList">
    <xs:restriction base="xs:NMTOKENS">
      <xs:whiteSpace value="collapse"/>
      <xs:length value="2"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="qenum" type="tns:QNameEnum" default="p:other"/>
  <xs:element name="qlen" type="tns:QNameLen" default="p:longname"/>
  <xs:element name="list" type="tns:TokenList" default="a b c"/>
</xs:schema>`,
			wantPass: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schemaOK, schemaErrs := runSchemacheck(t, tc.schema)
			resolveOK, resolveErrs := runResolverValidation(t, tc.schema)

			if schemaOK != resolveOK {
				t.Fatalf("schemacheck ok=%t errs=%v; resolver ok=%t errs=%v", schemaOK, schemaErrs, resolveOK, resolveErrs)
			}
			if schemaOK != tc.wantPass {
				t.Fatalf("schemacheck ok=%t, want %t (errs=%v)", schemaOK, tc.wantPass, schemaErrs)
			}
		})
	}
}

func runSchemacheck(t *testing.T, schemaXML string) (bool, []error) {
	t.Helper()
	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	errs := semanticcheck.ValidateStructure(schema)
	return len(errs) == 0, errs
}

func runResolverValidation(t *testing.T, schemaXML string) (bool, []error) {
	t.Helper()
	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	res := semanticresolve.NewResolver(schema)
	if err := res.Resolve(); err != nil {
		t.Fatalf("resolve schema: %v", err)
	}
	errs := semanticresolve.ValidateReferences(schema)
	return len(errs) == 0, errs
}
