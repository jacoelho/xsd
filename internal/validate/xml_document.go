package validate

import (
	"encoding/binary"
	"encoding/xml"
	"errors"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/diagnostictext"
	"github.com/jacoelho/xsd/internal/xmlstream"
	"github.com/jacoelho/xsd/xsderrors"
)

type xmlDocument[P any] struct {
	elements      []xmlDocumentElement[P]
	retainedPaths documentPathStore
	// pathText is the rendered path of the first pathTextDepth open elements,
	// kept so diagnostics under one ancestor chain share one rendering. It
	// never exceeds maxDiagnosticPathBytes: a longer path is elided and
	// rendered per diagnostic instead.
	pathText      string
	pathScratch   []byte
	pathTextDepth int
}

// xmlDocumentElement stores the expanded name only; prefixes are never
// retained, so paths render as lexical local names or expanded names.
type xmlDocumentElement[P any] struct {
	payload P
	name    xml.Name
	handle  xmlstream.Handle
	pathRef documentPathRef
	// pathEnd is this element's end offset in xmlDocument.pathText; valid only
	// below pathTextDepth.
	pathEnd  int
	pathMode xmlPathMode
}

// retainedPath preserves an exact diagnostic path within its validation call.
// IDs, IDREFs, and published identity tuples keep one when their element may
// pop before the diagnostic is emitted; pending selections keep only a depth
// and project it from the live path instead. The string is materialized only
// when a diagnostic is emitted.
type retainedPath struct {
	store *documentPathStore
	ref   documentPathRef
}

func (p retainedPath) String() string {
	return p.store.pathString(p.ref)
}

// documentPathStore holds document-owned, parent-linked encoded path suffixes
// that share live prefixes. Each live element is encoded at most once between
// rollback or pop boundaries (its pathRef), and namespace URIs are stored once
// per suffix, not per occurrence. References never outlive the validation
// call: a fatal start rollback clears references into new nodes and truncates
// them, semantic discard releases all of them, and reset drops capacity above
// the session high-water bound.
type documentPathStore struct {
	namespaceScratch map[string]int
	nodes            []documentPathNode
	namespaces       []string
}

type documentPathNode struct {
	text           string
	parent         documentPathRef
	namespaceStart int
	namespaceCount int
}

// documentPathRef identifies a complete segment boundary in an encoded node.
// Node indexes are one-based so the zero value is invalid.
type documentPathRef struct {
	node int
	end  int
}

type documentPathNamespaceEncoder struct {
	store *documentPathStore
	ids   map[string]int
	start int
}

type documentPathEncoding struct {
	text           string
	namespaceStart int
	namespaceCount int
}

type documentPathExtension struct {
	parent documentPathRef
	start  int
}

type xmlPathMode uint8

const (
	xmlPathInvalid xmlPathMode = iota
	xmlPathLexical
	xmlPathExpanded
)

const (
	documentPathNamespaceLinearLimit  = 8
	documentPathExpandedSegmentMarker = 0 // XML 1.0 forbids NUL in names.
)

type preparedXMLStart struct {
	name   xml.Name
	handle xmlstream.Handle
}

type xmlDocumentCheckpoint struct {
	depth          int
	pathNamespaces int
	pathNodes      int
}

func (d *xmlDocument[P]) startCheckpoint() xmlDocumentCheckpoint {
	return xmlDocumentCheckpoint{
		depth:          d.Depth(),
		pathNamespaces: len(d.retainedPaths.namespaces),
		pathNodes:      len(d.retainedPaths.nodes),
	}
}

func (d *xmlDocument[P]) rollbackStart(checkpoint xmlDocumentCheckpoint) {
	for i := range checkpoint.depth {
		if d.elements[i].pathRef.node > checkpoint.pathNodes {
			d.elements[i].pathRef = documentPathRef{}
		}
	}
	clear(d.elements[checkpoint.depth:])
	d.elements = d.elements[:checkpoint.depth]
	d.truncatePathText(checkpoint.depth)
	d.retainedPaths.truncate(checkpoint.pathNodes)
	d.retainedPaths.truncateNamespaces(checkpoint.pathNamespaces)
}

func (d *xmlDocument[P]) clearCurrentPayload() {
	if len(d.elements) == 0 {
		return
	}
	var zero P
	d.elements[len(d.elements)-1].payload = zero
}

