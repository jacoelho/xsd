package compile

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"iter"
	"maps"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlns"
	"github.com/jacoelho/xsd/xsderrors"
)

type rawDoc struct {
	root       *rawNode
	name       string
	key        string
	defaults   SchemaDefaults
	references []schemaReference
	nodes      int
}

type rawNode struct {
	doc      *rawDoc
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
	doc, err := parseRawSchemaDocument(name, key, data, limits)
	if err != nil {
		return nil, err
	}
	if admitErr := admitSchemaDocument(doc); admitErr != nil {
		return nil, xsderrors.WithPath(name, admitErr)
	}
	defaults, err := parseSchemaDefaults(doc.root)
	if err != nil {
		return nil, xsderrors.WithPath(name, err)
	}
	doc.defaults = defaults
	return doc, nil
}

func parseSchemaDefaults(root *rawNode) (SchemaDefaults, error) {
	target, hasTarget := root.attr(vocab.XSDAttrTargetNamespace)
	elementForm, hasElementForm := root.attr(vocab.XSDAttrElementFormDefault)
	attributeForm, hasAttributeForm := root.attr(vocab.XSDAttrAttributeFormDefault)
	defaults, err := ParseSchemaDefaults(SchemaDefaultAttrs{
		TargetNamespace:         target,
		BlockDefault:            root.attrValue(vocab.XSDAttrBlockDefault),
		FinalDefault:            root.attrValue(vocab.XSDAttrFinalDefault),
		ElementFormDefault:      elementForm,
		AttributeFormDefault:    attributeForm,
		HasTargetNamespace:      hasTarget,
		HasElementFormDefault:   hasElementForm,
		HasAttributeFormDefault: hasAttributeForm,
	})
	return defaults, withSchemaCompileLocation(root, err)
}

func parseRawSchemaDocument(name, key string, data []byte, limits Limits) (*rawDoc, error) {
	doc := &rawDoc{name: name, key: key}
	names := stream.NewCache()
	values := stream.NewCache()
	parser := new(stream.Parser)
	if err := parser.ResetWithLimit(bytes.NewReader(data), &names, &values, limits.MaxSchemaTokenBytes); err != nil {
		return nil, xsderrors.WithPath(name, schemaReaderError(err))
	}
	defer parser.Detach()
	parser.SetMaxAttrs(limits.MaxSchemaAttributes)
	parser.SetEmitComments(true)
	parser.SetEmitPI(true)
	state := schemaParseState{
		parser: parser,
		values: &values,
		doc:    doc,
		limits: limits,
	}
	if err := state.parse(); err != nil {
		return nil, xsderrors.WithPath(name, err)
	}
	doc.root, doc.nodes = state.root, state.nodes
	return doc, nil
}

type schemaParseFrame struct {
	node       *rawNode
	namespaces map[string]string
	name       xml.Name
	prefix     string
	textBytes  int64
}

type schemaParseState struct {
	parser *stream.Parser
	values *stream.Cache
	doc    *rawDoc
	root   *rawNode
	stack  []schemaParseFrame
	ns     xmlns.Stack
	nodes  int
	limits Limits
}

func (s *schemaParseState) parse() error {
	for {
		tok, err := s.parser.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			line, col := s.parser.Pos()
			return schemaStreamError(line, col, err)
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

func schemaReaderError(err error) error {
	return schemaStreamError(0, 0, err)
}

func schemaStreamError(line, col int, err error) error {
	if errors.Is(err, stream.ErrUnsupportedNonUTF8) {
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, "schema documents must be UTF-8")
	}
	var versionErr stream.UnsupportedXMLVersionError
	if errors.As(err, &versionErr) {
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedXML11, versionErr.Error())
	}
	if stream.IsTokenLimit(err) || stream.IsAttributeLimit(err) {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaLimit, line, col, err.Error(), err)
	}
	return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "invalid schema XML", err)
}

func (s *schemaParseState) handleToken(tok stream.Token) error {
	switch tok.Kind {
	case stream.KindStart:
		return s.start(tok.Start, tok.Line, tok.Column)
	case stream.KindEnd:
		return s.handleEndElement(tok.End, tok.Line, tok.Column)
	case stream.KindCharData:
		return s.chars(tok.Data, tok.Line, tok.Column)
	case stream.KindDirective, stream.KindComment:
		return s.ValidateDirective(tok.Kind, tok.Directive, nil, tok.Line, tok.Column)
	case stream.KindPI:
		return s.ValidateDirective(tok.Kind, tok.Data, tok.Directive, tok.Line, tok.Column)
	default:
		return nil
	}
}

