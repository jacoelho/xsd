package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/identity"
	"github.com/jacoelho/xsd/internal/runtime"
)

func resolveScopeErrors(scope *rtIdentityScope) []error {
	if scope == nil {
		return nil
	}
	constraints := make([]identity.Constraint, len(scope.constraints))
	names := make(map[runtime.ICID]string, len(scope.constraints))
	for i := range scope.constraints {
		constraint := &scope.constraints[i]
		rows := make([]identity.Row, len(constraint.rows))
		for j, row := range constraint.rows {
			rows[j] = identity.Row{Values: row.values, Hash: row.hash}
		}
		keyrefs := make([]identity.Row, len(constraint.keyrefRows))
		for j, row := range constraint.keyrefRows {
			keyrefs[j] = identity.Row{Values: row.values, Hash: row.hash}
		}
		constraints[i] = identity.Constraint{
			ID:         constraint.id,
			Category:   constraint.category,
			Referenced: constraint.referenced,
			Rows:       rows,
			Keyrefs:    keyrefs,
		}
		if constraint.name != "" {
			names[constraint.id] = constraint.name
		}
	}

	issues := identity.Resolve(constraints)
	if len(issues) == 0 {
		return nil
	}
	errs := make([]error, 0, len(issues))
	for _, issue := range issues {
		name := names[issue.Constraint]
		label := "identity constraint"
		if name != "" {
			label = fmt.Sprintf("identity constraint %s", name)
		}
		switch issue.Kind {
		case identity.IssueDuplicate:
			errs = append(errs, newValidationError(xsderrors.ErrIdentityDuplicate, fmt.Sprintf("%s duplicate", label)))
		case identity.IssueKeyrefMissing:
			errs = append(errs, newValidationError(xsderrors.ErrIdentityKeyRefFailed, fmt.Sprintf("%s keyref missing", label)))
		case identity.IssueKeyrefUndefined:
			errs = append(errs, newValidationError(xsderrors.ErrIdentityAbsent, fmt.Sprintf("%s keyref undefined", label)))
		default:
			errs = append(errs, newValidationError(xsderrors.ErrIdentityAbsent, fmt.Sprintf("%s violation", label)))
		}
	}
	return errs
}