func (d *xmlDocument[P]) PrepareStart(
	reader *xmlstream.Reader,
	limits Limits,
	line, col int,
) (preparedXMLStart, error) {
	handle, element, err := reader.Start()
	if err != nil {
		if errors.Is(err, xmlstream.ErrMultipleRoots) {
			return preparedXMLStart{}, validation(d.context(line, col), xsderrors.CodeValidationXML, msgMultipleRoots)
		}
		return preparedXMLStart{}, streamError(err, d.context(line, col), limits)
	}
	return preparedXMLStart{name: element, handle: handle}, nil
}

func (d *xmlDocument[P]) CommitStart(start preparedXMLStart, payload P) {
	d.appendStart(start, xmlPathLexical, payload)
}

func (d *xmlDocument[P]) CommitExpandedStart(start preparedXMLStart, payload P) {
	d.appendStart(start, xmlPathExpanded, payload)
}

func (d *xmlDocument[P]) appendStart(start preparedXMLStart, pathMode xmlPathMode, payload P) {
	d.elements = append(d.elements, xmlDocumentElement[P]{
		payload:  payload,
		name:     start.name,
		handle:   start.handle,
		pathMode: pathMode,
	})
}

func (*xmlDocument[P]) AbortStart(reader *xmlstream.Reader, start preparedXMLStart) error {
	return reader.AbortStart(start.handle)
}

func (d *xmlDocument[P]) ValidateEnd(reader *xmlstream.Reader, line, col int) error {
	if d.Depth() == 0 {
		return validation(d.context(line, col), xsderrors.CodeValidationXML, msgUnexpectedEnd)
	}

	if err := reader.MatchEnd(d.elements[len(d.elements)-1].handle); err != nil {
		return validation(d.context(line, col), xsderrors.CodeValidationXML, err.Error())
	}
	return nil
}

func (d *xmlDocument[P]) CommitEnd(reader *xmlstream.Reader) error {
	if d.Depth() == 0 {
		return xsderrors.InternalInvariant("cannot commit XML end element with no open element")
	}

	if err := reader.CommitEnd(d.elements[len(d.elements)-1].handle); err != nil {
		return xsderrors.InternalInvariant(err.Error())
	}

	i := len(d.elements) - 1
	d.elements[i] = xmlDocumentElement[P]{}
	d.elements = d.elements[:i]
	d.truncatePathText(i)
	return nil
}

// truncatePathText drops cached path text for elements at or above depth.
func (d *xmlDocument[P]) truncatePathText(depth int) {
	if d.pathTextDepth <= depth {
		return
	}
	d.pathTextDepth = depth
	if depth == 0 {
		d.pathText = ""
		return
	}
	d.pathText = d.pathText[:d.elements[depth-1].pathEnd]
}

// Complete reports document-end diagnostics at the reader position.
func (d *xmlDocument[P]) Complete(reader *xmlstream.Reader) error {
	if err := reader.Complete(); err != nil {
		line, col := readerPosition(reader)
		ctx := d.context(line, col)
		if errors.Is(err, xmlstream.ErrNoRoot) {
			return validation(ctx, xsderrors.CodeValidationXML, msgNoRoot)
		}
		if errors.Is(err, xmlstream.ErrUnclosedElements) {
			return validation(ctx, xsderrors.CodeValidationXML, msgUnclosedElement)
		}
		return validation(ctx, xsderrors.CodeValidationXML, err.Error())
	}
	return nil
}

func (d *xmlDocument[P]) Reset(maxRetainedCap int) {
	if cap(d.elements) > maxRetainedCap {
		d.elements = nil
	} else {
		clear(d.elements)
		d.elements = d.elements[:0]
	}
	d.pathText = ""
	d.pathTextDepth = 0
	d.retainedPaths.reset(maxRetainedCap)
}

func (d *xmlDocument[P]) Depth() int {
	return len(d.elements)
}

func (d *xmlDocument[P]) Current() (*P, bool) {
	if len(d.elements) == 0 {
		return nil, false
	}
	return &d.elements[len(d.elements)-1].payload, true
}

func (d *xmlDocument[P]) clearPayloads() {
	var zero P
	for i := range d.elements {
		d.elements[i].payload = zero
	}
}

// PathString renders the current instance path for a diagnostic.
func (d *xmlDocument[P]) PathString() string {
	return d.PathStringAtDepth(d.Depth())
}

