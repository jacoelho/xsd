package xsd_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestDerivationBudgetExhaustionKeepsSchemaLimitDiagnostic(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ name, schema string }{
		{name: "restriction", schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:attribute name="a" type="xs:string"/></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base">
    <xs:attribute name="a" type="xs:token"/>
  </xs:restriction></xs:complexContent></xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`},
		{name: "extension after restriction", schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A"><xs:attribute name="a" type="xs:int"/></xs:complexType>
  <xs:complexType name="R"><xs:complexContent><xs:restriction base="A"><xs:attribute name="a" use="prohibited"/></xs:restriction></xs:complexContent></xs:complexType>
  <xs:complexType name="E"><xs:complexContent><xs:extension base="R"><xs:attribute name="a" type="xs:int"/></xs:extension></xs:complexContent></xs:complexType>
  <xs:element name="root" type="E"/>
</xs:schema>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			schema := xsd.Bytes("derivation.xsd", []byte(test.schema))
			// Sweep failing phases without depending on an internal step count.
			for limit := 1; limit <= 1024; limit++ {
				engine, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDependencySteps: limit}, schema)
				if err == nil {
					if engine == nil {
						t.Fatal("successful compilation returned no engine")
					}
					return
				}
				if engine != nil {
					t.Fatal("failed compilation returned an engine")
				}
				expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
			}
			t.Fatal("small derivation schema did not compile within the dependency budget")
		})
	}
}

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
	for budget := 1; budget <= 1024; budget++ {
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
}
