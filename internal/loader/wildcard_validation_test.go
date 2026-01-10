package loader

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestWildcardUPA_ChoiceWildcardOverlapsElement(t *testing.T) {
	// UPA violation: wildcard in choice group overlaps with explicit element
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:element name="foo" type="xs:string"/>
        <xs:any namespace="##targetNamespace" processContents="skip"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: wildcard in choice overlaps with explicit element 'foo'")
	} else if !strings.Contains(err.Error(), "UPA violation") {
		t.Errorf("Expected UPA violation error, got: %v", err)
	}
}

func TestWildcardUPA_ChoiceWildcardOverlapsWildcard(t *testing.T) {
	// UPA violation: two overlapping wildcards in choice group
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:any namespace="##targetNamespace" processContents="skip"/>
        <xs:any namespace="##any" processContents="skip"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: overlapping wildcards in choice group")
	} else if !strings.Contains(err.Error(), "UPA violation") {
		t.Errorf("Expected UPA violation error, got: %v", err)
	}
}

func TestWildcardUPA_SequenceWildcardOverlapsElement_Valid(t *testing.T) {
	// valid: wildcard in sequence can overlap with explicit element (not a UPA violation)
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="foo" type="xs:string"/>
        <xs:any namespace="##targetNamespace" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err != nil {
		t.Errorf("Schema should be valid (sequence allows overlap): %v", err)
	}
}

func TestWildcardUPA_SequenceWildcardOverlapsWildcard_Valid(t *testing.T) {
	// valid: overlapping wildcards in sequence are allowed (not a UPA violation)
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:any namespace="##targetNamespace" processContents="skip"/>
        <xs:any namespace="##any" processContents="skip"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err != nil {
		t.Errorf("Schema should be valid (sequence allows overlap): %v", err)
	}
}

func TestWildcardUPA_NestedChoiceWildcardOverlapsElement(t *testing.T) {
	// UPA violation: wildcard in nested choice overlaps with explicit element
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:choice>
          <xs:element name="foo" type="xs:string"/>
          <xs:any namespace="##targetNamespace" processContents="skip"/>
        </xs:choice>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: wildcard in nested choice overlaps with explicit element")
	} else if !strings.Contains(err.Error(), "UPA violation") {
		t.Errorf("Expected UPA violation error, got: %v", err)
	}
}

func TestWildcardDerivation_RestrictionSubset_Valid(t *testing.T) {
	// valid: derived wildcard is a subset of base wildcard
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:any namespace="##any" processContents="skip"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="tns:BaseType">
        <xs:sequence>
          <xs:any namespace="##targetNamespace" processContents="skip"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err != nil {
		t.Errorf("Schema should be valid (##targetNamespace is subset of ##any): %v", err)
	}
}

func TestWildcardDerivation_RestrictionSubset_Invalid(t *testing.T) {
	// invalid: derived wildcard is not a subset of base wildcard
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:any namespace="##targetNamespace" processContents="skip"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="tns:BaseType">
        <xs:sequence>
          <xs:any namespace="##any" processContents="skip"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: ##any is not a subset of ##targetNamespace")
	} else if !strings.Contains(err.Error(), "wildcard") {
		t.Errorf("Expected wildcard error, got: %v", err)
	}
}

func TestWildcardDerivation_RestrictionProcessContentsWeaker_Invalid(t *testing.T) {
	// invalid: restriction cannot weaken processContents
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:any namespace="##any" processContents="strict"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="tns:BaseType">
        <xs:sequence>
          <xs:any namespace="##any" processContents="lax"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: processContents cannot be weakened in restriction")
	} else if !strings.Contains(err.Error(), "processContents") {
		t.Errorf("Expected processContents error, got: %v", err)
	}
}

func TestWildcardDerivation_RestrictionElementNamespace_Invalid(t *testing.T) {
	// invalid: element namespace must be allowed by base wildcard
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:any namespace="##other" processContents="skip"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="tns:BaseType">
        <xs:sequence>
          <xs:element name="foo" type="xs:string"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: element namespace is not allowed by base wildcard")
	} else if !strings.Contains(err.Error(), "base wildcard") {
		t.Errorf("Expected base wildcard namespace error, got: %v", err)
	}
}

func TestWildcardDerivation_RestrictionAddWildcard_Invalid(t *testing.T) {
	// invalid: cannot add wildcard in restriction when base has no wildcard
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:element name="foo" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="tns:BaseType">
        <xs:sequence>
          <xs:any namespace="##any" processContents="skip"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: cannot add wildcard in restriction when base has no wildcard")
	} else if !strings.Contains(err.Error(), "restriction") && !strings.Contains(err.Error(), "cannot restrict non-wildcard to wildcard") {
		t.Errorf("Expected restriction error about wildcard, got: %v", err)
	}
}

func TestWildcardDerivation_ExtensionUPA_Invalid(t *testing.T) {
	// invalid: repeating base wildcard overlaps extension wildcard (UPA violation)
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:any namespace="##targetNamespace" processContents="skip" minOccurs="0" maxOccurs="unbounded"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:extension base="tns:BaseType">
        <xs:sequence>
          <xs:any namespace="##targetNamespace" processContents="skip"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: repeating base wildcard overlaps extension wildcard")
	} else if !strings.Contains(err.Error(), "not deterministic") {
		t.Errorf("Expected extension UPA error, got: %v", err)
	}
}

