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

// typedSchemaParseFrame owns only the active element's admission facts. A
// completed element is published as a schemaNode; no generic XML node tree is
// retained while parsing the next sibling.
type typedSchemaParseFrame struct {
	node      *schemaNode
	name      xml.Name
	namespace xmlstream.Handle
	syntax    schemaSyntaxNode
	textBytes int64
	content   childContent
	opaque    bool
}

// childContent classifies the children a schema element admits as each child
// starts. The (annotation?) content models of the schema for schemas and the
// (appinfo | documentation)* content of xs:annotation are enforced here, so the
// first violation in document order is reported before the offending child's
// own XSD 1.0 checks run or its subtree is admitted. XSD 1.1 vocabulary is
// reported before this check; see rejectXSD11Vocabulary. Richer content models
// are checked when their component compiles.
type childContent uint8

const (
	childContentDeferred childContent = iota
	childContentAnnotationOnly
	childContentAnnotationPayload
)

// childContentFor classifies the content of the element started by start
// whose parent is parent. group and
// attributeGroup are references, with (annotation?) content, except as
// children of xs:schema. Element and attribute references have (annotation?)
// content by src-element.2.2 and src-attribute.3.2; at the top level their ref
// is itself an attribute error reported at their start.
func childContentFor(start *schemaSyntaxNode, parent xml.Name) childContent {
	name := start.Name
	if name.Space != vocab.XSDNamespaceURI {
		return childContentDeferred
	}
	switch name.Local {
	case annotationChild:
		return childContentAnnotationPayload
	case includeChild, importChild, notationChild, selectorChild, fieldChild, anyChild, anyAttribute:
		return childContentAnnotationOnly
	case groupChild, attributeGroup:
		if isXSDName(parent, vocab.XSDElemSchema) {
			return childContentDeferred
		}
		return childContentAnnotationOnly
	case elementChild, attributeChild:
		if _, isReference := start.attr(vocab.XSDAttrRef); isReference {
			return childContentAnnotationOnly
		}
		return childContentDeferred
	}
	if isFacetLocal(name.Local) {
		return childContentAnnotationOnly
	}
	return childContentDeferred
}

type typedSchemaIDRecord struct {
	node  *schemaNode
	value string
}

type typedSchemaParseState struct {
	parser             *xmlstream.Reader
	doc                *schemaDocument
	root               *schemaNode
	stack              []typedSchemaParseFrame
	ids                []typedSchemaIDRecord
	limits             Limits
	nodes              int
	rootSawDeclaration bool
}

// parseTypedSchemaSourceDocument admits XML directly into compact semantic
// source records. Syntax nodes are used only as transient admission facts for
// the active element and its direct children. The caller owns parser and may
// reuse it sequentially; this function detaches it before returning.
func parseTypedSchemaSourceDocument(name, key string, input io.Reader, limits Limits, parser *xmlstream.Reader) (*schemaDocument, error) {
	if parser == nil {
		return nil, xsderrors.InternalInvariant("schema parser reader is nil")
	}
	doc := &schemaDocument{name: name, key: key}
	if err := parser.Reset(input, xmlstream.Config{
		Limits: xmlstream.Limits{
			MaxTokenBytes: limits.MaxSchemaTokenBytes,
			MaxAttrs:      limits.MaxSchemaAttributes,
			MaxDepth:      limits.MaxSchemaDepth,
		},
		CommentMode: xmlstream.CommentModeBoundedDiscard,
		EmitPI:      true,
	}); err != nil {
		return nil, xsderrors.WithLocation(name, 0, 0, schemaReaderError(err, limits))
	}
	defer parser.Detach()
	state := typedSchemaParseState{parser: parser, doc: doc, limits: limits}
	if err := state.parse(); err != nil {
		return nil, xsderrors.WithLocation(name, 0, 0, err)
	}
	if state.root == nil {
		return nil, xsderrors.WithLocation(name, 0, 0,
			schemaParseAt(0, 0, xsderrors.CodeSchemaRoot, "empty schema document", nil))
	}
	if state.root.local != vocab.XSDElemSchema {
		return nil, xsderrors.WithLocation(name, state.root.line, state.root.column,
			schemaParseAt(state.root.line, state.root.column, xsderrors.CodeSchemaRoot, "root element must be xs:schema", nil))
	}
	// Lexical ids are checked only after the whole document is admitted, so a
	// structural error earlier in the document wins over a duplicate id.
	if err := state.validateIDs(); err != nil {
		return nil, xsderrors.WithLocation(name, 0, 0, err)
	}
	doc.root = state.root
	doc.nodes = state.nodes
	return doc, nil
}

