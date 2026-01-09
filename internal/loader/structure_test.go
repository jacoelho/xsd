package loader

import (
	"strings"
	"testing"
	"testing/fstest"

	schema "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/validation"
)

// TestCircularDerivation_TrueCycle tests that a true circular derivation (A -> B -> A) is correctly detected
func TestCircularDerivation_TrueCycle(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// type A extends Type B
	typeA := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "TypeA",
		},
	}
	typeA.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "TypeB",
			},
		},
	})
	schema.TypeDefs[typeA.QName] = typeA

	// type B extends Type A (circular)
	typeB := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "TypeB",
		},
	}
	typeB.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "TypeA",
			},
		},
	})
	schema.TypeDefs[typeB.QName] = typeB

	// should detect circular derivation
	err := validation.ValidateNoCircularDerivation(schema, typeA)
	if err == nil {
		t.Error("Should detect circular derivation for TypeA -> TypeB -> TypeA")
	}
	if err != nil && !strings.Contains(err.Error(), "circular derivation") {
		t.Errorf("Error should mention circular derivation, got: %v", err)
	}
}

// TestCircularDerivation_ValidDeepHierarchy tests that a valid deep hierarchy (A -> B -> C -> anyType) is NOT flagged as circular
func TestCircularDerivation_ValidDeepHierarchy(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// type C extends anyType (built-in)
	typeC := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "TypeC",
		},
	}
	typeC.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: types.XSDNamespace,
				Local:     "anyType",
			},
		},
	})
	schema.TypeDefs[typeC.QName] = typeC

	// type B extends Type C
	typeB := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "TypeB",
		},
	}
	typeB.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "TypeC",
			},
		},
	})
	schema.TypeDefs[typeB.QName] = typeB

	// type A extends Type B
	typeA := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "TypeA",
		},
	}
	typeA.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "TypeB",
			},
		},
	})
	schema.TypeDefs[typeA.QName] = typeA

	// should NOT detect circular derivation
	err := validation.ValidateNoCircularDerivation(schema, typeA)
	if err != nil {
		t.Errorf("Should NOT detect circular derivation for valid hierarchy A -> B -> C -> anyType, got: %v", err)
	}
}

// TestCircularDerivation_MultipleTypesFromSameBase tests that multiple types deriving from the same base is NOT flagged as circular
func TestCircularDerivation_MultipleTypesFromSameBase(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// base type extends anyType
	baseType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
	}
	baseType.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: types.XSDNamespace,
				Local:     "anyType",
			},
		},
	})
	schema.TypeDefs[baseType.QName] = baseType

	// type A extends BaseType
	typeA := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "TypeA",
		},
	}
	typeA.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "BaseType",
			},
		},
	})
	schema.TypeDefs[typeA.QName] = typeA

	// type B extends BaseType (same base as TypeA)
	typeB := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "TypeB",
		},
	}
	typeB.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "BaseType",
			},
		},
	})
	schema.TypeDefs[typeB.QName] = typeB

	// should NOT detect circular derivation for either type
	err := validation.ValidateNoCircularDerivation(schema, typeA)
	if err != nil {
		t.Errorf("Should NOT detect circular derivation for TypeA -> BaseType, got: %v", err)
	}

	err = validation.ValidateNoCircularDerivation(schema, typeB)
	if err != nil {
		t.Errorf("Should NOT detect circular derivation for TypeB -> BaseType, got: %v", err)
	}
}

// TestCircularDerivation_RedefineSelfExtension tests that a type extending itself in a redefine context is NOT flagged as circular
// This is the ipo4 case: AddressType is redefined to extend itself (referring to the old definition)
func TestCircularDerivation_RedefineSelfExtension(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// original AddressType (from base schema)
	originalAddressType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "AddressType",
		},
	}
	originalAddressType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind: types.Sequence,
			Particles: []types.Particle{
				&types.ElementDecl{
					Name: types.QName{Local: "name"},
					Type: types.GetBuiltin(types.TypeName("string")),
				},
			},
		},
	})
	schema.TypeDefs[originalAddressType.QName] = originalAddressType

	// redefined AddressType extends itself (valid in redefine context)
	redefinedAddressType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "AddressType",
		},
	}
	redefinedAddressType.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "AddressType",
			},
		},
	})
	// replace the original with the redefined version
	schema.TypeDefs[redefinedAddressType.QName] = redefinedAddressType

	// should NOT detect circular derivation (in redefine, extending self is valid)
	err := validation.ValidateNoCircularDerivation(schema, redefinedAddressType)
	if err != nil {
		t.Errorf("Should NOT detect circular derivation for redefined type extending itself, got: %v", err)
	}

	// also test that types extending the redefined AddressType work correctly
	usAddress := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "USAddress",
		},
	}
	usAddress.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "AddressType",
			},
		},
	})
	schema.TypeDefs[usAddress.QName] = usAddress

	err = validation.ValidateNoCircularDerivation(schema, usAddress)
	if err != nil {
		t.Errorf("Should NOT detect circular derivation for USAddress -> AddressType, got: %v", err)
	}
}

