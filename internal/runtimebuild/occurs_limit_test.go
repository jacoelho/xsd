package runtimebuild

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestBuildSchemaOccursLimitError(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:string" maxOccurs="2"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	parsed := mustResolveSchema(t, schemaXML)
	_, err := buildSchemaForTest(parsed, BuildConfig{MaxOccursLimit: 1})
	if err == nil {
		t.Fatalf("expected maxOccurs limit error")
	}
	if !errors.Is(err, types.ErrOccursTooLarge) {
		t.Fatalf("expected %v, got %v", types.ErrOccursTooLarge, err)
	}
}
