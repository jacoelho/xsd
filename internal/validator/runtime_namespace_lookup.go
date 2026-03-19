package validator

func (s *Session) lookupNamespace(prefix []byte) ([]byte, bool) {
	if s == nil {
		return nil, false
	}
	return s.Names.LookupNamespace(prefix)
}
