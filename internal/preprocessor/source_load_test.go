package preprocessor

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestLoadResolvedReturnsCachedSchema(t *testing.T) {
	t.Parallel()

	doc := &testReadCloser{Reader: strings.NewReader("")}
	want := parser.NewSchema()

	got, err := LoadResolved(doc, "schema.xsd", LoadCallbacks{
		Loaded: func() (*parser.Schema, bool) {
			return want, true
		},
		Close: func(doc io.Closer, _ string) error {
			return doc.Close()
		},
		Parse: func(io.ReadCloser, string) (*parser.ParseResult, error) {
			t.Fatal("Parse callback should not be called for cached schemas")
			return nil, nil
		},
		ApplyParsed: func(*parser.ParseResult, string) (*parser.Schema, error) {
			t.Fatal("ApplyParsed callback should not be called for cached schemas")
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("LoadResolved() error = %v", err)
	}
	if got != want {
		t.Fatalf("LoadResolved() schema = %p, want %p", got, want)
	}
	if !doc.closed {
		t.Fatal("LoadResolved() did not close cached document")
	}
}

func TestLoadResolvedReturnsCircularResultAndCloseError(t *testing.T) {
	t.Parallel()

	doc := &testReadCloser{
		Reader:   strings.NewReader(""),
		closeErr: errors.New("close failed"),
	}
	wantSchema := parser.NewSchema()
	wantErr := errors.New("circular")

	got, err := LoadResolved(doc, "schema.xsd", LoadCallbacks{
		Circular: func() (*parser.Schema, error) {
			return wantSchema, wantErr
		},
		Close: func(doc io.Closer, _ string) error {
			return doc.Close()
		},
		Parse: func(io.ReadCloser, string) (*parser.ParseResult, error) {
			t.Fatal("Parse callback should not be called for circular results")
			return nil, nil
		},
		ApplyParsed: func(*parser.ParseResult, string) (*parser.Schema, error) {
			t.Fatal("ApplyParsed callback should not be called for circular results")
			return nil, nil
		},
	})
	if got != wantSchema {
		t.Fatalf("LoadResolved() schema = %p, want %p", got, wantSchema)
	}
	if !errors.Is(err, wantErr) || !errors.Is(err, doc.closeErr) {
		t.Fatalf("LoadResolved() error = %v, want joined circular and close errors", err)
	}
}

func TestLoadResolvedParsesAndApplies(t *testing.T) {
	t.Parallel()

	doc := io.NopCloser(strings.NewReader(""))
	parsed := &parser.ParseResult{Schema: parser.NewSchema()}
	want := parser.NewSchema()

	got, err := LoadResolved(doc, "schema.xsd", LoadCallbacks{
		Close: func(io.Closer, string) error {
			t.Fatal("Close callback should not be called after Parse takes ownership")
			return nil
		},
		Parse: func(gotDoc io.ReadCloser, gotSystemID string) (*parser.ParseResult, error) {
			if gotDoc != doc {
				t.Fatalf("Parse() doc = %v, want original doc", gotDoc)
			}
			if gotSystemID != "schema.xsd" {
				t.Fatalf("Parse() systemID = %q, want schema.xsd", gotSystemID)
			}
			return parsed, nil
		},
		ApplyParsed: func(result *parser.ParseResult, gotSystemID string) (*parser.Schema, error) {
			if result != parsed {
				t.Fatalf("ApplyParsed() result = %p, want %p", result, parsed)
			}
			if gotSystemID != "schema.xsd" {
				t.Fatalf("ApplyParsed() systemID = %q, want schema.xsd", gotSystemID)
			}
			return want, nil
		},
	})
	if err != nil {
		t.Fatalf("LoadResolved() error = %v", err)
	}
	if got != want {
		t.Fatalf("LoadResolved() schema = %p, want %p", got, want)
	}
}
