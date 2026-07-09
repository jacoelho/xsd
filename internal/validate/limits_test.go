package validate

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateTextLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   TextLimitInput
		want bool
	}{
		{
			name: "unlimited",
			in:   TextLimitInput{CurrentBytes: 10, AppendBytes: 10},
		},
		{
			name: "within limit",
			in:   TextLimitInput{CurrentBytes: 2, AppendBytes: 3, MaxBytes: 5},
		},
		{
			name: "exceeds limit",
			in: TextLimitInput{
				Context:      StartContext{Path: "/root", Line: 8, Column: 9},
				CurrentBytes: 3,
				AppendBytes:  3,
				MaxBytes:     5,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateTextLimit(tt.in)
			if !tt.want {
				if err != nil {
					t.Fatalf("ValidateTextLimit() error = %v", err)
				}
				return
			}
			requireCode(t, err, xsderrors.CodeValidationLimit)
			if !strings.Contains(err.Error(), "instance text byte limit exceeded") {
				t.Fatalf("ValidateTextLimit() error = %v", err)
			}
		})
	}
}
