package xmlstream

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestReadSubtreeBytes(t *testing.T) {
	input := `<root><skip/><item><title>Go</title></item><tail/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // skip start
		t.Fatalf("skip start error = %v", err)
	}
	if err = r.SkipSubtree(); err != nil {
		t.Fatalf("SkipSubtree error = %v", err)
	}

	ev, err := r.Next() // item start
	if err != nil {
		t.Fatalf("item start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "item" {
		t.Fatalf("item event = %v %s", ev.Kind, ev.Name.String())
	}

	data, err := r.ReadSubtreeBytes()
	if err != nil {
		t.Fatalf("ReadSubtreeBytes error = %v", err)
	}
	var got struct {
		Title string `xml:"title"`
	}
	if err = xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("xml.Unmarshal error = %v", err)
	}
	if got.Title != "Go" {
		t.Fatalf("title = %q, want Go", got.Title)
	}

	ev, err = r.Next() // tail start
	if err != nil {
		t.Fatalf("tail start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "tail" {
		t.Fatalf("tail event = %v %s", ev.Kind, ev.Name.String())
	}
}

func TestReadSubtreeBytesEmptyElement(t *testing.T) {
	input := `<root><item/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // item start
		t.Fatalf("item start error = %v", err)
	}

	data, err := r.ReadSubtreeBytes()
	if err != nil {
		t.Fatalf("ReadSubtreeBytes error = %v", err)
	}
	var got struct {
		XMLName xml.Name `xml:"item"`
	}
	if err = xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("xml.Unmarshal error = %v", err)
	}
	if got.XMLName.Local != "item" {
		t.Fatalf("XMLName = %q, want item", got.XMLName.Local)
	}
}

func TestReadSubtreeBytesWithCommentsAndPI(t *testing.T) {
	input := `<root><item><!--c--><?pi data?></item></root>`
	r, err := NewReader(strings.NewReader(input), EmitComments(true), EmitPI(true))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // item start
		t.Fatalf("item start error = %v", err)
	}

	data, err := r.ReadSubtreeBytes()
	if err != nil {
		t.Fatalf("ReadSubtreeBytes error = %v", err)
	}
	if got := string(data); got != "<item><!--c--><?pi data?></item>" {
		t.Fatalf("ReadSubtreeBytes = %q, want <item><!--c--><?pi data?></item>", got)
	}
}

func TestReadSubtreeBytesReadError(t *testing.T) {
	input := `<root><item><child></item></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // item
		t.Fatalf("item start error = %v", err)
	}
	if _, err = r.ReadSubtreeBytes(); err == nil {
		t.Fatalf("ReadSubtreeBytes error = nil, want error")
	} else {
		var syntax *xmltext.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("ReadSubtreeBytes error type = %T, want *xmltext.SyntaxError", err)
		}
	}
}

func TestReadSubtreeBytesFollowedByDecode(t *testing.T) {
	input := `<root><a/><b/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // a
		t.Fatalf("a start error = %v", err)
	}
	if _, err = r.ReadSubtreeBytes(); err != nil {
		t.Fatalf("ReadSubtreeBytes error = %v", err)
	}
	if err = r.Decode(noopUnmarshal{}); !errors.Is(err, errNoStartElement) {
		t.Fatalf("Decode after ReadSubtreeBytes error = %v, want %v", err, errNoStartElement)
	}
}

