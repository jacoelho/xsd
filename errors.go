package xsd

import "github.com/jacoelho/xsd/internal/xsderrors"

type ErrorKind = xsderrors.ErrorKind

const (
	KindCaller     = xsderrors.KindCaller
	KindSchema     = xsderrors.KindSchema
	KindValidation = xsderrors.KindValidation
	KindIO         = xsderrors.KindIO
	KindInternal   = xsderrors.KindInternal
)

type ErrorCode = xsderrors.ErrorCode
type Error = xsderrors.Error
type Validation = xsderrors.Validation
type ValidationList = xsderrors.ValidationList

const (
	ErrNoRoot          ErrorCode = xsderrors.ErrNoRoot
	ErrSchemaNotLoaded ErrorCode = xsderrors.ErrSchemaNotLoaded
	ErrXMLParse        ErrorCode = xsderrors.ErrXMLParse

	ErrElementNotDeclared       ErrorCode = xsderrors.ErrElementNotDeclared
	ErrElementAbstract          ErrorCode = xsderrors.ErrElementAbstract
	ErrElementNotNillable       ErrorCode = xsderrors.ErrElementNotNillable
	ErrNilElementNotEmpty       ErrorCode = xsderrors.ErrNilElementNotEmpty
	ErrXsiTypeInvalid           ErrorCode = xsderrors.ErrXsiTypeInvalid
	ErrElementTypeAbstract      ErrorCode = xsderrors.ErrElementTypeAbstract
	ErrElementFixedValue        ErrorCode = xsderrors.ErrElementFixedValue
	ErrTextInElementOnly        ErrorCode = xsderrors.ErrTextInElementOnly
	ErrContentModelInvalid      ErrorCode = xsderrors.ErrContentModelInvalid
	ErrRequiredElementMissing   ErrorCode = xsderrors.ErrRequiredElementMissing
	ErrUnexpectedElement        ErrorCode = xsderrors.ErrUnexpectedElement
	ErrAttributeNotDeclared     ErrorCode = xsderrors.ErrAttributeNotDeclared
	ErrAttributeProhibited      ErrorCode = xsderrors.ErrAttributeProhibited
	ErrRequiredAttributeMissing ErrorCode = xsderrors.ErrRequiredAttributeMissing
	ErrAttributeFixedValue      ErrorCode = xsderrors.ErrAttributeFixedValue
	ErrWildcardNotDeclared      ErrorCode = xsderrors.ErrWildcardNotDeclared
	ErrDatatypeInvalid          ErrorCode = xsderrors.ErrDatatypeInvalid
	ErrFacetViolation           ErrorCode = xsderrors.ErrFacetViolation
	ErrDuplicateID              ErrorCode = xsderrors.ErrDuplicateID
	ErrIDRefNotFound            ErrorCode = xsderrors.ErrIDRefNotFound
	ErrMultipleIDAttr           ErrorCode = xsderrors.ErrMultipleIDAttr
	ErrIdentityDuplicate        ErrorCode = xsderrors.ErrIdentityDuplicate
	ErrIdentityAbsent           ErrorCode = xsderrors.ErrIdentityAbsent
	ErrIdentityKeyRefFailed     ErrorCode = xsderrors.ErrIdentityKeyRefFailed

	ErrValidateValueInvalid                 ErrorCode = xsderrors.ErrValidateValueInvalid
	ErrValidateValueFacet                   ErrorCode = xsderrors.ErrValidateValueFacet
	ErrValidateElementAbstract              ErrorCode = xsderrors.ErrValidateElementAbstract
	ErrValidateSimpleTypeAttrNotAllowed     ErrorCode = xsderrors.ErrValidateSimpleTypeAttrNotAllowed
	ErrValidateXsiTypeUnresolved            ErrorCode = xsderrors.ErrValidateXsiTypeUnresolved
	ErrValidateXsiTypeDerivationBlocked     ErrorCode = xsderrors.ErrValidateXsiTypeDerivationBlocked
	ErrValidateXsiNilNotNillable            ErrorCode = xsderrors.ErrValidateXsiNilNotNillable
	ErrValidateNilledHasFixed               ErrorCode = xsderrors.ErrValidateNilledHasFixed
	ErrValidateNilledNotEmpty               ErrorCode = xsderrors.ErrValidateNilledNotEmpty
	ErrValidateWildcardElemStrictUnresolved ErrorCode = xsderrors.ErrValidateWildcardElemStrictUnresolved
	ErrValidateWildcardAttrStrictUnresolved ErrorCode = xsderrors.ErrValidateWildcardAttrStrictUnresolved
	ErrValidateRootNotDeclared              ErrorCode = xsderrors.ErrValidateRootNotDeclared
)

func AsValidations(err error) ([]Validation, bool) {
	return xsderrors.AsValidations(err)
}

func KindOf(err error) (ErrorKind, bool) {
	return xsderrors.KindOf(err)
}