func (s *schemaParseState) start(start stream.StartElement, line, col int) error {
	prefix := start.Name.Space
	if s.nodes >= s.limits.MaxSchemaInstantiatedNodes {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaLimit, line, col, "schema nodes exceed MaxSchemaInstantiatedNodes", nil)
	}
	if s.limits.MaxSchemaDepth > 0 && len(s.stack)+1 > s.limits.MaxSchemaDepth {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaLimit, line, col, "schema XML nesting exceeds configured limit", nil)
	}
	if s.limits.MaxSchemaAttributes > 0 && len(start.Attr) > s.limits.MaxSchemaAttributes {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaLimit, line, col, "schema XML attributes exceed configured limit", nil)
	}
	if err := s.ns.PushStream(start.Attr, s.values); err != nil {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "invalid schema XML", err)
	}
	prepared, err := s.prepareSchemaStart(start, line, col)
	if err != nil {
		s.ns.Pop()
		return err
	}
	parentNS := map[string]string{vocab.XMLPrefix: vocab.XMLNamespaceURI}
	if len(s.stack) != 0 {
		parentNS = s.stack[len(s.stack)-1].namespaces
	}
	ns := parentNS
	clonedNS := false
	for _, a := range prepared.Attr {
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
	opaque := (len(s.stack) != 0 && s.stack[len(s.stack)-1].node == nil) || s.annotationPayloadEnvelopeOpen()
	var n *rawNode
	if !opaque {
		n = &rawNode{doc: s.doc, Name: prepared.Name, Attr: prepared.Attr, NS: ns, Line: line, Column: col}
	}
	if len(s.stack) == 0 && s.root != nil {
		s.ns.Pop()
		return xsderrors.SchemaParse(xsderrors.CodeSchemaRoot, line, col, "schema document has multiple roots", nil)
	}
	s.nodes++
	if n != nil && len(s.stack) == 0 {
		s.root = n
	} else if n != nil {
		parent := s.stack[len(s.stack)-1].node
		parent.Children = append(parent.Children, n)
	}
	s.stack = append(s.stack, schemaParseFrame{
		node: n, namespaces: ns, name: prepared.Name, prefix: prefix,
	})
	return nil
}

func (s *schemaParseState) prepareSchemaStart(start stream.StartElement, line, col int) (xml.StartElement, error) {
	name, ok := s.ns.ResolveName(start.Name, xmlns.ElementName)
	if !ok {
		return xml.StartElement{}, xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "invalid schema XML", errors.New("unbound namespace prefix "+start.Name.Space))
	}
	for i := range start.Attr {
		attr := &start.Attr[i]
		if xmlns.IsNamespaceName(attr.Name) || attr.Name.Space == "" {
			continue
		}
		resolved, ok := s.ns.ResolveName(attr.Name, xmlns.AttributeName)
		if !ok {
			return xml.StartElement{}, xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "invalid schema XML", errors.New("unbound namespace prefix "+attr.Name.Space))
		}
		attr.Name = resolved
	}
	if err := xmlns.ValidateUniqueAttributes(start.Attr); err != nil {
		return xml.StartElement{}, xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "invalid schema XML", err)
	}
	attrs := make([]xml.Attr, len(start.Attr))
	for i := range start.Attr {
		attrs[i] = xml.Attr{Name: start.Attr[i].Name, Value: start.Attr[i].StringValue(s.values)}
	}
	prepared := xml.StartElement{Name: name, Attr: attrs}
	if err := checkSchemaStartElementLimit(prepared, s.limits, line, col); err != nil {
		return xml.StartElement{}, err
	}
	return prepared, nil
}

func (s *schemaParseState) handleEndElement(end stream.EndElement, line, col int) error {
	if len(s.stack) == 0 {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "unexpected end element", nil)
	}
	frame := s.stack[len(s.stack)-1]
	name, ok := s.ns.ResolveName(end.Name, xmlns.ElementName)
	if !ok {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "invalid schema XML", errors.New("unbound namespace prefix "+end.Name.Space))
	}
	if end.Name.Space != frame.prefix || end.Name.Local != frame.name.Local || name != frame.name {
		return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "end element does not match start element", nil)
	}
	n := frame.node
	if n != nil && n.text != nil {
		n.Text = string(n.text)
		n.text = nil
	}
	s.stack = s.stack[:len(s.stack)-1]
	s.ns.Pop()
	return nil
}

func (s *schemaParseState) chars(t []byte, line, col int) error {
	if err := checkSchemaTokenLimit(int64(len(t)), s.limits, line, col, "schema XML text exceeds configured limit"); err != nil {
		return err
	}
	if len(s.stack) == 0 {
		if !lex.IsXMLWhitespaceBytes(t) {
			return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "schema XML text outside root element", nil)
		}
		return nil
	}
	last := len(s.stack) - 1
	s.stack[last].textBytes += int64(len(t))
	if err := checkSchemaTokenLimit(s.stack[last].textBytes, s.limits, line, col, "schema XML text exceeds configured limit"); err != nil {
		return err
	}
	if s.stack[last].node == nil || s.annotationPayloadEnvelopeOpen() {
		return nil
	}
	n := s.stack[last].node
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

