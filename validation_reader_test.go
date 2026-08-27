package xsd_test

import (
	"errors"
	"io"
	"math"
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
