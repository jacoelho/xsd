package loader

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestNotationEnumerationValidation tests that NOTATION enumeration values must reference declared notations
func TestNotationEnumerationValidation(t *testing.T) {
	tests := []struct {
		name      string
		schema    string
		wantError bool
		errMsg    string
	}{
		{
			name: "valid - enumeration references declared notation",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com">
  <xs:notation name="png" public="image/png"/>
  <xs:complexType name="Picture">
    <xs:attribute name="type">
      <xs:simpleType>
        <xs:restriction base="xs:NOTATION">
          <xs:enumeration value="png"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:attribute>
  </xs:complexType>
  <xs:element name="a" type="xs:string"/>
</xs:schema>`,
			wantError: false,
		},
		{
			name: "invalid - enumeration references undeclared notation",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com">
  <xs:notation name="png" public="image/png"/>
  <xs:complexType name="Picture">
    <xs:attribute name="type">
      <xs:simpleType>
        <xs:restriction base="xs:NOTATION">
          <xs:enumeration value="jpeg"/>
        </xs:restriction>
      </xs:simpleType>
    </xs:attribute>
  </xs:complexType>
  <xs:element name="a" type="xs:string"/>
</xs:schema>`,
			wantError: true,
			errMsg:    "does not reference a declared notation",
		},
		{
			name: "valid - multiple enumerations all declared",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com">
  <xs:notation name="png" public="image/png"/>
  <xs:notation name="gif" public="image/gif"/>
  <xs:notation name="jpeg" public="image/jpeg"/>
  <xs:simpleType name="imageFormat">
    <xs:restriction base="xs:NOTATION">
      <xs:enumeration value="png"/>
      <xs:enumeration value="gif"/>
      <xs:enumeration value="jpeg"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="a" type="xs:string"/>
</xs:schema>`,
			wantError: false,
		},
		{
			name: "invalid - one enumeration undeclared among many",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com">
  <xs:notation name="png" public="image/png"/>
  <xs:notation name="gif" public="image/gif"/>
  <xs:simpleType name="imageFormat">
    <xs:restriction base="xs:NOTATION">
      <xs:enumeration value="png"/>
      <xs:enumeration value="gif"/>
      <xs:enumeration value="bmp"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="a" type="xs:string"/>
</xs:schema>`,
			wantError: true,
			errMsg:    "bmp",
		},
		{
			name: "invalid - NOTATION restriction without enumeration facet",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://example.com">
  <xs:simpleType name="BadNotation">
    <xs:restriction base="xs:NOTATION"/>
  </xs:simpleType>
  <xs:element name="a" type="xs:string"/>
</xs:schema>`,
			wantError: true,
			errMsg:    "NOTATION restriction must have enumeration facet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFS := fstest.MapFS{
				"test.xsd": &fstest.MapFile{
					Data: []byte(tt.schema),
				},
			}

			loader := NewLoader(Config{
				FS: testFS,
			})

			_, err := loader.Load("test.xsd")

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error should contain %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestInvalidParticleOccurrenceConstraints tests that invalid minOccurs/maxOccurs combinations are rejected
func TestInvalidParticleOccurrenceConstraints(t *testing.T) {
	tests := []struct {
		name      string
		schema    string
		wantError bool
		errMsg    string
	}{
		{
			name: "minOccurs > maxOccurs should be invalid (maxOccurs=0 case)",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="B">
    <xs:sequence>
      <xs:element name="x"/>
    </xs:sequence>
  </xs:complexType>
  <xs:group name="A">
    <xs:sequence>
      <xs:element name="A"/>
      <xs:element name="B"/>
    </xs:sequence>
  </xs:group>
  <xs:element name="elem">
    <xs:complexType>
      <xs:complexContent>
        <xs:extension base="B">
          <xs:group ref="A" minOccurs="1" maxOccurs="0"/>
        </xs:extension>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			wantError: true,
			errMsg:    "maxOccurs cannot be 0 when minOccurs > 0",
		},
		{
			name: "minOccurs > maxOccurs should be invalid (general case)",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" minOccurs="5" maxOccurs="2"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			wantError: true,
			errMsg:    "maxOccurs less than minOccurs",
		},
		{
			name: "minOccurs = maxOccurs should be valid",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" minOccurs="2" maxOccurs="2"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			wantError: false,
		},
		{
			name: "minOccurs < maxOccurs should be valid",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" minOccurs="1" maxOccurs="10"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			wantError: false,
		},
		{
			name: "minOccurs with unbounded maxOccurs should be valid",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" minOccurs="5" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFS := fstest.MapFS{
				"test.xsd": &fstest.MapFile{
					Data: []byte(tt.schema),
				},
			}

			loader := NewLoader(Config{
				FS: testFS,
			})

			_, err := loader.Load("test.xsd")

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error should contain %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestExtensionOfAllGroup tests that extending a type with xs:all content model is rejected
func TestExtensionOfAllGroup(t *testing.T) {
	tests := []struct {
		name      string
		schema    string
		wantError bool
		errMsg    string
	}{
		{
			name: "extension of xs:all base type should be invalid",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    targetNamespace="http://xsdtesting"
    xmlns:x="http://xsdtesting"
    elementFormDefault="qualified">
  <xs:complexType name="base">
    <xs:all>
      <xs:element name="e1" type="xs:string"/>
      <xs:element name="e2" type="xs:string"/>
    </xs:all>
  </xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:extension base="x:base">
          <xs:sequence>
            <xs:element name="e3" type="xs:string"/>
          </xs:sequence>
        </xs:extension>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			wantError: true,
			errMsg:    "cannot extend type with non-emptiable xs:all content model",
		},
		{
			name: "extension of sequence base type should be valid",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    targetNamespace="http://xsdtesting"
    xmlns:x="http://xsdtesting">
  <xs:complexType name="base">
    <xs:sequence>
      <xs:element name="e1" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="doc">
    <xs:complexType>
      <xs:complexContent>
        <xs:extension base="x:base">
          <xs:sequence>
            <xs:element name="e2" type="xs:string"/>
          </xs:sequence>
        </xs:extension>
      </xs:complexContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFS := fstest.MapFS{
				"test.xsd": &fstest.MapFile{
					Data: []byte(tt.schema),
				},
			}

			loader := NewLoader(Config{
				FS: testFS,
			})

			_, err := loader.Load("test.xsd")

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error should contain %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

// TestAttributeReferenceFixedValueConflict tests that attribute references with conflicting fixed values are rejected
func TestAttributeReferenceFixedValueConflict(t *testing.T) {
	tests := []struct {
		name      string
		schema    string
		wantError bool
		errMsg    string
	}{
		{
			name: "attribute reference with conflicting fixed value should be invalid",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="att" fixed="123" />
  <xs:element name="doc">
    <xs:complexType>
      <xs:attribute ref="att" fixed="abc"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			wantError: true,
			errMsg:    "fixed value",
		},
		{
			name: "attribute reference with matching fixed value should be valid",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="att" fixed="123" />
  <xs:element name="doc">
    <xs:complexType>
      <xs:attribute ref="att" fixed="123"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			wantError: false,
		},
		{
			name: "attribute reference without fixed value should be valid",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="att" fixed="123" />
  <xs:element name="doc">
    <xs:complexType>
      <xs:attribute ref="att"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFS := fstest.MapFS{
				"test.xsd": &fstest.MapFile{
					Data: []byte(tt.schema),
				},
			}

			loader := NewLoader(Config{
				FS: testFS,
			})

			_, err := loader.Load("test.xsd")

			if tt.wantError {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error should contain %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
