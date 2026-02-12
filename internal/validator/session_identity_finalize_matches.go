package validator

import (
	ic "github.com/jacoelho/xsd/internal/identity"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *identityState) finalizeMatches(frame *rtIdentityFrame) {
	for _, match := range frame.matches {
		if match.invalid {
			delete(match.constraint.matches, match.id)
			continue
		}
		s.finalizeSelectorMatch(match)
		delete(match.constraint.matches, match.id)
	}
}

func (s *identityState) finalizeSelectorMatch(match *rtSelectorMatch) {
	state := match.constraint
	values := make([]runtime.ValueKey, 0, len(match.fields))
	for i := range match.fields {
		field := match.fields[i]
		switch {
		case field.multiple:
			state.violations = append(state.violations, identityViolation(state.category, "identity constraint field selects multiple nodes"))
			return
		case field.count == 0 || field.missing:
			if state.category == runtime.ICUnique || state.category == runtime.ICKeyRef {
				return
			}
			state.violations = append(state.violations, identityViolation(state.category, "identity constraint field is missing"))
			return
		case field.invalid || !field.hasValue:
			state.violations = append(state.violations, identityViolation(state.category, "identity constraint field selects non-simple content"))
			return
		default:
			values = append(values, freezeIdentityKey(s.arena, field.keyKind, field.keyBytes))
		}
	}
	row := rtIdentityRow{values: values, hash: ic.HashRow(values)}
	if state.category == runtime.ICKeyRef {
		state.keyrefRows = append(state.keyrefRows, row)
		return
	}
	state.rows = append(state.rows, row)
}
