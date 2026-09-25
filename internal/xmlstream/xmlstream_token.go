package xmlstream

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
// token. Callers that retain a value must use Reader.MaterializeValue or
// Reader.AppendValue before
// advancing the parser, then retain only the returned string or byte copy.
type Attr struct {
	Name  xml.Name
	Value string
	raw   []byte
}

// StringValue returns the attribute value as an owned string. It must be called
// while the token that produced a raw-backed attribute is still current.
func (a *Attr) stringValue(cache *cache) string {
	if a.Value == "" && len(a.raw) != 0 {
		a.Value = cache.Intern(a.raw)
	}
	return a.Value
}

// MaterializeValue returns the attribute value as an owned string when cache
// can materialize any parser-owned storage.
func (a *Attr) materializeValue(cache *cache) (string, bool) {
	if a.HasBorrowedValue() && cache == nil {
		return "", false
	}
	return a.stringValue(cache), true
}

// AppendValue appends the attribute value to dst. It must be called while the
// token that produced a raw-backed attribute is still current.

func (a *Attr) appendValue(dst []byte, cache *cache) []byte {
	if a.Value != "" || len(a.raw) == 0 {
		return append(dst, a.stringValue(cache)...)
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
