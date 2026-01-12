package xmltext

import "testing"

func TestValueCloneEmpty(t *testing.T) {
	var value Value
	if value.Clone() != nil {
		t.Fatalf("Value.Clone = %v, want nil", value.Clone())
	}
}
