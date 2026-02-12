package validator

type validationExecutor struct {
	s        *Session
	resolver sessionResolver
	rootSeen bool
	allowBOM bool
}

type subtreeSkipper interface {
	SkipSubtree() error
}

func newValidationExecutor(s *Session) *validationExecutor {
	return &validationExecutor{
		s:        s,
		resolver: sessionResolver{s: s},
		allowBOM: true,
	}
}
