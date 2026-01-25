package validator

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/loader"
)

func TestAttributeFormDefaultIncludeRespected(t *testing.T) {
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:main"
           targetNamespace="urn:main"
           elementFormDefault="qualified"
           attributeFormDefault="unqualified">
  <xs:include schemaLocation="included.xsd"/>
  <xs:element name="root" type="tns:Holder"/>
</xs:schema>`),
		},
		"included.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:main"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:complexType name="Holder">
    <xs:attribute name="attr" type="xs:string"/>
  </xs:complexType>
</xs:schema>`),
		},
	}

	schemaLoader := loader.NewLoader(loader.Config{FS: testFS})
	schema, err := schemaLoader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	v := New(mustCompile(t, schema))

	validDoc := `<m:root xmlns:m="urn:main" m:attr="ok"/>`
	violations := validateStream(t, v, validDoc)
	if len(violations) > 0 {
		t.Fatalf("expected no violations for qualified local attribute, got %v", violations)
	}

	invalidDoc := `<m:root xmlns:m="urn:main" attr="bad"/>`
	violations = validateStream(t, v, invalidDoc)
	if !hasViolationCode(violations, errors.ErrAttributeNotDeclared) {
		t.Fatalf("expected attribute not declared violation, got %v", violations)
	}
}

func TestAttributeFormDefaultImportRespected(t *testing.T) {
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:main"
           xmlns:o="urn:other"
           targetNamespace="urn:main"
           elementFormDefault="qualified"
           attributeFormDefault="unqualified">
  <xs:import namespace="urn:other" schemaLocation="other.xsd"/>
  <xs:element name="root" type="o:Holder"/>
</xs:schema>`),
		},
		"other.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:other"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:complexType name="Holder">
    <xs:attribute name="attr" type="xs:string"/>
  </xs:complexType>
</xs:schema>`),
		},
	}

	schemaLoader := loader.NewLoader(loader.Config{FS: testFS})
	schema, err := schemaLoader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	v := New(mustCompile(t, schema))

	validDoc := `<m:root xmlns:m="urn:main" xmlns:o="urn:other" o:attr="ok"/>`
	violations := validateStream(t, v, validDoc)
	if len(violations) > 0 {
		t.Fatalf("expected no violations for qualified imported attribute, got %v", violations)
	}

	invalidDoc := `<m:root xmlns:m="urn:main" xmlns:o="urn:other" attr="bad"/>`
	violations = validateStream(t, v, invalidDoc)
	if !hasViolationCode(violations, errors.ErrAttributeNotDeclared) {
		t.Fatalf("expected attribute not declared violation, got %v", violations)
	}
}
