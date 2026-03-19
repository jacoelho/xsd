package preprocessor

func (s *loadSession) rollback() {
	if s == nil || s.loader == nil {
		return
	}
	s.journal.Rollback(s.loader.stateRollbackCallbacks())
}
