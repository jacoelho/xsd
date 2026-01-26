package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
)

func TestEntityDeclarationsOptional(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:ENTITY"/>
</xs:schema>`

	v := mustNewValidator(t, schemaXML)
	violations, err := v.ValidateStream(strings.NewReader(`<root xmlns="urn:test">ent</root>`))
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestEntityDeclarationsEnforced(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:ENTITY"/>
</xs:schema>`

	v := mustNewValidator(t, schemaXML)
	entities := map[string]struct{}{"declared": {}}
	violations, err := v.ValidateStreamWithEntities(strings.NewReader(`<root xmlns="urn:test">missing</root>`), entities)
	if err != nil {
		t.Fatalf("ValidateStreamWithEntities() error = %v", err)
	}
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected datatype violation, got %v", violations)
	}
}

func TestEntitiesListDeclarationsEnforced(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:ENTITIES"/>
</xs:schema>`

	v := mustNewValidator(t, schemaXML)
	entities := map[string]struct{}{"one": {}, "two": {}}
	violations, err := v.ValidateStreamWithEntities(strings.NewReader(`<root xmlns="urn:test">one three</root>`), entities)
	if err != nil {
		t.Fatalf("ValidateStreamWithEntities() error = %v", err)
	}
	if !hasViolationCode(violations, errors.ErrDatatypeInvalid) {
		t.Fatalf("expected datatype violation, got %v", violations)
	}
}
