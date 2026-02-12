package validator

func (s *Session) finalizeIdentity() []error {
	if s == nil {
		return nil
	}
	if errs := s.icState.drainUncommitted(); len(errs) > 0 {
		return errs
	}
	if pending := s.icState.drainCommitted(); len(pending) > 0 {
		return pending
	}
	return nil
}
