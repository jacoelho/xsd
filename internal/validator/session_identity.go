package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

// SessionIdentity owns per-session identity-constraint state and caches.
type SessionIdentity struct {
	idTable       map[string]struct{}
	identityAttrs AttrNames
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
	id.identityAttrs.Reset(entryLimit)
}

func (id *SessionIdentity) Shrink(entryLimit int) {
	if id == nil {
		return
	}
	id.idRefs = shrinkSliceCap(id.idRefs, entryLimit)
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
		return collectIdentityAttrs(sess.rt, in.Attrs, in.Applied, sess.internIdentityAttrName)
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

func collectIdentityAttrs(rt *runtime.Schema, startAttrs []Start, applied []Applied, intern func(ns, local []byte) AttrNameID) []Attr {
	if len(startAttrs) == 0 && len(applied) == 0 {
		return nil
	}
	rawAttrs := make([]RawAttr, 0, len(startAttrs))
	for _, attr := range startAttrs {
		rawAttrs = append(rawAttrs, RawAttr{
			NSBytes:  attr.NSBytes,
			Local:    attr.Local,
			KeyBytes: attr.KeyBytes,
			Sym:      attr.Sym,
			NS:       attr.NS,
			KeyKind:  attr.KeyKind,
		})
	}
	appliedAttrs := make([]AppliedAttr, 0, len(applied))
	for _, ap := range applied {
		appliedAttrs = append(appliedAttrs, AppliedAttr{
			Name:     ap.Name,
			KeyBytes: ap.KeyBytes,
			KeyKind:  ap.KeyKind,
		})
	}
	return CollectAttrs(rt, rawAttrs, appliedAttrs, intern)
}
