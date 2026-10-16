package xsd_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSimpleContentChildTraversalContracts(t *testing.T) {
	for _, test := range []struct {
		name    string
		source  string
		code    xsderrors.Code
		message string
		line    int
		column  int
	}{
		{
			name: "final restriction anchors first facet",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Locked" final="restriction"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:complexType name="Amount"><xs:simpleContent><xs:extension base="Locked"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Derived"><xs:simpleContent>
      <xs:restriction base="Amount">
        <xs:pattern value=".*"/>
        <xs:length value="1"/>
      </xs:restriction>
  </xs:simpleContent></xs:complexType>
</xs:schema>`,
			code:    xsderrors.CodeSchemaReference,
			message: "base simple type final blocks restriction",
			line:    6,
			column:  9,
		},
		{
			name: "anonymous failure precedes outer facet",
			source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Amount"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Derived">
    <xs:simpleContent>
      <xs:restriction base="Amount">
        <xs:simpleType>
          <xs:restriction base="xs:string">
          <xs:length value="1"/>
          <xs:length value="2"/>
          </xs:restriction>
        </xs:simpleType>
        <xs:maxLength value="-1"/>
      </xs:restriction>
    </xs:simpleContent>
  </xs:complexType>
</xs:schema>`,
			code:    xsderrors.CodeSchemaFacet,
			message: "duplicate length facet",
			line:    9,
			column:  11,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, err := xsd.Compile(xsd.Bytes("simple-content-children.xsd", []byte(test.source)))
			if engine != nil {
				t.Fatal("Compile() published an engine for an invalid restriction")
			}
			diagnostic, ok := err.(*xsderrors.Error) //nolint:errorlint // Wrapping changes the public diagnostic contract.
			if !ok {
				t.Fatalf("Compile() error = %T %v, want direct structured diagnostic", err, err)
			}
			if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != test.code || diagnostic.Cause() != nil {
				t.Fatalf("diagnostic = (%s, %s, %v), want schema_compile, %s, nil cause", diagnostic.Category(), diagnostic.Code(), diagnostic.Cause(), test.code)
			}
			if diagnostic.Message() != test.message {
				t.Fatalf("Message() = %q, want %q", diagnostic.Message(), test.message)
			}
			if diagnostic.Path() != "simple-content-children.xsd" || diagnostic.Line() != test.line || diagnostic.Column() != test.column {
				t.Fatalf("location = %s:%d:%d, want simple-content-children.xsd:%d:%d", diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), test.line, test.column)
			}
		})
	}
	t.Run("no facet reuses final base", func(t *testing.T) {
		const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Locked" final="restriction"><xs:restriction base="xs:string"/></xs:simpleType>
  <xs:complexType name="Amount"><xs:simpleContent><xs:extension base="Locked"/></xs:simpleContent></xs:complexType>
  <xs:complexType name="Derived"><xs:simpleContent><xs:restriction base="Amount"/></xs:simpleContent></xs:complexType>
  <xs:element name="r" type="Derived"/>
</xs:schema>`
		engine, err := xsd.Compile(xsd.Bytes("simple-content-children.xsd", []byte(source)))
		if err != nil {
			t.Fatalf("Compile(): %v", err)
		}
		if err := engine.Validate(strings.NewReader("<r>anything</r>")); err != nil {
			t.Fatalf("Validate(): %v", err)
		}
	})
}
