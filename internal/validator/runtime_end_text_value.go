package validator

import "github.com/jacoelho/xsd/internal/runtime"

func (s *Session) resolveEndTextValue(
	result *endTextState,
	frame elemFrame,
	rawText []byte,
	hasContent bool,
	elem runtime.Element,
	elemOK bool,
	ct runtime.ComplexType,
	hasComplexText bool,
	resolver sessionResolver,
	path *string,
) []error {
	if result == nil {
		return nil
	}
	var errs []error

	trackDefault := func(value []byte, member runtime.ValidatorID) {
		if result.textValidator == 0 {
			return
		}
		if err := s.trackDefaultValue(result.textValidator, value, resolver, member); err != nil {
			s.ensurePath(path)
			errs = append(errs, err)
		}
	}

	fallback := selectTextDefaultFixed(hasContent, elem, elemOK, ct, hasComplexText)
	if fallback.present {
		result.canonText = valueBytes(s.rt.Values, fallback.value)
		result.textMember = fallback.member
		if fallback.key.Ref.Present {
			result.textKeyKind = fallback.key.Kind
			result.textKeyBytes = valueBytes(s.rt.Values, fallback.key.Ref)
		}
		trackDefault(result.canonText, result.textMember)
		return errs
	}

	requireCanonical := (elemOK && elem.Fixed.Present) || (hasComplexText && ct.TextFixed.Present)
	canon, metrics, err := s.ValidateTextValue(frame.typ, rawText, resolver, TextValueOptions{
		RequireCanonical: requireCanonical,
		NeedKey:          requireCanonical,
	})
	if err != nil {
		s.ensurePath(path)
		errs = append(errs, err)
		return errs
	}

	result.canonText = canon
	result.textKeyKind = metrics.keyKind
	result.textKeyBytes = metrics.keyBytes
	return errs
}
