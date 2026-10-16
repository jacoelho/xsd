package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestDeferredCompletionFailurePreservesSourceDiagnostic(t *testing.T) {
	t.Parallel()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="R" mixed="true">
    <xs:sequence>
      <xs:element name="child" minOccurs="0">
        <xs:complexType mixed="true">
          <xs:complexContent mixed="true">
            <xs:extension base="R">
              <xs:attribute name="a" type="xs:int" default="not-an-int"/>
            </xs:extension>
          </xs:complexContent>
        </xs:complexType>
      </xs:element>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="R"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("deferred-completion.xsd", []byte(schema)))
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if engine != nil || !ok {
		t.Fatalf("Invalid deferred completion: %v, %v, want nil engine and schema diagnostic", engine, err)
	}
	if diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaFacet || diagnostic.Message() != "invalid attribute default value for a" || diagnostic.Path() != "deferred-completion.xsd" || diagnostic.Line() != 8 || diagnostic.Column() != 15 {
		t.Fatalf("Invalid deferred completion: %v, want schema.facet at 8:15 for invalid attribute default", err)
	}
	corrected := strings.Replace(schema, `default="not-an-int"`, `default="1"`, 1)
	engine, err = xsd.Compile(xsd.Bytes("deferred-completion.xsd", []byte(corrected)))
	if err != nil || engine == nil {
		t.Fatalf("Corrected source: %v, %v", engine, err)
	}
	if err := engine.Validate(strings.NewReader(`<root><child/></root>`)); err != nil {
		t.Fatal(err)
	}
}
