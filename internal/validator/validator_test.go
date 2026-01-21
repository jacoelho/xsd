package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/loader"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/resolver"
)

func TestValidateSimpleElement(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/simple"
           xmlns:tns="http://example.com/simple"
           elementFormDefault="qualified">
  <xs:element name="message" type="xs:string"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<message xmlns="http://example.com/simple">Hello, World!</message>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	if len(violations) > 0 {
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestValidateComplexElement(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com/simple"
           xmlns:tns="http://example.com/simple"
           elementFormDefault="qualified">
  <xs:element name="person">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="name" type="xs:string"/>
        <xs:element name="age" type="xs:integer"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	xmlDoc := `<?xml version="1.0"?>
<person xmlns="http://example.com/simple">
  <name>John Doe</name>
  <age>30</age>
</person>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	if len(violations) > 0 {
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestValidateAttributeDefault(t *testing.T) {
	// create a simple schema with an attribute that has a default value
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="status" type="xs:string" default="active"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// test XML without the attribute (should use default)
	xmlWithoutAttr := `<?xml version="1.0"?>
<root xmlns="http://example.com/test"/>
`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlWithoutAttr)

	// should be valid - default value will be validated
	if len(violations) > 0 {
		t.Errorf("Expected no violations for missing attribute with default, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}

	// test XML with the attribute present
	xmlWithAttr := `<?xml version="1.0"?>
<root xmlns="http://example.com/test" status="inactive"/>
`

	violations = validateStream(t, v, xmlWithAttr)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for present attribute, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestValidateSubstitutionGroup(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" type="xs:string" substitutionGroup="head"/>
  
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="head"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// test XML with head element (should be valid)
	xmlWithHead := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <head>value</head>
</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlWithHead)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for head element, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}

	// test XML with member element (should be valid due to substitution group)
	xmlWithMember := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <member>value</member>
</root>`

	violations = validateStream(t, v, xmlWithMember)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for member element (substitution group), got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestValidateXsiAttributes(t *testing.T) {
	// test that standard xsi:* attributes are allowed without explicit declaration
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// test XML with xsi:schemaLocation attribute (should be allowed)
	xmlWithSchemaLocation := `<?xml version="1.0"?>
<root xmlns="http://example.com/test"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:schemaLocation="http://example.com/test schema.xsd">
  <child>value</child>
</root>`

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlWithSchemaLocation)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for xsi:schemaLocation attribute, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}

	// test XML with xsi:noNamespaceSchemaLocation attribute (should be allowed)
	xmlWithNoNamespaceSchemaLocation := `<?xml version="1.0"?>
<root xmlns="http://example.com/test"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:noNamespaceSchemaLocation="schema.xsd">
  <child>value</child>
</root>`
	violations = validateStream(t, v, xmlWithNoNamespaceSchemaLocation)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for xsi:noNamespaceSchemaLocation attribute, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}

	// test XML with both xsi attributes (should be allowed)
	xmlWithBoth := `<?xml version="1.0"?>
<root xmlns="http://example.com/test"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:schemaLocation="http://example.com/test schema.xsd"
      xsi:noNamespaceSchemaLocation="schema.xsd">
  <child>value</child>
</root>`

	violations = validateStream(t, v, xmlWithBoth)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for both xsi attributes, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}

	// test XML with unknown xsi attribute (should be rejected)
	xmlWithUnknown := `<?xml version="1.0"?>
<root xmlns="http://example.com/test"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:foo="bar">
  <child>value</child>
</root>`
	violations = validateStream(t, v, xmlWithUnknown)
	if !hasViolationCode(violations, errors.ErrAttributeNotDeclared) {
		t.Fatalf("Expected violation code %s, got: %v", errors.ErrAttributeNotDeclared, violations)
	}
}

func TestCircularAttributeGroupReference(t *testing.T) {
	// test that circular attribute group references are detected and rejected
	// AttributeGroup A references AttributeGroup B, which references AttributeGroup A
	// per XSD spec, this is an error and should be caught during schema resolution
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:attributeGroup name="GroupA">
    <xs:attribute name="attrA" type="xs:string"/>
    <xs:attributeGroup ref="tns:GroupB"/>
  </xs:attributeGroup>
  <xs:attributeGroup name="GroupB">
    <xs:attribute name="attrB" type="xs:string"/>
    <xs:attributeGroup ref="tns:GroupA"/>
  </xs:attributeGroup>
  <xs:element name="root">
    <xs:complexType>
      <xs:attributeGroup ref="tns:GroupA"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// resolve schema - this should detect the circular reference
	res := resolver.NewResolver(schema)
	err = res.Resolve()

	// circular attribute group references should be detected as an error
	if err == nil {
		t.Error("Expected error for circular attribute group reference, got none")
	} else if !strings.Contains(err.Error(), "circular") {
		t.Errorf("Expected circular reference error, got: %v", err)
	}
}

