package validator

import (
	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func identityViolation(category runtime.ICCategory, msg string) error {
	switch category {
	case runtime.ICKey:
		return newValidationError(xsderrors.ErrIdentityAbsent, msg)
	case runtime.ICUnique:
		return newValidationError(xsderrors.ErrIdentityDuplicate, msg)
	case runtime.ICKeyRef:
		return newValidationError(xsderrors.ErrIdentityKeyRefFailed, msg)
	default:
		return newValidationError(xsderrors.ErrIdentityAbsent, msg)
	}
}
