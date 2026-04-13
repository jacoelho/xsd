package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// StartElement validates one start-element event and returns resolved runtime metadata.
func (s *Session) StartElement(match StartMatch, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte, inputAttrs []Start, resolver value.NSResolver) (StartResult, error) {
	if s == nil || s.rt == nil {
		return StartResult{}, newValidationError(xsderrors.ErrSchemaNotLoaded, "schema not loaded")
	}
	classified, err := s.classifyAttrs(inputAttrs, false)
	if err != nil {
		return StartResult{}, err
	}
	return ResolveStartResult(s.rt, match, sym, nsID, nsBytes, classified, resolver)
}

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

	attrResult, err := s.validateAttributesClassifiedWithStorage(result.Type, attrs, resolver, classified, s.needsIdentityAttrs(result.Elem), false)
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

func (s *Session) makeStartAttrs(resolvedAttrs []xmlstream.ResolvedAttr) []Start {
	if len(resolvedAttrs) == 0 {
		return nil
	}
	out := s.attrState.Starts[:0]
	if cap(out) < len(resolvedAttrs) {
		out = make([]Start, 0, len(resolvedAttrs))
	}
	for _, attr := range resolvedAttrs {
		entry := s.internName(attr.NameID, attr.NS, attr.Local)
		local := attr.Local
		nsBytes := attr.NS
		nameCached := false
		storedNS, storedLocal := s.Names.EntryBytes(entry)
		if entry.LocalLen != 0 {
			local = storedLocal
			nameCached = true
		}
		if entry.NSLen != 0 {
			nsBytes = storedNS
			nameCached = true
		}
		out = append(out, Start{
			Sym:        entry.Sym,
			NS:         entry.NS,
			NSBytes:    nsBytes,
			Local:      local,
			NameCached: nameCached,
			Value:      attr.Value,
		})
	}
	s.attrState.Starts = out[:0]
	return out
}

func (s *Session) buildStartFrame(entry NameEntry, ev *xmlstream.ResolvedEvent, result StartResult, typ runtime.Type) (elemFrame, error) {
	plan, err := PlanStartFrame(
		StartNameInput{
			Local:  ev.Local,
			NS:     ev.NS,
			Cached: entry.LocalLen != 0 || entry.NSLen != 0,
		},
		result,
		typ,
		s.rt.ComplexTypes,
	)
	if err != nil {
		return elemFrame{}, err
	}

	frame := elemFrame{
		local:   plan.Local,
		ns:      plan.NS,
		model:   plan.Model,
		name:    NameID(ev.NameID),
		elem:    result.Elem,
		typ:     result.Type,
		content: plan.Content,
		mixed:   plan.Mixed,
		nilled:  result.Nilled,
	}
	if frame.model.Kind != runtime.ModelNone {
		state, err := s.InitModelState(frame.model)
		if err != nil {
			return elemFrame{}, err
		}
		frame.modelState = state
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
