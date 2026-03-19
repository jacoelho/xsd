package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
)

type validationDetails struct {
	code     xsderrors.ErrorCode
	msg      string
	actual   string
	expected []string
	ok       bool
}
