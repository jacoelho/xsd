package harness

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	xsderrors "github.com/jacoelho/xsd/errors"
)

type fakeValidator struct {
	err error
}

func (v fakeValidator) Validate(io.Reader) error {
	return v.err
}

type fakeEngine struct {
	loadErr     error
	validateErr error
}

func (e fakeEngine) Load(fs.FS, string) (Validator, error) {
	if e.loadErr != nil {
		return nil, e.loadErr
	}
	return fakeValidator{err: e.validateErr}, nil
}

func TestEquivalentNilErrors(t *testing.T) {
	if !Equivalent(Result{}, Result{}) {
		t.Fatal("Equivalent() = false, want true")
	}
}

func TestEquivalentValidationCodes(t *testing.T) {
	left := Result{ValidateErr: xsderrors.ValidationList{
		xsderrors.NewValidation(xsderrors.ErrXMLParse, "left", ""),
		xsderrors.NewValidation(xsderrors.ErrContentModelInvalid, "left", ""),
	}}
	right := Result{ValidateErr: xsderrors.ValidationList{
		xsderrors.NewValidation(xsderrors.ErrContentModelInvalid, "right", ""),
		xsderrors.NewValidation(xsderrors.ErrXMLParse, "right", ""),
	}}

	if !Equivalent(left, right) {
		t.Fatal("Equivalent() = false, want true")
	}
}

func TestEquivalentValidationCodeMismatch(t *testing.T) {
	left := Result{ValidateErr: xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrXMLParse, "x", "")}}
	right := Result{ValidateErr: xsderrors.ValidationList{xsderrors.NewValidation(xsderrors.ErrUnexpectedElement, "x", "")}}

	if Equivalent(left, right) {
		t.Fatal("Equivalent() = true, want false")
	}
}

func TestEquivalentNonValidationErrors(t *testing.T) {
	left := Result{LoadErr: fmt.Errorf("load failed")}
	right := Result{LoadErr: fmt.Errorf("load failed")}

	if !Equivalent(left, right) {
		t.Fatal("Equivalent() = false, want true")
	}
}

func TestRunCaseLoadError(t *testing.T) {
	result := RunCase(fakeEngine{loadErr: fmt.Errorf("boom")}, Case{})
	if result.LoadErr == nil {
		t.Fatal("LoadErr = nil, want non-nil")
	}
}

func TestRunCaseValidateError(t *testing.T) {
	result := RunCase(fakeEngine{validateErr: fmt.Errorf("bad doc")}, Case{Document: []byte("<root/>")})
	if result.ValidateErr == nil {
		t.Fatal("ValidateErr = nil, want non-nil")
	}
}

func TestCompare(t *testing.T) {
	caseData := Case{
		Name:       "simple",
		SchemaFS:   fstest.MapFS{"schema.xsd": &fstest.MapFile{Data: []byte("<xs:schema/>")}},
		SchemaPath: "schema.xsd",
		Document:   []byte(strings.TrimSpace("<root/>")),
	}
	diff := Compare(fakeEngine{}, fakeEngine{}, caseData)
	if !diff.Equal() {
		t.Fatal("Compare().Equal() = false, want true")
	}
}