func (s *schemaParseState) annotationPayloadEnvelopeOpen() bool {
	if len(s.stack) < 2 {
		return false
	}
	n := s.stack[len(s.stack)-1].node
	parent := s.stack[len(s.stack)-2].node
	if n == nil || parent == nil || parent.Name.Space != vocab.XSDNamespaceURI || parent.Name.Local != annotationChild {
		return false
	}
	return n.Name.Space == vocab.XSDNamespaceURI &&
		(n.Name.Local == vocab.XSDElemAppinfo || n.Name.Local == vocab.XSDElemDocumentation)
}

func admitSchemaDocument(doc *rawDoc) error {
	if err := validateSchemaRoot(doc.root); err != nil {
		return err
	}
	if err := rejectUnsupportedSchemaNodes(doc.root, nil); err != nil {
		return err
	}
	if err := validateSchemaTopLevelOrder(doc.root); err != nil {
		return err
	}
	if err := rejectUnknownSchemaAttributes(doc.root); err != nil {
		return err
	}
	if err := checkSchemaIDs(doc.root); err != nil {
		return err
	}
	if err := rejectInvalidSchemaNames(doc.root, nil); err != nil {
		return err
	}
	if err := rejectInvalidAnnotations(doc.root); err != nil {
		return err
	}
	return rejectInvalidSchemaTextAndDirectives(doc.root)
}

func validateSchemaTopLevelOrder(root *rawNode) error {
	sawDeclaration := false
	for child := range root.xsdChildren() {
		switch child.Name.Local {
		case annotationChild:
			continue
		case includeChild, importChild:
			if sawDeclaration {
				return schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "xs:"+child.Name.Local+" must precede global declarations")
			}
		default:
			sawDeclaration = true
		}
	}
	return nil
}

func rejectInvalidSchemaTextAndDirectives(n *rawNode) error {
	if n.Name.Space == vocab.XSDNamespaceURI && n.Name.Local != vocab.XSDElemAppinfo &&
		n.Name.Local != vocab.XSDElemDocumentation && lex.TrimXMLWhitespaceString(n.Text) != "" {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "xs:"+n.Name.Local+" cannot contain text")
	}
	if n.Name.Space == vocab.XSDNamespaceURI && (n.Name.Local == includeChild || n.Name.Local == importChild) {
		if err := checkChildOrderRules(n, annotationOnlyChildOrder(n.Name.Local)); err != nil {
			return err
		}
	}
	for _, child := range n.Children {
		if err := rejectInvalidSchemaTextAndDirectives(child); err != nil {
			return err
		}
	}
	return nil
}

func (s *schemaParseState) ValidateDirective(kind stream.TokenKind, first, second []byte, line, col int) error {
	switch kind {
	case stream.KindDirective:
		if err := checkSchemaTokenLimit(int64(len(first)), s.limits, line, col, "schema XML directive exceeds configured limit"); err != nil {
			return err
		}
		if stream.IsDOCTYPEDeclaration(first) {
			return xsderrors.UnsupportedAt(xsderrors.CodeUnsupportedDTD, line, col, "", "DTD declarations are not supported", nil)
		}
		return xsderrors.SchemaParse(xsderrors.CodeSchemaXML, line, col, "invalid schema XML", nil)
	case stream.KindPI:
		return checkSchemaTokenLimit(int64(len(first)+len(second)), s.limits, line, col, "schema XML processing instruction exceeds configured limit")
	case stream.KindComment:
		return checkSchemaTokenLimit(int64(len(first)), s.limits, line, col, "schema XML comment exceeds configured limit")
	default:
		return xsderrors.InternalInvariant("unexpected schema directive token")
	}
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
	if err := checkXMLBaseAttribute(n); err != nil {
		return err
	}
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
			if n.collapseAttributeWhitespace(local) {
				return lex.CollapseXMLWhitespace(a.Value), true
			}
			return a.Value, true
		}
	}
	return "", false
}

func (n *rawNode) collapseAttributeWhitespace(local string) bool {
	switch local {
	case vocab.XSDAttrDefault, vocab.XSDAttrValue:
		return false
	case vocab.XSDAttrFixed:
		return n.Name.Space == vocab.XSDNamespaceURI &&
			n.Name.Local != vocab.XSDElemElement && n.Name.Local != vocab.XSDElemAttribute
	default:
		return true
	}
}

func (n *rawNode) attrValue(local string) string {
	if v, ok := n.attr(local); ok {
		return v
	}
	return ""
}

func (n *rawNode) attrNS(namespace, local string) (string, bool) {
	for _, a := range n.Attr {
		if a.Name.Space == namespace && a.Name.Local == local {
			if namespace == vocab.XMLNamespaceURI && local == vocab.XMLAttrBase {
				return lex.CollapseXMLWhitespace(a.Value), true
			}
			return lex.ReplaceXMLWhitespace(a.Value), true
		}
	}
	return "", false
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