// Helper function to check if a string contains a substring
// TestMixedContentDerivation_ExtensionFromMixedToElementOnly tests that extension from mixed content
// to element-only content with additional particles is INVALID.
func TestMixedContentDerivation_ExtensionFromMixedToElementOnly(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// base type with mixed content
	baseType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "MixedBaseType",
		},
	}
	baseType.SetMixed(true)
	baseType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind: types.Sequence,
			Particles: []types.Particle{
				&types.ElementDecl{
					Name: types.QName{Local: "child"},
					Type: types.GetBuiltin(types.TypeName("string")),
				},
			},
		},
	})
	schema.TypeDefs[baseType.QName] = baseType

	// derived type extending base with element-only content (removing mixed)
	derivedType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "ElementOnlyType",
		},
		DerivationMethod: types.DerivationExtension,
	}
	derivedType.SetMixed(false) // element-only
	derivedType.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "MixedBaseType",
			},
			Particle: &types.ModelGroup{
				Kind: types.Sequence,
				Particles: []types.Particle{
					&types.ElementDecl{
						Name: types.QName{Local: "extra"},
						Type: types.GetBuiltin(types.TypeName("string")),
					},
				},
			},
		},
	})
	schema.TypeDefs[derivedType.QName] = derivedType

	// should be INVALID - extension must preserve mixed content
	err := validation.ValidateMixedContentDerivation(schema, derivedType)
	if err == nil {
		t.Error("Extension from mixed to element-only should be INVALID, but got no error")
	}
	if err != nil && !strings.Contains(err.Error(), "mixed") && !strings.Contains(err.Error(), "element-only") {
		t.Errorf("Error should mention mixed or element-only content, got: %v", err)
	}
}

func TestMixedContentDerivation_ExtensionFromMixedToElementOnlyNoParticle(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	baseType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "MixedBaseType",
		},
	}
	baseType.SetMixed(true)
	baseType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind: types.Sequence,
			Particles: []types.Particle{
				&types.ElementDecl{
					Name: types.QName{Local: "child"},
					Type: types.GetBuiltin(types.TypeName("string")),
				},
			},
		},
	})
	schema.TypeDefs[baseType.QName] = baseType

	derivedType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "ElementOnlyType",
		},
		DerivationMethod: types.DerivationExtension,
	}
	derivedType.SetMixed(false)
	derivedType.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "MixedBaseType",
			},
		},
	})
	schema.TypeDefs[derivedType.QName] = derivedType

	err := validation.ValidateMixedContentDerivation(schema, derivedType)
	if err != nil {
		t.Errorf("Extension with no particle should inherit mixed content, got error: %v", err)
	}
}

// TestMixedContentDerivation_RestrictionFromElementOnlyToMixed tests that restriction from element-only to mixed content is INVALID
func TestMixedContentDerivation_RestrictionFromElementOnlyToMixed(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// base type with element-only content
	baseType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "ElementOnlyBaseType",
		},
	}
	baseType.SetMixed(false)
	baseType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind: types.Sequence,
			Particles: []types.Particle{
				&types.ElementDecl{
					Name: types.QName{Local: "child"},
					Type: types.GetBuiltin(types.TypeName("string")),
				},
			},
		},
	})
	schema.TypeDefs[baseType.QName] = baseType

	// derived type restricting base to mixed content (adding mixed)
	derivedType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "MixedType",
		},
		DerivationMethod: types.DerivationRestriction,
	}
	derivedType.SetMixed(true) // mixed
	derivedType.SetContent(&types.ComplexContent{
		Mixed: true,
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "ElementOnlyBaseType",
			},
		},
	})
	schema.TypeDefs[derivedType.QName] = derivedType

	// should be INVALID - restriction cannot add mixed content
	err := validation.ValidateMixedContentDerivation(schema, derivedType)
	if err == nil {
		t.Error("Restriction from element-only to mixed should be INVALID, but got no error")
	}
	if err != nil && !strings.Contains(err.Error(), "mixed") {
		t.Errorf("Error should mention mixed content, got: %v", err)
	}
}

