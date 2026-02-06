package schemacheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// validateComplexTypeStructure validates structural constraints of a complex type.
// Does not validate references (which might be forward references or imports).
func validateComplexTypeStructure(schema *parser.Schema, complexType *types.ComplexType, context typeDefinitionContext) error {
	if err := validateContentStructure(schema, complexType.Content(), context); err != nil {
		return fmt.Errorf("content: %w", err)
	}

	if err := ValidateUPA(schema, complexType.Content(), schema.TargetNamespace); err != nil {
		return fmt.Errorf("UPA violation: %w", err)
	}

	if err := validateElementDeclarationsConsistent(schema, complexType); err != nil {
		return fmt.Errorf("element declarations consistent: %w", err)
	}

	if err := validateMixedContentDerivation(schema, complexType); err != nil {
		return fmt.Errorf("mixed content derivation: %w", err)
	}

	if err := validateWildcardDerivation(schema, complexType); err != nil {
		return fmt.Errorf("wildcard derivation: %w", err)
	}

	if err := validateAnyAttributeDerivation(schema, complexType); err != nil {
		return fmt.Errorf("anyAttribute derivation: %w", err)
	}
	if _, err := collectAnyAttributeFromType(schema, complexType); err != nil {
		return fmt.Errorf("anyAttribute: %w", err)
	}

	for _, attr := range complexType.Attributes() {
		if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
			return fmt.Errorf("attribute %s: %w", attr.Name, err)
		}
	}

	if content := complexType.Content(); content != nil {
		if ext := content.ExtensionDef(); ext != nil {
			for _, attr := range ext.Attributes {
				if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
					return fmt.Errorf("extension attribute %s: %w", attr.Name, err)
				}
			}
		}
		if restr := content.RestrictionDef(); restr != nil {
			for _, attr := range restr.Attributes {
				if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
					return fmt.Errorf("restriction attribute %s: %w", attr.Name, err)
				}
			}
		}
	}

	if err := validateAttributeUniqueness(schema, complexType); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}
	if err := validateExtensionAttributeUniqueness(schema, complexType); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}

	if err := validateIDAttributeCount(schema, complexType); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}

	if err := validateNoCircularDerivation(schema, complexType); err != nil {
		return fmt.Errorf("circular derivation: %w", err)
	}

	if err := validateDerivationConstraints(schema, complexType); err != nil {
		return fmt.Errorf("derivation constraints: %w", err)
	}

	return nil
}

// validateNoCircularDerivation validates that a complex type doesn't have circular derivation
// A type cannot (even indirectly) be its own base
func validateNoCircularDerivation(schema *parser.Schema, complexType *types.ComplexType) error {
	visited := make(map[types.QName]bool)
	return checkCircularDerivation(schema, complexType.QName, complexType, visited)
}

// checkCircularDerivation recursively checks for circular derivation
func checkCircularDerivation(schema *parser.Schema, originalQName types.QName, complexType *types.ComplexType, visited map[types.QName]bool) error {
	baseQName := complexType.Content().BaseTypeQName()

	// if we've already seen this type in the derivation chain, it's a cycle
	// except: if this type extends itself directly (baseQName == complexType.QName), allow it for redefine cases
	if visited[complexType.QName] {
		if baseQName.IsZero() || baseQName == complexType.QName {
			return nil // no derivation or self-reference (valid in redefine context)
		}
		return fmt.Errorf("complex type '%s' has circular derivation through '%s'", originalQName, complexType.QName)
	}

	if baseQName.IsZero() {
		return nil // no derivation
	}

	visited[complexType.QName] = true
	defer delete(visited, complexType.QName)

	// check if base type exists and is complex
	baseComplexType, ok := lookupComplexType(schema, baseQName)
	if !ok {
		// base type not found or not complex - no cycle possible
		return nil
	}

	// recursively check base type
	return checkCircularDerivation(schema, originalQName, baseComplexType, visited)
}