// pathStringWithChild renders the current path extended by a child that is not
// committed yet.
func (d *xmlDocument[P]) pathStringWithChild(name xml.Name, mode xmlPathMode) string {
	depth := d.Depth()
	child := xmlPathSegment{name: name, mode: mode}
	if parent, ok := d.pathPrefix(depth); ok {
		buf := d.pathScratch[:0]
		buf = append(buf, parent...)
		buf = appendDiagnosticPathSegment(buf, child)
		d.pathScratch = buf[:0]
		if len(buf) <= maxDiagnosticPathBytes {
			return string(buf)
		}
	}
	return renderDiagnosticPath(depth+1, func(i int) xmlPathSegment {
		if i == depth {
			return child
		}
		return d.pathSegment(i)
	})
}

func (d *xmlDocument[P]) PathStringAtDepth(depth int) string {
	if depth < 0 || depth > d.Depth() {
		panic("XML path depth is invalid")
	}
	if depth == 0 {
		return "/"
	}
	if path, ok := d.pathPrefix(depth); ok {
		return path
	}
	return renderDiagnosticPath(depth, d.pathSegment)
}

// pathPrefix returns the unelided path of the first depth elements, extending
// the cached text as needed. It reports false, leaving the cache as it was,
// when that path exceeds maxDiagnosticPathBytes and must be elided.
func (d *xmlDocument[P]) pathPrefix(depth int) (string, bool) {
	if depth == 0 {
		return "", true
	}
	if depth <= d.pathTextDepth {
		return d.pathText[:d.elements[depth-1].pathEnd], true
	}
	buf := d.pathScratch[:0]
	buf = append(buf, d.pathText...)
	for i := d.pathTextDepth; i < depth; i++ {
		buf = appendDiagnosticPathSegment(buf, d.pathSegment(i))
		if len(buf) > maxDiagnosticPathBytes {
			d.pathScratch = buf[:0]
			return "", false
		}
		// Offsets at or above pathTextDepth are ignored until the cache
		// is committed below.
		d.elements[i].pathEnd = len(buf)
	}
	d.pathScratch = buf[:0]
	d.pathText = string(buf)
	d.pathTextDepth = depth
	return d.pathText, true
}

func (d *xmlDocument[P]) pathSegment(i int) xmlPathSegment {
	return xmlPathSegment{name: d.elements[i].name, mode: d.elements[i].pathMode}
}

func (d *xmlDocument[P]) retainPathAtDepth(depth int) retainedPath {
	if depth < 0 || depth > d.Depth() {
		panic("XML path depth is invalid")
	}
	if depth == 0 {
		panic("retained XML path requires an element")
	}
	if ref := d.elements[depth-1].pathRef; ref.node != 0 {
		return retainedPath{store: &d.retainedPaths, ref: ref}
	}

	extension := d.pathExtension(depth)
	return d.retainPathSuffix(extension.start, depth, extension.parent)
}

func (d *xmlDocument[P]) pathExtension(depth int) documentPathExtension {
	extension := documentPathExtension{}
	for i := depth - 1; i >= 0; i-- {
		if ref := d.elements[i].pathRef; ref.node != 0 {
			extension.parent = ref
			extension.start = i + 1
			break
		}
	}
	return extension
}

func (d *xmlDocument[P]) retainPathSuffix(start, depth int, parent documentPathRef) retainedPath {
	encoding := d.encodePathSuffix(start, depth)
	d.retainedPaths.nodes = append(d.retainedPaths.nodes, documentPathNode{
		text:           encoding.text,
		parent:         parent,
		namespaceStart: encoding.namespaceStart,
		namespaceCount: encoding.namespaceCount,
	})
	node := len(d.retainedPaths.nodes)
	position := 0
	namespaces := d.retainedPaths.namespaces[encoding.namespaceStart : encoding.namespaceStart+encoding.namespaceCount]
	for i := start; i < depth; i++ {
		if d.elements[i].pathRef.node != 0 {
			panic("XML path element already has a retained reference")
		}
		position += d.encodedPathSegmentLength(i, encoding.text[position:], namespaces)
		d.elements[i].pathRef = documentPathRef{node: node, end: position}
	}
	if position != len(encoding.text) {
		panic("retained XML path suffix is incomplete")
	}
	return retainedPath{
		store: &d.retainedPaths,
		ref:   documentPathRef{node: node, end: position},
	}
}

