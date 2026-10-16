package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const schemaNodeAdmissionHeader = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:f="urn:foreign">`

func TestSchemaNodeAdmissionPreservesNamespaceAndGrammarPriority(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		body     string
		message  string
		location [2]int
	}{
		{"unknown-model", `  <xs:complexType name="C">
    <xs:sequence>
      <xs:future/>
      <xs:attribute/>
    </xs:sequence>
  </xs:complexType>`, "invalid model group child future", [2]int{4, 7}},
		{"foreign-restriction", `  <xs:simpleType name="T">
    <f:restriction base="a:b:c"/>
  </xs:simpleType>`, "foreign element restriction is not allowed in schema grammar", [2]int{3, 5}},
		{"foreign-notation", `  <xs:notation name="N" public="n">
    <f:annotation/>
  </xs:notation>`, "foreign element annotation is not allowed in schema grammar", [2]int{3, 5}},
		{"nonopaque-documentation", `  <xs:element name="r">
    <xs:documentation>
      <f:element/>
    </xs:documentation>
  </xs:element>`, "foreign element element is not allowed in schema grammar", [2]int{4, 7}}, //nolint:dupword // Preserve the exact parser diagnostic for the element vocabulary name.
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := tc.name + ".xsd"
			source := schemaNodeAdmissionHeader + "\n" + tc.body + "\n</xs:schema>"
			engine, err := xsd.Compile(xsd.Bytes(path, []byte(source)))
			if engine != nil {
				t.Fatal("invalid source published a schema")
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("Compile() error = %v, want structured diagnostic", err)
			}
			if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaContentModel || diagnostic.Message() != tc.message || diagnostic.Path() != path || diagnostic.Line() != tc.location[0] || diagnostic.Column() != tc.location[1] || diagnostic.Cause() != nil {
				t.Fatalf("Compile() diagnostic = %s/%s %q at %s:%d:%d cause=%v, want schema compile/content model %q at %s:%d:%d with no cause",
					diagnostic.Category(), diagnostic.Code(), diagnostic.Message(), diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), diagnostic.Cause(), tc.message, path, tc.location[0], tc.location[1])
			}
		})
	}
}

func TestSchemaNodeAdmissionAcceptsOpaqueVocabulary(t *testing.T) {
	t.Parallel()
	const body = `  <xs:annotation>
    <xs:appinfo><f:restriction base="a:b:c"/><f:annotation/><f:element/><xs:future/></xs:appinfo>
    <xs:documentation><f:restriction base="a:b:c"/><f:annotation/><f:element/><xs:future/></xs:documentation>
  </xs:annotation>
  <xs:element name="r" type="xs:string"/>`
	source := schemaNodeAdmissionHeader + "\n" + body + "\n</xs:schema>"
	engine, err := xsd.Compile(xsd.Bytes("opaque-opponent.xsd", []byte(source)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if engine == nil {
		t.Fatal("Compile() returned nil engine")
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if session == nil {
		t.Fatal("NewSession() returned nil session")
	}
	if err = session.Validate(strings.NewReader("<r/>")); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
