package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateIdentityConstraintUniqueness validates that identity constraint names are unique within the target namespace.
// Per XSD spec 3.11.2: "Constraint definition identities must be unique within an XML Schema"
// Constraints are identified by (name, target namespace).
func validateIdentityConstraintUniqueness(sch *parser.Schema) []error {
	var errs []error

	type constraintKey struct {
		name      string
		namespace types.NamespaceURI
	}
	constraintsByKey := make(map[constraintKey][]*types.IdentityConstraint)

	allConstraints := collectAllIdentityConstraints(sch)
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
