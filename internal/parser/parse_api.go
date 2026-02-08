package parser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"iter"

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
	importedNamespaces := make(map[types.NamespaceURI]bool)
	dirState := directiveState{}
	componentSubtrees := make([]topLevelComponentSubtree, 0, 8)
	var subtreeBuf bytes.Buffer
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
				continue
			}

			subtreeData, err := readSubtreeBytes(reader, &subtreeBuf)
			if err != nil {
				return nil, newParseError("parse XML", fmt.Errorf("xml read: %w", err))
			}
			if ev.Name.Namespace != xsdxml.XSDNamespace {
				continue
			}
			switch ev.Name.Local {
			case "annotation", "import", "include":
				doc, root, err := parseSubtreeIntoDoc(subtreeData, opts...)
				if err != nil {
					return nil, newParseError("parse XML", err)
				}
				if err := validateSchemaAttributeNamespaces(doc, root); err != nil {
					xsdxml.ReleaseDocument(doc)
					return nil, err
				}
				if err := parseDirectiveElement(doc, root, schema, result, importedNamespaces, &dirState); err != nil {
					xsdxml.ReleaseDocument(doc)
					return nil, err
				}
				xsdxml.ReleaseDocument(doc)
			case "redefine":
				return nil, fmt.Errorf("redefine is not supported")
			default:
				if !isTopLevelComponentElement(ev.Name.Local) {
					return nil, fmt.Errorf("unexpected top-level element '%s'", ev.Name.Local)
				}
				if isGlobalDeclElement(ev.Name.Local) {
					dirState.declIndex++
				}
				componentSubtrees = append(componentSubtrees, topLevelComponentSubtree{data: subtreeData})
			}

		case xmlstream.EventEndElement:
			if rootSeen && !rootClosed {
				rootClosed = true
			}
			allowBOM = false
		case xmlstream.EventCharData:
			if !rootSeen || rootClosed {
				if !xsdxml.IsIgnorableOutsideRoot(ev.Text, allowBOM) {
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
	for _, subtree := range componentSubtrees {
		doc, root, err := parseSubtreeIntoDoc(subtree.data, opts...)
		if err != nil {
			return nil, newParseError("parse XML", err)
		}
		if err := validateSchemaAttributeNamespaces(doc, root); err != nil {
			xsdxml.ReleaseDocument(doc)
			return nil, err
		}
		if err := parseTopLevelComponent(doc, root, schema); err != nil {
			xsdxml.ReleaseDocument(doc)
			return nil, err
		}
		xsdxml.ReleaseDocument(doc)
	}
	return result, nil
}

type topLevelComponentSubtree struct {
	data []byte
}

func readSubtreeBytes(reader *xmlstream.Reader, buf *bytes.Buffer) ([]byte, error) {
	buf.Reset()
	if _, err := reader.ReadSubtreeTo(buf); err != nil {
		return nil, err
	}
	return append([]byte(nil), buf.Bytes()...), nil
}

func parseSubtreeIntoDoc(data []byte, opts ...xmlstream.Option) (*xsdxml.Document, xsdxml.NodeID, error) {
	doc := xsdxml.AcquireDocument()
	if err := xsdxml.ParseIntoWithOptions(bytes.NewReader(data), doc, opts...); err != nil {
		xsdxml.ReleaseDocument(doc)
		return nil, xsdxml.InvalidNode, err
	}
	root := doc.DocumentElement()
	if root == xsdxml.InvalidNode {
		xsdxml.ReleaseDocument(doc)
		return nil, xsdxml.InvalidNode, io.ErrUnexpectedEOF
	}
	return doc, root, nil
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
			targetNSAttr = types.ApplyWhiteSpace(string(attr.Value), types.WhiteSpaceCollapse)
			targetNSFound = true
		case xsdxml.XSDNamespace:
			return fmt.Errorf("schema attribute 'targetNamespace' must be unprefixed (found '%s:targetNamespace')", attr.Name.Namespace)
		}
	}
	if !targetNSFound {
		schema.TargetNamespace = types.NamespaceEmpty
	} else {
		if targetNSAttr == "" {
			return fmt.Errorf("targetNamespace attribute cannot be empty (must be absent or have a non-empty value)")
		}
		schema.TargetNamespace = types.NamespaceURI(targetNSAttr)
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
		elemForm = types.ApplyWhiteSpace(elemForm, types.WhiteSpaceCollapse)
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
		attrForm = types.ApplyWhiteSpace(attrForm, types.WhiteSpaceCollapse)
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
		if types.TrimXMLWhitespace(blockDefault) == "" {
			return fmt.Errorf("blockDefault attribute cannot be empty")
		}
		block, err := parseDerivationSetWithValidation(
			blockDefault,
			types.DerivationSet(types.DerivationSubstitution|types.DerivationExtension|types.DerivationRestriction),
		)
		if err != nil {
			return fmt.Errorf("invalid blockDefault attribute value '%s': %w", blockDefault, err)
		}
		schema.BlockDefault = block
	}

	if finalDefault, ok := findStartAttr(start.Attrs, "finalDefault"); ok {
		if types.TrimXMLWhitespace(finalDefault) == "" {
			return fmt.Errorf("finalDefault attribute cannot be empty")
		}
		final, err := parseDerivationSetWithValidation(
			finalDefault,
			types.DerivationSet(types.DerivationExtension|types.DerivationRestriction|types.DerivationList|types.DerivationUnion),
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
