package validate

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

type oneByteReader struct {
	s     string
	off   int
	reads int
}

func (r *oneByteReader) Read(p []byte) (int, error) {
	if r.off >= len(r.s) {
		return 0, io.EOF
	}
	p[0] = r.s[r.off]
	r.off++
	r.reads++
	return 1, nil
}

func TestParserPreflightRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		code xsderrors.Code
	}{
		{name: "utf16 be bom", in: string([]byte{0xFE, 0xFF}) + "<root/>", code: xsderrors.CodeUnsupportedNonUTF8},
		{name: "utf16 le bom", in: string([]byte{0xFF, 0xFE}) + "<root/>", code: xsderrors.CodeUnsupportedNonUTF8},
		{name: "non utf8 declaration", in: `<?xml version="1.0" encoding="ISO-8859-1"?><root/>`, code: xsderrors.CodeUnsupportedNonUTF8},
		{name: "xml 11 declaration", in: `<?xml version="1.1"?><root/>`, code: xsderrors.CodeUnsupportedXML11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reader xmlstream.Reader
			diagnostic := requireDiagnostic(t, instanceReaderError(reader.Reset(strings.NewReader(tt.in), xmlstream.Config{})), tt.code)
			if tt.code == xsderrors.CodeUnsupportedXML11 && diagnostic.Cause() != nil {
				t.Fatalf("XML 1.1 cause = %T, want nil", diagnostic.Cause())
			}
		})
	}
}

func TestParserPreflightRejectsNilReader(t *testing.T) {
	var reader xmlstream.Reader
	requireCode(t, instanceReaderError(reader.Reset(nil, xmlstream.Config{})), xsderrors.CodeValidationXML)
}

func TestParserPreflightDoesNotReadWholeDocumentWithoutDeclaration(t *testing.T) {
	r := &oneByteReader{s: `<root>` + strings.Repeat("x", 1024)}
	var reader xmlstream.Reader
	if err := reader.Reset(r, xmlstream.Config{}); err != nil {
		t.Fatalf("Reader.Reset() error = %v", err)
	}
	if r.reads > xmlstream.XMLDeclarationPrefixLen {
		t.Fatalf("Reader.Reset() reads = %d, want at most %d", r.reads, xmlstream.XMLDeclarationPrefixLen)
	}
}

func TestStreamErrorClassifiesParserErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code xsderrors.Code
	}{
		{name: "token limit", err: parserErr(t, `<root>text</root>`, 1, 0), code: xsderrors.CodeValidationLimit},
		{name: "attribute limit", err: parserErr(t, `<root a="1" b="2"/>`, 0, 1), code: xsderrors.CodeValidationLimit},
		{name: "non utf8", err: xmlstream.ErrUnsupportedNonUTF8, code: xsderrors.CodeUnsupportedNonUTF8},
		{name: "xml 11", err: xmlstream.UnsupportedXMLVersionError{Version: "1.1"}, code: xsderrors.CodeUnsupportedXML11},
		{name: "entity", err: parserErr(t, `<root>&missing;</root>`, 0, 0), code: xsderrors.CodeUnsupportedExternal},
		{name: "syntax", err: errors.New("bad xml"), code: xsderrors.CodeValidationXML},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnostic := requireDiagnostic(t, StreamError(2, 3, "/root", tt.err), tt.code)
			if tt.code == xsderrors.CodeUnsupportedXML11 && diagnostic.Cause() != nil {
				t.Fatalf("XML 1.1 cause = %T, want nil", diagnostic.Cause())
			}
		})
	}
}

func parserErr(t *testing.T, doc string, maxTokenBytes int64, maxAttrs int) error {
	t.Helper()
	var reader xmlstream.Reader
	if err := reader.Reset(strings.NewReader(doc), xmlstream.Config{Limits: xmlstream.Limits{
		MaxTokenBytes: maxTokenBytes,
		MaxAttrs:      maxAttrs,
	}}); err != nil {
		return err
	}
	var frames []xmlstream.Handle
	for {
		tok, err := reader.Next()
		if err != nil {
			return err
		}
		switch tok.Kind {
		case xmlstream.KindStart:
			handle, _, startErr := reader.Start()
			if startErr != nil {
				return startErr
			}
			frames = append(frames, handle)
		case xmlstream.KindEnd:
			if len(frames) == 0 {
				return errors.New("unexpected end element")
			}
			handle := frames[len(frames)-1]
			if matchErr := reader.MatchEnd(handle); matchErr != nil {
				return matchErr
			}
			if commitErr := reader.CommitEnd(handle); commitErr != nil {
				return commitErr
			}
			frames = frames[:len(frames)-1]
		case xmlstream.KindCharData, xmlstream.KindDirective, xmlstream.KindComment, xmlstream.KindPI:
			continue
		}
	}
}

func requireCode(t *testing.T, err error, want xsderrors.Code) {
	t.Helper()
	if requireDiagnostic(t, err, want) == nil {
		t.Fatal("requireDiagnostic returned nil")
	}
}

func requireDiagnostic(t *testing.T, err error, want xsderrors.Code) *xsderrors.Error {
	t.Helper()
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error type = %T, want *xsderrors.Error", err)
	}
	if xerr.Code() != want {
		t.Fatalf("code = %s, want %s", xerr.Code(), want)
	}
	return xerr
}
