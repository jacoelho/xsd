package schemacheck

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

type facetConstraintState struct {
	minExclusive   *string
	maxExclusive   *string
	minInclusive   *string
	maxInclusive   *string
	length         *int
	minLength      *int
	maxLength      *int
	totalDigits    *int
	fractionDigits *int
	hasEnumeration bool
}

func isValidFacetName(name string) bool {
	switch name {
	case "length", "minLength", "maxLength", "pattern", "enumeration", "whiteSpace",
		"maxInclusive", "maxExclusive", "minInclusive", "minExclusive",
		"totalDigits", "fractionDigits":
		return true
	default:
		return false
	}
}

// ValidateFacetConstraints validates facet consistency and values for a base type.
func ValidateFacetConstraints(schema *parser.Schema, facetList []types.Facet, baseType types.Type, baseQName types.QName) error {
	baseTypeName := baseQName.Local
	isBuiltin := baseQName.Namespace == types.XSDNamespace
	var bt *types.BuiltinType
	if isBuiltin {
		bt = types.GetBuiltin(types.TypeName(baseTypeName))
	}

	state := facetConstraintState{}

	for _, facet := range facetList {
		name := facet.Name()

		// validate that the facet is a known XSD facet
		if !isValidFacetName(name) {
			return fmt.Errorf("unknown or invalid facet '%s' (not a valid XSD 1.0 facet)", name)
		}

		if err := state.captureFacet(name, facet); err != nil {
			return err
		}

		if err := types.ValidateFacetApplicability(name, baseType, baseQName); err != nil {
			return err
		}
	}

	if err := validateLengthFacetConstraints(&state, baseType, baseQName, baseTypeName); err != nil {
		return err
	}

	if err := validateRangeFacets(state.minExclusive, state.maxExclusive, state.minInclusive, state.maxInclusive, baseType, baseQName); err != nil {
		return err
	}

	// validate that range facet values are within the base type's value space
	if err := validateRangeFacetValues(state.minExclusive, state.maxExclusive, state.minInclusive, state.maxInclusive, baseType, bt); err != nil {
		return err
	}

	// per XSD spec: fractionDigits must be <= totalDigits
	if err := validateDigitsConstraints(&state, baseType, baseTypeName, isBuiltin); err != nil {
		return err
	}

	// validate enumeration values if base type is known
	if state.hasEnumeration && baseType != nil {
		if err := validateEnumerationValues(schema, facetList, baseType); err != nil {
			return err
		}
	}

	return nil
}

func (s *facetConstraintState) captureFacet(name string, facet types.Facet) error {
	switch name {
	case "minExclusive", "maxExclusive", "minInclusive", "maxInclusive":
		// all range facets are generic and implement LexicalFacet
		if lf, ok := facet.(types.LexicalFacet); ok {
			val := lf.GetLexical()
			if val == "" {
				return nil
			}
			switch name {
			case "minExclusive":
				s.minExclusive = &val
			case "maxExclusive":
				s.maxExclusive = &val
			case "minInclusive":
				s.minInclusive = &val
			case "maxInclusive":
				s.maxInclusive = &val
			}
		}
	case "length":
		if ivf, ok := facet.(types.IntValueFacet); ok {
			val := ivf.GetIntValue()
			s.length = &val
		}
	case "minLength":
		if ivf, ok := facet.(types.IntValueFacet); ok {
			val := ivf.GetIntValue()
			s.minLength = &val
		}
	case "maxLength":
		if ivf, ok := facet.(types.IntValueFacet); ok {
			val := ivf.GetIntValue()
			s.maxLength = &val
		}
	case "enumeration":
		s.hasEnumeration = true
	case "totalDigits":
		if ivf, ok := facet.(types.IntValueFacet); ok {
			val := ivf.GetIntValue()
			s.totalDigits = &val
		}
	case "fractionDigits":
		if ivf, ok := facet.(types.IntValueFacet); ok {
			val := ivf.GetIntValue()
			s.fractionDigits = &val
		}
	case "pattern":
		if patternFacet, ok := facet.(interface{ ValidateSyntax() error }); ok {
			if err := patternFacet.ValidateSyntax(); err != nil {
				return fmt.Errorf("pattern facet: %w", err)
			}
		}
	}
	return nil
}

func validateLengthFacetConstraints(state *facetConstraintState, baseType types.Type, baseQName types.QName, baseTypeName string) error {
	if state.length != nil && (state.minLength != nil || state.maxLength != nil) {
		if !isListTypeForFacets(baseType, baseQName) {
			return fmt.Errorf("length facet cannot be used together with minLength or maxLength")
		}
		if state.maxLength != nil {
			return fmt.Errorf("length facet cannot be used together with maxLength for list types")
		}
	}

	if state.minLength != nil && state.maxLength != nil {
		if *state.minLength > *state.maxLength {
			return fmt.Errorf("minLength (%d) must be <= maxLength (%d)", *state.minLength, *state.maxLength)
		}
	}

	// built-in list types require at least one item.
	if types.IsBuiltinListTypeName(baseTypeName) {
		if state.length != nil && *state.length < 1 {
			return fmt.Errorf("length (%d) must be >= 1 for list type %s", *state.length, baseTypeName)
		}
		if state.minLength != nil && *state.minLength < 1 {
			return fmt.Errorf("minLength (%d) must be >= 1 for list type %s", *state.minLength, baseTypeName)
		}
		if state.maxLength != nil && *state.maxLength < 1 {
			return fmt.Errorf("maxLength (%d) must be >= 1 for list type %s", *state.maxLength, baseTypeName)
		}
	}

	return nil
}

