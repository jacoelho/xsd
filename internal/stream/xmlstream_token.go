package stream

import "encoding/xml"

// StartElement is a parsed XML start element.
type StartElement struct {
	// Name.Space is the lexical namespace prefix.
	Name xml.Name
	Attr []Attr
}

// EndElement is a parsed XML end element.
type EndElement struct {
	// Name.Space is the lexical namespace prefix.
	Name xml.Name
}

// Attr may hold a parser-owned raw value. raw is valid only through the current
// token. Callers that retain a value must call StringValue or AppendValue before
// advancing the parser, then retain only the returned string or byte copy.
type Attr struct {
	Name  xml.Name
	Value string
	raw   []byte
}

// OwnedAttr returns an attribute whose value does not borrow parser storage.
func OwnedAttr(name xml.Name, value string) Attr {
	return Attr{Name: name, Value: value}
}

// OwnedAttrs returns an owned copy of attrs, materializing borrowed values.
func OwnedAttrs(attrs ...Attr) []Attr {
	owned := make([]Attr, len(attrs))
	for i, attr := range attrs {
		value := attr.Value
		if value == "" && len(attr.raw) != 0 {
			value = string(attr.raw)
		}
		owned[i] = Attr{Name: attr.Name, Value: value}
	}
	return owned
}

// OwnedStartElement returns a start element that does not borrow parser storage.
func OwnedStartElement(name xml.Name, attrs ...Attr) StartElement {
	return StartElement{Name: name, Attr: OwnedAttrs(attrs...)}
}

// StringValue returns the attribute value as an owned string. It must be called
// while the token that produced a raw-backed attribute is still current.
func (a *Attr) StringValue(cache *Cache) string {
	if a.Value == "" && len(a.raw) != 0 {
		a.Value = cache.Intern(a.raw)
	}
	return a.Value
}

// AppendValue appends the attribute value to dst. It must be called while the
// token that produced a raw-backed attribute is still current.
func (a *Attr) AppendValue(dst []byte, cache *Cache) []byte {
	if a.Value != "" || len(a.raw) == 0 {
		return append(dst, a.StringValue(cache)...)
	}
	return append(dst, a.raw...)
}

// HasBorrowedValue reports whether the value is parser-owned.
func (a *Attr) HasBorrowedValue() bool {
	return len(a.raw) != 0
}

// RawValue returns the borrowed raw value when one is available. Repository
// boundary checks restrict the result to the validation fast path; it must not
// be retained or used after the parser advances.
func (a *Attr) RawValue() ([]byte, bool) {
	if len(a.raw) == 0 {
		return nil, false
	}
	return a.raw, true
}

// AppendData appends token character data or PI target bytes to dst.
func (t Token) AppendData(dst []byte) []byte {
	return append(dst, t.Data...)
}

// AppendDirective appends token directive/comment/PI content bytes to dst.
func (t Token) AppendDirective(dst []byte) []byte {
	return append(dst, t.Directive...)
}

// XMLStartElement converts s to encoding/xml's StartElement.
func (s StartElement) XMLStartElement() xml.StartElement {
	attrs := make([]xml.Attr, len(s.Attr))
	for i, attr := range s.Attr {
		value := attr.Value
		if value == "" && len(attr.raw) != 0 {
			value = string(attr.raw)
		}
		attrs[i] = xml.Attr{Name: attr.Name, Value: value}
	}
	return xml.StartElement{Name: s.Name, Attr: attrs}
}
