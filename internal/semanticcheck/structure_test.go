package semanticcheck

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// TestCircularDerivation_TrueCycle tests that a true circular derivation (A -> B -> A) is correctly detected.
func TestCircularDerivation_TrueCycle(t *testing.T) {
	schema := &parser.Schema{
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
	err := validateNoCircularDerivation(schema, typeA)
	if err == nil {
		t.Error("Should detect circular derivation for TypeA -> TypeB -> TypeA")
	}
	if err != nil && !strings.Contains(err.Error(), "circular derivation") {
		t.Errorf("Error should mention circular derivation, got: %v", err)
	}
}

// TestCircularDerivation_ValidDeepHierarchy tests that a valid deep hierarchy (A -> B -> C -> anyType) is NOT flagged as circular.
func TestCircularDerivation_ValidDeepHierarchy(t *testing.T) {
	schema := &parser.Schema{
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
	err := validateNoCircularDerivation(schema, typeA)
	if err != nil {
		t.Errorf("Should NOT detect circular derivation for valid hierarchy A -> B -> C -> anyType, got: %v", err)
	}
}

func TestValidateStructureDeterministicOrder(t *testing.T) {
	schema := parser.NewSchema()
	schema.TargetNamespace = "urn:test"

	attrQName := types.QName{Namespace: "urn:test", Local: "1bad"}
	schema.AttributeDecls[attrQName] = &types.AttributeDecl{Name: attrQName}
	schema.GlobalDecls = append(schema.GlobalDecls, parser.GlobalDecl{Kind: parser.GlobalDeclAttribute, Name: attrQName})

	elemQName := types.QName{Namespace: "urn:test", Local: "2bad"}
	schema.ElementDecls[elemQName] = &types.ElementDecl{Name: elemQName}
	schema.GlobalDecls = append(schema.GlobalDecls, parser.GlobalDecl{Kind: parser.GlobalDeclElement, Name: elemQName})

	typeQName := types.QName{Namespace: "urn:test", Local: "3bad"}
	schema.TypeDefs[typeQName] = &types.SimpleType{QName: typeQName}
	schema.GlobalDecls = append(schema.GlobalDecls, parser.GlobalDecl{Kind: parser.GlobalDeclType, Name: typeQName})

	errs := ValidateStructure(schema)
	if len(errs) < 3 {
		t.Fatalf("errors = %d, want at least 3", len(errs))
	}
	if !strings.Contains(errs[0].Error(), "attribute") {
		t.Fatalf("first error = %q, want attribute error", errs[0].Error())
	}
	if !strings.Contains(errs[1].Error(), "element") {
		t.Fatalf("second error = %q, want element error", errs[1].Error())
	}
	if !strings.Contains(errs[2].Error(), "type") {
		t.Fatalf("third error = %q, want type error", errs[2].Error())
	}
}

func TestProhibitedAttributeWithFixedAllowed(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:attribute name="a" type="xs:string" use="prohibited" fixed="x"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	parsed, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	if errs := ValidateStructure(parsed); len(errs) != 0 {
		t.Fatalf("unexpected structure errors: %v", errs)
	}
}

// TestCircularDerivation_MultipleTypesFromSameBase tests that multiple types deriving from the same base is NOT flagged as circular.
func TestCircularDerivation_MultipleTypesFromSameBase(t *testing.T) {
	schema := &parser.Schema{
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
	err := validateNoCircularDerivation(schema, typeA)
	if err != nil {
		t.Errorf("Should NOT detect circular derivation for TypeA -> BaseType, got: %v", err)
	}

	err = validateNoCircularDerivation(schema, typeB)
	if err != nil {
		t.Errorf("Should NOT detect circular derivation for TypeB -> BaseType, got: %v", err)
	}
}

// TestCircularDerivation_RedefineSelfExtension tests that a type extending itself in a redefine context is NOT flagged as circular.
// This is the ipo4 case: AddressType is redefined to extend itself (referring to the old definition).
func TestCircularDerivation_RedefineSelfExtension(t *testing.T) {
	schema := &parser.Schema{
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
	err := validateNoCircularDerivation(schema, redefinedAddressType)
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

	err = validateNoCircularDerivation(schema, usAddress)
	if err != nil {
		t.Errorf("Should NOT detect circular derivation for USAddress -> AddressType, got: %v", err)
	}
}

// TestMixedContentDerivation_ExtensionFromMixedToElementOnly tests that extension from mixed content
// to element-only content with additional particles is INVALID.
func TestMixedContentDerivation_ExtensionFromMixedToElementOnly(t *testing.T) {
	schema := &parser.Schema{
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
	err := validateMixedContentDerivation(schema, derivedType)
	if err == nil {
		t.Error("Extension from mixed to element-only should be INVALID, but got no error")
	}
	if err != nil && !strings.Contains(err.Error(), "mixed") && !strings.Contains(err.Error(), "element-only") {
		t.Errorf("Error should mention mixed or element-only content, got: %v", err)
	}
}

func TestMixedContentDerivation_ExtensionFromMixedToElementOnlyNoParticle(t *testing.T) {
	schema := &parser.Schema{
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

	err := validateMixedContentDerivation(schema, derivedType)
	if err != nil {
		t.Errorf("Extension with no particle should inherit mixed content, got error: %v", err)
	}
}

// TestMixedContentDerivation_RestrictionFromElementOnlyToMixed tests that restriction from element-only to mixed content is INVALID.
func TestMixedContentDerivation_RestrictionFromElementOnlyToMixed(t *testing.T) {
	schema := &parser.Schema{
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
	err := validateMixedContentDerivation(schema, derivedType)
	if err == nil {
		t.Error("Restriction from element-only to mixed should be INVALID, but got no error")
	}
	if err != nil && !strings.Contains(err.Error(), "mixed") {
		t.Errorf("Error should mention mixed content, got: %v", err)
	}
}

// TestMixedContentDerivation_ExtensionFromElementOnlyToElementOnly tests that extension from element-only to element-only is VALID.
func TestMixedContentDerivation_ExtensionFromElementOnlyToElementOnly(t *testing.T) {
	schema := &parser.Schema{
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
	err := validateMixedContentDerivation(schema, derivedType)
	if err != nil {
		t.Errorf("Extension from element-only to element-only should be VALID, got error: %v", err)
	}
}

// TestMixedContentDerivation_ExtensionFromMixedToMixed tests that extension from mixed to mixed is VALID.
func TestMixedContentDerivation_ExtensionFromMixedToMixed(t *testing.T) {
	schema := &parser.Schema{
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
	err := validateMixedContentDerivation(schema, derivedType)
	if err != nil {
		t.Errorf("Extension from mixed to mixed should be VALID, got error: %v", err)
	}
}

// TestMixedContentDerivation_ExtensionFromElementOnlyToMixed tests that extension from element-only to mixed is INVALID.
func TestMixedContentDerivation_ExtensionFromElementOnlyToMixed(t *testing.T) {
	schema := &parser.Schema{
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
	err := validateMixedContentDerivation(schema, derivedType)
	if err == nil {
		t.Error("Extension from element-only to mixed should be INVALID, but got no error")
	}
	if err != nil && !strings.Contains(err.Error(), "mixed") && !strings.Contains(err.Error(), "element-only") {
		t.Errorf("Error should mention mixed or element-only content, got: %v", err)
	}
}

// TestMixedContentDerivation_RestrictionFromMixedToElementOnly tests that restriction from mixed to element-only is VALID.
func TestMixedContentDerivation_RestrictionFromMixedToElementOnly(t *testing.T) {
	schema := &parser.Schema{
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
	err := validateMixedContentDerivation(schema, derivedType)
	if err != nil {
		t.Errorf("Restriction from mixed to element-only should be VALID, got error: %v", err)
	}
}
