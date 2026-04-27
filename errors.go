package xsd

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/xsderrors"
)

// ErrorKind classifies errors by caller action.
type ErrorKind uint8

const (
	KindCaller     ErrorKind = ErrorKind(xsderrors.KindCaller)
	KindSchema     ErrorKind = ErrorKind(xsderrors.KindSchema)
	KindValidation ErrorKind = ErrorKind(xsderrors.KindValidation)
	KindIO         ErrorKind = ErrorKind(xsderrors.KindIO)
	KindInternal   ErrorKind = ErrorKind(xsderrors.KindInternal)
)

// ErrorCode represents a W3C XSD error code.
type ErrorCode string

// Error is a classified XSD error.
type Error struct {
	Kind     ErrorKind
	Code     ErrorCode
	Message  string
	Err      error
	Actual   string
	Expected []string
}

func (e Error) Error() string {
	if e.Message == "" && e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e Error) Unwrap() error {
	return e.Err
}

func (e Error) Is(target error) bool {
	t, ok := errorTarget(target)
	if !ok {
		return false
	}
	if t.Kind == 0 && t.Code == "" {
		return false
	}
	if t.Kind != 0 && e.Kind != t.Kind {
		return false
	}
	if t.Code != "" && e.Code != t.Code {
		return false
	}
	return true
}

// Validation describes a schema validation error with a W3C or local error code
// and optional instance path and line/column context.
//
//nolint:errname // public API name uses XSD domain term.
type Validation struct {
	Code    ErrorCode
	Message string
	// Document holds an optional document URI for error ordering.
	Document string
	Path     string
	Actual   string
	Expected []string
	Line     int
	Column   int
}

// ValidationList is an error that wraps one or more validation errors.
type ValidationList []Validation //nolint:errname // public API name.

func (v ValidationList) Error() string {
	ordered := v
	if len(v) > 1 {
		ordered = slices.Clone(v)
		ordered.Sort()
	}
	switch len(ordered) {
	case 0:
		return "no validation errors"
	case 1:
		return ordered[0].Error()
	default:
		return fmt.Sprintf("%s (and %d more)", ordered[0].Error(), len(ordered)-1)
	}
}

// Sort orders the validation list deterministically by document, line, column, code, and message.
func (v ValidationList) Sort() {
	if len(v) < 2 {
		return
	}
	slices.SortStableFunc(v, func(a, b Validation) int {
		if a.Document == "" && b.Document != "" {
			return 1
		}
		if a.Document != "" && b.Document == "" {
			return -1
		}
		if a.Document != b.Document {
			return cmp.Compare(a.Document, b.Document)
		}
		lineA := max(a.Line, 0)
		lineB := max(b.Line, 0)
		if lineA != lineB {
			return cmp.Compare(lineA, lineB)
		}
		colA := max(a.Column, 0)
		colB := max(b.Column, 0)
		if colA != colB {
			return cmp.Compare(colA, colB)
		}
		if a.Code != b.Code {
			return cmp.Compare(a.Code, b.Code)
		}
		return cmp.Compare(a.Message, b.Message)
	})
}

// Error formats the validation for display, including code, message, and context.
func (v *Validation) Error() string {
	if v == nil {
		return "validation <nil>"
	}

	var b strings.Builder
	b.WriteByte('[')
	b.WriteString(string(v.Code))
	b.WriteString("] ")
	b.WriteString(v.Message)
	if v.Path != "" {
		b.WriteString(" at ")
		b.WriteString(v.Path)
	}
	if v.Line > 0 && v.Column > 0 {
		if v.Path == "" {
			b.WriteString(" at line ")
			b.WriteString(strconv.Itoa(v.Line))
			b.WriteString(", column ")
			b.WriteString(strconv.Itoa(v.Column))
		} else {
			b.WriteString(" (line ")
			b.WriteString(strconv.Itoa(v.Line))
			b.WriteString(", column ")
			b.WriteString(strconv.Itoa(v.Column))
			b.WriteByte(')')
		}
	}
	if len(v.Expected) > 0 {
		b.WriteString(" (expected: ")
		b.WriteString(strings.Join(v.Expected, ", "))
		b.WriteByte(')')
	}
	if v.Actual != "" {
		b.WriteString(" (actual: ")
		b.WriteString(v.Actual)
		b.WriteByte(')')
	}
	return b.String()
}

