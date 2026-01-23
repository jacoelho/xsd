package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestDateTimeAllowsLeapSecond(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:dateTime"/>
</xs:schema>`

	docXML := `<root>1999-12-31T23:59:60Z</root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}
