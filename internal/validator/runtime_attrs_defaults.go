package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) applyDefaultAttrs(uses []runtime.AttrUse, present []bool, storeAttrs, seenID bool) ([]AttrApplied, error) {
	applied := s.attrAppliedBuf[:0]
	if cap(applied) < len(uses) {
		applied = make([]AttrApplied, 0, len(uses))
	}

	for i := range uses {
		use := &uses[i]
		if use.Use == runtime.AttrProhibited {
			continue
		}
		if i < len(present) && present[i] {
			continue
		}
		if use.Use == runtime.AttrRequired {
			return nil, newValidationError(xsderrors.ErrRequiredAttributeMissing, "required attribute missing")
		}
		selection := selectAttrDefaultFixed(use)
		if !selection.present {
			continue
		}
		if s.isIDValidator(use.Validator) {
			if seenID {
				return nil, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
			}
			seenID = true
		}
		canonical := valueBytes(s.rt.Values, selection.value)
		if err := s.trackDefaultValue(use.Validator, canonical, nil, selection.member); err != nil {
			return nil, err
		}
		if storeAttrs {
			kind, key, err := s.materializePolicyKey(use.Validator, canonical, selection.member, selection.key)
			if err != nil {
				return nil, err
			}
			applied = append(applied, AttrApplied{
				Name:     use.Name,
				Value:    selection.value,
				Fixed:    selection.fixed,
				KeyKind:  kind,
				KeyBytes: s.storeKey(key),
			})
			continue
		}
		applied = append(applied, AttrApplied{Name: use.Name, Value: selection.value, Fixed: selection.fixed})
	}

	return applied, nil
}
