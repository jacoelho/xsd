package schema

import (
	"encoding/xml"
	"errors"
	"io"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

type schemaDocument struct {
	root       *schemaNode
	arena      []*schemaNode
	name       string
	key        string
	defaults   SchemaDefaults
	references []schemaReference
	nodes      int
}

func (d *schemaDocument) node(id schemaNodeID) *schemaNode {
	if d == nil || int(id) >= len(d.arena) {
		return nil
	}
	return d.arena[id]
}

// schemaNodeID identifies one immutable semantic node in a source document.
// IDs are document-local and stable for the lifetime of the compiled graph;
// target-context views reuse the same IDs instead of copying the source tree.
type schemaNodeID uint32

type schemaNodeKey struct {
	doc  *schemaDocument
	view string
	id   schemaNodeID
}

func nodeKey(n *schemaNode, ctx *schemaSetDocument) schemaNodeKey {
	if n == nil {
		return schemaNodeKey{}
	}
	view := ""
	if ctx != nil {
		view = ctx.viewKey
	}
	return schemaNodeKey{doc: n.doc, id: n.id, view: view}
}

func schemaGlobalName(source *schemaSemanticSource) (LexicalAttribute, bool) {
	switch source := source.kind.(type) {
	case *schemaSimpleTypeSource:
		return source.Global.Name, true
	case *schemaComplexTypeSource:
		return source.Global.Name, true
	case *schemaElementSource:
		return source.Global.Name, true
	case *schemaAttributeSource:
		return source.Global.Name, true
	case *schemaGroupSource:
		return source.Global.Name, true
	case *schemaAttributeGroupSource:
		return source.Global.Name, true
	case *schemaNotationSource:
		return source.Global.Name, true
	default:
		return LexicalAttribute{}, false
	}
}

// schemaAttribute is the normalized expanded attribute representation retained
// by the semantic source document. Namespace declarations are consumed by the
// namespace stack and are deliberately not retained here.
type schemaAttribute struct {
	Name  xml.Name
	Value string
}

func schemaAttributesStream(element xml.Name, attrs []xmlstream.Attr) []schemaAttribute {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]schemaAttribute, 0, len(attrs))
	for _, attr := range attrs {
		if normalized, ok := normalizeSchemaAttribute(element, attr.Name, attr.Value); ok {
			out = append(out, normalized)
		}
	}
	return out
}

func normalizeSchemaAttribute(element, name xml.Name, value string) (schemaAttribute, bool) {
	// Namespace declarations are consumed by xmlstream.Reader.Start and belong
	// to the namespace frame, not the schema component's attribute vocabulary.
	if xmlstream.IsExpandedNamespaceName(name) {
		return schemaAttribute{}, false
	}
	if (name.Space == vocab.XMLNamespaceURI && name.Local == vocab.XMLAttrBase) ||
		(name.Space == "" && schemaAttributeCollapses(element, name.Local)) {
		value = lex.CollapseXMLWhitespace(value)
	} else if name.Space != "" {
		value = lex.ReplaceXMLWhitespace(value)
	}
	return schemaAttribute{Name: name, Value: value}, true
}

func schemaAttributeCollapses(element xml.Name, local string) bool {
	switch local {
	case vocab.XSDAttrDefault, vocab.XSDAttrValue:
		return false
	case vocab.XSDAttrFixed:
		return element.Space == vocab.XSDNamespaceURI &&
			element.Local != vocab.XSDElemElement && element.Local != vocab.XSDElemAttribute
	default:
		return true
	}
}

type schemaSyntaxNode struct {
	doc    *schemaDocument
	Name   xml.Name
	NS     xmlstream.Context
	Attrs  []schemaAttribute
	Line   int
	Column int
}

func (n *schemaSyntaxNode) schemaLocation() (path string, line int, column int) {
	if n == nil || n.doc == nil {
		return "", 0, 0
	}
	return n.doc.name, n.Line, n.Column
}

// parseSchemaDocument parses one source with the caller-owned parser. The
// parser is detached before this function returns and may be reused.
func parseSchemaDocument(name, key string, input io.Reader, limits Limits, parser *xmlstream.Reader) (*schemaDocument, error) {
	doc, err := parseTypedSchemaSourceDocument(name, key, input, limits, parser)
	if err != nil {
		return nil, err
	}
	defaults, err := parseDocumentDefaultsNode(doc.root)
	if err != nil {
		return nil, xsderrors.WithLocation(name, 0, 0, err)
	}
	doc.defaults = defaults
	return doc, nil
}