func TestValidateAllGroupMinOccursZero(t *testing.T) {
	// test that when an <all> group has minOccurs="0", all child elements become optional
	// this is based on the issue described in FIX_PLAN.md #2
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="elt1">
    <xs:complexType>
      <xs:all minOccurs="0">
        <xs:element name="elt2" type="xs:string"/>
        <xs:element name="elt3" type="xs:string"/>
        <xs:element name="elt4" type="xs:string"/>
      </xs:all>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// test XML with empty element (should be valid because all group has minOccurs="0")
	xmlEmpty := `<?xml version="1.0"?>
<elt1 xmlns="http://example.com/test"/>
`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlEmpty)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for empty element with all group minOccurs=0, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}

	// test XML with some elements (should be invalid because required elements are missing)
	xmlPartial := `<?xml version="1.0"?>
<elt1 xmlns="http://example.com/test">
  <elt2>value2</elt2>
</elt1>
`
	violations = validateStream(t, v, xmlPartial)
	if len(violations) == 0 {
		t.Fatalf("Expected violations for partial elements with all group minOccurs=0, got none")
	}
	found := false
	for _, v := range violations {
		if v.Code == string(errors.ErrRequiredElementMissing) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Expected violation code %s, got: %v", errors.ErrRequiredElementMissing, violations)
	}

	// test XML with all elements (should also be valid)
	xmlAll := `<?xml version="1.0"?>
<elt1 xmlns="http://example.com/test">
  <elt2>value2</elt2>
  <elt3>value3</elt3>
  <elt4>value4</elt4>
</elt1>
`
	violations = validateStream(t, v, xmlAll)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for all elements with all group minOccurs=0, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestValidateWildcardElement(t *testing.T) {
	// test that elements matching wildcards are allowed even without declarations
	// this tests the fix for FIX_PLAN.md #5
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// test XML with element that matches wildcard but has no declaration
	// this should be valid because processContents="skip"
	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value</foo>
</root>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for element matching wildcard with processContents=skip, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestValidateWildcardElementLax(t *testing.T) {
	// test wildcard with processContents="lax" - should allow elements without declarations
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="lax"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// test XML with element that matches wildcard but has no declaration
	// this should be valid because processContents="lax" allows elements without declarations
	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value</foo>
</root>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for element matching wildcard with processContents=lax, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestValidateWildcardAsTopLevelParticle(t *testing.T) {
	// test that a wildcard as a top-level particle (not nested in a sequence) is handled correctly
	// this tests the fix for validateParticle() not handling AnyElement
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:any namespace="##any" processContents="skip"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// test XML with element that matches wildcard
	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value</foo>
</root>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for element matching top-level wildcard, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestValidateWildcardWithMinOccursMaxOccurs(t *testing.T) {
	// test wildcard with minOccurs and maxOccurs constraints
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##any" processContents="skip" minOccurs="1" maxOccurs="2"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	// test XML with one element (should be valid)
	xmlDoc1 := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value</foo>
</root>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc1)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for one wildcard element, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}

	// test XML with two elements (should be valid)
	xmlDoc2 := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">
  <foo xmlns="http://other.com/ns">value1</foo>
  <bar xmlns="http://other.com/ns">value2</bar>
</root>`
	violations = validateStream(t, v, xmlDoc2)
	if len(violations) > 0 {
		t.Errorf("Expected no violations for two wildcard elements, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}

	// test XML with zero elements (should fail - minOccurs="1")
	xmlDoc3 := `<?xml version="1.0"?>
<root xmlns="http://example.com/test"/>
`
	violations = validateStream(t, v, xmlDoc3)
	if len(violations) == 0 {
		t.Error("Expected violation for missing required wildcard element (minOccurs=1), got none")
	} else {
		found := false
		for _, violation := range violations {
			errLower := strings.ToLower(violation.Error())
			if strings.Contains(errLower, "required") || strings.Contains(errLower, "missing") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected violation about missing required element, got: %v", violations)
		}
	}
}

func TestAttributeWildcardInheritance(t *testing.T) {
	// test that attribute wildcards are inherited from base types
	// base type has anyAttribute allowing any namespace
	// derived type extends base type
	// attribute matching wildcard should be allowed
	// note: processContents="skip" is used because strict mode requires declarations
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:anyAttribute namespace="##any" processContents="skip"/>
  </xs:complexType>
  
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:extension base="tns:BaseType">
        <xs:attribute name="localAttr" type="xs:string"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  
  <xs:element name="root" type="tns:DerivedType"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	if validationErrors := loader.ValidateSchema(schema); len(validationErrors) > 0 {
		t.Fatalf("Validate schema: %v", validationErrors)
	}

	// XML with attribute that should be allowed by inherited wildcard
	xmlDoc := `<?xml version="1.0"?>
<root xmlns="http://example.com/test"
      xmlns:other="http://example.com/other"
      localAttr="value"
      other:wildcardAttr="allowed"/>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc)

	// should have no violations - other:wildcardAttr should be allowed by inherited wildcard
	if len(violations) > 0 {
		t.Errorf("Expected no violations, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestAttributeWildcardOverride(t *testing.T) {
	// test that derived type's anyAttribute unions with base type's anyAttribute for extension
	// base type allows any namespace (##any)
	// derived type (extension) allows target namespace (##targetNamespace)
	// according to XSD spec, extension uses UNION, so result is ##any ∪ ##targetNamespace = ##any
	// attribute in other namespace should be allowed (because union = ##any)
	// note: processContents="skip" is used because strict mode requires declarations
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:anyAttribute namespace="##any" processContents="skip"/>
  </xs:complexType>
  
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:extension base="tns:BaseType">
        <xs:anyAttribute namespace="##targetNamespace" processContents="skip"/>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
  
  <xs:element name="root" type="tns:DerivedType"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	if validationErrors := loader.ValidateSchema(schema); len(validationErrors) > 0 {
		t.Fatalf("Validate schema: %v", validationErrors)
	}

	// XML with attribute in target namespace (should be allowed)
	// note: attributes without prefix are in no namespace, not default namespace
	// so we use a prefixed attribute in target namespace
	xmlDoc1 := `<?xml version="1.0"?>
<root xmlns="http://example.com/test"
      xmlns:tns="http://example.com/test"
      xmlns:other="http://example.com/other"
      tns:localAttr="value"/>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc1)

	// should have no violations - localAttr in target namespace should be allowed
	if len(violations) > 0 {
		t.Errorf("Expected no violations for target namespace attribute, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}

	// XML with attribute in other namespace (should be allowed - union = ##any)
	xmlDoc2 := `<?xml version="1.0"?>
<root xmlns="http://example.com/test"
      xmlns:other="http://example.com/other"
      other:wildcardAttr="allowed"/>`
	violations = validateStream(t, v, xmlDoc2)

	// should have no violations - other namespace attribute should be allowed because union = ##any
	if len(violations) > 0 {
		t.Errorf("Expected no violations for attribute in other namespace (union of ##any and ##targetNamespace = ##any), got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}
}

func TestValidateUnionWithInlineTypes(t *testing.T) {
	// test union type with inline simpleType children
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com/test"
           targetNamespace="http://example.com/test"
           elementFormDefault="qualified">
  <xs:simpleType name="StringOrInteger">
    <xs:union>
      <xs:simpleType>
        <xs:restriction base="xs:string">
          <xs:maxLength value="10"/>
        </xs:restriction>
      </xs:simpleType>
      <xs:simpleType>
        <xs:restriction base="xs:integer">
          <xs:minInclusive value="0"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:union>
  </xs:simpleType>
  
  <xs:element name="root" type="tns:StringOrInteger"/>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	if validationErrors := loader.ValidateSchema(schema); len(validationErrors) > 0 {
		t.Fatalf("Validate schema: %v", validationErrors)
	}

	// test valid string value (matches first inline type)
	xmlDoc1 := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">hello</root>`
	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, xmlDoc1)

	if len(violations) > 0 {
		t.Errorf("Expected no violations for valid string value, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}

	// test valid integer value (matches second inline type)
	xmlDoc2 := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">42</root>`
	violations = validateStream(t, v, xmlDoc2)

	if len(violations) > 0 {
		t.Errorf("Expected no violations for valid integer value, got %d:", len(violations))
		for _, v := range violations {
			t.Errorf("  %s", v.Error())
		}
	}

	// test invalid value (too long string)
	xmlDoc3 := `<?xml version="1.0"?>
<root xmlns="http://example.com/test">this string is too long</root>`
	violations = validateStream(t, v, xmlDoc3)

	if len(violations) == 0 {
		t.Error("Expected violation for string exceeding maxLength, got none")
	}

	// test that inline types are being used for validation
	// the key test is that values matching inline types should be accepted.
	// note: The validateUnionType function uses v.schema indirectly when validating
	// inline types - if an inline type is itself a union/list with QName references,
	// those are resolved through the recursive validateSimpleValue -> validateUnionType
}
