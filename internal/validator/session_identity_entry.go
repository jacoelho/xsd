package validator

func (s *Session) identityStart(in identityStartInput) error {
	if s == nil {
		return nil
	}
	snapshot := s.icState.checkpoint()
	err := s.icState.start(s, in)
	if err != nil {
		s.icState.rollback(snapshot)
	}
	return err
}