func (s *typedSchemaParseState) parse() error {
	for {
		tok, err := s.parser.Next()
		if xmlstream.IsEOF(err) {
			break
		}
		if err != nil {
			line, col := s.parser.Pos()
			return schemaStreamError(line, col, err, s.limits)
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

func (s *typedSchemaParseState) handleToken(tok *xmlstream.Token) error {
	switch tok.Kind { //nolint:exhaustive // Reader.Next rejects directives before consumers see them.
	case xmlstream.KindStart:
		return s.start(tok.Start, tok.Line, tok.Column)
	case xmlstream.KindEnd:
		return s.end(tok.Line, tok.Column)
	case xmlstream.KindCharData:
		return s.chars(tok.Data, tok.Line, tok.Column)
	case xmlstream.KindComment:
		return xsderrors.InternalInvariant("schema parser received a comment token despite comment discard mode")
	default: // processing instructions are ignored.
		return nil
	}
}

func (s *typedSchemaParseState) start(start xmlstream.StartElement, line, col int) error {
	if err := s.validateStartLimits(line, col); err != nil {
		return err
	}
	namespaceFrame, element, err := s.parser.Start()
	if err != nil {
		return schemaStreamError(line, col, err, s.limits)
	}
	// start is a by-value view of the current token; only the copied element
	// name is normalized and no borrowed token data is retained.
	start.Name = element

	parent := s.parentFrame()
	if rootErr := s.validateRootStart(parent, line, col); rootErr != nil {
		return s.abortStart(namespaceFrame, rootErr)
	}
	opaque := typedStartOpaque(parent, start.Name)
	if typedOpaqueDescendant(parent) {
		s.appendOpaqueFrame(start.Name, namespaceFrame, opaque)
		return nil
	}

	s.nodes++
	syntax, err := s.startSyntax(start, line, col)
	if err != nil {
		return s.abortStart(namespaceFrame, err)
	}
	frame := typedSchemaParseFrame{
		name:      start.Name,
		namespace: namespaceFrame,
		syntax:    syntax,
		content:   childContentFor(&syntax, typedParentName(parent)),
		opaque:    opaque,
	}
	if err := s.admitTypedFrame(frame, line, col); err != nil {
		return s.abortStart(namespaceFrame, err)
	}
	return nil
}

func typedParentName(parent *typedSchemaParseFrame) xml.Name {
	if parent == nil {
		return xml.Name{}
	}
	return parent.name
}

func (s *typedSchemaParseState) appendOpaqueFrame(name xml.Name, namespace xmlstream.Handle, opaque bool) {
	s.nodes++
	s.stack = append(s.stack, typedSchemaParseFrame{name: name, namespace: namespace, opaque: opaque})
}

func (s *typedSchemaParseState) admitTypedFrame(initial typedSchemaParseFrame, line, col int) error {
	parentIndex := len(s.stack) - 1
	s.stack = append(s.stack, initial)
	frame := &s.stack[len(s.stack)-1]
	var parent *typedSchemaParseFrame
	if parentIndex >= 0 {
		parent = &s.stack[parentIndex]
	}
	if err := admitTypedStart(&frame.syntax, parent); err != nil {
		s.stack = s.stack[:len(s.stack)-1]
		return err
	}
	var node *schemaNode
	if !frame.opaque {
		var err error
		node, err = s.admitTypedNode(frame.name, frame.syntax.Attrs, parent, line, col)
		if err != nil {
			s.stack = s.stack[:len(s.stack)-1]
			return err
		}
	}
	if err := s.admitRootChild(parent, frame.name, &frame.syntax); err != nil {
		s.stack = s.stack[:len(s.stack)-1]
		return err
	}
	frame.node = node
	if node != nil {
		s.recordTypedID(node, &frame.syntax)
	}
	return nil
}

func (s *typedSchemaParseState) validateRootStart(parent *typedSchemaParseFrame, line, col int) error {
	if parent == nil && s.root != nil {
		return schemaParseAt(line, col, xsderrors.CodeSchemaRoot, "schema document has multiple roots", nil)
	}
	return nil
}

func (s *typedSchemaParseState) startSyntax(start xmlstream.StartElement, line, col int) (schemaSyntaxNode, error) {
	attrs, err := schemaAttributesStream(s.parser, start.Name, start.Attr)
	if err != nil {
		return schemaSyntaxNode{}, err
	}
	return schemaSyntaxNode{
		doc:    s.doc,
		Name:   start.Name,
		NS:     s.parser.Context(),
		Attrs:  attrs,
		Line:   line,
		Column: col,
	}, nil
}

func admitTypedStart(n *schemaSyntaxNode, parent *typedSchemaParseFrame) error {
	if !shouldAdmitTypedStart(parent) {
		return nil
	}
	return admitStart(n, parent)
}

func (s *typedSchemaParseState) recordTypedID(node *schemaNode, syntax *schemaSyntaxNode) {
	if node == nil {
		return
	}
	if idValue, ok := syntax.attr(vocab.XSDAttrID); ok {
		s.ids = append(s.ids, typedSchemaIDRecord{node: node, value: idValue})
	}
}

func (s *typedSchemaParseState) validateStartLimits(line, col int) error {
	if s.nodes >= s.limits.MaxSchemaInstantiatedNodes {
		return schemaParseAt(line, col, xsderrors.CodeSchemaLimit, "schema nodes exceed MaxSchemaInstantiatedNodes", nil)
	}
	return nil
}

func shouldAdmitTypedStart(parent *typedSchemaParseFrame) bool {
	return parent == nil || !parent.opaque
}

// typedStartOpaque marks xs:appinfo and xs:documentation payload as opaque:
// it is consumed as namespace-well-formed XML and counted against node limits,
// but never admitted as schema syntax or retained.
func typedStartOpaque(parent *typedSchemaParseFrame, name xml.Name) bool {
	if parent == nil {
		return false
	}
	if parent.opaque {
		return true
	}
	return isXSDName(parent.name, annotationChild) && typedAnnotationEnvelope(name)
}

func typedOpaqueDescendant(parent *typedSchemaParseFrame) bool {
	return parent != nil && parent.opaque
}

func typedAnnotationEnvelope(name xml.Name) bool {
	return isXSDName(name, vocab.XSDElemAppinfo) || isXSDName(name, vocab.XSDElemDocumentation)
}

func isXSDName(name xml.Name, local string) bool {
	return name.Space == vocab.XSDNamespaceURI && name.Local == local
}

func (s *typedSchemaParseState) admitTypedNode(name xml.Name, attrs []schemaAttribute, parent *typedSchemaParseFrame, line, col int) (*schemaNode, error) {
	id, err := schemaNodeIDFor(len(s.doc.arena))
	if err != nil {
		return nil, schemaParseAt(line, col, xsderrors.CodeSchemaLimit, err.Error(), err)
	}
	var attributeMask uint64
	for _, attr := range attrs {
		if attr.Name.Space == "" {
			attributeMask |= typedAttributeMaskFor(attr.Name.Local)
		}
	}
	node := &schemaNode{
		doc:           s.doc,
		id:            id,
		local:         name.Local,
		attributeMask: attributeMask,
		line:          line,
		column:        col,
		namespace:     s.parser.Context(),
	}
	s.doc.arena = append(s.doc.arena, node)
	s.attachTypedNode(node, parent)
	return node, nil
}

func (s *typedSchemaParseState) attachTypedNode(node *schemaNode, parent *typedSchemaParseFrame) {
	if parent != nil && parent.node != nil {
		parent.node.children = append(parent.node.children, node.id)
		return
	}
	if parent == nil {
		s.root = node
		s.doc.root = node
	}
}

func (s *typedSchemaParseState) admitRootChild(parent *typedSchemaParseFrame, name xml.Name, syntax *schemaSyntaxNode) error {
	if parent == nil || parent.node != s.root || name.Space != vocab.XSDNamespaceURI {
		return nil
	}
	if name.Local != annotationChild && name.Local != includeChild && name.Local != importChild {
		s.rootSawDeclaration = true
	}
	if (name.Local == includeChild || name.Local == importChild) && s.rootSawDeclaration {
		return schemaCompileAt(syntax, xsderrors.CodeSchemaContentModel, "xs:"+diagnostictext.Quote(name.Local)+" must precede global declarations")
	}
	return nil
}

func (s *typedSchemaParseState) parentFrame() *typedSchemaParseFrame {
	if len(s.stack) == 0 {
		return nil
	}
	return &s.stack[len(s.stack)-1]
}

// admitStart checks XSD 1.1 vocabulary before any other schema rule; see
// rejectXSD11Vocabulary.
func admitStart(n *schemaSyntaxNode, parent *typedSchemaParseFrame) error {
	if n.Name.Space == vocab.XSDNamespaceURI {
		if err := rejectXSD11Vocabulary(n); err != nil {
			return err
		}
	}
	if parent == nil {
		return admitRootStart(n)
	}
	if n.Name.Space != vocab.XSDNamespaceURI {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "foreign element "+diagnostictext.Quote(n.Name.Local)+" is not allowed in schema grammar")
	}
	if err := admitChildContent(n, parent); err != nil {
		return err
	}
	if parent.content == childContentAnnotationPayload {
		return admitAnnotationEnvelopeStart(n, parent.name)
	}
	return admitXSDChildStart(n, parent.name)
}

// admitChildContent checks the schema-namespace child n against the content
// class of parent. Text earlier in parent precedes n in document order, so
// its diagnostic wins over n's XSD 1.0 violations, though not over XSD 1.1
// vocabulary, which rejectXSD11Vocabulary reports first.
func admitChildContent(n *schemaSyntaxNode, parent *typedSchemaParseFrame) error {
	var message string
	switch parent.content {
	case childContentDeferred:
		return nil
	case childContentAnnotationPayload:
		if typedAnnotationEnvelope(n.Name) {
			return nil
		}
		message = "xs:" + diagnostictext.Quote(parent.name.Local) + " can contain only xs:appinfo and xs:documentation, not xs:" + diagnostictext.Quote(n.Name.Local)
	case childContentAnnotationOnly:
		switch {
		case n.Name.Local != annotationChild:
			message = "xs:" + diagnostictext.Quote(parent.name.Local) + " can contain only xs:annotation, not xs:" + diagnostictext.Quote(n.Name.Local)
		case len(parent.node.children) != 0:
			message = "xs:" + diagnostictext.Quote(parent.name.Local) + " can contain at most one xs:annotation"
		default:
			return nil
		}
	}
	if err := rejectInvalidSchemaTextTyped(parent.node); err != nil {
		return err
	}
	return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, message)
}

func admitRootStart(n *schemaSyntaxNode) error {
	if !isXSDName(n.Name, vocab.XSDElemSchema) {
		return schemaCompileAt(n, xsderrors.CodeSchemaRoot, "root element must be xs:schema")
	}
	if err := rejectSchemaNamespaceAttributes(n); err != nil {
		return err
	}
	if err := checkSchemaNodeNamesSyntax(n, nil); err != nil {
		return err
	}
	if err := checkXMLBaseAttributeSyntax(n); err != nil {
		return err
	}
	return checkSchemaAttributesSyntax(n, false)
}

func admitAnnotationEnvelopeStart(n *schemaSyntaxNode, parent xml.Name) error {
	// The payload envelope is admitted separately; arbitrary descendants are
	// skipped by the caller after this attribute and annotation check.
	if err := rejectSchemaNamespaceAttributes(n); err != nil {
		return err
	}
	if err := checkSchemaNodeNamesSyntax(n, &schemaSyntaxNode{Name: parent}); err != nil {
		return err
	}
	if err := checkXMLBaseAttributeSyntax(n); err != nil {
		return err
	}
	if err := checkSchemaAttributesSyntax(n, false); err != nil {
		return err
	}
	return checkAnnotationEnvelopeStart(n)
}

// admitXSDChildStart runs start checks in a fixed order that decides which
// diagnostic wins when a node violates several rules. It runs after
// rejectXSD11Vocabulary and admitChildContent, so XSD 1.1 vocabulary and a
// child that its parent's content forbids report that violation rather than
// the child's own. The syntax facts it reads are transient; compilation never
// treats them as a generic XML tree. Reference and local-declaration attribute
// restrictions are compiler-owned.
func admitXSDChildStart(n *schemaSyntaxNode, parent xml.Name) error {
	if err := rejectSchemaNamespaceAttributes(n); err != nil {
		return err
	}
	if err := checkSchemaNodeNamesSyntax(n, &schemaSyntaxNode{Name: parent}); err != nil {
		return err
	}
	if err := checkUnsupportedXSDNodeSyntax(n, parent); err != nil {
		return err
	}
	topLevel := isXSDName(parent, vocab.XSDElemSchema)
	if err := checkXMLBaseAttributeSyntax(n); err != nil {
		return err
	}
	if err := checkSchemaAttributesSyntax(n, topLevel); err != nil {
		return err
	}
	if topLevel {
		return checkTopLevelSchemaChildSyntax(n)
	}
	return nil
}

func checkAnnotationEnvelopeStart(n *schemaSyntaxNode) error {
	if n.Name.Local != vocab.XSDElemDocumentation {
		return nil
	}
	for _, attr := range n.Attrs {
		if attr.Name.Space == vocab.XMLNamespaceURI && attr.Name.Local == vocab.XMLAttrLang && !lex.IsLanguage(lex.CollapseXMLWhitespace(attr.Value)) {
			return schemaCompileAt(n, xsderrors.CodeSchemaInvalidAttribute, "invalid xml:lang on xs:documentation")
		}
	}
	return nil
}

func (s *typedSchemaParseState) end(line, col int) error {
	if len(s.stack) == 0 {
		return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "unexpected end element", nil)
	}
	index := len(s.stack) - 1
	frame := &s.stack[index]
	if err := s.parser.MatchEnd(frame.namespace); err != nil {
		return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "invalid schema XML", err)
	}
	if err := s.parser.CommitEnd(frame.namespace); err != nil {
		return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "invalid schema XML", err)
	}
	s.stack = s.stack[:index]
	if frame.node == nil {
		return nil
	}
	return completeTypedNode(frame)
}

