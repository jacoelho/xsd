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

	tests := []struct {
		xml     string
		opts    Options
		message string
	}{
		{xml: `<root a="1" b="2"/>`, opts: Options{MaxInstanceAttributes: 1}, message: "MaxInstanceAttributes=1"},
		{xml: `<root><v/></root>`, opts: Options{MaxInstanceDepth: 1}, message: "MaxInstanceDepth=1"},
		{xml: `<root>text</root>`, opts: Options{MaxInstanceTokenBytes: 1}, message: "MaxInstanceTokenBytes=1"},
		{xml: `<r a="12" b="34"/>`, opts: Options{MaxInstanceTokenBytes: 6}, message: "MaxInstanceTokenBytes=6"},
		{xml: `<root>text</root>`, opts: Options{MaxInstanceBytes: 4}, message: "MaxInstanceBytes=4"},
	}
	for _, tt := range tests {
		diagnostic := requireDiagnostic(t, CheckXMLWellFormed(strings.NewReader(tt.xml), tt.opts), xsderrors.CodeValidationLimit)
		if !strings.Contains(diagnostic.Error(), tt.message) {
			t.Fatalf("CheckXMLWellFormed(%q) = %v, want message naming %s", tt.xml, diagnostic, tt.message)
		}
	}
}

// The schema-free check and schema validation read instances under the same
// limits, so for every limit value they accept or reject a document alike and
// report the same diagnostic.
func TestCheckXMLWellFormedMatchesSessionReaderLimits(t *testing.T) {
	t.Parallel()

	// root has xs:anyType, so every well-formed document under it is valid.
	pool := NewSessionPool(compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"/>
</xs:schema>`))
	const doc = `<root a="1" b="2"><v><w>text</w></v></root>`
	limits := []struct {
		name string
		opts func(int) Options
	}{
		{name: "attributes", opts: func(n int) Options { return Options{MaxInstanceAttributes: n} }},
		{name: "depth", opts: func(n int) Options { return Options{MaxInstanceDepth: n} }},
		{name: "token bytes", opts: func(n int) Options { return Options{MaxInstanceTokenBytes: int64(n)} }},
		{name: "input bytes", opts: func(n int) Options { return Options{MaxInstanceBytes: int64(n)} }},
	}
	for _, limit := range limits {
		t.Run(limit.name, func(t *testing.T) {
			t.Parallel()
			var accepted, rejected bool
			for n := 1; n <= len(doc)+1; n++ {
				opts := limit.opts(n)
				checked := CheckXMLWellFormed(strings.NewReader(doc), opts)
				validated := pool.Validate(strings.NewReader(doc), opts)
				if fmt.Sprint(checked) != fmt.Sprint(validated) {
					t.Fatalf("limit %d: CheckXMLWellFormed = %v, Validate = %v", n, checked, validated)
				}
				if checked == nil {
					accepted = true
				} else {
					rejected = true
				}
			}
			if !accepted || !rejected {
				t.Fatalf("sweep accepted=%t rejected=%t, want both", accepted, rejected)
			}
		})
	}
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
