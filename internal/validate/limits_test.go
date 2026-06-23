package validate

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestValidateElementLimits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   ElementLimitInput
		msg  string
	}{
		{
			name: "unlimited",
			in:   ElementLimitInput{Depth: 100, AttributeCount: 100},
		},
		{
			name: "within limits",
			in:   ElementLimitInput{Depth: 2, MaxDepth: 2, AttributeCount: 3, MaxAttributes: 3},
		},
		{
			name: "depth exceeded",
			in: ElementLimitInput{
				Context:        StartContext{Path: "/root", Line: 4, Column: 5},
				Depth:          3,
				MaxDepth:       2,
				AttributeCount: 1,
				MaxAttributes:  1,
			},
			msg: "instance depth limit exceeded",
		},
		{
			name: "attribute count exceeded",
			in: ElementLimitInput{
				Context:        StartContext{Path: "/root", Line: 6, Column: 7},
				Depth:          1,
				MaxDepth:       1,
				AttributeCount: 2,
				MaxAttributes:  1,
			},
			msg: "instance attribute limit exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateElementLimits(tt.in)
			if tt.msg == "" {
				if err != nil {
					t.Fatalf("ValidateElementLimits() error = %v", err)
				}
				return
			}
			requireCode(t, err, xsderrors.CodeValidationLimit)
			if !strings.Contains(err.Error(), tt.msg) {
				t.Fatalf("ValidateElementLimits() error = %v, want %q", err, tt.msg)
			}
		})
	}
}

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
