package validator

import (
	"errors"
	"fmt"
)

type valueErrorKind uint8

const (
	valueErrInvalid valueErrorKind = iota
	valueErrFacet
)

type valueError struct {
	msg  string
	kind valueErrorKind
}

func (e valueError) Error() string { return e.msg }

func valueErrorf(kind valueErrorKind, format string, args ...any) error {
	return valueError{kind: kind, msg: fmt.Sprintf(format, args...)}
}

func valueErrorMsg(kind valueErrorKind, msg string) error {
	return valueError{kind: kind, msg: msg}
}

func valueErrorKindOf(err error) (valueErrorKind, bool) {
	if err == nil {
		return 0, false
	}
	var ve valueError
	if errors.As(err, &ve) {
		return ve.kind, true
	}
	return 0, false
}
