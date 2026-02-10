package parser

import (
	"strings"
	"testing"
)

func TestElementDefaultFixedConflictTopLevelAndLocal(t *testing.T) {
	t.Run("top-level", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" default="a" fixed="b" type="xs:string"/>
</xs:schema>`
		_, err := Parse(strings.NewReader(schemaXML))
		if err == nil {
			t.Fatalf("Parse() expected error")
		}
		if !strings.Contains(err.Error(), "element cannot have both 'default' and 'fixed' attributes") {
			t.Fatalf("Parse() error = %v", err)
		}
	})

	t.Run("local", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" default="a" fixed="b" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
		_, err := Parse(strings.NewReader(schemaXML))
		if err == nil {
			t.Fatalf("Parse() expected error")
		}
		if !strings.Contains(err.Error(), "element cannot have both 'default' and 'fixed' attributes") {
			t.Fatalf("Parse() error = %v", err)
		}
	})
}

func TestElementInvalidChildTopLevelAndLocal(t *testing.T) {
	t.Run("top-level", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:attribute name="a" type="xs:string"/>
  </xs:element>
</xs:schema>`
		_, err := Parse(strings.NewReader(schemaXML))
		if err == nil {
			t.Fatalf("Parse() expected error")
		}
		if !strings.Contains(err.Error(), "invalid child element <attribute> in <element> declaration") {
			t.Fatalf("Parse() error = %v", err)
		}
	})

	t.Run("local", func(t *testing.T) {
		schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child">
          <xs:attribute name="a" type="xs:string"/>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
		_, err := Parse(strings.NewReader(schemaXML))
		if err == nil {
			t.Fatalf("Parse() expected error")
		}
		if !strings.Contains(err.Error(), "invalid child element <attribute> in <element> declaration") {
			t.Fatalf("Parse() error = %v", err)
		}
	})
}
