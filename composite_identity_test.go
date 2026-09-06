package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCompositeSimpleTypesPreserveDocumentIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, definition, item, valid, invalid string
	}{
		{
			name:       "element union ID",
			definition: `<xs:union memberTypes="xs:int xs:ID"/>`,
			item:       `<xs:element name="item" type="value" maxOccurs="unbounded"/>`,
			valid:      `<root><item>7</item><item>7</item><item>first</item><item>second</item></root>`,
			invalid:    `<root><item>same</item><item>same</item></root>`,
		},
		{
			name:       "attribute union ID",
			definition: `<xs:union memberTypes="xs:int xs:ID"/>`,
			item:       `<xs:element name="item" maxOccurs="unbounded"><xs:complexType><xs:attribute name="v" type="value"/></xs:complexType></xs:element>`,
			valid:      `<root><item v="7"/><item v="+7"/><item v="first"/><item v="second"/></root>`,
			invalid:    `<root><item v="same"/><item v="same"/></root>`,
		},
		{
			name:       "element union IDREF",
			definition: `<xs:union memberTypes="xs:int xs:IDREF"/>`,
			item:       `<xs:element name="item" type="value" maxOccurs="unbounded"/>`,
			valid:      `<root known="target"><item>7</item><item>target</item></root>`,
			invalid:    `<root><item>missing</item></root>`,
		},
		{
			name:       "attribute union IDREF",
			definition: `<xs:union memberTypes="xs:int xs:IDREF"/>`,
			item:       `<xs:element name="item" maxOccurs="unbounded"><xs:complexType><xs:attribute name="v" type="value"/></xs:complexType></xs:element>`,
			valid:      `<root known="target"><item v="7"/><item v="target"/></root>`,
			invalid:    `<root><item v="missing"/></root>`,
		},
		{
			name:       "element list of union IDREF",
			definition: `<xs:list><xs:simpleType><xs:union memberTypes="xs:int xs:IDREF"/></xs:simpleType></xs:list>`,
			item:       `<xs:element name="item" type="value" maxOccurs="unbounded"/>`,
			valid:      `<root known="target"><item>7 target 8</item></root>`,
			invalid:    `<root><item>missing 7</item></root>`,
		},
		{
			name:       "attribute list of union IDREF",
			definition: `<xs:list><xs:simpleType><xs:union memberTypes="xs:int xs:IDREF"/></xs:simpleType></xs:list>`,
			item:       `<xs:element name="item" maxOccurs="unbounded"><xs:complexType><xs:attribute name="v" type="value"/></xs:complexType></xs:element>`,
			valid:      `<root known="target"><item v="7 target 8"/></root>`,
			invalid:    `<root><item v="missing 7"/></root>`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="value">` + tc.definition + `</xs:simpleType><xs:element name="root"><xs:complexType><xs:sequence>` + tc.item + `</xs:sequence><xs:attribute name="known" type="xs:ID"/></xs:complexType></xs:element></xs:schema>`
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
			if err != nil {
				t.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if validateErr := session.Validate(strings.NewReader(tc.valid)); validateErr != nil {
				t.Fatalf("valid composite identity value: %v", validateErr)
			}
			err = session.Validate(strings.NewReader(tc.invalid))
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationType)
			if validateErr := session.Validate(strings.NewReader(tc.valid)); validateErr != nil {
				t.Fatalf("reuse after identity failure: %v", validateErr)
			}
		})
	}
}
