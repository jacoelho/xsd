package loader

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
)

func TestSelectorXPathValidation(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		shouldFail bool
		errorMsg   string
	}{
		{
			name: "selector selecting attribute should fail",
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
  </xs:element>
</xs:schema>`,
			shouldFail: false,
		},
		{
			name: "valid selector with descendant-or-self should pass",
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
      <xs:selector xpath=".//part"/>
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
      <xs:selector xpath="parts/part/text()"/>
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
      <xs:selector xpath="parts/part/@number"/>
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
      <xs:selector xpath="../part"/>
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
      <xs:selector xpath="parent::part"/>
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
				} else {
					// verify the error message
					if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
						t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
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

// TestValidateSelectorXPathDirect tests schemacheck.ValidateSelectorXPath function directly
func TestValidateSelectorXPathDirect(t *testing.T) {
	tests := []struct {
		name      string
		xpath     string
		shouldErr bool
		errorMsg  string
	}{
		{
			name:      "empty xpath should fail",
			xpath:     "",
			shouldErr: true,
			errorMsg:  "selector xpath cannot be empty",
		},
		{
			name:      "whitespace only should fail",
			xpath:     "   ",
			shouldErr: true,
			errorMsg:  "selector xpath cannot be empty",
		},
		{
			name:      "attribute selection at start should fail",
			xpath:     "@number",
			shouldErr: true,
			errorMsg:  "selector xpath cannot select attributes",
		},
		{
			name:      "attribute selection in middle should fail",
			xpath:     "parts/part/@number",
			shouldErr: true,
			errorMsg:  "selector xpath cannot select attributes",
		},
		{
			name:      "text node selection should fail",
			xpath:     "child::text()",
			shouldErr: true,
			errorMsg:  "selector xpath cannot select text nodes",
		},
		{
			name:      "text() at end should fail",
			xpath:     "text()",
			shouldErr: true,
			errorMsg:  "selector xpath cannot select text nodes",
		},
		{
			name:      "text() in middle should fail",
			xpath:     "parts/part/text()",
			shouldErr: true,
			errorMsg:  "selector xpath cannot select text nodes",
		},
		{
			name:      "parent navigation with .. should fail",
			xpath:     "../part",
			shouldErr: true,
			errorMsg:  "selector xpath cannot use parent navigation",
		},
		{
			name:      "parent navigation with .. in middle should fail",
			xpath:     "parts/../part",
			shouldErr: true,
			errorMsg:  "selector xpath cannot use parent navigation",
		},
		{
			name:      "parent navigation with parent:: should fail",
			xpath:     "parent::part",
			shouldErr: true,
			errorMsg:  "selector xpath cannot use parent navigation",
		},
		{
			name:      "valid element selection should pass",
			xpath:     "parts/part",
			shouldErr: false,
		},
		{
			name:      "valid descendant-or-self selection should pass",
			xpath:     ".//part",
			shouldErr: false,
		},
		{
			name:      "valid child axis should pass",
			xpath:     "child::part",
			shouldErr: false,
		},
		{
			name:      "valid descendant-or-self prefix should pass with whitespace",
			xpath:     ".// part",
			shouldErr: false,
		},
		{
			name:      "valid with wildcard should pass",
			xpath:     "parts/*",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := schemacheck.ValidateSelectorXPath(tt.xpath)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("schemacheck.ValidateSelectorXPath(%q) should have failed but succeeded", tt.xpath)
				} else {
					if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
						t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("schemacheck.ValidateSelectorXPath(%q) should have succeeded but got error: %v", tt.xpath, err)
				}
			}
		})
	}
}

// TestSelectorXPathInIdentityConstraint tests selector validation in identity constraints
func TestSelectorXPathInIdentityConstraint(t *testing.T) {
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
		ElementDecls:    make(map[types.QName]*types.ElementDecl),
	}

	complexType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "PurchaseReportType",
		},
	}
	complexType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind:      types.Sequence,
			MinOccurs: 1,
			MaxOccurs: 1,
		},
	})
	complexTypeQName := types.QName{
		Namespace: "http://example.com",
		Local:     "PurchaseReportType",
	}
	schema.TypeDefs[complexTypeQName] = complexType

	// test invalid selector - attribute selection
	elementDecl := &types.ElementDecl{
		Name: types.QName{
			Namespace: "http://example.com",
			Local:     "purchaseReport",
		},
		Type: complexType,
		Constraints: []*types.IdentityConstraint{
			{
				Name: "partKey",
				Type: types.KeyConstraint,
				Selector: types.Selector{
					XPath: "@number", // invalid - selects attribute
				},
				Fields: []types.Field{
					{XPath: "@number"},
				},
			},
		},
	}

	elementQName := types.QName{
		Namespace: "http://example.com",
		Local:     "purchaseReport",
	}

	err := schemacheck.ValidateElementDeclStructure(schema, elementQName, elementDecl)
	if err == nil {
		t.Error("schemacheck.ValidateElementDeclStructure should have failed for attribute selector")
	} else {
		if !strings.Contains(err.Error(), "selector xpath cannot select attributes") {
			t.Errorf("Expected error to mention attribute selection, got: %v", err)
		}
	}

	// test invalid selector - text node selection
	elementDecl2 := &types.ElementDecl{
		Name: types.QName{
			Namespace: "http://example.com",
			Local:     "purchaseReport2",
		},
		Type: complexType,
		Constraints: []*types.IdentityConstraint{
			{
				Name: "partKey",
				Type: types.KeyConstraint,
				Selector: types.Selector{
					XPath: "child::text()", // invalid - selects text
				},
				Fields: []types.Field{
					{XPath: "@number"},
				},
			},
		},
	}

	elementQName2 := types.QName{
		Namespace: "http://example.com",
		Local:     "purchaseReport2",
	}

	err = schemacheck.ValidateElementDeclStructure(schema, elementQName2, elementDecl2)
	if err == nil {
		t.Error("schemacheck.ValidateElementDeclStructure should have failed for text node selector")
	} else {
		if !strings.Contains(err.Error(), "selector xpath cannot select text nodes") {
			t.Errorf("Expected error to mention text node selection, got: %v", err)
		}
	}

	// test valid selector - element selection
	elementDecl3 := &types.ElementDecl{
		Name: types.QName{
			Namespace: "http://example.com",
			Local:     "purchaseReport3",
		},
		Type: complexType,
		Constraints: []*types.IdentityConstraint{
			{
				Name: "partKey",
				Type: types.KeyConstraint,
				Selector: types.Selector{
					XPath: "parts/part", // valid - selects elements
				},
				Fields: []types.Field{
					{XPath: "@number"},
				},
			},
		},
	}

	elementQName3 := types.QName{
		Namespace: "http://example.com",
		Local:     "purchaseReport3",
	}

	err = schemacheck.ValidateElementDeclStructure(schema, elementQName3, elementDecl3)
	if err != nil {
		t.Errorf("schemacheck.ValidateElementDeclStructure should have passed for valid element selector, got error: %v", err)
	}
}

// TestValidateFieldXPathDirect tests schemacheck.ValidateFieldXPath function directly
func TestValidateFieldXPathDirect(t *testing.T) {
	tests := []struct {
		name      string
		xpath     string
		shouldErr bool
		errorMsg  string
	}{
		{
			name:      "empty xpath should fail",
			xpath:     "",
			shouldErr: true,
			errorMsg:  "field xpath cannot be empty",
		},
		{
			name:      "whitespace only should fail",
			xpath:     "   ",
			shouldErr: true,
			errorMsg:  "field xpath cannot be empty",
		},
		{
			name:      "wildcard alone should succeed",
			xpath:     "*",
			shouldErr: false,
		},
		{
			name:      "wildcard with child axis should succeed",
			xpath:     "child::*",
			shouldErr: false,
		},
		{
			name:      "wildcard with descendant-or-self prefix should succeed",
			xpath:     ".//*",
			shouldErr: false,
		},
		{
			name:      "wildcard with descendant-or-self prefix should succeed with whitespace",
			xpath:     ".// *",
			shouldErr: false,
		},
		{
			name:      "wildcard in path should succeed",
			xpath:     "part/*",
			shouldErr: false,
		},
		{
			name:      "wildcard at end should succeed",
			xpath:     "part/*",
			shouldErr: false,
		},
		{
			name:      "parent axis should fail",
			xpath:     "parent::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'parent::'",
		},
		{
			name:      "ancestor axis should fail",
			xpath:     "ancestor::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'ancestor::'",
		},
		{
			name:      "ancestor-or-self axis should fail",
			xpath:     "ancestor-or-self::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'ancestor-or-self::'",
		},
		{
			name:      "following axis should fail",
			xpath:     "following::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'following::'",
		},
		{
			name:      "following-sibling axis should fail",
			xpath:     "following-sibling::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'following-sibling::'",
		},
		{
			name:      "preceding axis should fail",
			xpath:     "preceding::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'preceding::'",
		},
		{
			name:      "preceding-sibling axis should fail",
			xpath:     "preceding-sibling::part",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'preceding-sibling::'",
		},
		{
			name:      "namespace axis should fail",
			xpath:     "namespace::prefix",
			shouldErr: true,
			errorMsg:  "field xpath cannot use axis 'namespace::'",
		},
		{
			name:      "valid attribute should pass",
			xpath:     "@number",
			shouldErr: false,
		},
		{
			name:      "valid child element should pass",
			xpath:     "part",
			shouldErr: false,
		},
		{
			name:      "valid child axis should pass",
			xpath:     "child::part",
			shouldErr: false,
		},
		{
			name:      "valid descendant-or-self prefix should pass",
			xpath:     ".//part",
			shouldErr: false,
		},
		{
			name:      "valid descendant-or-self prefix should pass with whitespace",
			xpath:     ".// part",
			shouldErr: false,
		},
		{
			name:      "valid attribute axis should pass",
			xpath:     "attribute::number",
			shouldErr: false,
		},
		{
			name:      "valid child element attribute should pass",
			xpath:     "part/@id",
			shouldErr: false,
		},
		{
			name:      "valid path with child element should pass",
			xpath:     "parts/part",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := schemacheck.ValidateFieldXPath(tt.xpath)

			if tt.shouldErr {
				if err == nil {
					t.Errorf("schemacheck.ValidateFieldXPath(%q) should have failed but succeeded", tt.xpath)
				} else {
					if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
						t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("schemacheck.ValidateFieldXPath(%q) should have succeeded but got error: %v", tt.xpath, err)
				}
			}
		})
	}
}

// TestFieldXPathInIdentityConstraint tests field XPath validation in identity constraints
func TestFieldXPathInIdentityConstraint(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		shouldFail bool
		errorMsg   string
	}{
		{
			name: "field with wildcard should succeed",
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
      <xs:selector xpath="parts"/>
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
      <xs:selector xpath="parts"/>
      <xs:field xpath="parent::part/@number"/>
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
      <xs:selector xpath="parts"/>
      <xs:field xpath="ancestor::part/@number"/>
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
      <xs:selector xpath="parts"/>
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
      <xs:selector xpath="parts"/>
      <xs:field xpath="part/@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: false,
		},
		{
			name: "valid field with child axis should pass",
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
      <xs:selector xpath="parts"/>
      <xs:field xpath="child::part"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: false,
		},
		{
			name: "valid field with descendant-or-self should pass",
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
      <xs:selector xpath="parts"/>
      <xs:field xpath=".//part"/>
    </xs:key>
  </xs:element>
</xs:schema>`,
			shouldFail: false,
		},
		{
			name: "selector with wildcard should pass (wildcards allowed in selectors)",
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
      <xs:selector xpath="parts/*"/>
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
				} else {
					// verify the error message
					if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
						t.Errorf("Expected error to contain %q, got: %v", tt.errorMsg, err)
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
