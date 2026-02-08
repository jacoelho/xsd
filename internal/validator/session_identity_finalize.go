package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	ic "github.com/jacoelho/xsd/internal/identity"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *identityState) applyFieldCaptures(frame *rtIdentityFrame, elem *runtime.Element, in identityEndInput) {
	kind, key, ok := elementValueKey(frame, elem, in)
	for _, capture := range frame.captures {
		match := capture.match
		if match.invalid {
			continue
		}
		fieldState := &match.fields[capture.fieldIndex]
		if fieldState.multiple || fieldState.invalid {
			continue
		}
		if !ok {
			fieldState.missing = true
			continue
		}
		if kind == runtime.VKInvalid {
			fieldState.invalid = true
			continue
		}
		fieldState.keyKind = kind
		fieldState.keyBytes = append(fieldState.keyBytes[:0], key...)
		fieldState.hasValue = true
	}
}

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

func freezeIdentityKey(arena *Arena, kind runtime.ValueKind, key []byte) runtime.ValueKey {
	if len(key) == 0 {
		return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, nil)}
	}
	if arena == nil {
		copied := append([]byte(nil), key...)
		return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, copied), Bytes: copied}
	}
	buf := arena.Alloc(len(key))
	copy(buf, key)
	return runtime.ValueKey{Kind: kind, Hash: runtime.HashKey(kind, buf), Bytes: buf}
}

func identityViolation(category runtime.ICCategory, msg string) error {
	switch category {
	case runtime.ICKey:
		return newValidationError(xsderrors.ErrIdentityAbsent, msg)
	case runtime.ICUnique:
		return newValidationError(xsderrors.ErrIdentityDuplicate, msg)
	case runtime.ICKeyRef:
		return newValidationError(xsderrors.ErrIdentityKeyRefFailed, msg)
	default:
		return newValidationError(xsderrors.ErrIdentityAbsent, msg)
	}
}
