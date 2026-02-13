package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func validateIdentityConstraintUniquenessWithConstraints(_ *parser.Schema, allConstraints []*types.IdentityConstraint) []error {
	var errs []error

	type constraintKey struct {
		name      string
		namespace types.NamespaceURI
	}
	constraintsByKey := make(map[constraintKey][]*types.IdentityConstraint)

	for _, constraint := range allConstraints {
		key := constraintKey{
			name:      constraint.Name,
			namespace: constraint.TargetNamespace,
		}
		constraintsByKey[key] = append(constraintsByKey[key], constraint)
	}

	for key, constraints := range constraintsByKey {
		if len(constraints) > 1 {
			errs = append(errs, fmt.Errorf("identity constraint name '%s' is not unique within target namespace '%s' (%d definitions)",
				key.name, key.namespace, len(constraints)))
		}
	}

	return errs
}
