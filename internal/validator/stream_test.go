package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func TestStreamValidatorValidSequence(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:int" minOccurs="0"/>
      </xs:sequence>
      <xs:attribute name="id" type="xs:ID"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test" id="a1"><a>hi</a><b>1</b></root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}
}

func TestStreamValidatorPrefixedElements(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<t:root xmlns:t="urn:test"><t:child>ok</t:child></t:root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}
}

func TestStreamContentModelErrorPathIncludesChild(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test"><b/></root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) == 0 {
		t.Fatalf("expected violations")
	}
	if violations[0].Path != "/{urn:test}root/{urn:test}b" {
		t.Fatalf("expected path /{urn:test}root/{urn:test}b, got %q", violations[0].Path)
	}
}

func TestStreamValidatorUnexpectedChild(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test"><c/></root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) == 0 {
		t.Fatalf("expected violations")
	}
	if violations[0].Code != string(errors.ErrUnexpectedElement) {
		t.Fatalf("expected code %s, got %s", errors.ErrUnexpectedElement, violations[0].Code)
	}
}

func TestStreamValidatorWildcardSkip(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip"/>
        <xs:element name="tail" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test" xmlns:other="urn:other">
  <other:skip><bad/></other:skip>
  <tail>ok</tail>
</root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %d", len(violations))
	}
}

func TestStreamValidatorNilledElement(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="n" type="xs:string" nillable="true"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <n xsi:nil="true">text</n>
</root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) == 0 {
		t.Fatalf("expected violations")
	}
	if violations[0].Code != string(errors.ErrNilElementNotEmpty) {
		t.Fatalf("expected code %s, got %s", errors.ErrNilElementNotEmpty, violations[0].Code)
	}
}

func TestStreamValidatorNilledElementWhitespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="n" type="xs:string" nillable="true"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <n xsi:nil="true">
  
  </n>
</root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) == 0 {
		t.Fatalf("expected violations")
	}
	if violations[0].Code != string(errors.ErrNilElementNotEmpty) {
		t.Fatalf("expected code %s, got %s", errors.ErrNilElementNotEmpty, violations[0].Code)
	}
}

func TestStreamValidatorNilledElementNonXMLWhitespace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="n" type="xs:string" nillable="true"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <n xsi:nil="true">&#xA0;</n>
</root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) == 0 {
		t.Fatalf("expected violations")
	}
	if violations[0].Code != string(errors.ErrNilElementNotEmpty) {
		t.Fatalf("expected code %s, got %s", errors.ErrNilElementNotEmpty, violations[0].Code)
	}
}

func TestStreamValidatorDefaultQNameUsesSchemaContext(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:QName" default="tns:code"/>
</xs:schema>`

	docXML := `<root xmlns="urn:test"/>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestStreamValidatorComplexContentMixedAllowsText(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base" mixed="true">
    <xs:sequence>
      <xs:element name="child" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent mixed="true">
      <xs:extension base="tns:Base"/>
    </xs:complexContent>
  </xs:complexType>
  <xs:element name="root" type="tns:Derived"/>
</xs:schema>`

	docXML := `<root xmlns="urn:test">text<child>ok</child>more</root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestStreamValidatorRejectsBOMAfterRoot(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	docXML := `<root xmlns="urn:test"/>` + "\uFEFF"

	_, err := validateStreamDoc(t, schemaXML, docXML)
	if err == nil {
		t.Fatalf("expected BOM outside root to return an error")
	}
}

func TestToTypesQName(t *testing.T) {
	got := toTypesQName(xmlstream.QName{Namespace: "urn:test", Local: "root"})
	if got.Namespace != types.NamespaceURI("urn:test") || got.Local != "root" {
		t.Fatalf("QName = %v, want {urn:test}root", got)
	}
	got = toTypesQName(xmlstream.QName{Namespace: "", Local: "local"})
	if got.Namespace != types.NamespaceEmpty || got.Local != "local" {
		t.Fatalf("QName empty = %v, want %q local", got, "local")
	}
}

func TestStreamValidatorReportsLineColumn(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="age" type="xs:int"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test">
  <age>30a</age>
</root>`

	violations, err := validateStreamDoc(t, schemaXML, docXML)
	if err != nil {
		t.Fatalf("ValidateStream() error = %v", err)
	}
	if len(violations) == 0 {
		t.Fatalf("expected violations")
	}

	var got *errors.Validation
	for i := range violations {
		if violations[i].Code == string(errors.ErrDatatypeInvalid) {
			got = &violations[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("expected datatype violation")
	}
	if got.Line != 2 || got.Column != 8 {
		t.Fatalf("line/column = %d/%d, want 2/8", got.Line, got.Column)
	}
}

func validateStreamDoc(t *testing.T, schemaXML, docXML string) ([]errors.Validation, error) {
	t.Helper()

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}
	v := New(mustCompile(t, schema))
	return v.ValidateStream(strings.NewReader(docXML))
}
