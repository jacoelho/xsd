package validator

import xsderrors "github.com/jacoelho/xsd/internal/xsderrors"

func (s *Session) validationList() error {
	if s == nil || len(s.validationErrors) == 0 {
		return nil
	}
	out := make(xsderrors.ValidationList, len(s.validationErrors))
	copy(out, s.validationErrors)
	out.Sort()
	return out
}
