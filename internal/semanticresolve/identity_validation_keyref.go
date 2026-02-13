package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
)

// validateKeyrefConstraints validates keyref constraints against all known constraints.
func validateKeyrefConstraints(contextQName types.QName, constraints, allConstraints []*types.IdentityConstraint) []error {
	var errs []error

	for _, constraint := range constraints {
		if constraint.Type != types.KeyRefConstraint {
			continue
		}

		refQName := constraint.ReferQName
		if refQName.IsZero() {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' is missing refer attribute",
				contextQName, constraint.Name))
			continue
		}
		if refQName.Namespace != constraint.TargetNamespace {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' refers to '%s' in namespace '%s', which does not match target namespace '%s'",
				contextQName, constraint.Name, refQName.Local, refQName.Namespace, constraint.TargetNamespace))
			continue
		}

		var referencedConstraint *types.IdentityConstraint
		for _, other := range allConstraints {
			if other.Name == refQName.Local && other.TargetNamespace == refQName.Namespace {
				if other.Type == types.KeyConstraint || other.Type == types.UniqueConstraint {
					referencedConstraint = other
					break
				}
			}
		}

		if referencedConstraint == nil {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' references non-existent key/unique constraint '%s'",
				contextQName, constraint.Name, refQName.String()))
			continue
		}

		if len(constraint.Fields) != len(referencedConstraint.Fields) {
			errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' has %d fields but referenced constraint '%s' has %d fields",
				contextQName, constraint.Name, len(constraint.Fields), refQName.String(), len(referencedConstraint.Fields)))
			continue
		}

		for i := 0; i < len(constraint.Fields); i++ {
			keyrefField := constraint.Fields[i]
			refField := referencedConstraint.Fields[i]

			if keyrefField.ResolvedType != nil && refField.ResolvedType != nil {
				if !areFieldTypesCompatible(keyrefField.ResolvedType, refField.ResolvedType) {
					errs = append(errs, fmt.Errorf("element %s: keyref constraint '%s' field %d type '%s' is not compatible with referenced constraint '%s' field %d type '%s'",
						contextQName, constraint.Name, i+1, keyrefField.ResolvedType.Name(),
						refQName.String(), i+1, refField.ResolvedType.Name()))
				}
			}
		}
	}

	return errs
}