func (d *xmlDocument[P]) encodePathSuffix(start, depth int) documentPathEncoding {
	namespaces := documentPathNamespaceEncoder{store: &d.retainedPaths, start: len(d.retainedPaths.namespaces)}
	encodedLength := 0
	for i := start; i < depth; i++ {
		element := d.elements[i]
		switch element.pathMode {
		case xmlPathExpanded:
			namespaceID := namespaces.intern(element.name.Space)
			encodedLength += documentPathNameReferenceLength(namespaceID, element.name.Local)
		case xmlPathLexical:
			encodedLength += 1 + len(element.name.Local)
		case xmlPathInvalid:
			panic("XML path mode is invalid")
		default:
			panic("XML path mode is unknown")
		}
	}

	var text strings.Builder
	text.Grow(encodedLength)
	for i := start; i < depth; i++ {
		element := d.elements[i]
		switch element.pathMode {
		case xmlPathExpanded:
			namespaceID := namespaces.lookup(element.name.Space)
			appendDocumentPathNameReference(&text, namespaceID, element.name.Local)
		case xmlPathLexical:
			appendEncodedLexicalSegment(&text, element.name.Local)
		case xmlPathInvalid:
			panic("XML path mode is invalid")
		default:
			panic("XML path mode is unknown")
		}
	}
	encoding := documentPathEncoding{
		text:           text.String(),
		namespaceStart: namespaces.start,
		namespaceCount: len(d.retainedPaths.namespaces) - namespaces.start,
	}
	namespaces.release()
	return encoding
}

func (d *xmlDocument[P]) encodedPathSegmentLength(index int, text string, namespaces []string) int {
	element := d.elements[index]
	switch element.pathMode {
	case xmlPathExpanded:
		if text == "" || text[0] != documentPathExpandedSegmentMarker {
			panic("expanded XML path segment is not encoded")
		}
		_, consumed := decodeExpandedDocumentPathName(text[1:], namespaces)
		return 1 + consumed
	case xmlPathLexical:
		return 1 + len(element.name.Local)
	case xmlPathInvalid:
		panic("XML path mode is invalid")
	default:
		panic("XML path mode is unknown")
	}
}

func (e *documentPathNamespaceEncoder) intern(namespace string) int {
	if e.ids != nil {
		if id, ok := e.ids[namespace]; ok {
			return id
		}
		id := len(e.store.namespaces) - e.start
		e.store.namespaces = append(e.store.namespaces, namespace)
		e.ids[namespace] = id
		return id
	}
	namespaces := e.store.namespaces[e.start:]
	if id := slices.Index(namespaces, namespace); id >= 0 {
		return id
	}
	id := len(namespaces)
	e.store.namespaces = append(e.store.namespaces, namespace)
	if len(namespaces)+1 == documentPathNamespaceLinearLimit {
		e.indexNamespaces()
	}
	return id
}

func (e *documentPathNamespaceEncoder) lookup(namespace string) int {
	if e.ids != nil {
		if id, ok := e.ids[namespace]; ok {
			return id
		}
	} else if id := slices.Index(e.store.namespaces[e.start:], namespace); id >= 0 {
		return id
	}
	panic("expanded XML path namespace is not retained")
}

func (e *documentPathNamespaceEncoder) indexNamespaces() {
	if e.store.namespaceScratch == nil {
		e.store.namespaceScratch = make(map[string]int, documentPathNamespaceLinearLimit)
	}
	e.ids = e.store.namespaceScratch
	for id, namespace := range e.store.namespaces[e.start:] {
		e.ids[namespace] = id
	}
}

func (e *documentPathNamespaceEncoder) release() {
	clear(e.ids)
	e.ids = nil
}

func documentPathNameReferenceLength(namespaceID int, local string) int {
	return 1 + documentPathUintLength(namespaceID) + documentPathUintLength(len(local)) + len(local)
}

func documentPathUintLength(value int) int {
	var encoded [binary.MaxVarintLen64]byte
	return binary.PutUvarint(encoded[:], uint64(value)) //nolint:gosec // Path lengths and namespace IDs are non-negative.
}

func appendDocumentPathNameReference(text *strings.Builder, namespaceID int, local string) {
	text.WriteByte(documentPathExpandedSegmentMarker)
	appendDocumentPathUint(text, namespaceID)
	appendDocumentPathUint(text, len(local))
	text.WriteString(local)
}

