package validate

import (
	"errors"

	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/xsderrors"
)

func instanceReaderError(err error) error {
	switch {
	case errors.Is(err, stream.ErrXMLInputNilReader):
		return xsderrors.Validation(xsderrors.CodeValidationXML, "instance reader is nil", nil)
	case errors.Is(err, stream.ErrUnsupportedNonUTF8):
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, "instance documents must be UTF-8", err)
	case stream.IsInputLimit(err) || stream.IsTokenLimit(err) || stream.IsAttributeLimit(err):
		return validationReaderCause(xsderrors.CodeValidationLimit, 0, 0, "", err)
	default:
		if versionErr, ok := errors.AsType[stream.UnsupportedXMLVersionError](err); ok {
			return xsderrors.Unsupported(xsderrors.CodeUnsupportedXML11, versionErr.Error(), nil)
		}
		return validationReaderCause(xsderrors.CodeValidationXML, 0, 0, "", err)
	}
}

// StreamError classifies parser errors as validation diagnostics.
func StreamError(line, col int, path string, err error) error {
	if errors.Is(err, stream.ErrUnsupportedNonUTF8) {
		return xsderrors.WithLocation(path, line, col, xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, "instance documents must be UTF-8", err))
	}
	if versionErr, ok := errors.AsType[stream.UnsupportedXMLVersionError](err); ok {
		return xsderrors.WithLocation(path, line, col, xsderrors.Unsupported(xsderrors.CodeUnsupportedXML11, versionErr.Error(), nil))
	}
	if stream.IsInputLimit(err) || stream.IsTokenLimit(err) || stream.IsAttributeLimit(err) {
		return validationReaderCause(xsderrors.CodeValidationLimit, line, col, path, err)
	}
	if stream.IsUnsupportedEntityReference(err) {
		return xsderrors.WithLocation(path, line, col, xsderrors.Unsupported(xsderrors.CodeUnsupportedExternal, "external or undeclared entity resolution is not supported", err))
	}
	return validationReaderCause(xsderrors.CodeValidationXML, line, col, path, err)
}

func validationReaderCause(code xsderrors.Code, line, col int, path string, err error) error {
	return xsderrors.WithLocation(path, line, col, xsderrors.Validation(code, "", err))
}

// ValidateDirective rejects instance markup declarations. The stream parser
// only returns KindDirective for DOCTYPE declarations.
func ValidateDirective(ctx StartContext, _ []byte) error {
	return xsderrors.WithLocation(ctx.PathString(), ctx.Line, ctx.Column,
		xsderrors.Unsupported(xsderrors.CodeUnsupportedDTD, "DTD declarations are not supported", nil))
}