// TestMixedContentDerivation_ExtensionFromElementOnlyToElementOnly tests that extension from element-only to element-only is VALID
func TestMixedContentDerivation_ExtensionFromElementOnlyToElementOnly(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// base type with element-only content
	baseType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "ElementOnlyBaseType",
		},
	}
	baseType.SetMixed(false)
	baseType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind: types.Sequence,
			Particles: []types.Particle{
				&types.ElementDecl{
					Name: types.QName{Local: "child"},
					Type: types.GetBuiltin(types.TypeName("string")),
				},
			},
		},
	})
	schema.TypeDefs[baseType.QName] = baseType

	// derived type extending base with element-only content
	derivedType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "ElementOnlyDerivedType",
		},
		DerivationMethod: types.DerivationExtension,
	}
	derivedType.SetMixed(false) // element-only
	derivedType.SetContent(&types.ComplexContent{
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "ElementOnlyBaseType",
			},
		},
	})
	schema.TypeDefs[derivedType.QName] = derivedType

	// should be VALID
	err := validation.ValidateMixedContentDerivation(schema, derivedType)
	if err != nil {
		t.Errorf("Extension from element-only to element-only should be VALID, got error: %v", err)
	}
}

// TestMixedContentDerivation_ExtensionFromMixedToMixed tests that extension from mixed to mixed is VALID
func TestMixedContentDerivation_ExtensionFromMixedToMixed(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// base type with mixed content
	baseType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "MixedBaseType",
		},
	}
	baseType.SetMixed(true)
	baseType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind: types.Sequence,
			Particles: []types.Particle{
				&types.ElementDecl{
					Name: types.QName{Local: "child"},
					Type: types.GetBuiltin(types.TypeName("string")),
				},
			},
		},
	})
	schema.TypeDefs[baseType.QName] = baseType

	// derived type extending base with mixed content
	derivedType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "MixedDerivedType",
		},
		DerivationMethod: types.DerivationExtension,
	}
	derivedType.SetMixed(true) // mixed
	derivedType.SetContent(&types.ComplexContent{
		Mixed: true,
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "MixedBaseType",
			},
		},
	})
	schema.TypeDefs[derivedType.QName] = derivedType

	// should be VALID
	err := validation.ValidateMixedContentDerivation(schema, derivedType)
	if err != nil {
		t.Errorf("Extension from mixed to mixed should be VALID, got error: %v", err)
	}
}

// TestMixedContentDerivation_ExtensionFromElementOnlyToMixed tests that extension from element-only to mixed is INVALID
func TestMixedContentDerivation_ExtensionFromElementOnlyToMixed(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// base type with element-only content
	baseType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "ElementOnlyBaseType",
		},
	}
	baseType.SetMixed(false)
	baseType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind: types.Sequence,
			Particles: []types.Particle{
				&types.ElementDecl{
					Name: types.QName{Local: "child"},
					Type: types.GetBuiltin(types.TypeName("string")),
				},
			},
		},
	})
	schema.TypeDefs[baseType.QName] = baseType

	// derived type extending base with mixed content (adding mixed)
	derivedType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "MixedType",
		},
		DerivationMethod: types.DerivationExtension,
	}
	derivedType.SetMixed(true) // mixed
	derivedType.SetContent(&types.ComplexContent{
		Mixed: true,
		Extension: &types.Extension{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "ElementOnlyBaseType",
			},
		},
	})
	schema.TypeDefs[derivedType.QName] = derivedType

	// should be INVALID - extension cannot add mixed content (would allow text that base disallows)
	err := validation.ValidateMixedContentDerivation(schema, derivedType)
	if err == nil {
		t.Error("Extension from element-only to mixed should be INVALID, but got no error")
	}
	if err != nil && !strings.Contains(err.Error(), "mixed") && !strings.Contains(err.Error(), "element-only") {
		t.Errorf("Error should mention mixed or element-only content, got: %v", err)
	}
}

// TestMixedContentDerivation_RestrictionFromMixedToElementOnly tests that restriction from mixed to element-only is VALID
func TestMixedContentDerivation_RestrictionFromMixedToElementOnly(t *testing.T) {
	schema := &schema.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// base type with mixed content
	baseType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "MixedBaseType",
		},
	}
	baseType.SetMixed(true)
	baseType.SetContent(&types.ElementContent{
		Particle: &types.ModelGroup{
			Kind: types.Sequence,
			Particles: []types.Particle{
				&types.ElementDecl{
					Name: types.QName{Local: "child"},
					Type: types.GetBuiltin(types.TypeName("string")),
				},
			},
		},
	})
	schema.TypeDefs[baseType.QName] = baseType

	// derived type restricting base to element-only content (removing mixed)
	derivedType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "ElementOnlyType",
		},
		DerivationMethod: types.DerivationRestriction,
	}
	derivedType.SetMixed(false) // element-only
	derivedType.SetContent(&types.ComplexContent{
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "MixedBaseType",
			},
		},
	})
	schema.TypeDefs[derivedType.QName] = derivedType

	// should be VALID - restriction can remove mixed content (remove constraint)
	err := validation.ValidateMixedContentDerivation(schema, derivedType)
	if err != nil {
		t.Errorf("Restriction from mixed to element-only should be VALID, got error: %v", err)
	}
}

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
