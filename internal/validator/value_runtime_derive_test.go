package validator

import (
	"bytes"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	xsderrors "github.com/jacoelho/xsd/internal/xsderrors"
)

func TestDeriveAnyURI(t *testing.T) {
	t.Parallel()

	kind, keyBytes, err := derive(runtime.VAnyURI, []byte("https://example.com/a"), make([]byte, 0, 32))
	if err != nil {
		t.Fatalf("derive() error = %v", err)
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

	kind, keyBytes, err := derive(runtime.VHexBinary, []byte("0A"), make([]byte, 0, 8))
	if err != nil {
		t.Fatalf("derive() error = %v", err)
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

	_, _, err := derive(runtime.VQName, []byte("not-canonical"), nil)
	if err == nil {
		t.Fatal("derive() error = nil, want datatype invalid")
	}
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrDatatypeInvalid {
		t.Fatalf("derive() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrDatatypeInvalid)
	}
}

func TestDeriveInvalidBoolean(t *testing.T) {
	t.Parallel()

	_, _, err := derive(runtime.VBoolean, []byte("maybe"), nil)
	if err == nil {
		t.Fatal("derive() error = nil, want datatype invalid")
	}
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrDatatypeInvalid {
		t.Fatalf("derive() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrDatatypeInvalid)
	}
}

func TestDeriveUnsupportedKind(t *testing.T) {
	t.Parallel()

	_, _, err := derive(runtime.ValidatorKind(255), []byte("x"), nil)
	if err == nil {
		t.Fatal("derive() error = nil, want datatype invalid")
	}
	if code, ok := xsderrors.Info(err); !ok || code != xsderrors.ErrDatatypeInvalid {
		t.Fatalf("derive() code = %v, ok=%v, want %v", code, ok, xsderrors.ErrDatatypeInvalid)
	}
}
