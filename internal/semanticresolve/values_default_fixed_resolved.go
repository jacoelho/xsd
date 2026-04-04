package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

func validateDefaultOrFixedValueResolved(
	schema *parser.Schema,
	value string,
	typ model.Type,
	context map[string]string,
	policy idValuePolicy,
) error {
	return semantics.ValidateDefaultOrFixedResolved(schema, value, typ, context, toIDPolicy(policy))
}

func toIDPolicy(policy idValuePolicy) semantics.IDPolicy {
	if policy == idValuesDisallowed {
		return semantics.IDPolicyDisallow
	}
	return semantics.IDPolicyAllow
}
