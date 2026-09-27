package schema_test

import (
	"errors"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/validate"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestEmptyComplexContentRejectsCharacterInformationItems(t *testing.T) {
	t.Parallel()

	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType/></xs:element>
</xs:schema>`)
	for _, doc := range []string{
		"<root/>",
		"<root><!-- comment --></root>",
		"<root><?instruction?></root>",
		"<root><![CDATA[]]></root>",
	} {
		mustValidate(t, engine, doc)
	}
	for _, doc := range []string{
		"<root> </root>",
		"<root>\t</root>",
		"<root>\n</root>",
		"<root>&#x20;</root>",
		"<root><![CDATA[ ]]></root>",
	} {
		mustNotValidate(t, engine, doc, xsderrors.CodeValidationText)
	}
}

func TestComplexContentVarietyDistinguishesEmptyFromEmptiableModels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		schema     string
		valid      string
		bad        string
		code       xsderrors.Code
		unexpected string
		recovered  string
		malformed  string
		accepted   string
	}{
		{
			name: "direct empty model rejects child",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType/></xs:element>
</xs:schema>`,
			valid:      "<root/>",
			bad:        "<root> </root>",
			code:       xsderrors.CodeValidationText,
			unexpected: "<root><child/></root>",
			recovered:  "<root/>",
			malformed:  "<root><child></root>",
		},
		{
			name: "optional particle remains element only",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="child" minOccurs="0"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`,
			valid:    "<root> </root>",
			bad:      "<root>text</root>",
			code:     xsderrors.CodeValidationText,
			accepted: "<root><child/></root>",
		},
		{
			name: "nested empty particle remains element only",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence><xs:sequence/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`,
			valid: "<root> </root>",
			bad:   "<root>text</root>",
			code:  xsderrors.CodeValidationText,
		},
		{
			name: "zero top level particle is empty",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence minOccurs="0" maxOccurs="0">
    <xs:element name="child"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`,
			valid:      "<root/>",
			bad:        "<root> </root>",
			code:       xsderrors.CodeValidationText,
			unexpected: "<root><child/></root>",
			recovered:  "<root/>",
			malformed:  "<root><child></root>",
		},
		{
			name: "optional empty choice is empty",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:choice minOccurs="0"/></xs:complexType></xs:element>
</xs:schema>`,
			valid:      "<root/>",
			bad:        "<root> </root>",
			code:       xsderrors.CodeValidationText,
			unexpected: "<root><child/></root>",
			recovered:  "<root/>",
			malformed:  "<root><child></root>",
		},
		{
			name: "annotation-only sequence is empty",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:annotation><xs:documentation>metadata</xs:documentation></xs:annotation>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`,
			valid:      "<root/>",
			bad:        "<root> </root>",
			code:       xsderrors.CodeValidationText,
			unexpected: "<root><child/></root>",
			recovered:  "<root/>",
			malformed:  "<root><child></root>",
		},
		{
			name: "annotation-only all is empty",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:all>
    <xs:annotation><xs:documentation>metadata</xs:documentation></xs:annotation>
  </xs:all></xs:complexType></xs:element>
</xs:schema>`,
			valid: "<root/>",
			bad:   "<root> </root>",
			code:  xsderrors.CodeValidationText,
		},
		{
			name: "annotation-only optional choice is empty",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:choice minOccurs="0">
    <xs:annotation><xs:documentation>metadata</xs:documentation></xs:annotation>
  </xs:choice></xs:complexType></xs:element>
</xs:schema>`,
			valid: "<root/>",
			bad:   "<root> </root>",
			code:  xsderrors.CodeValidationText,
		},
		{
			name: "mixed empty model rejects child",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType mixed="true"/></xs:element>
</xs:schema>`,
			valid:      "<root>text</root>",
			bad:        "<root><child/></root>",
			code:       xsderrors.CodeValidationElement,
			unexpected: "<root><child/></root>",
			recovered:  "<root>text</root>",
			malformed:  "<root><child></root>",
		},
		{
			name: "named empty group reference is a present particle",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="empty"><xs:sequence/></xs:group>
  <xs:element name="root"><xs:complexType><xs:group ref="empty"/></xs:complexType></xs:element>
</xs:schema>`,
			valid: "<root> </root>",
			bad:   "<root>text</root>",
			code:  xsderrors.CodeValidationText,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			engine := mustCompile(t, tt.schema)
			mustValidate(t, engine, tt.valid)
			mustNotValidate(t, engine, tt.bad, tt.code)
			if tt.accepted != "" {
				mustValidate(t, engine, tt.accepted)
			}
			if tt.unexpected == "" {
				return
			}
			session, err := validate.NewSessionPool(engine).NewSession(validate.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if validationErr := session.Validate(strings.NewReader(tt.unexpected)); validationErr == nil {
				t.Fatalf("unexpected child %q was accepted", tt.unexpected)
			} else {
				expectCode(t, validationErr, xsderrors.CodeValidationElement)
			}
			if validationErr := session.Validate(strings.NewReader(tt.recovered)); validationErr != nil {
				t.Fatalf("session reuse after unexpected child failed: %v", validationErr)
			}
			fatalSession, err := validate.NewSessionPool(engine).NewSession(validate.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if err := fatalSession.Validate(strings.NewReader(tt.malformed)); err == nil {
				t.Fatalf("malformed nested XML %q was accepted", tt.malformed)
			} else {
				expectCode(t, err, xsderrors.CodeValidationXML)
			}
		})
	}
	requiredChoice := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:choice/></xs:complexType></xs:element>
</xs:schema>`)
	mustNotValidate(t, requiredChoice, "<root/>", xsderrors.CodeValidationContent)
	mustNotValidate(t, requiredChoice, "<root> </root>", xsderrors.CodeValidationContent)
	annotationRequiredChoice := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:choice>
    <xs:annotation><xs:documentation>metadata</xs:documentation></xs:annotation>
  </xs:choice></xs:complexType></xs:element>
