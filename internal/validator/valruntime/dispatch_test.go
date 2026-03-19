package valruntime

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestDispatchCanonical(t *testing.T) {
	t.Parallel()

	called := ""
	got, err := DispatchCanonical(
		runtime.VQName,
		CanonicalCallbacks[string]{
			Atomic: func() (string, error) {
				t.Fatal("Atomic should not be called")
				return "", nil
			},
			Temporal: func() (string, error) {
				t.Fatal("Temporal should not be called")
				return "", nil
			},
			AnyURI: func() (string, error) {
				t.Fatal("AnyURI should not be called")
				return "", nil
			},
			QName: func() (string, error) {
				called = "qname"
				return "ok", nil
			},
			HexBinary: func() (string, error) {
				t.Fatal("HexBinary should not be called")
				return "", nil
			},
			Base64Binary: func() (string, error) {
				t.Fatal("Base64Binary should not be called")
				return "", nil
			},
			List: func() (string, error) {
				t.Fatal("List should not be called")
				return "", nil
			},
			Union: func() (string, error) {
				t.Fatal("Union should not be called")
				return "", nil
			},
			Invalid: func(runtime.ValidatorKind) error {
				t.Fatal("Invalid should not be called")
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("DispatchCanonical() error = %v", err)
	}
	if got != "ok" {
		t.Fatalf("DispatchCanonical() = %q, want ok", got)
	}
	if called != "qname" {
		t.Fatalf("called = %q, want qname", called)
	}
}

func TestDispatchCanonicalInvalid(t *testing.T) {
	t.Parallel()

	want := errors.New("invalid kind")
	_, err := DispatchCanonical(
		runtime.ValidatorKind(255),
		CanonicalCallbacks[string]{
			Atomic:       func() (string, error) { t.Fatal("Atomic should not be called"); return "", nil },
			Temporal:     func() (string, error) { t.Fatal("Temporal should not be called"); return "", nil },
			AnyURI:       func() (string, error) { t.Fatal("AnyURI should not be called"); return "", nil },
			QName:        func() (string, error) { t.Fatal("QName should not be called"); return "", nil },
			HexBinary:    func() (string, error) { t.Fatal("HexBinary should not be called"); return "", nil },
			Base64Binary: func() (string, error) { t.Fatal("Base64Binary should not be called"); return "", nil },
			List:         func() (string, error) { t.Fatal("List should not be called"); return "", nil },
			Union:        func() (string, error) { t.Fatal("Union should not be called"); return "", nil },
			Invalid: func(runtime.ValidatorKind) error {
				return want
			},
		},
	)
	if !errors.Is(err, want) {
		t.Fatalf("DispatchCanonical() error = %v, want %v", err, want)
	}
}

func TestDispatchNoCanonical(t *testing.T) {
	t.Parallel()

	called := ""
	got, err := DispatchNoCanonical(
		runtime.VBase64Binary,
		NoCanonicalCallbacks[string]{
			Atomic: func() error {
				t.Fatal("Atomic should not be called")
				return nil
			},
			Temporal: func() error {
				t.Fatal("Temporal should not be called")
				return nil
			},
			AnyURI: func() error {
				t.Fatal("AnyURI should not be called")
				return nil
			},
			HexBinary: func() error {
				t.Fatal("HexBinary should not be called")
				return nil
			},
			Base64Binary: func() error {
				called = "base64"
				return nil
			},
			List: func() error {
				t.Fatal("List should not be called")
				return nil
			},
			Result: func() string {
				return "ok"
			},
			Invalid: func(runtime.ValidatorKind) error {
				t.Fatal("Invalid should not be called")
				return nil
			},
		},
	)
	if err != nil {
		t.Fatalf("DispatchNoCanonical() error = %v", err)
	}
	if got != "ok" {
		t.Fatalf("DispatchNoCanonical() = %q, want ok", got)
	}
	if called != "base64" {
		t.Fatalf("called = %q, want base64", called)
	}
}

func TestDispatchNoCanonicalInvalid(t *testing.T) {
	t.Parallel()

	want := errors.New("invalid kind")
	_, err := DispatchNoCanonical(
		runtime.VQName,
		NoCanonicalCallbacks[string]{
			Atomic:       func() error { t.Fatal("Atomic should not be called"); return nil },
			Temporal:     func() error { t.Fatal("Temporal should not be called"); return nil },
			AnyURI:       func() error { t.Fatal("AnyURI should not be called"); return nil },
			HexBinary:    func() error { t.Fatal("HexBinary should not be called"); return nil },
			Base64Binary: func() error { t.Fatal("Base64Binary should not be called"); return nil },
			List:         func() error { t.Fatal("List should not be called"); return nil },
			Result:       func() string { t.Fatal("Result should not be called"); return "" },
			Invalid: func(runtime.ValidatorKind) error {
				return want
			},
		},
	)
	if !errors.Is(err, want) {
		t.Fatalf("DispatchNoCanonical() error = %v, want %v", err, want)
	}
}
