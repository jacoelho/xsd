package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/xmlstream"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func (s *Session) handleEndElement(ev *xmlstream.ResolvedEvent, resolver sessionResolver) ([]error, string) {
	if ev == nil {
		return []error{fmt.Errorf("end element event missing")}, s.pathString()
	}
	if len(s.elemStack) == 0 {
		return []error{fmt.Errorf("unexpected end element")}, s.pathString()
	}
	index := len(s.elemStack) - 1
	frame := s.elemStack[index]

	var errs []error
	path := ""

	typ, ok := s.typeByID(frame.typ)
	if !ok {
		errs = s.appendEndError(errs, &path, fmt.Errorf("type %d not found", frame.typ))
	}
	elem, elemOK := s.element(frame.elem)
	if !elemOK {
		errs = s.appendEndError(errs, &path, fmt.Errorf("element %d out of range", frame.elem))
	}

	if frame.nilled {
		if (frame.text.HasText || frame.hasChildElements) && !frame.childErrorReported {
			errs = s.appendEndError(errs, &path, newValidationError(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty"))
		}
	} else if frame.model.Kind != runtime.ModelNone {
		if err := s.AcceptModel(frame.model, &frame.modelState); err != nil {
			errs = s.appendEndError(errs, &path, err)
		}
	}

	textErrs, textState := s.validateEndElementText(frame, typ, ok, elem, elemOK, resolver, &path)
	errs = append(errs, textErrs...)
	canonText := textState.canonText
	textKeyKind := textState.textKeyKind
	textKeyBytes := textState.textKeyBytes

	if err := s.identity.icState.end(s.rt, identityEndInput{
		Text:      canonText,
		TextState: frame.text,
		KeyKind:   textKeyKind,
		KeyBytes:  textKeyBytes,
	}); err != nil {
		errs = s.appendEndError(errs, &path, err)
	}

	if path == "" && s.identity.icState.HasCommitted() {
		path = s.pathString()
	}

	s.releaseText(frame.text)
	s.elemStack = s.elemStack[:index]
	s.popNamespaceScope()
	return errs, path
}

func (s *Session) appendEndError(errs []error, path *string, err error) []error {
	if err == nil {
		return errs
	}
	if path != nil && *path == "" {
		*path = s.pathString()
	}
	return append(errs, err)
}

func (s *Session) ensurePath(path *string) {
	if path == nil || *path != "" {
		return
	}
	*path = s.pathString()
}

func (s *Session) validateEndElementText(frame elemFrame, typ runtime.Type, typeOK bool, elem runtime.Element, elemOK bool, resolver sessionResolver, path *string) ([]error, endTextState) {
	result := endTextState{}
	hasValueConstraint := elemOK && (elem.Default.Present || elem.Fixed.Present)
	if frame.nilled || !typeOK || (typ.Kind != runtime.TypeSimple && typ.Kind != runtime.TypeBuiltin && frame.content != runtime.ContentSimple && !hasValueConstraint) {
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
	if len(fixedErrs) == 0 && frame.hasChildElements && elemOK && elem.Fixed.Present && typ.Kind == runtime.TypeComplex {
		s.ensurePath(path)
		errs = append(errs, newValidationError(xsderrors.ErrElementFixedValue, "fixed element value mismatch"))
	}

	return errs, result
}
