package xmltext

import (
	"errors"
	"fmt"
)

var (
	errNilReader           = errors.New("nil XML reader")
	errUnexpectedEOF       = errors.New("unexpected EOF")
	errInvalidName         = errors.New("invalid XML name")
	errInvalidEntity       = errors.New("invalid entity reference")
	errInvalidCharRef      = errors.New("invalid character reference")
	errInvalidChar         = errors.New("invalid XML character")
	errInvalidToken        = errors.New("invalid XML token")
	errInvalidComment      = errors.New("invalid XML comment")
	errInvalidPI           = errors.New("invalid XML processing instruction")
	errUnsupportedEncoding = errors.New("unsupported encoding")
	errTokenTooLarge       = errors.New("token exceeds MaxTokenSize")
	errDepthLimit          = errors.New("element depth exceeds MaxDepth")
	errAttrLimit           = errors.New("attribute count exceeds MaxAttrs")
	errDuplicateAttr       = errors.New("duplicate attribute name")
	errMismatchedEndTag    = errors.New("mismatched end element")
	errMultipleRoots       = errors.New("multiple root elements")
	errContentOutsideRoot  = errors.New("content outside root element")
	errMissingRoot         = errors.New("missing root element")
	errMisplacedDirective  = errors.New("directive outside prolog")
	errDuplicateDirective  = errors.New("duplicate directive")
	errMisplacedXMLDecl    = errors.New("XML declaration not at start")
	errDuplicateXMLDecl    = errors.New("duplicate XML declaration")
)

// SyntaxError reports a well-formedness error with location context.
type SyntaxError struct {
	Offset  int64
	Line    int
	Column  int
	Path    Path
	Snippet []byte
	Err     error
}

// Error formats the syntax error with location and cause.
func (e *SyntaxError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Line > 0 && e.Column > 0 {
		return fmt.Sprintf("xml syntax error at line %d, column %d: %v", e.Line, e.Column, e.Err)
	}
	return fmt.Sprintf("xml syntax error at offset %d: %v", e.Offset, e.Err)
}

// Unwrap exposes the underlying error.
func (e *SyntaxError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
