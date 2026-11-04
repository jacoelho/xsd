package xsd_test

import (
	"errors"
	"io"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const validationReaderTestSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r" type="xs:anyType"/></xs:schema>`

type dataAndErrorReader struct {
	data string
	err  error
	done bool
}

func (r *dataAndErrorReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), r.err
}

func TestMaxInstanceBytesBoundaryAndSessionRecovery(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	doc := `<r/>`
	if validationErr := engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{MaxInstanceBytes: math.MaxInt64}); validationErr != nil {
		t.Fatalf("Validate(MaxInt64 byte limit) error = %v", validationErr)
	}
	if validationErr := engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc))}); validationErr != nil {
		t.Fatalf("Validate(exact byte limit) error = %v", validationErr)
	}
	err = engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc) - 1)})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)

	session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc))})
	if err != nil {
		t.Fatal(err)
	}
	err = session.Validate(strings.NewReader(`<r> </r>`))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
	if err := session.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("Session.Validate() after byte limit error = %v", err)
	}
}

func TestMaxInstanceBytesPreservesSimultaneousReaderError(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("read failed")
	doc := `<r/>`
	err = engine.ValidateWithOptions(&dataAndErrorReader{data: doc, err: sentinel}, xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc) - 1)})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Validate() error = %v, want reader cause", err)
	}
}

func TestMaxInstanceBytesRejectsLimitJoinedWithEOFAndSessionRecovers(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	doc := `<r/>`
	if validationErr := engine.ValidateWithOptions(&dataAndErrorReader{data: doc, err: io.EOF}, xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc))}); validationErr != nil {
		t.Fatalf("Validate(exact data with EOF) error = %v", validationErr)
	}

	opts := xsd.ValidateOptions{MaxInstanceBytes: int64(len(doc))}
	session, err := engine.NewSession(opts)
	if err != nil {
		t.Fatal(err)
	}
	err = session.Validate(&dataAndErrorReader{data: doc + "X", err: io.EOF})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Session.Validate() error = %v, want EOF reader cause", err)
	}
	if err := session.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("Session.Validate() after joined limit/EOF error = %v", err)
	}
}

func TestValidationPreservesJoinedReaderErrorAfterBareCR(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("read failed")
	err = session.Validate(&dataAndErrorReader{data: `<r/>\r`, err: errors.Join(io.EOF, sentinel)})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Session.Validate() error = %v, want reader cause", err)
	}
	if err := session.Validate(strings.NewReader(`<r/>`)); err != nil {
		t.Fatalf("Session.Validate() after reader error = %v", err)
	}
}

func TestValidationPreflightReaderErrorIsStructured(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("read failed")
	err = engine.Validate(&dataAndErrorReader{data: `<r`, err: errors.Join(io.EOF, sentinel)})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)
	if !errors.Is(err, sentinel) || !errors.Is(err, io.EOF) {
		t.Fatalf("Validate() error = %v, want joined reader causes", err)
	}
}

func TestValidationParserErrorUsesFailurePosition(t *testing.T) {
	t.Parallel()
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name      string
		xml       string
		line, col int
	}{
		{name: "end tag", xml: "<r>\n</r x>", line: 2, col: 5},
		{name: "buffered character data", xml: "<r>\nabcdefgh\x01</r>", line: 2, col: 9},
		{name: "bare CR", xml: "<r>a\rb\x01</r>", line: 2, col: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := engine.Validate(strings.NewReader(test.xml))
			var xerr *xsderrors.Error
			if !errors.As(err, &xerr) {
				t.Fatalf("Validate() error = %T %v, want *xsderrors.Error", err, err)
			}
			if xerr.Code() != xsderrors.CodeValidationXML || xerr.Line() != test.line || xerr.Column() != test.col {
				t.Fatalf("Validate() diagnostic = %s at %d:%d, want %s at %d:%d", xerr.Code(), xerr.Line(), xerr.Column(), xsderrors.CodeValidationXML, test.line, test.col)
			}
		})
	}
}