func TestWildcardDerivation_ExtensionUPA_Valid(t *testing.T) {
	// valid: extension wildcard doesn't overlap with base element
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:element name="foo" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:extension base="tns:BaseType">
        <xs:sequence>
          <xs:any namespace="##other" processContents="skip"/>
        </xs:sequence>
      </xs:extension>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err != nil {
		t.Errorf("Schema should be valid (##other doesn't overlap with targetNamespace element): %v", err)
	}
}

func TestWildcardUPA_ChoiceDuplicateElementNames_Invalid(t *testing.T) {
	// UPA violation: duplicate element names with different types in choice
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:element name="foo" type="xs:string"/>
        <xs:element name="foo" type="xs:int"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: duplicate element names with different types in choice")
	} else if !strings.Contains(err.Error(), "UPA violation") &&
		!strings.Contains(err.Error(), "duplicate local element declaration") &&
		!strings.Contains(err.Error(), "duplicate element name") {
		t.Errorf("Expected UPA violation or duplicate element declaration error, got: %v", err)
	}
}

func TestWildcardUPA_ChoiceListWildcardOverlapsElement(t *testing.T) {
	// UPA violation: wildcard with namespace list overlaps with explicit element
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:element name="foo" type="xs:string"/>
        <xs:any namespace="##targetNamespace http://example.com" processContents="skip"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: wildcard namespace list overlaps with explicit element")
	} else if !strings.Contains(err.Error(), "UPA violation") {
		t.Errorf("Expected UPA violation error, got: %v", err)
	}
}

func TestWildcardDerivation_RestrictionListSubset_Valid(t *testing.T) {
	// valid: derived wildcard list is a subset of base wildcard list
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:any namespace="http://example.com http://other.com" processContents="skip"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="tns:BaseType">
        <xs:sequence>
          <xs:any namespace="http://example.com" processContents="skip"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err != nil {
		t.Errorf("Schema should be valid (list subset): %v", err)
	}
}

func TestWildcardDerivation_RestrictionListSubset_Invalid(t *testing.T) {
	// invalid: derived wildcard list is not a subset of base wildcard list
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:sequence>
      <xs:any namespace="http://example.com" processContents="skip"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="tns:BaseType">
        <xs:sequence>
          <xs:any namespace="http://example.com http://other.com" processContents="skip"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: derived list is not a subset of base list")
	} else if !strings.Contains(err.Error(), "wildcard") {
		t.Errorf("Expected wildcard error, got: %v", err)
	}
}

func TestWildcardDerivation_TargetNamespaceMismatch_Invalid(t *testing.T) {
	schemaA := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:b="b"
           targetNamespace="a"
           elementFormDefault="qualified">
  <xs:import namespace="b" schemaLocation="b.xsd"/>

  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="b:Base">
        <xs:sequence>
          <xs:any namespace="##other" processContents="skip"/>
        </xs:sequence>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	schemaB := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="b"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:any namespace="##other" processContents="skip"/>
    </xs:sequence>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"a.xsd": &fstest.MapFile{Data: []byte(schemaA)},
		"b.xsd": &fstest.MapFile{Data: []byte(schemaB)},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("a.xsd")
	if err == nil {
		t.Error("Schema should be invalid: wildcard restriction with differing targetNamespaces should fail")
	} else if !strings.Contains(err.Error(), "wildcard") {
		t.Errorf("Expected wildcard error, got: %v", err)
	}
}

func TestAnyAttributeDerivation_TargetNamespaceMismatch_Invalid(t *testing.T) {
	schemaA := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:b="b"
           targetNamespace="a"
           elementFormDefault="qualified">
  <xs:import namespace="b" schemaLocation="b.xsd"/>

  <xs:complexType name="derived">
    <xs:complexContent>
      <xs:restriction base="b:Base">
        <xs:anyAttribute namespace="##other" processContents="skip"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	schemaB := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="b"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:anyAttribute namespace="##other" processContents="skip"/>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"a.xsd": &fstest.MapFile{Data: []byte(schemaA)},
		"b.xsd": &fstest.MapFile{Data: []byte(schemaB)},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("a.xsd")
	if err == nil {
		t.Error("Schema should be invalid: anyAttribute restriction with differing targetNamespaces should fail")
	} else if !strings.Contains(err.Error(), "anyAttribute restriction") {
		t.Errorf("Expected anyAttribute restriction error, got: %v", err)
	}
}

func TestAnyAttributeDerivation_RestrictionProcessContentsWeaker_Invalid(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:complexType name="BaseType">
    <xs:anyAttribute namespace="##any" processContents="strict"/>
  </xs:complexType>
  <xs:complexType name="DerivedType">
    <xs:complexContent>
      <xs:restriction base="tns:BaseType">
        <xs:anyAttribute namespace="##any" processContents="skip"/>
      </xs:restriction>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err == nil {
		t.Error("Schema should be invalid: anyAttribute restriction cannot weaken processContents")
	} else if !strings.Contains(err.Error(), "anyAttribute restriction") {
		t.Errorf("Expected anyAttribute restriction error, got: %v", err)
	}
}

func TestWildcardUPA_ValidChoiceWithNonOverlappingWildcards(t *testing.T) {
	// valid: non-overlapping wildcards in choice group
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:choice>
        <xs:any namespace="##targetNamespace" processContents="skip"/>
        <xs:any namespace="##other" processContents="skip"/>
      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{
			Data: []byte(schemaXML),
		},
	}

	loader := NewLoader(Config{
		FS: testFS,
	})

	_, err := loader.Load("test.xsd")
	if err != nil {
		t.Errorf("Schema should be valid (non-overlapping wildcards): %v", err)
	}
}
