// Package format writes consistently indented XML documents for repository-owned tools.
package format

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/stream"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlns"
	"github.com/jacoelho/xsd/xsderrors"
)

const maxFormatDepth = 4096

const (
	categoryFormat   xsderrors.Category = "format"
	codeFormatXML    xsderrors.Code     = "format.xml"
	codeFormatOption xsderrors.Code     = "format.option"
	codeFormatLimit  xsderrors.Code     = "format.limit"
)

var (
	errFormatInputLimit  = errors.New("XML input byte limit exceeded")
	errFormatOutputLimit = errors.New("XML formatted output byte limit exceeded")
)

// Options controls XML formatting resource limits.
type Options struct {
	// MaxDepth limits nested XML elements. Zero uses the default formatter limit.
	MaxDepth int
	// MaxNodes limits retained XML nodes. Zero means unlimited.
	MaxNodes int
	// MaxInputBytes limits bytes read from r. Zero means unlimited.
	MaxInputBytes int64
	// MaxOutputBytes limits bytes written to w. Zero means unlimited.
	MaxOutputBytes int64
	// MaxTokenBytes limits retained XML token payload bytes. Zero means unlimited.
	MaxTokenBytes int64
}

type formatOptions struct {
	maxDepth       int
	maxNodes       int
	maxInputBytes  int64
	maxOutputBytes int64
	maxTokenBytes  int64
}

// XML writes a consistently indented XML document.
//
// XML builds an in-memory formatting tree before writing.
func XML(w io.Writer, r io.Reader) error {
	return XMLWithOptions(w, r, Options{})
}

// XMLWithOptions writes a consistently indented XML document with resource limits.
//
// XMLWithOptions builds an in-memory formatting tree before writing.
func XMLWithOptions(w io.Writer, r io.Reader, opts Options) error {
	if w == nil {
		return formatOptionErr(errors.New("nil writer"))
	}
	if r == nil {
		return formatOptionErr(errors.New("nil reader"))
	}
	limits, err := normalizeFormatOptions(opts)
	if err != nil {
		return formatOptionErr(err)
	}
	reader := r
	if limits.maxInputBytes > 0 {
		reader = &maxBytesReader{r: reader, max: limits.maxInputBytes, err: errFormatInputLimit}
	}
	writer := w
	if limits.maxOutputBytes > 0 {
		writer = &maxBytesWriter{w: writer, max: limits.maxOutputBytes, err: errFormatOutputLimit}
	}

	names := stream.NewCache()
	values := stream.NewCache()
	reader, err = stream.PrepareXMLReaderWithBuffer(reader, nil)
	if err != nil {
		return formatReaderErr(err)
	}
	p := new(stream.Parser)
	p.ResetWithLimit(reader, &names, &values, limits.maxTokenBytes)
	p.SetEmitComments(true)
	p.SetEmitPI(true)
	f := xmlFormatter{w: writer, p: p, maxDepth: limits.maxDepth, maxNodes: limits.maxNodes}
	err = f.format()
	var formatErr *xsderrors.Error
	if err != nil && !errors.As(err, &formatErr) && (errors.Is(err, errFormatInputLimit) || errors.Is(err, errFormatOutputLimit)) {
		return formatLimitErr(0, 0, err)
	}
	return err
}

func formatReaderErr(err error) error {
	switch {
	case errors.Is(err, stream.ErrXMLInputNilReader):
		return formatOptionErr(errors.New("nil reader"))
	case errors.Is(err, stream.ErrUnsupportedNonUTF8):
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, "XML documents must be UTF-8")
	case errors.Is(err, errFormatInputLimit):
		return formatLimitErr(0, 0, err)
	default:
		var versionErr stream.UnsupportedXMLVersionError
		if errors.As(err, &versionErr) {
			return xsderrors.Unsupported(xsderrors.CodeUnsupportedXML11, versionErr.Error())
		}
		return formatXMLErr(0, 0, err)
	}
}

