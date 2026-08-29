package format

import (
	"io"
	"strings"
	"testing"
)

type benchmarkWriterOnly struct {
	w io.Writer
}

func (w benchmarkWriterOnly) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

func BenchmarkXMLDuplicateAttributes(b *testing.B) {
	doc := duplicateAttributeDoc()
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		var out strings.Builder
		if err := XML(&out, strings.NewReader(doc)); err == nil {
			b.Fatal("XML() succeeded")
		}
	}
}

func BenchmarkXMLLargeAttribute(b *testing.B) {
	value := strings.Repeat("a", 64<<10)
	doc := `<root a="` + value + `"/>`
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		var out strings.Builder
		if err := XML(&out, strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkXMLMixedEscapedAttribute(b *testing.B) {
	value := strings.Repeat("abc&amp;&#10;&quot;", 4096)
	doc := `<root a="` + value + `"/>`
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		var out strings.Builder
		if err := XML(&out, strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkXMLMixedEscapedAttributeWriterOnly(b *testing.B) {
	value := strings.Repeat("abc&amp;&#10;&quot;", 4096)
	doc := `<root a="` + value + `"/>`
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		var out strings.Builder
		if err := XML(benchmarkWriterOnly{w: &out}, strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func duplicateAttributeDoc() string {
	var sb strings.Builder
	sb.WriteString(`<root`)
	for i := range 256 {
		sb.WriteString(` a`)
		sb.WriteByte(byte('a' + i%26))
		sb.WriteString(`="`)
		sb.WriteString(strings.Repeat("x", 32))
		sb.WriteString(`"`)
	}
	sb.WriteString(`/>`)
	return sb.String()
}
