package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// validateIdentityConstraintUniqueness validates that identity constraint names are unique within the target namespace.
// Per XSD spec 3.11.2: "Constraint definition identities must be unique within an XML Schema"
// Constraints are identified by (name, target namespace).
func validateIdentityConstraintUniqueness(sch *parser.Schema) []error {
	return validateIdentityConstraintUniquenessWithConstraints(sch, collectAllIdentityConstraints(sch))
}

func validateIdentityConstraintUniquenessWithConstraints(_ *parser.Schema, allConstraints []*model.IdentityConstraint) []error {
	var errs []error

	type constraintKey struct {
		name      string
		namespace model.NamespaceURI
	}
	constraintsByKey := make(map[constraintKey][]*model.IdentityConstraint)

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
