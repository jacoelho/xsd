package xmlstream

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

const allocsReaderMax = 20

func TestAllocations_ReaderSimple(t *testing.T) {
	doc := []byte(`<?xml version="1.0"?>
<root xmlns="urn:test">
  <child attr="value">text</child>
  <child2/>
</root>`)

	reader, err := NewReader(bytes.NewReader(doc))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	allocs := testing.AllocsPerRun(50, func() {
		if err := reader.Reset(bytes.NewReader(doc)); err != nil {
			t.Fatalf("Reset: %v", err)
		}
		for {
			_, err := reader.NextResolved()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				t.Fatalf("NextResolved: %v", err)
			}
		}
	})
	if allocs > allocsReaderMax {
		t.Fatalf("allocs = %.2f, want <= %d", allocs, allocsReaderMax)
	}
}
