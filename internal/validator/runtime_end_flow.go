package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
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
	textValidator := textState.textValidator
	textMember := textState.textMember

	if s.hasIdentityConstraints() && textKeyKind == runtime.VKInvalid && canonText != nil && textValidator != 0 {
		kind, key, err := s.keyForCanonicalValue(textValidator, canonText, resolver, textMember)
		if err != nil {
			errs = s.appendEndError(errs, &path, err)
		} else {
			textKeyKind = kind
			textKeyBytes = s.storeKey(key)
		}
	}

	if err := s.icState.end(s.rt, identityEndInput{
		Text:      canonText,
		TextState: frame.text,
		KeyKind:   textKeyKind,
		KeyBytes:  textKeyBytes,
	}); err != nil {
		errs = s.appendEndError(errs, &path, err)
	}

	if path == "" && s.icState.hasCommitted() {
		path = s.pathString()
	}

	s.releaseText(frame.text)
	s.elemStack = s.elemStack[:index]
	s.popNamespaceScope()
	return errs, path
}
