package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestFixedValueNormalization_Boolean(t *testing.T) {
	// test that fixed '1' matches 'true' after normalization
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:boolean" fixed="1"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// instance with 'true' should match fixed '1'
	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">true</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	if len(violations) > 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestFixedValueNormalization_Decimal(t *testing.T) {
	// test that fixed '1.0' matches '1.000' after normalization
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:decimal" fixed="1.0"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// instance with '1.000' should match fixed '1.0'
	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">1.000</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	if len(violations) > 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestFixedValueNormalization_StringWhitespace(t *testing.T) {
	// test that fixed 'abcd edfgh ' matches 'abcd edfgh' after whitespace normalization
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:token" fixed="abcd edfgh "/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// instance with 'abcd edfgh' should match fixed 'abcd edfgh ' after normalization
	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">abcd edfgh</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	if len(violations) > 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestFixedValueNormalization_StringTrailingWhitespace(t *testing.T) {
	// test that fixed 'ENU ' matches 'ENU' after whitespace normalization
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:token" fixed="ENU "/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// instance with 'ENU' should match fixed 'ENU ' after normalization
	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">ENU</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	if len(violations) > 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestFixedValueNormalization_DateWhitespace(t *testing.T) {
	// test that xs:date with valid format works with whitespace normalization
	// note: xs:date uses 'collapse' whitespace which removes leading/trailing whitespace
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:date" fixed="2004-04-05"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// instance with whitespace around valid date should match fixed value after normalization
	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">  2004-04-05  </root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	if len(violations) > 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestFixedValueNormalization_ListValueSpace(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="intList">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:intList" fixed="1 02"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="urn:test">1 2</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if len(violations) > 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestFixedValueNormalization_ListValueSpaceMismatch(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="intList">
    <xs:list itemType="xs:int"/>
  </xs:simpleType>
  <xs:element name="root" type="tns:intList" fixed="1 02"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="urn:test">1 3</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if len(violations) == 0 {
		t.Fatalf("expected fixed-value violation, got none")
	}
	if !hasViolationCode(violations, errors.ErrElementFixedValue) {
		t.Fatalf("expected code %s, got %v", errors.ErrElementFixedValue, violations)
	}
}

func TestFixedValueNormalization_UnionBoolean(t *testing.T) {
	// test that fixed '1' matches 'true' in a union type (stE050 case)
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" fixed="1">
    <xs:simpleType>
      <xs:union memberTypes="xs:boolean xs:int xs:string"/>
    </xs:simpleType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// instance with 'true' should match fixed '1' (both are boolean true)
	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">true</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	if len(violations) > 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestFixedValueNormalization_UnionDecimal(t *testing.T) {
	// test that fixed '1.0' matches '1.000' in a union type (stE055 case)
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" fixed="1.0">
    <xs:simpleType>
      <xs:union memberTypes="xs:boolean xs:float xs:double xs:normalizedString"/>
    </xs:simpleType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// instance with '1.000' should match fixed '1.0' (both are the same decimal value)
	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">1.000</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	if len(violations) > 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestFixedValueNormalization_FloatNaN(t *testing.T) {
	// NaN is not equal to itself in the XSD value space.
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:float" fixed="NaN"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">NaN</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	if !hasViolationCode(violations, errors.ErrElementFixedValue) {
		t.Fatalf("expected fixed-value violation, got %v", violations)
	}
}

func TestFixedValueNormalization_DoubleNaN(t *testing.T) {
	// NaN is not equal to itself in the XSD value space.
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:double" fixed="NaN"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">NaN</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	if !hasViolationCode(violations, errors.ErrElementFixedValue) {
		t.Fatalf("expected fixed-value violation, got %v", violations)
	}
}
