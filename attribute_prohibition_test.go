package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestAttributeProhibitionDependencyBudget(t *testing.T) {
	t.Parallel()

	for _, shape := range []struct{ name, attribute string }{
		{"local", `<xs:attribute name="a" use="prohibited"/>`},
		{"reference", `<xs:attribute ref="missing" use="prohibited"/>`},
	} {
		t.Run(shape.name, func(t *testing.T) {
			t.Parallel()
			for _, test := range []struct {
				name         string
				count, limit int
				wantLimit    bool
			}{
				{"single", 1, 32, false},
				{"bounded", 64, 32, true},
				{"generous", 64, 4096, false},
			} {
				t.Run(test.name, func(t *testing.T) {
					t.Parallel()
					document := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:attributeGroup name="G">` +
						strings.Repeat(shape.attribute, test.count) + `</xs:attributeGroup></xs:schema>`
					engine, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDependencySteps: test.limit}, xsd.Bytes("prohibitions.xsd", []byte(document)))
					if test.wantLimit {
						if engine != nil {
							t.Fatal("failed compilation published an engine")
						}
						expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
						return
					}
					if err != nil || engine == nil {
						t.Fatalf("compilation: engine=%v, error=%v", engine != nil, err)
					}
				})
			}
		})
	}
}

func TestAttributeDirectiveDiagnosticPrecedence(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ name, attributes, message string }{
		{"mode before reference shape", `ref="missing" type="xs:string" use="bad"`, "invalid attribute use bad"},
		{"reference shape before prohibition constraint", `ref="missing" type="xs:string" use="prohibited" default="x"`, "attribute ref cannot have attribute type"},
		{"prohibition constraint", `ref="missing" use="prohibited" default="x"`, "prohibited attribute cannot have default"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			const path = "attribute-directives.xsd"
			document := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="C">
    <xs:attribute ` + test.attributes + `/>
  </xs:complexType>
</xs:schema>`
			engine, err := xsd.Compile(xsd.Bytes(path, []byte(document)))
			if engine != nil {
				t.Fatal("failed compilation published an engine")
			}
			expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaInvalidAttribute)
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Message() != test.message || diagnostic.Path() != path || diagnostic.Line() != 3 || diagnostic.Column() != 5 {
				t.Fatalf("diagnostic = %v, want %q at %s:3:5", err, test.message, path)
			}
		})
	}
}