func TestReadSubtreeBytes_UnmarshalStruct(t *testing.T) {
	input := `<root><book id="a&amp;b"><title>Go</title><author>Rob</author></book></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // book start
		t.Fatalf("book start error = %v", err)
	}

	data, err := r.ReadSubtreeBytes()
	if err != nil {
		t.Fatalf("ReadSubtreeBytes error = %v", err)
	}
	var got struct {
		XMLName xml.Name `xml:"book"`
		ID      string   `xml:"id,attr"`
		Title   string   `xml:"title"`
		Author  string   `xml:"author"`
	}
	if err = xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("xml.Unmarshal error = %v", err)
	}
	if got.ID != "a&b" {
		t.Fatalf("id = %q, want a&b", got.ID)
	}
	if got.Title != "Go" || got.Author != "Rob" {
		t.Fatalf("title/author = %q/%q, want Go/Rob", got.Title, got.Author)
	}
}

func TestReadSubtreeBytes_UnmarshalNestedPaths(t *testing.T) {
	input := `<root><book id="b1"><meta><title>Go</title><author>Rob</author><publisher><name>Acme</name></publisher></meta><extra><note>Note</note></extra></book></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // book start
		t.Fatalf("book start error = %v", err)
	}

	data, err := r.ReadSubtreeBytes()
	if err != nil {
		t.Fatalf("ReadSubtreeBytes error = %v", err)
	}
	var got struct {
		XMLName xml.Name `xml:"book"`
		ID      string   `xml:"id,attr"`
		Meta    struct {
			Title     string `xml:"title"`
			Author    string `xml:"author"`
			Publisher struct {
				Name string `xml:"name"`
			} `xml:"publisher"`
		} `xml:"meta"`
		ExtraNote string `xml:"extra>note"`
	}
	if err = xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("xml.Unmarshal error = %v", err)
	}
	if got.ID != "b1" {
		t.Fatalf("id = %q, want b1", got.ID)
	}
	if got.Meta.Title != "Go" || got.Meta.Author != "Rob" {
		t.Fatalf("meta = %+v, want title=Go author=Rob", got.Meta)
	}
	if got.Meta.Publisher.Name != "Acme" {
		t.Fatalf("publisher = %q, want Acme", got.Meta.Publisher.Name)
	}
	if got.ExtraNote != "Note" {
		t.Fatalf("extra note = %q, want Note", got.ExtraNote)
	}
}

func TestReadSubtreeBytes_UnmarshalPathOnly(t *testing.T) {
	input := `<root><book id="b1"><meta><title>Go</title><author>Rob</author><publisher><name>Acme</name></publisher></meta><extra><note>Note</note></extra></book></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // book start
		t.Fatalf("book start error = %v", err)
	}

	data, err := r.ReadSubtreeBytes()
	if err != nil {
		t.Fatalf("ReadSubtreeBytes error = %v", err)
	}
	var got struct {
		XMLName       xml.Name `xml:"book"`
		ID            string   `xml:"id,attr"`
		Title         string   `xml:"meta>title"`
		Author        string   `xml:"meta>author"`
		PublisherName string   `xml:"meta>publisher>name"`
		Note          string   `xml:"extra>note"`
	}
	if err = xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("xml.Unmarshal error = %v", err)
	}
	if got.ID != "b1" {
		t.Fatalf("id = %q, want b1", got.ID)
	}
	if got.Title != "Go" || got.Author != "Rob" {
		t.Fatalf("title/author = %q/%q, want Go/Rob", got.Title, got.Author)
	}
	if got.PublisherName != "Acme" {
		t.Fatalf("publisher name = %q, want Acme", got.PublisherName)
	}
	if got.Note != "Note" {
		t.Fatalf("note = %q, want Note", got.Note)
	}
}

func TestReadSubtreeIntoShortBuffer(t *testing.T) {
	input := `<root><item>data</item></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // item start
		t.Fatalf("item start error = %v", err)
	}

	buf := make([]byte, 4)
	_, err = r.ReadSubtreeInto(buf)
	if err == nil {
		t.Fatalf("ReadSubtreeInto error = nil, want %v", io.ErrShortBuffer)
	}
	if !errors.Is(err, io.ErrShortBuffer) {
		t.Fatalf("ReadSubtreeInto error = %v, want %v", err, io.ErrShortBuffer)
	}
}

