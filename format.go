package xsd

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"slices"
)

const maxFormatDepth = 4096

// XMLFormatError reports malformed XML found while formatting.
type XMLFormatError struct {
	Err    error
	Line   int
	Column int
}

func (e *XMLFormatError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Line > 0 {
		return fmt.Sprintf("xml format error at %d:%d: %v", e.Line, e.Column, e.Err)
	}
	return "xml format error: " + e.Err.Error()
}

func (e *XMLFormatError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// FormatXML writes a consistently indented XML document.
func FormatXML(w io.Writer, r io.Reader) error {
	if w == nil {
		return &XMLFormatError{Err: errors.New("nil writer")}
	}
	if r == nil {
		return &XMLFormatError{Err: errors.New("nil reader")}
	}

	names := newByteStringCache(512, 256)
	values := newByteStringCache(512, 256)
	reader, err := prepareInstanceReader(r)
	if err != nil {
		return &XMLFormatError{Err: err}
	}
	p := newXMLStreamParser(reader, &names, &values)
	p.emitComments = true
	p.emitPI = true
	f := xmlFormatter{w: w, p: p}
	return f.format()
}

type xmlFormatter struct {
	w        io.Writer
	p        *xmlStreamParser
	ns       namespaceStack
	stack    []*formatElement
	items    []formatItem
	rootSeen bool
}

type formatItemKind uint8

const (
	formatItemElement formatItemKind = iota
	formatItemText
	formatItemComment
	formatItemPI
)

type formatItem struct {
	elem  *formatElement
	data  []byte
	pi    []byte
	line  int
	col   int
	kind  formatItemKind
	cdata bool
}

type formatElement struct {
	start    xml.StartElement
	children []formatItem
	line     int
	col      int
	preserve bool
}

func (f *xmlFormatter) format() error {
	for {
		tok, err := f.p.next()
		if err == io.EOF {
			return f.finish()
		}
		if err != nil {
			line, col := f.p.br.pos()
			return xmlFormatErr(line, col, err)
		}
		if err := f.collectToken(tok); err != nil {
			return err
		}
	}
}

func (f *xmlFormatter) finish() error {
	if len(f.stack) > 0 {
		line, col := f.p.br.pos()
		return xmlFormatErr(line, col, fmt.Errorf("unexpected EOF before end element </%s>", xmlQName(f.stack[len(f.stack)-1].start.Name)))
	}
	if !f.rootSeen {
		return xmlFormatErr(1, 1, errors.New("XML document is empty"))
	}
	return f.writeDocument()
}

func (f *xmlFormatter) collectToken(tok streamToken) error {
	switch tok.kind {
	case streamTokenStart:
		return f.collectStart(tok)
	case streamTokenEnd:
		return f.collectEnd(tok)
	case streamTokenCharData:
		return f.collectChars(tok)
	case streamTokenDirective:
		return xmlFormatErr(tok.line, tok.col, errors.New("DTD declarations are not supported"))
	case streamTokenComment:
		return f.collectComment(tok)
	case streamTokenPI:
		return f.collectPI(tok)
	default:
		return xmlFormatErr(tok.line, tok.col, errors.New("unknown XML token"))
	}
}

func (f *xmlFormatter) collectStart(tok streamToken) error {
	if err := validateUniqueAttributeNames(tok.start.Attr); err != nil {
		return xmlFormatErr(tok.line, tok.col, err)
	}
	if len(f.stack) >= maxFormatDepth {
		return xmlFormatErr(tok.line, tok.col, fmt.Errorf("XML nesting exceeds %d element limit", maxFormatDepth))
	}
	if err := f.ns.push(tok.start.Attr); err != nil {
		return xmlFormatErr(tok.line, tok.col, err)
	}
	if err := f.validateStartNamespaces(tok.start); err != nil {
		return xmlFormatErr(tok.line, tok.col, err)
	}
	preserve := xmlSpacePreserve(tok.start.Attr, false)
	if len(f.stack) == 0 {
		if f.rootSeen {
			return xmlFormatErr(tok.line, tok.col, errors.New("XML document has multiple roots"))
		}
		f.rootSeen = true
	} else {
		parent := f.stack[len(f.stack)-1]
		preserve = xmlSpacePreserve(tok.start.Attr, parent.preserve)
	}
	elem := &formatElement{start: cloneStartElement(tok.start), line: tok.line, col: tok.col, preserve: preserve}
	f.appendItem(formatItem{kind: formatItemElement, elem: elem, line: tok.line, col: tok.col})
	f.stack = append(f.stack, elem)
	return nil
}

func (f *xmlFormatter) collectEnd(tok streamToken) error {
	if len(f.stack) == 0 {
		return xmlFormatErr(tok.line, tok.col, errors.New("unexpected end element"))
	}
	frame := f.stack[len(f.stack)-1]
	if frame.start.Name != tok.end.Name {
		return xmlFormatErr(tok.line, tok.col, fmt.Errorf("end element </%s> does not match start element <%s>", xmlQName(tok.end.Name), xmlQName(frame.start.Name)))
	}
	f.stack = f.stack[:len(f.stack)-1]
	f.ns.pop()
	return nil
}

func (f *xmlFormatter) collectChars(tok streamToken) error {
	if len(f.stack) == 0 {
		if tok.cdata {
			return xmlFormatErr(tok.line, tok.col, errors.New("CDATA section outside root element"))
		}
		if isXMLWhitespaceBytes(tok.data) {
			return nil
		}
		return xmlFormatErr(tok.line, tok.col, errors.New("text outside root element"))
	}
	f.appendItem(formatItem{kind: formatItemText, data: cloneBytes(tok.data), cdata: tok.cdata, line: tok.line, col: tok.col})
	return nil
}

func (f *xmlFormatter) collectComment(tok streamToken) error {
	f.appendItem(formatItem{kind: formatItemComment, data: cloneBytes(tok.directive), line: tok.line, col: tok.col})
	return nil
}

func (f *xmlFormatter) collectPI(tok streamToken) error {
	f.appendItem(formatItem{kind: formatItemPI, data: cloneBytes(tok.data), pi: cloneBytes(tok.directive), line: tok.line, col: tok.col})
	return nil
}

func (f *xmlFormatter) appendItem(item formatItem) {
	if len(f.stack) == 0 {
		f.items = append(f.items, item)
		return
	}
	parent := f.stack[len(f.stack)-1]
	parent.children = append(parent.children, item)
}

func (f *xmlFormatter) validateStartNamespaces(start xml.StartElement) error {
	if _, err := f.resolveFormatName(start.Name, true); err != nil {
		return err
	}
	seen := make([]xml.Name, 0, len(start.Attr))
	for _, attr := range start.Attr {
		if isNamespaceAttr(attr) {
			continue
		}
		name, err := f.resolveFormatName(attr.Name, false)
		if err != nil {
			return err
		}
		if slices.Contains(seen, name) {
			return errors.New("duplicate attribute " + formatXMLName(name))
		}
		seen = append(seen, name)
	}
	return nil
}

func (f *xmlFormatter) resolveFormatName(name xml.Name, element bool) (xml.Name, error) {
	if name.Space != "" {
		uri, ok := f.ns.lookup(name.Space)
		if !ok {
			return xml.Name{}, errors.New("unbound namespace prefix " + name.Space)
		}
		return xml.Name{Space: uri, Local: name.Local}, nil
	}
	if element {
		uri, _ := f.ns.lookup("")
		return xml.Name{Space: uri, Local: name.Local}, nil
	}
	return name, nil
}

func (f *xmlFormatter) writeDocument() error {
	for i, item := range f.items {
		if i > 0 {
			if err := writeXMLIndent(f.w, 0); err != nil {
				return err
			}
		}
		if err := f.writeItem(item, 0, false); err != nil {
			return err
		}
	}
	return nil
}

func (f *xmlFormatter) writeItem(item formatItem, depth int, inline bool) error {
	switch item.kind {
	case formatItemElement:
		return f.writeElement(item.elem, depth, inline)
	case formatItemText:
		if err := writeXMLText(f.w, item.data, item.cdata); err != nil {
			return xmlFormatErr(item.line, item.col, err)
		}
		return nil
	case formatItemComment:
		if err := writeXMLComment(f.w, item.data); err != nil {
			return xmlFormatErr(item.line, item.col, err)
		}
		return nil
	case formatItemPI:
		if err := writeXMLPI(f.w, item.data, item.pi); err != nil {
			return xmlFormatErr(item.line, item.col, err)
		}
		return nil
	default:
		return xmlFormatErr(item.line, item.col, errors.New("unknown XML format item"))
	}
}

func (f *xmlFormatter) writeElement(elem *formatElement, depth int, inline bool) error {
	if err := writeXMLStart(f.w, elem.start); err != nil {
		return xmlFormatErr(elem.line, elem.col, err)
	}
	if inline || elem.inline() {
		for _, child := range elem.children {
			if err := f.writeItem(child, depth+1, true); err != nil {
				return err
			}
		}
		return writeXMLFormatEnd(f.w, elem)
	}

	children := elem.prettyChildren()
	for _, child := range children {
		if err := writeXMLIndent(f.w, depth+1); err != nil {
			return err
		}
		if err := f.writeItem(child, depth+1, false); err != nil {
			return err
		}
	}
	if len(children) > 0 {
		if err := writeXMLIndent(f.w, depth); err != nil {
			return err
		}
	}
	return writeXMLFormatEnd(f.w, elem)
}

func (e *formatElement) inline() bool {
	if e.preserve {
		return true
	}
	hasElement := false
	hasWhitespaceText := false
	hasInlineWhitespaceText := false
	hasCommentOrPI := false
	for _, child := range e.children {
		switch child.kind {
		case formatItemElement:
			hasElement = true
		case formatItemComment, formatItemPI:
			hasCommentOrPI = true
		case formatItemText:
			if child.cdata || !isXMLWhitespaceBytes(child.data) {
				return true
			}
			hasWhitespaceText = true
			if !hasXMLLineBreak(child.data) {
				hasInlineWhitespaceText = true
			}
		}
	}
	return hasInlineWhitespaceText || !hasElement && (hasWhitespaceText || hasCommentOrPI)
}

func (e *formatElement) prettyChildren() []formatItem {
	children := make([]formatItem, 0, len(e.children))
	for _, child := range e.children {
		if child.kind == formatItemText && !child.cdata && isXMLWhitespaceBytes(child.data) {
			continue
		}
		children = append(children, child)
	}
	return children
}

func writeXMLFormatEnd(w io.Writer, elem *formatElement) error {
	if err := writeXMLEnd(w, xml.EndElement{Name: elem.start.Name}); err != nil {
		return xmlFormatErr(elem.line, elem.col, err)
	}
	return nil
}

func writeXMLComment(w io.Writer, data []byte) error {
	if _, err := io.WriteString(w, "<!--"); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err := io.WriteString(w, "-->")
	return err
}

func writeXMLPI(w io.Writer, target, data []byte) error {
	if _, err := io.WriteString(w, "<?"); err != nil {
		return err
	}
	if _, err := w.Write(target); err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := io.WriteString(w, " "); err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "?>")
	return err
}

