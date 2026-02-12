package validator

import xsderrors "github.com/jacoelho/xsd/errors"

func (s *Session) recordID(valueBytes []byte) error {
	if s == nil {
		return nil
	}
	if s.idTable == nil {
		s.idTable = make(map[string]struct{}, 32)
	}
	key := string(valueBytes)
	if _, ok := s.idTable[key]; ok {
		return newValidationError(xsderrors.ErrDuplicateID, "duplicate ID value")
	}
	s.idTable[key] = struct{}{}
	return nil
}

func (s *Session) recordIDRef(valueBytes []byte) {
	if s == nil {
		return
	}
	s.idRefs = append(s.idRefs, string(valueBytes))
}

func (s *Session) validateIDRefs() []error {
	if s == nil {
		return nil
	}
	if len(s.idRefs) == 0 {
		return nil
	}
	var errs []error
	for _, ref := range s.idRefs {
		if _, ok := s.idTable[ref]; !ok {
			errs = append(errs, newValidationError(xsderrors.ErrIDRefNotFound, "IDREF value not found"))
		}
	}
	return errs
}