func TestValidationRejectsTruncatedTrailingMarkup(t *testing.T) {
	t.Parallel()
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{`</`, `</r `, `<next `} {
		t.Run(suffix, func(t *testing.T) {
			t.Parallel()
			err := engine.Validate(strings.NewReader(`<r/>` + suffix))
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)
		})
	}
}

type requestRecordingReader struct {
	src      io.Reader
	requests []int
}

func (r *requestRecordingReader) Read(p []byte) (int, error) {
	r.requests = append(r.requests, len(p))
	return r.src.Read(p)
}

// Validation reads the caller's reader in non-empty requests of at most 65536
// bytes, reached by a larger document, and never asks for more than
// MaxInstanceBytes+1.
func TestValidateReadRequestsAreBounded(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	doc := "<r>" + strings.Repeat("a", 70000) + "</r>"
	for _, limit := range []int64{0, int64(len(doc)), 100} {
		recorder := &requestRecordingReader{src: strings.NewReader(doc)}
		validationErr := engine.ValidateWithOptions(recorder, xsd.ValidateOptions{MaxInstanceBytes: limit})
		if (limit == 100) != (validationErr != nil) {
			t.Fatalf("MaxInstanceBytes=%d error = %v", limit, validationErr)
		}
		for i, n := range recorder.requests {
			if n <= 0 || n > 65536 || (limit > 0 && int64(n) > limit+1) {
				t.Fatalf("MaxInstanceBytes=%d request %d length = %d", limit, i, n)
			}
		}
		if got := slices.Max(recorder.requests); limit != 100 && got != 65536 {
			t.Fatalf("MaxInstanceBytes=%d largest request = %d, want 65536", limit, got)
		}
	}
}

func TestUndeclaredRootTypeRecovery(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	const namespaces = ` xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`
	selections := []struct {
		name, element, attrs string
		code                 xsderrors.Code
	}{
		{name: "unknown unprefixed", element: "u", attrs: ` xsi:type="Missing"`, code: xsderrors.CodeValidationType},
		{name: "unknown prefixed", element: "u", attrs: ` xsi:type="xs:Missing"`, code: xsderrors.CodeValidationType},
		{name: "malformed QName", element: "u", attrs: ` xsi:type="xs:string:extra"`, code: xsderrors.CodeValidationType},
		{name: "unbound prefix", element: "u", attrs: ` xsi:type="p:string"`, code: xsderrors.CodeValidationType},
		{name: "empty", element: "u", attrs: ` xsi:type=""`, code: xsderrors.CodeValidationType},
		{name: "whitespace", element: "u", attrs: ` xsi:type=" &#x9;&#xA;&#xD; "`, code: xsderrors.CodeValidationType},
		{name: "known type", element: "u", attrs: ` xsi:type="xs:string"`},
		{name: "declared root", element: "r", attrs: ` xsi:type="xs:Missing"`, code: xsderrors.CodeValidationType},
		{name: "no type", element: "u", code: xsderrors.CodeValidationRoot},
	}
	for _, maxErrors := range []int{0, 1} {
		name := "default error budget"
		if maxErrors == 1 {
			name = "one error budget"
		}
		t.Run(name, func(t *testing.T) {
			for _, selection := range selections {
				t.Run(selection.name, func(t *testing.T) {
					for _, ending := range []struct {
						name, text string
						malformed  bool
					}{
						{name: "complete", text: "</" + selection.element + ">"},
						{name: "mismatched", text: "</wrong>", malformed: true},
						{name: "truncated", malformed: true},
					} {
						t.Run(ending.name, func(t *testing.T) {
							session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: maxErrors})
							if err != nil {
								t.Fatal(err)
							}
							doc := "<" + selection.element + namespaces + selection.attrs + ">text" + ending.text
							err = session.Validate(strings.NewReader(doc))
							code := selection.code
							if ending.malformed {
								code = xsderrors.CodeValidationXML
							}
							if code == "" {
								if err != nil {
									t.Fatalf("Validate() error = %v", err)
								}
							} else {
								expectCategoryCode(t, err, xsderrors.CategoryValidation, code)
								if got := xsderrors.IsConclusiveValidation(err); got == ending.malformed {
									t.Fatalf("IsConclusiveValidation() = %t, malformed document = %t: %v", got, ending.malformed, err)
								}
							}
							if err := session.Validate(strings.NewReader(`<r/>`)); err != nil {
								t.Fatalf("Session.Validate() after previous document = %v", err)
							}
						})
					}
				})
			}
		})
	}
}

