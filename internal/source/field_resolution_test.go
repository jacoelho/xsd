package source

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/types"
)

// TestFieldResolution_AttributeField tests field type resolution for attribute fields
// Test case 1 from issue document
func TestFieldResolution_AttributeField(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="part">
    <xs:complexType>
      <xs:attribute name="number" type="xs:integer"/>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="."/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	partQName := types.QName{
		Namespace: "http://example.com",
		Local:     "part",
	}
	decl, ok := schema.ElementDecls[partQName]
	if !ok {
		t.Fatal("element 'part' not found")
	}

	var keyConstraint *types.IdentityConstraint
	for _, constraint := range decl.Constraints {
		if constraint.Name == "partKey" {
			keyConstraint = constraint
			break
		}
	}
	if keyConstraint == nil {
		t.Fatal("key constraint 'partKey' not found")
	}

	if len(keyConstraint.Fields) == 0 {
		t.Fatal("key constraint should have fields")
	}

	field := keyConstraint.Fields[0]
	if field.ResolvedType == nil {
		t.Error("field type should be resolved")
	}

	// verify it's integer type
	if field.ResolvedType.Name().Local != "integer" {
		t.Errorf("field type should be integer, got %s", field.ResolvedType.Name().Local)
	}
}

// TestFieldResolution_AttributeAxis tests field type resolution for attribute:: axis.
func TestFieldResolution_AttributeAxis(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="part">
    <xs:complexType>
      <xs:attribute name="number" type="xs:integer"/>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="."/>
      <xs:field xpath="attribute::number"/>
    </xs:key>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	partQName := types.QName{
		Namespace: "http://example.com",
		Local:     "part",
	}
	decl, ok := schema.ElementDecls[partQName]
	if !ok {
		t.Fatal("element 'part' not found")
	}

	var keyConstraint *types.IdentityConstraint
	for _, constraint := range decl.Constraints {
		if constraint.Name == "partKey" {
			keyConstraint = constraint
			break
		}
	}
	if keyConstraint == nil {
		t.Fatal("key constraint 'partKey' not found")
	}

	if len(keyConstraint.Fields) == 0 {
		t.Fatal("key constraint should have fields")
	}

	field := keyConstraint.Fields[0]
	if field.ResolvedType == nil {
		t.Fatal("field type should be resolved")
	}

	if field.ResolvedType.Name().Local != "integer" {
		t.Errorf("field type should be integer, got %s", field.ResolvedType.Name().Local)
	}
}

func TestLocalIdentityConstraintContext(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:local"
           xmlns:tns="urn:local"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="id" type="xs:string"/>
            </xs:sequence>
          </xs:complexType>
          <xs:key name="k">
            <xs:selector xpath="."/>
            <xs:field xpath="tns:id"/>
          </xs:key>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: testFS})
	if _, err := loader.Load("test.xsd"); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
}

func TestFieldResolution_UnionWithAttributeAllowed(t *testing.T) {
	// Union fields can mix element and attribute selections.
	// At runtime, only one branch will match, selecting either an element or an attribute.
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:union"
           xmlns:tns="urn:union"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
      </xs:sequence>
      <xs:attribute name="id" type="xs:string"/>
    </xs:complexType>
    <xs:key name="mixedKey">
      <xs:selector xpath="."/>
      <xs:field xpath="tns:a | @id"/>
    </xs:key>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: testFS})
	if _, err := loader.Load("test.xsd"); err != nil {
		t.Fatalf("expected union field mixing element and attribute to pass, got: %v", err)
	}
}

func TestFieldResolution_UnionIncompatibleTypesFails(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:union"
           xmlns:tns="urn:union"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:int"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="badKey">
      <xs:selector xpath="."/>
      <xs:field xpath="tns:a | tns:b"/>
    </xs:key>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: testFS})
	if _, err := loader.Load("test.xsd"); err == nil {
		t.Fatal("expected union field with incompatible types to fail")
	}
}

func TestFieldResolution_UnionCompatibleTypesPass(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:union"
           xmlns:tns="urn:union"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
        <xs:element name="b" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="goodKey">
      <xs:selector xpath="."/>
      <xs:field xpath="tns:a | tns:b"/>
    </xs:key>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: testFS})
	if _, err := loader.Load("test.xsd"); err != nil {
		t.Fatalf("expected union field with compatible types to pass, got: %v", err)
	}
}

func TestFieldResolution_FieldSelectsNillableKeyFails(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:nillable"
           xmlns:tns="urn:nillable"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string" nillable="true"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="badKey">
      <xs:selector xpath="."/>
      <xs:field xpath="tns:a"/>
    </xs:key>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: testFS})
	if _, err := loader.Load("test.xsd"); err == nil {
		t.Fatal("expected key field selecting nillable element to fail")
	}
}

