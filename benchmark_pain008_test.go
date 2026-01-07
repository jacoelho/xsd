package xsd_test

import (
	"bytes"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
)

var (
	pain008SchemaOnce     sync.Once
	pain008SchemaInstance *xsd.Schema
	pain008SchemaErr      error
)

func loadPain008Schema(tb testing.TB) *xsd.Schema {
	tb.Helper()

	pain008SchemaOnce.Do(func() {
		fsys := fstest.MapFS{
			"pain.008.001.02.xsd": &fstest.MapFile{Data: []byte(pain008Schema)},
		}

		pain008SchemaInstance, pain008SchemaErr = xsd.Load(fsys, "pain.008.001.02.xsd")
	})

	if pain008SchemaErr != nil {
		tb.Fatalf("load pain008 schema: %v", pain008SchemaErr)
	}

	return pain008SchemaInstance
}

func BenchmarkPain008Validate(b *testing.B) {
	schema := loadPain008Schema(b)
	xmlBytes := []byte(pain008XML)

	b.ReportAllocs()
	b.SetBytes(int64(len(xmlBytes)))

	reader := bytes.NewReader(nil)
	for b.Loop() {
		reader.Reset(xmlBytes)
		if err := schema.Validate(reader); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPain008Load(b *testing.B) {
	schemaBytes := []byte(pain008Schema)

	b.ReportAllocs()

	for b.Loop() {
		fsys := fstest.MapFS{
			"pain.008.001.02.xsd": &fstest.MapFile{Data: schemaBytes},
		}
		if _, err := xsd.Load(fsys, "pain.008.001.02.xsd"); err != nil {
			b.Fatal(err)
		}
	}
}
