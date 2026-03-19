package valruntime

import (
	"bytes"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator/diag"
)

func TestDeriveAnyURI(t *testing.T) {
	t.Parallel()

	kind, keyBytes, err := Derive(runtime.VAnyURI, []byte("https://example.com/a"), make([]byte, 0, 32))
	if err != nil {
		t.Fatalf("Derive() error = %v", err)
	}
	if kind != runtime.VKString {
		t.Fatalf("kind = %v, want %v", kind, runtime.VKString)
	}
	want := runtime.StringKeyBytes(nil, 1, []byte("https://example.com/a"))
	if !bytes.Equal(keyBytes, want) {
		t.Fatalf("key = %v, want %v", keyBytes, want)
	}
}

func TestDeriveHexBinary(t *testing.T) {
	t.Parallel()

	kind, keyBytes, err := Derive(runtime.VHexBinary, []byte("0A"), make([]byte, 0, 8))
	if err != nil {
		t.Fatalf("Derive() error = %v", err)
	}
	if kind != runtime.VKBinary {
		t.Fatalf("kind = %v, want %v", kind, runtime.VKBinary)
	}
	want := runtime.BinaryKeyBytes(nil, 0, []byte{0x0A})
	if !bytes.Equal(keyBytes, want) {
		t.Fatalf("key = %v, want %v", keyBytes, want)
	}
}

func TestDeriveInvalidQName(t *testing.T) {
	t.Parallel()

	_, _, err := Derive(runtime.VQName, []byte("not-canonical"), nil)
	if err == nil {
		t.Fatal("Derive() error = nil, want datatype invalid")
	}
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrDatatypeInvalid {
		t.Fatalf("Derive() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrDatatypeInvalid)
	}
}

func TestDeriveInvalidBoolean(t *testing.T) {
	t.Parallel()

	_, _, err := Derive(runtime.VBoolean, []byte("maybe"), nil)
	if err == nil {
		t.Fatal("Derive() error = nil, want datatype invalid")
	}
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrDatatypeInvalid {
		t.Fatalf("Derive() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrDatatypeInvalid)
	}
}

func TestDeriveUnsupportedKind(t *testing.T) {
	t.Parallel()

	_, _, err := Derive(runtime.ValidatorKind(255), []byte("x"), nil)
	if err == nil {
		t.Fatal("Derive() error = nil, want datatype invalid")
	}
	if code, ok := diag.Info(err); !ok || code != xsderrors.ErrDatatypeInvalid {
		t.Fatalf("Derive() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrDatatypeInvalid)
	}
}
