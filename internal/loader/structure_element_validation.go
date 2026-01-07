package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateElementDeclStructure validates structural constraints of an element declaration
// Does not validate references (which might be forward references or imports)
func validateElementDeclStructure(schema *schema.Schema, qname types.QName, decl *types.ElementDecl) error {
	// validate element name is a valid NCName (no spaces, valid XML name)
	// this is a structural constraint that is definitely invalid if violated
	if !isValidNCName(qname.Local) {
		return fmt.Errorf("invalid element name '%s': must be a valid NCName", qname.Local)
	}

	// element references don't need type validation - they inherit type from referenced element
	if decl.IsReference {
		return nil
	}

	// elements without explicit types default to anyType (handled by parser)
	// no need to check for nil Type here since parser always sets a type

	// validate identity constraints (key, keyref, unique)
	// identity constraints can only be placed on elements with complex types
	for _, constraint := range decl.Constraints {
		if err := validateIdentityConstraint(schema, constraint, decl); err != nil {
			return fmt.Errorf("identity constraint '%s': %w", constraint.Name, err)
		}
	}

	constraintNames := make(map[string]bool)
	for _, constraint := range decl.Constraints {
		if constraintNames[constraint.Name] {
			return fmt.Errorf("element %s: duplicate identity constraint name '%s'", decl.Name, constraint.Name)
		}
		constraintNames[constraint.Name] = true
	}

	// resolve field types for all constraints (after type resolution is complete)
	// field type resolution may fail for forward references or incomplete schemas,
	// so we don't fail validation if resolution fails - it will be caught later
	for _, constraint := range decl.Constraints {
		for i := range constraint.Fields {
			resolvedType, err := resolveFieldType(schema, &constraint.Fields[i], decl, constraint.Selector.XPath)
			if err == nil {
				constraint.Fields[i].ResolvedType = resolvedType
			}
			// if resolution fails, leave ResolvedType as nil - will be caught during reference validation
		}
	}

	// validate inline types (simpleType or complexType defined inline in the element)
	if decl.Type != nil {
		if st, ok := decl.Type.(*types.SimpleType); ok {
			if err := validateSimpleTypeStructure(schema, st); err != nil {
				return fmt.Errorf("inline simpleType: %w", err)
			}
		} else if ct, ok := decl.Type.(*types.ComplexType); ok {
			// global elements use isInline=false; the facet restriction constraint only applies to local elements
			if err := validateComplexTypeStructure(schema, ct, false); err != nil {
				return fmt.Errorf("inline complexType: %w", err)
			}
		}
	}

	// validate default value if present (basic validation only - full type checking in reference_validation.go)
	if decl.Default != "" {
		if err := validateDefaultOrFixedValue(decl.Default, decl.Type); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}

	// validate fixed value if present (basic validation only - full type checking in reference_validation.go)
	if decl.Fixed != "" {
		if err := validateDefaultOrFixedValue(decl.Fixed, decl.Type); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}

	// don't validate type references - they might be forward references or from imports
	// don't validate substitution group references - they might be forward references

	return nil
}