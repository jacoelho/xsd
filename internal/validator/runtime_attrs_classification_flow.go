package validator

import "github.com/jacoelho/xsd/internal/validator/attrs"

func (s *Session) classifyAttrs(input []attrs.Start, checkDuplicates bool) (attrs.Classification, error) {
	if s == nil {
		return attrs.Classification{}, nil
	}
	return s.attrState.Classify(s.rt, input, checkDuplicates)
}
