package validator

type validationExecutor struct {
	s        *Session
	rootSeen bool
	allowBOM bool
}
