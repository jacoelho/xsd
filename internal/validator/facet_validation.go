package validator

import (
	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

type facetValidationInput struct {
	data     *facetValidationData
	typ      types.Type
	compiled *grammar.CompiledType
	context  *facetValidationContext
	policy   errorPolicy
}

type facetValidationData struct {
	value  string
	facets []types.Facet
}

//nolint:govet // fieldalignment: prefer clarity over extra indirection.
type facetValidationContext struct {
	path      string
	callbacks *facetValidationCallbacks
}

type facetValidationCallbacks struct {
	validateQNameEnum func(string, *types.Enumeration) error
	makeViolation     func(error) errors.Validation
}

func validateFacets(input *facetValidationInput) (bool, []errors.Validation) {
	if input == nil || input.typ == nil || input.data == nil || len(input.data.facets) == 0 {
		return true, nil
	}

	var (
		path              string
		validateQNameEnum func(string, *types.Enumeration) error
		makeViolation     func(error) errors.Validation
	)
	if input.context != nil {
		path = input.context.path
		if input.context.callbacks != nil {
			validateQNameEnum = input.context.callbacks.validateQNameEnum
			makeViolation = input.context.callbacks.makeViolation
		}
	}
	if makeViolation == nil {
		makeViolation = func(err error) errors.Validation {
			return errors.NewValidation(errors.ErrFacetViolation, err.Error(), path)
		}
	}

	var violations []errors.Validation
	var typedValue types.TypedValue
	for _, facet := range input.data.facets {
		if shouldSkipLengthFacet(input.compiled, facet) {
			continue
		}
		if enumFacet, ok := facet.(*types.Enumeration); ok && validateQNameEnum != nil {
			if err := validateQNameEnum(input.data.value, enumFacet); err != nil {
				if input.policy == errorPolicySuppress {
					return false, nil
				}
				violations = append(violations, makeViolation(err))
			}
			continue
		}
		if lexicalFacet, ok := facet.(types.LexicalValidator); ok {
			if err := lexicalFacet.ValidateLexical(input.data.value, input.typ); err != nil {
				if input.policy == errorPolicySuppress {
					return false, nil
				}
				violations = append(violations, makeViolation(err))
			}
			continue
		}
		if typedValue == nil {
			typedValue = typedValueForFacets(input.data.value, input.typ, input.data.facets)
		}
		if err := facet.Validate(typedValue, input.typ); err != nil {
			if input.policy == errorPolicySuppress {
				return false, nil
			}
			violations = append(violations, makeViolation(err))
		}
	}

	if len(violations) > 0 {
		return false, violations
	}
	return true, nil
}