func TestReadSubtreeIntoReadError(t *testing.T) {
	input := `<root><item><child></item></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // item
		t.Fatalf("item start error = %v", err)
	}
	buf := make([]byte, 64)
	if _, err = r.ReadSubtreeInto(buf); err == nil {
		t.Fatalf("ReadSubtreeInto error = nil, want error")
	} else {
		var syntax *xmltext.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("ReadSubtreeInto error type = %T, want *xmltext.SyntaxError", err)
		}
	}
}

func TestReadSubtreeIntoExactFit(t *testing.T) {
	input := `<root><a/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // a start
		t.Fatalf("a start error = %v", err)
	}
	data, err := r.ReadSubtreeBytes()
	if err != nil {
		t.Fatalf("ReadSubtreeBytes error = %v", err)
	}

	r, err = NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // a start
		t.Fatalf("a start error = %v", err)
	}
	buf := make([]byte, len(data))
	n, err := r.ReadSubtreeInto(buf)
	if err != nil {
		t.Fatalf("ReadSubtreeInto error = %v", err)
	}
	if n != len(data) {
		t.Fatalf("ReadSubtreeInto n = %d, want %d", n, len(data))
	}
	if !bytes.Equal(buf, data) {
		t.Fatalf("ReadSubtreeInto bytes = %q, want %q", buf, data)
	}
}

func TestReadSubtreeInto_UnmarshalNestedPaths(t *testing.T) {
	input := `<root><book id="b1"><meta><title>Go</title><author>Rob</author><publisher><name>Acme</name></publisher></meta><extra><note>Note</note></extra></book></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // book
		t.Fatalf("book start error = %v", err)
	}

	buf := make([]byte, 2048)
	n, err := r.ReadSubtreeInto(buf)
	if err != nil {
		t.Fatalf("ReadSubtreeInto error = %v", err)
	}
	var got struct {
		XMLName xml.Name `xml:"book"`
		ID      string   `xml:"id,attr"`
		Meta    struct {
			Title     string `xml:"title"`
			Author    string `xml:"author"`
			Publisher struct {
				Name string `xml:"name"`
			} `xml:"publisher"`
		} `xml:"meta"`
		ExtraNote string `xml:"extra>note"`
	}
	if err = xml.Unmarshal(buf[:n], &got); err != nil {
		t.Fatalf("xml.Unmarshal error = %v", err)
	}
	if got.ID != "b1" {
		t.Fatalf("id = %q, want b1", got.ID)
	}
	if got.Meta.Title != "Go" || got.Meta.Author != "Rob" {
		t.Fatalf("meta = %+v, want title=Go author=Rob", got.Meta)
	}
	if got.Meta.Publisher.Name != "Acme" {
		t.Fatalf("publisher = %q, want Acme", got.Meta.Publisher.Name)
	}
	if got.ExtraNote != "Note" {
		t.Fatalf("extra note = %q, want Note", got.ExtraNote)
	}
}

func TestDecode(t *testing.T) {
	input := `<root><item><title>Go</title></item><after/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // item start
		t.Fatalf("item start error = %v", err)
	}

	var got itemTitle
	if err = r.Decode(&got); err != nil {
		t.Fatalf("Decode error = %v", err)
	}
	if got.title != "Go" {
		t.Fatalf("title = %q, want Go", got.title)
	}

	ev, err := r.Next() // after start
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "after" {
		t.Fatalf("after event = %v %s", ev.Kind, ev.Name.String())
	}
}

func TestDecodeElement(t *testing.T) {
	input := `<root><book id="a&amp;b"><title>Go</title><author>Rob</author></book><after/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	start, err := r.Next() // book start
	if err != nil {
		t.Fatalf("book start error = %v", err)
	}

	var got bookStream
	if err = r.DecodeElement(&got, start); err != nil {
		t.Fatalf("DecodeElement error = %v", err)
	}
	if got.ID != "a&b" || got.Title != "Go" || got.Author != "Rob" {
		t.Fatalf("book = %#v, want id=a&b title=Go author=Rob", got)
	}

	ev, err := r.Next() // after start
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "after" {
		t.Fatalf("after event = %v %s", ev.Kind, ev.Name.String())
	}
}

func TestDecodeElementNilUnmarshaler(t *testing.T) {
	input := `<root><item/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	start, err := r.Next()
	if err != nil {
		t.Fatalf("item start error = %v", err)
	}
	if err = r.DecodeElement(nil, start); !errors.Is(err, errNilUnmarshaler) {
		t.Fatalf("DecodeElement nil error = %v, want %v", err, errNilUnmarshaler)
	}
}

