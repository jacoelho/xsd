package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestXMLNamespaceAttributesAllowed(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))

	for _, doc := range []string{
		`<root xml:lang="en">ok</root>`,
		`<root xml:space="preserve">ok</root>`,
		`<root xml:base="http://example.com/">ok</root>`,
	} {
		if violations := validateStream(t, v, doc); len(violations) > 0 {
			t.Fatalf("expected no violations for %q, got %d", doc, len(violations))
		}
	}
}
