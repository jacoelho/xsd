package validator

import xsderrors "github.com/jacoelho/xsd/errors"

func (e *validationExecutor) finalize() error {
	if e == nil || e.s == nil {
		return nil
	}
	if !e.rootSeen {
		return xsderrors.ValidationList{e.s.newValidation(xsderrors.ErrNoRoot, "document has no root element", "", 0, 0)}
	}
	if len(e.s.elemStack) != 0 {
		return xsderrors.ValidationList{e.s.newValidation(xsderrors.ErrXMLParse, "document ended with unclosed elements", e.s.pathString(), 0, 0)}
	}
	if errs := e.s.validateIDRefs(); len(errs) > 0 {
		if fatal := e.s.recordValidationErrors(errs, 0, 0); fatal != nil {
			return fatal
		}
	}
	if errs := e.s.finalizeIdentity(); len(errs) > 0 {
		if fatal := e.s.recordValidationErrors(errs, 0, 0); fatal != nil {
			return fatal
		}
	}
	return e.s.validationList()
}
