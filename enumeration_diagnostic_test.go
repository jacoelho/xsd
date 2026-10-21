package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestImportedEnumerationDiagnostics(t *testing.T) {
	const dependency = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
  targetNamespace="http://www.w3.org/1999/xlink" xmlns:xlink="http://www.w3.org/1999/xlink">
  <xs:attribute name="actuate" type="xlink:actuateType"/>
  <xs:simpleType name="actuateType">
    <xs:restriction base="xs:token">
      <xs:enumeration value="onLoad"/>
      <xs:enumeration value="onRequest"/>
    </xs:restriction>
  </xs:simpleType>
</xs:schema>`
	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xlink="http://www.w3.org/1999/xlink"
  targetNamespace="urn:t" xmlns="urn:t" elementFormDefault="qualified">
  <xs:import namespace="http://www.w3.org/1999/xlink" schemaLocation="xlink.xsd"/>
  <xs:element name="P"><xs:complexType><xs:attribute ref="xlink:actuate"/></xs:complexType></xs:element>
  <xs:element name="E" type="xlink:actuateType"/>
</xs:schema>`
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		if location != "xlink.xsd" {
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
		return xsd.Bytes("xlink.xsd", []byte(dependency)), nil
	})
	engine, err := xsd.Compile(xsd.Bytes("main.xsd", []byte(root)).WithResolver(resolver))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, document, message, path string
		column                        int
	}{
		{name: "attribute", document: `<P xmlns="urn:t" xmlns:xlink="http://www.w3.org/1999/xlink" xlink:actuate="bogus"/>`, message: `invalid attribute "{http://www.w3.org/1999/xlink}actuate": value "bogus" is not one of ["onLoad", "onRequest"]`, path: "/P", column: 1},
		{name: "escaped attribute", document: `<P xmlns="urn:t" xmlns:xlink="http://www.w3.org/1999/xlink" xlink:actuate="b&#111;gus"/>`, message: `invalid attribute "{http://www.w3.org/1999/xlink}actuate": value "bogus" is not one of ["onLoad", "onRequest"]`, path: "/P", column: 1},
		{name: "element", document: `<E xmlns="urn:t">bogus</E>`, message: `invalid simple content: value "bogus" is not one of ["onLoad", "onRequest"]`, path: "/E", column: 23},
		{name: "onLoad", document: `<P xmlns="urn:t" xmlns:xlink="http://www.w3.org/1999/xlink" xlink:actuate="onLoad"/>`},
		{name: "onRequest", document: `<P xmlns="urn:t" xmlns:xlink="http://www.w3.org/1999/xlink" xlink:actuate="onRequest"/>`},
		{name: "normalized token", document: `<P xmlns="urn:t" xmlns:xlink="http://www.w3.org/1999/xlink" xlink:actuate=" onLoad "/>`},
		{name: "element control", document: `<E xmlns="urn:t">onRequest</E>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(test.document))
			if test.message == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
			assertStartDiagnostics(t, err, []startDiagnostic{{code: xsderrors.CodeValidationFacet, path: test.path, line: 1, column: test.column}})
			if got := err.Error(); !strings.HasSuffix(got, test.message) {
				t.Fatalf("diagnostic = %q, want suffix %q", got, test.message)
			}
		})
	}
}
