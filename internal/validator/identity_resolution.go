package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/ic"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *identityState) resolveScope(scope *rtIdentityScope) {
	if s == nil || scope == nil {
		return
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
	for _, issue := range issues {
		switch issue.Kind {
		case ic.IssueDuplicate:
			label := "unique"
			if issue.Category == runtime.ICKey {
				label = "key"
			}
			s.violations = append(s.violations, fmt.Errorf("identity: duplicate %s value", label))
		case ic.IssueKeyrefUndefined:
			s.violations = append(s.violations, fmt.Errorf("identity: keyref references missing key"))
		case ic.IssueKeyrefMissing:
			s.violations = append(s.violations, fmt.Errorf("identity: keyref value not found"))
		}
	}
}
