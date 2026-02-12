package validator

import "github.com/jacoelho/xsd/internal/runtime"

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