func TestDecodeElementClearsMatchingStart(t *testing.T) {
	input := `<root/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	start, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if !r.lastWasStart {
		t.Fatalf("lastWasStart = false, want true")
	}
	if err = r.DecodeElement(noopUnmarshal{}, start); err != nil {
		t.Fatalf("DecodeElement error = %v", err)
	}
	if r.lastWasStart {
		t.Fatalf("lastWasStart = true, want false")
	}
}

func TestDecodeElementKeepsMismatchedStart(t *testing.T) {
	input := `<root/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if !r.lastWasStart {
		t.Fatalf("lastWasStart = false, want true")
	}
	mismatched := Event{Kind: EventStartElement, Name: QName{Local: "other"}, ID: 42}
	if err = r.DecodeElement(noopUnmarshal{}, mismatched); err != nil {
		t.Fatalf("DecodeElement error = %v", err)
	}
	if !r.lastWasStart {
		t.Fatalf("lastWasStart = false, want true")
	}
}

func TestSameStartEventWrongKind(t *testing.T) {
	a := Event{Kind: EventCharData, Name: QName{Local: "root"}, ID: 1}
	b := Event{Kind: EventStartElement, Name: QName{Local: "root"}, ID: 1}
	if sameStartEvent(a, b) {
		t.Fatalf("sameStartEvent = true, want false")
	}
}

func TestNextRawReadSubtreeBytes(t *testing.T) {
	input := `<root><item><title>Go</title></item><after/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.NextRaw()
	if err != nil {
		t.Fatalf("item start error = %v", err)
	}
	if ev.Kind != EventStartElement || string(ev.Name.Full) != "item" {
		t.Fatalf("item start = %v %q, want item start", ev.Kind, ev.Name.Full)
	}
	data, err := r.ReadSubtreeBytes()
	if err != nil {
		t.Fatalf("ReadSubtreeBytes error = %v", err)
	}
	var got struct {
		Title string `xml:"title"`
	}
	if err = xml.Unmarshal(data, &got); err != nil {
		t.Fatalf("xml.Unmarshal error = %v", err)
	}
	if got.Title != "Go" {
		t.Fatalf("title = %q, want Go", got.Title)
	}
}

func TestNextRawDecode(t *testing.T) {
	input := `<root><item><title>Go</title></item><after/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // item start
		t.Fatalf("item start error = %v", err)
	}
	var got itemTitle
	if err = r.Decode(&got); err != nil {
		t.Fatalf("Decode error = %v", err)
	}
	if got.title != "Go" {
		t.Fatalf("title = %q, want Go", got.title)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "after" {
		t.Fatalf("after event = %v %s, want after start", ev.Kind, ev.Name.String())
	}
}

func TestDecodeErrors(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root/>"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if err = r.Decode(nil); !errors.Is(err, errNilUnmarshaler) {
		t.Fatalf("Decode nil error = %v, want %v", err, errNilUnmarshaler)
	}
	if err = r.Decode(noopUnmarshal{}); !errors.Is(err, errNoStartElement) {
		t.Fatalf("Decode before start error = %v, want %v", err, errNoStartElement)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if err = r.DecodeElement(noopUnmarshal{}, Event{Kind: EventCharData}); !errors.Is(err, errNoStartElement) {
		t.Fatalf("DecodeElement wrong kind error = %v, want %v", err, errNoStartElement)
	}
}