</xs:schema>`)
	mustNotValidate(t, annotationRequiredChoice, "<root/>", xsderrors.CodeValidationContent)
	mustNotValidate(t, annotationRequiredChoice, "<root> </root>", xsderrors.CodeValidationContent)

	// mixed=true supplies the XSD synthetic empty sequence for an
	// annotation-only optional choice. This must remain mixed rather than
	// becoming empty through the annotation filter.
	mixedAnnotation := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType mixed="true"><xs:choice minOccurs="0">
    <xs:annotation><xs:documentation>metadata</xs:documentation></xs:annotation>
  </xs:choice></xs:complexType></xs:element>
</xs:schema>`)
	mustValidate(t, mixedAnnotation, "<root>text</root>")
	mustValidate(t, mixedAnnotation, "<root> </root>")

	mixedRequiredAnnotation := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType mixed="true"><xs:choice>
    <xs:annotation><xs:documentation>metadata</xs:documentation></xs:annotation>
  </xs:choice></xs:complexType></xs:element>
</xs:schema>`)
	mustNotValidate(t, mixedRequiredAnnotation, "<root/>", xsderrors.CodeValidationContent)
	mustNotValidate(t, mixedRequiredAnnotation, "<root>text</root>", xsderrors.CodeValidationContent)
}

func TestComplexContentEmptyKindFlowsThroughDerivationAndXSIType(t *testing.T) {
	t.Parallel()

	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:complexType name="Elements"><xs:sequence><xs:element name="child" minOccurs="0"/></xs:sequence></xs:complexType>
	  <xs:complexType name="Empty"><xs:complexContent><xs:restriction base="Elements"/></xs:complexContent></xs:complexType>
  <xs:complexType name="EmptyExtension"><xs:complexContent><xs:extension base="Empty"/></xs:complexContent></xs:complexType>
  <xs:complexType name="EmptyRestriction"><xs:complexContent><xs:restriction base="Elements"/></xs:complexContent></xs:complexType>
  <xs:complexType name="MixedExtension"><xs:complexContent mixed="true"><xs:extension base="Empty"/></xs:complexContent></xs:complexType>
  <xs:element name="root" type="Elements"/>
  <xs:element name="emptyExtension" type="EmptyExtension"/>
  <xs:element name="emptyRestriction" type="EmptyRestriction"/>
  <xs:element name="mixedExtension" type="MixedExtension"/>
</xs:schema>`)
	mustNotValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Empty"> </root>`, xsderrors.CodeValidationText)
	mustValidate(t, engine, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="Elements"> </root>`)
	mustNotValidate(t, engine, `<emptyExtension> </emptyExtension>`, xsderrors.CodeValidationText)
	mustNotValidate(t, engine, `<emptyRestriction> </emptyRestriction>`, xsderrors.CodeValidationText)
	mustValidate(t, engine, `<mixedExtension>text</mixedExtension>`)

	// A mixed extension may start from true empty content, but the same
	// derivation from element-only content that merely normalizes to an empty
	// model remains disallowed.
	_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="ElementOnly"><xs:sequence><xs:element name="child" minOccurs="0"/></xs:sequence></xs:complexType>
  <xs:complexType name="Bad"><xs:complexContent mixed="true"><xs:extension base="ElementOnly"/></xs:complexContent></xs:complexType>
</xs:schema>`))})
	if err == nil {
		t.Fatal("mixed extension from normalized element-only content compiled")
	}
	expectCode(t, err, xsderrors.CodeSchemaContentModel)
}

func TestEmptyContentPreservesNilledAndSkippedPolicies(t *testing.T) {
	t.Parallel()

	nilled := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" nillable="true"><xs:complexType/></xs:element>
</xs:schema>`)
	mustValidate(t, nilled, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>`)
	mustNotValidate(t, nilled, `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"> </root>`, xsderrors.CodeValidationNil)

	skipped := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:o="urn:other">
  <xs:element name="root"><xs:complexType><xs:sequence><xs:any namespace="##other" processContents="skip"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	mustValidate(t, skipped, `<root xmlns:o="urn:other"><o:child> </o:child></root>`)
}

func TestEmptyContentRejectsWhitespaceAcrossReaderChunksAndAllowsReuse(t *testing.T) {
	t.Parallel()

	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType/></xs:element>
</xs:schema>`)
	session, err := validate.NewSessionPool(engine).NewSession(validate.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Validate(oneByteReader{r: strings.NewReader("<root> </root>")}); err == nil {
		t.Fatal("chunked empty-content whitespace was accepted")
	} else {
		var diagnostic *xsderrors.Error
		if !errors.As(err, &diagnostic) || diagnostic.Code() != xsderrors.CodeValidationText {
			t.Fatalf("chunked validation error = %v, want validation.text", err)
		}
	}
	if err := session.Validate(strings.NewReader("<root/>")); err != nil {
		t.Fatalf("session reuse after empty-content rejection failed: %v", err)
	}
}

type oneByteReader struct {
	r *strings.Reader
}

func (r oneByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return r.r.Read(p[:1])
}
