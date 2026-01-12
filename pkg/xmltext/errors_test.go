package xmltext

import (
	"errors"
	"testing"
)

func TestSyntaxErrorNil(t *testing.T) {
	var err *SyntaxError
	if got := err.Error(); got != "<nil>" {
		t.Fatalf("SyntaxError.Error = %q, want <nil>", got)
	}
}

func TestSyntaxErrorUnwrap(t *testing.T) {
	var err *SyntaxError
	if err.Unwrap() != nil {
		t.Fatalf("nil SyntaxError Unwrap = %v, want nil", err.Unwrap())
	}
	syntax := &SyntaxError{Err: errInvalidToken}
	if !errors.Is(syntax, errInvalidToken) {
		t.Fatalf("Unwrap = %v, want %v", syntax.Unwrap(), errInvalidToken)
	}
}
