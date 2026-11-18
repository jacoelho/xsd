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
	node *schemaNode
	// order is the content model that childContentFor selected for an
	// element whose content is childContentOrdered or childContentSchema;
	// orders is its progress through it.
	order     *childOrder
	name      xml.Name
	namespace xmlstream.Handle
	syntax    schemaSyntaxNode
	orders    childOrderState
	textBytes int64
	content   childContent
	placement schemaElementPlacement
	opaque    bool
}

// childContent classifies the children a schema element admits as each child
// starts, so the first violation in document order is reported before the
// offending child's own XSD 1.0 checks run or its subtree is admitted. The
// (annotation?) content models of the schema for schemas, the
// (appinfo | documentation)* content of xs:annotation, the placement of
// xs:appinfo and xs:documentation, and the single leading xs:annotation of
// every component are decided here; every other content model is the
// childOrder of the element. Unsupported vocabulary is reported before this
// check; see rejectUnsupportedVocabulary.
type childContent uint8

const (
	childContentOrdered childContent = iota
	childContentAnnotationOnly
	childContentAnnotationPayload
	// childContentSchema is the content of xs:schema, where xs:annotation may
	// appear anywhere.
	childContentSchema
)

// childContentFor classifies the content of the XSD element named local in
// placement, whose parent is named parent, and selects the childOrder of an
// ordered content. Only XSD-namespace elements reach it.
func childContentFor(local string, placement schemaElementPlacement, parent xml.Name) (childContent, *childOrder) {
	if placement == schemaPlacementReference {
		return childContentAnnotationOnly, nil
	}
	switch local {
	case vocab.XSDElemSchema:
		return childContentSchema, &schemaChildOrder
	case annotationChild:
		return childContentAnnotationPayload, nil
	case includeChild, importChild, notationChild, selectorChild, fieldChild, anyChild, anyAttribute:
		return childContentAnnotationOnly, nil
	}
	if isFacetLocal(local) {
		return childContentAnnotationOnly, nil
	}
	return childContentOrdered, childOrderFor(local, parent)
}

// schemaIDs is the set of id attribute values one schema document declares.
type schemaIDs struct {
	declared map[string]struct{}
}

// declare records value and reports whether the document declares it for the
// first time.
func (ids *schemaIDs) declare(value string) bool {
	if _, duplicate := ids.declared[value]; duplicate {
		return false
	}
	if ids.declared == nil {
		ids.declared = make(map[string]struct{})
	}
	ids.declared[value] = struct{}{}
	return true
}

// startTagAttributes is what the attributes of one start tag are admitted
// into: the values admitted from its attributes, the ids of its document, and
// the pattern admission of its schema set.
type startTagAttributes struct {
	values   *admittedValues
	ids      *schemaIDs
	patterns *schemaPatterns
}

type typedSchemaParseState struct {
	parser   *xmlstream.Reader
	doc      *schemaDocument
	root     *schemaNode
	patterns *schemaPatterns
	ids      schemaIDs
	stack    []typedSchemaParseFrame
	limits   Limits
	nodes    int
}

