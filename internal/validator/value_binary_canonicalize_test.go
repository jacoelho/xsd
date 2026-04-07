package validator

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestCanonicalizeHexBinarySetsLengthAndKey(t *testing.T) {
	t.Parallel()

	var sess Session
	metrics := &ValueMetrics{}

	canonical, err := sess.canonicalizeHexBinary([]byte("0a0B"), true, metrics)
	if err != nil {
		t.Fatalf("canonicalizeHexBinary() error = %v", err)
	}
	if got := string(canonical); got != "0A0B" {
		t.Fatalf("canonicalizeHexBinary() canonical = %q, want %q", got, "0A0B")
	}
	length, err := metrics.Cache.Length(runtime.VHexBinary, canonical)
	if err != nil {
		t.Fatalf("metrics.Cache.Length() error = %v", err)
	}
	if length != 2 {
		t.Fatalf("metrics.Cache.Length() = %d, want 2", length)
	}

	keyKind, keyBytes, ok := metrics.State.Key()
	if !ok {
		t.Fatal("canonicalizeHexBinary() should set key state")
	}
	if keyKind != runtime.VKBinary {
		t.Fatalf("canonicalizeHexBinary() key kind = %v, want %v", keyKind, runtime.VKBinary)
	}
	wantKey := runtime.BinaryKeyBytes(nil, 0, []byte{0x0a, 0x0b})
	if !bytes.Equal(keyBytes, wantKey) {
		t.Fatalf("canonicalizeHexBinary() key = %v, want %v", keyBytes, wantKey)
	}
}

func TestCanonicalizeBase64BinarySetsLengthAndKey(t *testing.T) {
	t.Parallel()

	var sess Session
	metrics := &ValueMetrics{}

	canonical, err := sess.canonicalizeBase64Binary([]byte("YWI="), true, metrics)
	if err != nil {
		t.Fatalf("canonicalizeBase64Binary() error = %v", err)
	}
	if got := string(canonical); got != "YWI=" {
		t.Fatalf("canonicalizeBase64Binary() canonical = %q, want %q", got, "YWI=")
	}
	length, err := metrics.Cache.Length(runtime.VBase64Binary, canonical)
	if err != nil {
		t.Fatalf("metrics.Cache.Length() error = %v", err)
	}
	if length != 2 {
		t.Fatalf("metrics.Cache.Length() = %d, want 2", length)
	}

	keyKind, keyBytes, ok := metrics.State.Key()
	if !ok {
		t.Fatal("canonicalizeBase64Binary() should set key state")
	}
	if keyKind != runtime.VKBinary {
		t.Fatalf("canonicalizeBase64Binary() key kind = %v, want %v", keyKind, runtime.VKBinary)
	}
	wantKey := runtime.BinaryKeyBytes(nil, 1, []byte("ab"))
	if !bytes.Equal(keyBytes, wantKey) {
		t.Fatalf("canonicalizeBase64Binary() key = %v, want %v", keyBytes, wantKey)
	}
}

func TestCanonicalizeHexBinaryNilMetricsNoPanic(t *testing.T) {
	t.Parallel()

	var sess Session
	canonical, err := sess.canonicalizeHexBinary([]byte("0A"), false, nil)
	if err != nil {
		t.Fatalf("canonicalizeHexBinary() error = %v", err)
	}
	if got := string(canonical); got != "0A" {
		t.Fatalf("canonicalizeHexBinary() canonical = %q, want %q", got, "0A")
	}
}
