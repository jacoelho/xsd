package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/facetvalue"
	model "github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	qnamelex "github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/typechain"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

// validateDefaultOrFixedValueWithContext validates that a default or fixed value is valid for the given type.
// This is used during structure validation; full facet validation happens after resolution.
func validateDefaultOrFixedValueWithContext(schema *parser.Schema, value string, typ model.Type, nsContext map[string]string) error {
	if typ == nil {
		// type not resolved yet - might be forward reference, skip validation
		return nil
	}

	if shouldDeferValueValidation(schema, typ) {
		return nil
	}

	if st, ok := typ.(*model.SimpleType); ok && model.IsPlaceholderSimpleType(st) && schema != nil {
		if resolved, ok := typechain.LookupType(schema, st.QName); ok {
			return validateDefaultOrFixedValueWithContext(schema, value, resolved, nsContext)
		}
	}

	// normalize value according to type's whitespace facet before validation
	// even for basic validation, we need to normalize to match XSD spec behavior
	normalizedValue := model.NormalizeWhiteSpace(value, typ)

	if facetvalue.IsQNameOrNotationType(typ) {
		if nsContext == nil {
			// Can't validate QName without context, skip for now
			return nil
		}
		if _, err := qnamelex.ParseQNameValue(normalizedValue, nsContext); err != nil {
			return err
		}
	}

	if typ.IsBuiltin() {
		bt := builtins.GetNS(typ.Name().Namespace, typ.Name().Local)
		if bt != nil {
			// ID types cannot have default or fixed values
			if typeresolve.IsIDOnlyType(typ.Name()) {
				return fmt.Errorf("type '%s' cannot have default or fixed values", typ.Name().Local)
			}
			if err := bt.Validate(normalizedValue); err != nil {
				return err
			}
		}
		return nil
	}

	if st, ok := typ.(*model.SimpleType); ok {
		// check if derived from ID type
		// First check the direct base QName
		if st.Restriction != nil && !st.Restriction.Base.IsZero() {
			if typeresolve.IsIDOnlyType(st.Restriction.Base) {
				return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
			}
		}
		// Also check resolved base if available
		if st.ResolvedBase != nil {
			if bt, ok := st.ResolvedBase.(*model.BuiltinType); ok && typeresolve.IsIDOnlyType(bt.Name()) {
				return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
			}
			if baseST, ok := st.ResolvedBase.(*model.SimpleType); ok && schema != nil && typeresolve.IsIDOnlyDerivedType(schema, baseST) {
				return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
			}
		} else if schema != nil && typeresolve.IsIDOnlyDerivedType(schema, st) {
			return fmt.Errorf("type '%s' (derived from ID) cannot have default or fixed values", typ.Name().Local)
		}
		return validateValueAgainstTypeWithFacets(schema, value, st, nsContext)
	}

	// handle complex types with simpleContent - need to validate against the text type
	if ct, ok := typ.(*model.ComplexType); ok {
		if content, ok := ct.Content().(*model.SimpleContent); ok {
			// for extension or restriction, validate against the base type
			var baseQName model.QName
			if content.Extension != nil {
				baseQName = content.Extension.Base
			} else if content.Restriction != nil {
				baseQName = content.Restriction.Base
			}

			if !baseQName.IsZero() {
				// check if base is a built-in type
				bt := builtins.GetNS(baseQName.Namespace, baseQName.Local)
				if bt != nil {
					if err := validateDefaultOrFixedValueWithContext(schema, value, bt, nsContext); err != nil {
						if typeresolve.IsIDOnlyType(baseQName) {
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

func shouldDeferValueValidation(schema *parser.Schema, typ model.Type) bool {
	st, ok := typ.(*model.SimpleType)
	if !ok {
		return false
	}
	if model.IsPlaceholderSimpleType(st) {
		if schema == nil {
			return true
		}
		if _, ok := typechain.LookupType(schema, st.QName); !ok {
			return true
		}
		return false
	}
	if st.Variety() == model.AtomicVariety && st.PrimitiveType() == nil {
		if st.Restriction != nil && !st.Restriction.Base.IsZero() && st.Restriction.Base.Namespace != model.XSDNamespace {
			return true
		}
	}
	return false
}
