package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typegraph"
	"github.com/jacoelho/xsd/internal/types"
)

// validateNoCircularDerivation validates that a complex type doesn't have circular derivation.
func validateNoCircularDerivation(schema *parser.Schema, complexType *types.ComplexType) error {
	visited := make(map[types.QName]bool)
	return checkCircularDerivation(schema, complexType.QName, complexType, visited)
}

// checkCircularDerivation recursively checks for circular derivation.
func checkCircularDerivation(schema *parser.Schema, originalQName types.QName, complexType *types.ComplexType, visited map[types.QName]bool) error {
	baseQName := complexType.Content().BaseTypeQName()
	if visited[complexType.QName] {
		if baseQName.IsZero() || baseQName == complexType.QName {
			return nil
		}
		return fmt.Errorf("complex type '%s' has circular derivation through '%s'", originalQName, complexType.QName)
	}
	if baseQName.IsZero() {
		return nil
	}

	visited[complexType.QName] = true
	defer delete(visited, complexType.QName)

	baseComplexType, ok := typegraph.LookupComplexType(schema, baseQName)
	if !ok {
		return nil
	}
	return checkCircularDerivation(schema, originalQName, baseComplexType, visited)
}

// validateDerivationConstraints validates final/block constraints on type derivation.
func validateDerivationConstraints(schema *parser.Schema, complexType *types.ComplexType) error {
	content := complexType.Content()
	baseQName := content.BaseTypeQName()
	if baseQName.IsZero() {
		return nil
	}
	baseCT, ok := typegraph.LookupComplexType(schema, baseQName)
	if !ok {
		return nil
	}
	if ext := content.ExtensionDef(); ext != nil && baseCT.Final.Has(types.DerivationExtension) {
		return fmt.Errorf("cannot extend type '%s': base type is final for extension", baseQName)
	}
	if restr := content.RestrictionDef(); restr != nil && baseCT.Final.Has(types.DerivationRestriction) {
		return fmt.Errorf("cannot restrict type '%s': base type is final for restriction", baseQName)
	}
	return nil
}

// validateMixedContentDerivation validates mixed content derivation constraints.
func validateMixedContentDerivation(schema *parser.Schema, complexType *types.ComplexType) error {
	if !complexType.IsDerived() {
		return nil
	}
	if _, isComplexContent := complexType.Content().(*types.ComplexContent); !isComplexContent {
		return nil
	}

	baseQName := complexType.Content().BaseTypeQName()
	if baseQName.IsZero() {
		return nil
	}
	baseComplexType, ok := typegraph.LookupComplexType(schema, baseQName)
	if !ok {
		return nil
	}

	baseMixed := baseComplexType.EffectiveMixed()
	derivedMixed := complexType.EffectiveMixed()

	if complexType.IsExtension() {
		if cc, ok := complexType.Content().(*types.ComplexContent); ok {
			if ext := cc.Extension; ext != nil {
				if ext.Particle == nil && !derivedMixed {
					return nil
				}
				if mg, ok := ext.Particle.(*types.ModelGroup); ok && len(mg.Particles) == 0 && !derivedMixed {
					return nil
				}
			}
		}
		if baseMixed && !derivedMixed {
			return fmt.Errorf("cannot extend mixed content type '%s' to element-only content", baseComplexType.QName.Local)
		}
		if !baseMixed && derivedMixed {
			return fmt.Errorf("cannot extend element-only content type '%s' to mixed content", baseComplexType.QName.Local)
		}
	} else if complexType.IsRestriction() {
		if !baseMixed && derivedMixed {
			return fmt.Errorf("cannot restrict element-only content type '%s' to mixed content", baseComplexType.QName.Local)
		}
	}

	return nil
}
