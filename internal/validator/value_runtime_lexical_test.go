package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestParseTemporalUnsupportedKind(t *testing.T) {
	t.Parallel()

	_, err := parseTemporal(runtime.VString, []byte("x"))
	if err == nil {
		t.Fatal("expected unsupported temporal kind error")
	}
	if err.Error() != "unsupported temporal kind 0" {
		t.Fatalf("error = %q, want %q", err.Error(), "unsupported temporal kind 0")
	}
}
