package validate

import (
	"errors"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/xsderrors"
)

func instanceReaderError(err error) error {
	switch {
	case errors.Is(err, stream.ErrXMLInputNilReader):
		return xsderrors.Validation(xsderrors.CodeValidationXML, 0, 0, "", "instance reader is nil")
	case errors.Is(err, stream.ErrUnsupportedNonUTF8):
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, "instance documents must be UTF-8")
	default:
		var versionErr stream.UnsupportedXMLVersionError
		if errors.As(err, &versionErr) {
			return xsderrors.Unsupported(xsderrors.CodeUnsupportedXML11, versionErr.Error())
		}
		return err
	}
}

// StreamError classifies parser errors as validation diagnostics.
func StreamError(line, col int, path string, err error) error {
	if errors.Is(err, stream.ErrUnsupportedNonUTF8) {
		return xsderrors.UnsupportedAt(xsderrors.CodeUnsupportedNonUTF8, line, col, path, "instance documents must be UTF-8", err)
	}
	var versionErr stream.UnsupportedXMLVersionError
	if errors.As(err, &versionErr) {
		return xsderrors.UnsupportedAt(xsderrors.CodeUnsupportedXML11, line, col, path, versionErr.Error(), err)
	}
	if stream.IsTokenLimit(err) || stream.IsAttributeLimit(err) {
		return xsderrors.Validation(xsderrors.CodeValidationLimit, line, col, path, err.Error())
	}
	if stream.IsUnsupportedEntityReference(err) {
		return xsderrors.UnsupportedAt(xsderrors.CodeUnsupportedExternal, line, col, path, "external or undeclared entity resolution is not supported", err)
	}
	return xsderrors.Validation(xsderrors.CodeValidationXML, line, col, path, err.Error())
}

// ValidateDirective rejects instance markup declarations. The stream parser
// only returns KindDirective for DOCTYPE declarations.
func ValidateDirective(ctx StartContext, _ []byte) error {
	return xsderrors.UnsupportedAt(xsderrors.CodeUnsupportedDTD, ctx.Line, ctx.Column, ctx.PathString(), "DTD declarations are not supported", nil)
}