func normalizeFormatOptions(opts Options) (formatOptions, error) {
	if opts.MaxDepth < 0 {
		return formatOptions{}, errors.New("MaxDepth cannot be negative")
	}
	if opts.MaxNodes < 0 {
		return formatOptions{}, errors.New("MaxNodes cannot be negative")
	}
	if opts.MaxInputBytes < 0 {
		return formatOptions{}, errors.New("MaxInputBytes cannot be negative")
	}
	if opts.MaxOutputBytes < 0 {
		return formatOptions{}, errors.New("MaxOutputBytes cannot be negative")
	}
	if opts.MaxTokenBytes < 0 {
		return formatOptions{}, errors.New("MaxTokenBytes cannot be negative")
	}
	maxDepth := opts.MaxDepth
	if maxDepth == 0 {
		maxDepth = maxFormatDepth
	}
	return formatOptions{
		maxDepth:       maxDepth,
		maxNodes:       opts.MaxNodes,
		maxInputBytes:  opts.MaxInputBytes,
		maxOutputBytes: opts.MaxOutputBytes,
		maxTokenBytes:  opts.MaxTokenBytes,
	}, nil
}

type maxBytesReader struct {
	r        io.Reader
	err      error
	max      int64
	n        int64
	exceeded bool
}

func (r *maxBytesReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.exceeded {
		return 0, r.err
	}
	if r.n >= r.max {
		var one [1]byte
		n, err := r.r.Read(one[:])
		if n > 0 {
			r.exceeded = true
			return 0, r.err
		}
		return 0, err
	}
	remaining := r.max - r.n
	if int64(len(p)) > remaining {
		p = p[:int(remaining)]
	}
	n, err := r.r.Read(p)
	r.n += int64(n)
	return n, err
}

type maxBytesWriter struct {
	w   io.Writer
	err error
	max int64
	n   int64
}

func (w *maxBytesWriter) Write(p []byte) (int, error) {
	remaining := w.max - w.n
	if int64(len(p)) <= remaining {
		n, err := w.w.Write(p)
		w.n += int64(n)
		return n, err
	}
	if remaining <= 0 {
		return 0, w.err
	}
	allowed := int(remaining)
	n, err := w.w.Write(p[:allowed])
	w.n += int64(n)
	if err != nil {
		return n, err
	}
	if n != allowed {
		return n, io.ErrShortWrite
	}
	return allowed, w.err
}

type xmlFormatter struct {
	w        io.Writer
	p        *stream.Parser
	ns       xmlns.Stack
	stack    []*formatElement
	items    []formatItem
	nodes    int
	maxDepth int
	maxNodes int
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
		tok, err := f.p.Next()
		if errors.Is(err, io.EOF) {
			return f.finish()
		}
		if err != nil {
			line, col := f.p.Pos()
			return xmlFormatErr(line, col, err)
		}
		if err := f.collectToken(tok); err != nil {
			return err
		}
	}
}

func (f *xmlFormatter) finish() error {
	if len(f.stack) > 0 {
		line, col := f.p.Pos()
		return xmlFormatErr(line, col, fmt.Errorf("unexpected EOF before end element </%s>", xmlQName(f.stack[len(f.stack)-1].start.Name)))
	}
	if !f.rootSeen {
		return xmlFormatErr(1, 1, errors.New("XML document is empty"))
	}
	return f.writeDocument()
}

func (f *xmlFormatter) collectToken(tok stream.Token) error {
	switch tok.Kind {
	case stream.KindStart:
		return f.collectStart(tok)
	case stream.KindEnd:
		return f.collectEnd(tok)
	case stream.KindCharData:
		return f.collectChars(tok)
	case stream.KindDirective:
		return xmlFormatErr(tok.Line, tok.Column, errors.New("DTD declarations are not supported"))
	case stream.KindComment:
		return f.appendItem(formatItem{kind: formatItemComment, data: tok.AppendDirective(nil), line: tok.Line, col: tok.Column})
	case stream.KindPI:
		return f.appendItem(formatItem{kind: formatItemPI, data: tok.AppendData(nil), pi: tok.AppendDirective(nil), line: tok.Line, col: tok.Column})
	default:
		return xmlFormatErr(tok.Line, tok.Column, errors.New("unknown XML token"))
	}
}

