package compile

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"iter"
	"maps"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/xsderrors"
)

type rawDoc struct {
	root *rawNode
	name string
	key  string
}

type rawNode struct {
	NS       map[string]string
	Name     xml.Name
	Text     string
	text     []byte
	Attr     []xml.Attr
	Children []*rawNode
	Line     int
	Column   int
}

func parseSchemaDocument(name, key string, data []byte, limits Limits) (*rawDoc, error) {
	data = bytes.TrimPrefix(data, stream.UTF8BOM)
	if enc := stream.DeclaredEncoding(data); enc != "" && !strings.EqualFold(enc, "UTF-8") && !strings.EqualFold(enc, "UTF8") {
		return nil, xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, "schema documents must be UTF-8")
	}
	if version := stream.DeclaredXMLVersion(data); version != "" && version != vocab.XMLVersion10 {
		return nil, xsderrors.Unsupported(xsderrors.CodeUnsupportedXML11, "XML version "+version+" is not supported")
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	state := schemaParseState{
		dec:    dec,
		limits: limits,
		nsStack: []map[string]string{{
			vocab.XMLPrefix: vocab.XMLNamespaceURI,
		}},
	}
	if err := state.parse(); err != nil {
		return nil, err
	}
	if err := validateSchemaRoot(state.root); err != nil {
		return nil, err
	}
	if err := rejectUnsupportedSchemaNodes(state.root, nil); err != nil {
		return nil, err
	}
	if err := rejectUnknownSchemaAttributes(state.root); err != nil {
		return nil, err
	}
	if err := checkSchemaIDs(state.root); err != nil {
		return nil, err
	}
	if err := rejectInvalidSchemaNames(state.root, nil); err != nil {
		return nil, err
	}
	if err := rejectInvalidAnnotations(state.root); err != nil {
		return nil, err
	}
	return &rawDoc{name: name, key: key, root: state.root}, nil
}

type schemaParseState struct {
	dec     *xml.Decoder
	root    *rawNode
	stack   []*rawNode
	nsStack []map[string]string
	limits  Limits
}

func (s *schemaParseState) parse() error {
	for {
		tok, err := s.dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			line, col := s.dec.InputPos()
			return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "invalid schema XML", err)
		}
		if err := s.handleToken(tok); err != nil {
			return err
		}
	}
	if len(s.stack) != 0 {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, 0, 0, "unclosed schema element", nil)
	}
	return nil
}

func (s *schemaParseState) handleToken(tok xml.Token) error {
	switch t := tok.(type) {
	case xml.StartElement:
		return s.handleStartElement(t)
	case xml.EndElement:
		return s.handleEndElement()
	case xml.CharData:
		return s.handleCharData(t)
	case xml.Directive:
		return s.handleDirective(t)
	case xml.ProcInst:
		return s.handleProcInst(t)
	case xml.Comment:
		return s.handleComment(t)
	default:
		return nil
	}
}

func (s *schemaParseState) handleStartElement(t xml.StartElement) error {
	line, col := s.dec.InputPos()
	if s.limits.MaxSchemaDepth > 0 && len(s.stack)+1 > s.limits.MaxSchemaDepth {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaLimit, line, col, "schema XML nesting exceeds configured limit", nil)
	}
	if s.limits.MaxSchemaAttributes > 0 && len(t.Attr) > s.limits.MaxSchemaAttributes {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaLimit, line, col, "schema XML attributes exceed configured limit", nil)
	}
	if err := checkSchemaStartElementLimit(t, s.limits, line, col); err != nil {
		return err
	}
	parentNS := s.nsStack[len(s.nsStack)-1]
	ns := parentNS
	clonedNS := false
	for _, a := range t.Attr {
		if a.Name.Space == vocab.XMLNSPrefix {
			if !clonedNS {
				ns = cloneNS(parentNS)
				clonedNS = true
			}
			ns[a.Name.Local] = a.Value
			continue
		}
		if a.Name.Space == "" && a.Name.Local == vocab.XMLNSPrefix {
			if !clonedNS {
				ns = cloneNS(parentNS)
				clonedNS = true
			}
			ns[""] = a.Value
		}
	}
	n := &rawNode{Name: t.Name, Attr: t.Attr, NS: ns, Line: line, Column: col}
	if len(s.stack) == 0 {
		if s.root != nil {
			return xsderrors.SchemaParse(xsderrors.CodeSchemaRoot, line, col, "schema document has multiple roots", nil)
		}
		s.root = n
	} else {
		parent := s.stack[len(s.stack)-1]
		parent.Children = append(parent.Children, n)
	}
	s.stack = append(s.stack, n)
	s.nsStack = append(s.nsStack, ns)
	return nil
}

func (s *schemaParseState) handleEndElement() error {
	if len(s.stack) == 0 {
		line, col := s.dec.InputPos()
		return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "unexpected end element", nil)
	}
	n := s.stack[len(s.stack)-1]
	if n.text != nil {
		n.Text = string(n.text)
		n.text = nil
	}
	s.stack = s.stack[:len(s.stack)-1]
	s.nsStack = s.nsStack[:len(s.nsStack)-1]
	return nil
}

