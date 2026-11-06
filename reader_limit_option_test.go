package xsd_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestInstanceReaderLimitsNameOptions(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		opts    xsd.ValidateOptions
		valid   string
		invalid string
		option  string
	}{
		{name: "depth", opts: xsd.ValidateOptions{MaxInstanceDepth: 2}, valid: `<r><a/></r>`, invalid: `<r><a><b/></a></r>`, option: "MaxInstanceDepth=2"},
		{name: "attributes", opts: xsd.ValidateOptions{MaxInstanceAttributes: 1}, valid: `<r a="1"/>`, invalid: `<r a="1" b="2"/>`, option: "MaxInstanceAttributes=1"},
		{name: "token", opts: xsd.ValidateOptions{MaxInstanceTokenBytes: 4}, valid: `<r>xxxx</r>`, invalid: `<r>xxxxx</r>`, option: "MaxInstanceTokenBytes=4"},
		{name: "input", opts: xsd.ValidateOptions{MaxInstanceBytes: 11}, valid: `<r>xxxx</r>`, invalid: `<r>xxxxx</r>`, option: "MaxInstanceBytes=11"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := engine.NewSession(tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if validErr := session.Validate(strings.NewReader(tt.valid)); validErr != nil {
				t.Fatalf("at-limit document: %v", validErr)
			}
			err = session.Validate(strings.NewReader(tt.invalid))
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
			if !strings.Contains(err.Error(), tt.option) {
				t.Fatalf("error %q does not name %s", err, tt.option)
			}
		})
	}
}

func TestSchemaReaderLimitsNameOptions(t *testing.T) {
	const prefix = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`
	tests := []struct {
		name    string
		opts    xsd.CompileOptions
		valid   string
		invalid string
		option  string
	}{
		{name: "depth", opts: xsd.CompileOptions{MaxSchemaDepth: 3},
			valid:   prefix + `<xs:element name="r"><xs:complexType/></xs:element></xs:schema>`,
			invalid: prefix + `<xs:element name="r"><xs:complexType><xs:sequence/></xs:complexType></xs:element></xs:schema>`,
			option:  "MaxSchemaDepth=3"},
		{name: "attributes", opts: xsd.CompileOptions{MaxSchemaAttributes: 3},
			valid:   `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r"/></xs:schema>`,
			invalid: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r" id="a" type="xs:string" nillable="true"/></xs:schema>`,
			option:  "MaxSchemaAttributes=3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := xsd.CompileWithOptions(tt.opts, xsd.Bytes("schema.xsd", []byte(tt.valid))); err != nil {
				t.Fatalf("at-limit schema: %v", err)
			}
			_, err := xsd.CompileWithOptions(tt.opts, xsd.Bytes("schema.xsd", []byte(tt.invalid)))
			expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaLimit)
			if !strings.Contains(err.Error(), tt.option) {
				t.Fatalf("error %q does not name %s", err, tt.option)
			}
		})
	}
}

func compileSchemaAt(src string, limit int64) error {
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaTokenBytes: limit}, xsd.Bytes("schema.xsd", []byte(src)))
	return err
}

func requireSchemaTokenLimit(t *testing.T, err error, limit int64) {
	t.Helper()
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaLimit)
	want := "MaxSchemaTokenBytes=" + strconv.FormatInt(limit, 10) + " exceeded"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err, want)
	}
}

// The reader measures tokens lexically, so a start tag compiles at a limit
// equal to its source length however many names it expands to.
func TestSchemaTokenLimitIsLexical(t *testing.T) {
	const xsNS = `xmlns:xs="http://www.w3.org/2001/XMLSchema"`
	uri := "urn:" + strings.Repeat("u", 120)
	tests := map[string]string{
		"target namespace":   `<xs:schema ` + xsNS + ` targetNamespace="` + uri + `"/>`,
		"foreign attributes": `<xs:schema ` + xsNS + ` xmlns:f="` + uri + `" f:a="1" f:b="2" f:c="3" f:d="4"/>`,
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			tag := src[:strings.Index(src, ">")+1]
			if err := compileSchemaAt(src, int64(len(tag))); err != nil {
				t.Fatalf("compile at lexical start tag size %d: %v", len(tag), err)
			}
		})
	}
}

