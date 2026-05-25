package xsd

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"maps"
	"strings"
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

func parseSchemaDocument(name, key string, data []byte, limits compileLimits) (*rawDoc, error) {
	data = bytes.TrimPrefix(data, utf8BOM)
	if version := declaredXMLVersion(data); version != "" && version != xmlVersion10 {
		return nil, unsupported(ErrUnsupportedXML11, "XML version "+version+" is not supported")
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	state := schemaParseState{
		dec:    dec,
		limits: limits,
		nsStack: []map[string]string{{
			xmlPrefix: xmlNamespaceURI,
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
	if err := rejectDuplicateSchemaIDs(state.root, make(map[string]bool)); err != nil {
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
	limits  compileLimits
}

func (s *schemaParseState) parse() error {
	for {
		tok, err := s.dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			line, col := s.dec.InputPos()
			return schemaParse(ErrSchemaXML, line, col, "invalid schema XML", err)
		}
		if err := s.handleToken(tok); err != nil {
			return err
		}
	}
	if len(s.stack) != 0 {
		return schemaParse(ErrSchemaXML, 0, 0, "unclosed schema element", nil)
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
	if s.limits.maxSchemaDepth > 0 && len(s.stack)+1 > s.limits.maxSchemaDepth {
		return schemaParse(ErrSchemaLimit, line, col, "schema XML nesting exceeds configured limit", nil)
	}
	if s.limits.maxSchemaAttributes > 0 && len(t.Attr) > s.limits.maxSchemaAttributes {
		return schemaParse(ErrSchemaLimit, line, col, "schema XML attributes exceed configured limit", nil)
	}
	if err := checkSchemaStartElementLimit(t, s.limits, line, col); err != nil {
		return err
	}
	parentNS := s.nsStack[len(s.nsStack)-1]
	ns := parentNS
	clonedNS := false
	for _, a := range t.Attr {
		if a.Name.Space == xmlnsPrefix {
			if !clonedNS {
				ns = cloneNS(parentNS)
				clonedNS = true
			}
			ns[a.Name.Local] = a.Value
			continue
		}
		if a.Name.Space == "" && a.Name.Local == xmlnsPrefix {
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
			return schemaParse(ErrSchemaRoot, line, col, "schema document has multiple roots", nil)
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
		return schemaParse(ErrSchemaXML, line, col, "unexpected end element", nil)
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
		if !isXMLWhitespaceBytes(t) {
			return schemaParse(ErrSchemaXML, line, col, "schema XML text outside root element", nil)
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
	if isDOCTYPEDeclaration(t) {
		return &Error{
			Category: UnsupportedErrorCategory,
			Code:     ErrUnsupportedDTD,
			Line:     line,
			Column:   col,
			Message:  "DTD declarations are not supported",
		}
	}
	return schemaParse(ErrSchemaXML, line, col, "invalid schema XML", nil)
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
		return schemaParse(ErrSchemaRoot, 0, 0, "empty schema document", nil)
	}
	if root.Name.Space != xsdNamespaceURI || root.Name.Local != xsdElemSchema {
		return schemaParse(ErrSchemaRoot, root.Line, root.Column, "root element must be xs:schema", nil)
	}
	return nil
}

func checkSchemaStartElementLimit(start xml.StartElement, limits compileLimits, line, col int) error {
	if limits.maxSchemaTokenBytes <= 0 {
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

// checkSchemaTokenLimit centralizes schema token limit diagnostics.
func checkSchemaTokenLimit(size int64, limits compileLimits, line, col int, msg string) error {
	if limits.maxSchemaTokenBytes > 0 && size > limits.maxSchemaTokenBytes {
		limitErr := schemaParse(ErrSchemaLimit, line, col, msg, nil)
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
	if parent != nil && parent.Name.Space == xsdNamespaceURI && parent.Name.Local == xsdElemAnnotation {
		if n.Name.Space == xsdNamespaceURI && (n.Name.Local == xsdElemAppinfo || n.Name.Local == xsdElemDocumentation) {
			return nil
		}
	}
	for _, attr := range n.Attr {
		if attr.Name.Space == xsdNamespaceURI {
			return schemaCompile(ErrSchemaInvalidAttribute, "schema namespace attribute "+attr.Name.Local+" is not allowed")
		}
	}
	if n.Name.Space == xsdNamespaceURI {
		switch n.Name.Local {
		case "redefine":
			return unsupported(ErrUnsupportedRedefine, "xs:redefine is not supported")
		case xsdElemNotation:
			if parent == nil || parent.Name.Space != xsdNamespaceURI || parent.Name.Local != xsdElemSchema {
				return schemaCompile(ErrSchemaContentModel, "xs:notation must be a top-level schema child")
			}
		case "assert", "alternative", "override", "openContent", "defaultOpenContent":
			return unsupported(ErrUnsupportedXSD11, "XSD 1.1 feature "+n.Name.Local+" is not supported")
		case xsdElemAny, xsdElemAnyAttribute:
			for _, attr := range []string{xsdAttrNotNamespace, xsdAttrNotQName} {
				if _, ok := n.attr(attr); ok {
					return unsupported(ErrUnsupportedXSD11, "XSD 1.1 wildcard attribute "+attr+" is not supported")
				}
			}
		}
	}
	for _, c := range n.Children {
		if err := rejectUnsupportedSchemaNodes(c, n); err != nil {
			return err
		}
	}
	return nil
}

func rejectUnknownSchemaAttributes(n *rawNode) error {
	if n.Name.Space == xsdNamespaceURI {
		for _, attr := range n.Attr {
			if isNamespaceAttr(attr) || attr.Name.Space != "" {
				continue
			}
			if !schemaAttributeAllowed(n.Name.Local, attr.Name.Local) {
				return schemaCompile(ErrSchemaInvalidAttribute, n.Name.Local+" cannot have attribute "+attr.Name.Local)
			}
		}
	}
	for _, child := range n.Children {
		if err := rejectUnknownSchemaAttributes(child); err != nil {
			return err
		}
	}
	return nil
}

func schemaAttributeAllowed(element, attr string) bool {
	switch element {
	case xsdElemSchema, xsdElemInclude, xsdElemImport, xsdElemAppinfo, xsdElemDocumentation:
		return schemaDocumentAttributeAllowed(element, attr)
	case xsdElemSimpleType, xsdElemRestriction, xsdElemExtension, xsdElemList, xsdElemUnion:
		return simpleDerivationAttributeAllowed(element, attr)
	case xsdElemComplexType, xsdElemAnnotation, xsdElemSimpleContent, xsdElemComplexContent:
		return complexDerivationAttributeAllowed(element, attr)
	case xsdElemGroup, xsdElemAll, xsdElemChoice, xsdElemSequence:
		return modelGroupAttributeAllowed(element, attr)
	case xsdElemElement:
		return isElementAttribute(attr)
	case xsdElemAttribute:
		return isAttributeAttribute(attr)
	case xsdElemAttributeGroup:
		return attr == xsdAttrID || attr == xsdAttrName || attr == xsdAttrRef
	case xsdElemAny:
		return isAnyParticleAttribute(attr)
	case xsdElemAnyAttribute:
		return isAnyAttributeAttribute(attr)
	case xsdElemUnique, xsdElemKey:
		return isIdentityAttribute(attr)
	case xsdElemKeyref:
		return isKeyrefAttribute(attr)
	case xsdElemSelector, xsdElemField:
		return isIdentityXPathAttribute(attr)
	case xsdElemNotation:
		return isNotationAttribute(attr)
	default:
		if isFacetNode(element) {
			return attr == xsdAttrID || attr == xsdAttrValue || attr == xsdAttrFixed
		}
		return true
	}
}

func schemaDocumentAttributeAllowed(element, attr string) bool {
	switch element {
	case xsdElemSchema:
		switch attr {
		case xsdAttrID, xsdAttrTargetNamespace, xsdAttrVersion, xsdAttrFinalDefault, xsdAttrBlockDefault, xsdAttrAttributeFormDefault, xsdAttrElementFormDefault:
			return true
		}
	case xsdElemInclude:
		return attr == xsdAttrID || attr == xsdAttrSchemaLocation
	case xsdElemImport:
		return attr == xsdAttrID || attr == xsdAttrNamespace || attr == xsdAttrSchemaLocation
	case xsdElemAppinfo, xsdElemDocumentation:
		return attr == xsdAttrSource
	default:
		return false
	}
	return false
}

func simpleDerivationAttributeAllowed(element, attr string) bool {
	switch element {
	case xsdElemSimpleType:
		return attr == xsdAttrID || attr == xsdAttrName || attr == xsdAttrFinal
	case xsdElemRestriction, xsdElemExtension:
		return attr == xsdAttrID || attr == xsdAttrBase
	case xsdElemList:
		return attr == xsdAttrID || attr == xsdAttrItemType
	case xsdElemUnion:
		return attr == xsdAttrID || attr == xsdAttrMemberTypes
	default:
		return false
	}
}

func complexDerivationAttributeAllowed(element, attr string) bool {
	switch element {
	case xsdElemComplexType:
		switch attr {
		case xsdAttrID, xsdAttrName, xsdAttrMixed, xsdAttrAbstract, xsdAttrBlock, xsdAttrFinal:
			return true
		}
	case xsdElemAnnotation, xsdElemSimpleContent:
		return attr == xsdAttrID
	case xsdElemComplexContent:
		return attr == xsdAttrID || attr == xsdAttrMixed
	default:
		return false
	}
	return false
}

func modelGroupAttributeAllowed(element, attr string) bool {
	switch element {
	case xsdElemGroup:
		switch attr {
		case xsdAttrID, xsdAttrName, xsdAttrRef, xsdAttrMinOccurs, xsdAttrMaxOccurs:
			return true
		}
	case xsdElemAll, xsdElemChoice, xsdElemSequence:
		return attr == xsdAttrID || attr == xsdAttrMinOccurs || attr == xsdAttrMaxOccurs
	default:
		return false
	}
	return false
}

func rejectDuplicateSchemaIDs(n *rawNode, seen map[string]bool) error {
	if id, ok := n.attr(xsdAttrID); ok {
		if seen[id] {
			return schemaCompile(ErrSchemaInvalidAttribute, "duplicate schema id "+id)
		}
		seen[id] = true
	}
	for _, c := range n.Children {
		if err := rejectDuplicateSchemaIDs(c, seen); err != nil {
			return err
		}
	}
	return nil
}

func rejectInvalidSchemaNames(n, parent *rawNode) error {
	if n.Name.Space == xsdNamespaceURI {
		if id, ok := n.attr(xsdAttrID); ok && !isNCName(id) {
			return schemaCompile(ErrSchemaInvalidAttribute, "schema id must be NCName")
		}
		if name, ok := n.attr(xsdAttrName); ok && !isNCName(name) {
			return schemaCompile(ErrSchemaInvalidAttribute, "schema component name must be NCName")
		}
		if n.Name.Local == xsdElemAttributeGroup && parent != nil && (parent.Name.Space != xsdNamespaceURI || parent.Name.Local != xsdElemSchema) {
			if _, ok := n.attr(xsdAttrName); ok {
				return schemaCompile(ErrSchemaInvalidAttribute, "attributeGroup use cannot have name")
			}
		}
	}
	for _, child := range n.Children {
		if err := rejectInvalidSchemaNames(child, n); err != nil {
			return err
		}
	}
	return nil
}

func rejectInvalidAnnotations(n *rawNode) error {
	if n.Name.Space == xsdNamespaceURI {
		done, err := validateAnnotationNode(n)
		if err != nil || done {
			return err
		}
	}
	for _, child := range n.Children {
		if err := rejectInvalidAnnotations(child); err != nil {
			return err
		}
	}
	return nil
}

func validateAnnotationNode(n *rawNode) (bool, error) {
	switch n.Name.Local {
	case xsdElemAppinfo:
		return true, nil
	case xsdElemDocumentation:
		return true, validateDocumentationNode(n)
	case xsdElemAnnotation:
		return false, validateAnnotationElement(n)
	case xsdElemSchema:
		return false, nil
	default:
		return false, validateComponentAnnotationPlacement(n)
	}
}

func validateDocumentationNode(n *rawNode) error {
	for _, attr := range n.Attr {
		if attr.Name.Space == xmlNamespaceURI && attr.Name.Local == xmlAttrLang && !validLanguageTag(attr.Value) {
			return schemaCompile(ErrSchemaInvalidAttribute, "invalid xml:lang on xs:documentation")
		}
	}
	return nil
}

func validateAnnotationElement(n *rawNode) error {
	for _, attr := range n.Attr {
		if attr.Name.Space == "" && attr.Name.Local != xsdAttrID {
			return schemaCompile(ErrSchemaInvalidAttribute, "attribute "+attr.Name.Local+" cannot appear on xs:annotation")
		}
	}
	for _, child := range n.Children {
		if child.Name.Space == xsdNamespaceURI && child.Name.Local == xsdElemAnnotation {
			return schemaCompile(ErrSchemaContentModel, "xs:annotation cannot contain xs:annotation")
		}
	}
	return nil
}

func validateComponentAnnotationPlacement(n *rawNode) error {
	annotations := 0
	seenNonAnnotation := false
	for _, child := range n.Children {
		if child.Name.Space != xsdNamespaceURI {
			continue
		}
		if child.Name.Local == xsdElemAnnotation {
			annotations++
			if annotations > 1 {
				return schemaCompile(ErrSchemaContentModel, "schema component cannot contain multiple annotations")
			}
			if seenNonAnnotation {
				return schemaCompile(ErrSchemaContentModel, n.Name.Local+" annotation must be first")
			}
			continue
		}
		seenNonAnnotation = true
	}
	return nil
}

func validLanguageTag(v string) bool {
	i := 0
	for part := range strings.SplitSeq(v, "-") {
		if part == "" || len(part) > 8 {
			return false
		}
		for _, r := range part {
			if i == 0 {
				if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
					return false
				}
				continue
			}
			if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				return false
			}
		}
		i++
	}
	return true
}

func (n *rawNode) attr(local string) (string, bool) {
	for _, a := range n.Attr {
		if a.Name.Space == "" && a.Name.Local == local {
			value := replaceXMLWhitespace(a.Value)
			if local == xsdAttrName || local == xsdAttrID {
				return normalizeWhitespace(value, whitespaceCollapse), true
			}
			return value, true
		}
	}
	return "", false
}

func (n *rawNode) attrDefault(local, def string) string {
	if v, ok := n.attr(local); ok {
		return v
	}
	return def
}

func (n *rawNode) xsChildren(local string) []*rawNode {
	var out []*rawNode
	for _, c := range n.Children {
		if c.Name.Space == xsdNamespaceURI && c.Name.Local == local {
			out = append(out, c)
		}
	}
	return out
}

func (n *rawNode) firstXS(local string) *rawNode {
	for _, c := range n.Children {
		if c.Name.Space == xsdNamespaceURI && c.Name.Local == local {
			return c
		}
	}
	return nil
}

func (n *rawNode) xsContentChildren() []*rawNode {
	var out []*rawNode
	for _, c := range n.Children {
		if c.Name.Space != xsdNamespaceURI {
			continue
		}
		if c.Name.Local == xsdElemAnnotation {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (n *rawNode) resolveQName(lexical string) (string, string, error) {
	prefix, local, prefixed, err := parseQNameParts(lexical)
	if err != nil {
		return "", "", err
	}
	if !prefixed {
		return n.NS[""], local, nil
	}
	ns, ok := n.NS[prefix]
	if !ok {
		return "", "", schemaCompile(ErrSchemaReference, "unbound QName prefix "+prefix)
	}
	return ns, local, nil
}

func parseQNameParts(lexical string) (string, string, bool, error) {
	lexical = trimXMLWhitespace(lexical)
	if lexical == "" {
		return "", "", false, schemaCompile(ErrSchemaReference, "invalid QName "+lexical)
	}
	prefix, local, ok := strings.Cut(lexical, ":")
	if !ok {
		if !isNCName(lexical) {
			return "", "", false, schemaCompile(ErrSchemaReference, "invalid QName "+lexical)
		}
		return "", lexical, false, nil
	}
	if prefix == "" || local == "" || strings.Contains(local, ":") || !isNCName(prefix) || !isNCName(local) {
		return "", "", false, schemaCompile(ErrSchemaReference, "invalid QName "+lexical)
	}
	return prefix, local, true, nil
}

func parseQNamePrefixWildcard(lexical string) (string, bool, error) {
	lexical = trimXMLWhitespace(lexical)
	prefix, local, ok := strings.Cut(lexical, ":")
	if !ok || local != "*" {
		return "", false, nil
	}
	if prefix == "" || !isNCName(prefix) {
		return "", true, schemaCompile(ErrSchemaReference, "invalid QName "+lexical)
	}
	return prefix, true, nil
}

func parseSchemaBool(v string) (bool, bool) {
	return parseBooleanLexical(trimXMLWhitespace(v))
}
