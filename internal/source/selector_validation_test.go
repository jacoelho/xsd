package source

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestSelectorXPathValidation(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		errorMsg   string
		shouldFail bool
	}{
		{
			name: "selector selecting attribute should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="@number"/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "selector xpath cannot select attributes",
		},
		{
			name: "selector selecting text node should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="child::text()"/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "selector xpath cannot select text nodes",
		},
		{
			name: "selector using attribute axis should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="attribute::number"/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "selector xpath cannot use axis 'attribute::'",
		},
		{
			name: "valid selector selecting elements should pass",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="tns:parts/tns:part"/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: false,
		},
		{
			name: "valid selector with descendant-or-self should pass",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath=".//tns:part"/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: false,
		},
		{
			name: "selector with namespace axis should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="namespace::*"/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "selector xpath cannot use axis 'namespace::'",
		},
		{
			name: "selector with text() in middle should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="tns:parts/tns:part/text()"/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "selector xpath cannot select text nodes",
		},
		{
			name: "selector with attribute in middle should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="tns:parts/tns:part/@number"/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "selector xpath cannot select attributes",
		},
		{
			name: "selector with parent navigation should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="../tns:part"/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "selector xpath cannot use parent navigation",
		},
		{
			name: "selector with parent:: axis should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="parent::tns:part"/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "selector xpath cannot use parent navigation",
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
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
				}
			} else if err != nil {
				t.Errorf("Schema should have loaded successfully but got error: %v", err)
			}
		})
	}
}

// TestFieldXPathInIdentityConstraint tests field XPath validation in identity constraints
func TestFieldXPathInIdentityConstraint(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		errorMsg   string
		shouldFail bool
	}{
		{
			name: "field with wildcard should succeed",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="tns:parts"/>
      <xs:field xpath="*"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: false,
		},
		{
			name: "field with parent axis should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="tns:parts"/>
      <xs:field xpath="parent::tns:part/@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "field xpath cannot use axis 'parent::'",
		},
		{
			name: "field with ancestor axis should fail",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="tns:parts"/>
      <xs:field xpath="ancestor::tns:part/@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: true,
			errorMsg:   "field xpath cannot use axis 'ancestor::'",
		},
		{
			name: "valid field with attribute should pass",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="tns:parts"/>
      <xs:field xpath="@number"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: false,
		},
		{
			name: "valid field with child element attribute should pass",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
                  <xs:attribute name="id" type="xs:string"/>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="tns:parts"/>
      <xs:field xpath="tns:part/@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: false,
		},
		{
			name: "valid field with child axis should pass",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
                  <xs:simpleContent>
                    <xs:extension base="xs:string">
                      <xs:attribute name="number" type="xs:string"/>
                    </xs:extension>
                  </xs:simpleContent>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="tns:parts"/>
      <xs:field xpath="child::tns:part"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: false,
		},
		{
			name: "valid field with descendant-or-self should pass",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
                  <xs:simpleContent>
                    <xs:extension base="xs:string">
                      <xs:attribute name="number" type="xs:string"/>
                    </xs:extension>
                  </xs:simpleContent>
                </xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="partKey">
      <xs:selector xpath="tns:parts"/>
      <xs:field xpath=".//tns:part"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: false,
		},
		{
			name: "selector with wildcard should pass (wildcards allowed in selectors)",
			schema: `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="http://example.com"
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
      <xs:selector xpath="tns:parts/*"/>
      <xs:field xpath="@number"/>
    </xs:key>
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
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
				}
			} else if err != nil {
				t.Errorf("Schema should have loaded successfully but got error: %v", err)
			}
		})
	}
}
