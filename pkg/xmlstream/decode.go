package xmlstream

import (
	"encoding/xml"
	"errors"
	"io"
)

var errNoStartElement = errors.New("expected start element event")
var errNilUnmarshaler = errors.New("nil Unmarshaler")
var errNilWriter = errors.New("nil writer")

// Unmarshaler is implemented by types that can unmarshal themselves from XML.
type Unmarshaler interface {
	UnmarshalXMLStream(r *Reader, start Event) error
}

// Decode unmarshals the current element subtree into v.
func (r *Reader) Decode(v Unmarshaler) error {
	if v == nil {
		return errNilUnmarshaler
	}
	start, ok := r.consumeStart()
	if !ok {
		return errNoStartElement
	}
	return v.UnmarshalXMLStream(r, start)
}

// DecodeElement unmarshals using the provided start event.
func (r *Reader) DecodeElement(v Unmarshaler, start Event) error {
	if v == nil {
		return errNilUnmarshaler
	}
	if start.Kind != EventStartElement {
		return errNoStartElement
	}
	if r != nil && r.lastWasStart && sameStartEvent(r.lastStart, start) {
		r.lastWasStart = false
		r.lastStart = Event{}
	}
	return v.UnmarshalXMLStream(r, start)
}

func sameStartEvent(a, b Event) bool {
	if a.Kind != EventStartElement {
		return false
	}
	if a.ID != b.ID {
		return false
	}
	return a.Name == b.Name
}

// ReadSubtreeTo streams the current element subtree to w.
func (r *Reader) ReadSubtreeTo(w io.Writer) (int64, error) {
	if w == nil {
		return 0, errNilWriter
	}
	start, ok := r.consumeStart()
	if !ok {
		return 0, errNoStartElement
	}
	return r.writeSubtree(w, start)
}

func (r *Reader) consumeStart() (Event, bool) {
	if r == nil || !r.lastWasStart {
		return Event{}, false
	}
	start := r.lastStart
	if len(start.Attrs) == 0 && len(r.lastRawAttrs) > 0 {
		if cap(r.attrBuf) < len(r.lastRawAttrs) {
			r.attrBuf = make([]Attr, 0, len(r.lastRawAttrs))
		} else {
			r.attrBuf = r.attrBuf[:0]
		}
		for idx, raw := range r.lastRawAttrs {
			info := r.lastRawInfo[idx]
			r.attrBuf = append(r.attrBuf, Attr{
				Name:  r.names.internBytes(info.namespace, info.local),
				Value: raw.Value,
			})
		}
		start.Attrs = r.attrBuf
	}
	r.lastWasStart = false
	r.lastStart = Event{}
	r.lastRawAttrs = nil
	r.lastRawInfo = nil
	return start, true
}

func (r *Reader) writeSubtree(w io.Writer, start Event) (int64, error) {
	cw := &countingWriter{w: w}
	enc := xml.NewEncoder(cw)
	start = r.withInScopeNamespaceDecls(start)
	startName, err := encodeStartEvent(enc, start)
	if err != nil {
		return cw.n, err
	}
	nameStack := []xml.Name{startName}
	depth := 1
	for depth > 0 {
		ev, err := r.Next()
		if err != nil {
			return cw.n, err
		}
		switch ev.Kind {
		case EventStartElement:
			ev = r.withInScopeNamespaceDecls(ev)
			name, err := encodeStartEvent(enc, ev)
			if err != nil {
				return cw.n, err
			}
			nameStack = append(nameStack, name)
			depth++
		case EventEndElement:
			last := len(nameStack) - 1
			if last < 0 {
				if err := encodeEvent(enc, ev); err != nil {
					return cw.n, err
				}
				depth--
				continue
			}
			if err := enc.EncodeToken(xml.EndElement{Name: nameStack[last]}); err != nil {
				return cw.n, err
			}
			nameStack = nameStack[:last]
			depth--
		default:
			if err := encodeEvent(enc, ev); err != nil {
				return cw.n, err
			}
		}
	}
	return cw.n, enc.Flush()
}

