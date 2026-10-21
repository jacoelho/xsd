package validate

import (
	"errors"

	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

func instanceReaderError(err error) error {
	switch {
	case errors.Is(err, xmlstream.ErrXMLInputNilReader):
		return xsderrors.Validation(xsderrors.CodeValidationXML, msgReaderNil, nil)
	case errors.Is(err, xmlstream.ErrUnsupportedNonUTF8):
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, msgInstanceUTF8, err)
	case xmlstream.IsInputLimit(err) || xmlstream.IsTokenLimit(err) || xmlstream.IsAttributeLimit(err):
		return validationReaderCause(xsderrors.CodeValidationLimit, 0, 0, "", err)
	default:
		if versionErr, ok := errors.AsType[xmlstream.UnsupportedXMLVersionError](err); ok {
			return xsderrors.Unsupported(xsderrors.CodeUnsupportedXML11, versionErr.Error(), nil)
		}
		return validationReaderCause(xsderrors.CodeValidationXML, 0, 0, "", err)
	}
}

// StreamError classifies parser errors as validation diagnostics.
func StreamError(line, col int, path string, err error) error {
	if errors.Is(err, xmlstream.ErrUnsupportedNonUTF8) {
		return xsderrors.WithLocation(path, line, col, xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, msgInstanceUTF8, err))
	}
	if versionErr, ok := errors.AsType[xmlstream.UnsupportedXMLVersionError](err); ok {
		return xsderrors.WithLocation(path, line, col, xsderrors.Unsupported(xsderrors.CodeUnsupportedXML11, versionErr.Error(), nil))
	}
	if xmlstream.IsInputLimit(err) || xmlstream.IsTokenLimit(err) || xmlstream.IsAttributeLimit(err) {
		return validationReaderCause(xsderrors.CodeValidationLimit, line, col, path, err)
	}
	if xmlstream.IsUnsupportedEntityReference(err) {
		return xsderrors.WithLocation(path, line, col, xsderrors.Unsupported(xsderrors.CodeUnsupportedExternal, msgExternalEntity, err))
	}
	if errors.Is(err, xmlstream.ErrUnsupportedDTD) {
		return xsderrors.WithLocation(path, line, col, xsderrors.Unsupported(xsderrors.CodeUnsupportedDTD, "", err))
	}
	if errors.Is(err, xmlstream.ErrTextOutsideRoot) {
		return xsderrors.WithLocation(path, line, col, xsderrors.Validation(xsderrors.CodeValidationXML, msgTextOutsideRoot, err))
	}
	if errors.Is(err, xmlstream.ErrCDATOutsideRoot) {
		return xsderrors.WithLocation(path, line, col, xsderrors.Validation(xsderrors.CodeValidationXML, msgCDATAOutsideRoot, err))
	}
	if errors.Is(err, xmlstream.ErrReferenceOutsideRoot) {
		return xsderrors.WithLocation(path, line, col, xsderrors.Validation(xsderrors.CodeValidationXML, msgReferenceOutsideRoot, err))
	}
	return validationReaderCause(xsderrors.CodeValidationXML, line, col, path, err)
}

func streamErrorPosition(reader *xmlstream.Reader, err error) (line, column int) {
	if boundary, ok := errors.AsType[*xmlstream.Error](err); ok && boundary != nil && boundary.Line > 0 {
		return boundary.Line, boundary.Column
	}
	return reader.Pos()
}

func validationReaderCause(code xsderrors.Code, line, col int, path string, err error) error {
	return xsderrors.WithLocation(path, line, col, xsderrors.Validation(code, "", err))
}
