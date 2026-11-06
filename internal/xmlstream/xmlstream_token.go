package xmlstream

import (
	"encoding/xml"
	"errors"
)

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

// ErrAttrValueSkipped reports that an attribute value was validated but not
// retained because Config.SkipOrdinaryAttrValues was set.
var ErrAttrValueSkipped = errors.New("xmlstream: attribute value skipped by reader configuration")

// Attr is a parsed attribute. A parser-produced value may be backed by raw
// parser storage that is valid only through the current token; read it with
// Reader.AttrValue, which returns an owned string. Values built with NewAttr
// are owned from construction.
type Attr struct {
	// Name is lexical before Reader.Start and expanded after successful
	// namespace admission. Namespace declarations use the XMLNS namespace URI
	// in the expanded form.
	Name    xml.Name
	value   string
	raw     []byte
	skipped bool
}

// NewAttr returns an attribute whose value is owned; AttrValue returns value without consulting the cache.
func NewAttr(name xml.Name, value string) Attr {
	return Attr{Name: name, value: value}
}

// attrValue returns the owned value, materializing raw parser storage through
// cache on first use.
func (a *Attr) attrValue(cache *cache) (string, error) {
	if a.skipped {
		return "", ErrAttrValueSkipped
	}
	if a.value == "" && len(a.raw) != 0 {
		a.value = cache.Intern(a.raw)
	}
	return a.value, nil
}

// RawValue returns the parser-owned raw value for a parser-produced attribute
// whose value was retained. It reports (nil, false) for skipped, empty and
// owned values. The result is valid only while the attribute's token is
// current and must not be retained; tests/stream_boundary_test.go
// (TestXMLStreamBorrowedValuesStayEphemeral) guards that boundary. Callers that
// need the value beyond the token use Reader.AttrValue.
func (a *Attr) RawValue() ([]byte, bool) {
	if len(a.raw) == 0 {
		return nil, false
	}
	return a.raw, true
}
