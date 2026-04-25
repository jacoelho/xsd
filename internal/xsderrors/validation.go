package xsderrors

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// ErrorCode represents a W3C XSD error code.
// See: https://www.w3.org/TR/xmlschema-1/#cvc-elt
type ErrorCode string

// ErrorKind classifies errors by caller action.
type ErrorKind uint8

const (
	KindCaller ErrorKind = iota + 1
	KindSchema
	KindValidation
	KindIO
	KindInternal
)

const (
	// ErrNoRoot indicates the XML document has no root element.
	ErrNoRoot ErrorCode = "xsd-no-root"
	// ErrSchemaNotLoaded indicates validation was attempted without a loaded schema.
	ErrSchemaNotLoaded ErrorCode = "xsd-schema-not-loaded"
	// ErrXMLParse indicates the XML document could not be parsed.
	ErrXMLParse ErrorCode = "xml-parse-error"

	// ErrElementNotDeclared indicates an element has no declaration.
	ErrElementNotDeclared ErrorCode = "cvc-elt.1"
	// ErrElementAbstract indicates an abstract element was used.
	ErrElementAbstract ErrorCode = "cvc-elt.2"
	// ErrElementNotNillable indicates xsi:nil was used on a non-nillable element.
	ErrElementNotNillable ErrorCode = "cvc-elt.3.1"
	// ErrNilElementNotEmpty indicates a nilled element had content.
	ErrNilElementNotEmpty ErrorCode = "cvc-elt.3.2.2"
	// ErrXsiTypeInvalid indicates an xsi:type could not be resolved or is invalid.
	ErrXsiTypeInvalid ErrorCode = "cvc-elt.4.3"
	// ErrElementTypeAbstract indicates an abstract type was used for an element.
	ErrElementTypeAbstract ErrorCode = "cvc-elt.4.2"
	// ErrElementFixedValue indicates a fixed element value was violated.
	ErrElementFixedValue ErrorCode = "cvc-elt.5.2.2.2"

	// ErrTextInElementOnly indicates text appeared in element-only content.
	ErrTextInElementOnly ErrorCode = "cvc-complex-type.2.3"
	// ErrContentModelInvalid indicates children violate the content schemaast.
	ErrContentModelInvalid ErrorCode = "cvc-complex-type.2.4"
	// ErrRequiredElementMissing indicates a required child element is missing.
	ErrRequiredElementMissing ErrorCode = "cvc-complex-type.2.4.b"
	// ErrUnexpectedElement indicates an unexpected child element.
	ErrUnexpectedElement ErrorCode = "cvc-complex-type.2.4.d"
	// ErrAttributeNotDeclared indicates an attribute is not declared.
	ErrAttributeNotDeclared ErrorCode = "cvc-complex-type.3.2.1"
	// ErrAttributeProhibited indicates a prohibited attribute is present.
	ErrAttributeProhibited ErrorCode = "cvc-complex-type.3.2.2"
	// ErrRequiredAttributeMissing indicates a required attribute is missing.
	ErrRequiredAttributeMissing ErrorCode = "cvc-complex-type.4"

	// ErrAttributeFixedValue indicates a fixed attribute value was violated.
	ErrAttributeFixedValue ErrorCode = "cvc-attribute.1"

	// ErrWildcardNotDeclared indicates a wildcard requires a declaration.
	ErrWildcardNotDeclared ErrorCode = "cvc-wildcard.1.2"

	// ErrDatatypeInvalid indicates a lexical value is invalid for its datatype.
	ErrDatatypeInvalid ErrorCode = "cvc-datatype-valid"
	// ErrFacetViolation indicates a value violates a facet constraint.
	ErrFacetViolation ErrorCode = "cvc-facet-valid"

	// ErrDuplicateID indicates a duplicate ID value.
	ErrDuplicateID ErrorCode = "cvc-id.2"
	// ErrIDRefNotFound indicates an IDREF was not found.
	ErrIDRefNotFound ErrorCode = "cvc-id.1.2"
	// ErrMultipleIDAttr indicates multiple ID attributes on the same element.
	ErrMultipleIDAttr ErrorCode = "cvc-id.3"

	// ErrIdentityDuplicate indicates an identity constraint is duplicated.
	ErrIdentityDuplicate ErrorCode = "cvc-identity-constraint.4.1"
	// ErrIdentityAbsent indicates an identity constraint is absent.
	ErrIdentityAbsent ErrorCode = "cvc-identity-constraint.4.2.1"
	// ErrIdentityKeyRefFailed indicates a keyref constraint failed.
	ErrIdentityKeyRefFailed ErrorCode = "cvc-identity-constraint.4.3"

	// ErrValidateValueInvalid indicates a value failed lexical parsing.
	ErrValidateValueInvalid ErrorCode = "xsd-value-invalid"
	// ErrValidateValueFacet indicates a value violated a facet constraint.
	ErrValidateValueFacet ErrorCode = "xsd-value-facet"
	// ErrValidateElementAbstract indicates an abstract element was used.
	ErrValidateElementAbstract ErrorCode = "xsd-element-abstract"
	// ErrValidateSimpleTypeAttrNotAllowed indicates attributes on simple types.
	ErrValidateSimpleTypeAttrNotAllowed ErrorCode = "xsd-simple-type-attr-not-allowed"
	// ErrValidateXsiTypeUnresolved indicates xsi:type could not be resolved.
	ErrValidateXsiTypeUnresolved ErrorCode = "xsd-xsi-type-unresolved"
	// ErrValidateXsiTypeDerivationBlocked indicates xsi:type derivation is blocked.
	ErrValidateXsiTypeDerivationBlocked ErrorCode = "xsd-xsi-type-derivation-blocked"
	// ErrValidateXsiNilNotNillable indicates xsi:nil used on non-nillable element.
	ErrValidateXsiNilNotNillable ErrorCode = "xsd-xsi-nil-not-nillable"
	// ErrValidateNilledHasFixed indicates a nilled element has a fixed value.
	ErrValidateNilledHasFixed ErrorCode = "xsd-nilled-has-fixed"
	// ErrValidateNilledNotEmpty indicates a nilled element has content.
	ErrValidateNilledNotEmpty ErrorCode = "xsd-nilled-not-empty"
	// ErrValidateWildcardElemStrictUnresolved indicates strict wildcard element unresolved.
	ErrValidateWildcardElemStrictUnresolved ErrorCode = "xsd-wildcard-elem-strict-unresolved"
	// ErrValidateWildcardAttrStrictUnresolved indicates strict wildcard attribute unresolved.
	ErrValidateWildcardAttrStrictUnresolved ErrorCode = "xsd-wildcard-attr-strict-unresolved"
	// ErrValidateRootNotDeclared indicates root element not declared.
	ErrValidateRootNotDeclared ErrorCode = "xsd-root-not-declared"
)

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
type ValidationList []Validation //nolint:errname // public API name, keep for compatibility.

// Error returns a compact summary of the validation errors.
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

// NewValidation builds a Validation with a code, message, and optional path.
func NewValidation(code ErrorCode, msg, path string) Validation {
	return Validation{Code: code, Message: msg, Path: path}
}

// AsValidations extracts validation errors from an error returned by validation helpers.
func AsValidations(err error) ([]Validation, bool) {
	list, ok := asValidationList(err)
	if !ok {
		return nil, false
	}
	cloned := slices.Clone(list)
	cloned.Sort()
	return []Validation(cloned), true
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

	return nil, false
}
