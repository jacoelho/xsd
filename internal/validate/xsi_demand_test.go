package validate

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestXSIIdentitySelectionActivationAndReuse(t *testing.T) {
	t.Parallel()
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
<xs:element name="root"><xs:complexType><xs:choice minOccurs="0" maxOccurs="unbounded">
<xs:element name="row" type="xs:string" nillable="true"/>
<xs:element name="other" type="xs:string" nillable="true"/>
</xs:choice></xs:complexType><xs:unique name="nils"><xs:selector xpath="row"/><xs:field xpath="@xsi:nil"/></xs:unique></xs:element></xs:schema>`)
	session, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	const valid = `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
<other xsi:nil="false"/>
<row xsi:nil="false"/>
<row xsi:nil="true"/>
<other xsi:nil="false"/>
</root>`
	duplicate := strings.Replace(valid, `<row xsi:nil="true"/>`, `<row xsi:nil="0"/>`, 1)
	for _, doc := range []string{valid, duplicate, valid, duplicate, valid} {
		validationErr := session.Validate(strings.NewReader(doc))
		if doc == duplicate {
			expectXSDCode(t, validationErr, xsderrors.CodeValidationIdentity)
		} else if validationErr != nil {
			t.Fatalf("Validate(valid) = %v", validationErr)
		}
	}
}

func TestXSIAttributesPreserveIDREFOnlyDocumentIdentity(t *testing.T) {
	t.Parallel()
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
<xs:element name="root"><xs:complexType><xs:sequence><xs:element name="item" type="xs:string" nillable="true"/></xs:sequence>
<xs:attribute name="id" type="xs:ID"/><xs:attribute name="ref" type="xs:IDREF"/>
</xs:complexType></xs:element></xs:schema>`)
	session, err := newSessionForTest(rt, Options{})
	if err != nil {
		t.Fatal(err)
	}
	const valid = `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" id="target" ref="target"><item xsi:nil="true" xsi:noNamespaceSchemaLocation="fixture.xsd"/></root>`
	unresolved := strings.Replace(valid, `ref="target"`, `ref="missing"`, 1)
	for _, doc := range []string{unresolved, valid, unresolved, valid} {
		validationErr := session.Validate(strings.NewReader(doc))
		if doc == unresolved {
			expectXSDCode(t, validationErr, xsderrors.CodeValidationType)
		} else if validationErr != nil {
			t.Fatalf("Validate(valid) = %v", validationErr)
		}
	}
}
