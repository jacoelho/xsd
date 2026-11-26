package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestFixedValueViolationOnIdentityFieldReportsOneDiagnosticOnEveryAdmissionPath(t *testing.T) {
	t.Parallel()
	engine, err := xsd.Compile(xsd.Bytes("fixed-identity.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="a" type="xs:string"/>
  <xs:attribute name="b" type="xs:int" fixed="1"/>
  <xs:element name="pa"><xs:complexType><xs:sequence>
    <xs:element name="c"><xs:complexType>
      <xs:attribute name="a" type="xs:string"/>
      <xs:attribute name="b" type="xs:int" fixed="1"/>
    </xs:complexType></xs:element>
  </xs:sequence></xs:complexType>
    <xs:unique name="UA"><xs:selector xpath="c"/><xs:field xpath="@a|@b"/></xs:unique>
  </xs:element>
  <xs:element name="pw"><xs:complexType><xs:sequence>
    <xs:element name="c"><xs:complexType><xs:anyAttribute processContents="strict"/></xs:complexType></xs:element>
  </xs:sequence></xs:complexType>
    <xs:unique name="UW"><xs:selector xpath="c"/><xs:field xpath="@a|@b"/></xs:unique>
  </xs:element>
  <xs:element name="pe"><xs:complexType><xs:sequence>
    <xs:element name="c"><xs:complexType><xs:sequence>
      <xs:element name="e" type="xs:string" minOccurs="0"/>
      <xs:element name="f" type="xs:int" fixed="1" minOccurs="0"/>
    </xs:sequence></xs:complexType></xs:element>
  </xs:sequence></xs:complexType>
    <xs:unique name="UE"><xs:selector xpath="c"/><xs:field xpath="e|f"/></xs:unique>
  </xs:element>
  <xs:element name="ps"><xs:complexType><xs:sequence>
    <xs:element name="c"><xs:complexType>
      <xs:attribute name="a" type="xs:string"/>
      <xs:attribute name="b" type="xs:string" fixed="x"/>
    </xs:complexType></xs:element>
  </xs:sequence></xs:complexType>
    <xs:unique name="US"><xs:selector xpath="c"/><xs:field xpath="@a|@b"/></xs:unique>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name string
		doc  string
		want []xsderrors.Code
	}{
		{"declared attribute violation", `<pa><c a="x" b="2"/></pa>`, []xsderrors.Code{xsderrors.CodeValidationAttribute}},
		{"wildcard attribute violation", `<pw><c a="x" b="2"/></pw>`, []xsderrors.Code{xsderrors.CodeValidationAttribute}},
		{"element field violation", `<pe><c><e>x</e><f>2</f></c></pe>`, []xsderrors.Code{xsderrors.CodeValidationElement}},
		{"string attribute violation", `<ps><c a="x" b="y"/></ps>`, []xsderrors.Code{xsderrors.CodeValidationAttribute}},
		{"declared attribute satisfied duplicate", `<pa><c a="x" b="01"/></pa>`, []xsderrors.Code{xsderrors.CodeValidationIdentity}},
		{"wildcard attribute satisfied duplicate", `<pw><c a="x" b="01"/></pw>`, []xsderrors.Code{xsderrors.CodeValidationIdentity}},
		{"element field satisfied duplicate", `<pe><c><e>x</e><f>01</f></c></pe>`, []xsderrors.Code{xsderrors.CodeValidationIdentity}},
		{"string attribute satisfied duplicate", `<ps><c a="x" b="x"/></ps>`, []xsderrors.Code{xsderrors.CodeValidationIdentity}},
		{"declared attribute single node", `<pa><c b="1"/></pa>`, nil},
		{"wildcard attribute single node", `<pw><c b="1"/></pw>`, nil},
		{"element field single node", `<pe><c><f>1</f></c></pe>`, nil},
		{"string attribute single node", `<ps><c b="x"/></ps>`, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := engine.ValidateWithOptions(strings.NewReader(tt.doc), xsd.ValidateOptions{MaxErrors: 10})
			var got []xsderrors.Code
			if err != nil {
				for _, leaf := range xsderrors.Flatten(err) {
					x, ok := errors.AsType[*xsderrors.Error](leaf)
					if !ok {
						t.Fatalf("diagnostic %T %v is not structured", leaf, leaf)
					}
					got = append(got, x.Code())
				}
			}
			if len(got) != len(tt.want) {
				t.Fatalf("diagnostics = %v (%v), want %v", got, err, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("diagnostics = %v (%v), want %v", got, err, tt.want)
				}
			}
		})
	}
}
