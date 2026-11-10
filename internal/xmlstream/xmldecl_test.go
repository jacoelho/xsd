package xmlstream

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// XML 1.0 Second Edition [23]-[26], [32], [80]-[81]: a malformed declaration is
// a syntax error; a well-formed version other than 1.0 or encoding other than
// UTF-8 is unsupported. Classification must not depend on whether the whole
// declaration fits in the Reset preview window.
func TestXMLDeclarationAdmission(t *testing.T) {
	t.Parallel()

	const (
		accepted = "accepted"
		syntax   = "syntax"
		encoding = "encoding"
	)
	for _, test := range []struct {
		decl    string
		want    string
		version string
	}{
		{decl: `version="1.0"`, want: accepted},
		{decl: `version='1.0'`, want: accepted},
		{decl: `version="1.0" encoding="UTF-8"`, want: accepted},
		{decl: `version="1.0" encoding='utf-8' standalone='yes'`, want: accepted},
		{decl: `version="1.0" encoding="UTF8"`, want: accepted},
		{decl: `version="1.0" standalone="no"`, want: accepted},

		{decl: `version=""`, want: syntax},
		{decl: `version="1.0 "`, want: syntax},
		{decl: `version="1,0"`, want: syntax},
		{decl: `version="1.0" encoding=""`, want: syntax},
		{decl: `version="1.0" encoding="UTF-8 "`, want: syntax},
		{decl: `version="1.0" encoding="8UTF"`, want: syntax},
		{decl: `version="1.0" encoding="utf 8"`, want: syntax},
		{decl: `encoding="UTF-8" version="1.0"`, want: syntax},
		{decl: `encoding="UTF-8"`, want: syntax},
		{decl: `version="1.0" version='1.0'`, want: syntax},
		{decl: `version="1.0" standalone="maybe"`, want: syntax},
		{decl: `version="1.0" standalone="yes" encoding="UTF-8"`, want: syntax},

		{decl: `version="1.1"`, version: "1.1"},
		{decl: `version="1.5"`, version: "1.5"},
		{decl: `version="2.0"`, version: "2.0"},
		{decl: `version="1.10"`, version: "1.10"},
		{decl: `version="1.0.0"`, version: "1.0.0"},
		{decl: `version="1.0" encoding="ISO-8859-1"`, want: encoding},
		{decl: `version="1.0" encoding="UTF-16"`, want: encoding},
		{decl: `version="1.0" encoding="ASCII"`, want: encoding},

		// The first defect in document order decides.
		{decl: `version="1.1" encoding="ISO-8859-1"`, version: "1.1"},
		{decl: `version="1,0" encoding="ISO-8859-1"`, want: syntax},
		{decl: `version="1.1" encoding="8UTF"`, version: "1.1"},
		{decl: `version="1.0" encoding="ISO-8859-1" standalone="maybe"`, want: encoding},
	} {
		for _, padded := range []bool{false, true} {
			name := test.decl
			if padded {
				name += "/padded"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				padding := " "
				if padded {
					padding = strings.Repeat(" ", maxXMLDeclarationPreviewBytes+1024)
				}
				err := readDocument(`<?xml` + padding + test.decl + `?><r/>`)
				switch {
				case test.version != "":
					feature, ok := UnsupportedFeatureOf(err)
					if !ok || feature != UnsupportedXMLVersion || err.Error() != "XML version "+test.version+" is not supported" {
						t.Fatalf("error = %v, want unsupported version %q", err, test.version)
					}
				case test.want == accepted:
					if err != nil {
						t.Fatalf("error = %v, want accepted", err)
					}
				case test.want == encoding:
					if !errors.Is(err, errUnsupportedNonUTF8) {
						t.Fatalf("error = %v, want %v", err, errUnsupportedNonUTF8)
					}
				default:
					var xmlErr *Error
					if !errors.As(err, &xmlErr) || xmlErr.Kind != ErrorSyntax {
						t.Fatalf("error = %v, want syntax error", err)
					}
				}
			})
		}
	}
}

func readDocument(document string) error {
	var reader Reader
	if err := reader.Reset(strings.NewReader(document), Config{}); err != nil {
		return err
	}
	return drainReader(&reader)
}

// A non-UTF-8 declaration is rejected wherever its end falls. Which call
// reports it, and where, characterizes the 64 KiB Reset preview window: a
// declaration ending within the preview fails Reset at 1:0; one ending beyond
// it passes Reset and fails the first Next after the declaration. Change these
// rows only when deliberately changing the preview cap.
func TestXMLDeclarationPreviewBoundary(t *testing.T) {
	t.Parallel()

	const prefix = `<?xml version="1.0" encoding="ISO-8859-1"`
	for _, bom := range []string{"", "\uFEFF"} {
		for _, test := range []struct {
			end     int
			atReset bool
			column  int
		}{
			{end: 4096, atReset: true},
			{end: 4097, atReset: true},
			{end: 49152, atReset: true},
			{end: 49153, atReset: true},
			{end: 65535, atReset: true},
			{end: 65536, atReset: true},
			{end: 65537, column: 65537},
		} {
			document := bom + prefix + strings.Repeat(" ", test.end-len(prefix)-len("?>")) + "?><r/>"
			for _, rc := range []readerCase{wholeInput, oneByteInput, dataErrInput} {
				var reader Reader
				err := reader.Reset(rc.open(document), Config{})
				if !test.atReset {
					if err != nil {
						t.Fatalf("bom=%t end=%d %s: Reset() error = %v, want nil", bom != "", test.end, rc.name, err)
					}
					_, err = reader.Next()
				}
				xerr, ok := errors.AsType[*Error](err)
				if !ok || !errors.Is(err, errUnsupportedNonUTF8) || xerr.Kind != ErrorUnsupported || xerr.Line != 1 || xerr.Column != test.column {
					t.Fatalf("bom=%t end=%d %s: error = %#v, want unsupported encoding at 1:%d", bom != "", test.end, rc.name, err, test.column)
				}
				reader.Detach()
			}
		}
	}
}

// An error returned by the caller's reader is never classified as this
// package's own limit or unsupported-feature failure, whatever its chain
// carries; the version error is reachable only through a Reader.
func TestCallerReaderErrorChainIsNotClassified(t *testing.T) {
	t.Parallel()

	err1 := readDocument(`<?xml version="1.1"?><r/>`)
	if feature, ok := UnsupportedFeatureOf(err1); !ok || feature != UnsupportedXMLVersion {
		t.Fatalf("setup error = %v, want unsupported XML version", err1)
	}
	for _, test := range []struct {
		name string
		err  error
	}{
		{"single", err1},
		{"wrapped", fmt.Errorf("transport: %w", err1)},
		{"joined", errors.Join(err1, errors.New("other"))},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var reader Reader
			err := reader.Reset(&errReader{err: test.err}, Config{})
			if err == nil {
				err = drainReader(&reader)
			}
			if _, ok := UnsupportedFeatureOf(err); ok {
				t.Errorf("UnsupportedFeatureOf(%v) classified a caller error", err)
			}
			if _, ok := LimitOf(err); ok {
				t.Errorf("LimitOf(%v) classified a caller error", err)
			}
			if !IsInputFailure(err) {
				t.Errorf("IsInputFailure(%v) = false, want true", err)
			}
			if !errors.Is(err, test.err) {
				t.Errorf("error %v lost cause %v", err, test.err)
			}
		})
	}
}

type errReader struct{ err error }

func (r *errReader) Read([]byte) (int, error) { return 0, r.err }
