package schemacheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

// validateDefaultOrFixedValue validates that a default or fixed value is valid for the given type.
// This is used during structure validation; full facet validation happens after resolution.
func validateDefaultOrFixedValue(value string, typ types.Type) error {
	if typ == nil {
		// type not resolved yet - might be forward reference, skip validation
		return nil
	}

	// normalize value according to type's whitespace facet before validation
	// even for basic validation, we need to normalize to match XSD spec behavior
	normalizedValue := types.NormalizeWhiteSpace(value, typ)

	if isQNameOrNotationType(typ) {
		if _, err := types.ParseQNameValue(normalizedValue, nsContext); err != nil {
			return err
		}
	}

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
		// check if derived from ID type
		if isIDOnlyDerivedType(st) {
			return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
		}
		// SimpleType.Validate normalizes internally, but we pass normalized value for consistency
		if err := st.Validate(normalizedValue); err != nil {
			return err
		}
		return nil
	}

	// handle complex types with simpleContent - need to validate against the text type
	if ct, ok := typ.(*types.ComplexType); ok {
		if content, ok := ct.Content().(*types.SimpleContent); ok {
			// for extension or restriction, validate against the base type
			var baseQName types.QName
			if content.Extension != nil {
				baseQName = content.Extension.Base
			} else if content.Restriction != nil {
				baseQName = content.Restriction.Base
			}

			if !baseQName.IsZero() {
				// check if base is a built-in type
				bt := types.GetBuiltinNS(baseQName.Namespace, baseQName.Local)
				if bt != nil {
					if err := validateDefaultOrFixedValue(value, bt, nsContext); err != nil {
						if isIDOnlyType(baseQName) {
							return fmt.Errorf("type '%s' (with simpleContent from ID) cannot have default or fixed values", typ.Name().Local)
						}
						return err
					}
				}
			}
		}
	}

	return nil
}

func isQNameOrNotationType(typ types.Type) bool {
	if typ == nil {
		return false
	}
	switch t := typ.(type) {
	case *types.SimpleType:
		return t.IsQNameOrNotationType()
	case *types.BuiltinType:
		return t.IsQNameOrNotationType()
	default:
		return types.IsQNameOrNotation(typ.Name())
	}
}
