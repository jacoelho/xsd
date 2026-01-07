package loader

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

func TestDuplicateConstraintNameValidation(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		shouldFail bool
		errorMsg   string
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

// TestDuplicateConstraintNameDirect tests validateElementDeclStructure directly
func TestDuplicateConstraintNameDirect(t *testing.T) {
	schema := &schema.Schema{
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
					XPath: "parts/part",
				},
				Fields: []types.Field{
					{XPath: "@number"},
				},
			},
			{
				Name: "partKey", // duplicate name
				Type: types.KeyConstraint,
				Selector: types.Selector{
					XPath: "parts/part",
				},
				Fields: []types.Field{
					{XPath: "@id"},
				},
			},
		},
	}

	elementQName := types.QName{
		Namespace: "http://example.com",
		Local:     "purchaseReport",
	}

	err := validateElementDeclStructure(schema, elementQName, elementDecl)
	if err == nil {
		t.Error("validateElementDeclStructure should have failed for duplicate constraint names")
	} else {
		if !strings.Contains(err.Error(), "duplicate") {
			t.Errorf("Expected error to mention 'duplicate', got: %v", err)
		}
		if !strings.Contains(err.Error(), "partKey") {
			t.Errorf("Expected error to mention constraint name 'partKey', got: %v", err)
		}
	}

	// test with unique constraint names (should pass)
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
					XPath: "parts/part",
				},
				Fields: []types.Field{
					{XPath: "@number"},
				},
			},
			{
				Name: "regionKey", // different name - should pass
				Type: types.UniqueConstraint,
				Selector: types.Selector{
					XPath: "regions/region",
				},
				Fields: []types.Field{
					{XPath: "@code"},
				},
			},
		},
	}

	elementQName2 := types.QName{
		Namespace: "http://example.com",
		Local:     "purchaseReport2",
	}

	err = validateElementDeclStructure(schema, elementQName2, elementDecl2)
	if err != nil {
		t.Errorf("validateElementDeclStructure should have passed for unique constraint names, got error: %v", err)
	}
}