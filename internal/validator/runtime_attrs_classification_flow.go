package validator

func (s *Session) classifyAttrs(input []Start, checkDuplicates bool) (Classification, error) {
	if s == nil {
		return Classification{}, nil
	}
	return s.attrState.Classify(s.rt, input, checkDuplicates)
}
