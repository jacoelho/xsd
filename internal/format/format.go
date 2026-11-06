// Package format writes consistently indented XML documents for repository-owned tools.
package format

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jacoelho/xsd/internal/lex"
	"github.com/jacoelho/xsd/internal/vocab"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

const (
	maxFormatDepth              = 4096
	defaultMaxFormatNodes       = 1_000_000
	defaultMaxFormatInputBytes  = int64(64 << 20)
	defaultMaxFormatOutputBytes = int64(64 << 20)
	defaultMaxFormatTokenBytes  = int64(4 << 20)
	xmlEscapeAmp                = "&amp;"
	xmlEscapeLT                 = "&lt;"
	xmlEscapeLF                 = "&#xA;"
)

var (
	errFormatOutputLimit = errors.New("XML formatted output byte limit exceeded")
)

// Options controls XML formatting resource limits.
type Options struct {
	// MaxDepth limits nested XML elements. Zero uses the default formatter limit.
	MaxDepth int
	// MaxNodes limits logical XML items processed by the formatter. Zero uses
	// the default formatter limit.
	MaxNodes int
	// MaxInputBytes limits bytes read from input. Zero uses the default formatter limit.
	MaxInputBytes int64
	// MaxOutputBytes limits bytes written to w. Zero uses the default formatter limit.
	MaxOutputBytes int64
	// MaxTokenBytes limits retained XML token payload bytes. Zero uses the default formatter limit.
	MaxTokenBytes int64
}

// XML writes a consistently indented XML document from the caller-owned input.
//
// Formatting validates input into a flat source-span tape, then streams output
// with only depth-bounded layout frames. The tape retains offsets and layout
// directives, while payloads remain in the caller-owned input string.
func XML(w io.Writer, input string) error {
	return XMLWithOptions(w, input, Options{})
}

// XMLWithOptions writes a consistently indented XML document with resource limits.
//
// It validates the complete caller-owned input before writing, then replays the
// source spans through a streaming renderer. No generic XML tree or decoded
// document representation is retained. Malformed input therefore writes
// nothing. A nil result means the complete document was written; on a writer
// failure or output limit, w may hold partial output.
func XMLWithOptions(w io.Writer, input string, opts Options) error {
	if w == nil {
		return formatOptionErr(errors.New("nil writer"))
	}
	limits, err := normalizeFormatOptions(opts)
	if err != nil {
		return formatOptionErr(err)
	}
	tape, err := analyze(input, limits)
	if err != nil {
		return err
	}
	writer := newMaxBytesWriter(w, limits.MaxOutputBytes, errFormatOutputLimit)
	f := xmlFormatter{
		w:      writer,
		input:  input,
		tape:   tape,
		limits: limits,
	}
	err = f.format()
	var formatErr *xsderrors.Error
	if err != nil && !errors.As(err, &formatErr) {
		if limitErr := classifyLimit(0, 0, err, limits); limitErr != nil {
			return limitErr
		}
	}
	return err
}

func newReader(input string, limits Options) (*xmlstream.Reader, error) {
	reader := new(xmlstream.Reader)
	if err := resetReader(reader, input, limits); err != nil {
		reader.Detach()
		return nil, err
	}
	return reader, nil
}

func resetReader(reader *xmlstream.Reader, input string, limits Options) error {
	err := reader.Reset(strings.NewReader(input), xmlstream.Config{
		Limits: xmlstream.Limits{
			MaxInputBytes: limits.MaxInputBytes,
			MaxTokenBytes: limits.MaxTokenBytes,
			MaxDepth:      limits.MaxDepth,
		},
		CommentMode: xmlstream.CommentModeEmit,
		EmitPI:      true,
		// Ordinary attribute values are decoded from their source spans only
		// during rendering. Namespace declarations and xml:space remain retained
		// for namespace admission and layout decisions.
		LazyAttrValues:         true,
		SkipOrdinaryAttrValues: true,
	})
	if err != nil {
		return formatReaderErr(err, limits)
	}
	return nil
}

func formatReaderErr(err error, limits Options) error {
	if feature, ok := xmlstream.UnsupportedFeatureOf(err); ok {
		return unsupportedFormatFeature(feature, err)
	}
	if errors.Is(err, xmlstream.ErrXMLInputNilReader) {
		return formatOptionErr(errors.New("nil reader"))
	}
	return formatBoundaryErr(err, limits)
}