func TestDecodeElementWrongKinds(t *testing.T) {
	var err error
	tests := []EventKind{
		EventEndElement,
		EventCharData,
		EventComment,
		EventPI,
		EventDirective,
	}
	for _, kind := range tests {
		ev := Event{Kind: kind}
		var r *Reader
		if err = r.DecodeElement(noopUnmarshal{}, ev); !errors.Is(err, errNoStartElement) {
			t.Fatalf("DecodeElement %v error = %v, want %v", kind, err, errNoStartElement)
		}
	}
}

func TestDecodeUnmarshalError(t *testing.T) {
	input := `<root/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	want := errors.New("unmarshal error")
	if err = r.Decode(errorUnmarshal{err: want}); !errors.Is(err, want) {
		t.Fatalf("Decode error = %v, want %v", err, want)
	}
}

func TestNestedDecode(t *testing.T) {
	input := `<root><item><title>Go</title></item><item><title>Rust</title></item></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	var got parentStream
	if err = r.Decode(&got); err != nil {
		t.Fatalf("Decode error = %v", err)
	}
	if len(got.titles) != 2 || got.titles[0] != "Go" || got.titles[1] != "Rust" {
		t.Fatalf("titles = %#v, want [Go Rust]", got.titles)
	}
}

func TestNextRawDecodeWithAttrs(t *testing.T) {
	input := `<root><book id="a&amp;b"><title>Go</title></book></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // book
		t.Fatalf("book start error = %v", err)
	}
	var got bookStream
	if err = r.Decode(&got); err != nil {
		t.Fatalf("Decode error = %v", err)
	}
	if got.ID != "a&b" {
		t.Fatalf("id = %q, want a&b", got.ID)
	}
}

func TestConsumeStartWithPreResolvedAttrs(t *testing.T) {
	input := `<root attr="value"/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	start, ok := r.consumeStart()
	if !ok {
		t.Fatalf("consumeStart ok = false, want true")
	}
	if len(start.Attrs) != len(ev.Attrs) {
		t.Fatalf("attrs len = %d, want %d", len(start.Attrs), len(ev.Attrs))
	}
	if got, ok := start.Attr("", "attr"); !ok || string(got) != "value" {
		t.Fatalf("attr = %q, ok=%v, want value, true", string(got), ok)
	}
}

func TestDecodeElementNilReader(t *testing.T) {
	var err error
	var r *Reader
	start := Event{Kind: EventStartElement, Name: QName{Local: "root"}, ID: 1}
	if err = r.DecodeElement(noopUnmarshal{}, start); err != nil {
		t.Fatalf("DecodeElement nil reader error = %v", err)
	}
}

func TestDecodeElementConsumesStart(t *testing.T) {
	input := `<root><item><name>Go</name></item><after/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	start, err := r.Next()
	if err != nil {
		t.Fatalf("item start error = %v", err)
	}
	var got itemTitle
	if err = r.DecodeElement(&got, start); err != nil {
		t.Fatalf("DecodeElement error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "after" {
		t.Fatalf("after event = %v %s, want after start", ev.Kind, ev.Name.String())
	}
}

func TestReadSubtreeIntoShortBufferConsumes(t *testing.T) {
	input := `<root><item><name>Go</name></item><after/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // item
		t.Fatalf("item start error = %v", err)
	}
	buf := make([]byte, 5)
	if _, err = r.ReadSubtreeInto(buf); !errors.Is(err, io.ErrShortBuffer) {
		t.Fatalf("ReadSubtreeInto error = %v, want %v", err, io.ErrShortBuffer)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "after" {
		t.Fatalf("after event = %v %s, want after start", ev.Kind, ev.Name.String())
	}
}

