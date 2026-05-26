package xsd

import "encoding/xml"

type streamStartElement struct {
	Name xml.Name
	Attr []streamAttr
}

type streamAttr struct {
	Name  xml.Name
	Value string
	Raw   []byte
}

func (a *streamAttr) stringValue(cache *byteStringCache) string {
	if a.Value == "" && len(a.Raw) != 0 {
		a.Value = cache.intern(a.Raw)
	}
	return a.Value
}

func (s streamStartElement) xmlStartElement() xml.StartElement {
	attrs := make([]xml.Attr, len(s.Attr))
	for i, attr := range s.Attr {
		value := attr.Value
		if value == "" && len(attr.Raw) != 0 {
			value = string(attr.Raw)
		}
		attrs[i] = xml.Attr{Name: attr.Name, Value: value}
	}
	return xml.StartElement{Name: s.Name, Attr: attrs}
}