func validateDigitsConstraints(state *facetConstraintState, baseType types.Type, baseTypeName string, isBuiltin bool) error {
	// per XSD spec: fractionDigits must be <= totalDigits
	if state.totalDigits != nil && state.fractionDigits != nil {
		if *state.fractionDigits > *state.totalDigits {
			return fmt.Errorf("fractionDigits (%d) must be <= totalDigits (%d)", *state.fractionDigits, *state.totalDigits)
		}
	}

	// per XSD spec: fractionDigits must be 0 for integer-derived types
	// integer types are derived from decimal with fractionDigits=0 fixed
	if state.fractionDigits != nil && *state.fractionDigits != 0 {
		if isBuiltin && isIntegerTypeName(baseTypeName) {
			return fmt.Errorf("fractionDigits must be 0 for integer type %s, got %d", baseTypeName, *state.fractionDigits)
		}
		// also check user-defined types derived from integer types
		if baseType != nil {
			effectiveTypeName := getEffectiveIntegerTypeName(baseType)
			if effectiveTypeName != "" {
				return fmt.Errorf("fractionDigits must be 0 for type derived from %s, got %d", effectiveTypeName, *state.fractionDigits)
			}
		}
	}

	return nil
}

func isListTypeForFacets(baseType types.Type, baseQName types.QName) bool {
	if st, ok := baseType.(*types.SimpleType); ok {
		if st.Variety() == types.ListVariety {
			return true
		}
	}
	if baseQName.Namespace == types.XSDNamespace && types.IsBuiltinListTypeName(baseQName.Local) {
		return true
	}
	return false
}

// isIntegerTypeName checks if a type name represents an integer-derived type
// Integer types are derived from decimal with fractionDigits=0 fixed
func isIntegerTypeName(typeName string) bool {
	integerTypes := []string{
		"integer", "long", "int", "short", "byte",
		"nonNegativeInteger", "positiveInteger", "unsignedLong", "unsignedInt",
		"unsignedShort", "unsignedByte", "nonPositiveInteger", "negativeInteger",
	}
	return slices.Contains(integerTypes, typeName)
}

// getEffectiveIntegerTypeName returns the name of the integer type if the given type
// is derived from an integer type (including user-defined types). Returns empty string
// if not derived from integer.
func getEffectiveIntegerTypeName(t types.Type) string {
	// walk up the type hierarchy to find if it's derived from an integer type
	visited := make(map[types.Type]bool)
	current := t
	for current != nil && !visited[current] {
		visited[current] = true

		var typeName string
		switch ct := current.(type) {
		case *types.BuiltinType:
			typeName = ct.Name().Local
		case *types.SimpleType:
			if ct.IsBuiltin() || ct.QName.Namespace == types.XSDNamespace {
				typeName = ct.QName.Local
			}
		}

		if typeName != "" && isIntegerTypeName(typeName) {
			return typeName
		}

		current = current.BaseType()
	}
	return ""
}

func shouldDeferEnumerationValidation(baseType types.Type) bool {
	st, ok := baseType.(*types.SimpleType)
	if !ok {
		return false
	}
	if st.ResolvedBase != nil {
		return false
	}
	if st.Restriction == nil || st.Restriction.Base.IsZero() {
		return false
	}
	return st.Restriction.Base.Namespace != types.XSDNamespace
}

// validateEnumerationValues validates that enumeration values are valid for the base type
func validateEnumerationValues(schema *parser.Schema, facetList []types.Facet, baseType types.Type) error {
	if baseType == nil {
		return nil
	}
	if shouldDeferEnumerationValidation(baseType) {
		return nil
	}
	for _, f := range facetList {
		if f.Name() != "enumeration" {
			continue
		}
		enum, ok := f.(*types.Enumeration)
		if !ok {
			continue
		}
		values := enum.Values()
		for i, val := range values {
			ctx := enumContext(enum, i)
			if err := validateValueAgainstTypeWithFacets(schema, val, baseType, ctx, make(map[types.Type]bool)); err != nil {
				return fmt.Errorf("enumeration value %d (%q) is not valid for base type %s: %w", i+1, val, baseType.Name().Local, err)
			}
		}
	}
	return nil
}

func enumContext(enum *types.Enumeration, index int) map[string]string {
	if enum == nil {
		return nil
	}
	contexts := enum.ValueContexts()
	if index < len(contexts) {
		return contexts[index]
	}
	return nil
}
