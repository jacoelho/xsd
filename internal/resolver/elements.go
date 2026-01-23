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

	if typesMatch(memberType, headType) {
		return nil
	}

	current := memberType
	visited := make(map[types.Type]bool)
	for current != nil && !typesMatch(current, headType) {
		if visited[current] {
			break
		}
		visited[current] = true

		base, method, err := derivationStep(schema, current)
		if err != nil {
			return fmt.Errorf("resolve substitution group derivation for %s: %w", memberQName, err)
		}
		if method != 0 && headDecl.Final.Has(method) {
			return fmt.Errorf("element %s cannot substitute for %s: head element is final for %s",
				memberQName, headDecl.Name, derivationMethodLabel(method))
		}
		current = base
	}

	return nil
}

// validateSubstitutionGroupDerivation validates that the member element's type is derived from the head element's type.
func validateSubstitutionGroupDerivation(schema *parser.Schema, memberQName types.QName, memberDecl, headDecl *types.ElementDecl) error {
	if isDefaultAnyType(memberDecl) && headDecl.Type != nil {
		memberDecl.Type = headDecl.Type
	}
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

	derivedValid := memberType == headType
	if !derivedValid {
		memberName := memberType.Name()
		headName := headType.Name()
		if !memberName.IsZero() && !headName.IsZero() && memberName == headName {
			derivedValid = true
		}
	}
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

func isAnyType(typ types.Type) bool {
	if typ == nil {
		return false
	}
	name := typ.Name()
	return name.Namespace == types.XSDNamespace && name.Local == "anyType"
}

func isDefaultAnyType(decl *types.ElementDecl) bool {
	if decl == nil || decl.TypeExplicit {
		return false
	}
	return isAnyType(decl.Type)
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

func typesMatch(a, b types.Type) bool {
	if a == nil || b == nil {
		return false
	}
	if a == b {
		return true
	}
	nameA := a.Name()
	nameB := b.Name()
	return !nameA.IsZero() && nameA == nameB
}

func derivationStep(schema *parser.Schema, typ types.Type) (types.Type, types.DerivationMethod, error) {
	switch typed := typ.(type) {
	case *types.BuiltinType:
		name := typed.Name().Local
		if name == string(types.TypeNameAnyType) {
			return nil, 0, nil
		}
		if name == string(types.TypeNameAnySimpleType) {
			return types.GetBuiltin(types.TypeNameAnyType), types.DerivationRestriction, nil
		}
		if st, ok := types.AsSimpleType(typed); ok && st.List != nil {
			return types.GetBuiltin(types.TypeNameAnySimpleType), types.DerivationList, nil
		}
		return typed.BaseType(), types.DerivationRestriction, nil
	case *types.ComplexType:
		if typed.DerivationMethod == 0 {
			return typed.ResolvedBase, 0, nil
		}
		base := typed.ResolvedBase
		if base == nil {
			baseQName := typed.Content().BaseTypeQName()
			if !baseQName.IsZero() {
				resolved, err := lookupType(schema, baseQName)
				if err != nil {
					return nil, typed.DerivationMethod, err
				}
				base = resolved
			}
		}
		return base, typed.DerivationMethod, nil
	case *types.SimpleType:
		if typed.List != nil {
			return types.GetBuiltin(types.TypeNameAnySimpleType), types.DerivationList, nil
		}
		if typed.Union != nil {
			return types.GetBuiltin(types.TypeNameAnySimpleType), types.DerivationUnion, nil
		}
		if typed.Restriction != nil {
			base := typed.ResolvedBase
			if base == nil && typed.Restriction.SimpleType != nil {
				base = typed.Restriction.SimpleType
			}
			if base == nil && !typed.Restriction.Base.IsZero() {
				resolved, err := lookupType(schema, typed.Restriction.Base)
				if err != nil {
					return nil, types.DerivationRestriction, err
				}
				base = resolved
			}
			return base, types.DerivationRestriction, nil
		}
	}
	return nil, 0, nil
}

func derivationMethodLabel(method types.DerivationMethod) string {
	switch method {
	case types.DerivationExtension:
		return "extension"
	case types.DerivationRestriction:
		return "restriction"
	case types.DerivationList:
		return "list"
	case types.DerivationUnion:
		return "union"
	default:
		return "unknown"
	}
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
