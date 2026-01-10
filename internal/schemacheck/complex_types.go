package schemacheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateComplexTypeStructure validates structural constraints of a complex type
// Does not validate references (which might be forward references or imports)
// isInline indicates if this complexType is defined inline in an element (local element)
func validateComplexTypeStructure(schema *parser.Schema, ct *types.ComplexType, isInline bool) error {
	if err := validateContentStructure(schema, ct.Content(), isInline); err != nil {
		return fmt.Errorf("content: %w", err)
	}

	if err := validateUPA(schema, ct.Content(), schema.TargetNamespace); err != nil {
		return fmt.Errorf("UPA violation: %w", err)
	}

	if err := validateElementDeclarationsConsistent(schema, ct); err != nil {
		return fmt.Errorf("element declarations consistent: %w", err)
	}

	if err := validateMixedContentDerivation(schema, ct); err != nil {
		return fmt.Errorf("mixed content derivation: %w", err)
	}

	if err := validateWildcardDerivation(schema, ct); err != nil {
		return fmt.Errorf("wildcard derivation: %w", err)
	}

	if err := validateAnyAttributeDerivation(schema, ct); err != nil {
		return fmt.Errorf("anyAttribute derivation: %w", err)
	}

	for _, attr := range ct.Attributes() {
		if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
			return fmt.Errorf("attribute %s: %w", attr.Name, err)
		}
	}

	if content := ct.Content(); content != nil {
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

	if err := validateAttributeUniqueness(schema, ct); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}

	if err := validateIDAttributeCount(schema, ct); err != nil {
		return fmt.Errorf("attributes: %w", err)
	}

	if err := validateNoCircularDerivation(schema, ct); err != nil {
		return fmt.Errorf("circular derivation: %w", err)
	}

	if err := validateDerivationConstraints(schema, ct); err != nil {
		return fmt.Errorf("derivation constraints: %w", err)
	}

	return nil
}

// validateNoCircularDerivation validates that a complex type doesn't have circular derivation
// A type cannot (even indirectly) be its own base
func validateNoCircularDerivation(schema *parser.Schema, ct *types.ComplexType) error {
	visited := make(map[types.QName]bool)
	return checkCircularDerivation(schema, ct.QName, ct, visited)
}

// checkCircularDerivation recursively checks for circular derivation
func checkCircularDerivation(schema *parser.Schema, originalQName types.QName, ct *types.ComplexType, visited map[types.QName]bool) error {
	baseQName := ct.Content().BaseTypeQName()

	// if we've already seen this type in the derivation chain, it's a cycle
	// except: if this type extends itself directly (baseQName == ct.QName), allow it for redefine cases
	if visited[ct.QName] {
		if baseQName.IsZero() || baseQName == ct.QName {
			return nil // no derivation or self-reference (valid in redefine context)
		}
		return fmt.Errorf("complex type '%s' has circular derivation through '%s'", originalQName, ct.QName)
	}

	if baseQName.IsZero() {
		return nil // no derivation
	}

	visited[ct.QName] = true
	defer delete(visited, ct.QName)

	// check if base type exists and is complex
	baseCT, ok := lookupComplexType(schema, baseQName)
	if !ok {
		// base type not found or not complex - no cycle possible
		return nil
	}

	// recursively check base type
	return checkCircularDerivation(schema, originalQName, baseCT, visited)
}

// validateDerivationConstraints validates final/block constraints on type derivation
// According to XSD spec: "Proper Derivation"
func validateDerivationConstraints(schema *parser.Schema, ct *types.ComplexType) error {
	content := ct.Content()
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
//   - Extension must preserve the mixed/element-only content kind.
//     If the extension adds no particle, it inherits the base content (including mixedness).
//   - Restriction cannot introduce mixed content when base is element-only.
func validateMixedContentDerivation(schema *parser.Schema, ct *types.ComplexType) error {
	if !ct.IsDerived() {
		return nil
	}

	// simpleContent doesn't have mixed content
	cc, isComplexContent := ct.Content().(*types.ComplexContent)
	if !isComplexContent {
		return nil
	}

	baseQName := ct.Content().BaseTypeQName()
	if baseQName.IsZero() {
		return nil
	}

	baseCT, ok := lookupComplexType(schema, baseQName)
	if !ok {
		return nil // base type not found or not complex
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
		// restriction: base mixed=false, derived mixed=true → INVALID (cannot add mixed)
		// restriction: base mixed=true, derived mixed=false → VALID (can remove mixed)
		if !baseMixed && derivedMixed {
			return fmt.Errorf("cannot restrict element-only content type '%s' to mixed content", baseCT.QName.Local)
		}
		// all other restriction combinations are valid
	}

	return nil
}

// validateElementDeclarationsConsistent validates that element declarations are consistent
// in extensions. According to XSD spec "Element Declarations Consistent": when extending
// a complex type, elements in the extension cannot have the same name as elements in the
// base type with different types.
func validateElementDeclarationsConsistent(schema *parser.Schema, ct *types.ComplexType) error {
	if !ct.IsExtension() {
		return nil
	}

	content := ct.Content()
	ext := content.ExtensionDef()
	if ext == nil {
		return nil
	}

	baseQName := content.BaseTypeQName()
	baseCT, ok := lookupComplexType(schema, baseQName)
	if !ok {
		return nil // base type not found or not complex
	}

	baseElements := collectAllElementDeclarationsFromType(schema, baseCT)

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

// collectAllElementDeclarationsFromType collects all element declarations from a complex type
// This recursively collects from the type's content model and its base types
func collectAllElementDeclarationsFromType(schema *parser.Schema, ct *types.ComplexType) []*types.ElementDecl {
	visited := make(map[types.QName]bool)
	return collectElementDeclarationsRecursive(schema, ct, visited)
}

// collectElementDeclarationsRecursive recursively collects element declarations from a type and its base types
func collectElementDeclarationsRecursive(schema *parser.Schema, ct *types.ComplexType, visited map[types.QName]bool) []*types.ElementDecl {
	// avoid infinite loops
	if visited[ct.QName] {
		return nil
	}
	visited[ct.QName] = true

	var result []*types.ElementDecl

	// collect from this type's content
	content := ct.Content()
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

func validateIDAttributeCount(schema *parser.Schema, ct *types.ComplexType) error {
	attrs := collectAllAttributesForValidation(schema, ct)
	idCount := 0
	for _, attr := range attrs {
		if attr.Use == types.Prohibited {
			continue
		}
		if attr.Type == nil {
			continue
		}
		typeName := attr.Type.Name()
		if typeName.Namespace == types.XSDNamespace && typeName.Local == "ID" {
			idCount++
			continue
		}
		if st, ok := attr.Type.(*types.SimpleType); ok {
			if isIDOnlyDerivedType(st) {
				idCount++
			}
		}
	}
	if idCount > 1 {
		return fmt.Errorf("type %s has multiple ID attributes", ct.QName.Local)
	}
	return nil
}
