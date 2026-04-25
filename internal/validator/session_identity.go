package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

// SessionIdentity owns per-session identity-constraint state and caches.
type SessionIdentity struct {
	idTable       map[string]struct{}
	identityAttrs AttrNames
	attrScratch   []Attr
	idRefs        []string
	icState       identityState
}

type identityState struct {
	arena *Arena
	State[RuntimeFrame]
}

type identityStartInput struct {
	Attrs   []Start
	Applied []Applied
	Elem    runtime.ElemID
	Type    runtime.TypeID
	Sym     runtime.SymbolID
	NS      runtime.NamespaceID
	Nilled  bool
}

type identityEndInput struct {
	Text      []byte
	KeyBytes  []byte
	TextState TextState
	KeyKind   runtime.ValueKind
}

func (id *SessionIdentity) Reset(arena *Arena, entryLimit, idTableLimit int) {
	if id == nil {
		return
	}
	id.icState.Reset(arena)
	id.resetIDTable(idTableLimit)
	id.idRefs = id.idRefs[:0]
	id.attrScratch = id.attrScratch[:0]
	id.identityAttrs.Reset(entryLimit)
}

func (id *SessionIdentity) Shrink(entryLimit int) {
	if id == nil {
		return
	}
	id.idRefs = shrinkSliceCap(id.idRefs, entryLimit)
	id.attrScratch = shrinkSliceCap(id.attrScratch, entryLimit)
	id.icState.Shrink(entryLimit)
}

func (id *SessionIdentity) resetIDTable(limit int) {
	if id == nil || id.idTable == nil {
		return
	}
	if len(id.idTable) > limit {
		id.idTable = nil
		return
	}
	clear(id.idTable)
}

func (s *identityState) Reset(arena *Arena) {
	if s == nil {
		return
	}
	s.arena = arena
	s.State.Reset()
}

func (s *identityState) Shrink(entryLimit int) {
	if s == nil {
		return
	}
	s.Uncommitted = shrinkSliceCap(s.Uncommitted, entryLimit)
	s.Committed = shrinkSliceCap(s.Committed, entryLimit)
	dropStacksOverCap(entryLimit, &s.Frames, &s.Scopes)
}

func (s *Session) internIdentityAttrName(ns, local []byte) AttrNameID {
	if s == nil {
		return 0
	}
	return s.identity.identityAttrs.Intern(NameHash(ns, local), ns, local)
}

func (s *Session) identityStart(in identityStartInput) error {
	if s == nil {
		return nil
	}
	snapshot := s.identity.icState.Checkpoint()
	err := s.identity.icState.start(s, in)
	if err != nil {
		s.identity.icState.Rollback(snapshot)
	}
	return err
}

func (s *identityState) start(sess *Session, in identityStartInput) error {
	if sess == nil || sess.rt == nil {
		return fmt.Errorf("identity: schema missing")
	}
	return StartFrame(sess.rt, &s.State, StartInput{
		Elem:   in.Elem,
		Type:   in.Type,
		Sym:    in.Sym,
		NS:     in.NS,
		Nilled: in.Nilled,
	}, func() []Attr {
		if len(in.Attrs) == 0 && len(in.Applied) == 0 {
			return nil
		}
		return sess.collectIdentityAttrs(in.Attrs, in.Applied)
	})
}

func (s *identityState) end(rt *runtime.Schema, in identityEndInput) error {
	if rt == nil || !s.Active || s.Frames.Len() == 0 {
		return nil
	}
	frames := s.Frames.Items()
	index := len(frames) - 1
	frame := &frames[index]
	if err := CloseFrame(rt, s.arena, &s.State, frame.ID, frame.Elem, frame.Nilled, frame.Captures, frame.Matches, in.KeyKind, in.KeyBytes); err != nil {
		return err
	}

	s.Frames.Pop()
	if s.Frames.Len() == 0 && s.Scopes.Len() == 0 {
		s.Active = false
	}
	return nil
}

func (s *Session) collectIdentityAttrs(startAttrs []Start, applied []Applied) []Attr {
	if s == nil || s.rt == nil || len(startAttrs) == 0 && len(applied) == 0 {
		return nil
	}
	out := s.identity.attrScratch[:0]
	for _, attr := range startAttrs {
		local := attr.Local
		if len(local) == 0 && attr.Sym != 0 {
			local = s.rt.Symbols.LocalBytes(attr.Sym)
		}
		nsBytes := attr.NSBytes
		if len(nsBytes) == 0 && attr.NS != 0 {
			nsBytes = s.rt.Namespaces.Bytes(attr.NS)
		}
		nameID := AttrNameID(0)
		if attr.Sym == 0 {
			nameID = s.internIdentityAttrName(nsBytes, local)
		}
		out = append(out, Attr{
			Sym:      attr.Sym,
			NS:       attr.NS,
			NSBytes:  nsBytes,
			Local:    local,
			KeyKind:  attr.KeyKind,
			KeyBytes: attr.KeyBytes,
			NameID:   nameID,
		})
	}
	for _, attr := range applied {
		if attr.Name == 0 {
			continue
		}
		nsID := runtime.NamespaceID(0)
		if int(attr.Name) < len(s.rt.Symbols.NS) {
			nsID = s.rt.Symbols.NS[attr.Name]
		}
		out = append(out, Attr{
			Sym:      attr.Name,
			NS:       nsID,
			NSBytes:  s.rt.Namespaces.Bytes(nsID),
			Local:    s.rt.Symbols.LocalBytes(attr.Name),
			KeyKind:  attr.KeyKind,
			KeyBytes: attr.KeyBytes,
		})
	}
	s.identity.attrScratch = out
	return out
}
