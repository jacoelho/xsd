package facets

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/builtins"
	"github.com/jacoelho/xsd/internal/facetvalue"
	"github.com/jacoelho/xsd/internal/model"
)

// SchemaConstraintInput carries schema-time facet consistency inputs.
type SchemaConstraintInput struct {
	BaseType  model.Type
	BaseQName model.QName
	FacetList []model.Facet
}

// SchemaConstraintCallbacks provides semantic checks delegated to callers.
type SchemaConstraintCallbacks struct {
	ValidateRangeConsistency func(minExclusive, maxExclusive, minInclusive, maxInclusive *string, baseType model.Type, baseQName model.QName) error
	ValidateRangeValues      func(minExclusive, maxExclusive, minInclusive, maxInclusive *string, baseType model.Type, bt *model.BuiltinType) error
	ValidateEnumerationValue func(value string, baseType model.Type, context map[string]string) error
}

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

// ValidateSchemaConstraints validates schema-time facet consistency for a base type.
func ValidateSchemaConstraints(in SchemaConstraintInput, cb SchemaConstraintCallbacks) error {
	baseTypeName := in.BaseQName.Local
	isBuiltin := in.BaseQName.Namespace == model.XSDNamespace
	var bt *model.BuiltinType
	if isBuiltin {
		bt = builtins.Get(model.TypeName(baseTypeName))
	}

	state := facetConstraintState{}

	for _, facet := range in.FacetList {
		name := facet.Name()
		if !IsValidFacetName(name) {
			return fmt.Errorf("unknown or invalid facet '%s' (not a valid XSD 1.0 facet)", name)
		}
		if err := state.captureFacet(name, facet); err != nil {
			return err
		}
		if err := facetvalue.ValidateApplicability(name, in.BaseType, in.BaseQName); err != nil {
			return err
		}
	}

	if err := validateLengthFacetConstraints(&state, in.BaseType, in.BaseQName, baseTypeName); err != nil {
		return err
	}
	if cb.ValidateRangeConsistency != nil {
		if err := cb.ValidateRangeConsistency(state.minExclusive, state.maxExclusive, state.minInclusive, state.maxInclusive, in.BaseType, in.BaseQName); err != nil {
			return err
		}
	}
	if cb.ValidateRangeValues != nil {
		if err := cb.ValidateRangeValues(state.minExclusive, state.maxExclusive, state.minInclusive, state.maxInclusive, in.BaseType, bt); err != nil {
			return err
		}
	}
	if err := validateDigitsConstraints(&state, in.BaseType, baseTypeName, isBuiltin); err != nil {
		return err
	}

	if state.hasEnumeration && in.BaseType != nil {
		if err := validateEnumerationValues(in.FacetList, in.BaseType, cb.ValidateEnumerationValue); err != nil {
			return err
		}
	}

	return nil
}

// IsValidFacetName reports whether name is an XSD 1.0 facet name.
func IsValidFacetName(name string) bool {
	switch name {
	case "length", "minLength", "maxLength", "pattern", "enumeration", "whiteSpace",
		"maxInclusive", "maxExclusive", "minInclusive", "minExclusive",
		"totalDigits", "fractionDigits":
		return true
	default:
		return false
	}
}

