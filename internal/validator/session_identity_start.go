package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/validator/identity"
)

func (s *Session) identityStart(in identityStartInput) error {
	if s == nil {
		return nil
	}
	snapshot := s.icState.Checkpoint()
	err := s.icState.start(s, in)
	if err != nil {
		s.icState.Rollback(snapshot)
	}
	return err
}

func (s *identityState) start(sess *Session, in identityStartInput) error {
	if sess == nil || sess.rt == nil {
		return fmt.Errorf("identity: schema missing")
	}
	return identity.StartFrame(sess.rt, &s.State, identity.StartInput{
		Elem:   in.Elem,
		Type:   in.Type,
		Sym:    in.Sym,
		NS:     in.NS,
		Nilled: in.Nilled,
	}, func() []identity.Attr {
		if len(in.Attrs) == 0 && len(in.Applied) == 0 {
			return nil
		}
		return collectIdentityAttrs(sess.rt, in.Attrs, in.Applied, sess.internIdentityAttrName)
	})
}
