package validator

import (
	"bytes"
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (s *Session) handleStartElement(ev *xmlstream.ResolvedEvent, resolver sessionResolver) error {
	if ev == nil {
		return fmt.Errorf("start element event missing")
	}
	entry := s.internName(ev.NameID, ev.NS, ev.Local)
	sym := entry.Sym
	nsID := entry.NS

	decls := s.reader.NamespaceDeclsAt(ev.ScopeDepth)
	s.pushNamespaceScope(decls)

	var match StartMatch
	parentIndex := -1
	if len(s.elemStack) > 0 {
		parentIndex = len(s.elemStack) - 1
		s.elemStack[parentIndex].hasChildElements = true
	}
	if parentIndex == -1 {
		switch s.rt.RootPolicy {
		case runtime.RootAny:
			if sym == 0 {
				if err := s.reader.SkipSubtree(); err != nil {
					s.popNamespaceScope()
					return err
				}
				s.popNamespaceScope()
				return nil
			}
			elemID, ok := s.globalElementBySymbol(sym)
			if !ok {
				if err := s.reader.SkipSubtree(); err != nil {
					s.popNamespaceScope()
					return err
				}
				s.popNamespaceScope()
				return nil
			}
			match = StartMatch{Kind: MatchElem, Elem: elemID}
		case runtime.RootStrict:
			if sym == 0 {
				s.popNamespaceScope()
				return newValidationError(xsderrors.ErrValidateRootNotDeclared, "root element not declared")
			}
			elemID, ok := s.globalElementBySymbol(sym)
			if !ok {
				s.popNamespaceScope()
				return newValidationError(xsderrors.ErrValidateRootNotDeclared, "root element not declared")
			}
			match = StartMatch{Kind: MatchElem, Elem: elemID}
		}
	} else {
		parent := &s.elemStack[parentIndex]
		if parent.nilled {
			parent.childErrorReported = true
			s.popNamespaceScope()
			return newValidationError(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty")
		}
		if parent.content == runtime.ContentSimple || parent.content == runtime.ContentEmpty {
			parent.childErrorReported = true
			s.popNamespaceScope()
			if parent.content == runtime.ContentSimple {
				return newValidationError(xsderrors.ErrTextInElementOnly, "element not allowed in simple content")
			}
			return newValidationError(xsderrors.ErrUnexpectedElement, "element not allowed in empty content")
		}
		if parent.model.Kind == runtime.ModelNone {
			s.popNamespaceScope()
			return newValidationError(xsderrors.ErrUnexpectedElement, "no content model match")
		}
		var err error
		match, err = s.StepModel(parent.model, &parent.modelState, sym, nsID, ev.NS)
		if err != nil {
			s.popNamespaceScope()
			return err
		}
	}

	attrs := s.makeStartAttrs(ev.Attrs)
	result, err := s.StartElement(match, sym, nsID, ev.NS, attrs, resolver)
	if err != nil {
		s.popNamespaceScope()
		return err
	}
	if result.Skip {
		err = s.reader.SkipSubtree()
		if err != nil {
			s.popNamespaceScope()
			return err
		}
		s.popNamespaceScope()
		return nil
	}

	attrResult, err := s.ValidateAttributes(result.Type, attrs, resolver)
	if err != nil {
		s.popNamespaceScope()
		return err
	}

	typ, ok := s.typeByID(result.Type)
	if !ok {
		s.popNamespaceScope()
		return fmt.Errorf("type %d not found", result.Type)
	}

	frame := elemFrame{
		name:   NameID(ev.NameID),
		elem:   result.Elem,
		typ:    result.Type,
		nilled: result.Nilled,
	}
	if entry.LocalLen == 0 && entry.NSLen == 0 {
		if len(ev.Local) > 0 {
			frame.local = append([]byte(nil), ev.Local...)
		}
		if len(ev.NS) > 0 {
			frame.ns = append([]byte(nil), ev.NS...)
		}
	}

	switch typ.Kind {
	case runtime.TypeSimple, runtime.TypeBuiltin:
		frame.content = runtime.ContentSimple
	case runtime.TypeComplex:
		if typ.Complex.ID == 0 || int(typ.Complex.ID) >= len(s.rt.ComplexTypes) {
			s.popNamespaceScope()
			return fmt.Errorf("complex type %d missing", result.Type)
		}
		ct := s.rt.ComplexTypes[typ.Complex.ID]
		frame.content = ct.Content
		frame.mixed = ct.Mixed
		frame.model = ct.Model
		if frame.model.Kind != runtime.ModelNone {
			state, err := s.InitModelState(frame.model)
			if err != nil {
				s.popNamespaceScope()
				return err
			}
			frame.modelState = state
		}
	default:
		s.popNamespaceScope()
		return fmt.Errorf("unknown type kind %d", typ.Kind)
	}

	s.ResetText(&frame.text)
	s.elemStack = append(s.elemStack, frame)

	if err := s.identityStart(identityStartInput{
		Elem:    result.Elem,
		Type:    result.Type,
		Sym:     sym,
		NS:      nsID,
		Attrs:   attrResult.Attrs,
		Applied: attrResult.Applied,
		Nilled:  result.Nilled,
	}); err != nil {
		s.releaseText(frame.text)
		s.elemStack = s.elemStack[:len(s.elemStack)-1]
		s.popNamespaceScope()
		return err
	}

	return nil
}

func (s *Session) handleCharData(ev *xmlstream.ResolvedEvent) error {
	if ev == nil {
		return fmt.Errorf("character data event missing")
	}
	if len(s.elemStack) == 0 {
		return nil
	}
	frame := &s.elemStack[len(s.elemStack)-1]
	return s.ConsumeText(&frame.text, frame.content, frame.mixed, frame.nilled, ev.Text)
}

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
		if path == "" {
			path = s.pathString()
		}
		errs = append(errs, fmt.Errorf("type %d not found", frame.typ))
	}
	elem, elemOK := s.element(frame.elem)
	if !elemOK {
		if path == "" {
			path = s.pathString()
		}
		errs = append(errs, fmt.Errorf("element %d out of range", frame.elem))
	}

	if frame.nilled {
		if (frame.text.HasText || frame.hasChildElements) && !frame.childErrorReported {
			if path == "" {
				path = s.pathString()
			}
			errs = append(errs, newValidationError(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty"))
		}
	} else {
		if frame.model.Kind != runtime.ModelNone {
			if err := s.AcceptModel(frame.model, &frame.modelState); err != nil {
				if path == "" {
					path = s.pathString()
				}
				errs = append(errs, err)
			}
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
			if path == "" {
				path = s.pathString()
			}
			errs = append(errs, err)
		} else {
			textKeyKind = kind
			textKeyBytes = s.storeKey(key)
		}
	}

	if err := s.identityEnd(identityEndInput{
		Text:      canonText,
		TextState: frame.text,
		KeyKind:   textKeyKind,
		KeyBytes:  textKeyBytes,
	}); err != nil {
		if path == "" {
			path = s.pathString()
		}
		errs = append(errs, err)
	}

	if path == "" && len(s.icState.pending) > 0 {
		path = s.pathString()
	}

	s.releaseText(frame.text)
	s.elemStack = s.elemStack[:index]
	s.popNamespaceScope()
	return errs, path
}

type endTextState struct {
	canonText     []byte
	textKeyKind   runtime.ValueKind
	textKeyBytes  []byte
	textValidator runtime.ValidatorID
	textMember    runtime.ValidatorID
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

func (s *Session) ensurePath(path *string) {
	if path == nil || *path != "" {
		return
	}
	*path = s.pathString()
}