func TestReadSubtreeIntoEntities(t *testing.T) {
	input := `<root><item id="a&amp;b">x &amp; y</item></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // item
		t.Fatalf("item start error = %v", err)
	}
	buf := make([]byte, 0, 64)
	n, err := r.ReadSubtreeInto(buf[:cap(buf)])
	if err != nil {
		t.Fatalf("ReadSubtreeInto error = %v", err)
	}
	got := string(buf[:n])
	if got != `<item id="a&amp;b">x &amp; y</item>` {
		t.Fatalf("ReadSubtreeInto = %q, want <item id=\"a&amp;b\">x &amp; y</item>", got)
	}
}

func TestReadSubtreeIntoZeroLengthBuffer(t *testing.T) {
	input := `<root><item/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // item
		t.Fatalf("item start error = %v", err)
	}
	if _, err = r.ReadSubtreeInto(nil); !errors.Is(err, io.ErrShortBuffer) {
		t.Fatalf("ReadSubtreeInto error = %v, want %v", err, io.ErrShortBuffer)
	}
}

func TestNextRawDecodeReusesAttrBuf(t *testing.T) {
	input := `<root a="1" b="2"><child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	r.attrBuf = make([]Attr, 1, 10)
	before := cap(r.attrBuf)
	if _, err = r.ReadSubtreeBytes(); err != nil {
		t.Fatalf("ReadSubtreeBytes error = %v", err)
	}
	if cap(r.attrBuf) != before {
		t.Fatalf("attrBuf cap = %d, want %d", cap(r.attrBuf), before)
	}
}

func TestSplitPI(t *testing.T) {
	tests := []struct {
		input  string
		target string
		inst   string
	}{
		{"", "", ""},
		{"target", "target", ""},
		{"target data", "target", "data"},
		{" target\tdata ", "target", "data"},
		{"target   data here", "target", "data here"},
	}
	for _, tt := range tests {
		target, inst := splitPI([]byte(tt.input))
		if target != tt.target {
			t.Fatalf("splitPI(%q) target = %q, want %q", tt.input, target, tt.target)
		}
		if string(inst) != tt.inst {
			t.Fatalf("splitPI(%q) inst = %q, want %q", tt.input, string(inst), tt.inst)
		}
	}
}

func TestEncodeEventSpecialTokens(t *testing.T) {
	var err error
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err = encodeEvent(enc, Event{Kind: EventDirective, Text: []byte("DOCTYPE root")}); err != nil {
		t.Fatalf("encode directive error = %v", err)
	}
	if err = encodeEvent(enc, Event{Kind: EventPI, Text: []byte("pi data")}); err != nil {
		t.Fatalf("encode PI error = %v", err)
	}
	if err = encodeEvent(enc, Event{Kind: EventComment, Text: []byte("comment")}); err != nil {
		t.Fatalf("encode comment error = %v", err)
	}
	if err = enc.Flush(); err != nil {
		t.Fatalf("encoder flush error = %v", err)
	}
	if got := buf.String(); got != "<!DOCTYPE root><?pi data?><!--comment-->" {
		t.Fatalf("encoded = %q, want <!DOCTYPE root><?pi data?><!--comment-->", got)
	}
}

func TestEncodeEventEmptyDirective(t *testing.T) {
	var err error
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err = encodeEvent(enc, Event{Kind: EventDirective, Text: nil}); err != nil {
		t.Fatalf("encode directive error = %v", err)
	}
	if err = enc.Flush(); err != nil {
		t.Fatalf("encoder flush error = %v", err)
	}
	if got := buf.String(); got != "<!>" {
		t.Fatalf("encoded = %q, want <>", got)
	}
}

func TestSubtreeWriterExactCapacity(t *testing.T) {
	dst := make([]byte, 3)
	w := subtreeWriter{dst: dst}
	n, err := w.Write([]byte("abc"))
	if err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if n != 3 {
		t.Fatalf("Write n = %d, want 3", n)
	}
	if w.n != 3 || w.short {
		t.Fatalf("writer state n=%d short=%v, want n=3 short=false", w.n, w.short)
	}
	if string(dst) != "abc" {
		t.Fatalf("dst = %q, want abc", string(dst))
	}
}

func TestSubtreeWriterEmptyWrite(t *testing.T) {
	dst := make([]byte, 2)
	w := subtreeWriter{dst: dst}
	n, err := w.Write(nil)
	if err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if n != 0 || w.n != 0 || w.short {
		t.Fatalf("writer state n=%d short=%v, want n=0 short=false", w.n, w.short)
	}
}

func TestSubtreeWriterBufferFull(t *testing.T) {
	var err error
	dst := make([]byte, 1)
	w := subtreeWriter{dst: dst}
	if _, err = w.Write([]byte("a")); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if w.n != 1 || w.short {
		t.Fatalf("writer state n=%d short=%v, want n=1 short=false", w.n, w.short)
	}
	if _, err = w.Write([]byte("b")); err != nil {
		t.Fatalf("Write error = %v", err)
	}
	if w.n != 1 || !w.short {
		t.Fatalf("writer state n=%d short=%v, want n=1 short=true", w.n, w.short)
	}
}

func TestSubtreeWriterPartialThenOverflow(t *testing.T) {
	dst := make([]byte, 3)
	w := subtreeWriter{dst: dst}
	if n, err := w.Write([]byte("abcd")); err != nil {
		t.Fatalf("Write error = %v", err)
	} else if n != 4 {
		t.Fatalf("Write n = %d, want 4", n)
	}
	if w.n != 3 || !w.short {
		t.Fatalf("writer state n=%d short=%v, want n=3 short=true", w.n, w.short)
	}
	if string(dst) != "abc" {
		t.Fatalf("dst = %q, want abc", string(dst))
	}
	if n, err := w.Write([]byte("e")); err != nil {
		t.Fatalf("Write error = %v", err)
	} else if n != 1 {
		t.Fatalf("Write n = %d, want 1", n)
	}
	if w.n != 3 || !w.short {
		t.Fatalf("writer state n=%d short=%v, want n=3 short=true", w.n, w.short)
	}
}

func TestEncodeEventUnknownKind(t *testing.T) {
	var err error
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err = encodeEvent(enc, Event{Kind: EventKind(99)}); err != nil {
		t.Fatalf("encode unknown error = %v", err)
	}
	if err = enc.Flush(); err != nil {
		t.Fatalf("encoder flush error = %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("encoded = %q, want empty", buf.String())
	}
}

func TestWriteSubtreeEncodeErrorInCharData(t *testing.T) {
	input := `<root><item>text</item></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	start, err := r.Next()
	if err != nil {
		t.Fatalf("item start error = %v", err)
	}
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err = encodeEvent(enc, start); err != nil {
		t.Fatalf("encode start error = %v", err)
	}
	if err = enc.Flush(); err != nil {
		t.Fatalf("encoder flush error = %v", err)
	}
	writer := &byteLimitWriter{limit: buf.Len()}
	if err = r.writeSubtree(writer, start); err == nil {
		t.Fatalf("writeSubtree error = nil, want error")
	}
}

