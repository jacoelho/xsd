package validatorgen

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestUnionValidatorMismatchReturnsError(t *testing.T) {
	comp := newCompiler(nil)
	_, err := comp.addUnionValidator(runtime.WS_Preserve, runtime.FacetProgramRef{}, []runtime.ValidatorID{1}, nil, "U", 0)
	if err == nil {
		t.Fatalf("expected union member mismatch error")
	}
	if !strings.Contains(err.Error(), "validators=1") || !strings.Contains(err.Error(), "memberTypes=0") {
		t.Fatalf("unexpected error: %v", err)
	}
}
