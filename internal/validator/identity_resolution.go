package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/ic"
)

func resolveScopeErrors(scope *rtIdentityScope) []error {
	if scope == nil {
		return nil
	}
	constraints := make([]ic.Constraint, len(scope.constraints))
	for i := range scope.constraints {
		constraint := &scope.constraints[i]
		rows := make([]ic.Row, len(constraint.rows))
		for j, row := range constraint.rows {
			rows[j] = ic.Row{Values: row.values, Hash: row.hash}
		}
		keyrefs := make([]ic.Row, len(constraint.keyrefRows))
		for j, row := range constraint.keyrefRows {
			keyrefs[j] = ic.Row{Values: row.values, Hash: row.hash}
		}
		constraints[i] = ic.Constraint{
			ID:         constraint.id,
			Category:   constraint.category,
			Referenced: constraint.referenced,
			Rows:       rows,
			Keyrefs:    keyrefs,
		}
	}

	issues := ic.Resolve(constraints)
	if len(issues) == 0 {
		return nil
	}
	errs := make([]error, 0, len(issues))
	for _, issue := range issues {
		switch issue.Kind {
		case ic.IssueDuplicate:
			errs = append(errs, newValidationError(xsderrors.ErrIdentityDuplicate, "identity constraint duplicate"))
		case ic.IssueKeyrefMissing:
			errs = append(errs, newValidationError(xsderrors.ErrIdentityKeyRefFailed, "identity constraint keyref missing"))
		case ic.IssueKeyrefUndefined:
			errs = append(errs, newValidationError(xsderrors.ErrIdentityAbsent, "identity constraint keyref undefined"))
		default:
			errs = append(errs, newValidationError(xsderrors.ErrIdentityAbsent, "identity constraint violation"))
		}
	}
	return errs
}
