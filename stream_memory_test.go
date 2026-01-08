package xsd_test

import (
	"io"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
)

var (
	streamRootStart = []byte(`<root xmlns="urn:test">`)
	streamItem      = []byte(`<item>1</item>`)
	streamRootEnd   = []byte(`</root>`)
)

type itemStream struct {
	remaining int
	state     int
	buf       []byte
	offset    int
}

const (
	streamStateStart = iota
	streamStateItems
	streamStateEnd
	streamStateDone
)

func newItemStream(count int) io.Reader {
	return &itemStream{
		remaining: count,
		state:     streamStateStart,
	}
}

func (s *itemStream) Read(p []byte) (int, error) {
	if s.state == streamStateDone {
		return 0, io.EOF
	}
	if len(p) == 0 {
		return 0, nil
	}

	if s.offset >= len(s.buf) {
		switch s.state {
		case streamStateStart:
			s.buf = streamRootStart
		case streamStateItems:
			if s.remaining == 0 {
				s.state = streamStateEnd
				s.buf = streamRootEnd
			} else {
				s.buf = streamItem
				s.remaining--
			}
		case streamStateEnd:
			s.state = streamStateDone
			return 0, io.EOF
		}
		s.offset = 0
	}

	n := copy(p, s.buf[s.offset:])
	s.offset += n
	if s.offset >= len(s.buf) {
		switch s.state {
		case streamStateStart:
			s.state = streamStateItems
		case streamStateEnd:
			s.state = streamStateDone
		}
	}
	return n, nil
}

func TestStreamValidatorConstantMemory(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" type="xs:int" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	fsys := fstest.MapFS{
		"stream.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}
	schema, err := xsd.Load(fsys, "stream.xsd")
	if err != nil {
		t.Fatalf("Load schema: %v", err)
	}

	runStreamValidation(t, schema, 5)
	runtime.GC()

	heap10 := measureStreamHeapDelta(t, schema, 10)
	heap1000 := measureStreamHeapDelta(t, schema, 1000)

	const maxDelta = 512 * 1024
	if heap1000 > heap10+maxDelta {
		t.Fatalf("heap usage grew: 10 items=%d bytes, 1000 items=%d bytes (delta=%d)",
			heap10, heap1000, heap1000-heap10)
	}
}

func runStreamValidation(t *testing.T, schema *xsd.Schema, count int) {
	t.Helper()

	err := schema.Validate(newItemStream(count))
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}
}

func measureStreamHeapDelta(t *testing.T, schema *xsd.Schema, count int) uint64 {
	t.Helper()

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	runStreamValidation(t, schema, count)

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	if after.HeapAlloc < before.HeapAlloc {
		return 0
	}
	return after.HeapAlloc - before.HeapAlloc
}