func TestWriteSubtreeStartEncodeError(t *testing.T) {
	var err error
	r := &Reader{}
	start := Event{Kind: EventStartElement, Name: QName{}}
	if err = r.writeSubtree(io.Discard, start); err == nil {
		t.Fatalf("writeSubtree error = nil, want error")
	} else if errors.Is(err, errNilReader) {
		t.Fatalf("writeSubtree error = %v, want encode error", err)
	}
}

func TestWriteSubtreeWriterError(t *testing.T) {
	input := `<root><item><title>Go</title></item></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	start, err := r.Next()
	if err != nil {
		t.Fatalf("item start error = %v", err)
	}
	writer := &failingWriter{err: errors.New("write failed")}
	if err = r.writeSubtree(writer, start); err == nil {
		t.Fatalf("writeSubtree error = nil, want error")
	}
}

type failingWriter struct {
	err error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	if w.err == nil {
		return len(p), nil
	}
	return 0, w.err
}

func TestWriteSubtreeWriterErrorAfterStart(t *testing.T) {
	input := `<root><item>text</item></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	start, err := r.Next()
	if err != nil {
		t.Fatalf("item start error = %v", err)
	}
	writer := &byteLimitWriter{limit: 8}
	if err = r.writeSubtree(writer, start); err == nil {
		t.Fatalf("writeSubtree error = nil, want error")
	}
}

