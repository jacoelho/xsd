package compile

import (
	"context"
	"errors"

	"github.com/jacoelho/xsd/xsderrors"
)

func compileContextError(ctx context.Context) error {
	return compileContextErrorWith(ctx, nil)
}

func compileContextErrorWith(ctx context.Context, observed error) error {
	if ctx == nil {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaRead, "context is nil")
	}
	if cause := context.Cause(ctx); cause != nil {
		if observed != nil && !errors.Is(observed, cause) {
			cause = errors.Join(cause, observed)
		} else if observed != nil {
			cause = observed
		}
		return xsderrors.Canceled(xsderrors.CodeCompileCanceled, "schema compilation canceled", cause)
	}
	return nil
}
