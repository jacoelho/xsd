package validate

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/stream"
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

func TestPrepareInstanceReaderRejectsInvalidInputs(t *testing.T) {
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
			_, err := PrepareInstanceReaderWithBuffer(strings.NewReader(tt.in), nil)
			requireCode(t, err, tt.code)
		})
	}
}

func TestPrepareInstanceReaderRejectsNilReader(t *testing.T) {
	_, err := PrepareInstanceReaderWithBuffer(nil, nil)
	requireCode(t, err, xsderrors.CodeValidationXML)
}

func TestPrepareInstanceReaderDoesNotReadWholeDocumentWithoutDeclaration(t *testing.T) {
	r := &oneByteReader{s: `<root>` + strings.Repeat("x", 1024)}
	if _, err := PrepareInstanceReaderWithBuffer(r, nil); err != nil {
		t.Fatalf("PrepareInstanceReaderWithBuffer() error = %v", err)
	}
	if r.reads > stream.XMLDeclarationPrefixLen {
		t.Fatalf("PrepareInstanceReaderWithBuffer() reads = %d, want at most %d", r.reads, stream.XMLDeclarationPrefixLen)
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
		{name: "entity", err: parserErr(t, `<root>&missing;</root>`, 0, 0), code: xsderrors.CodeUnsupportedExternal},
		{name: "syntax", err: errors.New("bad xml"), code: xsderrors.CodeValidationXML},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireCode(t, StreamError(2, 3, "/root", tt.err), tt.code)
		})
	}
}

func TestValidateDirectiveRejectsDTD(t *testing.T) {
	t.Parallel()

	err := ValidateDirective(StartContext{Path: "/", Line: 2, Column: 3}, []byte("DOCTYPE r"))
	requireCode(t, err, xsderrors.CodeUnsupportedDTD)
	if !strings.Contains(err.Error(), "DTD declarations are not supported") {
		t.Fatalf("ValidateDirective() error = %v", err)
	}
}

func parserErr(t *testing.T, doc string, maxTokenBytes int64, maxAttrs int) error {
	t.Helper()
	names := stream.NewCache()
	values := stream.NewCache()
	var p stream.Parser
	p.ResetWithLimit(strings.NewReader(doc), &names, &values, maxTokenBytes)
	p.SetMaxAttrs(maxAttrs)
	for {
		_, err := p.Next()
		if err != nil {
			return err
		}
	}
}

func requireCode(t *testing.T, err error, want xsderrors.Code) {
	t.Helper()
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error type = %T, want *xsderrors.Error", err)
	}
	if xerr.Code != want {
		t.Fatalf("code = %s, want %s", xerr.Code, want)
	}
}