func TestWriteSubtreeFlushError(t *testing.T) {
	input := `<root><item>text</item></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	start, err := r.Next()
	if err != nil {
		t.Fatalf("item start error = %v", err)
	}
	writer := &flushFailWriter{err: errors.New("flush failed")}
	if err = r.writeSubtree(writer, start); err == nil {
		t.Fatalf("writeSubtree error = nil, want error")
	}
	if writer.calls == 0 {
		t.Fatalf("flush writer calls = 0, want > 0")
	}
}

type byteLimitWriter struct {
	limit int
	n     int
}

func (w *byteLimitWriter) Write(p []byte) (int, error) {
	if w.n >= w.limit {
		return 0, errors.New("write limit reached")
	}
	avail := w.limit - w.n
	if len(p) > avail {
		w.n += avail
		return avail, errors.New("write limit reached")
	}
	w.n += len(p)
	return len(p), nil
}

type flushFailWriter struct {
	err   error
	calls int
}

func (w *flushFailWriter) Write(p []byte) (int, error) {
	w.calls++
	return 0, w.err
}

type itemTitle struct {
	title string
}

func (i *itemTitle) UnmarshalXMLStream(r *Reader, start Event) error {
	if start.Kind != EventStartElement || start.Name.Local != "item" {
		return errNoStartElement
	}
	var current string
	for {
		ev, err := r.Next()
		if err != nil {
			return err
		}
		switch ev.Kind {
		case EventStartElement:
			current = ev.Name.Local
		case EventCharData:
			if current == "title" {
				i.title = string(ev.Text)
			}
		case EventEndElement:
			if ev.Name.Local == start.Name.Local && ev.Name.Namespace == start.Name.Namespace {
				return nil
			}
			current = ""
		}
	}
}

type bookStream struct {
	ID     string
	Title  string
	Author string
}

func (b *bookStream) UnmarshalXMLStream(r *Reader, start Event) error {
	if start.Kind != EventStartElement || start.Name.Local != "book" {
		return errNoStartElement
	}
	if id, ok := start.Attr("", "id"); ok {
		b.ID = string(id)
	}
	var current string
	for {
		ev, err := r.Next()
		if err != nil {
			return err
		}
		switch ev.Kind {
		case EventStartElement:
			current = ev.Name.Local
		case EventCharData:
			switch current {
			case "title":
				b.Title = string(ev.Text)
			case "author":
				b.Author = string(ev.Text)
			}
		case EventEndElement:
			if ev.Name.Local == start.Name.Local && ev.Name.Namespace == start.Name.Namespace {
				return nil
			}
			current = ""
		}
	}
}

type noopUnmarshal struct{}

func (noopUnmarshal) UnmarshalXMLStream(*Reader, Event) error {
	return nil
}

type errorUnmarshal struct {
	err error
}

func (e errorUnmarshal) UnmarshalXMLStream(*Reader, Event) error {
	return e.err
}

type parentStream struct {
	titles []string
}

func (p *parentStream) UnmarshalXMLStream(r *Reader, start Event) error {
	if start.Kind != EventStartElement || start.Name.Local != "root" {
		return errNoStartElement
	}
	for {
		ev, err := r.Next()
		if err != nil {
			return err
		}
		switch ev.Kind {
		case EventStartElement:
			if ev.Name.Local != "item" {
				continue
			}
			var item itemTitle
			itemErr := r.Decode(&item)
			if itemErr != nil {
				return itemErr
			}
			p.titles = append(p.titles, item.title)
		case EventEndElement:
			if ev.Name.Local == start.Name.Local && ev.Name.Namespace == start.Name.Namespace {
				return nil
			}
		}
	}
}
