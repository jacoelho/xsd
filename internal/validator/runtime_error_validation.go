package validator

import xsderrors "github.com/jacoelho/xsd/internal/xsderrors"

func (s *Session) newValidation(code xsderrors.ErrorCode, msg, path string, line, column int) xsderrors.Validation {
	return xsderrors.Validation{
		Code:     code,
		Message:  msg,
		Document: s.io.documentURI,
		Path:     path,
		Line:     line,
		Column:   column,
	}
}