func completeTypedNode(frame *typedSchemaParseFrame) error {
	if err := frame.syntax.buildSemanticSource(frame.node); err != nil {
		return withSchemaCompileLocation(frame.node, err)
	}
	if !schemaNodeNeedsDeferredQNameContext(frame.node, frame.syntax.Attrs) {
		frame.node.namespace = xmlstream.Context{}
	}
	if err := checkAnnotationEnvelopeStart(&frame.syntax); err != nil {
		return err
	}
	return admitEnd(frame.node)
}

// schemaNodeNeedsDeferredQNameContext identifies source values whose QName
// interpretation is intentionally deferred until the owning type or XPath is
// compiled. Schema QName attributes are resolved while the element is active,
// so their namespace bindings do not need to survive admission.
func schemaNodeNeedsDeferredQNameContext(n *schemaNode, attrs []schemaAttribute) bool {
	if n == nil {
		return false
	}
	has := func(local string) bool {
		for _, attr := range attrs {
			if attr.Name.Space == "" && attr.Name.Local == local {
				return true
			}
		}
		return false
	}
	switch n.local {
	case vocab.XSDElemElement, vocab.XSDElemAttribute:
		return has(vocab.XSDAttrDefault) || has(vocab.XSDAttrFixed)
	case vocab.XSDFacetEnumeration:
		return has(vocab.XSDAttrValue)
	case vocab.XSDElemSelector, vocab.XSDElemField:
		return has(vocab.XSDAttrXPath)
	default:
		return false
	}
}

