package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestCanonicalizeAtomicDecimalSetsCanonicalMeasureAndKey(t *testing.T) {
	t.Parallel()

	var sess Session
	metrics := &ValueMetrics{}

	canonical, err := sess.canonicalizeAtomic(
		runtime.ValidatorMeta{Kind: runtime.VDecimal},
		[]byte("12.34"),
		true,
		metrics,
	)
	if err != nil {
		t.Fatalf("canonicalizeAtomic() error = %v", err)
	}
	if got := string(canonical); got != "12.34" {
		t.Fatalf("canonicalizeAtomic() canonical = %q, want %q", got, "12.34")
	}

	kind, key, ok := metrics.State.Key()
	if !ok {
		t.Fatal("canonicalizeAtomic() should set key state")
	}
	if kind != runtime.VKDecimal {
		t.Fatalf("canonicalizeAtomic() key kind = %v, want %v", kind, runtime.VKDecimal)
	}
	if len(key) == 0 {
		t.Fatal("canonicalizeAtomic() key bytes should be non-empty")
	}

	dec, err := metrics.Cache.Decimal(canonical)
	if err != nil {
		t.Fatalf("metrics.Cache.Decimal() error = %v", err)
	}
	if got := dec.RenderCanonical(nil); string(got) != "12.34" {
		t.Fatalf("metrics.Cache.Decimal() canonical = %q, want %q", got, "12.34")
	}
}

func TestCanonicalizeAtomicStringRequiresKindResolver(t *testing.T) {
	t.Parallel()

	var sess Session
	sess.rt = newRuntimeSchema(t)

	_, err := sess.canonicalizeAtomic(
		runtime.ValidatorMeta{Kind: runtime.VString},
		[]byte("en"),
		false,
		nil,
	)
	if err == nil || err.Error() != "string validator out of range" {
		t.Fatalf("canonicalizeAtomic() error = %v, want string validator out of range", err)
	}
}
