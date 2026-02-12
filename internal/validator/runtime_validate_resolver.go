package validator

type sessionResolver struct {
	s *Session
}

func (r sessionResolver) ResolvePrefix(prefix []byte) ([]byte, bool) {
	if r.s == nil {
		return nil, false
	}
	return r.s.lookupNamespace(prefix)
}
