package source

import (
	"strings"
	"testing"
	"testing/fstest"
)

// TestValidateElementDeclarationsConsistent tests the "Element Declarations Consistent" constraint
// According to XSD spec: when extending a complex type, elements in the extension cannot have
// the same name as elements in the base type with different types.
func TestValidateElementDeclarationsConsistent(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		errMsg  string
		wantErr bool
	}{
		{
			name: "valid extension with different element names",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType">
					<xs:sequence>
						<xs:element name="child1" type="xs:integer"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="ExtendedType">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:element name="child2" type="xs:string"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: false,
		},
		{
			name: "invalid extension with same element name but different type",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType">
					<xs:sequence>
						<xs:element name="child1" type="xs:integer"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="ExtendedType">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:element name="child1" type="xs:date"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "Element Declarations Consistent",
		},
		{
			name: "valid extension with same element name and same type",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType">
					<xs:sequence>
						<xs:element name="child1" type="xs:integer"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="ExtendedType">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:element name="child1" type="xs:integer"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: false,
		},
		{
			name: "extension with nested groups - element conflict",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType">
					<xs:sequence>
						<xs:element name="child1" type="xs:integer"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="ExtendedType">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:choice>
									<xs:element name="child1" type="xs:date"/>
								</xs:choice>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "Element Declarations Consistent",
		},
		{
			name: "extension from derived type - checks base chain",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType">
					<xs:sequence>
						<xs:element name="child1" type="xs:integer"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="MiddleType">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:element name="child2" type="xs:string"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
				<xs:complexType name="ExtendedType">
					<xs:complexContent>
						<xs:extension base="MiddleType">
							<xs:sequence>
								<xs:element name="child1" type="xs:date"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "Element Declarations Consistent",
		},
		{
			name: "restriction element type must be derived from base element type",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType">
					<xs:sequence>
						<xs:element name="child1" type="xs:integer" minOccurs="0"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="RestrictedType">
					<xs:complexContent>
						<xs:restriction base="BaseType">
							<xs:sequence>
								<xs:element name="child1" type="xs:date"/>
							</xs:sequence>
						</xs:restriction>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true, // per XSD 1.0 spec: restriction element type must be derived from base element type
			errMsg:  "type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"test.xsd": &fstest.MapFile{
					Data: []byte(tt.schema),
				},
			}
			cfg := Config{
				FS: fsys,
			}
			l := NewLoader(cfg)
			_, err := loadAndPrepare(t, l, "test.xsd")

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected schema validation error but got none")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected schema to be valid but got error: %v", err)
				}
			}
		})
	}
}