// unsupportedFormatFeature maps reader rejections on both the Reset and token
// paths, so a feature has one format code wherever the reader detects it.
// Token-path callers add the position; Reset rejections precede any token.
func unsupportedFormatFeature(feature xmlstream.UnsupportedFeature, err error) error {
	switch feature {
	case xmlstream.UnsupportedDTD:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedDTD, "", err)
	case xmlstream.UnsupportedNonUTF8:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedNonUTF8, "XML documents must be UTF-8", err)
	case xmlstream.UnsupportedXMLVersion:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedXMLVersion, err.Error(), nil)
	case xmlstream.UnsupportedEntityReference:
		return xsderrors.Unsupported(xsderrors.CodeUnsupportedExternal, "XML documents cannot reference undeclared entities", err)
	}
	return xsderrors.InternalInvariant("unknown xmlstream unsupported feature")
}

func formatBoundaryErr(err error, limits Options) error {
	if boundary, ok := errors.AsType[*xmlstream.Error](err); ok {
		return xmlFormatErr(boundary.Line, boundary.Column, boundary, limits)
	}
	return xmlFormatErr(0, 0, err, limits)
}

func normalizeFormatOptions(opts Options) (Options, error) {
	if opts.MaxDepth < 0 {
		return Options{}, errors.New("MaxDepth cannot be negative")
	}
	if opts.MaxNodes < 0 {
		return Options{}, errors.New("MaxNodes cannot be negative")
	}
	if opts.MaxInputBytes < 0 {
		return Options{}, errors.New("MaxInputBytes cannot be negative")
	}
	if opts.MaxOutputBytes < 0 {
		return Options{}, errors.New("MaxOutputBytes cannot be negative")
	}
	if opts.MaxTokenBytes < 0 {
		return Options{}, errors.New("MaxTokenBytes cannot be negative")
	}
	if opts.MaxDepth == 0 {
		opts.MaxDepth = maxFormatDepth
	}
	if opts.MaxNodes == 0 {
		opts.MaxNodes = defaultMaxFormatNodes
	}
	if opts.MaxInputBytes == 0 {
		opts.MaxInputBytes = defaultMaxFormatInputBytes
	}
	if opts.MaxOutputBytes == 0 {
		opts.MaxOutputBytes = defaultMaxFormatOutputBytes
	}
	if opts.MaxTokenBytes == 0 {
		opts.MaxTokenBytes = defaultMaxFormatTokenBytes
	}
	return opts, nil
}

// maxBytesWriter is the single owner of output accounting. It reports every
// incomplete delegated write as an error, including one whose writer returned
// a nil error, so formatter success implies complete output.
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

func (w *maxBytesWriter) WriteByte(value byte) error {
	_, err := w.Write([]byte{value})
	return err
}

