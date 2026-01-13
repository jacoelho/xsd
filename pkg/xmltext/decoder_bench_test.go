package xmltext

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
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
	var buf TokenBuffer

	for b.Loop() {
		dec.Reset(bytes.NewReader(data))
		for {
			err := dec.ReadTokenInto(&tok, &buf)
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
	var buf TokenBuffer

	for b.Loop() {
		dec.Reset(bytes.NewReader(data), opts)
		for {
			err := dec.ReadTokenInto(&tok, &buf)
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
