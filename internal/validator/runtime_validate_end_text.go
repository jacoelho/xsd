package validator

import (
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

	fallback := selectTextDefaultFixed(hasContent, elem, elemOK, ct, hasComplexText)
	if fallback.present {
		result.canonText = valueBytes(s.rt.Values, fallback.value)
		result.textMember = fallback.member
		if fallback.key.Ref.Present {
			result.textKeyKind = fallback.key.Kind
			result.textKeyBytes = valueBytes(s.rt.Values, fallback.key.Ref)
		}
		trackDefault(result.canonText, result.textMember)
	} else {
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

	fixed := selectTextFixedConstraint(elem, elemOK, ct, hasComplexText)
	if result.canonText != nil && hasContent && fixed.present {
		matched, err := s.fixedValueMatches(
			result.textValidator,
			fixed.member,
			result.canonText,
			valueMetrics{keyKind: result.textKeyKind, keyBytes: result.textKeyBytes},
			resolver,
			fixed.value,
			fixed.key,
		)
		if err != nil {
			s.ensurePath(path)
			errs = append(errs, err)
		} else if !matched {
			s.ensurePath(path)
			errs = append(errs, newValidationError(xsderrors.ErrElementFixedValue, "fixed element value mismatch"))
		}
	}

	return errs, result
}
