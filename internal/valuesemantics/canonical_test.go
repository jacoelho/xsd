package valuesemantics

import (
	"testing"

	"github.com/jacoelho/xsd/internal/num"
)

func TestCanonicalizeBoolean(t *testing.T) {
	v, canon, err := CanonicalizeBoolean([]byte("1"))
	if err != nil {
		t.Fatalf("CanonicalizeBoolean() error = %v", err)
	}
	if !v || string(canon) != "true" {
		t.Fatalf("CanonicalizeBoolean() = (%v,%q), want (true,\"true\")", v, canon)
	}
}

func TestCanonicalizeInteger(t *testing.T) {
	val, canon, err := CanonicalizeInteger([]byte("+042"), nil)
	if err != nil {
		t.Fatalf("CanonicalizeInteger() error = %v", err)
	}
	want, perr := num.ParseInt([]byte("42"))
	if perr != nil {
		t.Fatalf("ParseInt(42) error = %v", perr)
	}
	if val.Compare(want) != 0 {
		t.Fatalf("value = %q, want 42", string(val.RenderCanonical(nil)))
	}
	if string(canon) != "42" {
		t.Fatalf("canonical = %q, want \"42\"", canon)
	}
}

func TestCanonicalizeDuration(t *testing.T) {
	_, canon, err := CanonicalizeDuration([]byte("P1Y"))
	if err != nil {
		t.Fatalf("CanonicalizeDuration() error = %v", err)
	}
	if len(canon) == 0 {
		t.Fatal("canonical duration is empty")
	}
}
