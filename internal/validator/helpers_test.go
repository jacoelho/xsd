package validator

import "github.com/jacoelho/xsd/internal/xsderrors"

func hasViolationCode(violations []xsderrors.Validation, code xsderrors.ErrorCode) bool {
	for _, v := range violations {
		if v.Code == code {
			return true
		}
	}
	return false
}
