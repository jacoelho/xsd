package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// isValidNCName checks if a string is a valid NCName
func isValidNCName(s string) bool {
	return types.IsValidNCName(s)
}

// ElementTypesCompatible checks if two element declaration types are consistent.
// Treats nil types as compatible only when both are nil (implicit anyType).
func ElementTypesCompatible(a, b types.Type) bool {
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

// ResolveTypeReference resolves a type reference in schema validation contexts.
func ResolveTypeReference(schema *parser.Schema, typ types.Type, policy typeops.TypeReferencePolicy) types.Type {
	return typeops.ResolveTypeReference(schema, typ, policy)
}

// resolveTypeForFinalValidation resolves a type reference for substitution group final checks.
func resolveTypeForFinalValidation(schema *parser.Schema, typ types.Type) types.Type {
	return ResolveTypeReference(schema, typ, typeops.TypeReferenceAllowMissing)
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
		return validateComplexTypeStructure(schema, t, typeDefinitionGlobal)
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
		switch bt := baseType.(type) {
		case *types.SimpleType:
			baseWS = bt.WhiteSpace()
		case *types.BuiltinType:
			baseWS = bt.WhiteSpace()
		}
	} else if !baseQName.IsZero() && baseQName.Namespace == types.XSDNamespace {
		// built-in type by QName
		builtinType := types.GetBuiltinNS(baseQName.Namespace, baseQName.Local)
		if builtinType != nil {
			baseWS = builtinType.WhiteSpace()
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
// reference declared notations in the schema.
func validateNotationEnumeration(schema *parser.Schema, facetList []types.Facet) error {
	if schema == nil {
		return nil
	}
	for _, facet := range facetList {
		enum, ok := facet.(*types.Enumeration)
		if !ok {
			continue
		}
		values := enum.Values()
		qnames, err := enum.ResolveQNameValues()
		if err != nil {
			return err
		}
		for i, qname := range qnames {
			if _, ok := schema.NotationDecls[qname]; !ok {
				return fmt.Errorf("enumeration value %q does not reference a declared notation", values[i])
			}
		}
	}
	return nil
}
