package schemacheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// validateDefaultOrFixedValueWithContext validates that a default or fixed value is valid for the given type.
// This is used during structure validation; full facet validation happens after resolution.
func validateDefaultOrFixedValueWithContext(schema *parser.Schema, value string, typ types.Type, nsContext map[string]string) error {
	if typ == nil {
		// type not resolved yet - might be forward reference, skip validation
		return nil
	}

	if shouldDeferValueValidation(schema, typ) {
		return nil
	}

	if st, ok := typ.(*types.SimpleType); ok && types.IsPlaceholderSimpleType(st) && schema != nil {
		if resolved, ok := lookupTypeDef(schema, st.QName); ok {
			return validateDefaultOrFixedValueWithContext(schema, value, resolved, nsContext)
		}
	}

	// normalize value according to type's whitespace facet before validation
	// even for basic validation, we need to normalize to match XSD spec behavior
	normalizedValue := types.NormalizeWhiteSpace(value, typ)

	if types.IsQNameOrNotationType(typ) {
		if nsContext == nil {
			// Can't validate QName without context, skip for now
			return nil
		}
		if _, err := types.ParseQNameValue(normalizedValue, nsContext); err != nil {
			return err
		}
	}

	if typ.IsBuiltin() {
		bt := types.GetBuiltinNS(typ.Name().Namespace, typ.Name().Local)
		if bt != nil {
			// ID types cannot have default or fixed values
			if typeops.IsIDOnlyType(typ.Name()) {
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
		// First check the direct base QName
		if st.Restriction != nil && !st.Restriction.Base.IsZero() {
			if typeops.IsIDOnlyType(st.Restriction.Base) {
				return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
			}
		}
		// Also check resolved base if available
		if st.ResolvedBase != nil {
			if bt, ok := st.ResolvedBase.(*types.BuiltinType); ok && typeops.IsIDOnlyType(bt.Name()) {
				return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
			}
			if baseST, ok := st.ResolvedBase.(*types.SimpleType); ok && schema != nil && typeops.IsIDOnlyDerivedType(schema, baseST) {
				return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
			}
		} else if schema != nil && typeops.IsIDOnlyDerivedType(schema, st) {
			return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
		}
		return validateValueAgainstTypeWithFacets(schema, value, st, nsContext, make(map[types.Type]bool))
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
					if err := validateDefaultOrFixedValueWithContext(schema, value, bt, nsContext); err != nil {
						if typeops.IsIDOnlyType(baseQName) {
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

func shouldDeferValueValidation(schema *parser.Schema, typ types.Type) bool {
	st, ok := typ.(*types.SimpleType)
	if !ok {
		return false
	}
	if types.IsPlaceholderSimpleType(st) {
		if schema == nil {
			return true
		}
		if _, ok := lookupTypeDef(schema, st.QName); !ok {
			return true
		}
		return false
	}
	if st.Variety() == types.AtomicVariety && st.PrimitiveType() == nil {
		if st.Restriction != nil && !st.Restriction.Base.IsZero() && st.Restriction.Base.Namespace != types.XSDNamespace {
			return true
		}
	}
	return false
}