func admitEnd(n *schemaNode) error {
	if err := rejectInvalidSchemaTextTyped(n); err != nil {
		return err
	}
	return checkSchemaAnnotationNode(n)
}

func rejectInvalidSchemaTextTyped(n *schemaNode) error {
	if n == nil || n.local == vocab.XSDElemAppinfo || n.local == vocab.XSDElemDocumentation {
		return nil
	}
	if n.hasNonWhitespaceText {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "xs:"+diagnostictext.Quote(n.local)+" cannot contain text")
	}
	return nil
}

// checkSchemaAnnotationNode enforces a single leading xs:annotation in the
// content of n. Children of xs:annotation and of (annotation?) elements are
// admitted by admitChildContent before this runs.
func checkSchemaAnnotationNode(n *schemaNode) error {
	if n.local == vocab.XSDElemSchema || n.local == vocab.XSDElemAppinfo || n.local == vocab.XSDElemDocumentation {
		return nil
	}
	seenAnnotation, seenNonAnnotation := false, false
	for child := range n.xsdChildren() {
		if child.local != annotationChild {
			seenNonAnnotation = true
			continue
		}
		if seenAnnotation {
			return schemaCompileAt(child, xsderrors.CodeSchemaContentModel, "schema component cannot contain multiple annotations")
		}
		if seenNonAnnotation {
			return schemaCompileAt(child, xsderrors.CodeSchemaContentModel, diagnostictext.Quote(n.local)+" annotation must be first")
		}
		seenAnnotation = true
	}
	return nil
}

