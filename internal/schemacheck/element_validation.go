package schemacheck

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateElementDeclStructure validates structural constraints of an element declaration
// Does not validate references (which might be forward references or imports)
func validateElementDeclStructure(schema *parser.Schema, qname types.QName, decl *types.ElementDecl) error {
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
		if err := validateIdentityConstraint(constraint); err != nil {
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
			resolvedType, err := ResolveFieldType(schema, &constraint.Fields[i], decl, constraint.Selector.XPath, constraint.NamespaceContext)
			if err != nil {
				// Nillable element check only applies to xs:key constraints
				// For xs:unique and xs:keyref, nillable elements are allowed (nil values are excluded)
				if errors.Is(err, ErrFieldSelectsNillable) {
					if resolvedType != nil {
						constraint.Fields[i].ResolvedType = resolvedType
					}
					if constraint.Type == types.KeyConstraint {
						return fmt.Errorf("element %s identity constraint '%s': field %d '%s' selects nillable element", decl.Name, constraint.Name, i+1, constraint.Fields[i].XPath)
					}
					// For unique/keyref, ignore nillable error and continue
					continue
				}
				if errors.Is(err, ErrFieldSelectsComplexContent) {
					continue
				}
				// For union fields with incompatible types, report the error during structure validation
				// UNLESS it's due to selector union (which is allowed - field can have different types for different selector paths)
				// (reference validation will allow them if needed)
				if errors.Is(err, ErrFieldXPathIncompatibleTypes) {
					// Check if error was converted to unresolvable (which means selector union allowed it)
					if !errors.Is(err, ErrXPathUnresolvable) {
						return fmt.Errorf("identity constraint '%s': field %d '%s': %w", constraint.Name, i+1, constraint.Fields[i].XPath, err)
					}
					// If it was converted to unresolvable, treat as unresolvable (allow it)
				}
				// For other errors (unresolvable), leave ResolvedType as nil - will be caught during reference validation
				if errors.Is(err, ErrXPathUnresolvable) {
					continue
				}
				// For incompatible types that weren't converted, report the error
				if errors.Is(err, ErrFieldXPathIncompatibleTypes) {
					return fmt.Errorf("identity constraint '%s': field %d '%s': %w", constraint.Name, i+1, constraint.Fields[i].XPath, err)
				}
				continue
			}
			constraint.Fields[i].ResolvedType = resolvedType
		}
	}

	// validate inline types (simpleType or complexType defined inline in the element)
	if decl.Type != nil {
		switch typ := decl.Type.(type) {
		case *types.SimpleType:
			if err := validateSimpleTypeStructure(schema, typ); err != nil {
				return fmt.Errorf("inline simpleType: %w", err)
			}
		case *types.ComplexType:
			if err := validateComplexTypeStructure(schema, typ, typeDefinitionGlobal); err != nil {
				return fmt.Errorf("inline complexType: %w", err)
			}
		}
	}

	// validate default value if present (basic validation only - full type checking after resolution)
	if decl.HasDefault {
		if err := validateDefaultOrFixedValueWithContext(schema, decl.Default, decl.Type, decl.DefaultContext); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}

	// validate fixed value if present (basic validation only - full type checking after resolution)
	if decl.HasFixed {
		if err := validateDefaultOrFixedValueWithContext(schema, decl.Fixed, decl.Type, decl.FixedContext); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}

	// don't validate type references - they might be forward references or from imports
	// don't validate substitution group references - they might be forward references

	return nil
}
