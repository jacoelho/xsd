package validator

func (s *Session) prepareAttrClasses(count int) []attrClass {
	classes := s.attrClassBuf
	if cap(classes) < count {
		classes = make([]attrClass, count)
	} else {
		classes = classes[:count]
	}
	s.attrClassBuf = classes
	return classes
}