func parseDocumentDefaultsNode(root *schemaNode) (SchemaDefaults, error) {
	if root == nil {
		return SchemaDefaults{}, xsderrors.InternalInvariant("schema defaults require a parsed root")
	}
	source, ok := root.semantic.kind.(*schemaDocumentSource)
	if !ok || source == nil {
		return SchemaDefaults{}, withSchemaCompileLocation(root, xsderrors.InternalInvariant("schema defaults require a typed document source"))
	}
	defaults, err := ParseSchemaDefaults(*source)
	return defaults, withSchemaCompileLocation(root, err)
}

func schemaReaderError(err error) error {
	return schemaStreamError(0, 0, err)
}

func schemaStreamError(line, col int, err error) error {
	if boundary, ok := errors.AsType[*xmlstream.Error](err); ok && boundary != nil && boundary.Line > 0 {
		line, col = boundary.Line, boundary.Column
	}
	if errors.Is(err, xmlstream.ErrUnsupportedNonUTF8) {
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, "schema documents must be UTF-8", err)
	}
	if versionErr, ok := errors.AsType[xmlstream.UnsupportedXMLVersionError](err); ok {
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedXML11, versionErr.Error(), nil)
	}
	if errors.Is(err, xmlstream.ErrUnsupportedDTD) {
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedDTD, "DTD declarations are not supported", err)
	}
	if xmlstream.IsTokenLimit(err) || xmlstream.IsAttributeLimit(err) {
		return schemaParseAt(line, col, xsderrors.CodeSchemaLimit, err.Error(), err)
	}
	return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "invalid schema XML", err)
}

func schemaNodeIDFor(index int) (schemaNodeID, error) {
	if index < 0 || uint64(index) > uint64(^schemaNodeID(0)) {
		return 0, errors.New("schema node ID space exhausted")
	}
	return schemaNodeID(index), nil
}

func checkSchemaStartElementLimitStream(start xmlstream.StartElement, limits Limits, line, col int) error {
	if limits.MaxSchemaTokenBytes <= 0 {
		return nil
	}
	size := int64(len(start.Name.Space) + len(start.Name.Local))
	if err := checkSchemaTokenLimit(size, limits, line, col, "schema XML start element exceeds configured limit"); err != nil {
		return err
	}
	for _, attr := range start.Attr {
		if err := checkSchemaTokenLimit(int64(len(attr.Value)), limits, line, col, "schema XML attribute value exceeds configured limit"); err != nil {
			return err
		}
		size += int64(len(attr.Name.Space) + len(attr.Name.Local) + len(attr.Value))
		if err := checkSchemaTokenLimit(size, limits, line, col, "schema XML start element exceeds configured limit"); err != nil {
			return err
		}
	}
	return nil
}

func checkSchemaTokenLimit(size int64, limits Limits, line, col int, msg string) error {
	if limits.MaxSchemaTokenBytes > 0 && size > limits.MaxSchemaTokenBytes {
		limitErr := schemaParseAt(line, col, xsderrors.CodeSchemaLimit, msg, nil)
		return limitErr
	}
	return nil
}

func (n *schemaSyntaxNode) attr(local string) (string, bool) {
	for _, a := range n.Attrs {
		if a.Name.Space == "" && a.Name.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

func (n *schemaSyntaxNode) attrNS(namespace, local string) (string, bool) {
	for _, a := range n.Attrs {
		if a.Name.Space == namespace && a.Name.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

func (n *schemaSyntaxNode) resolveQName(lexical string) (xml.Name, error) {
	parts, err := checkSchemaQNamePartsSyntax(n, lexical)
	if err != nil {
		return xml.Name{}, err
	}
	if !parts.Prefixed {
		ns, _ := n.NS.Lookup("")
		return xml.Name{Space: ns, Local: parts.Local}, nil
	}
	ns, ok := n.NS.Lookup(parts.Prefix)
	if !ok {
		return xml.Name{}, schemaCompileAt(n, xsderrors.CodeSchemaReference, "unbound QName prefix "+parts.Prefix)
	}
	return xml.Name{Space: ns, Local: parts.Local}, nil
}
