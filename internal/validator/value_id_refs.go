package validator

import (
	"unsafe"

	xsderrors "github.com/jacoelho/xsd/errors"
)

func (s *Session) recordID(valueBytes []byte) error {
	if s == nil {
		return nil
	}
	if s.identity.idTable == nil {
		s.identity.idTable = make(map[string]struct{}, 32)
	}
	key := unsafeBytesString(valueBytes)
	if _, ok := s.identity.idTable[key]; ok {
		return newValidationError(xsderrors.ErrDuplicateID, "duplicate ID value")
	}
	s.identity.idTable[s.storeIDString(valueBytes)] = struct{}{}
	return nil
}

func (s *Session) recordIDRef(valueBytes []byte) {
	if s == nil {
		return
	}
	s.identity.idRefs = append(s.identity.idRefs, s.storeIDString(valueBytes))
}

func (s *Session) validateIDRefs() []error {
	if s == nil {
		return nil
	}
	if len(s.identity.idRefs) == 0 {
		return nil
	}
	var errs []error
	for _, ref := range s.identity.idRefs {
		if _, ok := s.identity.idTable[ref]; !ok {
			errs = append(errs, newValidationError(xsderrors.ErrIDRefNotFound, "IDREF value not found"))
		}
	}
	return errs
}

func (s *Session) storeIDString(valueBytes []byte) string {
	if len(valueBytes) == 0 {
		return ""
	}
	if s == nil {
		return unsafeBytesString(valueBytes)
	}
	stable := s.Arena.Alloc(len(valueBytes))
	copy(stable, valueBytes)
	return unsafeBytesString(stable)
}

func unsafeBytesString(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(data), len(data))
}
