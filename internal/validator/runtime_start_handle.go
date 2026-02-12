package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func (s *Session) handleStartElement(ev *xmlstream.ResolvedEvent, resolver sessionResolver) error {
	if ev == nil {
		return fmt.Errorf("start element event missing")
	}
	entry := s.internName(ev.NameID, ev.NS, ev.Local)
	sym := entry.Sym
	nsID := entry.NS

	s.pushNamespaceScope(s.reader.NamespaceDecls(ev.ScopeDepth))

	var match StartMatch
	if len(s.elemStack) == 0 {
		decision, err := s.resolveRootStartMatch(sym, nsID)
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
	classified, err := s.classifyAttrs(attrs, true)
	if err != nil {
		s.popNamespaceScope()
		return err
	}
	result, err := s.startElementClassified(match, sym, nsID, ev.NS, resolver, classified)
	if err != nil {
		s.popNamespaceScope()
		return err
	}
	if result.Skip {
		return s.skipSubtreeAndPopScope()
	}

	attrResult, err := s.validateAttributesClassified(result.Type, attrs, resolver, classified)
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