func (s *typedSchemaParseState) chars(t []byte, line, col int) error {
	if len(s.stack) == 0 {
		return nil
	}
	last := len(s.stack) - 1
	s.stack[last].textBytes += int64(len(t))
	// The reader enforces per-token bytes; only text accumulated across
	// tokens within one element is invisible to it.
	if limit := s.limits.MaxSchemaTokenBytes; limit > 0 && s.stack[last].textBytes > limit {
		return schemaParseAt(line, col, xsderrors.CodeSchemaLimit, "MaxSchemaTokenBytes="+strconv.FormatInt(limit, 10)+" exceeded: schema element character data too large", nil)
	}
	if s.stack[last].node == nil || s.stack[last].opaque {
		return nil
	}
	if !lex.IsXMLWhitespaceBytes(t) {
		s.stack[last].node.hasNonWhitespaceText = true
	}
	return nil
}

func (s *typedSchemaParseState) abortStart(frame xmlstream.Handle, cause error) error {
	if cause == nil {
		return nil
	}
	if abortErr := s.parser.AbortStart(frame); abortErr != nil {
		return errors.Join(cause, abortErr)
	}
	return cause
}

func (s *typedSchemaParseState) validateIDs() error {
	seen := make(map[string]bool, len(s.ids))
	for _, id := range s.ids {
		if seen[id.value] {
			return schemaCompileAt(id.node, xsderrors.CodeSchemaInvalidAttribute, "duplicate schema id "+diagnostictext.Quote(id.value))
		}
		seen[id.value] = true
	}
	return nil
}
