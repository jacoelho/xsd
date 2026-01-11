package xmltext

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

var (
	benchOnce sync.Once
	benchData []byte
	benchErr  error
)

func loadBenchData(tb testing.TB) []byte {
	benchOnce.Do(func() {
		path := filepath.Join("..", "..", "testdata", "gml", "xsd", "gml.xsd")
		benchData, benchErr = os.ReadFile(path)
	})
	if benchErr != nil {
		tb.Fatalf("read benchmark data error = %v", benchErr)
	}
	return benchData
}

func BenchmarkXMLTextDecoder(b *testing.B) {
	data := loadBenchData(b)
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()

	dec := NewDecoder(bytes.NewReader(data))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec.Reset(bytes.NewReader(data))
		for {
			_, err := dec.ReadToken()
			if err == io.EOF {
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec := xml.NewDecoder(bytes.NewReader(data))
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatalf("encoding/xml error = %v", err)
			}
		}
	}
}
