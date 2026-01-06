package loader

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

// validateDefaultOrFixedValue validates that a default or fixed value is valid for the given type
// This is used during schema validation to ensure default/fixed values are valid
// Basic validation only - full validation with facets happens in reference_validation.go
func validateDefaultOrFixedValue(value string, typ types.Type) error {
	if typ == nil {
		// Type not resolved yet - might be forward reference, skip validation
		return nil
	}

	// Normalize value according to type's whitespace facet before validation
	// Even for basic validation, we need to normalize to match XSD spec behavior
	normalizedValue := types.NormalizeWhiteSpace(value, typ)

	if typ.IsBuiltin() {
		bt := types.GetBuiltinNS(typ.Name().Namespace, typ.Name().Local)
		if bt != nil {
			// ID types cannot have default or fixed values
			if isIDOnlyType(typ.Name()) {
				return fmt.Errorf("type '%s' cannot have default or fixed values", typ.Name().Local)
			}
			if err := bt.Validate(normalizedValue); err != nil {
				return err
			}
		}
		return nil
	}

	if st, ok := typ.(*types.SimpleType); ok {
		// Check if derived from ID type
		if isIDOnlyDerivedType(st) {
			return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
		}
		// SimpleType.Validate normalizes internally, but we pass normalized value for consistency
		if err := st.Validate(normalizedValue); err != nil {
			return err
		}
		return nil
	}

	// Handle complex types with simpleContent - need to validate against the text type
	if ct, ok := typ.(*types.ComplexType); ok {
		if content, ok := ct.Content().(*types.SimpleContent); ok {
			// For extension or restriction, validate against the base type
			var baseQName types.QName
			if content.Extension != nil {
				baseQName = content.Extension.Base
			} else if content.Restriction != nil {
				baseQName = content.Restriction.Base
			}

			if !baseQName.IsZero() {
				// Check if base is a built-in type
				bt := types.GetBuiltinNS(baseQName.Namespace, baseQName.Local)
				if bt != nil {
					// ID types cannot have default or fixed values
					if isIDOnlyType(baseQName) {
						return fmt.Errorf("type '%s' (with simpleContent from ID) cannot have default or fixed values", typ.Name().Local)
					}
					// Normalize for the base type's whitespace facet
					baseNormalized := types.NormalizeWhiteSpace(value, bt)
					if err := bt.Validate(baseNormalized); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
