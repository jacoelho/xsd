package xmlstream

import (
	"encoding/xml"
	"errors"
	"strings"
	"testing"
)

// TestReaderAttrValueMatrix pins the Reader.AttrValue contract across the
// reader configuration matrix. Expected values come from XML attribute-value
// decoding (reference expansion, line-end normalization), not from the parser.
func TestReaderAttrValueMatrix(t *testing.T) {
	t.Parallel()
	const doc = `<r xmlns="urn:d" xmlns:p="urn:p" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" ` +
		`plain="v" empty="" ref="x&amp;y&#65;" xml:space="preserve" xsi:type="p:t" split="abcdefghij"/>`
	reads := func(whole bool) []scriptedRead {
		if whole {
			return []scriptedRead{{data: []byte(doc)}}
		}
		// Split inside the attribute value and inside a character reference.
		cut := strings.Index(doc, "abcde") + 3
		ref := strings.Index(doc, "&amp;") + 2
		return []scriptedRead{{data: []byte(doc[:ref])}, {data: []byte(doc[ref:cut])}, {data: []byte(doc[cut:])}}
	}
	type want struct {
		name     xml.Name
		value    string
		ordinary bool
	}
	wants := []want{
		{xml.Name{Space: "xmlns", Local: "p"}, "urn:p", false},
		{xml.Name{Space: "xmlns", Local: "xsi"}, "http://www.w3.org/2001/XMLSchema-instance", false},
		{xml.Name{Local: "plain"}, "v", true},
		{xml.Name{Local: "empty"}, "", true},
		{xml.Name{Local: "ref"}, "x&yA", true},
		{xml.Name{Space: "xml", Local: "space"}, "preserve", false},
		{xml.Name{Space: "xsi", Local: "type"}, "p:t", true},
		{xml.Name{Local: "split"}, "abcdefghij", true},
	}
	for _, lazy := range []bool{true, false} {
		for _, skip := range []bool{true, false} {
			for _, whole := range []bool{true, false} {
				name := map[bool]string{true: "lazy", false: "eager"}[lazy] + "/" +
					map[bool]string{true: "skip", false: "noskip"}[skip] + "/" +
					map[bool]string{true: "whole", false: "split"}[whole]
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					var reader Reader
					t.Cleanup(reader.Detach)
					config := Config{LazyAttrValues: lazy, SkipOrdinaryAttrValues: skip}
					if err := reader.Reset(&scriptedReader{reads: reads(whole)}, config); err != nil {
						t.Fatal(err)
					}
					token := mustNextToken(t, &reader)
					// Namespace declarations and xmlns attributes precede expansion;
					// compare by lexical name while the token is current.
					if len(token.Start.Attr) != len(wants)+1 {
						t.Fatalf("attribute count = %d, want %d", len(token.Start.Attr), len(wants)+1)
					}
					attrs := token.Start.Attr
					offset := 0
					for i := range attrs {
						// The default namespace declaration xmlns="urn:d" is first.
						if i == 0 {
							value, err := reader.AttrValue(&attrs[0])
							if err != nil || value != "urn:d" {
								t.Fatalf("xmlns AttrValue() = %q, %v", value, err)
							}
							offset = 1
							continue
						}
						w := wants[i-offset]
						if attrs[i].Name != w.name {
							t.Fatalf("attribute %d name = %+v, want %+v", i, attrs[i].Name, w.name)
						}
						value, err := reader.AttrValue(&attrs[i])
						if skip && w.ordinary {
							raw, retained := attrs[i].RawValue()
							if !errors.Is(err, ErrAttrValueSkipped) || value != "" || retained || raw != nil {
								t.Fatalf("%v skipped AttrValue() = %q, %v; RawValue = %q, %v", w.name, value, err, raw, retained)
							}
							continue
						}
						if err != nil || value != w.value {
							t.Fatalf("%v AttrValue() = %q, %v, want %q", w.name, value, err, w.value)
						}
						again, err := reader.AttrValue(&attrs[i])
						if err != nil || again != value {
							t.Fatalf("%v repeated AttrValue() = %q, %v, want %q", w.name, again, err, value)
						}
					}
				})
			}
		}
	}
}

func TestNewAttrRoundTripsThroughAttrValue(t *testing.T) {
	t.Parallel()
	var reader Reader
	for _, value := range []string{"", "v", "x&y"} {
		attr := NewAttr(xml.Name{Local: "a"}, value)
		got, err := reader.AttrValue(&attr)
		if err != nil || got != value {
			t.Fatalf("AttrValue(NewAttr(%q)) = %q, %v", value, got, err)
		}
		if raw, ok := attr.RawValue(); ok || raw != nil {
			t.Fatalf("owned RawValue() = %q, %v, want none", raw, ok)
		}
	}
}

func TestAttrValueDoesNotAllocateAfterMaterialization(t *testing.T) {
	var reader Reader
	t.Cleanup(reader.Detach)
	if err := reader.Reset(strings.NewReader(`<r a="value"/>`), Config{LazyAttrValues: true}); err != nil {
		t.Fatal(err)
	}
	token := mustNextToken(t, &reader)
	attr := &token.Start.Attr[0]
	if _, err := reader.AttrValue(attr); err != nil {
		t.Fatal(err)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		if _, err := reader.AttrValue(attr); err != nil {
			t.Fatal(err)
		}
	}); allocs != 0 {
		t.Fatalf("AttrValue() allocations = %g, want 0", allocs)
	}
}
