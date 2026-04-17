package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// StartElement validates one start-element event and returns resolved runtime metadata.
func (s *Session) StartElement(match StartMatch, sym runtime.SymbolID, nsID runtime.NamespaceID, nsBytes []byte, inputAttrs []Start, resolver value.NSResolver) (StartResult, error) {
	plan, err := s.planStartElement(startPlanInput{
		Match:               match,
		Entry:               NameEntry{Sym: sym, NS: nsID},
		NS:                  nsBytes,
		Resolver:            resolver,
		HasMatch:            true,
		CheckAttrDuplicates: false,
		Attrs:               inputAttrs,
	})
	if err != nil {
		return StartResult{}, err
	}
	return plan.Result, nil
}

func (s *Session) handleStartElement(ev *xmlstream.ResolvedEvent, resolver sessionResolver) error {
	if ev == nil {
		return fmt.Errorf("start element event missing")
	}
	entry := s.internName(ev.NameID, ev.NS, ev.Local)

	s.pushNamespaceScope(s.io.reader.NamespaceDecls(ev.ScopeDepth))

	var parent *elemFrame
	if len(s.elemStack) != 0 {
		parent = &s.elemStack[len(s.elemStack)-1]
		parent.hasChildElements = true
	}

	attrs := s.makeStartAttrs(ev.Attrs)
	plan, err := s.planStartElement(startPlanInput{
		Entry:                entry,
		Local:                ev.Local,
		NS:                   ev.NS,
		Name:                 NameID(ev.NameID),
		Resolver:             resolver,
		Parent:               parent,
		Root:                 len(s.elemStack) == 0,
		CheckAttrDuplicates:  true,
		TrackIdentityAttrs:   true,
		ValidateAttrs:        true,
		BuildFrame:           true,
		StoreAttributeValues: false,
		Attrs:                attrs,
	})
	if err != nil {
		if parent != nil && plan.ChildErrorReported {
			parent.childErrorReported = true
		}
		s.popNamespaceScope()
		return err
	}
	if plan.Skip {
		return s.skipSubtreeAndPopScope()
	}
	frame := plan.Frame
	s.ResetText(&frame.text)
	s.elemStack = append(s.elemStack, frame)

	plan.Identity.Attrs = plan.Attrs.Attrs
	plan.Identity.Applied = plan.Attrs.Applied
	if err := s.identityStart(plan.Identity); err != nil {
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
	out := s.attrs.attrState.Starts[:0]
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
	s.attrs.attrState.Starts = out[:0]
	return out
}

func (s *Session) skipSubtreeAndPopScope() error {
	if err := s.io.reader.SkipSubtree(); err != nil {
		s.popNamespaceScope()
		return err
	}
	s.popNamespaceScope()
	return nil
}
