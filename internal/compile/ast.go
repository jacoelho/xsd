package compile

import (
	"bytes"
	"encoding/xml"
	"errors"
	"iter"

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
	NS       xmlns.Context
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
		return nil, xsderrors.WithLocation(name, 0, 0, admitErr)
	}
	defaults, err := parseSchemaDefaults(doc.root)
	if err != nil {
		return nil, xsderrors.WithLocation(name, 0, 0, err)
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
	if err := parser.ResetWithConfig(bytes.NewReader(data), &names, &values, stream.Config{
		Limits: stream.Limits{
			MaxTokenBytes: limits.MaxSchemaTokenBytes,
			MaxAttrs:      limits.MaxSchemaAttributes,
		},
		EmitComments: true,
		EmitPI:       true,
	}); err != nil {
		return nil, xsderrors.WithLocation(name, 0, 0, schemaReaderError(err))
	}
	defer parser.Detach()
	state := schemaParseState{
		parser: parser,
		values: &values,
		doc:    doc,
		limits: limits,
	}
	if err := state.parse(); err != nil {
		return nil, xsderrors.WithLocation(name, 0, 0, err)
	}
	doc.root, doc.nodes = state.root, state.nodes
	return doc, nil
}

type schemaParseFrame struct {
	node      *rawNode
	namespace xmlns.Frame
	textBytes int64
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
		if stream.IsOnlyEOF(err) {
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
		return schemaParseAt(0, 0, xsderrors.CodeSchemaXML, "unclosed schema element", nil)
	}
	return nil
}

func schemaReaderError(err error) error {
	return schemaStreamError(0, 0, err)
}

func schemaStreamError(line, col int, err error) error {
	if errors.Is(err, stream.ErrUnsupportedNonUTF8) {
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, "schema documents must be UTF-8", err)
	}
	var versionErr stream.UnsupportedXMLVersionError
	if errors.As(err, &versionErr) {
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedXML11, versionErr.Error(), nil)
	}
	if stream.IsTokenLimit(err) || stream.IsAttributeLimit(err) {
		return schemaParseAt(line, col, xsderrors.CodeSchemaLimit, err.Error(), err)
	}
	return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "invalid schema XML", err)
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
	if err := s.validateStartLimits(start, line, col); err != nil {
		return err
	}
	namespaceFrame, element, err := s.ns.StartStream(&start, s.values)
	if err != nil {
		return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "invalid schema XML", err)
	}
	prepared, err := s.prepareSchemaStart(start, element, line, col)
	if abortErr := s.abortStart(namespaceFrame, err); abortErr != nil {
		return abortErr
	}
	opaque := (len(s.stack) != 0 && s.stack[len(s.stack)-1].node == nil) || s.annotationPayloadEnvelopeOpen()
	var n *rawNode
	if !opaque {
		n = &rawNode{doc: s.doc, Name: prepared.Name, Attr: prepared.Attr, NS: s.ns.Context(), Line: line, Column: col}
	}
	if len(s.stack) == 0 && s.root != nil {
		multipleRoots := schemaParseAt(line, col, xsderrors.CodeSchemaRoot, "schema document has multiple roots", nil)
		return s.abortStart(namespaceFrame, multipleRoots)
	}
	s.nodes++
	s.attachNode(n)
	s.stack = append(s.stack, schemaParseFrame{node: n, namespace: namespaceFrame})
	return nil
}

func (s *schemaParseState) validateStartLimits(start stream.StartElement, line, col int) error {
	if s.nodes >= s.limits.MaxSchemaInstantiatedNodes {
		return schemaParseAt(line, col, xsderrors.CodeSchemaLimit, "schema nodes exceed MaxSchemaInstantiatedNodes", nil)
	}
	if s.limits.MaxSchemaDepth > 0 && len(s.stack)+1 > s.limits.MaxSchemaDepth {
		return schemaParseAt(line, col, xsderrors.CodeSchemaLimit, "schema XML nesting exceeds configured limit", nil)
	}
	if s.limits.MaxSchemaAttributes > 0 && len(start.Attr) > s.limits.MaxSchemaAttributes {
		return schemaParseAt(line, col, xsderrors.CodeSchemaLimit, "schema XML attributes exceed configured limit", nil)
	}
	return nil
}

func (s *schemaParseState) abortStart(frame xmlns.Frame, cause error) error {
	if cause == nil {
		return nil
	}
	if abortErr := s.ns.Abort(frame); abortErr != nil {
		return errors.Join(cause, abortErr)
	}
	return cause
}

