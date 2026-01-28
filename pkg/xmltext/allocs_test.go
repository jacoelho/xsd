package xmltext

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

const allocsDecoderMax = 25

func TestAllocations_DecoderSimple(t *testing.T) {
	doc := []byte(`<?xml version="1.0"?>
<root>
  <child attr="value">text</child>
  <child2/>
</root>`)

	dec := NewDecoder(bytes.NewReader(doc))
	var tok Token
	tok.Reserve(TokenSizes{
		Attrs:     4,
		AttrName:  64,
		AttrValue: 128,
	})

	allocs := testing.AllocsPerRun(50, func() {
		dec.Reset(bytes.NewReader(doc))
		for {
			if err := dec.ReadTokenInto(&tok); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				t.Fatalf("ReadTokenInto: %v", err)
			}
		}
	})
	if allocs > allocsDecoderMax {
		t.Fatalf("allocs = %.2f, want <= %d", allocs, allocsDecoderMax)
	}
}
