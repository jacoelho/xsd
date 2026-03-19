package valruntime

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

type mapResolver map[string]string

func (r mapResolver) ResolvePrefix(prefix []byte) ([]byte, bool) {
	if v, ok := r[string(prefix)]; ok {
		return []byte(v), true
	}
	return nil, false
}

func TestAtomicDecimalSetsCanonicalMeasureAndKey(t *testing.T) {
	var cache Cache
	result, bufs, err := Atomic(
		runtime.ValidatorMeta{Kind: runtime.VDecimal},
		[]byte("12.34"),
		true,
		KindLoader{},
		CanonicalBuffers{
			Buf1: make([]byte, 0, 32),
			Buf2: make([]byte, 0, 32),
			Key:  make([]byte, 0, 32),
		},
		&cache,
	)
	if err != nil {
		t.Fatalf("Atomic() error = %v", err)
	}
	if got := string(result.Canonical); got != "12.34" {
		t.Fatalf("Atomic() canonical = %q, want %q", got, "12.34")
	}
	if !result.HasKey() {
		t.Fatal("Atomic() should derive a key")
	}
	if len(bufs.Key) == 0 {
		t.Fatal("Atomic() should reuse key scratch")
	}
	dec, err := cache.Decimal(result.Canonical)
	if err != nil {
		t.Fatalf("cache.Decimal() error = %v", err)
	}
	if got := dec.RenderCanonical(nil); string(got) != "12.34" {
		t.Fatalf("cache.Decimal() canonical = %q, want %q", got, "12.34")
	}
}

func TestAtomicStringRequiresKindLoader(t *testing.T) {
	_, _, err := Atomic(
		runtime.ValidatorMeta{Kind: runtime.VString},
		[]byte("en"),
		false,
		KindLoader{},
		CanonicalBuffers{},
		nil,
	)
	if err == nil || err.Error() != "string validator out of range" {
		t.Fatalf("Atomic() error = %v, want string validator out of range", err)
	}
}

func TestTemporalCanonicalizesAndKeys(t *testing.T) {
	result, _, err := Temporal(
		runtime.VDate,
		[]byte("2001-10-26"),
		true,
		CanonicalBuffers{Key: make([]byte, 0, 32)},
	)
	if err != nil {
		t.Fatalf("Temporal() error = %v", err)
	}
	if got := string(result.Canonical); got != "2001-10-26" {
		t.Fatalf("Temporal() canonical = %q, want %q", got, "2001-10-26")
	}
	if !result.HasKey() {
		t.Fatal("Temporal() should derive a key")
	}
}

func TestHexBinaryCanonicalizesAndMeasuresLength(t *testing.T) {
	var cache Cache
	result, _, err := HexBinary(
		[]byte("0a0B"),
		true,
		CanonicalBuffers{
			Value: make([]byte, 0, 8),
			Key:   make([]byte, 0, 8),
		},
		&cache,
	)
	if err != nil {
		t.Fatalf("HexBinary() error = %v", err)
	}
	if got := string(result.Canonical); got != "0A0B" {
		t.Fatalf("HexBinary() canonical = %q, want %q", got, "0A0B")
	}
	length, err := cache.Length(runtime.VHexBinary, result.Canonical)
	if err != nil {
		t.Fatalf("cache.Length() error = %v", err)
	}
	if length != 2 {
		t.Fatalf("cache.Length() = %d, want 2", length)
	}
	if !result.HasKey() {
		t.Fatal("HexBinary() should derive a key")
	}
}

func TestQNameCanonicalizesAndDerivesKey(t *testing.T) {
	result, bufs, err := QName(
		runtime.VQName,
		[]byte("p:local"),
		mapResolver{"p": "urn:test"},
		true,
		nil,
		CanonicalBuffers{
			Value: make([]byte, 0, 32),
			Key:   make([]byte, 0, 32),
		},
	)
	if err != nil {
		t.Fatalf("QName() error = %v", err)
	}
	if got := string(result.Canonical); got != "urn:test\x00local" {
		t.Fatalf("QName() canonical = %q, want %q", got, "urn:test\x00local")
	}
	if !result.HasKey() {
		t.Fatal("QName() should derive a key")
	}
	if len(bufs.Value) == 0 || len(bufs.Key) == 0 {
		t.Fatal("QName() should reuse caller-owned buffers")
	}
}

func TestQNameNotationRequiresDeclaredCallback(t *testing.T) {
	_, _, err := QName(
		runtime.VNotation,
		[]byte("p:local"),
		mapResolver{"p": "urn:test"},
		false,
		func([]byte) bool { return false },
		CanonicalBuffers{Value: make([]byte, 0, 32)},
	)
	if err == nil || err.Error() != "notation not declared" {
		t.Fatalf("QName() error = %v, want notation not declared", err)
	}
}

func TestSplitQNameRejectsMissingSeparator(t *testing.T) {
	_, _, err := SplitQName([]byte("not-canonical"))
	if err == nil || err.Error() != "invalid canonical QName" {
		t.Fatalf("SplitQName() error = %v, want invalid canonical QName", err)
	}
}
