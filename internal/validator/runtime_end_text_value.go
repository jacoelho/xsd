package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

func (s *Session) resolveEndTextValue(
	result *endTextState,
	frame elemFrame,
	rawText []byte,
	hasContent bool,
	elem *runtime.Element,
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

	fallback := selectTextDefaultOrFixed(hasContent, elem, elemOK, ct, hasComplexText)
	if fallback.Present {
		result.canonText = s.rt.Value(fallback.Value)
		result.textMember = fallback.Member
		if fallback.Key.Ref.Present {
			result.textKeyKind = fallback.Key.Kind
			result.textKeyBytes = s.rt.Value(fallback.Key.Ref)
		}
		trackDefault(result.canonText, result.textMember)
		return errs
	}

	if result.textValidator == 0 {
		if elemOK && elem.Fixed.Present {
			result.canonText = rawText
			if elem.FixedKey.Ref.Present {
				key := runtime.StringKeyBytes(s.buffers.keyTmp[:0], 0, rawText)
				s.buffers.keyTmp = key
				result.textKeyKind = runtime.VKString
				result.textKeyBytes = key
			}
		}
		return errs
	}

	requireCanonical := (elemOK && elem.Fixed.Present) || (hasComplexText && ct.TextFixed.Present)
	validated, err := newValueRunner(s).validateTextSession(textValueRequest{
		Type:     frame.typ,
		Lexical:  rawText,
		Resolver: resolver,
		Options: TextValueOptions{
			RequireCanonical: requireCanonical,
			NeedKey:          requireCanonical || s.hasIdentityConstraints(),
		},
	})
	if err != nil {
		s.ensurePath(path)
		errs = append(errs, err)
		return errs
	}

	result.canonText = validated.Canonical
	result.textKeyKind = validated.KeyKind
	result.textKeyBytes = validated.KeyBytes
	return errs
}
