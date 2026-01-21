package validator

import (
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestNonXMLWhitespaceAroundValues(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		doc    string
	}{
		{
			name: "boolean",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:boolean"/>
</xs:schema>`,
			doc: `<root>&#xA0;true&#xA0;</root>`,
		},
		{
			name: "integer",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:int"/>
</xs:schema>`,
			doc: `<root>&#xA0;42&#xA0;</root>`,
		},
		{
			name: "dateTime",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:dateTime"/>
</xs:schema>`,
			doc: `<root>&#xA0;2001-01-01T00:00:00&#xA0;</root>`,
		},
		{
			name: "QName",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:QName"/>
</xs:schema>`,
			doc: `<root xmlns:ex="urn:ex">&#xA0;ex:val&#xA0;</root>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations, err := validateStreamDoc(t, tt.schema, tt.doc)
			if err != nil {
				t.Fatalf("ValidateStream() error = %v", err)
			}
			if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
				t.Fatalf("expected datatype violation, got %v", violations)
			}
		})
	}
}
