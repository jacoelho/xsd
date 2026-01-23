package validator

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/loader"
)

func TestChameleonWildcardTargetNamespaceElement(t *testing.T) {
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:main"
           targetNamespace="urn:main"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="root" type="tns:Holder"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Holder">
    <xs:sequence>
      <xs:any namespace="##targetNamespace" processContents="skip"/>
    </xs:sequence>
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

	validDoc := `<m:root xmlns:m="urn:main"><m:child/></m:root>`
	violations := validateStream(t, v, validDoc)
	if len(violations) > 0 {
		t.Fatalf("expected no violations for target namespace child, got %v", violations)
	}

	invalidDoc := `<m:root xmlns:m="urn:main"><child/></m:root>`
	violations = validateStream(t, v, invalidDoc)
	if len(violations) == 0 {
		t.Fatalf("expected violations for no-namespace child")
	}
}

func TestChameleonWildcardTargetNamespaceAttribute(t *testing.T) {
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:main"
           targetNamespace="urn:main"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="root" type="tns:Holder"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Holder">
    <xs:anyAttribute namespace="##targetNamespace" processContents="skip"/>
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
		t.Fatalf("expected no violations for target namespace attribute, got %v", violations)
	}

	invalidDoc := `<m:root xmlns:m="urn:main" attr="bad"/>`
	violations = validateStream(t, v, invalidDoc)
	if !hasViolationCode(violations, errors.ErrAttributeNotDeclared) {
		t.Fatalf("expected attribute not declared violation, got %v", violations)
	}
}

func TestChameleonIncludePreservesUnqualifiedLocals(t *testing.T) {
	testFS := fstest.MapFS{
		"main.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:main"
           targetNamespace="urn:main"
           elementFormDefault="qualified">
  <xs:include schemaLocation="common.xsd"/>
  <xs:element name="parent" type="tns:ParentType"/>
</xs:schema>`),
		},
		"common.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           elementFormDefault="qualified"
           attributeFormDefault="qualified">
  <xs:complexType name="ParentType">
    <xs:sequence>
      <xs:element name="child" type="xs:string" form="unqualified"/>
    </xs:sequence>
    <xs:attribute name="attr" type="xs:string" use="optional" form="unqualified"/>
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

	validDoc := `<m:parent xmlns:m="urn:main" attr="v"><child>ok</child></m:parent>`
	violations := validateStream(t, v, validDoc)
	if len(violations) > 0 {
		t.Fatalf("expected no violations for unqualified locals, got %v", violations)
	}

	invalidDoc := `<m:parent xmlns:m="urn:main" m:attr="v"><m:child>ok</m:child></m:parent>`
	violations = validateStream(t, v, invalidDoc)
	if len(violations) == 0 {
		t.Fatalf("expected violations for qualified local names")
	}
}
