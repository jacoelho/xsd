package validator

func (s *Session) skipSubtreeAndPopScope() error {
	if err := s.reader.SkipSubtree(); err != nil {
		s.popNamespaceScope()
		return err
	}
	s.popNamespaceScope()
	return nil
}
