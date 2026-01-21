package resolver

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
)

// collectElementReferences collects element references from content models.
func collectElementReferences(content types.Content) []*types.ElementDecl {
	var refs []*types.ElementDecl
	if err := schemacheck.WalkContentParticles(content, func(particle types.Particle) error {
		refs = append(refs, collectElementReferencesFromParticles([]types.Particle{particle})...)
		return nil
	}); err != nil {
		return refs
	}
	return refs
}

// collectElementReferencesFromParticles collects element references from particles.
func collectElementReferencesFromParticles(particles []types.Particle) []*types.ElementDecl {
	visited := make(map[*types.ModelGroup]bool)
	return collectElementReferencesFromParticlesWithVisited(particles, visited)
}

// collectElementReferencesFromParticlesWithVisited collects element references with cycle detection.
func collectElementReferencesFromParticlesWithVisited(particles []types.Particle, visited map[*types.ModelGroup]bool) []*types.ElementDecl {
	var refs []*types.ElementDecl
	for _, particle := range particles {
		switch p := particle.(type) {
		case *types.ElementDecl:
			if p.IsReference {
				refs = append(refs, p)
			}
		case *types.ModelGroup:
			// skip if already visited (prevents infinite recursion in cyclic groups).
			if visited[p] {
				continue
			}
			visited[p] = true
			refs = append(refs, collectElementReferencesFromParticlesWithVisited(p.Particles, visited)...)
		}
	}
	return refs
}

func validateElementValueConstraints(schema *parser.Schema, decl *types.ElementDecl) error {
	if decl == nil {
		return nil
	}

	resolvedType := resolveTypeForFinalValidation(schema, decl.Type)
	if isDirectNotationType(resolvedType) {
		return fmt.Errorf("element cannot use NOTATION type")
	}

	if !decl.HasDefault && !decl.HasFixed {
		return nil
	}

	// per XSD spec 3.3.5.2, elements can have default/fixed values only if content type is:
	// simple type, simpleContent, or mixed content.
	if ct, ok := resolvedType.(*types.ComplexType); ok {
		_, isSimpleContent := ct.Content().(*types.SimpleContent)
		if !isSimpleContent && !ct.EffectiveMixed() {
			if decl.HasDefault {
				return fmt.Errorf("element with element-only complex type cannot have default value")
			}
			return fmt.Errorf("element with element-only complex type cannot have fixed value")
		}
	}

	if decl.HasDefault {
		if err := validateDefaultOrFixedValueWithResolvedType(schema, decl.Default, resolvedType, decl.DefaultContext); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}
	if decl.HasFixed {
		if err := validateDefaultOrFixedValueWithResolvedType(schema, decl.Fixed, resolvedType, decl.FixedContext); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}
	return nil
}

// validateSubstitutionGroupFinal validates that the substitution group member's derivation
// method is not blocked by the head element's final attribute.
func validateSubstitutionGroupFinal(schema *parser.Schema, memberQName types.QName, memberDecl, headDecl *types.ElementDecl) error {
	// if head element has no final constraints, any derivation is allowed.
	if headDecl.Final == 0 {
		return nil
	}

	// we need to check if the member's type is derived from the head's type.
	memberType := memberDecl.Type
	headType := headDecl.Type

	if memberType == nil || headType == nil {
		return nil // can't validate without types.
	}

	// resolve types if they are placeholders.
	memberType = resolveTypeForFinalValidation(schema, memberType)
	headType = resolveTypeForFinalValidation(schema, headType)

	if memberType == nil || headType == nil {
		return nil // can't validate without resolved types.
	}

	// check if member type is a complex type with derivation.
	memberCT, ok := memberType.(*types.ComplexType)
	if !ok {
		// simple type derivation - check restriction.
		if headDecl.Final.Has(types.DerivationRestriction) {
			return fmt.Errorf("element %s cannot substitute for %s: head element is final for restriction", memberQName, headDecl.Name)
		}
		return nil
	}

	content := memberCT.Content()
	switch c := content.(type) {
	case *types.ComplexContent:
		if c.Extension != nil {
			// check if the base type matches the head's type.
			baseQName := c.Extension.Base
			if typesAreEqual(baseQName, headType) || isTypeInDerivationChain(schema, baseQName, headType) {
				if headDecl.Final.Has(types.DerivationExtension) {
					return fmt.Errorf("element %s cannot substitute for %s: head element is final for extension", memberQName, headDecl.Name)
				}
			}
		}
		if c.Restriction != nil {
			baseQName := c.Restriction.Base
			if typesAreEqual(baseQName, headType) || isTypeInDerivationChain(schema, baseQName, headType) {
				if headDecl.Final.Has(types.DerivationRestriction) {
					return fmt.Errorf("element %s cannot substitute for %s: head element is final for restriction", memberQName, headDecl.Name)
				}
			}
		}
	case *types.SimpleContent:
		if c.Extension != nil {
			if headDecl.Final.Has(types.DerivationExtension) {
				return fmt.Errorf("element %s cannot substitute for %s: head element is final for extension", memberQName, headDecl.Name)
			}
		}
		if c.Restriction != nil {
			if headDecl.Final.Has(types.DerivationRestriction) {
				return fmt.Errorf("element %s cannot substitute for %s: head element is final for restriction", memberQName, headDecl.Name)
			}
		}
	}

	return nil
}

