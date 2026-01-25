package parser

import (
	"strings"
	"testing"
)

func TestParseRequiresExplicitXSPrefix(t *testing.T) {
	schema := `<?xml version="1.0"?>
<schema xmlns="http://www.w3.org/2001/XMLSchema">
  <element name="root" type="xs:string"/>
</schema>`

	if _, err := Parse(strings.NewReader(schema)); err == nil {
		t.Fatalf("expected undefined namespace prefix error for xs")
	}
}

func TestParseExplicitXSPrefixAllowed(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	if _, err := Parse(strings.NewReader(schema)); err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
}
