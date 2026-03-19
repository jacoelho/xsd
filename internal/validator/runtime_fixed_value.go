package validator

import (
	"bytes"

	"github.com/jacoelho/xsd/internal/runtime"
)

type selectedValue struct {
	Value   runtime.ValueRef
	Key     runtime.ValueKeyRef
	Member  runtime.ValidatorID
	Fixed   bool
	Present bool
}

func selectTextDefaultOrFixed(
	hasContent bool,
	elem runtime.Element,
	elemOK bool,
	ct runtime.ComplexType,
	hasComplexText bool,
) selectedValue {
	if hasContent {
		return selectedValue{}
	}
	switch {
	case elemOK && elem.Fixed.Present:
		return selectedValue{
			Value:   elem.Fixed,
			Key:     elem.FixedKey,
			Member:  elem.FixedMember,
			Fixed:   true,
			Present: true,
		}
	case elemOK && elem.Default.Present:
		return selectedValue{
			Value:   elem.Default,
			Key:     elem.DefaultKey,
			Member:  elem.DefaultMember,
			Present: true,
		}
	case hasComplexText && ct.TextFixed.Present:
		return selectedValue{
			Value:   ct.TextFixed,
			Member:  ct.TextFixedMember,
			Fixed:   true,
			Present: true,
		}
	case hasComplexText && ct.TextDefault.Present:
		return selectedValue{
			Value:   ct.TextDefault,
			Member:  ct.TextDefaultMember,
			Present: true,
		}
	default:
		return selectedValue{}
	}
}

func selectTextFixedConstraint(
	elem runtime.Element,
	elemOK bool,
	ct runtime.ComplexType,
	hasComplexText bool,
) selectedValue {
	switch {
	case elemOK && elem.Fixed.Present:
		return selectedValue{
			Value:   elem.Fixed,
			Key:     elem.FixedKey,
			Member:  elem.FixedMember,
			Fixed:   true,
			Present: true,
		}
	case hasComplexText && ct.TextFixed.Present:
		return selectedValue{
			Value:   ct.TextFixed,
			Member:  ct.TextFixedMember,
			Fixed:   true,
			Present: true,
		}
	default:
		return selectedValue{}
	}
}

func materializeValueKey(
	validator runtime.ValidatorID,
	canonical []byte,
	member runtime.ValidatorID,
	stored runtime.ValueKeyRef,
	readValue func(runtime.ValueRef) []byte,
	deriveKey func(runtime.ValidatorID, []byte, runtime.ValidatorID) (runtime.ValueKind, []byte, error),
) (runtime.ValueKind, []byte, error) {
	if stored.Ref.Present {
		return stored.Kind, readValue(stored.Ref), nil
	}
	return deriveKey(validator, canonical, member)
}

func matchFixedValue(
	validator runtime.ValidatorID,
	member runtime.ValidatorID,
	canonical []byte,
	observedKind runtime.ValueKind,
	observedKey []byte,
	hasObservedKey bool,
	fixed runtime.ValueRef,
	fixedKey runtime.ValueKeyRef,
	readValue func(runtime.ValueRef) []byte,
	deriveKey func(runtime.ValidatorID, []byte, runtime.ValidatorID) (runtime.ValueKind, []byte, error),
) (bool, error) {
	if fixedKey.Ref.Present {
		actualKind := observedKind
		actualKey := observedKey
		if !hasObservedKey {
			var err error
			actualKind, actualKey, err = deriveKey(validator, canonical, member)
			if err != nil {
				return false, err
			}
		}
		return actualKind == fixedKey.Kind && bytes.Equal(actualKey, readValue(fixedKey.Ref)), nil
	}
	return bytes.Equal(canonical, readValue(fixed)), nil
}
