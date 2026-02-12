package semanticresolve

import (
	parser "github.com/jacoelho/xsd/internal/parser"
	model "github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/valuevalidate"
)

func validateDefaultOrFixedValueResolved(
	schema *parser.Schema,
	value string,
	typ model.Type,
	context map[string]string,
	policy idValuePolicy,
) error {
	return valuevalidate.ValidateDefaultOrFixedResolved(schema, value, typ, context, toIDPolicy(policy))
}

func toIDPolicy(policy idValuePolicy) valuevalidate.IDPolicy {
	if policy == idValuesDisallowed {
		return valuevalidate.IDPolicyDisallow
	}
	return valuevalidate.IDPolicyAllow
}
