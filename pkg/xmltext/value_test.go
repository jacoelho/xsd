package xmltext

import "testing"

func TestValueCloneEmpty(t *testing.T) {
	var value Value
	if value.Clone() != nil {
		t.Fatalf("Value.Clone = %v, want nil", value.Clone())
	}
}

func TestValueHelpers(t *testing.T) {
	value := Value("<a><b/></a>")
	if !value.IsValid() {
		t.Fatalf("Value.IsValid = false, want true")
	}
	invalid := Value("<a>")
	if invalid.IsValid() {
		t.Fatalf("Value.IsValid = true, want false")
	}
	if got := value.Clone(); string(got) != string(value) {
		t.Fatalf("Clone = %q, want %q", got, value)
	}
}