const (
	ErrNoRoot             ErrorCode = ErrorCode(xsderrors.ErrNoRoot)
	ErrSchemaNotLoaded    ErrorCode = ErrorCode(xsderrors.ErrSchemaNotLoaded)
	ErrXMLParse           ErrorCode = ErrorCode(xsderrors.ErrXMLParse)
	ErrCaller             ErrorCode = ErrorCode(xsderrors.ErrCaller)
	ErrIO                 ErrorCode = ErrorCode(xsderrors.ErrIO)
	ErrSchemaParse        ErrorCode = ErrorCode(xsderrors.ErrSchemaParse)
	ErrSchemaSemantic     ErrorCode = ErrorCode(xsderrors.ErrSchemaSemantic)
	ErrInternal           ErrorCode = ErrorCode(xsderrors.ErrInternal)
	ErrValidationInternal ErrorCode = ErrorCode(xsderrors.ErrValidationInternal)

	ErrElementNotDeclared       ErrorCode = ErrorCode(xsderrors.ErrElementNotDeclared)
	ErrElementAbstract          ErrorCode = ErrorCode(xsderrors.ErrElementAbstract)
	ErrElementNotNillable       ErrorCode = ErrorCode(xsderrors.ErrElementNotNillable)
	ErrNilElementNotEmpty       ErrorCode = ErrorCode(xsderrors.ErrNilElementNotEmpty)
	ErrXsiTypeInvalid           ErrorCode = ErrorCode(xsderrors.ErrXsiTypeInvalid)
	ErrElementTypeAbstract      ErrorCode = ErrorCode(xsderrors.ErrElementTypeAbstract)
	ErrElementFixedValue        ErrorCode = ErrorCode(xsderrors.ErrElementFixedValue)
	ErrTextInElementOnly        ErrorCode = ErrorCode(xsderrors.ErrTextInElementOnly)
	ErrContentModelInvalid      ErrorCode = ErrorCode(xsderrors.ErrContentModelInvalid)
	ErrRequiredElementMissing   ErrorCode = ErrorCode(xsderrors.ErrRequiredElementMissing)
	ErrUnexpectedElement        ErrorCode = ErrorCode(xsderrors.ErrUnexpectedElement)
	ErrAttributeNotDeclared     ErrorCode = ErrorCode(xsderrors.ErrAttributeNotDeclared)
	ErrAttributeProhibited      ErrorCode = ErrorCode(xsderrors.ErrAttributeProhibited)
	ErrRequiredAttributeMissing ErrorCode = ErrorCode(xsderrors.ErrRequiredAttributeMissing)
	ErrAttributeFixedValue      ErrorCode = ErrorCode(xsderrors.ErrAttributeFixedValue)
	ErrWildcardNotDeclared      ErrorCode = ErrorCode(xsderrors.ErrWildcardNotDeclared)
	ErrDatatypeInvalid          ErrorCode = ErrorCode(xsderrors.ErrDatatypeInvalid)
	ErrFacetViolation           ErrorCode = ErrorCode(xsderrors.ErrFacetViolation)
	ErrDuplicateID              ErrorCode = ErrorCode(xsderrors.ErrDuplicateID)
	ErrIDRefNotFound            ErrorCode = ErrorCode(xsderrors.ErrIDRefNotFound)
	ErrMultipleIDAttr           ErrorCode = ErrorCode(xsderrors.ErrMultipleIDAttr)
	ErrIdentityDuplicate        ErrorCode = ErrorCode(xsderrors.ErrIdentityDuplicate)
	ErrIdentityAbsent           ErrorCode = ErrorCode(xsderrors.ErrIdentityAbsent)
	ErrIdentityKeyRefFailed     ErrorCode = ErrorCode(xsderrors.ErrIdentityKeyRefFailed)

	ErrValidateValueInvalid                 ErrorCode = ErrorCode(xsderrors.ErrValidateValueInvalid)
	ErrValidateValueFacet                   ErrorCode = ErrorCode(xsderrors.ErrValidateValueFacet)
	ErrValidateElementAbstract              ErrorCode = ErrorCode(xsderrors.ErrValidateElementAbstract)
	ErrValidateSimpleTypeAttrNotAllowed     ErrorCode = ErrorCode(xsderrors.ErrValidateSimpleTypeAttrNotAllowed)
	ErrValidateXsiTypeUnresolved            ErrorCode = ErrorCode(xsderrors.ErrValidateXsiTypeUnresolved)
	ErrValidateXsiTypeDerivationBlocked     ErrorCode = ErrorCode(xsderrors.ErrValidateXsiTypeDerivationBlocked)
	ErrValidateXsiNilNotNillable            ErrorCode = ErrorCode(xsderrors.ErrValidateXsiNilNotNillable)
	ErrValidateNilledHasFixed               ErrorCode = ErrorCode(xsderrors.ErrValidateNilledHasFixed)
	ErrValidateNilledNotEmpty               ErrorCode = ErrorCode(xsderrors.ErrValidateNilledNotEmpty)
	ErrValidateWildcardElemStrictUnresolved ErrorCode = ErrorCode(xsderrors.ErrValidateWildcardElemStrictUnresolved)
	ErrValidateWildcardAttrStrictUnresolved ErrorCode = ErrorCode(xsderrors.ErrValidateWildcardAttrStrictUnresolved)
	ErrValidateRootNotDeclared              ErrorCode = ErrorCode(xsderrors.ErrValidateRootNotDeclared)
)

