package validator

import "fmt"

func (s *identityState) start(sess *Session, in identityStartInput) error {
	if sess == nil || sess.rt == nil {
		return fmt.Errorf("identity: schema missing")
	}
	rt := sess.rt
	elem, ok := elementByID(rt, in.Elem)
	if !ok {
		return fmt.Errorf("identity: element %d not found", in.Elem)
	}
	hasConstraints := elem.ICLen > 0
	if !s.active && !hasConstraints {
		return nil
	}
	s.active = true

	s.nextNodeID++
	frame := rtIdentityFrame{
		id:     s.nextNodeID,
		depth:  s.frames.Len(),
		sym:    in.Sym,
		ns:     in.NS,
		elem:   in.Elem,
		typ:    in.Type,
		nilled: in.Nilled,
	}
	s.frames.Push(frame)
	frames := s.frames.Items()
	current := &frames[len(frames)-1]

	if hasConstraints {
		if err := s.openScope(rt, current, elem); err != nil {
			return err
		}
	}
	if s.scopes.Len() == 0 {
		return nil
	}

	var attrs []rtIdentityAttr
	if len(in.Attrs) != 0 || len(in.Applied) != 0 {
		attrs = collectIdentityAttrs(rt, in.Attrs, in.Applied, sess.internIdentityAttrName)
	}
	s.matchSelectors(rt, current.depth)
	s.applyFieldSelections(rt, current.depth, attrs)
	return nil
}