func appendDocumentPathUint(text *strings.Builder, value int) {
	var encoded [binary.MaxVarintLen64]byte
	length := binary.PutUvarint(encoded[:], uint64(value)) //nolint:gosec // Path lengths and namespace IDs are non-negative.
	text.Write(encoded[:length])
}

// appendEncodedLexicalSegment writes the stored encoding of a lexical segment.
func appendEncodedLexicalSegment(text *strings.Builder, local string) {
	text.WriteByte('/')
	text.WriteString(local)
}

// maxDiagnosticPathBytes is an upper bound on every rendered instance path.
// One escaped value renders to at most 4*diagnostictext.MaxValueBytes+3 bytes,
// so an expanded segment ("/{" namespace "}" local) is at most
// 3+2*(4*MaxValueBytes+3) = 1033 bytes, under half the budget: the first and
// last segments always fit.
const maxDiagnosticPathBytes = 4096

const diagnosticPathElision = "/..."

type xmlPathSegment struct {
	name xml.Name
	mode xmlPathMode
}

func appendDiagnosticPathSegment(buf []byte, segment xmlPathSegment) []byte {
	buf = append(buf, '/')
	switch segment.mode {
	case xmlPathExpanded:
		buf = append(buf, '{')
		buf = diagnostictext.AppendEscaped(buf, segment.name.Space)
		buf = append(buf, '}')
	case xmlPathLexical:
	case xmlPathInvalid:
		panic("XML path mode is invalid")
	default:
		panic("XML path mode is unknown")
	}
	return diagnostictext.AppendEscaped(buf, segment.name.Local)
}

// maxDiagnosticPathSegmentBytes bounds one rendered segment; see
// maxDiagnosticPathBytes.
const maxDiagnosticPathSegmentBytes = 3 + 2*(4*diagnostictext.MaxValueBytes+3)

// renderDiagnosticPath renders count segments, read through segmentAt, as a
// display path. Names are bounded and escaped by diagnostictext. A path over
// maxDiagnosticPathBytes has its middle elided. Segments are read by index and
// rendering stops once the budget is exceeded, so the work and the scratch
// memory are bounded by the budget, not by the depth.
func renderDiagnosticPath(count int, segmentAt func(int) xmlPathSegment) string {
	if count == 0 {
		return "/"
	}
	// The loop exits after the first segment that crosses the budget, so this
	// buffer never grows.
	var stack [maxDiagnosticPathBytes + maxDiagnosticPathSegmentBytes]byte
	buf := stack[:0]
	prefix, prefixBytes := 0, 0
	for i := range count {
		buf = appendDiagnosticPathSegment(buf, segmentAt(i))
		if len(buf) > maxDiagnosticPathBytes {
			return elideDiagnosticPath(count, segmentAt, buf[:prefixBytes], prefix)
		}
		if len(buf) <= maxDiagnosticPathBytes/2 {
			prefix, prefixBytes = i+1, len(buf)
		}
	}
	return string(buf)
}

// elideDiagnosticPath writes head, the longest whole-segment prefix within half
// the budget covering the first prefix segments, then "/...", then the longest
// whole-segment suffix that fits the rest.
func elideDiagnosticPath(count int, segmentAt func(int) xmlPathSegment, head []byte, prefix int) string {
	remainder := maxDiagnosticPathBytes - len(head) - len(diagnosticPathElision)
	var scratch [maxDiagnosticPathSegmentBytes]byte
	suffix, suffixBytes := count, 0
	for suffix > prefix {
		segment := appendDiagnosticPathSegment(scratch[:0], segmentAt(suffix-1))
		if suffixBytes+len(segment) > remainder {
			break
		}
		suffixBytes += len(segment)
		suffix--
	}
	out := make([]byte, 0, len(head)+len(diagnosticPathElision)+suffixBytes)
	out = append(out, head...)
	out = append(out, diagnosticPathElision...)
	for i := suffix; i < count; i++ {
		out = appendDiagnosticPathSegment(out, segmentAt(i))
	}
	return string(out)
}

func (d *xmlDocument[P]) discardRetainedPaths() {
	for i := range d.elements {
		d.elements[i].pathRef = documentPathRef{}
	}
	d.retainedPaths.truncate(0)
	d.retainedPaths.truncateNamespaces(0)
}

