package semantics

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func validateDefaultOrFixedValueResolved(
	schema *parser.Schema,
	value string,
	typ model.Type,
	context map[string]string,
	policy idValuePolicy,
) error {
	return ValidateDefaultOrFixedResolved(schema, value, typ, context, toIDPolicy(policy))
}

func toIDPolicy(policy idValuePolicy) IDPolicy {
	if policy == idValuesDisallowed {
		return IDPolicyDisallow
	}
	return IDPolicyAllow
}
