package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// validateAttributeUniqueness validates that no two attributes in a complex type
// share the same name and namespace.
func validateAttributeUniqueness(schema *schema.Schema, ct *types.ComplexType) error {
	allAttributes := collectAllAttributesForValidation(schema, ct)

	seen := make(map[types.QName]bool)
	for _, attr := range allAttributes {
		key := effectiveAttributeQNameForValidation(schema, attr)
		if seen[key] {
			return fmt.Errorf("duplicate attribute '%s' in namespace '%s'", attr.Name.Local, attr.Name.Namespace)
		}
		seen[key] = true
	}

	return nil
}

// validateAttributeGroupUniqueness validates that no two attributes in the group
// share the same name and namespace.
func validateAttributeGroupUniqueness(schema *schema.Schema, ag *types.AttributeGroup) error {
	seen := make(map[types.QName]bool)
	for _, attr := range ag.Attributes {
		key := effectiveAttributeQNameForValidation(schema, attr)
		if seen[key] {
			return fmt.Errorf("duplicate attribute '%s' in namespace '%s'", attr.Name.Local, attr.Name.Namespace)
		}
		seen[key] = true
	}
	return nil
}
