package xsd_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCompiledSchemaLocationHintsDoNotSupplyComponents(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="T"><xs:restriction base="xs:decimal"/></xs:simpleType>
  <xs:element name="number" type="T"/>
  <xs:element name="strict"><xs:complexType><xs:sequence>
    <xs:any minOccurs="0" maxOccurs="unbounded"/>
  </xs:sequence><xs:anyAttribute/></xs:complexType></xs:element>
  <xs:element name="lax"><xs:complexType><xs:sequence>
    <xs:any minOccurs="0" maxOccurs="unbounded" processContents="lax"/>
  </xs:sequence><xs:anyAttribute processContents="lax"/></xs:complexType></xs:element>
  <xs:element name="skip"><xs:complexType><xs:sequence>
    <xs:any minOccurs="0" maxOccurs="unbounded" processContents="skip"/>
  </xs:sequence><xs:anyAttribute processContents="skip"/></xs:complexType></xs:element>
  <xs:element name="errors"><xs:complexType><xs:sequence>
    <xs:element name="n" type="xs:decimal"/><xs:any/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("compiled.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	// An available hinted document must not add components to this engine.
	hinted := filepath.Join(t.TempDir(), "additional.xsd")
	writeTestFile(t, hinted, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="missing" type="xs:string"/></xs:schema>`)
	const namespaces = ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:p="urn:missing" `
	hints := fmt.Sprintf(`xsi:noNamespaceSchemaLocation="%s" xsi:schemaLocation="urn:missing %s"`, hinted, hinted)
	for _, test := range []struct {
		name, doc string
		codes     []xsderrors.Code
	}{
		{name: "missing root", doc: `<missing %s/>`, codes: []xsderrors.Code{xsderrors.CodeValidationRoot}},
		{name: "missing namespaced root", doc: `<p:missing %s/>`, codes: []xsderrors.Code{xsderrors.CodeValidationRoot}},
		{name: "missing type", doc: `<number %s xsi:type="p:Missing">1</number>`, codes: []xsderrors.Code{xsderrors.CodeValidationType}},
		{name: "strict missing child", doc: `<strict %s><missing/></strict>`, codes: []xsderrors.Code{xsderrors.CodeValidationElement}},
		{name: "strict missing attribute", doc: `<strict %s extra="1"/>`, codes: []xsderrors.Code{xsderrors.CodeValidationAttribute}},
		{name: "known valid value", doc: `<number %s>1</number>`},
		{name: "known invalid value", doc: `<number %s>bad</number>`, codes: []xsderrors.Code{xsderrors.CodeValidationFacet}},
		{name: "provided root type", doc: `<missing %s xsi:type="T">1</missing>`},
		{name: "provided strict child type", doc: `<strict %s><missing xsi:type="T">1</missing></strict>`},
		{name: "provided strict child invalid value", doc: `<strict %s><missing xsi:type="T">bad</missing></strict>`, codes: []xsderrors.Code{xsderrors.CodeValidationFacet}},
		{name: "missing strict child type", doc: `<strict %s><missing xsi:type="p:Missing"/></strict>`, codes: []xsderrors.Code{xsderrors.CodeValidationType}},
		{name: "lax missing components", doc: `<lax %s extra="1"><missing/></lax>`},
		{name: "skip missing components", doc: `<skip %s extra="1"><missing/></skip>`},
		{name: "successive semantic errors", doc: `<errors %s><n>bad</n><missing/></errors>`, codes: []xsderrors.Code{xsderrors.CodeValidationFacet, xsderrors.CodeValidationElement}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, maxErrors := range []int{0, 1, 2} {
				session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: maxErrors})
				if err != nil {
					t.Fatal(err)
				}
				want := test.codes
				if maxErrors > 0 && len(want) > maxErrors {
					want = want[:maxErrors]
				}
				for _, attributes := range []string{"", hints} {
					doc := fmt.Sprintf(test.doc, namespaces+attributes)
					err := session.Validate(strings.NewReader(doc))
					if len(want) == 0 {
						if err != nil {
							t.Fatalf("Validate(maxErrors=%d, hints=%t): %v", maxErrors, attributes != "", err)
						}
					} else {
						if xsderrors.IsUnsupported(err) || !xsderrors.IsConclusiveValidation(err) {
							t.Fatalf("Validate(maxErrors=%d, hints=%t) = %v, want conclusive validation errors", maxErrors, attributes != "", err)
						}
						var codes []xsderrors.Code
						for _, leaf := range xsderrors.Flatten(err) {
							diagnostic, ok := errors.AsType[*xsderrors.Error](leaf)
							if !ok || diagnostic.Category() != xsderrors.CategoryValidation {
								t.Fatalf("unexpected diagnostic %T: %v", leaf, leaf)
							}
							codes = append(codes, diagnostic.Code())
						}
						if !slices.Equal(codes, want) {
							t.Fatalf("Validate(maxErrors=%d, hints=%t) codes=%v, want %v", maxErrors, attributes != "", codes, want)
						}
					}
					if err := session.Validate(strings.NewReader(`<number>2</number>`)); err != nil {
						t.Fatalf("Validate after prior assessment: %v", err)
					}
				}
			}
		})
	}
}
