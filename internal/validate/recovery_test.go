package validate

import (
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestRecoverableError(t *testing.T) {
	t.Parallel()

	err := xsderrors.Validation(xsderrors.CodeValidationElement, 1, 2, "/root", "unexpected element")
	if !RecoverableError(fmt.Errorf("wrapped: %w", err)) {
		t.Fatal("RecoverableError(wrapped validation element) = false, want true")
	}
	if RecoverableError(xsderrors.Validation(xsderrors.CodeValidationXML, 1, 2, "/", "bad XML")) {
		t.Fatal("RecoverableError(validation XML) = true, want false")
	}
	if RecoverableError(xsderrors.InternalInvariant("broken state")) {
		t.Fatal("RecoverableError(internal invariant) = true, want false")
	}
	if RecoverableError(fmt.Errorf("plain error")) {
		t.Fatal("RecoverableError(plain error) = true, want false")
	}
}

func TestRecoverableValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category xsderrors.Category
		code     xsderrors.Code
		want     bool
	}{
		{
			name:     "validation element",
			category: xsderrors.CategoryValidation,
			code:     xsderrors.CodeValidationElement,
			want:     true,
		},
		{
			name:     "validation xml",
			category: xsderrors.CategoryValidation,
			code:     xsderrors.CodeValidationXML,
		},
		{
			name:     "validation limit",
			category: xsderrors.CategoryValidation,
			code:     xsderrors.CodeValidationLimit,
		},
		{
			name:     "unsupported",
			category: xsderrors.CategoryUnsupported,
			code:     xsderrors.CodeUnsupportedExternal,
		},
		{
			name:     "internal",
			category: xsderrors.CategoryInternal,
			code:     xsderrors.CodeInternalInvariant,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := RecoverableValidation(tt.category, tt.code)
			if got != tt.want {
				t.Fatalf("RecoverableValidation(%q, %q) = %v, want %v", tt.category, tt.code, got, tt.want)
			}
		})
	}
}

func TestRecoveryLimitReached(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		count int
		limit int
		want  bool
	}{
		{name: "unlimited zero", count: 10},
		{name: "unlimited negative", count: 10, limit: -1},
		{name: "below limit", count: 1, limit: 2},
		{name: "at limit", count: 2, limit: 2, want: true},
		{name: "above limit", count: 3, limit: 2, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := RecoveryLimitReached(tt.count, tt.limit)
			if got != tt.want {
				t.Fatalf("RecoveryLimitReached(%d, %d) = %v, want %v", tt.count, tt.limit, got, tt.want)
			}
		})
	}
}