func (s *schemaParseState) attachNode(n *rawNode) {
	if n == nil {
		return
	}
	if len(s.stack) == 0 {
		s.root = n
		return
	}
	parent := s.stack[len(s.stack)-1].node
	parent.Children = append(parent.Children, n)
}

func (s *schemaParseState) prepareSchemaStart(start stream.StartElement, element xmlns.Element, line, col int) (xml.StartElement, error) {
	attrs := make([]xml.Attr, len(start.Attr))
	for i := range start.Attr {
		attrs[i] = xml.Attr{Name: start.Attr[i].Name, Value: start.Attr[i].StringValue(s.values)}
	}
	prepared := xml.StartElement{Name: element.Name, Attr: attrs}
	if err := checkSchemaStartElementLimit(prepared, s.limits, line, col); err != nil {
		return xml.StartElement{}, err
	}
	return prepared, nil
}

func (s *schemaParseState) handleEndElement(end stream.EndElement, line, col int) error {
	if len(s.stack) == 0 {
		return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "unexpected end element", nil)
	}
	frame := s.stack[len(s.stack)-1]
	if err := s.ns.End(frame.namespace, xmlns.Lexical(end.Name)); err != nil {
		return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "invalid schema XML", err)
	}
	n := frame.node
	if n != nil && n.text != nil {
		n.Text = string(n.text)
		n.text = nil
	}
	s.stack = s.stack[:len(s.stack)-1]
	return nil
}

func (s *schemaParseState) chars(t []byte, line, col int) error {
	if err := checkSchemaTokenLimit(int64(len(t)), s.limits, line, col, "schema XML text exceeds configured limit"); err != nil {
		return err
	}
	if len(s.stack) == 0 {
		if !lex.IsXMLWhitespaceBytes(t) {
			return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "schema XML text outside root element", nil)
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
	if err := rejectInvalidSchemaText(n); err != nil {
		return err
	}
	if err := rejectInvalidReferenceDirectives(n); err != nil {
		return err
	}
	for _, child := range n.Children {
		if err := rejectInvalidSchemaTextAndDirectives(child); err != nil {
			return err
		}
	}
	return nil
}

func rejectInvalidSchemaText(n *rawNode) error {
	if n.Name.Space != vocab.XSDNamespaceURI || n.Name.Local == vocab.XSDElemAppinfo || n.Name.Local == vocab.XSDElemDocumentation {
		return nil
	}
	if lex.TrimXMLWhitespaceString(n.Text) != "" {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "xs:"+n.Name.Local+" cannot contain text")
	}
	return nil
}

func rejectInvalidReferenceDirectives(n *rawNode) error {
	if n.Name.Space != vocab.XSDNamespaceURI || n.Name.Local != includeChild && n.Name.Local != importChild {
		return nil
	}
	return checkChildOrderRules(n, annotationOnlyChildOrder(n.Name.Local))
}

func (s *schemaParseState) ValidateDirective(kind stream.TokenKind, first, second []byte, line, col int) error {
	switch kind {
	case stream.KindDirective:
		if err := checkSchemaTokenLimit(int64(len(first)), s.limits, line, col, "schema XML directive exceeds configured limit"); err != nil {
			return err
		}
		if stream.IsDOCTYPEDeclaration(first) {
			return xsderrors.WithLocation("", line, col, xsderrors.Unsupported(xsderrors.CodeUnsupportedDTD, "DTD declarations are not supported", nil))
		}
		return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "invalid schema XML", nil)
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
		return schemaParseAt(0, 0, xsderrors.CodeSchemaRoot, "empty schema document", nil)
	}
	if root.Name.Space != vocab.XSDNamespaceURI || root.Name.Local != vocab.XSDElemSchema {
		return schemaParseAt(root.Line, root.Column, xsderrors.CodeSchemaRoot, "root element must be xs:schema", nil)
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
		limitErr := schemaParseAt(line, col, xsderrors.CodeSchemaLimit, msg, nil)
		return limitErr
	}
	return nil
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
		ns, _ := n.NS.Lookup("")
		return ns, local, nil
	}
	ns, ok := n.NS.Lookup(prefix)
	if !ok {
		return "", "", schemaCompileAt(n, xsderrors.CodeSchemaReference, "unbound QName prefix "+prefix)
	}
	return ns, local, nil
}
