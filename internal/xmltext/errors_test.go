package xmltext

import (
	"errors"
	"strings"
	"testing"
)

func TestSyntaxErrorNil(t *testing.T) {
	var err *SyntaxError
	if got := err.Error(); got != "<nil>" {
		t.Fatalf("SyntaxError.Error = %q, want <nil>", got)
	}
}

func TestSyntaxErrorFormatting(t *testing.T) {
	syntax := &SyntaxError{Line: 2, Column: 3, Err: errInvalidToken}
	if got := syntax.Error(); !strings.Contains(got, "line 2") {
		t.Fatalf("Error = %q, want line 2", got)
	}
	syntax = &SyntaxError{Offset: 10, Err: errInvalidToken}
	if got := syntax.Error(); !strings.Contains(got, "offset 10") {
		t.Fatalf("Error = %q, want offset 10", got)
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
