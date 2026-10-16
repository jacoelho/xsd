package xsd_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestIdentityCompilationDiagnosticPrecedence(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, source, message string
		code                  xsderrors.Code
		line, column          int
	}{
		{"grammar", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:key name="key">
      <xs:selector xpath="["/>
      <xs:field xpath=""/>
    </xs:key>
  </xs:element>
</xs:schema>`, "field xpath is empty", xsderrors.CodeSchemaIdentity, 5, 7},
		{"reference", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:keyref name="reference" refer="unknown">
      <xs:selector xpath="["/>
      <xs:field xpath="@id"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`, "unknown keyref refer unknown", xsderrors.CodeSchemaReference, 3, 5},
		{"field", `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:key name="key">
      <xs:selector xpath="item"/>
      <xs:field xpath="@"/>
    </xs:key>
  </xs:element>
</xs:schema>`, "invalid identity field XPath @", xsderrors.CodeSchemaIdentity, 5, 7},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			engine, err := xsd.Compile(xsd.Bytes(test.name+".xsd", []byte(test.source)))
			if engine != nil {
				t.Fatal("failed compilation returned an engine")
			}
			expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, test.code)
			if diagnostics := xsderrors.Flatten(err); len(diagnostics) != 1 {
				t.Fatalf("diagnostics = %v; want one", diagnostics)
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic == nil {
				t.Fatalf("missing structured diagnostic: %v", err)
			}
			if diagnostic.Message() != test.message || diagnostic.Path() != test.name+".xsd" ||
				diagnostic.Line() != test.line || diagnostic.Column() != test.column || diagnostic.Cause() != nil {
				t.Fatalf("diagnostic = %v; want %q at %s.xsd:%d:%d with nil cause", diagnostic, test.message, test.name, test.line, test.column)
			}
		})
	}
}
