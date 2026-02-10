package parser

import (
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmllex"
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
	return ParseWithImportsOptionsWithPool(r, xsdxml.NewDocumentPool(), opts...)
}

// ParseWithImportsOptionsWithPool parses an XSD schema with XML reader options and an explicit document pool.
func ParseWithImportsOptionsWithPool(r io.Reader, pool *xsdxml.DocumentPool, opts ...xmlstream.Option) (*ParseResult, error) {
	reader, err := xmlstream.NewReader(r, opts...)
	if err != nil {
		return nil, newParseError("parse XML", fmt.Errorf("xml reader: %w", err))
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
	allowBOM := true
	rootSeen := false
	rootClosed := false

	for {
		ev, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, newParseError("parse XML", fmt.Errorf("xml read: %w", err))
		}

		switch ev.Kind {
		case xmlstream.EventStartElement:
			if rootClosed {
				return nil, newParseError("parse XML", fmt.Errorf("unexpected element %s after document end", ev.Name.Local))
			}
			allowBOM = false
			if !rootSeen {
				rootSeen = true
				if ev.Name.Local != "schema" || ev.Name.Namespace != xsdxml.XSDNamespace {
					return nil, fmt.Errorf("root element must be xs:schema, got {%s}%s", ev.Name.Namespace, ev.Name.Local)
				}
				if err := parseSchemaAttributesFromStart(ev, reader.NamespaceDeclsSeq(ev.ScopeDepth), schema); err != nil {
					return nil, err
				}
				applyImportedNamespaces(schema, importedNamespaces)
				continue
			}
			if ev.Name.Namespace != xsdxml.XSDNamespace {
				if err := reader.SkipSubtree(); err != nil {
					return nil, newParseError("parse XML", fmt.Errorf("xml skip for element %s: %w", ev.Name.String(), err))
				}
				continue
			}

			doc, root, err := parseSubtreeIntoDoc(reader, ev, pool)
			if err != nil {
				return nil, newParseError("parse XML", fmt.Errorf("xml read for element %s: %w", ev.Name.String(), err))
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
			if rootSeen && !rootClosed {
				rootClosed = true
			}
			allowBOM = false
		case xmlstream.EventCharData:
			if !rootSeen || rootClosed {
				if !xmllex.IsIgnorableOutsideRoot(ev.Text, allowBOM) {
					return nil, newParseError("parse XML", fmt.Errorf("unexpected character data outside root element"))
				}
				allowBOM = false
			}
		case xmlstream.EventComment, xmlstream.EventPI, xmlstream.EventDirective:
			if !rootSeen || rootClosed {
				allowBOM = false
			}
		}
	}

	if !rootSeen {
		return nil, fmt.Errorf("empty document")
	}

	applyImportedNamespaces(schema, importedNamespaces)
	return result, nil
}

func parseSubtreeIntoDoc(reader *xmlstream.Reader, start xmlstream.Event, pool *xsdxml.DocumentPool) (*xsdxml.Document, xsdxml.NodeID, error) {
	doc := pool.Acquire()
	if err := xsdxml.ParseSubtreeInto(reader, start, doc); err != nil {
		pool.Release(doc)
		return nil, xsdxml.InvalidNode, err
	}
	root := doc.DocumentElement()
	if root == xsdxml.InvalidNode {
		pool.Release(doc)
		return nil, xsdxml.InvalidNode, io.ErrUnexpectedEOF
	}
	return doc, root, nil
}

func parseDirectiveSubtree(doc *xsdxml.Document, root xsdxml.NodeID, schema *Schema, result *ParseResult, importedNamespaces map[model.NamespaceURI]bool, state *directiveState, pool *xsdxml.DocumentPool) error {
	defer pool.Release(doc)
	if err := validateSchemaAttributeNamespaces(doc, root); err != nil {
		return err
	}
	return parseDirectiveElement(doc, root, schema, result, importedNamespaces, state)
}

func parseTopLevelComponentSubtree(doc *xsdxml.Document, root xsdxml.NodeID, schema *Schema, pool *xsdxml.DocumentPool) error {
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

	targetNSAttr := ""
	targetNSFound := false
	for _, attr := range start.Attrs {
		if attr.Name.Local != "targetNamespace" {
			continue
		}
		switch attr.Name.Namespace {
		case "":
			targetNSAttr = model.ApplyWhiteSpace(string(attr.Value), model.WhiteSpaceCollapse)
			targetNSFound = true
		case xsdxml.XSDNamespace:
			return fmt.Errorf("schema attribute 'targetNamespace' must be unprefixed (found '%s:targetNamespace')", attr.Name.Namespace)
		}
	}
	if !targetNSFound {
		schema.TargetNamespace = model.NamespaceEmpty
	} else {
		if targetNSAttr == "" {
			return fmt.Errorf("targetNamespace attribute cannot be empty (must be absent or have a non-empty value)")
		}
		schema.TargetNamespace = model.NamespaceURI(targetNSAttr)
	}

	if decls != nil {
		for decl := range decls {
			if decl.Prefix == "" {
				schema.NamespaceDecls[""] = decl.URI
				continue
			}
			if decl.URI == "" {
				return fmt.Errorf("namespace prefix %q cannot be bound to empty namespace", decl.Prefix)
			}
			schema.NamespaceDecls[decl.Prefix] = decl.URI
		}
	}

	if elemForm, ok := findStartAttr(start.Attrs, "elementFormDefault"); ok {
		elemForm = model.ApplyWhiteSpace(elemForm, model.WhiteSpaceCollapse)
		if elemForm == "" {
			return fmt.Errorf("elementFormDefault attribute cannot be empty")
		}
		switch elemForm {
		case "qualified":
			schema.ElementFormDefault = Qualified
		case "unqualified":
			schema.ElementFormDefault = Unqualified
		default:
			return fmt.Errorf("invalid elementFormDefault attribute value '%s': must be 'qualified' or 'unqualified'", elemForm)
		}
	}

	if attrForm, ok := findStartAttr(start.Attrs, "attributeFormDefault"); ok {
		attrForm = model.ApplyWhiteSpace(attrForm, model.WhiteSpaceCollapse)
		if attrForm == "" {
			return fmt.Errorf("attributeFormDefault attribute cannot be empty")
		}
		switch attrForm {
		case "qualified":
			schema.AttributeFormDefault = Qualified
		case "unqualified":
			schema.AttributeFormDefault = Unqualified
		default:
			return fmt.Errorf("invalid attributeFormDefault attribute value '%s': must be 'qualified' or 'unqualified'", attrForm)
		}
	}

	if blockDefault, ok := findStartAttr(start.Attrs, "blockDefault"); ok {
		if model.TrimXMLWhitespace(blockDefault) == "" {
			return fmt.Errorf("blockDefault attribute cannot be empty")
		}
		block, err := parseDerivationSetWithValidation(
			blockDefault,
			model.DerivationSet(model.DerivationSubstitution|model.DerivationExtension|model.DerivationRestriction),
		)
		if err != nil {
			return fmt.Errorf("invalid blockDefault attribute value '%s': %w", blockDefault, err)
		}
		schema.BlockDefault = block
	}

	if finalDefault, ok := findStartAttr(start.Attrs, "finalDefault"); ok {
		if model.TrimXMLWhitespace(finalDefault) == "" {
			return fmt.Errorf("finalDefault attribute cannot be empty")
		}
		final, err := parseDerivationSetWithValidation(
			finalDefault,
			model.DerivationSet(model.DerivationExtension|model.DerivationRestriction|model.DerivationList|model.DerivationUnion),
		)
		if err != nil {
			return fmt.Errorf("invalid finalDefault attribute value '%s': %w", finalDefault, err)
		}
		schema.FinalDefault = final
	}

	return nil
}

func validateSchemaStartAttributeNamespaces(start xmlstream.Event) error {
	for _, attr := range start.Attrs {
		if attr.Name.Namespace == xsdxml.XMLNSNamespace {
			continue
		}
		if attr.Name.Namespace == xsdxml.XSDNamespace {
			return fmt.Errorf("schema attribute '%s' on <schema> must be unprefixed", attr.Name.Local)
		}
	}
	return nil
}

func findStartAttr(attrs []xmlstream.Attr, local string) (string, bool) {
	for _, attr := range attrs {
		if attr.Name.Namespace != "" {
			continue
		}
		if attr.Name.Local == local {
			return string(attr.Value), true
		}
	}
	return "", false
}