// Each case passes at the lexical boundary L (bytes of names and values the
// reader holds for the token) and fails at L-1.
func TestSchemaTokenLimitBoundaries(t *testing.T) {
	const open = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`
	const closing = `</xs:schema>`
	const n = 60
	tests := []struct {
		name  string
		src   string
		limit int64
		line  int
	}{
		{"attribute value on non-root element",
			open + "\n\n" + `<xs:element name="` + strings.Repeat("a", n) + `"/>` + closing,
			int64(len("xs:element") + len("name") + n), 3},
		{"foreign element name",
			open + `<xs:annotation><xs:appinfo><` + strings.Repeat("e", n) + ` xmlns="urn:foreign"/></xs:appinfo></xs:annotation>` + closing,
			int64(n + len("xmlns") + len("urn:foreign")), 1},
		{"processing instruction",
			open + `<xs:annotation><?pi ` + strings.Repeat("d", n) + `?></xs:annotation>` + closing,
			int64(len("pi") + n), 1},
		{"text run",
			open + `<xs:annotation><xs:documentation>` + strings.Repeat("t", n) + `</xs:documentation></xs:annotation>` + closing,
			n, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := compileSchemaAt(tt.src, tt.limit); err != nil {
				t.Fatalf("compile at limit %d: %v", tt.limit, err)
			}
			err := compileSchemaAt(tt.src, tt.limit-1)
			requireSchemaTokenLimit(t, err, tt.limit-1)
			if xerr, ok := errors.AsType[*xsderrors.Error](err); !ok || xerr.Line() != tt.line {
				t.Fatalf("diagnostic line = %v, want %d (%v)", xerr, tt.line, err)
			}
		})
	}
}

// Character data split across text tokens is invisible to the reader's
// per-token limit; the schema measures it per element.
func TestSchemaTokenLimitCountsAccumulatedText(t *testing.T) {
	doc := func(body string) string {
		return `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:documentation>` + body + `</xs:documentation></xs:annotation></xs:schema>`
	}
	// The limit sits above every start tag so only the text decides.
	const half, limit = 100, 150
	text := strings.Repeat("a", half)
	for _, part := range []string{text, `<![CDATA[` + text + `]]>`} {
		if err := compileSchemaAt(doc(part), limit); err != nil {
			t.Fatalf("one half at limit %d: %v", limit, err)
		}
	}
	both := doc(text + `<![CDATA[` + text + `]]>`)
	requireSchemaTokenLimit(t, compileSchemaAt(both, limit), limit)
	if err := compileSchemaAt(both, 2*half); err != nil {
		t.Fatalf("accumulated text at limit %d: %v", 2*half, err)
	}
}

// Every site that maps a reader limit failure reports '<option>=<n> exceeded'.
func TestInstanceReaderLimitMappingSites(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		opts   xsd.ValidateOptions
		doc    string
		option string
	}{
		{"reset", xsd.ValidateOptions{MaxInstanceBytes: 1}, `<r/>`, "MaxInstanceBytes=1 exceeded"},
		{"token loop", xsd.ValidateOptions{MaxInstanceTokenBytes: 3}, `<r>xxxx</r>`, "MaxInstanceTokenBytes=3 exceeded"},
		{"start preparation", xsd.ValidateOptions{MaxInstanceDepth: 1}, `<r><a/></r>`, "MaxInstanceDepth=1 exceeded"},
		{"parse error after start", xsd.ValidateOptions{MaxInstanceAttributes: 1}, `<r a="1" b="2"/>`, "MaxInstanceAttributes=1 exceeded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, sessErr := engine.NewSession(tt.opts)
			if sessErr != nil {
				t.Fatal(sessErr)
			}
			valErr := session.Validate(strings.NewReader(tt.doc))
			expectCategoryCode(t, valErr, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
			if !strings.Contains(valErr.Error(), tt.option) {
				t.Fatalf("error %q does not contain %q", valErr, tt.option)
			}
		})
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = session.Validate(strings.NewReader(`<r a="1" a="2"/>`))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)
	if xerr, ok := errors.AsType[*xsderrors.Error](err); !ok || xerr.Line() == 0 {
		t.Fatalf("duplicate attribute error has no position: %v", err)
	}
}
