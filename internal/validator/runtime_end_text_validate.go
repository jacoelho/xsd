package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

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
	ct, hasComplexText, textValidator, typeErr := s.resolveEndTextType(frame, typ)
	if typeErr != nil {
		s.ensurePath(path)
		errs = append(errs, typeErr)
	}
	result.textValidator = textValidator

	valueErrs := s.resolveEndTextValue(&result, frame, rawText, hasContent, elem, elemOK, ct, hasComplexText, resolver, path)
	errs = append(errs, valueErrs...)

	fixedErrs := s.validateEndTextFixed(result, hasContent, elem, elemOK, ct, hasComplexText, resolver, path)
	errs = append(errs, fixedErrs...)

	return errs, result
}
