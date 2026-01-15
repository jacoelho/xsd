package resolver

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/schemacheck"
	"github.com/jacoelho/xsd/internal/types"
)

// collectAllIdentityConstraints collects all identity constraints from the schema
// including constraints on local elements in content models.
func collectAllIdentityConstraints(schema *parser.Schema) []*types.IdentityConstraint {
	var all []*types.IdentityConstraint
	visitedGroups := make(map[*types.ModelGroup]bool)
	visitedTypes := make(map[*types.ComplexType]bool)

	collectFromContent := func(content types.Content) {
		all = append(all, collectIdentityConstraintsFromContentWithVisited(content, visitedGroups, visitedTypes)...)
	}

	for _, decl := range schema.ElementDecls {
		all = append(all, decl.Constraints...)
		// also check inline type's content model.
		if ct, ok := decl.Type.(*types.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, typ := range schema.TypeDefs {
		if ct, ok := typ.(*types.ComplexType); ok {
			collectFromContent(ct.Content())
		}
	}

	for _, group := range schema.Groups {
		all = append(all, collectIdentityConstraintsFromParticlesWithVisited(group.Particles, visitedGroups, visitedTypes)...)
	}

	return all
}

// collectIdentityConstraintsFromContent collects identity constraints from content models.
func collectIdentityConstraintsFromContent(content types.Content) []*types.IdentityConstraint {
	visited := make(map[*types.ModelGroup]bool)
	visitedTypes := make(map[*types.ComplexType]bool)
	return collectIdentityConstraintsFromContentWithVisited(content, visited, visitedTypes)
}

// collectIdentityConstraintsFromContentWithVisited collects identity constraints with cycle detection.
func collectIdentityConstraintsFromContentWithVisited(content types.Content, visited map[*types.ModelGroup]bool, visitedTypes map[*types.ComplexType]bool) []*types.IdentityConstraint {
	var constraints []*types.IdentityConstraint
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			constraints = append(constraints, collectIdentityConstraintsFromParticlesWithVisited([]types.Particle{c.Particle}, visited, visitedTypes)...)
		}
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			constraints = append(constraints, collectIdentityConstraintsFromParticlesWithVisited([]types.Particle{c.Extension.Particle}, visited, visitedTypes)...)
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			constraints = append(constraints, collectIdentityConstraintsFromParticlesWithVisited([]types.Particle{c.Restriction.Particle}, visited, visitedTypes)...)
		}
	}
	return constraints
}

// collectIdentityConstraintsFromParticlesWithVisited collects identity constraints with cycle detection.
func collectIdentityConstraintsFromParticlesWithVisited(particles []types.Particle, visited map[*types.ModelGroup]bool, visitedTypes map[*types.ComplexType]bool) []*types.IdentityConstraint {
	var constraints []*types.IdentityConstraint
	for _, particle := range particles {
		switch p := particle.(type) {
		case *types.ElementDecl:
			constraints = append(constraints, p.Constraints...)
			// also check inline type's content model (for nested local elements).
			if ct, ok := p.Type.(*types.ComplexType); ok {
				// skip if already visited (prevents infinite recursion).
				if visitedTypes[ct] {
					continue
				}
				visitedTypes[ct] = true
				constraints = append(constraints, collectIdentityConstraintsFromContentWithVisited(ct.Content(), visited, visitedTypes)...)
			}
		case *types.ModelGroup:
			// skip if already visited (prevents infinite recursion in cyclic groups).
			if visited[p] {
				continue
			}
			visited[p] = true
			constraints = append(constraints, collectIdentityConstraintsFromParticlesWithVisited(p.Particles, visited, visitedTypes)...)
		}
	}
	return constraints
}

// validateIdentityConstraintUniqueness validates that identity constraint names are unique within the target namespace.
// Per XSD spec 3.11.2: "Constraint definition identities must be unique within an XML Schema"
// Constraints are identified by (name, target namespace).
func validateIdentityConstraintUniqueness(schema *parser.Schema) []error {
	var errs []error

	// identity constraints are identified by (name, targetNamespace) per XSD spec.
	// the targetNamespace comes from the enclosing <xs:schema> element, stored in
	// IdentityConstraint.TargetNamespace during parsing.
	//
	// map: (constraint name, target namespace) -> list of constraints with that identity.
	type constraintKey struct {
		name      string
		namespace types.NamespaceURI
	}
	constraintsByKey := make(map[constraintKey][]*types.IdentityConstraint)

	allConstraints := collectAllIdentityConstraints(schema)
	for _, constraint := range allConstraints {
		key := constraintKey{
			name:      constraint.Name,
			namespace: constraint.TargetNamespace,
		}
		constraintsByKey[key] = append(constraintsByKey[key], constraint)
	}

	// check for duplicates (more than one constraint with same identity).
	for key, constraints := range constraintsByKey {
		if len(constraints) > 1 {
			errs = append(errs, fmt.Errorf("identity constraint name '%s' is not unique within target namespace '%s' (%d definitions)",
				key.name, key.namespace, len(constraints)))
		}
	}

	return errs
}

