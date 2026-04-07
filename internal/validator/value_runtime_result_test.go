package validator

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestStateKey(t *testing.T) {
	t.Parallel()

	var state ValueState
	if state.HasKey() {
		t.Fatal("HasKey() = true, want false")
	}

	state.SetKey(runtime.VKString, []byte("abc"))
	kind, key, ok := state.Key()
	if !ok {
		t.Fatal("Key() ok = false, want true")
	}
	if kind != runtime.VKString {
		t.Fatalf("kind = %v, want %v", kind, runtime.VKString)
	}
	if !bytes.Equal(key, []byte("abc")) {
		t.Fatalf("key = %q, want %q", key, "abc")
	}
}

func TestStateActualAndFlags(t *testing.T) {
	t.Parallel()

	var state ValueState
	state.SetActual(7, 9)
	state.SetPatternChecked(true)
	state.SetEnumChecked(true)

	typeID, validatorID := state.Actual()
	if typeID != 7 || validatorID != 9 {
		t.Fatalf("Actual() = (%d, %d), want (7, 9)", typeID, validatorID)
	}
	if !state.PatternChecked() {
		t.Fatal("PatternChecked() = false, want true")
	}
	if !state.EnumChecked() {
		t.Fatal("EnumChecked() = false, want true")
	}
}
