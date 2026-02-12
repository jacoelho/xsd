package validator

import xsderrors "github.com/jacoelho/xsd/errors"

type validationError struct {
	code     xsderrors.ErrorCode
	msg      string
	actual   string
	expected []string
}

func (e validationError) Error() string {
	return e.msg
}

type validationDetails struct {
	code     xsderrors.ErrorCode
	msg      string
	actual   string
	expected []string
	ok       bool
}
