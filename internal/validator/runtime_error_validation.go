package validator

import xsderrors "github.com/jacoelho/xsd/errors"

func (s *Session) newValidation(code xsderrors.ErrorCode, msg, path string, line, column int) xsderrors.Validation {
	return xsderrors.Validation{
		Code:     string(code),
		Message:  msg,
		Document: s.documentURI,
		Path:     path,
		Line:     line,
		Column:   column,
	}
}
