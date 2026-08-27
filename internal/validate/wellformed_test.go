package validate

import (
	"errors"
	"fmt"
	"io"
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
		{name: "valid compact document", xml: `<root><v>1</v></root>`},
		{name: "mismatched end tag", xml: `<root><v>1</root>`, code: xsderrors.CodeValidationXML},
		{name: "multiple roots", xml: `<a/><b/>`, code: xsderrors.CodeValidationXML},
		{name: "text outside root", xml: `text<root/>`, code: xsderrors.CodeValidationText},
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
