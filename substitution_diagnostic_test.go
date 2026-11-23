package xsd_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSubstitutionDiagnosticNamesAnAnonymousTypeByItsVariety(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		want   string
	}{
		{
			name:   "named member and head types",
			schema: `<xs:complexType name="H"/><xs:element name="h" type="t:H"/><xs:element name="m" type="xs:string" substitutionGroup="t:h"/>`,
			want:   `substitution group member "{urn:t}m" type "{http://www.w3.org/2001/XMLSchema}string" is not derived from head "{urn:t}h" type "{urn:t}H"`,
		},
		{
			name:   "anonymous complex member type",
			schema: `<xs:complexType name="H"/><xs:element name="h" type="t:H"/><xs:element name="m" substitutionGroup="t:h"><xs:complexType/></xs:element>`,
			want:   `substitution group member "{urn:t}m" local complex type is not derived from head "{urn:t}h" type "{urn:t}H"`,
		},
		{
			name:   "anonymous simple member type",
			schema: `<xs:element name="h" type="xs:int"/><xs:element name="m" substitutionGroup="t:h"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:element>`,
			want:   `substitution group member "{urn:t}m" local simple type is not derived from head "{urn:t}h" type "{http://www.w3.org/2001/XMLSchema}int"`,
		},
		{
			name:   "anonymous complex head type",
			schema: `<xs:element name="h"><xs:complexType/></xs:element><xs:element name="m" type="xs:string" substitutionGroup="t:h"/>`,
			want:   `substitution group member "{urn:t}m" type "{http://www.w3.org/2001/XMLSchema}string" is not derived from head "{urn:t}h" local complex type`,
		},
		{
			name:   "anonymous simple head type",
			schema: `<xs:element name="h"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:element><xs:element name="m" type="xs:string" substitutionGroup="t:h"/>`,
			want:   `substitution group member "{urn:t}m" type "{http://www.w3.org/2001/XMLSchema}string" is not derived from head "{urn:t}h" local simple type`,
		},
		{
			name:   "anonymous member and head types",
			schema: `<xs:element name="h"><xs:simpleType><xs:restriction base="xs:string"/></xs:simpleType></xs:element><xs:element name="m" substitutionGroup="t:h"><xs:complexType/></xs:element>`,
			want:   `substitution group member "{urn:t}m" local complex type is not derived from head "{urn:t}h" local simple type`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(upaSchema(test.schema))))
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != xsderrors.CodeSchemaReference {
				t.Fatalf("Compile() error = %v, want %s", err, xsderrors.CodeSchemaReference)
			}
			if got := diagnostic.Message(); got != test.want {
				t.Fatalf("Compile() message = %q, want %q", got, test.want)
			}
		})
	}
}
