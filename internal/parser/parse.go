package parser

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// Match pkg/xmlstream.readerBufferSize so xmlstream.NewReader can reuse
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

// ParseResult contains the parsed schema and import/include directives
type ParseResult struct {
	Schema     *Schema
	Directives []Directive
	Imports    []ImportInfo
	Includes   []IncludeInfo
}

// Parse parses an XSD schema from a reader
func Parse(r io.Reader) (*Schema, error) {
	result, err := ParseWithImportsOptions(r)
	if err != nil {
		return nil, err
	}
	return result.Schema, nil
}

// ParseWithImportsOptions parses an XSD schema with XML reader options.
func ParseWithImportsOptions(r io.Reader, opts ...xmlstream.Option) (*ParseResult, error) {
	return ParseWithImportsOptionsWithPool(r, NewDocumentPool(), opts...)
}

// ParseWithImportsOptionsWithPool parses an XSD schema with XML reader options and an explicit document pool.
func ParseWithImportsOptionsWithPool(r io.Reader, pool *DocumentPool, opts ...xmlstream.Option) (*ParseResult, error) {
	if pool == nil {
		pool = NewDocumentPool()
	}
	buffered := acquireParseReader(r)
	defer releaseParseReader(buffered)

	reader, err := xmlstream.NewReader(buffered, opts...)
	if err != nil {
		return nil, newParseError(fmt.Errorf("xml reader: %w", err))
	}
	session := newParseSession(reader, pool)
	if err := session.parse(); err != nil {
		return nil, err
	}
	return session.result, nil
}

type parseSession struct {
	reader             *xmlstream.Reader
	pool               *DocumentPool
	schema             *Schema
	result             *ParseResult
	importedNamespaces map[model.NamespaceURI]bool
	dirState           directiveState
	docState           value.DocumentState
}

func newParseSession(reader *xmlstream.Reader, pool *DocumentPool) *parseSession {
	schema := NewSchema()
	return &parseSession{
		reader: reader,
		pool:   pool,
		schema: schema,
		result: &ParseResult{
			Schema:     schema,
			Directives: []Directive{},
			Imports:    []ImportInfo{},
			Includes:   []IncludeInfo{},
		},
		importedNamespaces: make(map[model.NamespaceURI]bool),
		docState:           value.NewDocumentState(),
	}
}

func (s *parseSession) parse() error {
	for {
		ev, err := s.reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return newParseError(fmt.Errorf("xml read: %w", err))
		}
		if err := s.handleEvent(ev); err != nil {
			return err
		}
	}
	if !s.docState.RootSeen() {
		return wrapParseErr(fmt.Errorf("empty document"))
	}
	applyImportedNamespaces(s.schema, s.importedNamespaces)
	return nil
}

func (s *parseSession) handleEvent(ev xmlstream.Event) error {
	switch ev.Kind {
	case xmlstream.EventStartElement:
		return s.handleStartElement(ev)
	case xmlstream.EventEndElement:
		s.docState.OnEndElement(s.docState.RootSeen() && !s.docState.RootClosed())
	case xmlstream.EventCharData:
		if (!s.docState.RootSeen() || s.docState.RootClosed()) && !s.docState.ValidateOutsideCharData(ev.Text) {
			return newParseError(fmt.Errorf("unexpected character data outside root element"))
		}
	case xmlstream.EventComment, xmlstream.EventPI, xmlstream.EventDirective:
		if !s.docState.RootSeen() || s.docState.RootClosed() {
			s.docState.OnOutsideMarkup()
		}
	}
	return nil
}

func (s *parseSession) handleStartElement(ev xmlstream.Event) error {
	if !s.docState.StartElementAllowed() {
		return newParseError(fmt.Errorf("unexpected element %s after document end", ev.Name.Local))
	}
	rootSeen := s.docState.RootSeen()
	s.docState.OnStartElement()
	if !rootSeen {
		return s.handleSchemaRoot(ev)
	}
	if ev.Name.Namespace != value.XSDNamespace {
		if err := s.reader.SkipSubtree(); err != nil {
			return newParseError(fmt.Errorf("xml skip for element %s: %w", ev.Name.String(), err))
		}
		return nil
	}
	return s.handleTopLevelSubtree(ev)
}

func (s *parseSession) handleSchemaRoot(ev xmlstream.Event) error {
	if ev.Name.Local != "schema" || ev.Name.Namespace != value.XSDNamespace {
		return wrapParseErr(fmt.Errorf("root element must be xs:schema, got {%s}%s", ev.Name.Namespace, ev.Name.Local))
	}
	if err := s.parseSchemaAttributesFromStart(ev); err != nil {
		return wrapParseErr(err)
	}
	applyImportedNamespaces(s.schema, s.importedNamespaces)
	return nil
}

func (s *parseSession) handleTopLevelSubtree(ev xmlstream.Event) error {
	doc := s.pool.Acquire()
	if err := parseSubtreeInto(s.reader, ev, doc); err != nil {
		s.pool.Release(doc)
		return newParseError(fmt.Errorf("xml read for element %s: %w", ev.Name.String(), err))
	}
	root := doc.DocumentElement()
	if root == InvalidNode {
		s.pool.Release(doc)
		return newParseError(fmt.Errorf("xml read for element %s: %w", ev.Name.String(), io.ErrUnexpectedEOF))
	}
	defer s.pool.Release(doc)

	switch ev.Name.Local {
	case "annotation", "import", "include":
		if err := validateSchemaAttributeNamespaces(doc, root); err != nil {
			return wrapParseErr(err)
		}
		if err := s.parseDirectiveElement(doc); err != nil {
			return wrapParseErr(err)
		}
		applyImportedNamespaces(s.schema, s.importedNamespaces)
		return nil
	case "redefine":
		return wrapParseErr(fmt.Errorf("redefine is not supported"))
	case "element", "complexType", "simpleType", "group", "attribute", "attributeGroup", "notation":
		s.dirState.declIndex++
		fallthrough
	case "key", "keyref", "unique":
		if err := validateSchemaAttributeNamespaces(doc, root); err != nil {
			return wrapParseErr(err)
		}
		return wrapParseErr(parseTopLevelComponent(doc, root, s.schema))
	default:
		return wrapParseErr(fmt.Errorf("unexpected top-level element '%s'", ev.Name.Local))
	}
}

func (s *parseSession) parseSchemaAttributesFromStart(start xmlstream.Event) error {
	attrs := make([]schemaAttribute, 0, len(start.Attrs))
	for _, attr := range start.Attrs {
		if attr.Name.Namespace == value.XMLNSNamespace {
			continue
		}
		if attr.Name.Namespace == value.XSDNamespace {
			return fmt.Errorf("schema attribute '%s' on <schema> must be unprefixed", attr.Name.Local)
		}
		attrs = append(attrs, schemaAttribute{
			namespace: attr.Name.Namespace,
			local:     attr.Name.Local,
			value:     string(attr.Value),
		})
	}
	var nsDecls []schemaNamespaceDecl
	decls := s.reader.NamespaceDeclsSeq(start.ScopeDepth)
	if decls != nil {
		nsDecls = make([]schemaNamespaceDecl, 0, 8)
		for decl := range decls {
			nsDecls = append(nsDecls, schemaNamespaceDecl{
				prefix: decl.Prefix,
				uri:    decl.URI,
			})
		}
	}
	return applySchemaRootAttributes(s.schema, attrs, nsDecls)
}