// TestValidateGroupOccurrenceConstraints tests that groups cannot have
// minOccurs="0" or maxOccurs="unbounded" directly.
func TestValidateGroupOccurrenceConstraints(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		errMsg  string
		wantErr bool
	}{
		{
			name: "valid group with default occurrences",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:group name="validGroup">
					<xs:sequence>
						<xs:element name="a" type="xs:string"/>
					</xs:sequence>
				</xs:group>
			</xs:schema>`,
			wantErr: false,
		},
		{
			name: "invalid group with minOccurs=0",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:group name="invalidGroup">
					<xs:sequence minOccurs="0">
						<xs:element name="a" type="xs:string"/>
					</xs:sequence>
				</xs:group>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "minOccurs='0'",
		},
		{
			name: "invalid group with maxOccurs=unbounded",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:group name="invalidGroup">
					<xs:sequence maxOccurs="unbounded">
						<xs:element name="a" type="xs:string"/>
					</xs:sequence>
				</xs:group>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "maxOccurs='unbounded'",
		},
		{
			name: "invalid group with minOccurs=2",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:group name="invalidGroup">
					<xs:sequence minOccurs="2" maxOccurs="1">
						<xs:element name="a" type="xs:string"/>
					</xs:sequence>
				</xs:group>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "minOccurs='1' and maxOccurs='1'",
		},
		{
			name: "invalid group with maxOccurs=2",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:group name="invalidGroup">
					<xs:sequence minOccurs="1" maxOccurs="2">
						<xs:element name="a" type="xs:string"/>
					</xs:sequence>
				</xs:group>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "minOccurs='1' and maxOccurs='1'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"test.xsd": &fstest.MapFile{
					Data: []byte(tt.schema),
				},
			}
			cfg := Config{
				FS: fsys,
			}
			l := NewLoader(cfg)
			_, err := loadAndPrepare(t, l, "test.xsd")

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected schema validation error but got none")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected schema to be valid but got error: %v", err)
				}
			}
		})
	}
}

// TestValidateMixedContentDerivation checks mixed-content derivation constraints.
// It rejects element-only and mixed-content mismatches per XSD rules.
func TestValidateMixedContentDerivation(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		errMsg  string
		wantErr bool
	}{
		{
			name: "valid extension - both element-only",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType">
					<xs:sequence>
						<xs:element name="child1" type="xs:string"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="ExtendedType">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:element name="child2" type="xs:string"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: false,
		},
		{
			name: "valid extension - both mixed",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType" mixed="true">
					<xs:choice minOccurs="0" maxOccurs="unbounded">
						<xs:element name="child1" type="xs:string"/>
					</xs:choice>
				</xs:complexType>
				<xs:complexType name="ExtendedType" mixed="true">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:element name="child2" type="xs:string"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: false,
		},
		{
			name: "valid extension - base mixed, no extension content",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType" mixed="true">
					<xs:sequence>
						<xs:element name="child1" type="xs:string"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="ExtendedType">
					<xs:complexContent>
						<xs:extension base="BaseType"/>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: false,
		},
		{
			name: "invalid extension - base mixed, derived element-only",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType" mixed="true">
					<xs:choice minOccurs="0" maxOccurs="unbounded">
						<xs:element name="child1" type="xs:string"/>
					</xs:choice>
				</xs:complexType>
				<xs:complexType name="ExtendedType">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:element name="child2" type="xs:string"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "mixed content",
		},
		{
			name: "invalid extension - base element-only, derived mixed",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType">
					<xs:sequence>
						<xs:element name="child1" type="xs:string"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="ExtendedType" mixed="true">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:element name="child2" type="xs:string"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "mixed content",
		},
		{
			name: "restriction does not check mixed content consistency",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType" mixed="true">
					<xs:sequence>
						<xs:element name="child1" type="xs:string" minOccurs="0"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="RestrictedType" mixed="true">
					<xs:complexContent>
						<xs:restriction base="BaseType">
							<xs:sequence>
								<xs:element name="child1" type="xs:string"/>
							</xs:sequence>
						</xs:restriction>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: false, // restrictions have different rules, and mixed content can be the same
		},
		{
			name: "invalid extension from derived type - base chain with mixed to element-only",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType" mixed="true">
					<xs:choice minOccurs="0" maxOccurs="unbounded">
						<xs:element name="child1" type="xs:string"/>
					</xs:choice>
				</xs:complexType>
				<xs:complexType name="MiddleType" mixed="true">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:element name="child2" type="xs:string"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
				<xs:complexType name="ExtendedType">
					<xs:complexContent>
						<xs:extension base="MiddleType">
							<xs:sequence>
								<xs:element name="child3" type="xs:string"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "mixed content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"test.xsd": &fstest.MapFile{
					Data: []byte(tt.schema),
				},
			}
			cfg := Config{
				FS: fsys,
			}
			l := NewLoader(cfg)
			_, err := loadAndPrepare(t, l, "test.xsd")

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected schema validation error but got none")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected schema to be valid but got error: %v", err)
				}
			}
		})
	}
}

// TestValidateComplexTypeStructureIntegration tests the full integration
// of complex type validation with all constraints.
func TestValidateComplexTypeStructureIntegration(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		errMsg  string
		wantErr bool
	}{
		{
			name: "valid complex type with all constraints satisfied",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="ValidType">
					<xs:sequence>
						<xs:element name="child1" type="xs:string"/>
						<xs:element name="child2" type="xs:integer"/>
					</xs:sequence>
					<xs:attribute name="attr1" type="xs:string"/>
				</xs:complexType>
			</xs:schema>`,
			wantErr: false,
		},
		{
			name: "invalid - element declarations consistent violation",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType">
					<xs:sequence>
						<xs:element name="child1" type="xs:integer"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="ExtendedType">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:element name="child1" type="xs:date"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "Element Declarations Consistent",
		},
		{
			name: "invalid - extension from mixed to element-only",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType" mixed="true">
					<xs:choice minOccurs="0" maxOccurs="unbounded">
						<xs:element name="child1" type="xs:string"/>
					</xs:choice>
				</xs:complexType>
				<xs:complexType name="ExtendedType">
					<xs:complexContent>
						<xs:extension base="BaseType">
							<xs:sequence>
								<xs:element name="child2" type="xs:string"/>
							</xs:sequence>
						</xs:extension>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "mixed content",
		},
		{
			name: "invalid - restriction from element-only to mixed",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BaseType">
					<xs:sequence>
						<xs:element name="child1" type="xs:string"/>
					</xs:sequence>
				</xs:complexType>
				<xs:complexType name="RestrictedType" mixed="true">
					<xs:complexContent>
						<xs:restriction base="BaseType">
							<xs:sequence>
								<xs:element name="child1" type="xs:string"/>
							</xs:sequence>
						</xs:restriction>
					</xs:complexContent>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "mixed",
		},
		{
			name: "invalid - group occurrence constraint violation",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:group name="InvalidGroup">
					<xs:sequence minOccurs="0">
						<xs:element name="a" type="xs:string"/>
					</xs:sequence>
				</xs:group>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "minOccurs='0'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := fstest.MapFS{
				"test.xsd": &fstest.MapFile{
					Data: []byte(tt.schema),
				},
			}
			cfg := Config{
				FS: fsys,
			}
			l := NewLoader(cfg)
			_, err := loadAndPrepare(t, l, "test.xsd")

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected schema validation error but got none")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error to contain %q, got: %v", tt.errMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected schema to be valid but got error: %v", err)
				}
			}
		})
	}
}

