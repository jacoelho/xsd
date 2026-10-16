package xmlstream

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReaderAttributeScannerRefills(t *testing.T) {
	t.Parallel()
	for _, mode := range []struct {
		name string
		skip bool
	}{{"retained", false}, {"skipped", true}} {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			input := &scriptedReader{reads: []scriptedRead{
				{data: []byte("<r xmlns:p=\"urn:p\" a='x&am")},
				{data: []byte("p;\r")},
				{data: []byte("\n\xc3")},
				{data: []byte("\xa9' xmlns=\"\" xml:space=\"preserve\" b=\"\"><p:c/></r>")},
			}}
			var reader Reader
			t.Cleanup(reader.Detach)
			if err := reader.Reset(input, Config{LazyAttrValues: true, SkipOrdinaryAttrValues: mode.skip}); err != nil {
				t.Fatal(err)
			}
			token := mustNextToken(t, &reader)
			if token.Kind != KindStart || len(token.Start.Attr) != 5 {
				t.Fatalf("root token = %+v, want start with five attributes", token)
			}
			for i, want := range [5]string{"urn:p", "x& é", "", "preserve", ""} {
				raw, retained := token.Start.Attr[i].RawValue()
				if mode.skip && (i == 1 || i == 4) {
					if retained || raw != nil {
						t.Fatalf("ordinary attribute %d = %q, %v, want omitted", i, raw, retained)
					}
					continue
				}
				if want == "" {
					text, ok := reader.MaterializeValue(&token.Start.Attr[i])
					if retained || raw != nil || !ok || text != "" {
						t.Fatalf("empty attribute %d = %q/%v, %q/%v", i, raw, retained, text, ok)
					}
					continue
				}
				if !retained || string(raw) != want {
					t.Fatalf("attribute %d = %q, %v, want retained %q", i, raw, retained, want)
				}
			}
			root, _, err := reader.Start()
			if err != nil {
				t.Fatal(err)
			}
			_ = mustNextToken(t, &reader)
			child, element, err := reader.Start()
			if err != nil {
				t.Fatal(err)
			}
			if element.Name != (xml.Name{Space: "urn:p", Local: "c"}) {
				t.Fatalf("child = %+v, want {urn:p}c", element.Name)
			}
			for _, handle := range [2]Handle{child, root} {
				if token := mustNextToken(t, &reader); token.Kind != KindEnd {
					t.Fatalf("token = %+v, want end", token)
				}
				if err := reader.End(handle); err != nil {
					t.Fatal(err)
				}
			}
			if token, err := reader.Next(); token != nil || !errors.Is(err, io.EOF) {
				t.Fatalf("after root = %v, %v, want EOF", token, err)
			}
			if err := reader.Complete(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReaderAttributeScannerTerminalFailures(t *testing.T) {
	t.Parallel()
	sourceErr := errors.New("attribute source failed")
	for _, mode := range []struct {
		name string
		skip bool
	}{{"retained", false}, {"skipped", true}} {
		for _, test := range []struct {
			name       string
			terminal   error
			wantSource bool
			wantEOF    bool
		}{
			{name: "EOF", terminal: io.EOF},
			{name: "source", terminal: sourceErr, wantSource: true},
			{name: "joined", terminal: errors.Join(io.EOF, sourceErr), wantSource: true, wantEOF: true},
		} {
			t.Run(mode.name+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				input := &scriptedReader{reads: []scriptedRead{
					{data: []byte(`<r a="ab`)},
					{data: []byte("c"), err: test.terminal},
				}}
				var reader Reader
				t.Cleanup(reader.Detach)
				config := Config{LazyAttrValues: true, SkipOrdinaryAttrValues: mode.skip}
				if err := reader.Reset(input, config); err != nil {
					t.Fatal(err)
				}
				token, err := reader.Next()
				boundary, ok := errors.AsType[*Error](err)
				if token != nil || !ok || boundary.Kind != ErrorSyntax || boundary.Line != 1 || boundary.Column != 9 {
					t.Fatalf("Next() = %v, %#v, want syntax failure at 1:9", token, boundary)
				}
				if !test.wantSource && err.Error() != "unexpected EOF in attribute value" {
					t.Fatalf("error = %q, want attribute EOF context", err)
				}
				if errors.Is(err, sourceErr) != test.wantSource || errors.Is(err, io.EOF) != test.wantEOF {
					t.Fatalf("error causes = %v, want source=%v EOF=%v", err, test.wantSource, test.wantEOF)
				}
				assertAttributeReaderReset(t, &reader, config)
			})
		}
	}
}

func TestReaderAttributeScannerTokenBoundary(t *testing.T) {
	t.Parallel()
	for _, mode := range []struct {
		name string
		skip bool
	}{{"retained", false}, {"skipped", true}} {
		for _, test := range []struct {
			name  string
			limit int64
			valid bool
		}{{"exact", 5, true}, {"below", 4, false}} {
			t.Run(mode.name+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				var reader Reader
				t.Cleanup(reader.Detach)
				config := Config{LazyAttrValues: true, SkipOrdinaryAttrValues: mode.skip, Limits: Limits{MaxTokenBytes: test.limit}}
				if err := reader.Reset(strings.NewReader(`<r a="abc"/>`), config); err != nil {
					t.Fatal(err)
				}
				token, err := reader.Next()
				if test.valid {
					if err != nil || token == nil || token.Kind != KindStart {
						t.Fatalf("exact admission = %v, %v, want start", token, err)
					}
				} else {
					boundary, ok := errors.AsType[*Error](err)
					if token != nil || !ok || boundary.Kind != ErrorLimit || !IsTokenLimit(err) || boundary.Line != 1 || boundary.Column != 6 {
						t.Fatalf("below admission = %v, %#v, want token limit at 1:6", token, boundary)
					}
				}
				assertAttributeReaderReset(t, &reader, config)
			})
		}
	}
}

func assertAttributeReaderReset(t *testing.T, reader *Reader, config Config) {
	t.Helper()
	reader.Detach()
	if err := reader.Reset(strings.NewReader(`<ok/>`), config); err != nil {
		t.Fatal(err)
	}
	if token := mustNextToken(t, reader); token.Kind != KindStart || token.Start.Name.Local != "ok" {
		t.Fatalf("reset token = %+v, want ok start", token)
	}
	handle, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if token := mustNextToken(t, reader); token.Kind != KindEnd {
		t.Fatalf("reset token = %+v, want end", token)
	}
	if err := reader.End(handle); err != nil {
		t.Fatal(err)
	}
	if token, err := reader.Next(); token != nil || !errors.Is(err, io.EOF) {
		t.Fatalf("reset completion = %v, %v, want EOF", token, err)
	}
	if err := reader.Complete(); err != nil {
		t.Fatal(err)
	}
}
