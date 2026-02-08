package validator

import (
	"bytes"
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

type endTextState struct {
	canonText     []byte
	textKeyBytes  []byte
	textValidator runtime.ValidatorID
	textMember    runtime.ValidatorID
	textKeyKind   runtime.ValueKind
}

func (s *Session) validateEndElementText(frame elemFrame, typ runtime.Type, typeOK bool, elem runtime.Element, elemOK bool, resolver sessionResolver, path *string) ([]error, endTextState) {
	result := endTextState{}
	if frame.nilled || !typeOK || (typ.Kind != runtime.TypeSimple && typ.Kind != runtime.TypeBuiltin && frame.content != runtime.ContentSimple) {
		return nil, result
	}

	var errs []error
	if frame.hasChildElements && !frame.childErrorReported {
		s.ensurePath(path)
		errs = append(errs, newValidationError(xsderrors.ErrTextInElementOnly, "element not allowed in simple content"))
	}

	rawText := s.TextSlice(frame.text)
	hasContent := frame.text.HasText || frame.hasChildElements
	var ct runtime.ComplexType
	hasComplexText := false
	if typ.Kind == runtime.TypeComplex {
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(s.rt.ComplexTypes) {
			s.ensurePath(path)
			errs = append(errs, fmt.Errorf("complex type %d missing", frame.typ))
		} else {
			ct = s.rt.ComplexTypes[typ.Complex.ID]
			hasComplexText = true
		}
	}

	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		result.textValidator = typ.Validator
	case runtime.TypeComplex:
		if hasComplexText {
			result.textValidator = ct.TextValidator
		}
	}

	trackDefault := func(value []byte, member runtime.ValidatorID) {
		if result.textValidator == 0 {
			return
		}
		if err := s.trackDefaultValue(result.textValidator, value, resolver, member); err != nil {
			s.ensurePath(path)
			errs = append(errs, err)
		}
	}

	switch {
	case !hasContent && elemOK && elem.Fixed.Present:
		result.canonText = valueBytes(s.rt.Values, elem.Fixed)
		result.textMember = elem.FixedMember
		if elem.FixedKey.Ref.Present {
			result.textKeyKind = elem.FixedKey.Kind
			result.textKeyBytes = valueKeyBytes(s.rt.Values, elem.FixedKey)
		}
		trackDefault(result.canonText, result.textMember)
	case !hasContent && elemOK && elem.Default.Present:
		result.canonText = valueBytes(s.rt.Values, elem.Default)
		result.textMember = elem.DefaultMember
		if elem.DefaultKey.Ref.Present {
			result.textKeyKind = elem.DefaultKey.Kind
			result.textKeyBytes = valueKeyBytes(s.rt.Values, elem.DefaultKey)
		}
		trackDefault(result.canonText, result.textMember)
	case !hasContent && hasComplexText && ct.TextFixed.Present:
		result.canonText = valueBytes(s.rt.Values, ct.TextFixed)
		result.textMember = ct.TextFixedMember
		trackDefault(result.canonText, result.textMember)
	case !hasContent && hasComplexText && ct.TextDefault.Present:
		result.canonText = valueBytes(s.rt.Values, ct.TextDefault)
		result.textMember = ct.TextDefaultMember
		trackDefault(result.canonText, result.textMember)
	default:
		requireCanonical := (elemOK && elem.Fixed.Present) || (hasComplexText && ct.TextFixed.Present)
		canon, metrics, err := s.ValidateTextValue(frame.typ, rawText, resolver, TextValueOptions{
			RequireCanonical: requireCanonical,
			NeedKey:          requireCanonical,
		})
		if err != nil {
			s.ensurePath(path)
			errs = append(errs, err)
		} else {
			result.canonText = canon
			result.textKeyKind = metrics.keyKind
			result.textKeyBytes = metrics.keyBytes
		}
	}

	if result.canonText != nil && elemOK && (frame.text.HasText || hasContent) && elem.Fixed.Present {
		matched := false
		keyCompareErr := false
		if elem.FixedKey.Ref.Present {
			actualKind := result.textKeyKind
			actualKey := result.textKeyBytes
			if actualKind == runtime.VKInvalid {
				kind, key, err := s.keyForCanonicalValue(result.textValidator, result.canonText, resolver, result.textMember)
				if err != nil {
					s.ensurePath(path)
					errs = append(errs, err)
					keyCompareErr = true
				} else {
					actualKind = kind
					actualKey = key
				}
			}
			if actualKind == elem.FixedKey.Kind && bytes.Equal(actualKey, valueKeyBytes(s.rt.Values, elem.FixedKey)) {
				matched = true
			}
		} else {
			fixed := valueBytes(s.rt.Values, elem.Fixed)
			matched = bytes.Equal(result.canonText, fixed)
		}
		if !matched && !keyCompareErr {
			s.ensurePath(path)
			errs = append(errs, newValidationError(xsderrors.ErrElementFixedValue, "fixed element value mismatch"))
		}
	} else if result.canonText != nil && (frame.text.HasText || hasContent) && hasComplexText && ct.TextFixed.Present && (!elemOK || !elem.Fixed.Present) {
		fixed := valueBytes(s.rt.Values, ct.TextFixed)
		if !bytes.Equal(result.canonText, fixed) {
			s.ensurePath(path)
			errs = append(errs, newValidationError(xsderrors.ErrElementFixedValue, "fixed element value mismatch"))
		}
	}

	return errs, result
}