// TestAllGroupConstraints tests XSD 1.0 constraints on xs:all model groups
func TestAllGroupConstraints(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		errMsg  string
		wantErr bool
	}{
		{
			name: "valid xs:all with default occurrences",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="AllType">
					<xs:all>
						<xs:element name="a" type="xs:string"/>
						<xs:element name="b" type="xs:string"/>
					</xs:all>
				</xs:complexType>
			</xs:schema>`,
			wantErr: false,
		},
		{
			name: "valid xs:all with minOccurs=0",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="AllType">
					<xs:all minOccurs="0">
						<xs:element name="a" type="xs:string"/>
						<xs:element name="b" type="xs:string"/>
					</xs:all>
				</xs:complexType>
			</xs:schema>`,
			wantErr: false,
		},
		{
			name: "invalid xs:all with maxOccurs > 1",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="AllType">
					<xs:all maxOccurs="2">
						<xs:element name="a" type="xs:string"/>
					</xs:all>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "xs:all must have maxOccurs='1'",
		},
		{
			name: "invalid xs:all with minOccurs > 1",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="AllType">
					<xs:all minOccurs="2">
						<xs:element name="a" type="xs:string"/>
					</xs:all>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "xs:all must have minOccurs='0' or '1'",
		},
		{
			name: "invalid xs:all member with maxOccurs > 1",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="AllType">
					<xs:all>
						<xs:element name="a" type="xs:string" maxOccurs="2"/>
					</xs:all>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "maxOccurs <= 1",
		},
		{
			name: "valid xs:all member with minOccurs=0",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="AllType">
					<xs:all>
						<xs:element name="a" type="xs:string" minOccurs="0"/>
						<xs:element name="b" type="xs:string"/>
					</xs:all>
				</xs:complexType>
			</xs:schema>`,
			wantErr: false,
		},
		{
			name: "invalid xs:all inside xs:sequence",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BadType">
					<xs:sequence>
						<xs:all>
							<xs:element name="a" type="xs:string"/>
						</xs:all>
					</xs:sequence>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "xs:all cannot appear as a child",
		},
		{
			name: "invalid xs:all inside xs:choice",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
				<xs:complexType name="BadType">
					<xs:choice>
						<xs:all>
							<xs:element name="a" type="xs:string"/>
						</xs:all>
					</xs:choice>
				</xs:complexType>
			</xs:schema>`,
			wantErr: true,
			errMsg:  "xs:all cannot appear as a child",
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

			_, err := loadAndPrepare(t, loader, "test.xsd")

			if tt.wantErr {
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

func TestComplexContentExtensionFromSimpleContentBase(t *testing.T) {
	schemaXML := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
		<xs:complexType name="BaseType">
			<xs:simpleContent>
				<xs:extension base="xs:string">
					<xs:attribute name="field1" type="xs:string"/>
				</xs:extension>
			</xs:simpleContent>
		</xs:complexType>
		<xs:complexType name="DerivedType">
			<xs:complexContent>
				<xs:extension base="BaseType">
					<xs:attribute name="field2" type="xs:string"/>
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

	if _, err := loadAndPrepare(t, loader, "test.xsd"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
