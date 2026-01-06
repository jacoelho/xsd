package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateNoCircularDerivation validates that a complex type doesn't have circular derivation
// A type cannot (even indirectly) be its own base
func validateNoCircularDerivation(schema *schema.Schema, ct *types.ComplexType) error {
	visited := make(map[types.QName]bool)
	return checkCircularDerivation(schema, ct.QName, ct, visited)
}

// checkCircularDerivation recursively checks for circular derivation
func checkCircularDerivation(schema *schema.Schema, originalQName types.QName, ct *types.ComplexType, visited map[types.QName]bool) error {
	baseQName := ct.Content().BaseTypeQName()

	// If we've already seen this type in the derivation chain, it's a cycle
	// EXCEPT: if this type extends itself directly (baseQName == ct.QName), allow it for redefine cases
	if visited[ct.QName] {
		if baseQName.IsZero() || baseQName == ct.QName {
			return nil // No derivation or self-reference (valid in redefine context)
		}
		return fmt.Errorf("complex type '%s' has circular derivation through '%s'", originalQName, ct.QName)
	}

	if baseQName.IsZero() {
		return nil // No derivation
	}

	visited[ct.QName] = true
	defer delete(visited, ct.QName)

	// Check if base type exists and is complex
	baseType, ok := schema.TypeDefs[baseQName]
	if !ok {
		// Base type not found - might be builtin or forward reference, skip cycle check
		return nil
	}

	baseCT, ok := baseType.(*types.ComplexType)
	if !ok {
		// Base type is not complex - no cycle possible
		return nil
	}

	// Recursively check base type
	return checkCircularDerivation(schema, originalQName, baseCT, visited)
}

// validateDerivationConstraints validates final/block constraints on type derivation
// According to XSD spec: "Proper Derivation"
func validateDerivationConstraints(schema *schema.Schema, ct *types.ComplexType) error {
	content := ct.Content()
	baseQName := content.BaseTypeQName()
	if baseQName.IsZero() {
		return nil // No derivation
	}

	baseType, ok := schema.TypeDefs[baseQName]
	if !ok {
		return nil // Base type not found - might be builtin or forward reference
	}

	baseCT, ok := baseType.(*types.ComplexType)
	if !ok {
		return nil // Base type is not complex
	}

	// Check final constraint: base type cannot be final for the derivation method being used
	if ext := content.ExtensionDef(); ext != nil && baseCT.Final.Has(types.DerivationExtension) {
		return fmt.Errorf("cannot extend type '%s': base type is final for extension", baseQName)
	}
	if restr := content.RestrictionDef(); restr != nil && baseCT.Final.Has(types.DerivationRestriction) {
		return fmt.Errorf("cannot restrict type '%s': base type is final for restriction", baseQName)
	}
	return nil
}

// validateMixedContentDerivation validates mixed content derivation constraints.
// Per XSD 1.0 Structures spec section 3.4.2 (Derivation Valid for complex types):
//   - Extension must preserve the mixed/element-only content kind.
//     If the extension adds no particle, it inherits the base content (including mixedness).
//   - Restriction cannot introduce mixed content when base is element-only.
func validateMixedContentDerivation(schema *schema.Schema, ct *types.ComplexType) error {
	if !ct.IsDerived() {
		return nil
	}

	// SimpleContent doesn't have mixed content
	cc, isComplexContent := ct.Content().(*types.ComplexContent)
	if !isComplexContent {
		return nil
	}

	baseQName := ct.Content().BaseTypeQName()
	if baseQName.IsZero() {
		return nil
	}

	baseType, ok := schema.TypeDefs[baseQName]
	if !ok {
		return nil // Base type not found
	}

	baseCT, ok := baseType.(*types.ComplexType)
	if !ok {
		return nil // Base type is not complex
	}

	baseMixed := baseCT.Mixed()
	if baseCC, ok := baseCT.Content().(*types.ComplexContent); ok && baseCC.Mixed {
		baseMixed = true
	}

	derivedMixed := ct.Mixed() || cc.Mixed

	if ct.IsExtension() {
		if baseMixed && !derivedMixed {
			if cc.Extension != nil && cc.Extension.Particle == nil {
				return nil
			}
			return fmt.Errorf("cannot extend mixed content type '%s' to element-only content", baseCT.QName.Local)
		}
		if !baseMixed && derivedMixed {
			return fmt.Errorf("cannot extend element-only content type '%s' to mixed content", baseCT.QName.Local)
		}
	} else if ct.IsRestriction() {
		// Restriction: base mixed=false, derived mixed=true → INVALID (cannot add mixed)
		// Restriction: base mixed=true, derived mixed=false → VALID (can remove mixed)
		if !baseMixed && derivedMixed {
			return fmt.Errorf("cannot restrict element-only content type '%s' to mixed content", baseCT.QName.Local)
		}
		// All other restriction combinations are valid
	}

	return nil
}
