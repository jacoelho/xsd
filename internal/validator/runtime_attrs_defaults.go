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
		if use.Fixed.Present {
			if s.isIDValidator(use.Validator) {
				if seenID {
					return nil, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
				}
				seenID = true
			}
			fixedValue := valueBytes(s.rt.Values, use.Fixed)
			if err := s.trackDefaultValue(use.Validator, fixedValue, nil, use.FixedMember); err != nil {
				return nil, err
			}
			if storeAttrs {
				kind := use.FixedKey.Kind
				key := valueKeyBytes(s.rt.Values, use.FixedKey)
				if !use.FixedKey.Ref.Present {
					var err error
					kind, key, err = s.keyForCanonicalValue(use.Validator, fixedValue, nil, use.FixedMember)
					if err != nil {
						return nil, err
					}
				}
				applied = append(applied, AttrApplied{
					Name:     use.Name,
					Value:    use.Fixed,
					Fixed:    true,
					KeyKind:  kind,
					KeyBytes: s.storeKey(key),
				})
			} else {
				applied = append(applied, AttrApplied{Name: use.Name, Value: use.Fixed, Fixed: true})
			}
			continue
		}
		if use.Default.Present {
			if s.isIDValidator(use.Validator) {
				if seenID {
					return nil, newValidationError(xsderrors.ErrMultipleIDAttr, "multiple ID attributes on element")
				}
				seenID = true
			}
			defaultValue := valueBytes(s.rt.Values, use.Default)
			if err := s.trackDefaultValue(use.Validator, defaultValue, nil, use.DefaultMember); err != nil {
				return nil, err
			}
			if storeAttrs {
				kind := use.DefaultKey.Kind
				key := valueKeyBytes(s.rt.Values, use.DefaultKey)
				if !use.DefaultKey.Ref.Present {
					var err error
					kind, key, err = s.keyForCanonicalValue(use.Validator, defaultValue, nil, use.DefaultMember)
					if err != nil {
						return nil, err
					}
				}
				applied = append(applied, AttrApplied{
					Name:     use.Name,
					Value:    use.Default,
					KeyKind:  kind,
					KeyBytes: s.storeKey(key),
				})
			} else {
				applied = append(applied, AttrApplied{Name: use.Name, Value: use.Default})
			}
		}
	}

	return applied, nil
}
