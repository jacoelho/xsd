package num

import (
	"bytes"
	"math"
	"testing"
)

func TestFromInt64MinInt64(t *testing.T) {
	v := FromInt64(math.MinInt64)
	canonical := v.RenderCanonical(nil)
	if string(canonical) != "-9223372036854775808" {
		t.Fatalf("canonical = %q, want %q", canonical, "-9223372036854775808")
	}
	parsed, err := ParseInt(canonical)
	if err != nil {
		t.Fatalf("parse canonical: %v", err)
	}
	if parsed.Sign != v.Sign || !bytes.Equal(parsed.Digits, v.Digits) {
		t.Fatalf("round-trip mismatch: parsed=%v, want=%v", parsed, v)
	}
}