// validateKeyrefConstraints validates keyref constraints against all known constraints.
func validateKeyrefConstraints(contextQName types.QName, constraints, allConstraints []*types.IdentityConstraint) []error {
	var errs []error

	for _, constraint := range constraints {
		if constraint.Type != types.KeyRefConstraint {
			continue
		}

		refQName := constraint.ReferQName
		if refQName.IsZero() {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' is missing refer attribute",
				contextQName, constraint.Name))
			continue
		}
		if refQName.Namespace != constraint.TargetNamespace {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' refers to '%s' in namespace '%s', which does not match target namespace '%s'",
				contextQName, constraint.Name, refQName.Local, refQName.Namespace, constraint.TargetNamespace))
			continue
		}

		var referencedConstraint *types.IdentityConstraint
		for _, other := range allConstraints {
			if other.Name == refQName.Local && other.TargetNamespace == refQName.Namespace {
				if other.Type == types.KeyConstraint || other.Type == types.UniqueConstraint {
					referencedConstraint = other
					break
				}
			}
		}

		if referencedConstraint == nil {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' references non-existent key/unique constraint '%s'",
				contextQName, constraint.Name, refQName.String()))
			continue
		}

		if len(constraint.Fields) != len(referencedConstraint.Fields) {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' has %d fields but referenced constraint '%s' has %d fields",
				contextQName, constraint.Name, len(constraint.Fields), refQName.String(), len(referencedConstraint.Fields)))
			continue
		}

		// validate field type compatibility (if types can be resolved).
		for i := 0; i < len(constraint.Fields); i++ {
			keyrefField := constraint.Fields[i]
			refField := referencedConstraint.Fields[i]

			if keyrefField.ResolvedType != nil && refField.ResolvedType != nil {
				if !areFieldTypesCompatible(keyrefField.ResolvedType, refField.ResolvedType) {
					errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' field %d type '%s' is not compatible with referenced constraint '%s' field %d type '%s'",
						contextQName, constraint.Name, i+1, keyrefField.ResolvedType.Name(),
						refQName.String(), i+1, refField.ResolvedType.Name()))
				}
			}
		}
	}

	return errs
}

// validateIdentityConstraintResolution validates that identity constraint selector and fields can be resolved.
// This validation is lenient - only definitively invalid cases are rejected.
// Resolution failures due to namespace handling, wildcards, or implementation limitations are ignored.
func validateIdentityConstraintResolution(schema *parser.Schema, constraint *types.IdentityConstraint, decl *types.ElementDecl) error {
	for i, field := range constraint.Fields {
		selectedElementType, err := schemacheck.ResolveSelectorElementType(schema, decl, constraint.Selector.XPath)
		if err != nil || selectedElementType == nil {
			continue
		}

		_, err = schemacheck.ResolveFieldType(schema, &field, decl, constraint.Selector.XPath)
		if err != nil {
			// only fail on definitively invalid cases: field '.' on element-only complex content.
			// per XSD spec Section 13.2: fields must select attributes or elements with simple content.
			if errors.Is(err, schemacheck.ErrFieldSelectsComplexContent) {
				if ct, ok := selectedElementType.(*types.ComplexType); ok && !ct.Mixed() {
					return fmt.Errorf("field %d '%s': %w", i+1, field.XPath, err)
				}
			}
		}
	}
	return nil
}

// areFieldTypesCompatible checks if two field types are compatible for keyref schemacheck.
// Types are compatible if:
// 1. They are identical
// 2. One is derived from the other
// 3. Both derive from the same primitive type
func areFieldTypesCompatible(field1Type, field2Type types.Type) bool {
	if field1Type == nil || field2Type == nil {
		return false
	}

	// same type is always compatible.
	if field1Type.Name() == field2Type.Name() {
		return true
	}

	// check if one is derived from the other.
	if isDerivedFrom(field1Type, field2Type) {
		return true
	}
	if isDerivedFrom(field2Type, field1Type) {
		return true
	}

	// check if both derive from the same primitive type.
	prim1 := getPrimitiveType(field1Type)
	prim2 := getPrimitiveType(field2Type)
	if prim1 != nil && prim2 != nil && prim1.Name() == prim2.Name() {
		return true
	}

	return false
}

// isDerivedFrom checks if type1 is derived (directly or indirectly) from type2.
// Works for both SimpleType and BuiltinType.
func isDerivedFrom(type1, type2 types.Type) bool {
	return types.IsDerivedFrom(type1, type2)
}

// getPrimitiveType returns the primitive type for a given type.
func getPrimitiveType(typ types.Type) types.Type {
	if typ == nil {
		return nil
	}

	if st, ok := typ.(*types.SimpleType); ok {
		return st.PrimitiveType()
	}

	// built-in types have PrimitiveType() method via Type interface.
	primitive := typ.PrimitiveType()
	if primitive != nil {
		return primitive
	}

	return nil
}
