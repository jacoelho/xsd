package xmltext

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	benchOnce sync.Once
	benchData []byte
	errBench  error
)

func loadBenchData(tb testing.TB) []byte {
	benchOnce.Do(func() {
		path := filepath.Join("..", "..", "testdata", "gml", "xsd", "gml.xsd")
		benchData, errBench = os.ReadFile(path)
	})
	if errBench != nil {
		tb.Fatalf("read benchmark data error = %v", errBench)
	}
	return benchData
}

func BenchmarkXMLTextDecoder(b *testing.B) {
	data := loadBenchData(b)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()

	dec := NewDecoder(bytes.NewReader(data))
	var tok Token

	for b.Loop() {
		dec.Reset(bytes.NewReader(data))
		for {
			err := dec.ReadTokenInto(&tok)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				b.Fatalf("ReadToken error = %v", err)
			}
		}
	}
}

func BenchmarkXMLTextDecoderEncodingXML(b *testing.B) {
	data := loadBenchData(b)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()

	opts := JoinOptions(
		ResolveEntities(true),
		EmitComments(true),
		EmitPI(true),
		EmitDirectives(true),
		CoalesceCharData(true),
	)
	dec := NewDecoder(bytes.NewReader(data), opts)
	var tok Token

	for b.Loop() {
		dec.Reset(bytes.NewReader(data), opts)
		for {
			err := dec.ReadTokenInto(&tok)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				b.Fatalf("ReadToken error = %v", err)
			}
		}
	}
}

func BenchmarkEncodingXMLDecoder(b *testing.B) {
	data := loadBenchData(b)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()

	for b.Loop() {
		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, err := dec.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				b.Fatalf("encoding/xml error = %v", err)
			}
		}
	}
}

func BenchmarkScanCharDataSpanParse(b *testing.B) {
	resolver := &entityResolver{}
	benchmarks := []struct {
		name string
		data []byte
	}{
		{name: "PlainASCII", data: []byte(strings.Repeat("plain text > content ", 512))},
		{name: "EntityHeavy", data: []byte(strings.Repeat("a&amp;b ", 512))},
		{name: "BracketHeavy", data: []byte(strings.Repeat("]]a ", 512))},
		{name: "UTF8", data: []byte(strings.Repeat("cafe\u00e9 text ", 512))},
	}

	for _, bench := range benchmarks {
		b.Run(bench.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(bench.data)))
			for b.Loop() {
				rawNeeds, err := scanCharDataSpanParse(bench.data, resolver)
				if err != nil {
					b.Fatalf("scanCharDataSpanParse error = %v", err)
				}
				if bench.name == "EntityHeavy" && !rawNeeds {
					b.Fatalf("scanCharDataSpanParse rawNeeds = false, want true")
				}
			}
		})
	}
}
