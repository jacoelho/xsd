package validate

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

type wellFormedEOFReader struct {
	data string
	done bool
}

func (r *wellFormedEOFReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(p, r.data), io.EOF
}

func TestCheckXMLWellFormed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		xml  string
		code xsderrors.Code
	}{
		{name: "literal outer whitespace", xml: " \t<root>&#32;<![CDATA[ ]]></root>\r\n"},
		{name: "reference before root", xml: `&#32;<root/>`, code: xsderrors.CodeValidationXML},
		{name: "reference after root", xml: `<root/> &#x20;`, code: xsderrors.CodeValidationXML},
		{name: "empty CDATA before root", xml: `<![CDATA[]]><root/>`, code: xsderrors.CodeValidationXML},
		{name: "CDATA after root", xml: `<root/><![CDATA[ ]]>`, code: xsderrors.CodeValidationXML},
		{name: "valid compact document", xml: `<root><v>1</v></root>`},
		{name: "mismatched end tag", xml: `<root><v>1</root>`, code: xsderrors.CodeValidationXML},
		{name: "multiple roots", xml: `<a/><b/>`, code: xsderrors.CodeValidationXML},
		{name: "text outside root", xml: `text<root/>`, code: xsderrors.CodeValidationXML},
		{name: "unbound namespace prefix", xml: `<p:root/>`, code: xsderrors.CodeValidationXML},
		{name: "directive", xml: `<!DOCTYPE root><root/>`, code: xsderrors.CodeUnsupportedDTD},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := CheckXMLWellFormed(strings.NewReader(tt.xml), Options{})
			if tt.code == "" {
				if err != nil {
					t.Fatalf("CheckXMLWellFormed() error = %v", err)
				}
				return
			}
			requireCode(t, err, tt.code)
		})
	}
}

func TestCheckXMLWellFormedUsesBufferedCharacterFailurePosition(t *testing.T) {
	t.Parallel()
	err := CheckXMLWellFormed(strings.NewReader("<root>\nabcdefgh\x01</root>"), Options{})
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("CheckXMLWellFormed() error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Code() != xsderrors.CodeValidationXML || xerr.Line() != 2 || xerr.Column() != 9 {
		t.Fatalf("diagnostic = %s at %d:%d, want %s at 2:9", xerr.Code(), xerr.Line(), xerr.Column(), xsderrors.CodeValidationXML)
	}
}

func TestCheckXMLWellFormedHonorsParserLimits(t *testing.T) {
	t.Parallel()

	err := CheckXMLWellFormed(strings.NewReader(`<root a="1" b="2"/>`), Options{MaxInstanceAttributes: 1})
	requireCode(t, err, xsderrors.CodeValidationLimit)

	err = CheckXMLWellFormed(strings.NewReader(`<root><v/></root>`), Options{MaxInstanceDepth: 1})
	requireCode(t, err, xsderrors.CodeValidationLimit)

	err = CheckXMLWellFormed(strings.NewReader(`<root>text</root>`), Options{MaxInstanceTokenBytes: 1})
	requireCode(t, err, xsderrors.CodeValidationLimit)

	err = CheckXMLWellFormed(strings.NewReader(`<r a="12" b="34"/>`), Options{MaxInstanceTokenBytes: 6})
	requireCode(t, err, xsderrors.CodeValidationLimit)
}

func TestCheckXMLWellFormedDefaultAllowsW3CAttributeGroupOracle(t *testing.T) {
	var xml strings.Builder
	xml.WriteString("<root")
	for i := range 2_000 {
		fmt.Fprintf(&xml, ` a%d="1"`, i)
	}
	xml.WriteString("/>")

	if err := CheckXMLWellFormed(strings.NewReader(xml.String()), Options{}); err != nil {
		t.Fatalf("CheckXMLWellFormed() error = %v", err)
	}
}

func TestCheckXMLWellFormedRejectsInputLimitJoinedWithEOF(t *testing.T) {
	doc := `<r/>`
	err := CheckXMLWellFormed(&wellFormedEOFReader{data: doc + "X"}, Options{MaxInstanceBytes: int64(len(doc))})
	requireCode(t, err, xsderrors.CodeValidationLimit)
}

type wellFormedRequestRecorder struct {
	src      io.Reader
	requests []int
}

func (r *wellFormedRequestRecorder) Read(p []byte) (int, error) {
	r.requests = append(r.requests, len(p))
	return r.src.Read(p)
}

// The well-formedness check passes the caller's reader to the tokenizer, so
// its read requests keep the tokenizer's bound: at most 65536 bytes, reached
// by an input larger than that.
func TestCheckXMLWellFormedReadRequestsAreBounded(t *testing.T) {
	t.Parallel()

	for _, doc := range []string{"<r/>", "<r>" + strings.Repeat("a", 70000) + "</r>"} {
		recorder := &wellFormedRequestRecorder{src: strings.NewReader(doc)}
		if err := CheckXMLWellFormed(recorder, Options{}); err != nil {
			t.Fatalf("CheckXMLWellFormed(%d bytes) error = %v", len(doc), err)
		}
		for i, n := range recorder.requests {
			if n <= 0 || n > 65536 {
				t.Fatalf("%d-byte document request %d length = %d, want (0, 65536]", len(doc), i, n)
			}
		}
		if got := slices.Max(recorder.requests); len(doc) > 65536 && got != 65536 {
			t.Fatalf("%d-byte document largest request = %d, want 65536", len(doc), got)
		}
	}
}
