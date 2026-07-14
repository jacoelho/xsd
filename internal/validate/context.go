package validate

import (
	"context"
	"errors"

	"github.com/jacoelho/xsd/xsderrors"
)

func validationContextError(ctx context.Context) error {
	if ctx == nil {
		return xsderrors.Validation(xsderrors.CodeValidationOption, 0, 0, "", "context is nil")
	}
	return validationContextDoneError(ctx, ctx.Done(), nil)
}

func validationContextDoneError(ctx context.Context, done <-chan struct{}, observed error) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
	default:
		return nil
	}
	if cause := context.Cause(ctx); cause != nil {
		if observed != nil && !errors.Is(observed, cause) {
			cause = errors.Join(cause, observed)
		} else if observed != nil {
			cause = observed
		}
		return xsderrors.Canceled(xsderrors.CodeValidationCanceled, "validation canceled", cause)
	}
	return nil
}
