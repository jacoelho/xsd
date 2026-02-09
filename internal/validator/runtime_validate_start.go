package validator

import (
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

	s.pushNamespaceScope(s.reader.NamespaceDeclsSeq(ev.ScopeDepth))

	var match StartMatch
	if len(s.elemStack) == 0 {
		decision, err := s.resolveRootStartMatch(sym)
		if err != nil {
			s.popNamespaceScope()
			return err
		}
		if decision.skip {
			return s.skipSubtreeAndPopScope()
		}
		match = decision.match
	} else {
		parent := &s.elemStack[len(s.elemStack)-1]
		parent.hasChildElements = true
		var err error
		match, err = s.resolveChildStartMatch(parent, sym, nsID, ev.NS)
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
		return s.skipSubtreeAndPopScope()
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
	frame, err := s.buildStartFrame(entry, ev, result, typ)
	if err != nil {
		s.popNamespaceScope()
		return err
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

type rootStartDecision struct {
	match StartMatch
	skip  bool
}

func (s *Session) resolveRootStartMatch(sym runtime.SymbolID) (rootStartDecision, error) {
	switch s.rt.RootPolicy {
	case runtime.RootAny:
		if sym == 0 {
			return rootStartDecision{skip: true}, nil
		}
		elemID, ok := s.globalElementBySymbol(sym)
		if !ok {
			return rootStartDecision{skip: true}, nil
		}
		return rootStartDecision{match: StartMatch{Kind: MatchElem, Elem: elemID}}, nil
	case runtime.RootStrict:
		if sym == 0 {
			return rootStartDecision{}, newValidationError(xsderrors.ErrValidateRootNotDeclared, "root element not declared")
		}
		elemID, ok := s.globalElementBySymbol(sym)
		if !ok {
			return rootStartDecision{}, newValidationError(xsderrors.ErrValidateRootNotDeclared, "root element not declared")
		}
		return rootStartDecision{match: StartMatch{Kind: MatchElem, Elem: elemID}}, nil
	default:
		return rootStartDecision{}, newValidationError(xsderrors.ErrValidateRootNotDeclared, "root element not declared")
	}
}

func (s *Session) resolveChildStartMatch(parent *elemFrame, sym runtime.SymbolID, nsID runtime.NamespaceID, ns []byte) (StartMatch, error) {
	if parent == nil {
		return StartMatch{}, fmt.Errorf("parent frame missing")
	}
	if parent.nilled {
		parent.childErrorReported = true
		return StartMatch{}, newValidationError(xsderrors.ErrValidateNilledNotEmpty, "element with xsi:nil='true' must be empty")
	}
	if parent.content == runtime.ContentSimple || parent.content == runtime.ContentEmpty {
		parent.childErrorReported = true
		if parent.content == runtime.ContentSimple {
			return StartMatch{}, newValidationError(xsderrors.ErrTextInElementOnly, "element not allowed in simple content")
		}
		return StartMatch{}, newValidationError(xsderrors.ErrUnexpectedElement, "element not allowed in empty content")
	}
	if parent.model.Kind == runtime.ModelNone {
		return StartMatch{}, newValidationError(xsderrors.ErrUnexpectedElement, "no content model match")
	}
	return s.StepModel(parent.model, &parent.modelState, sym, nsID, ns)
}

func (s *Session) buildStartFrame(entry nameEntry, ev *xmlstream.ResolvedEvent, result StartResult, typ runtime.Type) (elemFrame, error) {
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
			return elemFrame{}, fmt.Errorf("complex type %d missing", result.Type)
		}
		ct := s.rt.ComplexTypes[typ.Complex.ID]
		frame.content = ct.Content
		frame.mixed = ct.Mixed
		frame.model = ct.Model
		if frame.model.Kind != runtime.ModelNone {
			state, err := s.InitModelState(frame.model)
			if err != nil {
				return elemFrame{}, err
			}
			frame.modelState = state
		}
	default:
		return elemFrame{}, fmt.Errorf("unknown type kind %d", typ.Kind)
	}
	return frame, nil
}

func (s *Session) skipSubtreeAndPopScope() error {
	if err := s.reader.SkipSubtree(); err != nil {
		s.popNamespaceScope()
		return err
	}
	s.popNamespaceScope()
	return nil
}
