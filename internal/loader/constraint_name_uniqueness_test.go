package loader

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestDuplicateConstraintNameValidation(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		errorMsg   string
		shouldFail bool
	}{
		{
			name: "duplicate key constraint names",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="purchaseReport">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="parts">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="part" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="number" type="xs:string"/>
                  <xs:attribute name="id" type="xs:string"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="parts/part"/>
      <xs:field xpath="@number"/>
    </xs:key>
    <xs:key name="partKey">
      <xs:selector xpath="parts/part"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "duplicate identity constraint name 'partKey'",
		},
		{
			name: "different constraint types with same name",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="purchaseReport">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="parts">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="part" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="number" type="xs:string"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="parts/part"/>
      <xs:field xpath="@number"/>
    </xs:key>
    <xs:unique name="partKey">
      <xs:selector xpath="parts/part"/>
      <xs:field xpath="@number"/>
    </xs:unique>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "duplicate identity constraint name 'partKey'",
		},
		{
			name: "unique constraint names should pass",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="http://example.com"
           elementFormDefault="qualified">
  <xs:element name="purchaseReport">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="parts">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="part" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="number" type="xs:string"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
        <xs:element name="regions">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="region" maxOccurs="unbounded">
                <xs:complexType>
                  <xs:attribute name="code" type="xs:string"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="parts/part"/>
      <xs:field xpath="@number"/>
    </xs:key>
    <xs:unique name="regionKey">
      <xs:selector xpath="regions/region"/>
      <xs:field xpath="@code"/>
    </xs:unique>
  </xs:element>
</xs:schema>`,
			shouldFail: false,
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

			if tt.shouldFail {
				if err == nil {
					t.Error("Schema loading should have failed but succeeded")
				} else {
					// verify the error mentions duplicate constraint name
					if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
						t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
					}
					// verify it mentions duplicate
					if !strings.Contains(err.Error(), "duplicate") {
						t.Errorf("Expected error to mention 'duplicate', got: %v", err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Schema should have loaded successfully but got error: %v", err)
				}
			}
		})
	}
}
