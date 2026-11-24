package validate

import (
	"errors"
	"io"
	"strconv"

	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

// streamError is the single mapping from xmlstream reader errors to instance
// diagnostics. It classifies by origin, so a caller reader's error is never
// taken for this package's own failure whatever its chain carries. Order:
// reader limits ('<option>=<n> exceeded' at ctx), unsupported features,
// caller-reader failures (validation.read, including a nil reader; exclusive),
// outside-root sentinels, and finally well-formedness failures.
// resetInstanceReader locates prologue failures at path "/" and the reader
// position (line 1 column 1 when none); a nil reader stays unlocated.
func streamError(ctx startContext, err error, limits Limits) error {
	line, col, path := ctx.Line, ctx.Column, ctx.PathString()
	if limit, ok := xmlstream.LimitOf(err); ok {
		option, configured := readerLimitOption(limit, limits)
		return xsderrors.WithLocation(path, line, col, xsderrors.Validation(xsderrors.CodeValidationLimit, option+"="+strconv.FormatInt(configured, 10)+" exceeded", err))
	}
	if feature, ok := xmlstream.UnsupportedFeatureOf(err); ok {
		return xsderrors.WithLocation(path, line, col, unsupportedInstanceFeature(feature, err))
	}
	if xmlstream.IsInputFailure(err) {
		if errors.Is(err, xmlstream.ErrXMLInputNilReader) {
			return xsderrors.Validation(xsderrors.CodeValidationRead, msgReaderNil, nil)
		}
		return xsderrors.WithLocation(path, line, col, xsderrors.Validation(xsderrors.CodeValidationRead, "read instance document", err))
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

func unsupportedInstanceFeature(feature xmlstream.UnsupportedFeature, err error) error {
	switch feature {
	case xmlstream.UnsupportedDTD:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedDTD, "", err)
	case xmlstream.UnsupportedNonUTF8:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, msgInstanceUTF8, err)
	case xmlstream.UnsupportedXMLVersion:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedXMLVersion, err.Error(), nil)
	case xmlstream.UnsupportedEntityReference:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedExternal, msgExternalEntity, err)
	}
	return xsderrors.InternalInvariant("unknown xmlstream unsupported feature")
}

func validationReaderCause(code xsderrors.Code, line, col int, path string, err error) error {
	return xsderrors.WithLocation(path, line, col, xsderrors.Validation(code, "", err))
}

// resetInstanceReader starts reader on one instance document under the
// instance limits. Session validation and the schema-free well-formedness check
// both start here, so they enforce the same reader limits and classify reset
// failures the same way.
func resetInstanceReader(reader *xmlstream.Reader, r io.Reader, limits Limits) error {
	if err := reader.Reset(r, limits.readerConfig()); err != nil {
		// A failure while reading the document prologue precedes any element,
		// so it is located at the document root path.
		line, col := reader.ErrorPos(err)
		return streamError(startContext{Path: "/", Line: line, Column: col}, err, limits)
	}
	return nil
}
