package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/qname"
)

// validateTypeDefStructure validates structural constraints of a type definition
// Does not validate references (which might be forward references or imports)
func validateTypeDefStructure(schema *parser.Schema, typeQName model.QName, typ model.Type) error {
	// this is a structural constraint that is definitely invalid if violated
	if !qname.IsValidNCName(typeQName.Local) {
		return fmt.Errorf("invalid type name '%s': must be a valid NCName", typeQName.Local)
	}

	switch t := typ.(type) {
	case *model.SimpleType:
		return validateSimpleTypeStructure(schema, t)
	case *model.ComplexType:
		// named type definitions (top-level types), so isInline=false
		return validateComplexTypeStructure(schema, t, typeDefinitionGlobal)
	default:
		return fmt.Errorf("unknown type kind")
	}
}

// validateWhiteSpaceRestriction validates that a derived type's whiteSpace is not less
// restrictive than the base type's. Order: preserve (0) < replace (1) < collapse (2)
// Only checks if whiteSpace was explicitly set in the restriction.
func validateWhiteSpaceRestriction(derivedType *model.SimpleType, baseType model.Type, baseQName model.QName) error {
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
	var baseWS model.WhiteSpace
	if baseType != nil {
		switch bt := baseType.(type) {
		case *model.SimpleType:
			baseWS = bt.WhiteSpace()
		case *model.BuiltinType:
			baseWS = bt.WhiteSpace()
		}
	} else if !baseQName.IsZero() && baseQName.Namespace == model.XSDNamespace {
		// built-in type by QName
		builtinType := builtins.GetNS(baseQName.Namespace, baseQName.Local)
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
func validateNotationEnumeration(schema *parser.Schema, facetList []model.Facet) error {
	if schema == nil {
		return nil
	}
	for _, facet := range facetList {
		enum, ok := facet.(*model.Enumeration)
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
