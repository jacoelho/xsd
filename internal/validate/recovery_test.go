package validate

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

// TestAssessmentFailure pins which recovered diagnostics invalidate the
// current element: conclusive validation evidence other than identity, which
// belongs to the constraint scope.
func TestAssessmentFailure(t *testing.T) {
	t.Parallel()

	validation := func(code xsderrors.Code, cause error) error {
		return xsderrors.WithLocation("/root", 1, 2, xsderrors.Validation(code, "diagnostic", cause))
	}
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "root", err: validation(xsderrors.CodeValidationRoot, nil), want: true},
		{name: "element", err: validation(xsderrors.CodeValidationElement, nil), want: true},
		{name: "attribute", err: validation(xsderrors.CodeValidationAttribute, nil), want: true},
		{name: "text", err: validation(xsderrors.CodeValidationText, nil), want: true},
		{name: "type", err: validation(xsderrors.CodeValidationType, nil), want: true},
		{name: "facet", err: validation(xsderrors.CodeValidationFacet, nil), want: true},
		{name: "content", err: validation(xsderrors.CodeValidationContent, nil), want: true},
		{name: "nil", err: validation(xsderrors.CodeValidationNil, nil), want: true},
		{name: "wrapped element", err: fmt.Errorf("wrapped: %w", validation(xsderrors.CodeValidationElement, nil)), want: true},
		{name: "identity", err: validation(xsderrors.CodeValidationIdentity, nil)},
		{name: "limit", err: validation(xsderrors.CodeValidationLimit, nil)},
		{name: "xml", err: validation(xsderrors.CodeValidationXML, nil)},
		{name: "option", err: validation(xsderrors.CodeValidationOption, nil)},
		{name: "session", err: validation(xsderrors.CodeValidationSession, nil)},
		{name: "facet caused by limit", err: validation(xsderrors.CodeValidationFacet, validation(xsderrors.CodeValidationLimit, nil))},
		{name: "internal", err: xsderrors.InternalInvariant("broken state")},
		{name: "plain", err: errors.New("plain error")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := assessmentFailure(tt.err); got != tt.want {
				t.Fatalf("assessmentFailure(%v) = %v, want %v", tt.err, got, tt.want)
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
