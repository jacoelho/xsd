package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestXMLDocumentRejectsNonliteralOuterWhitespace(t *testing.T) {
	t.Parallel()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{`<![CDATA[]]>`, `<![CDATA[ ]]>`, `&#32;`, `&#x20;`, " \t&#9;\n"} {
		for _, position := range []string{"before", "after"} {
			t.Run(position+text, func(t *testing.T) {
				t.Parallel()
				wrap := func(document string) string {
					if position == "before" {
						return text + document
					}
					return document + text
				}
				_, err := xsd.Compile(xsd.Bytes("malformed.xsd", []byte(wrap(schema))))
				diagnostic, ok := errors.AsType[*xsderrors.Error](err)
				if !ok || diagnostic.Code() != xsderrors.CodeSchemaXML || diagnostic.Path() != "malformed.xsd" || diagnostic.Line() == 0 || diagnostic.Column() == 0 {
					t.Fatalf("Compile() = %v, want located schema.xml diagnostic", err)
				}
				session, err := engine.NewSession(xsd.ValidateOptions{})
				if err != nil {
					t.Fatal(err)
				}
				err = session.Validate(strings.NewReader(wrap(`<root/>`)))
				expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)
				if err := session.Validate(strings.NewReader(" \r\n<root>&#32;<![CDATA[ ]]></root>\t")); err != nil {
					t.Fatalf("Session.Validate() after malformed document = %v", err)
				}
			})
		}
	}
}

func TestCompileAllowsLiteralOuterWhitespaceAndInnerCharacterData(t *testing.T) {
	t.Parallel()
	const schema = " \r\n" + `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
<![CDATA[ ]]>&#32;<xs:annotation><xs:documentation><![CDATA[annotation]]>&#32;</xs:documentation></xs:annotation>
<xs:element name="root" type="xs:string"/></xs:schema>` + "\t\n"
	if _, err := xsd.Compile(xsd.Bytes("valid.xsd", []byte(schema))); err != nil {
		t.Fatalf("Compile() = %v", err)
	}
}

func TestOuterReferenceOverridesSemanticErrorAndSessionRecovers(t *testing.T) {
	t.Parallel()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:int"/></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: 1})
	if err != nil {
		t.Fatal(err)
	}
	err = session.Validate(strings.NewReader(`<root>bad</root>&#32;`))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)
	if err := session.Validate(strings.NewReader(`<root>7</root>`)); err != nil {
		t.Fatalf("Session.Validate() after syntax-only failure = %v", err)
	}
}