func TestUndeclaredRootTypeRecoveryPreservesFatalErrors(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(validationReaderTestSchema)))
	if err != nil {
		t.Fatal(err)
	}
	const start = `<u xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="xs:Missing">`
	const tokenLimit = 1024
	byteLimit := int64(len(start) + 8)
	sentinel := errors.New("read failed after root start")
	for _, maxErrors := range []int{0, 1} {
		name := "default error budget"
		if maxErrors == 1 {
			name = "one error budget"
		}
		t.Run(name, func(t *testing.T) {
			for _, test := range []struct {
				name     string
				tail     string
				readErr  error
				opts     xsd.ValidateOptions
				code     xsderrors.Code
				boundary string
			}{
				{name: "reader error", readErr: sentinel, code: xsderrors.CodeValidationXML},
				{name: "data with reader error", tail: "text</u>", readErr: sentinel, code: xsderrors.CodeValidationXML},
				{name: "data with joined EOF and reader error", tail: "text</u>", readErr: errors.Join(io.EOF, sentinel), code: xsderrors.CodeValidationXML},
				{
					name: "byte limit", tail: "more text</u>", opts: xsd.ValidateOptions{MaxInstanceBytes: byteLimit}, code: xsderrors.CodeValidationLimit,
					boundary: "<r>" + strings.Repeat("x", int(byteLimit)-len("<r></r>")) + "</r>",
				},
				{
					name: "token limit", tail: strings.Repeat("x", tokenLimit+1) + "</u>", opts: xsd.ValidateOptions{MaxInstanceTokenBytes: tokenLimit}, code: xsderrors.CodeValidationLimit,
					boundary: "<r>" + strings.Repeat("x", tokenLimit) + "</r>",
				},
				{
					name: "depth limit", tail: "<a><b/></a></u>", opts: xsd.ValidateOptions{MaxInstanceDepth: 2}, code: xsderrors.CodeValidationLimit,
					boundary: "<r><child/></r>",
				},
			} {
				t.Run(test.name, func(t *testing.T) {
					test.opts.MaxErrors = maxErrors
					session, err := engine.NewSession(test.opts)
					if err != nil {
						t.Fatal(err)
					}
					var tail io.Reader = strings.NewReader(test.tail)
					if test.readErr != nil {
						tail = &dataAndErrorReader{data: test.tail, err: test.readErr}
					}
					// A read boundary after the complete start keeps the fatal condition
					// later than type selection instead of failing during tokenization.
					err = session.Validate(io.MultiReader(strings.NewReader(start), tail))
					expectCategoryCode(t, err, xsderrors.CategoryValidation, test.code)
					if xsderrors.IsConclusiveValidation(err) {
						t.Fatalf("fatal failure is conclusive: %v", err)
					}
					if test.readErr != nil && !errors.Is(err, sentinel) {
						t.Fatalf("Validate() error = %v, want reader cause", err)
					}
					if errors.Is(test.readErr, io.EOF) && !errors.Is(err, io.EOF) {
						t.Fatalf("Validate() error = %v, want joined EOF cause", err)
					}
					if test.boundary != "" {
						if err := session.Validate(strings.NewReader(test.boundary)); err != nil {
							t.Fatalf("Session.Validate() at limit after failure = %v", err)
						}
					}
					if err := session.Validate(strings.NewReader(`<r/>`)); err != nil {
						t.Fatalf("Session.Validate() after fatal failure = %v", err)
					}
				})
			}
		})
	}
}
