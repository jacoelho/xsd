package errors

import (
	"fmt"
	"testing"
)

func TestValidationErrorFormatting(t *testing.T) {
	tests := []struct {
		name string
		want string
		v    Validation
	}{
		{
			name: "message only",
			v:    Validation{Code: "cvc-elt.1", Message: "missing element"},
			want: "[cvc-elt.1] missing element",
		},
		{
			name: "with path",
			v:    Validation{Code: "cvc-elt.1", Message: "missing element", Path: "/root/child"},
			want: "[cvc-elt.1] missing element at /root/child",
		},
		{
			name: "with expected",
			v: Validation{
				Code:     "cvc-elt.1",
				Message:  "unexpected element",
				Expected: []string{"a", "b"},
			},
			want: "[cvc-elt.1] unexpected element (expected: a, b)",
		},
		{
			name: "with actual",
			v: Validation{
				Code:    "cvc-elt.1",
				Message: "unexpected element",
				Actual:  "c",
			},
			want: "[cvc-elt.1] unexpected element (actual: c)",
		},
		{
			name: "with all",
			v: Validation{
				Code:     "cvc-elt.1",
				Message:  "unexpected element",
				Path:     "/root/child",
				Expected: []string{"a"},
				Actual:   "b",
			},
			want: "[cvc-elt.1] unexpected element at /root/child (expected: a) (actual: b)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewValidation(t *testing.T) {
	v := NewValidation(ErrNoRoot, "missing root", "/")
	if v.Code != string(ErrNoRoot) {
		t.Fatalf("Code = %q, want %q", v.Code, ErrNoRoot)
	}
	if v.Message != "missing root" {
		t.Fatalf("Message = %q, want %q", v.Message, "missing root")
	}
	if v.Path != "/" {
		t.Fatalf("Path = %q, want %q", v.Path, "/")
	}
}

func TestNewValidationf(t *testing.T) {
	v := NewValidationf(ErrElementNotDeclared, "/root", "element %s not declared", "child")
	if v.Code != string(ErrElementNotDeclared) {
		t.Fatalf("Code = %q, want %q", v.Code, ErrElementNotDeclared)
	}
	if v.Message != "element child not declared" {
		t.Fatalf("Message = %q, want %q", v.Message, "element child not declared")
	}
	if v.Path != "/root" {
		t.Fatalf("Path = %q, want %q", v.Path, "/root")
	}
}

func TestValidationListError(t *testing.T) {
	one := Validation{Code: "cvc-elt.1", Message: "missing element"}
	two := Validation{Code: "cvc-elt.2", Message: "element is abstract"}

	tests := []struct {
		name string
		want string
		list ValidationList
	}{
		{
			name: "single",
			list: ValidationList{one},
			want: "[cvc-elt.1] missing element",
		},
		{
			name: "multiple",
			list: ValidationList{one, two},
			want: "[cvc-elt.1] missing element (and 1 more)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.list.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAsValidations(t *testing.T) {
	list := ValidationList{
		{Code: "cvc-elt.1", Message: "missing element"},
		{Code: "cvc-elt.2", Message: "element is abstract"},
	}
	wrapped := fmt.Errorf("validation failed: %w", list)

	got, ok := AsValidations(wrapped)
	if !ok {
		t.Fatalf("AsValidations() ok = false, want true")
	}
	if len(got) != 2 {
		t.Fatalf("AsValidations() len = %d, want 2", len(got))
	}
	if got[0].Code != "cvc-elt.1" || got[1].Code != "cvc-elt.2" {
		t.Fatalf("AsValidations() codes = %v, want [cvc-elt.1 cvc-elt.2]", []string{got[0].Code, got[1].Code})
	}
}
