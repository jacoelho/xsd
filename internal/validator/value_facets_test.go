package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestDigitCountsInvalidInteger(t *testing.T) {
	if _, _, err := digitCounts(runtime.VInteger, []byte("not-int"), nil); err == nil {
		t.Fatalf("expected error for invalid integer")
	}
}
