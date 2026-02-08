package validator

import (
	"github.com/jacoelho/xsd/internal/runtime"
)

var errInvalidIntegerLexical = valueError{kind: valueErrInvalid, msg: "invalid integer"}

func unionMemberLexicallyImpossible(kind runtime.ValidatorKind, lexical []byte) bool {
	switch kind {
	case runtime.VInteger:
		return !isIntegerLexical(lexical)
	default:
		return false
	}
}

func unionMemberLexicalMismatch(kind runtime.ValidatorKind) error {
	switch kind {
	case runtime.VInteger:
		return errInvalidIntegerLexical
	default:
		return nil
	}
}

func isIntegerLexical(lexical []byte) bool {
	if len(lexical) == 0 {
		return false
	}
	start := 0
	if lexical[0] == '+' || lexical[0] == '-' {
		start = 1
	}
	if start >= len(lexical) {
		return false
	}
	for _, b := range lexical[start:] {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}
