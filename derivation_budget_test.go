package xsd_test

import (
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestDerivationBudgetExhaustionKeepsSchemaLimitDiagnostic(t *testing.T) {
	t.Parallel()

	schema := xsd.Bytes("restriction.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:attribute name="a" type="xs:string"/></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base">
    <xs:attribute name="a" type="xs:token"/>
  </xs:restriction></xs:complexContent></xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`))
	// Sweep the budget through every failing compilation phase without fixing
	// the test to one internal step count or publication order.
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
	t.Fatal("small restriction schema did not compile within the dependency budget")
}