// validateDerivationConstraints validates final/block constraints on type derivation
// According to XSD spec: "Proper Derivation"
func validateDerivationConstraints(schema *parser.Schema, complexType *types.ComplexType) error {
	content := complexType.Content()
	baseQName := content.BaseTypeQName()
	if baseQName.IsZero() {
		return nil // no derivation
	}

	baseCT, ok := lookupComplexType(schema, baseQName)
	if !ok {
		return nil // base type not found or not complex
	}

	// check final constraint: base type cannot be final for the derivation method being used
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
//   - Extension must preserve the mixed/element-only content kind when it adds
//     explicit content; when the extension adds no particle and effective mixed
//     is false, the base content type (and mixedness) is inherited.
//   - Restriction cannot introduce mixed content when base is element-only.
func validateMixedContentDerivation(schema *parser.Schema, complexType *types.ComplexType) error {
	if !complexType.IsDerived() {
		return nil
	}

	// simpleContent doesn't have mixed content
	if _, isComplexContent := complexType.Content().(*types.ComplexContent); !isComplexContent {
		return nil
	}

	baseQName := complexType.Content().BaseTypeQName()
	if baseQName.IsZero() {
		return nil
	}

	baseComplexType, ok := lookupComplexType(schema, baseQName)
	if !ok {
		return nil // base type not found or not complex
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
		// restriction: base mixed=false, derived mixed=true → INVALID (cannot add mixed)
		// restriction: base mixed=true, derived mixed=false → VALID (can remove mixed)
		if !baseMixed && derivedMixed {
			return fmt.Errorf("cannot restrict element-only content type '%s' to mixed content", baseComplexType.QName.Local)
		}
		// all other restriction combinations are valid
	}

	return nil
}

// validateElementDeclarationsConsistent validates that element declarations are consistent
// in extensions. According to XSD spec "Element Declarations Consistent": when extending
// a complex type, elements in the extension cannot have the same name as elements in the
// base type with different types.
func validateElementDeclarationsConsistent(schema *parser.Schema, complexType *types.ComplexType) error {
	if !complexType.IsExtension() {
		return nil
	}

	content := complexType.Content()
	ext := content.ExtensionDef()
	if ext == nil {
		return nil
	}

	baseQName := content.BaseTypeQName()
	baseComplexType, ok := lookupComplexType(schema, baseQName)
	if !ok {
		return nil // base type not found or not complex
	}

	baseElements := CollectAllElementDeclarationsFromType(schema, baseComplexType)

	// simpleContent extensions don't have particles
	if ext.Particle == nil {
		return nil
	}
	extElements := collectElementDeclarationsFromParticle(ext.Particle)

	for _, extElem := range extElements {
		for _, baseElem := range baseElements {
			// check if names match (same local name and namespace)
			if extElem.Name == baseElem.Name {
				// names match - types must also match
				// compare types by checking if they're the same object or have the same QName
				extTypeQName := getTypeQName(extElem.Type)
				baseTypeQName := getTypeQName(baseElem.Type)
				if extTypeQName != baseTypeQName {
					return fmt.Errorf("element '%s' in extension has type '%s' but base type has type '%s' (Element Declarations Consistent violation)", extElem.Name.Local, extTypeQName, baseTypeQName)
				}
			}
		}
	}

	return nil
}

// CollectAllElementDeclarationsFromType collects all element declarations from a complex type.
// This recursively collects from the type's content model and its base types.
func CollectAllElementDeclarationsFromType(schema *parser.Schema, complexType *types.ComplexType) []*types.ElementDecl {
	visited := make(map[types.QName]bool)
	return collectElementDeclarationsRecursive(schema, complexType, visited)
}

// collectElementDeclarationsRecursive recursively collects element declarations from a type and its base types
func collectElementDeclarationsRecursive(schema *parser.Schema, complexType *types.ComplexType, visited map[types.QName]bool) []*types.ElementDecl {
	// avoid infinite loops
	if visited[complexType.QName] {
		return nil
	}
	visited[complexType.QName] = true

	var result []*types.ElementDecl

	// collect from this type's content
	content := complexType.Content()
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			result = append(result, collectElementDeclarationsFromParticle(c.Particle)...)
		}
	case *types.ComplexContent:
		// for extensions, collect from extension particles
		if c.Extension != nil && c.Extension.Particle != nil {
			result = append(result, collectElementDeclarationsFromParticle(c.Extension.Particle)...)
		}
		// for restrictions, collect from restriction particles (which restrict base)
		if c.Restriction != nil && c.Restriction.Particle != nil {
			result = append(result, collectElementDeclarationsFromParticle(c.Restriction.Particle)...)
		}
		// also collect from base type recursively
		var baseQName types.QName
		if c.Extension != nil {
			baseQName = c.Extension.Base
		} else if c.Restriction != nil {
			baseQName = c.Restriction.Base
		}
		if !baseQName.IsZero() {
			if baseCT, ok := lookupComplexType(schema, baseQName); ok {
				result = append(result, collectElementDeclarationsRecursive(schema, baseCT, visited)...)
			}
		}
	}
	return result
}

// collectElementDeclarationsFromParticle collects all element declarations from a particle (recursively)
func collectElementDeclarationsFromParticle(particle types.Particle) []*types.ElementDecl {
	var result []*types.ElementDecl
	switch p := particle.(type) {
	case *types.ModelGroup:
		// recursively collect from all particles in the group
		for _, child := range p.Particles {
			result = append(result, collectElementDeclarationsFromParticle(child)...)
		}
	case *types.ElementDecl:
		result = append(result, p)
	case *types.AnyElement:
		// wildcards don't have element declarations
	}
	return result
}

func validateIDAttributeCount(schema *parser.Schema, complexType *types.ComplexType) error {
	attrs := collectAllAttributesForValidation(schema, complexType)
	idCount := 0
	for _, attr := range attrs {
		if attr.Use == types.Prohibited {
			continue
		}
		if attr.Type == nil {
			continue
		}
		resolvedType := ResolveTypeReference(schema, attr.Type, TypeReferenceAllowMissing)
		if resolvedType == nil {
			continue
		}
		typeName := resolvedType.Name()
		if typeName.Namespace == types.XSDNamespace && typeName.Local == string(types.TypeNameID) {
			idCount++
			continue
		}
		if simpleType, ok := resolvedType.(*types.SimpleType); ok {
			if typeops.IsIDOnlyDerivedType(schema, simpleType) {
				idCount++
			}
		}
	}
	if idCount > 1 {
		return fmt.Errorf("type %s has multiple ID attributes", complexType.QName.Local)
	}
	return nil
}
