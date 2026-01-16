package xmlstream

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
)

var errNoStartElement = errors.New("expected start element event")
var errNilUnmarshaler = errors.New("nil Unmarshaler")

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
//
//nolint:gocritic // keep value semantics for immutable start events.
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

//nolint:gocritic // keep value semantics for start event comparisons.
func sameStartEvent(a, b Event) bool {
	if a.Kind != EventStartElement {
		return false
	}
	if a.ID != b.ID {
		return false
	}
	return a.Name == b.Name
}

// ReadSubtreeInto writes the current element subtree into dst.
// It returns io.ErrShortBuffer if dst is too small and still consumes the subtree.
func (r *Reader) ReadSubtreeInto(dst []byte) (int, error) {
	start, ok := r.consumeStart()
	if !ok {
		return 0, errNoStartElement
	}
	writer := subtreeWriter{dst: dst}
	if err := r.writeSubtree(&writer, start); err != nil {
		return writer.n, err
	}
	if writer.short {
		return writer.n, io.ErrShortBuffer
	}
	return writer.n, nil
}

// ReadSubtreeBytes returns the current element subtree as a newly allocated slice.
func (r *Reader) ReadSubtreeBytes() ([]byte, error) {
	start, ok := r.consumeStart()
	if !ok {
		return nil, errNoStartElement
	}
	var buf bytes.Buffer
	if err := r.writeSubtree(&buf, start); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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

//nolint:gocritic // keep value semantics for subtree start events.
func (r *Reader) writeSubtree(w io.Writer, start Event) error {
	enc := xml.NewEncoder(w)
	if err := encodeEvent(enc, start); err != nil {
		return err
	}
	depth := 1
	for depth > 0 {
		ev, err := r.Next()
		if err != nil {
			return err
		}
		if err := encodeEvent(enc, ev); err != nil {
			return err
		}
		switch ev.Kind {
		case EventStartElement:
			depth++
		case EventEndElement:
			depth--
		}
	}
	return enc.Flush()
}

//nolint:gocritic // keep value semantics for encoded events.
func encodeEvent(enc *xml.Encoder, ev Event) error {
	switch ev.Kind {
	case EventStartElement:
		attrs := make([]xml.Attr, 0, len(ev.Attrs))
		for _, attr := range ev.Attrs {
			attrs = append(attrs, xml.Attr{
				Name:  xml.Name{Space: attr.Name.Namespace, Local: attr.Name.Local},
				Value: string(attr.Value),
			})
		}
		return enc.EncodeToken(xml.StartElement{
			Name: xml.Name{Space: ev.Name.Namespace, Local: ev.Name.Local},
			Attr: attrs,
		})
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

func splitPI(data []byte) (string, []byte) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return "", nil
	}
	idx := bytes.IndexAny(data, " \t\n\r")
	if idx < 0 {
		return string(data), nil
	}
	target := string(data[:idx])
	inst := bytes.TrimSpace(data[idx:])
	return target, inst
}

type subtreeWriter struct {
	dst   []byte
	n     int
	short bool
}

func (w *subtreeWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.n >= len(w.dst) {
		w.short = true
		return len(p), nil
	}
	avail := len(w.dst) - w.n
	if len(p) > avail {
		w.n += copy(w.dst[w.n:], p[:avail])
		w.short = true
		return len(p), nil
	}
	w.n += copy(w.dst[w.n:], p)
	return len(p), nil
}
