package schemaanalysis_test

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	resolver "github.com/jacoelho/xsd/internal/semanticresolve"
)

func TestMissingTypeAllowedDuringResolution(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="MissingType"/>
</xs:schema>`

	sch, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if err := resolver.NewResolver(sch).Resolve(); err == nil {
		t.Fatalf("expected missing type to fail resolution")
	}
}