func TestFieldResolution_FieldSelectsNillableKeyrefAllowed(t *testing.T) {
	// Per XSD 1.0 spec, keyref fields CAN select nillable elements.
	// Nil values in keyref fields cause the tuple to be excluded from the check.
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:nillable"
           xmlns:tns="urn:nillable"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string" nillable="true"/>
        <xs:element name="b" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="goodKey">
      <xs:selector xpath="."/>
      <xs:field xpath="tns:b"/>
    </xs:key>
    <xs:keyref name="goodRef" refer="tns:goodKey">
      <xs:selector xpath="."/>
      <xs:field xpath="tns:a"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: testFS})
	if _, err := loader.Load("test.xsd"); err != nil {
		t.Fatalf("expected keyref field selecting nillable element to pass, got: %v", err)
	}
}

// TestFieldResolution_DescendantAttributeField tests descendant attribute field resolution.
func TestFieldResolution_DescendantAttributeField(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="container">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="part">
          <xs:complexType>
            <xs:attribute name="number" type="xs:integer"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="."/>
      <xs:field xpath=".//@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	containerQName := types.QName{
		Namespace: "http://example.com",
		Local:     "container",
	}
	decl, ok := schema.ElementDecls[containerQName]
	if !ok {
		t.Fatal("element 'container' not found")
	}

	var keyConstraint *types.IdentityConstraint
	for _, constraint := range decl.Constraints {
		if constraint.Name == "partKey" {
			keyConstraint = constraint
			break
		}
	}
	if keyConstraint == nil {
		t.Fatal("key constraint 'partKey' not found")
	}

	if len(keyConstraint.Fields) == 0 {
		t.Fatal("key constraint should have fields")
	}

	field := keyConstraint.Fields[0]
	if field.ResolvedType == nil {
		t.Fatal("field type should be resolved")
	}

	if field.ResolvedType.Name().Local != "integer" {
		t.Errorf("field type should be integer, got %s", field.ResolvedType.Name().Local)
	}
}

// TestFieldResolution_ChildElementField tests field type resolution for child element fields
// Test case 2 from issue document
func TestFieldResolution_ChildElementField(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="part">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="number" type="xs:integer"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="."/>
      <xs:field xpath="tns:number"/>
    </xs:key>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	partQName := types.QName{
		Namespace: "http://example.com",
		Local:     "part",
	}
	decl, ok := schema.ElementDecls[partQName]
	if !ok {
		t.Fatal("element 'part' not found")
	}

	var keyConstraint *types.IdentityConstraint
	for _, constraint := range decl.Constraints {
		if constraint.Name == "partKey" {
			keyConstraint = constraint
			break
		}
	}
	if keyConstraint == nil {
		t.Fatal("key constraint 'partKey' not found")
	}

	if len(keyConstraint.Fields) == 0 {
		t.Fatal("key constraint should have fields")
	}

	field := keyConstraint.Fields[0]
	if field.ResolvedType == nil {
		t.Fatal("field type should be resolved")
	}

	// verify it's integer type
	if field.ResolvedType.Name().Local != "integer" {
		t.Errorf("field type should be integer, got %s", field.ResolvedType.Name().Local)
	}
}

// TestFieldResolution_DescendantElementField tests field type resolution for descendant element fields
// Test case 3 from issue document - nested path with element and attribute
func TestFieldResolution_DescendantElementField(t *testing.T) {
	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="container">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="parts">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="part">
                <xs:complexType>
                  <xs:attribute name="number" type="xs:integer"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="tns:parts"/>
      <xs:field xpath="tns:part/@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	schema, err := loader.Load("test.xsd")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	containerQName := types.QName{
		Namespace: "http://example.com",
		Local:     "container",
	}
	decl, ok := schema.ElementDecls[containerQName]
	if !ok {
		t.Fatal("element 'container' not found")
	}

	var keyConstraint *types.IdentityConstraint
	for _, constraint := range decl.Constraints {
		if constraint.Name == "partKey" {
			keyConstraint = constraint
			break
		}
	}
	if keyConstraint == nil {
		t.Fatal("key constraint 'partKey' not found")
	}

	if len(keyConstraint.Fields) == 0 {
		t.Fatal("key constraint should have fields")
	}

	field := keyConstraint.Fields[0]
	if field.ResolvedType == nil {
		t.Fatal("field type should be resolved for nested path part/@number")
	}

	// verify it's integer type
	if field.ResolvedType.Name().Local != "integer" {
		t.Errorf("field type should be integer, got %s", field.ResolvedType.Name().Local)
	}
}
