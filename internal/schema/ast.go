package schema

import (
	"encoding/xml"
	"errors"
	"io"
	"strconv"

	"github.com/jacoelho/xsd/internal/diagnostictext"
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
	defaults   schemaDefaults
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

func schemaGlobalName(source *schemaSemanticSource) (lexicalAttribute, bool) {
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
		return lexicalAttribute{}, false
	}
}

// schemaAttribute is the normalized expanded attribute representation retained
// by the semantic source document. Namespace declarations are consumed by the
// namespace stack and are deliberately not retained here.
type schemaAttribute struct {
	Name  xml.Name
	Value string
}

// schemaAttrValue reads attr through reader. The schema reader never skips
// values, so a failure is an invariant violation.
func schemaAttrValue(reader *xmlstream.Reader, attr *xmlstream.Attr) (string, error) {
	value, err := reader.AttrValue(attr)
	if err != nil {
		return "", xsderrors.InternalInvariant("schema reader skipped attribute value")
	}
	return value, nil
}

func schemaAttributesStream(reader *xmlstream.Reader, element xml.Name, attrs []xmlstream.Attr) ([]schemaAttribute, error) {
	if len(attrs) == 0 {
		return nil, nil
	}
	out := make([]schemaAttribute, 0, len(attrs))
	for i := range attrs {
		attr := &attrs[i]
		value, err := schemaAttrValue(reader, attr)
		if err != nil {
			return nil, err
		}
		if normalized, ok := normalizeSchemaAttribute(element, attr.Name, value); ok {
			out = append(out, normalized)
		}
	}
	return out, nil
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

func parseDocumentDefaultsNode(root *schemaNode) (schemaDefaults, error) {
	if root == nil {
		return schemaDefaults{}, xsderrors.InternalInvariant("schema defaults require a parsed root")
	}
	source, ok := root.semantic.kind.(*schemaDocumentSource)
	if !ok || source == nil {
		return schemaDefaults{}, withSchemaCompileLocation(root, xsderrors.InternalInvariant("schema defaults require a typed document source"))
	}
	defaults, err := parseSchemaDefaults(*source)
	return defaults, withSchemaCompileLocation(root, err)
}

func schemaReaderError(err error, limits Limits) error {
	return schemaStreamError(0, 0, err, limits)
}

func schemaStreamError(line, col int, err error, limits Limits) error {
	if boundary, ok := errors.AsType[*xmlstream.Error](err); ok && boundary != nil && boundary.Line > 0 {
		line, col = boundary.Line, boundary.Column
	}
	if feature, ok := xmlstream.UnsupportedFeatureOf(err); ok {
		return unsupportedSchemaFeature(feature, err)
	}
	if limit, ok := xmlstream.LimitOf(err); ok {
		return schemaReaderLimitError(limit, line, col, err, limits)
	}
	return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "invalid schema XML", err)
}

// schemaReaderLimitError maps every reader limit to the schema option that
// configured it. The schema reader sets no input-byte limit.
func schemaReaderLimitError(limit xmlstream.Limit, line, col int, err error, limits Limits) error {
	var option string
	var configured int64
	switch limit {
	case xmlstream.LimitTokenBytes:
		option, configured = "MaxSchemaTokenBytes", limits.MaxSchemaTokenBytes
	case xmlstream.LimitAttrs:
		option, configured = "MaxSchemaAttributes", int64(limits.MaxSchemaAttributes)
	case xmlstream.LimitDepth:
		option, configured = "MaxSchemaDepth", int64(limits.MaxSchemaDepth)
	case xmlstream.LimitInputBytes:
		return xsderrors.InternalInvariant("schema reader has no input limit")
	}
	return schemaParseAt(line, col, xsderrors.CodeSchemaLimit, option+"="+strconv.FormatInt(configured, 10)+" exceeded", err)
}

func unsupportedSchemaFeature(feature xmlstream.UnsupportedFeature, err error) error {
	switch feature {
	case xmlstream.UnsupportedDTD:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedDTD, "", err)
	case xmlstream.UnsupportedNonUTF8:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, "schema documents must be UTF-8", err)
	case xmlstream.UnsupportedXMLVersion:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedXMLVersion, err.Error(), nil)
	case xmlstream.UnsupportedEntityReference:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedExternal, "schema documents cannot reference undeclared entities", err)
	}
	return xsderrors.InternalInvariant("unknown xmlstream unsupported feature")
}

func schemaNodeIDFor(index int) (schemaNodeID, error) {
	if index < 0 || uint64(index) > uint64(^schemaNodeID(0)) {
		return 0, errors.New("schema node ID space exhausted")
	}
	return schemaNodeID(index), nil
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
		return xml.Name{}, schemaCompileAt(n, xsderrors.CodeSchemaReference, "unbound QName prefix "+diagnostictext.Quote(parts.Prefix))
	}
	return xml.Name{Space: ns, Local: parts.Local}, nil
}