func (r *Reader) withInScopeNamespaceDecls(start Event) Event {
	if r == nil || start.Kind != EventStartElement || start.ScopeDepth < 0 {
		return start
	}

	prefixSeen := make(map[string]struct{}, len(start.Attrs))
	for _, attr := range start.Attrs {
		if attr.Name.Namespace != XMLNSNamespace {
			continue
		}
		prefix := attr.Name.Local
		if prefix == "xmlns" {
			prefix = ""
		}
		prefixSeen[prefix] = struct{}{}
	}

	prefixOrder := make([]string, 0, 8)
	prefixURI := make(map[string]string, 8)
	for depth := 0; depth <= start.ScopeDepth; depth++ {
		for decl := range r.NamespaceDeclsSeq(depth) {
			if _, exists := prefixURI[decl.Prefix]; !exists {
				prefixOrder = append(prefixOrder, decl.Prefix)
			}
			prefixURI[decl.Prefix] = decl.URI
		}
	}
	if len(prefixURI) == 0 {
		return start
	}

	attrs := start.Attrs
	for _, prefix := range prefixOrder {
		if _, exists := prefixSeen[prefix]; exists {
			continue
		}
		local := prefix
		if local == "" {
			local = "xmlns"
		}
		attrs = append(attrs, Attr{
			Name:  QName{Namespace: XMLNSNamespace, Local: local},
			Value: []byte(prefixURI[prefix]),
		})
	}
	start.Attrs = attrs
	return start
}

func encodeEvent(enc *xml.Encoder, ev Event) error {
	switch ev.Kind {
	case EventStartElement:
		_, err := encodeStartEvent(enc, ev)
		return err
	case EventEndElement:
		return enc.EncodeToken(xml.EndElement{
			Name: xml.Name{Space: ev.Name.Namespace, Local: ev.Name.Local},
		})
	case EventCharData:
		return enc.EncodeToken(xml.CharData(ev.Text))
	case EventComment:
		return enc.EncodeToken(xml.Comment(ev.Text))
	case EventDirective:
		return enc.EncodeToken(xml.Directive(ev.Text))
	case EventPI:
		target, inst := splitPI(ev.Text)
		return enc.EncodeToken(xml.ProcInst{Target: target, Inst: inst})
	}
	return nil
}

func encodeStartEvent(enc *xml.Encoder, ev Event) (xml.Name, error) {
	prefixByNS := make(map[string]string, 8)
	prefixByNS[XMLNamespace] = "xml"
	declAttrs := make([]xml.Attr, 0, len(ev.Attrs))
	attrs := make([]xml.Attr, 0, len(ev.Attrs))
	for _, attr := range ev.Attrs {
		if attr.Name.Namespace != XMLNSNamespace {
			continue
		}
		prefix := ""
		local := "xmlns"
		if attr.Name.Local != "xmlns" {
			local = "xmlns:" + attr.Name.Local
			prefix = attr.Name.Local
		}
		ns := string(attr.Value)
		if existing, ok := prefixByNS[ns]; !ok || (existing == "" && prefix != "") {
			prefixByNS[ns] = prefix
		}
		declAttrs = append(declAttrs, xml.Attr{
			Name:  xml.Name{Local: local},
			Value: string(attr.Value),
		})
	}
	attrs = append(attrs, declAttrs...)
	for _, attr := range ev.Attrs {
		if attr.Name.Namespace == XMLNSNamespace {
			continue
		}
		name := xml.Name{Local: attr.Name.Local}
		if attr.Name.Namespace != "" {
			if prefix, ok := prefixByNS[attr.Name.Namespace]; ok && prefix != "" {
				name = xml.Name{Local: prefix + ":" + attr.Name.Local}
			} else {
				name = xml.Name{Space: attr.Name.Namespace, Local: attr.Name.Local}
			}
		}
		attrs = append(attrs, xml.Attr{
			Name:  name,
			Value: string(attr.Value),
		})
	}
	name := xml.Name{Space: ev.Name.Namespace, Local: ev.Name.Local}
	if ev.Name.Namespace != "" {
		if prefix, ok := prefixByNS[ev.Name.Namespace]; ok {
			if prefix == "" {
				name = xml.Name{Local: ev.Name.Local}
			} else {
				name = xml.Name{Local: prefix + ":" + ev.Name.Local}
			}
		}
	}
	if err := enc.EncodeToken(xml.StartElement{Name: name, Attr: attrs}); err != nil {
		return xml.Name{}, err
	}
	return name, nil
}

func splitPI(data []byte) (string, []byte) {
	start := 0
	end := len(data)
	for start < end && isXMLSpace(data[start]) {
		start++
	}
	for end > start && isXMLSpace(data[end-1]) {
		end--
	}
	data = data[start:end]
	if len(data) == 0 {
		return "", nil
	}
	idx := -1
	for i, b := range data {
		if isXMLSpace(b) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return string(data), nil
	}
	target := string(data[:idx])
	rest := data[idx:]
	for len(rest) > 0 && isXMLSpace(rest[0]) {
		rest = rest[1:]
	}
	for len(rest) > 0 && isXMLSpace(rest[len(rest)-1]) {
		rest = rest[:len(rest)-1]
	}
	return target, rest
}

func isXMLSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}
