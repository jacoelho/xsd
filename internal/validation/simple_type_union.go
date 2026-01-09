package validation

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateUnionType validates a union type definition
func validateUnionType(schema *parser.Schema, unionType *types.UnionType) error {
	// union must have at least one member type (from attribute or inline)
	if len(unionType.MemberTypes) == 0 && len(unionType.InlineTypes) == 0 {
		return fmt.Errorf("union type must have at least one member type")
	}

	// validate that all member types are simple types (not complex types)
	// union types can only have simple types as members
	for i, memberQName := range unionType.MemberTypes {
		// check if it's a built-in type (all built-in types in XSD namespace are simple)
		if memberQName.Namespace == types.XSDNamespace {
			// check if it's an XSD 1.1 type (not supported)
			if isXSD11Type(memberQName.Local) {
				return fmt.Errorf("union memberType %d: '%s' is an XSD 1.1 type (not supported in XSD 1.0)", i+1, memberQName.Local)
			}
			// built-in types in XSD namespace are always simple types
			continue
		}

		if memberType, ok := schema.TypeDefs[memberQName]; ok {
			// union members must be simple types, not complex types
			if _, isComplex := memberType.(*types.ComplexType); isComplex {
				return fmt.Errorf("union memberType %d: '%s' is a complex type (union types can only have simple types as members)", i+1, memberQName.Local)
			}
		}
	}

	// validate inline types (they're already SimpleType, so no need to check)
	// inline types are parsed as SimpleType, so they're always valid

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
