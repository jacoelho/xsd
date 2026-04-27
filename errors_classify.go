package xsd

import (
	"errors"
	"io/fs"
	"os"
	"slices"

	"github.com/jacoelho/xsd/internal/schemaast"
	"github.com/jacoelho/xsd/internal/xmltext"
	"github.com/jacoelho/xsd/internal/xsderrors"
)

func classifyCallerError(err error) error {
	return classifyPublicError(err, KindCaller, ErrCaller)
}

func classifyIOError(err error) error {
	return classifyPublicError(err, KindIO, ErrIO)
}

func classifySchemaError(err error) error {
	if err == nil {
		return nil
	}
	if isIOError(err) {
		return classifyIOError(err)
	}
	if isSchemaParseError(err) {
		return classifyPublicError(err, KindSchema, ErrSchemaParse)
	}
	return classifyPublicError(err, KindSchema, ErrSchemaSemantic)
}

func classifyValidationInternalError(err error) error {
	return classifyPublicError(err, KindInternal, ErrValidationInternal)
}

func classifyValidationBoundaryError(err error) error {
	if err == nil {
		return nil
	}
	if list, ok := asValidationList(err); ok {
		return list
	}
	if existing, ok := existingClassifiedError(err); ok {
		return existing
	}
	if isIOError(err) {
		return classifyIOError(err)
	}
	return classifyValidationInternalError(err)
}

func classifyInternalError(err error) error {
	return classifyPublicError(err, KindInternal, ErrInternal)
}

func classifyPublicError(err error, kind ErrorKind, code ErrorCode) error {
	if err == nil {
		return nil
	}
	if existing, ok := existingClassifiedError(err); ok {
		return existing
	}
	return Error{
		Kind:    kind,
		Code:    code,
		Message: err.Error(),
		Err:     err,
	}
}

func existingClassifiedError(err error) (Error, bool) {
	switch e := err.(type) {
	case Error:
		e.Expected = slices.Clone(e.Expected)
		return e, true
	case *Error:
		if e == nil {
			return Error{}, false
		}
		root := *e
		root.Expected = slices.Clone(e.Expected)
		return root, true
	case xsderrors.Error:
		return fromInternalError(e), true
	case *xsderrors.Error:
		if e == nil {
			return Error{}, false
		}
		return fromInternalError(*e), true
	}

	var root Error
	if errors.As(err, &root) && root.Kind != 0 && root.Code != "" {
		root.Message = err.Error()
		root.Err = err
		root.Expected = slices.Clone(root.Expected)
		return root, true
	}

	var internal xsderrors.Error
	if errors.As(err, &internal) && internal.Kind != 0 && internal.Code != "" {
		root = fromInternalError(internal)
		root.Message = err.Error()
		root.Err = err
		return root, true
	}
	return Error{}, false
}

func isIOError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) || os.IsNotExist(err) || os.IsPermission(err) {
		return true
	}
	var pathErr *fs.PathError
	return errors.As(err, &pathErr)
}

func isSchemaParseError(err error) bool {
	if err == nil {
		return false
	}
	var parseErr *schemaast.ParseError
	if errors.As(err, &parseErr) {
		return true
	}
	var syntaxErr *xmltext.SyntaxError
	return errors.As(err, &syntaxErr)
}
