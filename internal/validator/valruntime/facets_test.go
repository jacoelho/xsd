package valruntime

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestValidateEmptyProgram(t *testing.T) {
	t.Parallel()

	keyBuf, err := Validate(runtime.ValidatorMeta{Kind: runtime.VString}, Tables{}, []byte("value"), []byte("value"), &State{}, nil)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if keyBuf != nil {
		t.Fatalf("keyBuf = %v, want nil", keyBuf)
	}
}