func (s *facetConstraintState) captureFacet(name string, facet model.Facet) error {
	switch name {
	case "minExclusive", "maxExclusive", "minInclusive", "maxInclusive":
		if lf, ok := facet.(model.LexicalFacet); ok {
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
		if ivf, ok := facet.(model.IntValueFacet); ok {
			val := ivf.GetIntValue()
			s.length = &val
		}
	case "minLength":
		if ivf, ok := facet.(model.IntValueFacet); ok {
			val := ivf.GetIntValue()
			s.minLength = &val
		}
	case "maxLength":
		if ivf, ok := facet.(model.IntValueFacet); ok {
			val := ivf.GetIntValue()
			s.maxLength = &val
		}
	case "enumeration":
		s.hasEnumeration = true
	case "totalDigits":
		if ivf, ok := facet.(model.IntValueFacet); ok {
			val := ivf.GetIntValue()
			s.totalDigits = &val
		}
	case "fractionDigits":
		if ivf, ok := facet.(model.IntValueFacet); ok {
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

func validateLengthFacetConstraints(state *facetConstraintState, baseType model.Type, baseQName model.QName, baseTypeName string) error {
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

	if builtins.IsBuiltinListTypeName(baseTypeName) {
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

func validateDigitsConstraints(state *facetConstraintState, baseType model.Type, baseTypeName string, isBuiltin bool) error {
	if state.totalDigits != nil && state.fractionDigits != nil {
		if *state.fractionDigits > *state.totalDigits {
			return fmt.Errorf("fractionDigits (%d) must be <= totalDigits (%d)", *state.fractionDigits, *state.totalDigits)
		}
	}

	if state.fractionDigits != nil && *state.fractionDigits != 0 {
		if isBuiltin && model.IsIntegerTypeName(baseTypeName) {
			return fmt.Errorf("fractionDigits must be 0 for integer type %s, got %d", baseTypeName, *state.fractionDigits)
		}
		if baseType != nil {
			effectiveTypeName := getEffectiveIntegerTypeName(baseType)
			if effectiveTypeName != "" {
				return fmt.Errorf("fractionDigits must be 0 for type derived from %s, got %d", effectiveTypeName, *state.fractionDigits)
			}
		}
	}

	return nil
}

func isListTypeForFacets(baseType model.Type, baseQName model.QName) bool {
	if st, ok := baseType.(*model.SimpleType); ok {
		if st.Variety() == model.ListVariety {
			return true
		}
	}
	if baseQName.Namespace == model.XSDNamespace && builtins.IsBuiltinListTypeName(baseQName.Local) {
		return true
	}
	return false
}

func getEffectiveIntegerTypeName(t model.Type) string {
	visited := make(map[model.Type]bool)
	current := t
	for current != nil && !visited[current] {
		visited[current] = true

		var typeName string
		switch ct := current.(type) {
		case *model.BuiltinType:
			typeName = ct.Name().Local
		case *model.SimpleType:
			if ct.IsBuiltin() || ct.QName.Namespace == model.XSDNamespace {
				typeName = ct.QName.Local
			}
		}

		if typeName != "" && model.IsIntegerTypeName(typeName) {
			return typeName
		}

		current = current.BaseType()
	}
	return ""
}

func shouldDeferEnumerationValidation(baseType model.Type) bool {
	st, ok := baseType.(*model.SimpleType)
	if !ok {
		return false
	}
	if st.ResolvedBase != nil {
		return false
	}
	if st.Restriction == nil || st.Restriction.Base.IsZero() {
		return false
	}
	return st.Restriction.Base.Namespace != model.XSDNamespace
}

func validateEnumerationValues(facetList []model.Facet, baseType model.Type, validateValue func(value string, baseType model.Type, context map[string]string) error) error {
	if baseType == nil || validateValue == nil {
		return nil
	}
	if shouldDeferEnumerationValidation(baseType) {
		return nil
	}
	for _, f := range facetList {
		if f.Name() != "enumeration" {
			continue
		}
		enum, ok := f.(*model.Enumeration)
		if !ok {
			continue
		}
		values := enum.Values()
		for i, val := range values {
			ctx := enumContext(enum, i)
			if err := validateValue(val, baseType, ctx); err != nil {
				return fmt.Errorf("enumeration value %d (%q) is not valid for base type %s: %w", i+1, val, baseType.Name().Local, err)
			}
		}
	}
	return nil
}

func enumContext(enum *model.Enumeration, index int) map[string]string {
	if enum == nil {
		return nil
	}
	contexts := enum.ValueContexts()
	if index < len(contexts) {
		return contexts[index]
	}
	return nil
}
