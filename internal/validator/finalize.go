package validator

import (
	"fmt"
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

// ByteArena provides stable byte storage for one validation session.
type ByteArena interface {
	Alloc(n int) []byte
}

// Violation describes one finalized identity-constraint failure.
type Violation struct {
	Code    xsderrors.ErrorCode
	Message string
}

func (v Violation) ValidationCode() xsderrors.ErrorCode {
	return v.Code
}

func (v Violation) ValidationMessage() string {
	return v.Message
}

// ElementValueKey returns the comparable value key for one element capture.
func ElementValueKey(nilled bool, keyKind runtime.ValueKind, keyBytes []byte) (runtime.ValueKind, []byte, bool) {
	if nilled {
		return runtime.VKInvalid, nil, false
	}
	if keyKind == runtime.VKInvalid {
		return runtime.VKInvalid, nil, true
	}
	return keyKind, keyBytes, true
}

// FinalizeMatches drains all selector matches rooted at the current frame.
func FinalizeMatches(arena ByteArena, matches []*Match) {
	for _, match := range matches {
		if match == nil || match.Constraint == nil {
			continue
		}
		if match.Invalid {
			delete(match.Constraint.Matches, match.ID)
			continue
		}
		finalizeMatch(arena, match)
		delete(match.Constraint.Matches, match.ID)
	}
}

// ResolveScope evaluates duplicate and keyref violations for one closed scope.
func ResolveScope(scope *Scope) []Violation {
	if scope == nil {
		return nil
	}
	constraints := make([]resolutionConstraint, len(scope.Constraints))
	names := make(map[runtime.ICID]string, len(scope.Constraints))
	for i := range scope.Constraints {
		constraint := &scope.Constraints[i]
		constraints[i] = resolutionConstraint{
			ID:         constraint.ID,
			Category:   constraint.Category,
			Referenced: constraint.Referenced,
			Rows:       slices.Clone(constraint.Rows),
			Keyrefs:    slices.Clone(constraint.KeyrefRows),
		}
		if constraint.Name != "" {
			names[constraint.ID] = constraint.Name
		}
	}

	issues := resolveConstraintIssues(constraints)
	if len(issues) == 0 {
		return nil
	}

	out := make([]Violation, 0, len(issues))
	for _, issue := range issues {
		label := "identity constraint"
		if name := names[issue.Constraint]; name != "" {
			label = fmt.Sprintf("identity constraint %s", name)
		}
		switch issue.Kind {
		case issueDuplicate:
			out = append(out, Violation{
				Code:    xsderrors.ErrIdentityDuplicate,
				Message: fmt.Sprintf("%s duplicate", label),
			})
		case issueKeyrefMissing:
			out = append(out, Violation{
				Code:    xsderrors.ErrIdentityKeyRefFailed,
				Message: fmt.Sprintf("%s keyref missing", label),
			})
		case issueKeyrefUndefined:
			out = append(out, Violation{
				Code:    xsderrors.ErrIdentityAbsent,
				Message: fmt.Sprintf("%s keyref undefined", label),
			})
		default:
			out = append(out, Violation{
				Code:    xsderrors.ErrIdentityAbsent,
				Message: fmt.Sprintf("%s violation", label),
			})
		}
	}
	return out
}

func finalizeMatch(arena ByteArena, match *Match) {
	state := match.Constraint
	values := make([]runtime.ValueKey, 0, len(match.Fields))
	for i := range match.Fields {
		field := match.Fields[i]
		switch {
		case field.Multiple:
			state.Violations = append(state.Violations, violation(state.Category, "identity constraint field selects multiple nodes"))
			return
		case field.Count == 0 || field.Missing:
			if state.Category == runtime.ICUnique || state.Category == runtime.ICKeyRef {
				return
			}
			state.Violations = append(state.Violations, violation(state.Category, "identity constraint field is missing"))
			return
		case field.Invalid || !field.HasValue:
			state.Violations = append(state.Violations, violation(state.Category, "identity constraint field selects non-simple content"))
			return
		default:
			values = append(values, freezeKey(arena, field.KeyKind, field.KeyBytes))
		}
	}

	row := Row{Values: values, Hash: hashRow(values)}
	if state.Category == runtime.ICKeyRef {
		state.KeyrefRows = append(state.KeyrefRows, row)
		return
	}
	state.Rows = append(state.Rows, row)
}

func freezeKey(arena ByteArena, kind runtime.ValueKind, key []byte) runtime.ValueKey {
	if len(key) == 0 {
		return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, nil)}
	}
	if arena == nil {
		copied := slices.Clone(key)
		return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, copied), Bytes: copied}
	}
	buf := arena.Alloc(len(key))
	copy(buf, key)
	return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, buf), Bytes: buf}
}

func violation(category runtime.ICCategory, msg string) Violation {
	switch category {
	case runtime.ICKey:
		return Violation{Code: xsderrors.ErrIdentityAbsent, Message: msg}
	case runtime.ICUnique:
		return Violation{Code: xsderrors.ErrIdentityDuplicate, Message: msg}
	case runtime.ICKeyRef:
		return Violation{Code: xsderrors.ErrIdentityKeyRefFailed, Message: msg}
	default:
		return Violation{Code: xsderrors.ErrIdentityAbsent, Message: msg}
	}
}
