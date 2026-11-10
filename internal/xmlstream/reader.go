package xmlstream

import (
	"bytes"
	"errors"

	"github.com/jacoelho/xsd/internal/diagnostictext"
)

const maxXMLDeclarationPreviewBytes = xmlInputBufferSize

// ErrXMLInputNilReader reports a nil XML input reader.
var ErrXMLInputNilReader = errors.New("xml input reader is nil")

// inputError marks a failure of the caller's io.Reader, as opposed to a
// failure of the document. readUnderlying wraps every reader error other than
// io.EOF itself. It is the only way a reader error is classified as
// an input failure.
type inputError struct{ cause error }

func (e *inputError) Error() string { return e.cause.Error() }
func (e *inputError) Unwrap() error { return e.cause }

// IsInputFailure reports whether err is a failure of the caller's io.Reader
// (a read error, no progress, or a nil reader) rather than of the document.
// It reports false when LimitOf or UnsupportedFeatureOf classifies err. Those
// two checks matter only for this package's own limit or unsupported error
// joined with a reader failure, which keeps its precedence; a caller's reader
// error is never classified by them. It also reports false for io.EOF itself;
// a reader error that wraps or joins io.EOF is marked by readUnderlying like
// any other reader error and reports true. Error.Kind is ErrorInput exactly
// when it reports true.
func IsInputFailure(err error) bool {
	if _, ok := UnsupportedFeatureOf(err); ok {
		return false
	}
	if _, ok := LimitOf(err); ok {
		return false
	}
	_, ok := errors.AsType[*inputError](err)
	return ok
}

// originates reports whether match holds for err or an error it wraps,
// without descending into an *inputError: what the caller's reader
// returned is never classified as this package's own failure. It walks
// Unwrap() error and Unwrap() []error only and deliberately ignores Is/As
// methods on intermediate errors: this package's sentinels are plain
// values it wraps itself.
func originates(err error, match func(error) bool) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*inputError); ok { //nolint:errorlint // origin boundary, inspected per node
		return false
	}
	if match(err) {
		return true
	}
	switch x := err.(type) { //nolint:errorlint // origin walk inspects each node
	case interface{ Unwrap() error }:
		return originates(x.Unwrap(), match)
	case interface{ Unwrap() []error }:
		for _, e := range x.Unwrap() {
			if originates(e, match) {
				return true
			}
		}
	}
	return false
}

func originatesSentinel(err, target error) bool {
	return originates(err, func(e error) bool { return e == target }) //nolint:errorlint // sentinel identity
}

func errNilReader() error { return &inputError{cause: ErrXMLInputNilReader} }

// errUnsupportedNonUTF8 reports XML input that declares or uses a non-UTF-8 encoding.
var errUnsupportedNonUTF8 = errors.New("xml input must be UTF-8")

// unsupportedXMLVersionError reports an XML version this tokenizer does not
// support. version is already bounded and escaped by diagnostictext, so
// every consumer of Error() inherits the bound.
type unsupportedXMLVersionError struct{ version string }

func newUnsupportedXMLVersionError(version string) unsupportedXMLVersionError {
	return unsupportedXMLVersionError{version: string(diagnostictext.AppendEscaped(nil, version))}
}

func (e unsupportedXMLVersionError) Error() string {
	return "XML version " + e.version + " is not supported"
}

func (p *parser) prepareXMLProlog() error {
	peek, err := p.initialXMLBytes()
	if err != nil {
		return err
	}
	if startsUTF16(peek) {
		return errUnsupportedNonUTF8
	}
	if !StartsXMLDeclaration(peek) {
		return nil
	}
	content, complete := bytes.CutSuffix(p.peekXMLDeclaration()[len("<?xml"):], []byte("?>"))
	if !complete {
		return nil
	}
	// Reset reports only version and encoding rejections early; grammar errors
	// are left to the parser so they carry a position. Admission stops at the
	// first defect in document order, so both paths classify a declaration alike.
	err = ValidateXMLDeclContent(content)
	if err == nil {
		return nil
	}
	var version unsupportedXMLVersionError
	if errors.Is(err, errUnsupportedNonUTF8) || errors.As(err, &version) {
		return err
	}
	return nil
}

func (p *parser) initialXMLBytes() ([]byte, error) {
	peek, err := p.br.ensure(XMLDeclarationPrefixLen)
	if err != nil && !IsEOF(err) {
		return nil, err
	}
	if !HasUTF8BOM(peek) {
		return peek, nil
	}
	p.br.discardUTF8BOM()
	peek, err = p.br.ensure(XMLDeclarationPrefixLen)
	if err != nil && !IsEOF(err) {
		return nil, err
	}
	return peek, nil
}

func startsUTF16(peek []byte) bool {
	return len(peek) >= 2 &&
		(peek[0] == 0xFE && peek[1] == 0xFF || peek[0] == 0xFF && peek[1] == 0xFE)
}

func (p *parser) peekXMLDeclaration() []byte {
	n := XMLDeclarationPrefixLen
	for {
		peek, err := p.br.ensure(n)
		if end := bytes.Index(peek, []byte("?>")); end >= 0 {
			return peek[:end+2]
		}
		if err != nil || n == maxXMLDeclarationPreviewBytes {
			return peek
		}
		n *= 2
		if n > maxXMLDeclarationPreviewBytes {
			n = maxXMLDeclarationPreviewBytes
		}
	}
}
