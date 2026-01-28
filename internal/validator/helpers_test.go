package validator

import "github.com/jacoelho/xsd/errors"

func hasViolationCode(violations []errors.Validation, code errors.ErrorCode) bool {
	for _, v := range violations {
		if v.Code == string(code) {
			return true
		}
	}
	return false
}
