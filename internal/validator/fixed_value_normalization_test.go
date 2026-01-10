package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestFixedValueNormalization_Boolean(t *testing.T) {
	t.Skip("TODO: implement value-space comparison for fixed values")
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
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestFixedValueNormalization_Decimal(t *testing.T) {
	t.Skip("TODO: implement value-space comparison for fixed values")
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
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestFixedValueNormalization_StringWhitespace(t *testing.T) {
	t.Skip("TODO: implement value-space comparison for fixed values")
	// test that fixed 'abcd edfgh ' matches 'abcd edfgh' after whitespace normalization
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string" fixed="abcd edfgh "/>
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
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestFixedValueNormalization_StringTrailingWhitespace(t *testing.T) {
	t.Skip("TODO: implement value-space comparison for fixed values")
	// test that fixed 'ENU ' matches 'ENU' after whitespace normalization
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string" fixed="ENU "/>
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
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestFixedValueNormalization_DateWhitespace(t *testing.T) {
	t.Skip("TODO: implement value-space comparison for fixed values")
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
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestFixedValueNormalization_UnionBoolean(t *testing.T) {
	t.Skip("TODO: implement value-space comparison for fixed values")
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
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestFixedValueNormalization_UnionDecimal(t *testing.T) {
	t.Skip("TODO: implement value-space comparison for fixed values")
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
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}
