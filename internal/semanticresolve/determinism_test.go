package semanticresolve

import (
	"strings"
	"testing"

	parser "github.com/jacoelho/xsd/internal/parser"
)

func TestValidateReferencesDeterministicOrder(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="a" type="MissingA"/>
  <xs:element name="b" type="MissingB"/>
  <xs:complexType name="T">
    <xs:sequence>
      <xs:element ref="missingRef"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="T"/>
</xs:schema>`

	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	var first []string
	for i := range 5 {
		errs := ValidateReferences(sch)
		if len(errs) == 0 {
			t.Fatalf("expected reference errors")
		}
		current := make([]string, len(errs))
		for i, err := range errs {
			if err == nil {
				current[i] = "<nil>"
				continue
			}
			current[i] = err.Error()
		}
		if i == 0 {
			first = current
			continue
		}
		if strings.Join(current, "\n") != strings.Join(first, "\n") {
			t.Fatalf("error order changed:\nfirst=%v\ncurrent=%v", first, current)
		}
	}
}
