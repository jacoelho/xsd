package xmlstream

import "testing"

func TestUnsafeStringEmpty(t *testing.T) {
	if got := unsafeString(nil); got != "" {
		t.Fatalf("unsafeString(nil) = %q, want empty", got)
	}
	if got := unsafeString([]byte{}); got != "" {
		t.Fatalf("unsafeString(empty) = %q, want empty", got)
	}
}
