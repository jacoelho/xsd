package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

// Applied records one defaulted or fixed attribute emitted by validation.
type Applied struct {
	KeyBytes []byte
	Value    runtime.ValueRef
	Name     runtime.SymbolID
	Fixed    bool
	KeyKind  runtime.ValueKind
}

// Selection describes one selected default or fixed attribute policy.
type Selection struct {
	Value   runtime.ValueRef
	Key     runtime.ValueKeyRef
	Member  runtime.ValidatorID
	Fixed   bool
	Present bool
}

// SelectDefaultOrFixed chooses the applicable default/fixed policy for one
// attribute use.
func SelectDefaultOrFixed(use *runtime.AttrUse) Selection {
	if use == nil {
		return Selection{}
	}
	if use.Fixed.Present {
		return Selection{
			Value:   use.Fixed,
			Key:     use.FixedKey,
			Member:  use.FixedMember,
			Fixed:   true,
			Present: true,
		}
	}
	if use.Default.Present {
		return Selection{
			Value:   use.Default,
			Key:     use.DefaultKey,
			Member:  use.DefaultMember,
			Present: true,
		}
	}
	return Selection{}
}

// ApplyDefaults applies attribute default/fixed policy for one complex-type use list.
func ApplyDefaults(
	uses []runtime.AttrUse,
	present []bool,
	storeAttrs, seenID bool,
	applied []Applied,
	selectAttr func(*runtime.AttrUse) Selection,
	isIDValidator func(runtime.ValidatorID) bool,
	readValue func(runtime.ValueRef) []byte,
	trackDefault func(runtime.ValidatorID, []byte, runtime.ValidatorID) error,
	materializeKey func(runtime.ValidatorID, []byte, runtime.ValidatorID, runtime.ValueKeyRef) (runtime.ValueKind, []byte, error),
	storeKey func([]byte) []byte,
) ([]Applied, error) {
	applied = prepareApplied(applied, len(uses))

	for i := range uses {
		use := &uses[i]
		if use.Use == runtime.AttrProhibited {
			continue
		}
		if i < len(present) && present[i] {
			continue
		}
		if use.Use == runtime.AttrRequired {
			return nil, xsderrors.New(xsderrors.ErrRequiredAttributeMissing, "required attribute missing")
		}

		selection := selectAttr(use)
		if !selection.Present {
			continue
		}
		if isIDValidator != nil && isIDValidator(use.Validator) {
			if seenID {
				return nil, xsderrors.New(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
			}
			seenID = true
		}

		canonical := readValue(selection.Value)
		if err := trackDefault(use.Validator, canonical, selection.Member); err != nil {
			return nil, err
		}

		appliedAttr := Applied{
			Name:  use.Name,
			Value: selection.Value,
			Fixed: selection.Fixed,
		}
		if storeAttrs {
			kind, key, err := materializeKey(use.Validator, canonical, selection.Member, selection.Key)
			if err != nil {
				return nil, err
			}
			if storeKey != nil {
				key = storeKey(key)
			}
			appliedAttr.KeyKind = kind
			appliedAttr.KeyBytes = key
		}
		applied = append(applied, appliedAttr)
	}

	return applied, nil
}

func prepareApplied(applied []Applied, count int) []Applied {
	if cap(applied) < count {
		return make([]Applied, 0, count)
	}
	return applied[:0]
}
