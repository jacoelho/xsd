package validator

import (
	"fmt"

	xsdErrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/ic"
	"github.com/jacoelho/xsd/internal/runtime"
)

func resolveScopeErrors(scope *rtIdentityScope) []error {
	if scope == nil {
		return nil
	}
	constraints := make([]ic.Constraint, len(scope.constraints))
	names := make(map[runtime.ICID]string, len(scope.constraints))
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
		if constraint.name != "" {
			names[constraint.id] = constraint.name
		}
	}

	issues := ic.Resolve(constraints)
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
		case ic.IssueDuplicate:
			errs = append(errs, newValidationError(xsdErrors.ErrIdentityDuplicate, fmt.Sprintf("%s duplicate", label)))
		case ic.IssueKeyrefMissing:
			errs = append(errs, newValidationError(xsdErrors.ErrIdentityKeyRefFailed, fmt.Sprintf("%s keyref missing", label)))
		case ic.IssueKeyrefUndefined:
			errs = append(errs, newValidationError(xsdErrors.ErrIdentityAbsent, fmt.Sprintf("%s keyref undefined", label)))
		default:
			errs = append(errs, newValidationError(xsdErrors.ErrIdentityAbsent, fmt.Sprintf("%s violation", label)))
		}
	}
	return errs
}