func (s *schemaParseState) handleCharData(t xml.CharData) error {
	line, col := s.dec.InputPos()
	if err := checkSchemaTokenLimit(int64(len(t)), s.limits, line, col, "schema XML text exceeds configured limit"); err != nil {
		return err
	}
	if len(s.stack) == 0 {
		if !lex.IsXMLWhitespaceBytes(t) {
			return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "schema XML text outside root element", nil)
		}
		return nil
	}
	n := s.stack[len(s.stack)-1]
	if err := checkSchemaTokenLimit(int64(len(n.Text)+len(n.text)+len(t)), s.limits, line, col, "schema XML text exceeds configured limit"); err != nil {
		return err
	}
	if n.Text == "" && n.text == nil {
		n.Text = string(t)
		return nil
	}
	if n.text == nil {
		n.text = append(n.text, n.Text...)
		n.Text = ""
	}
	n.text = append(n.text, t...)
	return nil
}

func (s *schemaParseState) handleDirective(t xml.Directive) error {
	line, col := s.dec.InputPos()
	if err := checkSchemaTokenLimit(int64(len(t)), s.limits, line, col, "schema XML directive exceeds configured limit"); err != nil {
		return err
	}
	if stream.IsDOCTYPEDeclaration(t) {
		return xsderrors.UnsupportedAt(xsderrors.CodeUnsupportedDTD, line, col, "", "DTD declarations are not supported", nil)
	}
	return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "invalid schema XML", nil)
}

func (s *schemaParseState) handleProcInst(t xml.ProcInst) error {
	line, col := s.dec.InputPos()
	size := int64(len(t.Target) + len(t.Inst))
	return checkSchemaTokenLimit(size, s.limits, line, col, "schema XML processing instruction exceeds configured limit")
}

func (s *schemaParseState) handleComment(t xml.Comment) error {
	line, col := s.dec.InputPos()
	return checkSchemaTokenLimit(int64(len(t)), s.limits, line, col, "schema XML comment exceeds configured limit")
}

func validateSchemaRoot(root *rawNode) error {
	if root == nil {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaRoot, 0, 0, "empty schema document", nil)
	}
	if root.Name.Space != vocab.XSDNamespaceURI || root.Name.Local != vocab.XSDElemSchema {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaRoot, root.Line, root.Column, "root element must be xs:schema", nil)
	}
	return nil
}

func checkSchemaStartElementLimit(start xml.StartElement, limits Limits, line, col int) error {
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
		limitErr := xsderrors.SchemaParse(xsderrors.CodeSchemaLimit, line, col, msg, nil)
		return limitErr
	}
	return nil
}

func cloneNS(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src)+2)
	maps.Copy(dst, src)
	return dst
}

func rejectUnsupportedSchemaNodes(n, parent *rawNode) error {
	skipChildren, err := checkUnsupportedSchemaNode(n, parent)
	if err != nil || skipChildren {
		return err
	}
	for _, c := range n.Children {
		if err := rejectUnsupportedSchemaNodes(c, n); err != nil {
			return err
		}
	}
	return nil
}

func rejectUnknownSchemaAttributes(n *rawNode) error {
	if n.Name.Space == vocab.XSDNamespaceURI {
		if err := checkRawSchemaAttributes(n); err != nil {
			return err
		}
	}
	for _, child := range n.Children {
		if err := rejectUnknownSchemaAttributes(child); err != nil {
			return err
		}
	}
	return nil
}

func rejectInvalidSchemaNames(n, parent *rawNode) error {
	if err := checkSchemaNodeNames(n, parent); err != nil {
		return err
	}
	for _, child := range n.Children {
		if err := rejectInvalidSchemaNames(child, n); err != nil {
			return err
		}
	}
	return nil
}

func rejectInvalidAnnotations(n *rawNode) error {
	skipChildren, err := checkSchemaAnnotationNode(n)
	if err != nil || skipChildren {
		return err
	}
	for _, child := range n.Children {
		if err := rejectInvalidAnnotations(child); err != nil {
			return err
		}
	}
	return nil
}

func (n *rawNode) attr(local string) (string, bool) {
	for _, a := range n.Attr {
		if a.Name.Space == "" && a.Name.Local == local {
			if local == vocab.XSDAttrName || local == vocab.XSDAttrID {
				return lex.CollapseXMLWhitespace(a.Value), true
			}
			return lex.ReplaceXMLWhitespace(a.Value), true
		}
	}
	return "", false
}

func (n *rawNode) attrValue(local string) string {
	if v, ok := n.attr(local); ok {
		return v
	}
	return ""
}

func (n *rawNode) xsSimpleTypeChildren() []*rawNode {
	var out []*rawNode
	for _, c := range n.Children {
		if c.Name.Space == vocab.XSDNamespaceURI && c.Name.Local == vocab.XSDElemSimpleType {
			out = append(out, c)
		}
	}
	return out
}

// xsdChildren yields the XSD-namespace element children of n in document order.
func (n *rawNode) xsdChildren() iter.Seq[*rawNode] {
	return func(yield func(*rawNode) bool) {
		for _, c := range n.Children {
			if c.Name.Space != vocab.XSDNamespaceURI {
				continue
			}
			if !yield(c) {
				return
			}
		}
	}
}

func (n *rawNode) firstXS(local string) *rawNode {
	for _, c := range n.Children {
		if c.Name.Space == vocab.XSDNamespaceURI && c.Name.Local == local {
			return c
		}
	}
	return nil
}

func (n *rawNode) resolveQName(lexical string) (string, string, error) {
	prefix, local, prefixed, err := checkSchemaQNameParts(n, lexical)
	if err != nil {
		return "", "", err
	}
	if !prefixed {
		return n.NS[""], local, nil
	}
	ns, ok := n.NS[prefix]
	if !ok {
		return "", "", schemaCompileAt(n, xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
	}
	return ns, local, nil
}
