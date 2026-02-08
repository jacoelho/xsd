package parser

import (
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

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

// newParseError creates a new ParseError with the schema-parse-error code
func newParseError(msg string, err error) *ParseError {
	return &ParseError{
		Code:    "schema-parse-error",
		Message: msg,
		Err:     err,
	}
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

// getNameAttr returns the name attribute value with whitespace trimmed.
// XSD attribute values should be normalized per XML spec, so we always trim.
func getNameAttr(doc *xsdxml.Document, elem xsdxml.NodeID) string {
	return types.TrimXMLWhitespace(doc.GetAttribute(elem, "name"))
}

// ParseResult contains the parsed schema and import/include directives
type ParseResult struct {
	Schema     *Schema
	Directives []Directive
	Imports    []ImportInfo
	Includes   []IncludeInfo
}

// Parse parses an XSD schema from a reader
func Parse(r io.Reader) (*Schema, error) {
	result, err := ParseWithImports(r)
	if err != nil {
		return nil, err
	}
	return result.Schema, nil
}

// ParseWithImports parses an XSD schema and returns import/include information
func ParseWithImports(r io.Reader) (*ParseResult, error) {
	return ParseWithImportsOptions(r)
}

// ParseWithImportsOptions parses an XSD schema with XML reader options.
func ParseWithImportsOptions(r io.Reader, opts ...xmlstream.Option) (*ParseResult, error) {
	doc := xsdxml.AcquireDocument()
	defer xsdxml.ReleaseDocument(doc)

	if err := xsdxml.ParseIntoWithOptions(r, doc, opts...); err != nil {
		return nil, newParseError("parse XML", err)
	}

	root := doc.DocumentElement()
	if root == xsdxml.InvalidNode {
		return nil, fmt.Errorf("empty document")
	}

	if doc.LocalName(root) != "schema" || doc.NamespaceURI(root) != xsdxml.XSDNamespace {
		return nil, fmt.Errorf("root element must be xs:schema, got {%s}%s",
			doc.NamespaceURI(root), doc.LocalName(root))
	}

	schema := NewSchema()
	if err := parseSchemaAttributes(doc, root, schema); err != nil {
		return nil, err
	}

	result := &ParseResult{
		Schema:     schema,
		Directives: []Directive{},
		Imports:    []ImportInfo{},
		Includes:   []IncludeInfo{},
	}

	importedNamespaces, err := parseDirectives(doc, root, schema, result)
	if err != nil {
		return nil, err
	}
	applyImportedNamespaces(schema, importedNamespaces)
	if err := parseComponents(doc, root, schema); err != nil {
		return nil, err
	}

	return result, nil
}
