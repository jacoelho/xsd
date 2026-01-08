package validator

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/loader"
)

func TestNotationValidation(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		instance   string
		wantValid  bool
		wantErrMsg string // substring expected in violation message (if wantValid=false)
	}{
		{
			name: "valid notation reference",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="gif" public="image/gif"/>
  <xs:notation name="jpeg" public="image/jpeg"/>
  
  <xs:simpleType name="imageFormat">
    <xs:restriction base="xs:NOTATION">
      <xs:enumeration value="gif"/>
      <xs:enumeration value="jpeg"/>
    </xs:restriction>
  </xs:simpleType>
  
  <xs:element name="image">
    <xs:complexType>
      <xs:attribute name="format" type="imageFormat"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			instance: `<?xml version="1.0"?>
<image format="gif"/>`,
			wantValid: true,
		},
		{
			name: "invalid notation reference - undeclared notation",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="gif" public="image/gif"/>
  
  <xs:simpleType name="imageFormat">
    <xs:restriction base="xs:NOTATION">
      <xs:enumeration value="gif"/>
    </xs:restriction>
  </xs:simpleType>
  
  <xs:element name="image">
    <xs:complexType>
      <xs:attribute name="format" type="imageFormat"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			instance: `<?xml version="1.0"?>
<image format="png"/>`,
			wantValid:  false,
			wantErrMsg: "does not reference a declared notation",
		},
		{
			name: "notation in element content",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:notation name="pdf" public="application/pdf"/>
  
  <xs:simpleType name="docFormat">
    <xs:restriction base="xs:NOTATION">
      <xs:enumeration value="pdf"/>
    </xs:restriction>
  </xs:simpleType>
  
  <xs:element name="document" type="docFormat"/>
</xs:schema>`,
			instance: `<?xml version="1.0"?>
<document>pdf</document>`,
			wantValid: true,
		},
		{
			name: "notation with namespace prefix",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" 
           targetNamespace="http://example.com/media"
           xmlns:m="http://example.com/media">
  <xs:notation name="mp3" public="audio/mpeg"/>
  
  <xs:simpleType name="audioFormat">
    <xs:restriction base="xs:NOTATION">
      <xs:enumeration value="m:mp3"/>
    </xs:restriction>
  </xs:simpleType>
  
  <xs:element name="audio">
    <xs:complexType>
      <xs:attribute name="format" type="m:audioFormat"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`,
			instance: `<?xml version="1.0"?>
<m:audio xmlns:m="http://example.com/media" format="m:mp3"/>`,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFS := fstest.MapFS{
				"test.xsd": &fstest.MapFile{
					Data: []byte(tt.schema),
				},
			}

			l := loader.NewLoader(loader.Config{
				FS: testFS,
			})

			schema, err := l.Load("test.xsd")
			if err != nil {
				t.Fatalf("Failed to load schema: %v", err)
			}

			v := New(mustCompile(t, schema))
			violations, err := v.ValidateStream(bytes.NewReader([]byte(tt.instance)))
			if err != nil {
				t.Fatalf("ValidateStream() error: %v", err)
			}

			if tt.wantValid {
				if len(violations) > 0 {
					t.Errorf("Expected no violations, got %d:", len(violations))
					for _, v := range violations {
						t.Errorf("  [%s] %s at %s", v.Code, v.Message, v.Path)
					}
				}
			} else {
				if len(violations) == 0 {
					t.Error("Expected violations, got none")
				} else {
					found := false
					for _, v := range violations {
						if strings.Contains(v.Message, tt.wantErrMsg) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected violation containing %q, got:", tt.wantErrMsg)
						for _, v := range violations {
							t.Errorf("  [%s] %s at %s", v.Code, v.Message, v.Path)
						}
					}
				}
			}
		})
	}
}
