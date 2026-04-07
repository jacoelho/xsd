package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestValidateEmptyProgram(t *testing.T) {
	t.Parallel()

	keyBuf, err := validateRuntimeFacets(
		runtime.ValidatorMeta{Kind: runtime.VString},
		nil,
		nil,
		runtime.EnumTable{},
		runtime.ValueBlob{},
		[]byte("value"),
		[]byte("value"),
		&ValueMetrics{},
		nil,
	)
	if err != nil {
		t.Fatalf("validateRuntimeFacets() error = %v", err)
	}
	if keyBuf != nil {
		t.Fatalf("keyBuf = %v, want nil", keyBuf)
	}
}
