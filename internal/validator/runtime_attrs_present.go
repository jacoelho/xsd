package validator

func (s *Session) prepareAttrPresent(size int) []bool {
	present := s.attrPresent
	if cap(present) < size {
		present = make([]bool, size)
	} else {
		present = present[:size]
		for i := range present {
			present[i] = false
		}
	}
	s.attrPresent = present
	return present
}
