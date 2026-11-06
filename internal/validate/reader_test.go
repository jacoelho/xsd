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
		{name: "xml 11 declaration", in: `<?xml version="1.1"?><root/>`, code: xsderrors.CodeUnsupportedXMLVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reader xmlstream.Reader
			diagnostic := requireDiagnostic(t, streamError(reader.Reset(strings.NewReader(tt.in), xmlstream.Config{}), startContext{}, Limits{}), tt.code)
			// Reset precedes the first token, so no instance location exists.
			if diagnostic.Line() != 0 || diagnostic.Column() != 0 || diagnostic.Path() != "" {
				t.Fatalf("location = %d:%d %q, want none", diagnostic.Line(), diagnostic.Column(), diagnostic.Path())
			}
			if tt.code == xsderrors.CodeUnsupportedXMLVersion && diagnostic.Cause() != nil {
				t.Fatalf("XML 1.1 cause = %T, want nil", diagnostic.Cause())
			}
		})
	}
}

func TestParserPreflightRejectsNilReader(t *testing.T) {
	var reader xmlstream.Reader
	requireCode(t, streamError(reader.Reset(nil, xmlstream.Config{}), startContext{}, Limits{}), xsderrors.CodeValidationXML)
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
		{name: "non utf8", err: parserErr(t, `<?xml version="1.0" encoding="ISO-8859-1"?><root/>`), code: xsderrors.CodeUnsupportedNonUTF8},
		{name: "xml 11", err: parserErr(t, `<?xml version="1.1"?><root/>`), code: xsderrors.CodeUnsupportedXMLVersion},
		{name: "dtd", err: parserErr(t, `<!DOCTYPE root><root/>`), code: xsderrors.CodeUnsupportedDTD},
		{name: "entity", err: parserErr(t, `<root>&missing;</root>`), code: xsderrors.CodeUnsupportedExternal},
		{name: "attribute entity", err: parserErr(t, `<root a="&missing;"/>`), code: xsderrors.CodeUnsupportedExternal},
		{name: "colon entity name", err: parserErr(t, `<root>&a:b;</root>`), code: xsderrors.CodeValidationXML},
		{name: "colon PI target", err: parserErr(t, `<root><?a:b?></root>`), code: xsderrors.CodeValidationXML},
		{name: "syntax", err: errors.New("bad xml"), code: xsderrors.CodeValidationXML},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnostic := requireDiagnostic(t, streamError(tt.err, startContext{Path: "/root", Line: 2, Column: 3}, Limits{}), tt.code)
			if diagnostic.Line() != 2 || diagnostic.Column() != 3 || diagnostic.Path() != "/root" {
				t.Fatalf("location = %d:%d %q, want 2:3 /root", diagnostic.Line(), diagnostic.Column(), diagnostic.Path())
			}
			if tt.code == xsderrors.CodeUnsupportedXMLVersion && diagnostic.Cause() != nil {
				t.Fatalf("XML 1.1 cause = %T, want nil", diagnostic.Cause())
			}
		})
	}
}

func TestStreamDTDDiagnosticPreservesCause(t *testing.T) {
	streamErr := parserErr(t, "\n<!DOCTYPE root><root/>")
	err := streamError(streamErr, startContext{Path: "/", Line: 2, Column: 1}, Limits{})
	if feature, ok := xmlstream.UnsupportedFeatureOf(err); !ok || feature != xmlstream.UnsupportedDTD {
		t.Fatalf("DTD cause lost: %v", err)
	}
	boundary, ok := errors.AsType[*xmlstream.Error](err)
	if !ok || boundary.Kind != xmlstream.ErrorUnsupported || boundary.Line != 2 || boundary.Column != 1 {
		t.Fatalf("stream cause = %+v, want unsupported at 2:1", boundary)
	}
}

func parserErr(t *testing.T, doc string) error {
	t.Helper()
	var reader xmlstream.Reader
	if err := reader.Reset(strings.NewReader(doc), xmlstream.Config{}); err != nil {
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
