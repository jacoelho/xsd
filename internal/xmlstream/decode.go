package xmlstream

import "errors"

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
