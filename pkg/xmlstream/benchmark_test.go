package xmlstream

import (
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"testing"
)

var rawItemLocal = []byte("item")

func BenchmarkXMLStream_UnmarshalStream(b *testing.B) {
	data := benchmarkInput(500)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		r, err := NewReader(bytes.NewReader(data))
		if err != nil {
			b.Fatalf("NewReader error = %v", err)
		}
		for {
			ev, err := r.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatalf("Next error = %v", err)
			}
			if ev.Kind == EventStartElement && ev.Name.Local == "item" {
				var item benchStreamItem
				if err := r.DecodeElement(&item, ev); err != nil {
					b.Fatalf("DecodeElement error = %v", err)
				}
			}
		}
	}
}

func BenchmarkXMLStream_NextRawScan(b *testing.B) {
	data := benchmarkInput(500)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		r, err := NewReader(bytes.NewReader(data))
		if err != nil {
			b.Fatalf("NewReader error = %v", err)
		}
		for {
			ev, err := r.NextRaw()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatalf("NextRaw error = %v", err)
			}
			if ev.Kind == EventStartElement && bytes.Equal(ev.Name.Local, rawItemLocal) {
				// No-op; scanning only.
			}
		}
	}
}

func BenchmarkXMLStream_SubtreeCopyUnmarshal(b *testing.B) {
	data := benchmarkInput(500)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		r, err := NewReader(bytes.NewReader(data))
		if err != nil {
			b.Fatalf("NewReader error = %v", err)
		}
		for {
			ev, err := r.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatalf("Next error = %v", err)
			}
			if ev.Kind == EventStartElement && ev.Name.Local == "item" {
				buf, err := r.ReadSubtreeBytes()
				if err != nil {
					b.Fatalf("ReadSubtreeBytes error = %v", err)
				}
				var item benchXMLItem
				if err := xml.Unmarshal(buf, &item); err != nil {
					b.Fatalf("xml.Unmarshal error = %v", err)
				}
			}
		}
	}
}

func BenchmarkEncodingXML_DecodeElement(b *testing.B) {
	data := benchmarkInput(500)
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatalf("Token error = %v", err)
			}
			start, ok := tok.(xml.StartElement)
			if !ok {
				continue
			}
			if start.Name.Local != "item" {
				continue
			}
			var item benchXMLItem
			if err := dec.DecodeElement(&item, &start); err != nil {
				b.Fatalf("DecodeElement error = %v", err)
			}
		}
	}
}

type benchStreamItem struct {
	ID     string
	Title  string
	Author string
}

func (b *benchStreamItem) UnmarshalXMLStream(r *Reader, start Event) error {
	if start.Kind != EventStartElement || start.Name.Local != "item" {
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

type benchXMLItem struct {
	ID     string `xml:"id,attr"`
	Title  string `xml:"title"`
	Author string `xml:"author"`
}

func benchmarkInput(items int) []byte {
	var b strings.Builder
	b.Grow(items * 64)
	b.WriteString("<root>")
	for i := 0; i < items; i++ {
		b.WriteString("<item id=\"")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\"><title>Title")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("</title><author>Author")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("</author></item>")
	}
	b.WriteString("</root>")
	return []byte(b.String())
}
