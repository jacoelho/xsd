package xmlstream

import (
	"errors"
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
				var versionErr UnsupportedXMLVersionError
				switch {
				case test.version != "":
					if !errors.As(err, &versionErr) || versionErr.Version != test.version {
						t.Fatalf("error = %v, want unsupported version %q", err, test.version)
					}
				case test.want == accepted:
					if err != nil {
						t.Fatalf("error = %v, want accepted", err)
					}
				case test.want == encoding:
					if !errors.Is(err, ErrUnsupportedNonUTF8) {
						t.Fatalf("error = %v, want %v", err, ErrUnsupportedNonUTF8)
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
