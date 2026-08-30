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

const (
	maxFormatDepth              = 4096
	defaultMaxFormatNodes       = 1_000_000
	defaultMaxFormatInputBytes  = int64(64 << 20)
	defaultMaxFormatOutputBytes = int64(64 << 20)
	defaultMaxFormatTokenBytes  = int64(4 << 20)
)

var (
	errFormatOutputLimit = errors.New("XML formatted output byte limit exceeded")
)

// Options controls XML formatting resource limits.
type Options struct {
	// MaxDepth limits nested XML elements. Zero uses the default formatter limit.
	MaxDepth int
	// MaxNodes limits retained XML nodes. Zero uses the default formatter limit.
	MaxNodes int
	// MaxInputBytes limits bytes read from r. Zero uses the default formatter limit.
	MaxInputBytes int64
	// MaxOutputBytes limits bytes written to w. Zero uses the default formatter limit.
	MaxOutputBytes int64
	// MaxTokenBytes limits retained XML token payload bytes. Zero uses the default formatter limit.
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
	writer := newMaxBytesWriter(w, limits.maxOutputBytes, errFormatOutputLimit)

	names := stream.NewCache()
	values := stream.NewCache()
	p := new(stream.Parser)
	err = p.ResetWithConfig(r, &names, &values, stream.Config{
		Limits: stream.Limits{
			MaxInputBytes: limits.maxInputBytes,
			MaxTokenBytes: limits.maxTokenBytes,
		},
		EmitComments: true,
		EmitPI:       true,
	})
	if err != nil {
		return formatReaderErr(err)
	}
	defer p.Detach()
	f := xmlFormatter{w: writer, p: p, maxDepth: limits.maxDepth, maxNodes: limits.maxNodes}
	err = f.format()
	var formatErr *xsderrors.Error
	if err != nil && !errors.As(err, &formatErr) && (stream.IsInputLimit(err) || errors.Is(err, errFormatOutputLimit)) {
		return formatLimitErr(0, 0, err)
	}
	return err
}

