package validate

import (
	"errors"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func simpleValueMetadataInvariant(err error) error {
	if errors.Is(err, runtime.ErrSimpleValueMetadata) {
		return xsderrors.InternalInvariant("simple value metadata is invalid")
	}
	return nil
}
