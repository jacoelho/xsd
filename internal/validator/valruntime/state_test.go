package valruntime

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestStateAccessors(t *testing.T) {
	var state State

	if cache := state.MeasureCache(); cache == nil {
		t.Fatal("MeasureCache() = nil, want cache")
	}
	if got := state.ResultState(); got == nil {
		t.Fatal("ResultState() = nil, want result state")
	}

	state.Result.SetKey(runtime.VKString, []byte("k"))
	kind, key, ok := state.ResultState().Key()
	if !ok {
		t.Fatal("ResultState().Key() reported no key")
	}
	if kind != runtime.VKString {
		t.Fatalf("ResultState().Key() kind = %v, want %v", kind, runtime.VKString)
	}
	if string(key) != "k" {
		t.Fatalf("ResultState().Key() key = %q, want %q", key, "k")
	}
}

func TestNilStateAccessors(t *testing.T) {
	var state *State
	if state.MeasureCache() != nil {
		t.Fatal("MeasureCache() on nil state = non-nil, want nil")
	}
	if state.ResultState() != nil {
		t.Fatal("ResultState() on nil state = non-nil, want nil")
	}
}
