package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestSchemaSimpleContentCycleContract(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, source, message string }{
		{name: "simple-content-self", source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:f="urn:foreign">
  <xs:complexType name="C">
    <xs:simpleContent><xs:extension base="C"/></xs:simpleContent>
  </xs:complexType>
  <xs:element name="r" type="C"/>
</xs:schema>`, message: "cyclic complex type C"},
		{name: "simple-content-mutual", source: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:f="urn:foreign">
  <xs:complexType name="C">
    <xs:simpleContent><xs:extension base="B"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="B">
    <xs:simpleContent><xs:extension base="C"/></xs:simpleContent>
  </xs:complexType>
  <xs:element name="r" type="C"/>
</xs:schema>`, message: "cyclic complex type B"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := tc.name + ".xsd"
			engine, err := xsd.Compile(xsd.Bytes(path, []byte(tc.source)))
			if engine != nil {
				t.Fatal("cyclic source published a schema")
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("Compile() error = %v, want structured cycle diagnostic", err)
			}
			if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaReference || diagnostic.Message() != tc.message || diagnostic.Path() != path || diagnostic.Line() != 3 || diagnostic.Column() != 23 || diagnostic.Cause() != nil {
				t.Fatalf("Compile() diagnostic = %s/%s %q at %s:%d:%d cause=%v, want schema compile/reference %q at %s:3:23 with no cause", diagnostic.Category(), diagnostic.Code(), diagnostic.Message(), diagnostic.Path(), diagnostic.Line(), diagnostic.Column(), diagnostic.Cause(), tc.message, path)
			}
		})
	}
	t.Run("forward-reference", func(t *testing.T) {
		t.Parallel()
		const source = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:f="urn:foreign">
  <xs:complexType name="C">
    <xs:simpleContent><xs:extension base="B"/></xs:simpleContent>
  </xs:complexType>
  <xs:complexType name="B">
    <xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent>
  </xs:complexType>
  <xs:element name="r" type="C"/>
</xs:schema>`
		engine, err := xsd.Compile(xsd.Bytes("simple-content-forward.xsd", []byte(source)))
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
		if err = session.Validate(strings.NewReader("<r>text</r>")); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})
}
