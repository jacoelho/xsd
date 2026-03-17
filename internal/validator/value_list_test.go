package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestValidateCollapsedFloatList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "finite", input: "1 .25 -0.5 1.5E2", want: true},
		{name: "signed finite", input: "+1 -.25 -1.0e+2", want: true},
		{name: "special literals", input: "INF -INF NaN", want: true},
		{name: "plus inf invalid", input: "+INF", want: false},
		{name: "dangling sign", input: "-", want: false},
		{name: "bad exponent", input: "1e", want: false},
		{name: "bad literal terminator", input: "INFx", want: false},
		{name: "bad numeric terminator", input: "1x", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCollapsedFloatList([]byte(tt.input), runtime.VDouble)
			if (err == nil) != tt.want {
				t.Fatalf("validateCollapsedFloatList(%q) err = %v, want success=%v", tt.input, err, tt.want)
			}
		})
	}
}