func (w *maxBytesStringWriter) WriteByte(value byte) error {
	return w.maxBytesWriter.WriteByte(value)
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

// formatAnalysis is the only formatter stage that counts processed nodes and
// drives Reader admission. Rendering consumes only its complete tape.
type formatAnalysis struct {
	reader   *xmlstream.Reader
	stack    []analysisFrame
	tape     []formatEvent
	nodes    int
	maxNodes int
	limits   Options
}

type formatEvent struct {
	start            int
	end              int
	line             int
	column           int
	kind             xmlstream.TokenKind
	textKind         xmlstream.CharacterDataKind
	ignoreWhitespace bool
	// inline is meaningful on start events only, and is final only after the
	// matching end commits, because later content can change the decision.
	inline bool
}

type analysisFrame struct {
	handle              xmlstream.Handle
	name                xml.Name
	startEvent          int
	preserve            bool
	hasElement          bool
	hasNonElementLayout bool
	hasInlineContent    bool
}

type xmlFormatter struct {
	w             io.Writer
	input         string
	tape          []formatEvent
	stack         []formatFrame
	topLevelItems int
	limits        Options
}

type formatFrame struct {
	inline     bool
	wroteChild bool
}

func analyze(input string, limits Options) ([]formatEvent, error) {
	reader, err := newReader(input, limits)
	if err != nil {
		return nil, err
	}
	a := formatAnalysis{reader: reader, limits: limits, maxNodes: limits.MaxNodes}
	if err := a.format(); err != nil {
		reader.Detach()
		return nil, err
	}
	reader.Detach()
	return a.tape, nil
}

func (a *formatAnalysis) format() error {
	for {
		tok, err := a.reader.Next()
		if xmlstream.IsOnlyEOF(err) {
			return a.finishEOF()
		}
		if err != nil {
			return readerBoundaryError(a.reader, a.limits, err)
		}
		if err := a.collectToken(tok); err != nil {
			return err
		}
	}
}

func (a *formatAnalysis) finishEOF() error {
	if len(a.stack) > 0 {
		line, col := a.reader.Pos()
		return xmlFormatErr(line, col, fmt.Errorf("unexpected EOF before end element </%s>", xmlQName(a.stack[len(a.stack)-1].name)), a.limits)
	}
	if completeErr := a.reader.Complete(); completeErr != nil {
		if errors.Is(completeErr, xmlstream.ErrNoRoot) {
			return xmlFormatErr(1, 1, errors.New("XML document is empty"), a.limits)
		}
		return readerBoundaryError(a.reader, a.limits, completeErr)
	}
	return nil
}

//nolint:gocognit // This dispatch owns one state transition per validated XML token.
func (a *formatAnalysis) collectToken(tok *xmlstream.Token) error {
	switch tok.Kind { //nolint:exhaustive // Reader.Next rejects directives before consumers see them.
	case xmlstream.KindStart:
		if err := a.collectStart(tok); err != nil {
			return err
		}
	case xmlstream.KindEnd:
		if err := a.collectEnd(tok); err != nil {
			return err
		}
	case xmlstream.KindCharData:
		if err := a.collectChars(tok); err != nil {
			return err
		}
	case xmlstream.KindComment, xmlstream.KindPI:
		if err := a.appendNode(tok.Line, tok.Column); err != nil {
			return err
		}
		if len(a.stack) != 0 {
			a.stack[len(a.stack)-1].hasNonElementLayout = true
		}
	default:
		return xmlFormatErr(tok.Line, tok.Column, errors.New("unknown XML token"), a.limits)
	}
	a.tape = append(a.tape, formatEvent{
		start:            tok.StartOffset,
		end:              tok.EndOffset,
		line:             tok.Line,
		column:           tok.Column,
		kind:             tok.Kind,
		textKind:         tok.TextKind,
		ignoreWhitespace: tok.Kind == xmlstream.KindCharData && tokenIsIgnorableBlockWhitespace(tok),
	})
	return nil
}

func (a *formatAnalysis) collectStart(tok *xmlstream.Token) error {
	inherited := xmlSpaceDefault
	if len(a.stack) != 0 {
		inherited = a.stack[len(a.stack)-1].preserve
	}
	preserve, err := a.xmlSpacePreserve(tok.Start.Attr, inherited)
	if err != nil {
		return err
	}
	handle, _, err := a.reader.Start()
	if err != nil {
		return readerBoundaryError(a.reader, a.limits, err)
	}
	if err := a.appendNode(tok.Line, tok.Column); err != nil {
		return a.abortStart(handle, err)
	}
	startEvent := len(a.tape)
	if len(a.stack) != 0 {
		a.stack[len(a.stack)-1].hasElement = true
	}
	a.stack = append(a.stack, analysisFrame{
		handle:     handle,
		name:       tok.Start.Name,
		startEvent: startEvent,
		preserve:   preserve,
	})
	return nil
}

func (a *formatAnalysis) collectEnd(tok *xmlstream.Token) error {
	if len(a.stack) == 0 {
		return xmlFormatErr(tok.Line, tok.Column, errors.New("unexpected end element"), a.limits)
	}
	frame := &a.stack[len(a.stack)-1]
	if err := a.reader.MatchEnd(frame.handle); err != nil {
		return readerBoundaryError(a.reader, a.limits, err)
	}
	if err := a.reader.CommitEnd(frame.handle); err != nil {
		return readerBoundaryError(a.reader, a.limits, err)
	}
	a.tape[frame.startEvent].inline = frame.inline()
	a.stack = a.stack[:len(a.stack)-1]
	return nil
}

func (a *formatAnalysis) collectChars(tok *xmlstream.Token) error {
	if len(a.stack) == 0 {
		// Reader.Next rejects non-whitespace text, references, and CDATA
		// outside the document element; literal whitespace is the only token
		// admitted here.
		return nil
	}
	if err := a.appendNode(tok.Line, tok.Column); err != nil {
		return err
	}
	frame := &a.stack[len(a.stack)-1]
	if tokenIsInlineContent(tok) {
		frame.hasInlineContent = true
	} else {
		frame.hasNonElementLayout = true
	}
	return nil
}

func (a *formatAnalysis) appendNode(line, col int) error {
	if a.maxNodes > 0 && a.nodes+1 > a.maxNodes {
		return formatLimitErr(line, col, errors.New("XML node limit exceeded"))
	}
	a.nodes++
	return nil
}

func (a *formatAnalysis) abortStart(frame xmlstream.Handle, err error) error {
	if abortErr := a.reader.AbortStart(frame); abortErr != nil {
		return errors.Join(err, abortErr)
	}
	return err
}

func (f *xmlFormatter) format() error {
	for _, event := range f.tape {
		if err := f.renderEvent(event); err != nil {
			return err
		}
	}
	return nil
}

func (f *xmlFormatter) renderEvent(event formatEvent) error {
	switch event.kind { //nolint:exhaustive // The analysis tape cannot contain rejected directives.
	case xmlstream.KindStart:
		return f.renderStart(event)
	case xmlstream.KindEnd:
		return f.renderEnd(event)
	case xmlstream.KindCharData:
		return f.renderChars(event)
	case xmlstream.KindComment, xmlstream.KindPI:
		return f.renderMisc(event)
	default:
		return xmlFormatErr(event.line, event.column, errors.New("unknown XML token"), f.limits)
	}
}

func (f *xmlFormatter) renderMisc(event formatEvent) error {
	if err := f.prepareItem(); err != nil {
		return err
	}
	var err error
	if event.kind == xmlstream.KindComment {
		err = writeSourceComment(f.w, f.input, event.start, event.end)
	} else {
		err = writeSourcePI(f.w, f.input, event.start, event.end)
	}
	if err != nil {
		return xmlFormatErr(event.line, event.column, err, f.limits)
	}
	return nil
}

func (f *xmlFormatter) renderStart(event formatEvent) error {
	parentInline := len(f.stack) != 0 && f.stack[len(f.stack)-1].inline
	if err := f.prepareItem(); err != nil {
		return err
	}
	nameStart, nameEnd := sourceNameSpan(f.input, event.start, event.end)
	if err := writeSourceStart(f.w, f.input, event.end, nameStart, nameEnd); err != nil {
		return xmlFormatErr(event.line, event.column, err, f.limits)
	}
	f.stack = append(f.stack, formatFrame{
		inline: parentInline || event.inline,
	})
	return nil
}

func (f *xmlFormatter) renderEnd(event formatEvent) error {
	frame := &f.stack[len(f.stack)-1]
	if !frame.inline && frame.wroteChild {
		if err := writeXMLIndent(f.w, len(f.stack)-1); err != nil {
			return err
		}
	}
	if err := writeSourceEnd(f.w, f.input, event.start, event.end); err != nil {
		return xmlFormatErr(event.line, event.column, err, f.limits)
	}
	f.stack = f.stack[:len(f.stack)-1]
	return nil
}

func (f *xmlFormatter) renderChars(event formatEvent) error {
	if len(f.stack) == 0 {
		return nil
	}
	if !f.stack[len(f.stack)-1].inline && event.ignoreWhitespace {
		return nil
	}
	if err := f.prepareItem(); err != nil {
		return err
	}
	if err := writeSourceText(f.w, f.input, event); err != nil {
		return xmlFormatErr(event.line, event.column, err, f.limits)
	}
	return nil
}

func (f *xmlFormatter) prepareItem() error {
	if len(f.stack) == 0 {
		if f.topLevelItems > 0 {
			if err := writeXMLIndent(f.w, 0); err != nil {
				return err
			}
		}
		f.topLevelItems++
		return nil
	}
	parent := &f.stack[len(f.stack)-1]
	if parent.inline {
		return nil
	}
	if err := writeXMLIndent(f.w, len(f.stack)); err != nil {
		return err
	}
	parent.wroteChild = true
	return nil
}

func tokenIsInlineContent(tok *xmlstream.Token) bool {
	return tok.TextKind == xmlstream.CharacterDataCDATA || !lex.IsXMLWhitespaceBytes(tok.Data) || !hasXMLLineBreak(tok.Data)
}

func tokenIsIgnorableBlockWhitespace(tok *xmlstream.Token) bool {
	return tok.TextKind != xmlstream.CharacterDataCDATA && lex.IsXMLWhitespaceBytes(tok.Data)
}

func readerBoundaryError(reader *xmlstream.Reader, limits Options, err error) error {
	if feature, ok := xmlstream.UnsupportedFeatureOf(err); ok {
		line, col := reader.Pos()
		if boundary, isBoundary := errors.AsType[*xmlstream.Error](err); isBoundary && boundary.Line > 0 {
			line, col = boundary.Line, boundary.Column
		}
		return xsderrors.WithLocation("", line, col, unsupportedFormatFeature(feature, err))
	}
	if boundary, ok := errors.AsType[*xmlstream.Error](err); ok {
		switch {
		case errors.Is(boundary, xmlstream.ErrCDATOutsideRoot):
			return xmlFormatErr(boundary.Line, boundary.Column, errors.New("CDATA section outside root element"), limits)
		case errors.Is(boundary, xmlstream.ErrReferenceOutsideRoot):
			return xmlFormatErr(boundary.Line, boundary.Column, errors.New("reference outside root element"), limits)
		case errors.Is(boundary, xmlstream.ErrTextOutsideRoot):
			return xmlFormatErr(boundary.Line, boundary.Column, errors.New("text outside root element"), limits)
		}
		return xmlFormatErr(boundary.Line, boundary.Column, boundary, limits)
	}
	line, col := reader.Pos()
	return xmlFormatErr(line, col, err, limits)
}

func (f analysisFrame) inline() bool {
	if f.preserve || f.hasInlineContent {
		return true
	}
	return !f.hasElement && f.hasNonElementLayout
}

const xmlSpaceDefault = false

func (a *formatAnalysis) xmlSpacePreserve(attrs []xmlstream.Attr, inherited bool) (bool, error) {
	for i := range attrs {
		attr := &attrs[i]
		if !xmlstream.IsLexicalXMLSpaceName(attr.Name) {
			continue
		}
		value, err := a.reader.AttrValue(attr)
		if err != nil {
			return false, xsderrors.InternalInvariant("format reader skipped xml:space value")
		}
		switch value {
		case vocab.XMLValuePreserve:
			return true, nil
		case vocab.XMLValueDefault:
			return false, nil
		}
	}
	return inherited, nil
}

func hasXMLLineBreak(data []byte) bool {
	for _, b := range data {
		if b == '\n' || b == '\r' {
			return true
		}
	}
	return false
}

func xmlFormatErr(line, col int, err error, opts Options) error {
	if err == nil {
		return nil
	}
	if limitErr := classifyLimit(line, col, err, opts); limitErr != nil {
		return limitErr
	}
	return formatXMLErr(line, col, err)
}

// classifyLimit returns the format.limit diagnostic for the formatter output
// limit or a reader limit, and nil for any other error.
func classifyLimit(line, col int, err error, opts Options) error {
	if errors.Is(err, errFormatOutputLimit) {
		return formatLimitErr(line, col, err)
	}
	if limit, ok := xmlstream.LimitOf(err); ok {
		return formatReaderLimitErr(limit, line, col, err, opts)
	}
	return nil
}

// formatReaderLimitErr names the formatter option behind every reader limit.
// The formatter configures no attribute limit, so LimitAttrs is an invariant
// violation.
func formatReaderLimitErr(limit xmlstream.Limit, line, col int, err error, opts Options) error {
	var option string
	var configured int64
	switch limit {
	case xmlstream.LimitInputBytes:
		option, configured = "MaxInputBytes", opts.MaxInputBytes
	case xmlstream.LimitTokenBytes:
		option, configured = "MaxTokenBytes", opts.MaxTokenBytes
	case xmlstream.LimitDepth:
		option, configured = "MaxDepth", int64(opts.MaxDepth)
	case xmlstream.LimitAttrs:
		return xsderrors.InternalInvariant("format reader has no attribute limit")
	}
	return formatLimitErr(line, col, fmt.Errorf("%s=%d exceeded: %w", option, configured, err))
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

func xmlAttributeEscape(b byte) string {
	switch b {
	case '&':
		return xmlEscapeAmp
	case '<':
		return xmlEscapeLT
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

func xmlQName(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	return name.Space + ":" + name.Local
}