func writeXMLText(w io.Writer, data []byte, cdata bool) error {
	if cdata {
		return writeXMLCDATA(w, data)
	}
	return xml.EscapeText(w, data)
}

func xmlSpacePreserve(attrs []xml.Attr, inherited bool) bool {
	for _, attr := range attrs {
		if attr.Name.Space != "xml" || attr.Name.Local != "space" {
			continue
		}
		switch attr.Value {
		case "preserve":
			return true
		case "default":
			return false
		}
	}
	return inherited
}

func hasXMLLineBreak(data []byte) bool {
	for _, b := range data {
		if b == '\n' || b == '\r' {
			return true
		}
	}
	return false
}

func cloneBytes(data []byte) []byte {
	return slices.Clone(data)
}

func cloneStartElement(start xml.StartElement) xml.StartElement {
	start.Attr = slices.Clone(start.Attr)
	return start
}

func xmlFormatErr(line, col int, err error) error {
	if err == nil {
		return nil
	}
	return &XMLFormatError{Line: line, Column: col, Err: err}
}

func writeXMLStart(w io.Writer, start xml.StartElement) error {
	if _, err := io.WriteString(w, "<"); err != nil {
		return err
	}
	if _, err := io.WriteString(w, xmlQName(start.Name)); err != nil {
		return err
	}
	for _, attr := range start.Attr {
		if _, err := io.WriteString(w, " "); err != nil {
			return err
		}
		if _, err := io.WriteString(w, xmlQName(attr.Name)); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "=\""); err != nil {
			return err
		}
		if err := writeXMLAttrValue(w, attr.Value); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "\""); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, ">")
	return err
}

