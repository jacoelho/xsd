package schemaanalysis

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestDetectCyclesDeterministic(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A">
    <xs:complexContent>
      <xs:extension base="B"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:complexType name="B">
    <xs:complexContent>
      <xs:extension base="A"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	var first string
	for i := range 5 {
		err := DetectCycles(sch)
		if err == nil {
			t.Fatalf("expected cycle detection error")
		}
		if i == 0 {
			first = err.Error()
			continue
		}
		if err.Error() != first {
			t.Fatalf("cycle error mismatch: %q vs %q", err.Error(), first)
		}
	}
}