// validateSubstitutionGroupDerivation validates that the member element's type is derived from the head element's type.
func validateSubstitutionGroupDerivation(schema *parser.Schema, memberQName types.QName, memberDecl, headDecl *types.ElementDecl) error {
	memberType := resolveTypeForFinalValidation(schema, memberDecl.Type)
	headType := resolveTypeForFinalValidation(schema, headDecl.Type)
	if memberType == nil || headType == nil {
		return nil
	}
	if !memberDecl.SubstitutionGroup.IsZero() && !memberDecl.TypeExplicit && isDefaultAnyType(memberDecl.Type) {
		memberType = headType
	}

	// anyType accepts any derived type.
	if headType.Name().Namespace == types.XSDNamespace && headType.Name().Local == "anyType" {
		return nil
	}

	// anySimpleType is the base for all simple types (including list/union).
	if headType.Name().Namespace == types.XSDNamespace && headType.Name().Local == "anySimpleType" {
		switch memberType.(type) {
		case *types.SimpleType, *types.BuiltinType:
			return nil
		}
	}

	derivedValid := memberType.Name() == headType.Name()
	if !derivedValid && !types.IsValidlyDerivedFrom(memberType, headType) {
		if memberCT, ok := memberType.(*types.ComplexType); ok {
			baseQName := memberCT.Content().BaseTypeQName()
			if typesAreEqual(baseQName, headType) || isTypeInDerivationChain(schema, baseQName, headType) {
				derivedValid = true
			}
		}
		if !derivedValid {
			return fmt.Errorf("element %s: type '%s' is not derived from substitution group head type '%s'",
				memberQName, memberType.Name(), headType.Name())
		}
	}

	return nil
}

func isDefaultAnyType(typ types.Type) bool {
	ct, ok := typ.(*types.ComplexType)
	if !ok {
		return false
	}
	return ct.QName.Namespace == types.XSDNamespace && ct.QName.Local == "anyType"
}

// typesAreEqual checks if a QName refers to the same type.
func typesAreEqual(qname types.QName, typ types.Type) bool {
	return typ.Name() == qname
}

// isTypeInDerivationChain checks if the given QName is anywhere in the derivation chain of the target type.
func isTypeInDerivationChain(schema *parser.Schema, qname types.QName, targetType types.Type) bool {
	// get the target type's name.
	targetQName := targetType.Name()

	// walk up the derivation chain from qname to see if we reach targetQName.
	current := qname
	visited := make(map[types.QName]bool)

	for !current.IsZero() && !visited[current] {
		visited[current] = true

		if current == targetQName {
			return true
		}

		typeDef, ok := schema.TypeDefs[current]
		if !ok {
			return false
		}

		ct, ok := typeDef.(*types.ComplexType)
		if !ok {
			return false
		}

		current = ct.Content().BaseTypeQName()
	}

	return false
}

// validateNoCyclicSubstitutionGroups checks for cycles in substitution group chains.
func validateNoCyclicSubstitutionGroups(schema *parser.Schema) error {
	// for each element with a substitution group, follow the chain and check for cycles.
	for startQName, decl := range schema.ElementDecls {
		if decl.SubstitutionGroup.IsZero() {
			continue
		}

		visited := make(map[types.QName]bool)
		visited[startQName] = true

		current := decl.SubstitutionGroup
		for !current.IsZero() {
			if visited[current] {
				return fmt.Errorf("cyclic substitution group detected: element %s is part of a cycle", startQName)
			}
			visited[current] = true

			nextDecl, exists := schema.ElementDecls[current]
			if !exists {
				// referenced element doesn't exist - already reported elsewhere.
				break
			}
			current = nextDecl.SubstitutionGroup
		}
	}

	return nil
}
