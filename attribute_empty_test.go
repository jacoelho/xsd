package xsd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestEmptyAttributeValidationContracts(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("attributes.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="ordinary"><xs:complexType>
    <xs:attribute name="optional" type="xs:string"/>
  </xs:complexType></xs:element>
  <xs:element name="wildcard"><xs:complexType>
    <xs:anyAttribute processContents="strict"/>
  </xs:complexType></xs:element>
  <xs:element name="required"><xs:complexType>
    <xs:attribute name="value" type="xs:string" use="required"/>
  </xs:complexType></xs:element>
  <xs:element name="defaulted"><xs:complexType>
    <xs:attribute name="value" type="xs:string" default="fallback"/>
  </xs:complexType><xs:key name="defaultKey"><xs:selector xpath="."/><xs:field xpath="@value"/></xs:key></xs:element>
  <xs:element name="fixed"><xs:complexType>
    <xs:attribute name="value" type="xs:string" fixed="fixed"/>
  </xs:complexType><xs:key name="fixedKey"><xs:selector xpath="."/><xs:field xpath="@value"/></xs:key></xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	tests := []struct {
		name     string
		document string
		wantErr  bool
		wantCode xsderrors.Code
	}{
		{name: "ordinary empty", document: `<ordinary/>`},
		{name: "wildcard empty", document: `<wildcard/>`},
		{name: "required missing", document: `<required/>`, wantErr: true, wantCode: xsderrors.CodeValidationAttribute},
		{name: "default omitted", document: `<defaulted/>`},
		{name: "fixed omitted", document: `<fixed/>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(test.document))
			if !test.wantErr {
				if err != nil {
					t.Fatalf("Validate(%s) error = %v, want nil", test.document, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate(%s) succeeded, want %s", test.document, test.wantCode)
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != test.wantCode {
				t.Fatalf("Validate(%s) error = %v, want diagnostic code %s", test.document, err, test.wantCode)
			}
		})
	}
}

func TestEmptyAttributesDoNotAllocateForDeclarations(t *testing.T) {
	document := "<root>" + strings.Repeat("<row/>", 128) + "</root>"
	var baseline float64
	for _, width := range []int{0, 65, 4097} {
		engine, err := xsd.Compile(xsd.Bytes("attributes.xsd", []byte(emptyAttributeSchema(width))))
		if err != nil {
			t.Fatal(err)
		}
		session, err := engine.NewSession(xsd.ValidateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		reader := strings.NewReader(document)
		allocations := testing.AllocsPerRun(100, func() {
			reader.Reset(document)
			if err := session.Validate(reader); err != nil {
				t.Fatal(err)
			}
		})
		if width == 0 {
			baseline = allocations
		} else if allocations > baseline {
			t.Errorf("%d optional declarations: %g allocations, want at most attribute-free baseline %g", width, allocations, baseline)
		}
	}
}

func BenchmarkSessionValidateEmptyAttributes(b *testing.B) {
	document := "<root>" + strings.Repeat("<row/>", 128) + "</root>"
	for _, width := range []int{64, 65, 4096, 4097} {
		b.Run(fmt.Sprintf("width_%d", width), func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("attributes.xsd", []byte(emptyAttributeSchema(width))))
			if err != nil {
				b.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				b.Fatal(err)
			}
			reader := strings.NewReader(document)
			if err := session.Validate(reader); err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(document)))
			b.ReportAllocs()
			for b.Loop() {
				reader.Reset(document)
				if err := session.Validate(reader); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func emptyAttributeSchema(width int) string {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="row" minOccurs="0" maxOccurs="unbounded"><xs:complexType>`)
	for i := range width {
		fmt.Fprintf(&schema, `<xs:attribute name="a%d" type="xs:string"/>`, i)
	}
	schema.WriteString(`<xs:anyAttribute processContents="skip"/>
    </xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	return schema.String()
}
