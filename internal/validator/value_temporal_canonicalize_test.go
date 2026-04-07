package validator

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestCanonicalizeTemporalCanonicalAndKey(t *testing.T) {
	t.Parallel()

	var sess Session
	metrics := &ValueMetrics{}

	canon, err := sess.canonicalizeTemporal(runtime.VDate, []byte("2001-10-26"), true, metrics)
	if err != nil {
		t.Fatalf("canonicalizeTemporal() error = %v", err)
	}
	if got := string(canon); got != "2001-10-26" {
		t.Fatalf("canonicalizeTemporal() canonical = %q, want %q", got, "2001-10-26")
	}

	keyKind, keyBytes, ok := metrics.State.Key()
	if !ok {
		t.Fatal("canonicalizeTemporal() should set key state")
	}
	if keyKind != runtime.VKDateTime {
		t.Fatalf("canonicalizeTemporal() key kind = %v, want %v", keyKind, runtime.VKDateTime)
	}
	if len(keyBytes) == 0 {
		t.Fatal("canonicalizeTemporal() key bytes should be non-empty")
	}
	if !bytes.Equal(sess.keyTmp, keyBytes) {
		t.Fatalf("canonicalizeTemporal() session key scratch = %v, want %v", sess.keyTmp, keyBytes)
	}
}

func TestCanonicalizeTemporalUnsupportedKind(t *testing.T) {
	t.Parallel()

	var sess Session
	_, err := sess.canonicalizeTemporal(runtime.VAnyURI, []byte("https://example.com"), false, nil)
	if err == nil {
		t.Fatal("canonicalizeTemporal() expected error for non-temporal kind")
	}
}
