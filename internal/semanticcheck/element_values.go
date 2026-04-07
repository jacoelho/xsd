package semanticcheck

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
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
		if resolved, ok := semantics.LookupType(schema, st.QName); ok {
			typ = resolved
		}
	}
	if shouldDeferValueValidation(schema, typ) {
		return nil
	}

	if model.IsQNameOrNotationType(typ) && nsContext == nil {
		return nil
	}

	return semantics.ValidateDefaultOrFixedResolved(
		schema,
		value,
		typ,
		nsContext,
		semantics.IDPolicyDisallow,
	)
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
		if _, ok := semantics.LookupType(schema, st.QName); !ok {
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
