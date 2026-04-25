package schemaast

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sync"
)

// Match internal/xmlstream.readerBufferSize so xmlstream.NewReader can reuse
// the buffered reader instead of allocating a second one per schema parse.
const parseReaderBufferSize = 256 * 1024

type emptySource struct{}

func (emptySource) Read([]byte) (int, error) {
	return 0, io.EOF
}

var parseReaderPool = sync.Pool{
	New: func() any {
		return bufio.NewReaderSize(emptySource{}, parseReaderBufferSize)
	},
}

func acquireParseReader(src io.Reader) *bufio.Reader {
	reader := parseReaderPool.Get().(*bufio.Reader)
	reader.Reset(src)
	return reader
}

func releaseParseReader(reader *bufio.Reader) {
	if reader == nil {
		return
	}
	reader.Reset(emptySource{})
	parseReaderPool.Put(reader)
}

// ParseError represents a schema parsing error with an error code
type ParseError struct {
	Err     error
	Code    string
	Message string
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error, if any.
func (e *ParseError) Unwrap() error {
	return e.Err
}

// newParseError creates a new ParseError with the schema-parse-error code.
func newParseError(err error) *ParseError {
	return &ParseError{
		Code:    "schema-parse-error",
		Message: "parse XML",
		Err:     err,
	}
}

func wrapParseErr(err error) error {
	if err == nil {
		return nil
	}
	var parseErr *ParseError
	if errors.As(err, &parseErr) {
		return err
	}
	return newParseError(err)
}

// ImportInfo represents an import directive from an XSD schema.
// Imports allow referencing components from a different namespace.
type ImportInfo struct {
	Namespace      string
	SchemaLocation string
}

// IncludeInfo represents an include directive from an XSD schema.
// Includes allow referencing components from the same namespace or no namespace.
type IncludeInfo struct {
	SchemaLocation string
	DeclIndex      int
	IncludeIndex   int
}

// DirectiveKind represents an include/import directive in document order.
type DirectiveKind uint8

const (
	DirectiveInclude DirectiveKind = iota
	DirectiveImport
)

// Directive preserves the document order for include/import directives.
type Directive struct {
	Import  ImportInfo
	Include IncludeInfo
	Kind    DirectiveKind
}

// ParseResult contains the parsed schema document and import/include directives.
type ParseResult struct {
	Document   *SchemaDocument
	Directives []Directive
	Imports    []ImportInfo
	Includes   []IncludeInfo
}
