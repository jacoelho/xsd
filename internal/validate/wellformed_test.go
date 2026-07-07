package validate

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

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

func TestCheckXMLWellFormedHonorsParserLimits(t *testing.T) {
	t.Parallel()

	err := CheckXMLWellFormed(strings.NewReader(`<root a="1" b="2"/>`), Options{MaxInstanceAttributes: 1})
	requireCode(t, err, xsderrors.CodeValidationLimit)

	err = CheckXMLWellFormed(strings.NewReader(`<root><v/></root>`), Options{MaxInstanceDepth: 1})
	requireCode(t, err, xsderrors.CodeValidationLimit)
}
