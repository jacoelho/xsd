package parser

import (
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmllex"
	"github.com/jacoelho/xsd/internal/xmlnames"
	"github.com/jacoelho/xsd/internal/xmltree"
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
	return ParseWithImportsOptionsWithPool(r, xmltree.NewDocumentPool(), opts...)
}

// ParseWithImportsOptionsWithPool parses an XSD schema with XML reader options and an explicit document pool.
func ParseWithImportsOptionsWithPool(r io.Reader, pool *xmltree.DocumentPool, opts ...xmlstream.Option) (*ParseResult, error) {
	if pool == nil {
		pool = xmltree.NewDocumentPool()
	}
	reader, err := xmlstream.NewReader(r, opts...)
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
	pool               *xmltree.DocumentPool
	schema             *Schema
	result             *ParseResult
	importedNamespaces map[model.NamespaceURI]bool
	dirState           directiveState
	docState           xmllex.DocumentState
}

func newParseSession(reader *xmlstream.Reader, pool *xmltree.DocumentPool) *parseSession {
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
		docState:           xmllex.NewDocumentState(),
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
		if err := s.handleCharData(ev); err != nil {
			return err
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
	if ev.Name.Namespace != xmlnames.XSDNamespace {
		return s.skipForeignSubtree(ev)
	}
	return s.handleTopLevelSubtree(ev)
}

func (s *parseSession) handleSchemaRoot(ev xmlstream.Event) error {
	if ev.Name.Local != "schema" || ev.Name.Namespace != xmlnames.XSDNamespace {
		return wrapParseErr(fmt.Errorf("root element must be xs:schema, got {%s}%s", ev.Name.Namespace, ev.Name.Local))
	}
	if err := parseSchemaAttributesFromStart(ev, s.reader.NamespaceDeclsSeq(ev.ScopeDepth), s.schema); err != nil {
		return wrapParseErr(err)
	}
	applyImportedNamespaces(s.schema, s.importedNamespaces)
	return nil
}

func (s *parseSession) skipForeignSubtree(ev xmlstream.Event) error {
	if err := s.reader.SkipSubtree(); err != nil {
		return newParseError(fmt.Errorf("xml skip for element %s: %w", ev.Name.String(), err))
	}
	return nil
}

func (s *parseSession) handleTopLevelSubtree(ev xmlstream.Event) error {
	doc, root, err := parseSubtreeIntoDoc(s.reader, ev, s.pool)
	if err != nil {
		return newParseError(fmt.Errorf("xml read for element %s: %w", ev.Name.String(), err))
	}

	switch ev.Name.Local {
	case "annotation", "import", "include":
		if err := parseDirectiveSubtree(doc, root, s.schema, s.result, s.importedNamespaces, &s.dirState, s.pool); err != nil {
			return wrapParseErr(err)
		}
		applyImportedNamespaces(s.schema, s.importedNamespaces)
		return nil
	case "redefine":
		s.pool.Release(doc)
		return wrapParseErr(fmt.Errorf("redefine is not supported"))
	default:
		if !isTopLevelComponentElement(ev.Name.Local) {
			s.pool.Release(doc)
			return wrapParseErr(fmt.Errorf("unexpected top-level element '%s'", ev.Name.Local))
		}
		if isGlobalDeclElement(ev.Name.Local) {
			s.dirState.declIndex++
		}
		return wrapParseErr(parseTopLevelComponentSubtree(doc, root, s.schema, s.pool))
	}
}

func (s *parseSession) handleCharData(ev xmlstream.Event) error {
	if s.docState.RootSeen() && !s.docState.RootClosed() {
		return nil
	}
	if !s.docState.ValidateOutsideCharData(ev.Text) {
		return newParseError(fmt.Errorf("unexpected character data outside root element"))
	}
	return nil
}

func parseSubtreeIntoDoc(reader *xmlstream.Reader, start xmlstream.Event, pool *xmltree.DocumentPool) (*xmltree.Document, xmltree.NodeID, error) {
	doc := pool.Acquire()
	if err := xmltree.ParseSubtreeInto(reader, start, doc); err != nil {
		pool.Release(doc)
		return nil, xmltree.InvalidNode, err
	}
	root := doc.DocumentElement()
	if root == xmltree.InvalidNode {
		pool.Release(doc)
		return nil, xmltree.InvalidNode, io.ErrUnexpectedEOF
	}
	return doc, root, nil
}

func parseDirectiveSubtree(doc *xmltree.Document, root xmltree.NodeID, schema *Schema, result *ParseResult, importedNamespaces map[model.NamespaceURI]bool, state *directiveState, pool *xmltree.DocumentPool) error {
	defer pool.Release(doc)
	if err := validateSchemaAttributeNamespaces(doc, root); err != nil {
		return err
	}
	return parseDirectiveElement(doc, root, schema, result, importedNamespaces, state)
}

func parseTopLevelComponentSubtree(doc *xmltree.Document, root xmltree.NodeID, schema *Schema, pool *xmltree.DocumentPool) error {
	defer pool.Release(doc)
	if err := validateSchemaAttributeNamespaces(doc, root); err != nil {
		return err
	}
	return parseTopLevelComponent(doc, root, schema)
}

func isTopLevelComponentElement(localName string) bool {
	switch localName {
	case "element", "complexType", "simpleType", "group", "attribute", "attributeGroup", "notation", "key", "keyref", "unique":
		return true
	default:
		return false
	}
}

func parseSchemaAttributesFromStart(start xmlstream.Event, decls iter.Seq[xmlstream.NamespaceDecl], schema *Schema) error {
	if err := validateSchemaStartAttributeNamespaces(start); err != nil {
		return err
	}
	attrs := make([]schemaAttribute, 0, len(start.Attrs))
	for _, attr := range start.Attrs {
		attrs = append(attrs, schemaAttribute{
			namespace: attr.Name.Namespace,
			local:     attr.Name.Local,
			value:     string(attr.Value),
		})
	}
	var nsDecls []schemaNamespaceDecl
	if decls != nil {
		nsDecls = make([]schemaNamespaceDecl, 0, 8)
		for decl := range decls {
			nsDecls = append(nsDecls, schemaNamespaceDecl{
				prefix: decl.Prefix,
				uri:    decl.URI,
			})
		}
	}
	return applySchemaRootAttributes(schema, attrs, nsDecls)
}

func validateSchemaStartAttributeNamespaces(start xmlstream.Event) error {
	for _, attr := range start.Attrs {
		if attr.Name.Namespace == xmlnames.XMLNSNamespace {
			continue
		}
		if attr.Name.Namespace == xmlnames.XSDNamespace {
			return fmt.Errorf("schema attribute '%s' on <schema> must be unprefixed", attr.Name.Local)
		}
	}
	return nil
}
