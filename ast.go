package xsd

import (
	"bytes"
	"encoding/xml"
	"io"
	"maps"
	"strings"
)

type rawDoc struct {
	root *rawNode
	name string
}

type rawNode struct {
	NS       map[string]string
	Name     xml.Name
	Text     string
	Attr     []xml.Attr
	Children []*rawNode
	Line     int
	Column   int
}

func parseSchemaDocument(name string, data []byte, limits compileLimits) (*rawDoc, error) {
	data = bytes.TrimPrefix(data, utf8BOM)
	if version := declaredXMLVersion(data); version != "" && version != "1.0" {
		return nil, unsupported(ErrUnsupportedXML11, "XML version "+version+" is not supported")
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	state := schemaParseState{
		dec:    dec,
		limits: limits,
		nsStack: []map[string]string{{
			"xml": xmlNamespaceURI,
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
	if err := rejectDuplicateSchemaIDs(state.root, make(map[string]bool)); err != nil {
		return nil, err
	}
	if err := rejectInvalidSchemaNames(state.root, nil); err != nil {
		return nil, err
	}
	if err := rejectInvalidAnnotations(state.root); err != nil {
		return nil, err
	}
	return &rawDoc{name: name, root: state.root}, nil
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
		if err == io.EOF {
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
	ns := cloneNS(s.nsStack[len(s.nsStack)-1])
	for _, a := range t.Attr {
		if a.Name.Space == "xmlns" {
			ns[a.Name.Local] = a.Value
			continue
		}
		if a.Name.Space == "" && a.Name.Local == "xmlns" {
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
	s.stack = s.stack[:len(s.stack)-1]
	s.nsStack = s.nsStack[:len(s.nsStack)-1]
	return nil
}

func (s *schemaParseState) handleCharData(t xml.CharData) error {
	line, col := s.dec.InputPos()
	if err := checkSchemaTokenLimit(len(t), s.limits, line, col, "schema XML text exceeds configured limit"); err != nil {
		return err
	}
	if len(s.stack) == 0 {
		if !isXMLWhitespaceBytes(t) {
			return schemaParse(ErrSchemaXML, line, col, "schema XML text outside root element", nil)
		}
		return nil
	}
	n := s.stack[len(s.stack)-1]
	if err := checkSchemaTokenLimit(len(n.Text)+len(t), s.limits, line, col, "schema XML text exceeds configured limit"); err != nil {
		return err
	}
	n.Text += string(t)
	return nil
}

func (s *schemaParseState) handleDirective(t xml.Directive) error {
	line, col := s.dec.InputPos()
	if err := checkSchemaTokenLimit(len(t), s.limits, line, col, "schema XML directive exceeds configured limit"); err != nil {
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
	size := len(t.Target) + len(t.Inst)
	return checkSchemaTokenLimit(size, s.limits, line, col, "schema XML processing instruction exceeds configured limit")
}

func (s *schemaParseState) handleComment(t xml.Comment) error {
	line, col := s.dec.InputPos()
	return checkSchemaTokenLimit(len(t), s.limits, line, col, "schema XML comment exceeds configured limit")
}

func validateSchemaRoot(root *rawNode) error {
	if root == nil {
		return schemaParse(ErrSchemaRoot, 0, 0, "empty schema document", nil)
	}
	if root.Name.Space != xsdNamespaceURI || root.Name.Local != "schema" {
		return schemaParse(ErrSchemaRoot, root.Line, root.Column, "root element must be xs:schema", nil)
	}
	return nil
}

func checkSchemaStartElementLimit(start xml.StartElement, limits compileLimits, line, col int) error {
	if limits.maxSchemaTokenBytes <= 0 {
		return nil
	}
	size := len(start.Name.Space) + len(start.Name.Local)
	if err := checkSchemaTokenLimit(size, limits, line, col, "schema XML start element exceeds configured limit"); err != nil {
		return err
	}
	for _, attr := range start.Attr {
		if err := checkSchemaTokenLimit(len(attr.Value), limits, line, col, "schema XML attribute value exceeds configured limit"); err != nil {
			return err
		}
		size += len(attr.Name.Space) + len(attr.Name.Local) + len(attr.Value)
		if err := checkSchemaTokenLimit(size, limits, line, col, "schema XML start element exceeds configured limit"); err != nil {
			return err
		}
	}
	return nil
}

func checkSchemaTokenLimit(size int, limits compileLimits, line, col int, msg string) error {
	if limits.maxSchemaTokenBytes > 0 && size > limits.maxSchemaTokenBytes {
		return schemaParse(ErrSchemaLimit, line, col, msg, nil)
	}
	return nil
}

func cloneNS(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src)+2)
	maps.Copy(dst, src)
	return dst
}

func rejectUnsupportedSchemaNodes(n, parent *rawNode) error {
	if parent != nil && parent.Name.Space == xsdNamespaceURI && parent.Name.Local == "annotation" {
		if n.Name.Space == xsdNamespaceURI && (n.Name.Local == "appinfo" || n.Name.Local == "documentation") {
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
		case "notation":
			if parent == nil || parent.Name.Space != xsdNamespaceURI || parent.Name.Local != "schema" {
				return schemaCompile(ErrSchemaContentModel, "xs:notation must be a top-level schema child")
			}
		case "assert", "alternative", "override", "openContent", "defaultOpenContent":
			return unsupported(ErrUnsupportedXSD11, "XSD 1.1 feature "+n.Name.Local+" is not supported")
		case "any", "anyAttribute":
			for _, attr := range []string{"notNamespace", "notQName"} {
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

func rejectDuplicateSchemaIDs(n *rawNode, seen map[string]bool) error {
	if id, ok := n.attr("id"); ok {
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
		if id, ok := n.attr("id"); ok && !isNCName(id) {
			return schemaCompile(ErrSchemaInvalidAttribute, "schema id must be NCName")
		}
		if name, ok := n.attr("name"); ok && !isNCName(name) {
			return schemaCompile(ErrSchemaInvalidAttribute, "schema component name must be NCName")
		}
		if n.Name.Local == "attributeGroup" && parent != nil && (parent.Name.Space != xsdNamespaceURI || parent.Name.Local != "schema") {
			if _, ok := n.attr("name"); ok {
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
	case "appinfo":
		return true, nil
	case "documentation":
		return true, validateDocumentationNode(n)
	case "annotation":
		return false, validateAnnotationElement(n)
	case "schema":
		return false, nil
	default:
		return false, validateComponentAnnotationPlacement(n)
	}
}

func validateDocumentationNode(n *rawNode) error {
	for _, attr := range n.Attr {
		if attr.Name.Space == xmlNamespaceURI && attr.Name.Local == "lang" && !validLanguageTag(attr.Value) {
			return schemaCompile(ErrSchemaInvalidAttribute, "invalid xml:lang on xs:documentation")
		}
	}
	return nil
}

func validateAnnotationElement(n *rawNode) error {
	for _, attr := range n.Attr {
		if attr.Name.Space == "" && attr.Name.Local != "id" {
			return schemaCompile(ErrSchemaInvalidAttribute, "attribute "+attr.Name.Local+" cannot appear on xs:annotation")
		}
	}
	for _, child := range n.Children {
		if child.Name.Space == xsdNamespaceURI && child.Name.Local == "annotation" {
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
		if child.Name.Local == "annotation" {
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
	parts := strings.Split(v, "-")
	if len(parts) == 0 {
		return false
	}
	for i, part := range parts {
		if len(part) == 0 || len(part) > 8 {
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
	}
	return true
}

func (n *rawNode) attr(local string) (string, bool) {
	for _, a := range n.Attr {
		if a.Name.Space == "" && a.Name.Local == local {
			value := normalizeXMLAttributeWhitespace(a.Value)
			if local == "name" || local == "id" {
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
		switch c.Name.Local {
		case "annotation":
			continue
		default:
			out = append(out, c)
		}
	}
	return out
}

func (n *rawNode) resolveQName(lexical string) (string, string, error) {
	lexical = strings.TrimSpace(lexical)
	if lexical == "" {
		return "", "", schemaCompile(ErrSchemaReference, "invalid QName "+lexical)
	}
	prefix, local, ok := strings.Cut(lexical, ":")
	if !ok {
		return n.NS[""], lexical, nil
	}
	if prefix == "" || local == "" || strings.Contains(local, ":") {
		return "", "", schemaCompile(ErrSchemaReference, "invalid QName "+lexical)
	}
	ns, ok := n.NS[prefix]
	if !ok {
		return "", "", schemaCompile(ErrSchemaReference, "unbound QName prefix "+prefix)
	}
	return ns, local, nil
}

func parseSchemaBool(v string) (bool, bool) {
	switch strings.TrimSpace(v) {
	case "true", "1":
		return true, true
	case "false", "0":
		return false, true
	default:
		return false, false
	}
}
