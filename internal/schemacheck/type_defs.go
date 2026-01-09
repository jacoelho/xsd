package schemacheck

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// isValidNCName checks if a string is a valid NCName
func isValidNCName(s string) bool {
	return types.IsValidNCName(s)
}

// elementTypesCompatible checks if two element declaration types are consistent.
// Treats nil types as compatible only when both are nil (implicit anyType).
func elementTypesCompatible(a, b types.Type) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	nameA := a.Name()
	nameB := b.Name()
	if !nameA.IsZero() || !nameB.IsZero() {
		return nameA == nameB
	}

	return a == b
}

// resolveTypeReference resolves a type reference in schema validation contexts.
func resolveTypeReference(schema *parser.Schema, typ types.Type, allowMissing bool) types.Type {
	if typ == nil {
		return nil
	}

	// if it's a placeholder SimpleType (has QName but not builtin and no Restriction/List/Union),
	if st, ok := typ.(*types.SimpleType); ok {
		// check if it's a placeholder: not builtin, has QName, but no Restriction/List/Union
		if !st.IsBuiltin() && st.Restriction == nil && st.List == nil && st.Union == nil {
			// this is a placeholder - resolve from schema.TypeDefs
			if resolvedType, ok := schema.TypeDefs[st.QName]; ok {
				return resolvedType
			}
			if allowMissing {
				return typ
			}
			// type not found - return nil to indicate error
			return nil
		}
		// not a placeholder, return as-is
		return typ
	}

	// already a resolved type (ComplexType or non-placeholder SimpleType)
	return typ
}

// resolveTypeForFinalValidation resolves a type reference for substitution group final checks.
func resolveTypeForFinalValidation(schema *parser.Schema, typ types.Type) types.Type {
	return resolveTypeReference(schema, typ, true)
}

// validateTypeDefStructure validates structural constraints of a type definition
// Does not validate references (which might be forward references or imports)
func validateTypeDefStructure(schema *parser.Schema, qname types.QName, typ types.Type) error {
	// this is a structural constraint that is definitely invalid if violated
	if !isValidNCName(qname.Local) {
		return fmt.Errorf("invalid type name '%s': must be a valid NCName", qname.Local)
	}

	switch t := typ.(type) {
	case *types.SimpleType:
		return validateSimpleTypeStructure(schema, t)
	case *types.ComplexType:
		// named type definitions (top-level types), so isInline=false
		return validateComplexTypeStructure(schema, t, false)
	default:
		return fmt.Errorf("unknown type kind")
	}
}

// validateWhiteSpaceRestriction validates that a derived type's whiteSpace is not less
// restrictive than the base type's. Order: preserve (0) < replace (1) < collapse (2)
// Only checks if whiteSpace was explicitly set in the restriction.
func validateWhiteSpaceRestriction(derivedType *types.SimpleType, baseType types.Type, baseQName types.QName) error {
	if derivedType == nil {
		return nil
	}

	// only validate if whiteSpace was explicitly set in this restriction
	// if not explicitly set, the type inherits from base (which is valid)
	if !derivedType.WhiteSpaceExplicit() {
		return nil
	}

	derivedWS := derivedType.WhiteSpace()

	// get base type's whiteSpace
	var baseWS types.WhiteSpace
	if baseType != nil {
		if baseST, ok := baseType.(*types.SimpleType); ok {
			baseWS = baseST.WhiteSpace()
		} else if baseBT, ok := baseType.(*types.BuiltinType); ok {
			baseWS = baseBT.WhiteSpace()
		}
	} else if !baseQName.IsZero() && baseQName.Namespace == types.XSDNamespace {
		// built-in type by QName
		bt := types.GetBuiltinNS(baseQName.Namespace, baseQName.Local)
		if bt != nil {
			baseWS = bt.WhiteSpace()
		}
	}

	// preserve = 0, replace = 1, collapse = 2
	if derivedWS < baseWS {
		return fmt.Errorf("whiteSpace facet value '%s' cannot be less restrictive than base type's '%s'",
			whiteSpaceName(derivedWS), whiteSpaceName(baseWS))
	}

	return nil
}

// validateNotationEnumeration validates that enumeration values for NOTATION types
// reference declared notations in the schema
func validateNotationEnumeration(schema *parser.Schema, facetList []types.Facet, targetNS types.NamespaceURI) error {
	var enumValues []string
	for _, f := range facetList {
		if enum, ok := f.(*types.Enumeration); ok {
			enumValues = append(enumValues, enum.Values...)
		}
	}

	if len(enumValues) == 0 {
		return nil
	}

	// per XSD spec, NOTATION enumeration values cannot be empty strings
	for _, val := range enumValues {
		if val == "" {
			return fmt.Errorf("NOTATION enumeration value cannot be empty")
		}
		// parse value as QName (may be prefixed like "ns:notation")
		var qname types.QName
		if before, after, ok := strings.Cut(val, ":"); ok {
			// prefixed QName - resolve prefix to namespace
			prefix := before
			local := after

			ns, ok := schema.NamespaceDecls[prefix]
			if !ok {
				return fmt.Errorf("enumeration value %q uses undeclared prefix %q", val, prefix)
			}
			qname = types.QName{Local: local, Namespace: types.NamespaceURI(ns)}
		} else {
			// unprefixed - use target namespace
			qname = types.QName{Local: val, Namespace: targetNS}
		}

		if _, ok := schema.NotationDecls[qname]; !ok {
			return fmt.Errorf("enumeration value %q does not reference a declared notation", val)
		}
	}

	return nil
}