func (f *xmlFormatter) collectStart(tok stream.Token) error {
	if len(f.stack) >= f.maxDepth {
		return formatLimitErr(tok.Line, tok.Column, fmt.Errorf("XML nesting exceeds %d element limit", f.maxDepth))
	}
	start := tok.Start.XMLStartElement()
	if err := f.ns.Push(start.Attr); err != nil {
		return xmlFormatErr(tok.Line, tok.Column, err)
	}
	if err := f.validateStartNamespaces(start); err != nil {
		return xmlFormatErr(tok.Line, tok.Column, err)
	}
	preserve := xmlSpacePreserve(start.Attr, xmlSpaceDefault)
	if len(f.stack) == 0 {
		if f.rootSeen {
			return xmlFormatErr(tok.Line, tok.Column, errors.New("XML document has multiple roots"))
		}
		f.rootSeen = true
	} else {
		parent := f.stack[len(f.stack)-1]
		preserve = xmlSpacePreserve(start.Attr, parent.preserve)
	}
	elem := &formatElement{start: start, line: tok.Line, col: tok.Column, preserve: preserve}
	if err := f.appendItem(formatItem{kind: formatItemElement, elem: elem, line: tok.Line, col: tok.Column}); err != nil {
		return err
	}
	f.stack = append(f.stack, elem)
	return nil
}

func (f *xmlFormatter) collectEnd(tok stream.Token) error {
	if len(f.stack) == 0 {
		return xmlFormatErr(tok.Line, tok.Column, errors.New("unexpected end element"))
	}
	frame := f.stack[len(f.stack)-1]
	if frame.start.Name != tok.End.Name {
		return xmlFormatErr(tok.Line, tok.Column, fmt.Errorf("end element </%s> does not match start element <%s>", xmlQName(tok.End.Name), xmlQName(frame.start.Name)))
	}
	f.stack = f.stack[:len(f.stack)-1]
	f.ns.Pop()
	return nil
}

func (f *xmlFormatter) collectChars(tok stream.Token) error {
	if len(f.stack) == 0 {
		if tok.CDATA {
			return xmlFormatErr(tok.Line, tok.Column, errors.New("CDATA section outside root element"))
		}
		if lex.IsXMLWhitespaceBytes(tok.Data) {
			return nil
		}
		return xmlFormatErr(tok.Line, tok.Column, errors.New("text outside root element"))
	}
	return f.appendItem(formatItem{kind: formatItemText, data: tok.AppendData(nil), cdata: tok.CDATA, line: tok.Line, col: tok.Column})
}

func (f *xmlFormatter) appendItem(item formatItem) error {
	if f.maxNodes > 0 && f.nodes+1 > f.maxNodes {
		return formatLimitErr(item.line, item.col, errors.New("XML node limit exceeded"))
	}
	f.nodes++
	if len(f.stack) == 0 {
		f.items = append(f.items, item)
		return nil
	}
	parent := f.stack[len(f.stack)-1]
	parent.children = append(parent.children, item)
	return nil
}

func (f *xmlFormatter) validateStartNamespaces(start xml.StartElement) error {
	if _, err := f.resolveFormatName(start.Name, xmlns.ElementName); err != nil {
		return err
	}
	var seen xmlns.NameSet
	for _, attr := range start.Attr {
		name := attr.Name
		if !xmlns.IsNamespaceAttr(attr) {
			var err error
			name, err = f.resolveFormatName(attr.Name, xmlns.AttributeName)
			if err != nil {
				return err
			}
		}
		if err := seen.AddAttribute(name); err != nil {
			return err
		}
	}
	return nil
}

func (f *xmlFormatter) resolveFormatName(name xml.Name, kind xmlns.NameKind) (xml.Name, error) {
	resolved, ok := f.ns.ResolveName(name, kind)
	if !ok {
		return xml.Name{}, errors.New("unbound namespace prefix " + name.Space)
	}
	return resolved, nil
}

func (f *xmlFormatter) writeDocument() error {
	for i, item := range f.items {
		if i > 0 {
			if err := writeXMLIndent(f.w, 0); err != nil {
				return err
			}
		}
		if err := f.writeItem(item, 0, formatBlock); err != nil {
			return err
		}
	}
	return nil
}

type formatWriteMode uint8

const (
	formatBlock formatWriteMode = iota
	formatInline
)