func formatReaderErr(err error) error {
	switch {
	case errors.Is(err, stream.ErrXMLInputNilReader):
		return formatOptionErr(errors.New("nil reader"))
	case errors.Is(err, stream.ErrUnsupportedNonUTF8):
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, "XML documents must be UTF-8", err)
	case stream.IsInputLimit(err):
		return formatLimitErr(0, 0, err)
	default:
		if versionErr, ok := errors.AsType[stream.UnsupportedXMLVersionError](err); ok {
			return xsderrors.Unsupported(xsderrors.CodeUnsupportedXML11, versionErr.Error(), nil)
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
	maxNodes := opts.MaxNodes
	if maxNodes == 0 {
		maxNodes = defaultMaxFormatNodes
	}
	maxInputBytes := opts.MaxInputBytes
	if maxInputBytes == 0 {
		maxInputBytes = defaultMaxFormatInputBytes
	}
	maxOutputBytes := opts.MaxOutputBytes
	if maxOutputBytes == 0 {
		maxOutputBytes = defaultMaxFormatOutputBytes
	}
	maxTokenBytes := opts.MaxTokenBytes
	if maxTokenBytes == 0 {
		maxTokenBytes = defaultMaxFormatTokenBytes
	}
	return formatOptions{
		maxDepth:       maxDepth,
		maxNodes:       maxNodes,
		maxInputBytes:  maxInputBytes,
		maxOutputBytes: maxOutputBytes,
		maxTokenBytes:  maxTokenBytes,
	}, nil
}

type maxBytesWriter struct {
	w   io.Writer
	err error
	max int64
	n   int64
}

func newMaxBytesWriter(w io.Writer, maxBytes int64, err error) io.Writer {
	bounded := maxBytesWriter{w: w, max: maxBytes, err: err}
	if _, ok := w.(io.StringWriter); ok {
		return &maxBytesStringWriter{maxBytesWriter: bounded}
	}
	return &bounded
}

type maxBytesStringWriter struct {
	maxBytesWriter
}

func (w *maxBytesWriter) Write(p []byte) (int, error) {
	remaining := w.max - w.n
	if int64(len(p)) <= remaining {
		n, err := w.w.Write(p)
		return w.record(len(p), n, err)
	}
	if remaining <= 0 {
		return 0, w.err
	}
	allowed := int(remaining)
	n, err := w.w.Write(p[:allowed])
	n, err = w.record(allowed, n, err)
	if err != nil {
		return n, err
	}
	return n, w.err
}

func (w *maxBytesStringWriter) WriteString(s string) (int, error) {
	remaining := w.max - w.n
	if int64(len(s)) <= remaining {
		n, err := w.stringWriter().WriteString(s)
		return w.record(len(s), n, err)
	}
	if remaining <= 0 {
		return 0, w.err
	}
	allowed := int(remaining)
	n, err := w.stringWriter().WriteString(s[:allowed])
	n, err = w.record(allowed, n, err)
	if err != nil {
		return n, err
	}
	return n, w.err
}

func (w *maxBytesStringWriter) stringWriter() io.StringWriter {
	sw, ok := w.w.(io.StringWriter)
	if !ok {
		panic("format: maxBytesStringWriter delegate lost io.StringWriter")
	}
	return sw
}

func (w *maxBytesWriter) record(want, n int, err error) (int, error) {
	if n < 0 || n > want {
		if err != nil {
			return 0, errors.Join(io.ErrShortWrite, err)
		}
		return 0, io.ErrShortWrite
	}
	w.n += int64(n)
	if n != want && err == nil {
		return n, io.ErrShortWrite
	}
	if err != nil {
		return n, err
	}
	return n, nil
}

type xmlFormatter struct {
	w        io.Writer
	p        *stream.Parser
	stack    []*formatElement
	items    []formatItem
	ns       xmlns.Stack
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
	elem     *formatElement
	data     []byte
	pi       []byte
	line     int
	col      int
	kind     formatItemKind
	textMode xmlTextMode
}

type formatElement struct {
	namespace xmlns.Frame
	start     xml.StartElement
	children  []formatItem
	line      int
	col       int
	preserve  bool
}

func (f *xmlFormatter) format() error {
	for {
		tok, err := f.p.Next()
		if stream.IsOnlyEOF(err) {
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
	frame, _, err := f.ns.StartXML(start)
	if err != nil {
		return xmlFormatErr(tok.Line, tok.Column, err)
	}
	inherited, err := f.startWhitespaceMode(frame, tok.Line, tok.Column)
	if err != nil {
		return err
	}
	preserve := xmlSpacePreserve(start.Attr, inherited)
	elem := &formatElement{start: start, namespace: frame, line: tok.Line, col: tok.Column, preserve: preserve}
	if err := f.appendItem(formatItem{kind: formatItemElement, elem: elem, line: tok.Line, col: tok.Column}); err != nil {
		return f.abortStart(frame, err)
	}
	f.stack = append(f.stack, elem)
	return nil
}

func (f *xmlFormatter) startWhitespaceMode(frame xmlns.Frame, line, col int) (bool, error) {
	if len(f.stack) != 0 {
		return f.stack[len(f.stack)-1].preserve, nil
	}
	if f.rootSeen {
		err := xmlFormatErr(line, col, errors.New("XML document has multiple roots"))
		return false, f.abortStart(frame, err)
	}
	f.rootSeen = true
	return xmlSpaceDefault, nil
}

func (f *xmlFormatter) abortStart(frame xmlns.Frame, err error) error {
	if abortErr := f.ns.Abort(frame); abortErr != nil {
		return errors.Join(err, abortErr)
	}
	return err
}

func (f *xmlFormatter) collectEnd(tok stream.Token) error {
	if len(f.stack) == 0 {
		return xmlFormatErr(tok.Line, tok.Column, errors.New("unexpected end element"))
	}
	frame := f.stack[len(f.stack)-1]
	if err := f.ns.End(frame.namespace, xmlns.Lexical(tok.End.Name)); err != nil {
		return xmlFormatErr(tok.Line, tok.Column, err)
	}
	f.stack = f.stack[:len(f.stack)-1]
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
	textMode := xmlTextEscaped
	if tok.CDATA {
		textMode = xmlTextCDATA
	}
	return f.appendItem(formatItem{kind: formatItemText, data: tok.AppendData(nil), textMode: textMode, line: tok.Line, col: tok.Column})
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
		if err := writeXMLText(f.w, item.data, item.textMode); err != nil {
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
		return f.writeInlineElement(elem, depth)
	}
	return f.writeBlockElement(elem, depth)
}

func (f *xmlFormatter) writeInlineElement(elem *formatElement, depth int) error {
	for _, child := range elem.children {
		if err := f.writeItem(child, depth+1, formatInline); err != nil {
			return err
		}
	}
	return writeXMLFormatEnd(f.w, elem)
}

func (f *xmlFormatter) writeBlockElement(elem *formatElement, depth int) error {
	wroteChild := false
	for _, child := range elem.children {
		if child.ignorableBlockWhitespace() {
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

func (item formatItem) ignorableBlockWhitespace() bool {
	return item.kind == formatItemText && item.textMode == xmlTextEscaped && lex.IsXMLWhitespaceBytes(item.data)
}

func (e *formatElement) inline() bool {
	if e.preserve {
		return true
	}
	hasElement := false
	hasNonElementLayout := false
	for _, child := range e.children {
		switch child.inlineDisposition() {
		case inlineElement:
			hasElement = true
		case inlineLayout:
			hasNonElementLayout = true
		case inlineContent:
			return true
		case inlineIgnored:
		}
	}
	return !hasElement && hasNonElementLayout
}

type inlineItemDisposition uint8

const (
	inlineIgnored inlineItemDisposition = iota
	inlineElement
	inlineLayout
	inlineContent
)

func (item formatItem) inlineDisposition() inlineItemDisposition {
	switch item.kind {
	case formatItemElement:
		return inlineElement
	case formatItemComment, formatItemPI:
		return inlineLayout
	case formatItemText:
		if item.textMode == xmlTextCDATA || !lex.IsXMLWhitespaceBytes(item.data) || !hasXMLLineBreak(item.data) {
			return inlineContent
		}
		return inlineLayout
	default:
		return inlineIgnored
	}
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

type xmlTextMode uint8

const (
	xmlTextEscaped xmlTextMode = iota
	xmlTextCDATA
)

func writeXMLText(w io.Writer, data []byte, mode xmlTextMode) error {
	if mode == xmlTextCDATA {
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
	if stream.IsInputLimit(err) || errors.Is(err, errFormatOutputLimit) || stream.IsTokenLimit(err) || stream.IsAttributeLimit(err) {
		return formatLimitErr(line, col, err)
	}
	return formatXMLErr(line, col, err)
}

func formatOptionErr(err error) error {
	return formatErr(xsderrors.CodeFormatOption, 0, 0, err)
}

func formatXMLErr(line, col int, err error) error {
	return formatErr(xsderrors.CodeFormatXML, line, col, err)
}

func formatLimitErr(line, col int, err error) error {
	return formatErr(xsderrors.CodeFormatLimit, line, col, err)
}

func formatErr(code xsderrors.Code, line, col int, err error) error {
	return xsderrors.WithLocation("", line, col, xsderrors.Format(code, err))
}

func writeXMLStart(w io.Writer, start xml.StartElement) error {
	if _, err := io.WriteString(w, "<"); err != nil {
		return err
	}
	if err := writeXMLQName(w, start.Name); err != nil {
		return err
	}
	for _, attr := range start.Attr {
		if err := writeXMLAttribute(w, attr); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, ">")
	return err
}

func writeXMLAttribute(w io.Writer, attr xml.Attr) error {
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
	_, err := io.WriteString(w, "\"")
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
		esc := xmlAttributeEscape(value[i])
		if esc == "" {
			continue
		}
		if err := writeXMLAttributeChunk(w, value[start:i], esc); err != nil {
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

func xmlAttributeEscape(b byte) string {
	switch b {
	case '&':
		return "&amp;"
	case '<':
		return "&lt;"
	case '"':
		return "&quot;"
	case '\n':
		return "&#10;"
	case '\r':
		return "&#13;"
	case '\t':
		return "&#9;"
	default:
		return ""
	}
}

func writeXMLAttributeChunk(w io.Writer, raw, escaped string) error {
	if raw != "" {
		if _, err := io.WriteString(w, raw); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, escaped)
	return err
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
