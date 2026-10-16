package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestModelGroupDependencyBudgetPreservesReferenceLocation(t *testing.T) {
	t.Parallel()
	const schemaText = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="G"><xs:sequence><xs:element name="v" type="xs:string"/></xs:sequence></xs:group>
  <xs:complexType name="C">
    <xs:group ref="G" minOccurs="2" maxOccurs="3"/>
  </xs:complexType>
  <xs:element name="r" type="C"/>
</xs:schema>`
	const path = "model-group-work.xsd"
	input := xsd.Bytes(path, []byte(schemaText))
	var bounded *xsd.Engine
	located := false
	for budget := 1; budget <= 64; budget++ {
		engine, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDependencySteps: budget}, input)
		if err == nil {
			if engine == nil {
				t.Fatal("successful bounded compile returned no engine")
			}
			bounded = engine
			break
		}
		if engine != nil {
			t.Fatal("failed compile published an engine")
		}
		diagnostic, ok := errors.AsType[*xsderrors.Error](err)
		if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaLimit {
			t.Fatalf("bounded compile returned %v, want schema-limit failure", err)
		}
		if diagnostic.Path() == path && diagnostic.Line() == 4 && diagnostic.Column() == 5 {
			located = true
		}
	}
	if bounded == nil || !located {
		t.Fatalf("bounded compilation: engine=%v, reference-location failure=%v", bounded != nil, located)
	}
	defaultEngine, err := xsd.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, engine := range []*xsd.Engine{bounded, defaultEngine} {
		session, err := engine.NewSession(xsd.ValidateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct {
			document string
			valid    bool
		}{
			{"<r><v/><v/></r>", true},
			{"<r><v/><v/><v/></r>", true},
			{"<r><v/></r>", false},
			{"<r><v/><v/><v/><v/></r>", false},
		} {
			err := session.Validate(strings.NewReader(test.document))
			if test.valid {
				if err != nil {
					t.Fatalf("valid document %s: %v", test.document, err)
				}
			} else {
				if !xsderrors.IsConclusiveValidation(err) {
					t.Fatalf("invalid document %s: %v", test.document, err)
				}
				if err := session.Validate(strings.NewReader("<r><v/><v/></r>")); err != nil {
					t.Fatalf("validation after rejection: %v", err)
				}
			}
		}
	}
}
