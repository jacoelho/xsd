package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateUnionType validates a union type definition
func validateUnionType(schema *schema.Schema, unionType *types.UnionType) error {
	// Union must have at least one member type (from attribute or inline)
	if len(unionType.MemberTypes) == 0 && len(unionType.InlineTypes) == 0 {
		return fmt.Errorf("union type must have at least one member type")
	}

	// Validate that all member types are simple types (not complex types)
	// Union types can only have simple types as members
	for i, memberQName := range unionType.MemberTypes {
		// Check if it's a built-in type (all built-in types in XSD namespace are simple)
		if memberQName.Namespace == types.XSDNamespace {
			// Check if it's an XSD 1.1 type (not supported)
			if isXSD11Type(memberQName.Local) {
				return fmt.Errorf("union memberType %d: '%s' is an XSD 1.1 type (not supported in XSD 1.0)", i+1, memberQName.Local)
			}
			// Built-in types in XSD namespace are always simple types
			continue
		}

		if memberType, ok := schema.TypeDefs[memberQName]; ok {
			// Union members must be simple types, not complex types
			if _, isComplex := memberType.(*types.ComplexType); isComplex {
				return fmt.Errorf("union memberType %d: '%s' is a complex type (union types can only have simple types as members)", i+1, memberQName.Local)
			}
		}
	}

	// Validate inline types (they're already SimpleType, so no need to check)
	// Inline types are parsed as SimpleType, so they're always valid

	return nil
}

// isXSD11Type checks if a type name is an XSD 1.1 type (not supported in XSD 1.0)
func isXSD11Type(typeName string) bool {
	xsd11Types := map[string]bool{
		"timeDuration":      true,
		"yearMonthDuration": true,
		"dayTimeDuration":   true,
		"dateTimeStamp":     true,
		"precisionDecimal":  true,
	}
	return xsd11Types[typeName]
}
