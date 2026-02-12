package harness

import (
	"bytes"
	"cmp"
	"io"
	"io/fs"
	"slices"

	xsderrors "github.com/jacoelho/xsd/errors"
)

// Validator validates one XML document stream.
type Validator interface {
	Validate(r io.Reader) error
}

// Engine loads a schema and returns a validator.
type Engine interface {
	Load(fsys fs.FS, location string) (Validator, error)
}

// Case describes one differential validation scenario.
type Case struct {
	Name       string
	SchemaFS   fs.FS
	SchemaPath string
	Document   []byte
}

// Result captures load and validate outcomes.
type Result struct {
	LoadErr     error
	ValidateErr error
}

// Diff stores side-by-side outcomes.
type Diff struct {
	Left  Result
	Right Result
}

// Equal reports whether both sides are equivalent.
func (d Diff) Equal() bool {
	return Equivalent(d.Left, d.Right)
}

// RunCase executes one engine against one case.
func RunCase(engine Engine, tc Case) Result {
	if engine == nil {
		return Result{LoadErr: io.ErrUnexpectedEOF}
	}
	validator, err := engine.Load(tc.SchemaFS, tc.SchemaPath)
	if err != nil {
		return Result{LoadErr: err}
	}
	if validator == nil {
		return Result{LoadErr: io.ErrUnexpectedEOF}
	}
	return Result{ValidateErr: validator.Validate(bytes.NewReader(tc.Document))}
}

// Compare runs both engines and returns a diff.
func Compare(left, right Engine, tc Case) Diff {
	return Diff{
		Left:  RunCase(left, tc),
		Right: RunCase(right, tc),
	}
}

// Equivalent checks whether two results are behaviorally equivalent.
func Equivalent(left, right Result) bool {
	if !equivalentError(left.LoadErr, right.LoadErr) {
		return false
	}
	return equivalentError(left.ValidateErr, right.ValidateErr)
}

func equivalentError(left, right error) bool {
	leftCodes, leftIsValidation := validationCodes(left)
	rightCodes, rightIsValidation := validationCodes(right)

	if leftIsValidation || rightIsValidation {
		if !leftIsValidation || !rightIsValidation {
			return false
		}
		if len(leftCodes) != len(rightCodes) {
			return false
		}
		for index := range leftCodes {
			if leftCodes[index] != rightCodes[index] {
				return false
			}
		}
		return true
	}

	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Error() == right.Error()
}

func validationCodes(err error) ([]string, bool) {
	if err == nil {
		return nil, false
	}
	validations, ok := xsderrors.AsValidations(err)
	if !ok {
		return nil, false
	}
	codes := make([]string, 0, len(validations))
	for _, validation := range validations {
		codes = append(codes, validation.Code)
	}
	slices.SortStableFunc(codes, cmp.Compare[string])
	return codes, true
}