func writeXMLEnd(w io.Writer, end xml.EndElement) error {
	_, err := io.WriteString(w, "</"+xmlQName(end.Name)+">")
	return err
}

func writeXMLIndent(w io.Writer, depth int) error {
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	for range depth {
		if _, err := io.WriteString(w, "  "); err != nil {
			return err
		}
	}
	return nil
}

func writeXMLAttrValue(w io.Writer, value string) error {
	for _, r := range value {
		switch r {
		case '&':
			if _, err := io.WriteString(w, "&amp;"); err != nil {
				return err
			}
		case '<':
			if _, err := io.WriteString(w, "&lt;"); err != nil {
				return err
			}
		case '"':
			if _, err := io.WriteString(w, "&quot;"); err != nil {
				return err
			}
		case '\n':
			if _, err := io.WriteString(w, "&#10;"); err != nil {
				return err
			}
		case '\r':
			if _, err := io.WriteString(w, "&#13;"); err != nil {
				return err
			}
		case '\t':
			if _, err := io.WriteString(w, "&#9;"); err != nil {
				return err
			}
		default:
			if _, err := io.WriteString(w, string(r)); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeXMLCDATA(w io.Writer, data []byte) error {
	if _, err := io.WriteString(w, "<![CDATA["); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	_, err := io.WriteString(w, "]]>")
	return err
}

func xmlQName(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	return name.Space + ":" + name.Local
}
