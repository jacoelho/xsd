package preprocessor

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestLoadSkipsMissingWhenAllowed(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("missing")
	result, err := Load(LoadConfig[string]{
		AllowMissing: true,
		Resolve: func() (io.ReadCloser, string, error) {
			return nil, "", wantErr
		},
		IsNotFound: func(err error) bool {
			return errors.Is(err, wantErr)
		},
		Key:   func(string) string { return "" },
		Load:  func(io.ReadCloser, string, string) (*parser.Schema, error) { return nil, nil },
		Close: func(io.Closer, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if result.Status != StatusSkippedMissing {
		t.Fatalf("Load() status = %v, want %v", result.Status, StatusSkippedMissing)
	}
}

func TestLoadDefersMergedTargetAndCloses(t *testing.T) {
	t.Parallel()

	closed := false
	result, err := Load(LoadConfig[string]{
		Resolve: func() (io.ReadCloser, string, error) {
			return &closer{Reader: strings.NewReader(""), onClose: func() { closed = true }}, "a.xsd", nil
		},
		Key: func(systemID string) string {
			return systemID
		},
		AlreadyMerged: func(target string) bool {
			return target == "a.xsd"
		},
		Load: func(io.ReadCloser, string, string) (*parser.Schema, error) {
			t.Fatal("Load callback should not be called for merged targets")
			return nil, nil
		},
		Close: func(doc io.Closer, _ string) error {
			return doc.Close()
		},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if result.Status != StatusDeferred {
		t.Fatalf("Load() status = %v, want %v", result.Status, StatusDeferred)
	}
	if result.Target != "a.xsd" {
		t.Fatalf("Load() target = %q, want a.xsd", result.Target)
	}
	if !closed {
		t.Fatal("Close callback was not called")
	}
}

func TestLoadDefersLoadingTargetAndCallsOnLoading(t *testing.T) {
	t.Parallel()

	called := false
	result, err := Load(LoadConfig[string]{
		Resolve: func() (io.ReadCloser, string, error) {
			return io.NopCloser(strings.NewReader("")), "b.xsd", nil
		},
		Key: func(systemID string) string {
			return systemID
		},
		IsLoading: func(target string) bool {
			return target == "b.xsd"
		},
		OnLoading: func(target string) {
			if target != "b.xsd" {
				t.Fatalf("OnLoading() target = %q, want b.xsd", target)
			}
			called = true
		},
		Load: func(io.ReadCloser, string, string) (*parser.Schema, error) {
			t.Fatal("Load callback should not be called for loading targets")
			return nil, nil
		},
		Close: func(doc io.Closer, _ string) error {
			return doc.Close()
		},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if result.Status != StatusDeferred {
		t.Fatalf("Load() status = %v, want %v", result.Status, StatusDeferred)
	}
	if !called {
		t.Fatal("OnLoading callback was not called")
	}
}

func TestLoadReturnsLoadedSchema(t *testing.T) {
	t.Parallel()

	wantSchema := parser.NewSchema()
	result, err := Load(LoadConfig[string]{
		Resolve: func() (io.ReadCloser, string, error) {
			return io.NopCloser(strings.NewReader("")), "c.xsd", nil
		},
		Key: func(systemID string) string {
			return systemID
		},
		Load: func(doc io.ReadCloser, systemID, target string) (*parser.Schema, error) {
			if err := doc.Close(); err != nil {
				t.Fatalf("Close() error = %v", err)
			}
			if systemID != "c.xsd" || target != "c.xsd" {
				t.Fatalf("Load() got (%q, %q), want (c.xsd, c.xsd)", systemID, target)
			}
			return wantSchema, nil
		},
		Close: func(doc io.Closer, _ string) error {
			return doc.Close()
		},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if result.Status != StatusLoaded {
		t.Fatalf("Load() status = %v, want %v", result.Status, StatusLoaded)
	}
	if result.Target != "c.xsd" || result.Schema != wantSchema {
		t.Fatalf("Load() result = %+v, want loaded schema for c.xsd", result)
	}
}

type closer struct {
	io.Reader
	onClose func()
}

func (c *closer) Close() error {
	if c.onClose != nil {
		c.onClose()
	}
	return nil
}