func (f *xmlFormatter) writeItem(item formatItem, depth int, mode formatWriteMode) error {
	switch item.kind {
	case formatItemElement:
		return f.writeElement(item.elem, depth, mode)
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

func (f *xmlFormatter) writeElement(elem *formatElement, depth int, mode formatWriteMode) error {
	if err := writeXMLStart(f.w, elem.start); err != nil {
		return xmlFormatErr(elem.line, elem.col, err)
	}
	if mode == formatInline || elem.inline() {
		for _, child := range elem.children {
			if err := f.writeItem(child, depth+1, formatInline); err != nil {
				return err
			}
		}
		return writeXMLFormatEnd(f.w, elem)
	}

	wroteChild := false
	for _, child := range elem.children {
		if child.kind == formatItemText && !child.cdata && lex.IsXMLWhitespaceBytes(child.data) {
			continue
		}
		if err := writeXMLIndent(f.w, depth+1); err != nil {
			return err
		}
		if err := f.writeItem(child, depth+1, formatBlock); err != nil {
			return err
		}
		wroteChild = true
	}
	if wroteChild {
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
	hasNonElementLayout := false
	for _, child := range e.children {
		switch child.kind {
		case formatItemElement:
			hasElement = true
		case formatItemComment, formatItemPI:
			hasNonElementLayout = true
		case formatItemText:
			if child.cdata || !lex.IsXMLWhitespaceBytes(child.data) {
				return true
			}
			if !hasXMLLineBreak(child.data) {
				return true
			}
			hasNonElementLayout = true
		}
	}
	return !hasElement && hasNonElementLayout
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

const xmlSpaceDefault = false

func xmlSpacePreserve(attrs []xml.Attr, inherited bool) bool {
	for _, attr := range attrs {
		if attr.Name.Space != vocab.XMLPrefix || attr.Name.Local != vocab.XMLAttrSpace {
			continue
		}
		switch attr.Value {
		case vocab.XMLValuePreserve:
			return true
		case vocab.XMLValueDefault:
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

func xmlFormatErr(line, col int, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errFormatInputLimit) || errors.Is(err, errFormatOutputLimit) || stream.IsTokenLimit(err) || stream.IsAttributeLimit(err) {
		return formatLimitErr(line, col, err)
	}
	return formatXMLErr(line, col, err)
}

func formatOptionErr(err error) error {
	return formatErr(codeFormatOption, 0, 0, err)
}

func formatXMLErr(line, col int, err error) error {
	return formatErr(codeFormatXML, line, col, err)
}

func formatLimitErr(line, col int, err error) error {
	return formatErr(codeFormatLimit, line, col, err)
}

func formatErr(code xsderrors.Code, line, col int, err error) error {
	return &xsderrors.Error{Category: categoryFormat, Code: code, Line: line, Column: col, Err: err}
}

func writeXMLStart(w io.Writer, start xml.StartElement) error {
	if _, err := io.WriteString(w, "<"); err != nil {
		return err
	}
	if err := writeXMLQName(w, start.Name); err != nil {
		return err
	}
	for _, attr := range start.Attr {
		if _, err := io.WriteString(w, " "); err != nil {
			return err
		}
		if err := writeXMLQName(w, attr.Name); err != nil {
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
	if _, err := io.WriteString(w, "</"); err != nil {
		return err
	}
	if err := writeXMLQName(w, end.Name); err != nil {
		return err
	}
	_, err := io.WriteString(w, ">")
	return err
}

func writeXMLQName(w io.Writer, name xml.Name) error {
	if name.Space != "" {
		if _, err := io.WriteString(w, name.Space); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ":"); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, name.Local)
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
	start := 0
	for i := range len(value) {
		var esc string
		switch value[i] {
		case '&':
			esc = "&amp;"
		case '<':
			esc = "&lt;"
		case '"':
			esc = "&quot;"
		case '\n':
			esc = "&#10;"
		case '\r':
			esc = "&#13;"
		case '\t':
			esc = "&#9;"
		}
		if esc == "" {
			continue
		}
		if start < i {
			if _, err := io.WriteString(w, value[start:i]); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, esc); err != nil {
			return err
		}
		start = i + 1
	}
	if start < len(value) {
		if _, err := io.WriteString(w, value[start:]); err != nil {
			return err
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
