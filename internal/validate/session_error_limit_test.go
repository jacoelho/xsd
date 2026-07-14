package validate

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestMaxErrorsCapsCollectionWithoutSkippingXMLSyntax(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="v" type="xs:int" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	t.Run("caps collected validation errors", func(t *testing.T) {
		t.Parallel()

		err := Validate(rt, strings.NewReader(`<root><v>x</v><v>y</v></root>`), Options{MaxErrors: 1})
		requireCode(t, err, xsderrors.CodeValidationFacet)
		if multiple, ok := errors.AsType[xsderrors.Errors](err); ok {
			t.Fatalf("Validate() returned %d errors, want one", len(multiple))
		}
	})

	t.Run("reports later XML syntax error", func(t *testing.T) {
		t.Parallel()

		err := Validate(rt, strings.NewReader(`<root><v>x</v><v>1</root>`), Options{MaxErrors: 1})
		requireCode(t, err, xsderrors.CodeValidationXML)
	})

	t.Run("reports XML syntax error at EOF", func(t *testing.T) {
		t.Parallel()

		err := Validate(rt, strings.NewReader(`<root><v>x</v>`), Options{MaxErrors: 1})
		requireCode(t, err, xsderrors.CodeValidationXML)
	})

	t.Run("reports character data after root", func(t *testing.T) {
		t.Parallel()

		err := Validate(rt, strings.NewReader(`<root><v>x</v></root>tail`), Options{MaxErrors: 1})
		requireCode(t, err, xsderrors.CodeValidationText)
	})
}

func TestMaxErrorsStopsSemanticValidationTail(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:ID" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)

	err := Validate(
		rt,
		strings.NewReader(`<root><bad/><item>a</item><item>b</item></root>`),
		Options{MaxErrors: 1, MaxIdentityEntries: 1},
	)
	requireCode(t, err, xsderrors.CodeValidationElement)
}

func TestMaxErrorsStopsSemanticValidationWithinTriggeringToken(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="child">
        <xs:complexType><xs:sequence>
          <xs:element name="row" minOccurs="0" maxOccurs="unbounded">
            <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
          </xs:element>
        </xs:sequence></xs:complexType>
        <xs:key name="childKey"><xs:selector xpath="row"/><xs:field xpath="@id"/></xs:key>
      </xs:element>
    </xs:sequence></xs:complexType>
    <xs:key name="rootKey"><xs:selector xpath="child"/><xs:field xpath="@missing"/></xs:key>
  </xs:element>
</xs:schema>`)
	doc := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><child xsi:schemaLocation="odd"/></root>`
	err := Validate(
		rt,
		strings.NewReader(doc),
		Options{MaxErrors: 1, MaxIdentityScopes: 1},
	)
	requireCode(t, err, xsderrors.CodeValidationAttribute)
}

func TestNilledChildConsumesOneErrorSlot(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="n" nillable="true">
        <xs:complexType><xs:sequence><xs:element name="child"/></xs:sequence></xs:complexType>
      </xs:element>
      <xs:element name="v" type="xs:int"/>
    </xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)
	doc := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><n xsi:nil="true"><child/></n><v>x</v></root>`
	err := Validate(rt, strings.NewReader(doc), Options{MaxErrors: 2})
	multiple, ok := errors.AsType[xsderrors.Errors](err)
	if !ok || len(multiple) != 2 {
		t.Fatalf("Validate() error = %v, want two validation errors", err)
	}
	want := []xsderrors.Code{xsderrors.CodeValidationNil, xsderrors.CodeValidationFacet}
	for i, item := range multiple {
		xerr, ok := errors.AsType[*xsderrors.Error](item)
		if !ok || xerr.Code != want[i] {
			t.Fatalf("Validate() error %d = %v, want code %s", i, item, want[i])
		}
	}

	err = Validate(
		rt,
		strings.NewReader(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><n xsi:nil="true">text</n><v>1</v></root>`),
		Options{},
	)
	requireCode(t, err, xsderrors.CodeValidationNil)
}

func TestSessionReuseClearsSyntaxOnlyMode(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="v" type="xs:int" maxOccurs="unbounded"/>
    </xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)
	session, err := NewSession(rt, Options{MaxErrors: 1})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}

	err = session.Validate(strings.NewReader(`<root><v>x</v><v>y</v></root>`))
	requireCode(t, err, xsderrors.CodeValidationFacet)
	err = session.Validate(strings.NewReader(`<root><v>z</v></root>`))
	requireCode(t, err, xsderrors.CodeValidationFacet)
}
