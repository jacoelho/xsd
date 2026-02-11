package validator

import (
	"fmt"

	xsderrors "github.com/jacoelho/xsd/errors"
)

type modelMatchAccumulator struct {
	match StartMatch
	found bool
}

func (a *modelMatchAccumulator) add(match StartMatch, onAmbiguous func() error) error {
	if !a.found {
		a.match = match
		a.found = true
		return nil
	}
	if onAmbiguous == nil {
		return fmt.Errorf("ambiguous content model match")
	}
	return onAmbiguous()
}

func (a *modelMatchAccumulator) result() (StartMatch, error) {
	if !a.found {
		return StartMatch{}, noContentModelMatchError()
	}
	return a.match, nil
}

func noContentModelMatchError() error {
	return newValidationError(xsderrors.ErrUnexpectedElement, "no content model match")
}

func ambiguousContentModelMatchError() error {
	return newValidationError(xsderrors.ErrContentModelInvalid, "ambiguous content model match")
}
