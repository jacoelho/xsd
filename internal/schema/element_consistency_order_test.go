package schema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestElementConsistencyNestedMergeIsDeterministic(t *testing.T) {
	t.Parallel()
	var nested strings.Builder
	for i := range 200 {
		fmt.Fprintf(&nested, `<xs:element name="n%d" type="xs:string"/>`, i)
	}
	document := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="x" type="xs:string"/>
    <xs:choice>%s<xs:element name="x" type="xs:string"/></xs:choice>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`, nested.String())
	c, model := newEmptyComplexTypePublicationCompiler(t, document)
	changeConsistencyLocalType(t, &c.rt, "x", "int")

	check := func() (string, int) {
		steps := 0
		checker := newElementDeclarationConsistencyChecker(&c.rt, func(n int) error {
			steps += n
			return nil
		})
		err := checker.checkModel(model)
		expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
		return err.Error(), steps
	}
	message, steps := check()
	for range 32 {
		if again, againSteps := check(); again != message || againSteps != steps {
			t.Fatalf("repeated check = %q after %d steps, want %q after %d steps", again, againSteps, message, steps)
		}
	}
}
