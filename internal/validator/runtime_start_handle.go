package validator

import (
	"fmt"

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

	s.pushNamespaceScope(s.reader.NamespaceDecls(ev.ScopeDepth))

	eventInput := StartEventInput{
		Root: len(s.elemStack) == 0,
		Sym:  sym,
		NSID: nsID,
		NS:   ev.NS,
	}
	var parent *elemFrame
	if !eventInput.Root {
		parent = &s.elemStack[len(s.elemStack)-1]
		parent.hasChildElements = true
		eventInput.Parent = StartChildInput{
			Content: parent.content,
			Model:   parent.model,
			Nilled:  parent.nilled,
		}
	}

	attrs := s.makeStartAttrs(ev.Attrs)
	classified, err := s.classifyAttrs(attrs, true)
	if err != nil {
		s.popNamespaceScope()
		return err
	}
	eventInput.Attrs = classified
	event, err := ResolveStartEvent(
		s.rt,
		eventInput,
		resolver,
		func(ref runtime.ModelRef, sym runtime.SymbolID, nsID runtime.NamespaceID, ns []byte) (StartMatch, error) {
			return s.StepModel(ref, &parent.modelState, sym, nsID, ns)
		},
	)
	if err != nil {
		if parent != nil && event.ChildErrorReported {
			parent.childErrorReported = true
		}
		s.popNamespaceScope()
		return err
	}
	if event.Result.Skip {
		return s.skipSubtreeAndPopScope()
	}
	result := event.Result

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
