package parser

import (
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmllex"
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
	reader, err := xmlstream.NewReader(r, opts...)
	if err != nil {
		return nil, newParseError(fmt.Errorf("xml reader: %w", err))
	}
	schema := NewSchema()
	result := &ParseResult{
		Schema:     schema,
		Directives: []Directive{},
		Imports:    []ImportInfo{},
		Includes:   []IncludeInfo{},
	}
	importedNamespaces := make(map[model.NamespaceURI]bool)
	dirState := directiveState{}
	docState := xmllex.NewDocumentState()

	for {
		ev, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, newParseError(fmt.Errorf("xml read: %w", err))
		}

		switch ev.Kind {
		case xmlstream.EventStartElement:
			if !docState.StartElementAllowed() {
				return nil, newParseError(fmt.Errorf("unexpected element %s after document end", ev.Name.Local))
			}
			rootSeen := docState.RootSeen()
			docState.OnStartElement()
			if !rootSeen {
				if ev.Name.Local != "schema" || ev.Name.Namespace != xmltree.XSDNamespace {
					return nil, fmt.Errorf("root element must be xs:schema, got {%s}%s", ev.Name.Namespace, ev.Name.Local)
				}
				if err := parseSchemaAttributesFromStart(ev, reader.NamespaceDeclsSeq(ev.ScopeDepth), schema); err != nil {
					return nil, err
				}
				applyImportedNamespaces(schema, importedNamespaces)
				continue
			}
			if ev.Name.Namespace != xmltree.XSDNamespace {
				if err := reader.SkipSubtree(); err != nil {
					return nil, newParseError(fmt.Errorf("xml skip for element %s: %w", ev.Name.String(), err))
				}
				continue
			}

			doc, root, err := parseSubtreeIntoDoc(reader, ev, pool)
			if err != nil {
				return nil, newParseError(fmt.Errorf("xml read for element %s: %w", ev.Name.String(), err))
			}

			switch ev.Name.Local {
			case "annotation", "import", "include":
				if err := parseDirectiveSubtree(doc, root, schema, result, importedNamespaces, &dirState, pool); err != nil {
					return nil, err
				}
				applyImportedNamespaces(schema, importedNamespaces)
			case "redefine":
				return nil, fmt.Errorf("redefine is not supported")
			default:
				if !isTopLevelComponentElement(ev.Name.Local) {
					pool.Release(doc)
					return nil, fmt.Errorf("unexpected top-level element '%s'", ev.Name.Local)
				}
				if isGlobalDeclElement(ev.Name.Local) {
					dirState.declIndex++
				}
				if err := parseTopLevelComponentSubtree(doc, root, schema, pool); err != nil {
					return nil, err
				}
			}

		case xmlstream.EventEndElement:
			docState.OnEndElement(docState.RootSeen() && !docState.RootClosed())
		case xmlstream.EventCharData:
			if !docState.RootSeen() || docState.RootClosed() {
				if !docState.ValidateOutsideCharData(ev.Text) {
					return nil, newParseError(fmt.Errorf("unexpected character data outside root element"))
				}
			}
		case xmlstream.EventComment, xmlstream.EventPI, xmlstream.EventDirective:
			if !docState.RootSeen() || docState.RootClosed() {
				docState.OnOutsideMarkup()
			}
		}
	}

	if !docState.RootSeen() {
		return nil, fmt.Errorf("empty document")
	}

	applyImportedNamespaces(schema, importedNamespaces)
	return result, nil
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
		if attr.Name.Namespace == xmltree.XMLNSNamespace {
			continue
		}
		if attr.Name.Namespace == xmltree.XSDNamespace {
			return fmt.Errorf("schema attribute '%s' on <schema> must be unprefixed", attr.Name.Local)
		}
	}
	return nil
}
