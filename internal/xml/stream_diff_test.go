package xsdxml

import (
	"encoding/xml"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

type normName struct {
	namespace string
	local     string
}

type normAttr struct {
	namespace string
	local     string
	value     string
}

type normEvent struct {
	name  normName
	text  string
	attrs []normAttr
	kind  EventKind
}

func TestStreamDecoderMatchesEncodingXML(t *testing.T) {
	tests := []string{
		`<root attr="v">text</root>`,
		`<root xmlns="urn:root" xmlns:p="urn:p"><p:child p:attr="v"/></root>`,
		`<root xmlns="urn:root"><child xmlns=""><inner/></child></root>`,
	}

	for _, xmlData := range tests {
		got, err := streamEvents(xmlData)
		if err != nil {
			t.Fatalf("stream events error for %q: %v", xmlData, err)
		}
		want, err := encodingXMLEvents(xmlData)
		if err != nil {
			t.Fatalf("encoding/xml events error for %q: %v", xmlData, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("event mismatch for %q:\n got: %#v\nwant: %#v", xmlData, got, want)
		}
	}
}

func streamEvents(xmlData string) ([]normEvent, error) {
	dec, err := NewStreamDecoder(strings.NewReader(xmlData))
	if err != nil {
		return nil, err
	}
	var events []normEvent
	for {
		ev, err := dec.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		switch ev.Kind {
		case EventStartElement:
			events = append(events, normEvent{
				kind: EventStartElement,
				name: normName{
					namespace: ev.Name.Namespace.String(),
					local:     ev.Name.Local,
				},
				attrs: normalizeAttrs(ev.Attrs),
			})
		case EventEndElement:
			events = append(events, normEvent{
				kind: EventEndElement,
				name: normName{
					namespace: ev.Name.Namespace.String(),
					local:     ev.Name.Local,
				},
			})
		case EventCharData:
			events = appendCharData(events, string(ev.Text))
		}
	}
	return events, nil
}

func encodingXMLEvents(xmlData string) ([]normEvent, error) {
	dec := xml.NewDecoder(strings.NewReader(xmlData))
	var events []normEvent
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			events = append(events, normEvent{
				kind: EventStartElement,
				name: normName{
					namespace: t.Name.Space,
					local:     t.Name.Local,
				},
				attrs: normalizeXMLAttrs(t.Attr),
			})
		case xml.EndElement:
			events = append(events, normEvent{
				kind: EventEndElement,
				name: normName{
					namespace: t.Name.Space,
					local:     t.Name.Local,
				},
			})
		case xml.CharData:
			events = appendCharData(events, string(t))
		}
	}
	return events, nil
}

func normalizeAttrs(attrs []Attr) []normAttr {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]normAttr, len(attrs))
	for i, attr := range attrs {
		out[i] = normAttr{
			namespace: attr.NamespaceURI(),
			local:     attr.LocalName(),
			value:     strings.Clone(attr.Value()),
		}
	}
	return out
}

func normalizeXMLAttrs(attrs []xml.Attr) []normAttr {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]normAttr, len(attrs))
	for i, attr := range attrs {
		out[i] = normAttr{
			namespace: normalizeXMLNamespace(attr.Name.Space, attr.Name.Local),
			local:     attr.Name.Local,
			value:     attr.Value,
		}
	}
	return out
}

func normalizeXMLNamespace(space, local string) string {
	if space == "xmlns" || (space == "" && local == "xmlns") {
		return XMLNSNamespace
	}
	return space
}

func appendCharData(events []normEvent, text string) []normEvent {
	if len(events) == 0 {
		return append(events, normEvent{kind: EventCharData, text: text})
	}
	last := &events[len(events)-1]
	if last.kind == EventCharData {
		last.text += text
		return events
	}
	return append(events, normEvent{kind: EventCharData, text: text})
}
