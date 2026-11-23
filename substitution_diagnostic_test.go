package xsd_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSubstitutionDiagnosticNamesAnAnonymousTypeByItsKind(t *testing.T) {
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

func TestSubstitutionDiagnosticNamesAnUnavailableTypeApartFromANamedTypeOfTheSameName(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:o="urn:o">
  <xs:import namespace="urn:o"/>
  <xs:simpleType name="missing"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:element name="h" type="missing"/>
  <xs:element name="m" type="o:Gone" substitutionGroup="h"/>
</xs:schema>`
	_, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Code() != xsderrors.CodeSchemaReference {
		t.Fatalf("Compile() error = %v, want %s", err, xsderrors.CodeSchemaReference)
	}
	const want = `substitution group member "m" unavailable type is not derived from head "h" type "missing"`
	if got := diagnostic.Message(); got != want {
		t.Fatalf("Compile() message = %q, want %q", got, want)
	}
}

func TestCyclicTypeDiagnosticNamesTheType(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		schema string
		want   string
	}{
		{
			name:   "complex",
			schema: `<xs:complexType name="A"><xs:complexContent><xs:extension base="t:B"/></xs:complexContent></xs:complexType><xs:complexType name="B"><xs:complexContent><xs:extension base="t:A"/></xs:complexContent></xs:complexType><xs:element name="r" type="t:A"/>`,
			want:   `cyclic complex type "{urn:t}A"`,
		},
		{
			name:   "simple",
			schema: `<xs:simpleType name="A"><xs:restriction base="t:B"/></xs:simpleType><xs:simpleType name="B"><xs:restriction base="t:A"/></xs:simpleType><xs:element name="r" type="t:A"/>`,
			want:   `cyclic simple type "{urn:t}A"`,
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