// parseTypedSchemaSourceDocument admits XML directly into compact semantic
// source records. Syntax nodes are used only as transient admission facts for
// the active element and its direct children. The caller owns parser and may
// reuse it sequentially; this function detaches it before returning.
func parseTypedSchemaSourceDocument(name, key string, input io.Reader, limits Limits, parser *xmlstream.Reader, patterns *schemaPatterns) (*schemaDocument, error) {
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
	state := typedSchemaParseState{parser: parser, doc: doc, limits: limits, patterns: patterns}
	if err := state.parse(); err != nil {
		return nil, xsderrors.WithLocation(name, 0, 0, err)
	}
	if state.root == nil {
		line, col := parser.Pos()
		return nil, xsderrors.WithLocation(name, 0, 0,
			schemaParseAt(line, col, xsderrors.CodeSchemaRoot, "empty schema document", nil))
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
		line, col := s.parser.Pos()
		return schemaParseAt(line, col, xsderrors.CodeSchemaXML, "unclosed schema element", nil)
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
		opaque:    opaque,
	}
	if start.Name.Space == vocab.XSDNamespaceURI {
		parentName := typedParentName(parent)
		frame.placement = schemaElementPlacementOf(&syntax, parentName)
		frame.content, frame.order = childContentFor(start.Name.Local, frame.placement, parentName)
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
	var values admittedValues
	if err := admitTypedStart(&frame.syntax, frame.placement, parent, startTagAttributes{values: &values, ids: &s.ids, patterns: s.patterns}); err != nil {
		s.stack = s.stack[:len(s.stack)-1]
		return err
	}
	var node *schemaNode
	if !frame.opaque {
		var err error
		node, err = s.admitTypedNode(frame.name, parent, line, col)
		if err != nil {
			s.stack = s.stack[:len(s.stack)-1]
			return err
		}
	}
	frame.node = node
	if node != nil {
		frame.syntax.buildSemanticSource(node, &values)
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

func admitTypedStart(n *schemaSyntaxNode, placement schemaElementPlacement, parent *typedSchemaParseFrame, attrs startTagAttributes) error {
	if !shouldAdmitTypedStart(parent) {
		return nil
	}
	return admitStart(n, placement, parent, attrs)
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

func (s *typedSchemaParseState) admitTypedNode(name xml.Name, parent *typedSchemaParseFrame, line, col int) (*schemaNode, error) {
	id, err := schemaNodeIDFor(len(s.doc.arena))
	if err != nil {
		return nil, schemaParseAt(line, col, xsderrors.CodeSchemaLimit, err.Error(), err)
	}
	node := &schemaNode{
		doc:       s.doc,
		id:        id,
		local:     name.Local,
		line:      line,
		column:    col,
		namespace: s.parser.Context(),
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

func (s *typedSchemaParseState) parentFrame() *typedSchemaParseFrame {
	if len(s.stack) == 0 {
		return nil
	}
	return &s.stack[len(s.stack)-1]
}

// admitStart checks unsupported vocabulary before any other rule except that
// the root is xs:schema; see rejectUnsupportedVocabulary. Text earlier in
// parent precedes n in document order, so its diagnostic wins over every other
// violation of n. The attributes of n are admitted into attrs.
func admitStart(n *schemaSyntaxNode, placement schemaElementPlacement, parent *typedSchemaParseFrame, attrs startTagAttributes) error {
	if parent == nil {
		return admitRootStart(n, attrs)
	}
	foreign := n.Name.Space != vocab.XSDNamespaceURI
	if !foreign {
		if err := rejectUnsupportedVocabulary(n); err != nil {
			return err
		}
	}
	if err := rejectInvalidSchemaTextTyped(parent.node); err != nil {
		return err
	}
	if foreign {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "foreign element "+diagnostictext.Quote(n.Name.Local)+" is not allowed in schema grammar")
	}
	if err := admitChildContent(n, parent); err != nil {
		return err
	}
	if parent.content == childContentAnnotationPayload {
		return admitAnnotationEnvelopeStart(n, attrs)
	}
	return admitXSDChildStart(n, parent.name, placement, attrs)
}

// admitChildContent checks the schema-namespace child n against the content
// of parent and admits it there.
func admitChildContent(n *schemaSyntaxNode, parent *typedSchemaParseFrame) error {
	if (parent.content == childContentOrdered || parent.content == childContentSchema) && parent.order == nil {
		return withSchemaCompileLocation(n, xsderrors.InternalInvariant("schema element xs:"+diagnostictext.Quote(parent.name.Local)+" has no child order"))
	}
	if message := childContentViolation(n.Name.Local, parent); message != "" {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, message)
	}
	if parent.content == childContentOrdered || parent.content == childContentSchema {
		return parent.orders.accept(n, parent.order, parent.node)
	}
	return nil
}

// childContentViolation returns why parent cannot contain a schema-namespace
// child named child after the children parent already has, as far as its
// annotation-only content, annotation payload or annotation placement decide,
// or "" when they admit it. The childOrder of an ordered content decides the
// rest; see admitChildContent.
func childContentViolation(child string, parent *typedSchemaParseFrame) string {
	switch parent.content {
	case childContentAnnotationPayload:
		if child == vocab.XSDElemAppinfo || child == vocab.XSDElemDocumentation {
			return ""
		}
		return "xs:" + diagnostictext.Quote(parent.name.Local) + " can contain only xs:appinfo and xs:documentation, not xs:" + diagnostictext.Quote(child)
	case childContentAnnotationOnly:
		if child != annotationChild {
			return "xs:" + diagnostictext.Quote(parent.name.Local) + " can contain only xs:annotation, not xs:" + diagnostictext.Quote(child)
		}
	case childContentOrdered, childContentSchema:
	}
	return annotationPlacementViolation(child, parent)
}

// annotationPlacementViolation returns why child, an xs:annotation,
// xs:appinfo or xs:documentation, cannot appear in parent at this position,
// or "" when it can or is none of them.
func annotationPlacementViolation(child string, parent *typedSchemaParseFrame) string {
	switch child {
	case vocab.XSDElemAppinfo, vocab.XSDElemDocumentation:
		return "xs:" + diagnostictext.Quote(child) + " can appear only in xs:annotation"
	case annotationChild:
		if parent.content != childContentSchema && len(parent.node.children) != 0 {
			return "xs:" + diagnostictext.Quote(parent.name.Local) + " can contain xs:annotation only as its first child"
		}
	}
	return ""
}

func admitRootStart(n *schemaSyntaxNode, attrs startTagAttributes) error {
	if !isXSDName(n.Name, vocab.XSDElemSchema) {
		return schemaCompileAt(n, xsderrors.CodeSchemaRoot, "root element must be xs:schema")
	}
	if err := rejectUnsupportedVocabulary(n); err != nil {
		return err
	}
	return checkSchemaAttributesSyntax(n, schemaPlacementOther, attrs)
}

func admitAnnotationEnvelopeStart(n *schemaSyntaxNode, attrs startTagAttributes) error {
	// The payload envelope is admitted separately; arbitrary descendants are
	// skipped by the caller after this attribute check.
	return checkSchemaAttributesSyntax(n, schemaPlacementOther, attrs)
}

// admitXSDChildStart checks the attributes of n in start-tag order, then the
// attributes its placement requires. It runs after rejectUnsupportedVocabulary
// and admitChildContent, so unsupported vocabulary and a child that its
// parent's content forbids report that violation rather than the child's own.
// The syntax facts it reads are transient; compilation never treats them as a
// generic XML tree. Which unqualified attributes an element admits and
// requires in its placement, and which of them exclude each other, is decided
// here, so those violations precede every later child in document order.
func admitXSDChildStart(n *schemaSyntaxNode, parent xml.Name, placement schemaElementPlacement, attrs startTagAttributes) error {
	if err := checkSchemaAttributesSyntax(n, placement, attrs); err != nil {
		return err
	}
	if err := checkRequiredSchemaAttributes(n, parent, placement); err != nil {
		return err
	}
	if err := checkValueConstraintAttributes(n); err != nil {
		return err
	}
	return checkAttributeDeclarationName(n)
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

// completeTypedNode checks what the end tag of frame decides: text in the
// element precedes its end tag, so a text violation wins over a missing
// required child.
func completeTypedNode(frame *typedSchemaParseFrame) error {
	if !schemaNodeNeedsDeferredQNameContext(frame.node, frame.syntax.Attrs) {
		frame.node.namespace = xmlstream.Context{}
	}
	if err := rejectInvalidSchemaTextTyped(frame.node); err != nil {
		return err
	}
	if frame.order == nil {
		return nil
	}
	if rule := frame.orders.missingChild(frame.order, frame.node); rule != nil {
		return schemaCompileAt(frame.node, rule.MissingCode, rule.MissingMsg)
	}
	return nil
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

func rejectInvalidSchemaTextTyped(n *schemaNode) error {
	if n == nil || n.local == vocab.XSDElemAppinfo || n.local == vocab.XSDElemDocumentation {
		return nil
	}
	if n.hasNonWhitespaceText {
		return schemaCompileAt(n, xsderrors.CodeSchemaContentModel, "xs:"+diagnostictext.Quote(n.local)+" cannot contain text")
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