func (s *documentPathStore) pathString(ref documentPathRef) string {
	if ref.node == 0 {
		panic("retained XML path node is invalid")
	}
	var inlineChain [32]documentPathRef
	chain := inlineChain[:0]
	for ref.node != 0 {
		current := s.referencedNode(ref)
		chain = append(chain, ref)
		ref = current.parent
	}

	var inline [32]xmlPathSegment
	segments := inline[:0]
	for _, ref := range slices.Backward(chain) {
		current := s.nodes[ref.node-1]
		segments = appendStoredPathSegments(segments, current.text[:ref.end], s.nodeNamespaces(current))
	}
	return renderDiagnosticPath(len(segments), func(i int) xmlPathSegment { return segments[i] })
}

func (s *documentPathStore) referencedNode(ref documentPathRef) documentPathNode {
	if ref.node <= 0 || ref.node > len(s.nodes) {
		panic("retained XML path node is invalid")
	}
	current := s.nodes[ref.node-1]
	if ref.end <= 0 || ref.end > len(current.text) {
		panic("retained XML path boundary is invalid")
	}
	if current.parent.node < 0 || current.parent.node >= ref.node ||
		(current.parent.node == 0 && current.parent.end != 0) {
		panic("retained XML path parent is invalid")
	}
	return current
}

func (s *documentPathStore) nodeNamespaces(node documentPathNode) []string {
	if node.namespaceStart < 0 || node.namespaceCount < 0 ||
		node.namespaceStart > len(s.namespaces)-node.namespaceCount {
		panic("retained XML path namespace range is invalid")
	}
	return s.namespaces[node.namespaceStart : node.namespaceStart+node.namespaceCount]
}

// appendStoredPathSegments decodes stored path text: lexical segments are
// "/local" runs and expanded segments start with the marker byte.
func appendStoredPathSegments(segments []xmlPathSegment, text string, namespaces []string) []xmlPathSegment {
	for text != "" {
		if text[0] == documentPathExpandedSegmentMarker {
			name, consumed := decodeExpandedDocumentPathName(text[1:], namespaces)
			segments = append(segments, xmlPathSegment{name: name, mode: xmlPathExpanded})
			text = text[1+consumed:]
			continue
		}
		if text[0] != '/' {
			panic("lexical XML path segment is not encoded")
		}
		end := len(text)
		if next := strings.IndexAny(text[1:], "/\x00"); next >= 0 {
			end = 1 + next
		}
		segments = append(segments, xmlPathSegment{name: xml.Name{Local: text[1:end]}, mode: xmlPathLexical})
		text = text[end:]
	}
	return segments
}

func decodeExpandedDocumentPathName(text string, namespaces []string) (xml.Name, int) {
	namespaceID, namespaceBytes := decodeDocumentPathUint(text)
	if namespaceID < 0 || namespaceID >= len(namespaces) {
		panic("expanded XML path namespace is invalid")
	}
	localLength, lengthBytes := decodeDocumentPathUint(text[namespaceBytes:])
	localStart := namespaceBytes + lengthBytes
	if localLength < 0 || localLength > len(text)-localStart {
		panic("expanded XML path local name is invalid")
	}
	localEnd := localStart + localLength
	return xml.Name{Space: namespaces[namespaceID], Local: text[localStart:localEnd]}, localEnd
}

func decodeDocumentPathUint(text string) (decoded, consumed int) {
	value, consumed := binary.Uvarint([]byte(text))
	if consumed <= 0 {
		panic("encoded XML path integer is invalid")
	}
	decoded = int(value) //nolint:gosec // The round trip below rejects values outside int's range.
	if decoded < 0 || uint64(decoded) != value {
		panic("encoded XML path integer is invalid")
	}
	return decoded, consumed
}

func (s *documentPathStore) truncate(length int) {
	clear(s.nodes[length:])
	s.nodes = s.nodes[:length]
}

func (s *documentPathStore) truncateNamespaces(length int) {
	clear(s.namespaces[length:])
	s.namespaces = s.namespaces[:length]
}

func (s *documentPathStore) reset(maxRetainedCap int) {
	oversizedNamespaces := cap(s.namespaces) > maxRetainedCap
	if cap(s.nodes) > maxRetainedCap {
		s.nodes = nil
	} else {
		s.truncate(0)
	}
	if oversizedNamespaces {
		s.namespaces = nil
	} else {
		s.truncateNamespaces(0)
	}
	if oversizedNamespaces {
		s.namespaceScratch = nil
	} else {
		clear(s.namespaceScratch)
	}
}

func (d *xmlDocument[P]) context(line, col int) startContext {
	return startContext{document: d, Line: line, Column: col}
}