func AsValidations(err error) ([]Validation, bool) {
	list, ok := asValidationList(err)
	if !ok {
		return nil, false
	}
	cloned := slices.Clone(list)
	cloned.Sort()
	return []Validation(cloned), true
}

func KindOf(err error) (ErrorKind, bool) {
	if err == nil {
		return 0, false
	}
	var xsdErr Error
	if errors.As(err, &xsdErr) && xsdErr.Kind != 0 {
		return xsdErr.Kind, true
	}
	if _, ok := asValidationList(err); ok {
		return KindValidation, true
	}
	var internalErr xsderrors.Error
	if errors.As(err, &internalErr) && internalErr.Kind != 0 {
		return fromInternalErrorKind(internalErr.Kind), true
	}
	return 0, false
}

func errorTarget(target error) (Error, bool) {
	var root Error
	if errors.As(target, &root) {
		return root, true
	}
	var internal xsderrors.Error
	if errors.As(target, &internal) {
		return fromInternalError(internal), true
	}
	return Error{}, false
}

func asValidationList(err error) (ValidationList, bool) {
	if err == nil {
		return nil, false
	}
	var list ValidationList
	if errors.As(err, &list) {
		return list, true
	}
	var listPtr *ValidationList
	if errors.As(err, &listPtr) && listPtr != nil {
		return *listPtr, true
	}
	var internalList xsderrors.ValidationList
	if errors.As(err, &internalList) {
		return fromInternalValidationList(internalList), true
	}
	var internalListPtr *xsderrors.ValidationList
	if errors.As(err, &internalListPtr) && internalListPtr != nil {
		return fromInternalValidationList(*internalListPtr), true
	}
	return nil, false
}

func fromInternalError(err xsderrors.Error) Error {
	return Error{
		Kind:     fromInternalErrorKind(err.Kind),
		Code:     ErrorCode(err.Code),
		Message:  err.Message,
		Err:      err.Err,
		Actual:   err.Actual,
		Expected: slices.Clone(err.Expected),
	}
}

func fromInternalErrorKind(kind xsderrors.ErrorKind) ErrorKind {
	switch kind {
	case xsderrors.KindCaller:
		return KindCaller
	case xsderrors.KindSchema:
		return KindSchema
	case xsderrors.KindValidation:
		return KindValidation
	case xsderrors.KindIO:
		return KindIO
	case xsderrors.KindInternal:
		return KindInternal
	default:
		return ErrorKind(kind)
	}
}

func fromInternalValidation(v xsderrors.Validation) Validation {
	return Validation{
		Code:     ErrorCode(v.Code),
		Message:  v.Message,
		Document: v.Document,
		Path:     v.Path,
		Actual:   v.Actual,
		Expected: slices.Clone(v.Expected),
		Line:     v.Line,
		Column:   v.Column,
	}
}

func fromInternalValidationList(list xsderrors.ValidationList) ValidationList {
	out := make(ValidationList, len(list))
	for i, v := range list {
		out[i] = fromInternalValidation(v)
	}
	return out
}
