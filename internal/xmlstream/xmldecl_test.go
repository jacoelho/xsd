package xmlstream

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
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
					if !errors.As(err, &xmlErr) || xmlErr.kind != errorSyntax {
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
				if !ok || !errors.Is(err, errUnsupportedNonUTF8) || xerr.kind != errorUnsupported || xerr.line != 1 || xerr.column != test.column {
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

func TestNextDoesNotAdoptReaderErrorFromCallerChain(t *testing.T) {
	t.Parallel()

	newOuter := func(callerErr error) *Reader {
		outer := io.MultiReader(strings.NewReader("<r>"+strings.Repeat("a", 3*xmlInputBufferSize)), iotest.ErrReader(callerErr))
		reader := new(Reader)
		if err := reader.Reset(outer, Config{}); err != nil {
			t.Fatalf("Reset() = %v, want nil", err)
		}
		return reader
	}
	ctrlReader := newOuter(errors.New("plain"))
	ctrlErr := drainReader(ctrlReader)
	ctrlLine, ctrlCol := ctrlReader.ErrorPos(ctrlErr)
	if ctrlLine != 1 || ctrlCol <= len("<r>") {
		t.Fatalf("control position = %d:%d, want line 1 past the preview", ctrlLine, ctrlCol)
	}

	dtd := readDocument("\n\n<!DOCTYPE r><r/>")
	if feature, ok := UnsupportedFeatureOf(dtd); !ok || feature != UnsupportedDTD {
		t.Fatalf("dtd setup = %v", dtd)
	}
	var depthReader Reader
	if err := depthReader.Reset(strings.NewReader("\n\n<a><b/></a>"), Config{Limits: Limits{MaxDepth: 1}}); err != nil {
		t.Fatal(err)
	}
	depth := drainReader(&depthReader)
	if limit, ok := LimitOf(depth); !ok || limit != LimitDepth {
		t.Fatalf("depth setup = %v", depth)
	}
	var misuseReader Reader
	if err := misuseReader.Reset(strings.NewReader("\n\n<r/>"), Config{}); err != nil {
		t.Fatal(err)
	}
	for started := false; !started; {
		started = mustNextToken(t, &misuseReader).Kind == KindStart
	}
	_, positioned := misuseReader.Next()
	if line, _ := misuseReader.ErrorPos(positioned); !errors.Is(positioned, ErrPendingStart) || !IsMisuse(positioned) || line <= 1 {
		t.Fatalf("positioned misuse setup = %v at line %d", positioned, line)
	}
	var zero Reader
	_, unpositioned := zero.Next()
	if !IsMisuse(unpositioned) {
		t.Fatalf("unpositioned misuse setup = %v", unpositioned)
	}

	inners := []struct {
		name string
		err  error
	}{
		{"dtd", dtd},
		{"depth", depth},
		{"positioned misuse", positioned},
		{"unpositioned misuse", unpositioned},
	}
	wraps := []struct {
		name string
		wrap func(error) error
	}{
		{"as is", func(err error) error { return err }},
		{"wrapped", func(err error) error { return fmt.Errorf("transport: %w", err) }},
		{"joined", func(err error) error { return errors.Join(err, errors.New("other")) }},
	}
	for _, inner := range inners {
		for _, w := range wraps {
			t.Run(inner.name+"/"+w.name, func(t *testing.T) {
				t.Parallel()
				reader := newOuter(w.wrap(inner.err))
				err := drainReader(reader)
				if !IsInputFailure(err) {
					t.Errorf("IsInputFailure(%v) = false, want true", err)
				}
				if _, ok := LimitOf(err); ok {
					t.Errorf("LimitOf(%v) classified a caller error", err)
				}
				if _, ok := UnsupportedFeatureOf(err); ok {
					t.Errorf("UnsupportedFeatureOf(%v) classified a caller error", err)
				}
				if IsMisuse(err) {
					t.Errorf("IsMisuse(%v) classified a caller error", err)
				}
				if !errors.Is(err, inner.err) {
					t.Errorf("error %v lost cause %v", err, inner.err)
				}
				if line, col := reader.ErrorPos(err); line != ctrlLine || col != ctrlCol {
					t.Errorf("ErrorPos = %d:%d, want %d:%d", line, col, ctrlLine, ctrlCol)
				}
			})
		}
	}
}

type errReader struct{ err error }

func (r *errReader) Read([]byte) (int, error) { return 0, r.err }
