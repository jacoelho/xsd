package xmltext

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

const (
	fuzzMaxItems   = 64
	fuzzMaxNameLen = 12
	fuzzMaxTextLen = 64
	fuzzMaxAttrLen = 32
	fuzzMaxAttrs   = 4
)

const (
	fuzzNameStartAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_"
	fuzzNameAlphabet      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789-."
	fuzzTextAlphabet      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var fuzzWhitespace = [...]string{" ", "\t", "\n", "\r\n"}

func FuzzDecoderGeneratedXML(f *testing.F) {
	f.Add([]byte("seed"))
	f.Add([]byte("<root/>"))
	f.Add([]byte("attr"))
	f.Add([]byte("text&chars"))
	f.Add([]byte("longer-seed-to-drive-more-structure"))
	f.Fuzz(func(t *testing.T, data []byte) {
		input := buildFuzzXML(data)
		tokens, err := readXMLTextTokens(input)
		encTokens, encErr := readEncodingXMLTokens(input)
		if (err == nil) != (encErr == nil) {
			t.Fatalf("decoder mismatch xmltext=%v encoding=%v input=%q", err, encErr, input)
		}
		if err != nil {
			return
		}
		if !tokensEqual(tokens, encTokens) {
			t.Fatalf("tokens mismatch input=%q\nxmltext=%v\nencoding=%v", input, tokens, encTokens)
		}
	})
}

func FuzzDecoderRawInput(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("<root/>"))
	f.Add([]byte("<root><child/></root>"))
	f.Add([]byte("<root><child>text</child></root>"))
	f.Add([]byte("<?xml version=\"1.0\"?><root/>"))
	f.Add([]byte("<root attr=\"1 &amp; 2\"/>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		dec := NewDecoder(bytes.NewReader(data))
		var tok Token
		maxTokens := min(len(data)+8, 10000)
		for range maxTokens {
			if err := dec.ReadTokenInto(&tok); err != nil {
				return
			}
		}
		t.Fatalf("decoder did not finish after %d tokens", maxTokens)
	})
}

func buildFuzzXML(data []byte) string {
	if len(data) == 0 {
		return "<root/>"
	}
	idx := 0
	var b strings.Builder
	b.Grow(256)
	b.WriteString("<root>")
	items := int(nextFuzzByte(data, &idx)%fuzzMaxItems) + 1
	for range items {
		if nextFuzzByte(data, &idx)%3 == 0 {
			appendElement(&b, data, &idx, 0)
		} else {
			appendText(&b, data, &idx, fuzzMaxTextLen)
		}
		if b.Len() > 48*1024 {
			break
		}
	}
	b.WriteString("</root>")
	return b.String()
}

func appendElement(b *strings.Builder, data []byte, idx *int, depth int) {
	name := buildName(data, idx, fuzzMaxNameLen)
	b.WriteByte('<')
	b.WriteString(name)
	attrCount := int(nextFuzzByte(data, idx) % fuzzMaxAttrs)
	seenAttrs := make(map[string]struct{}, attrCount)
	for range attrCount {
		appendWhitespace(b, data, idx)
		attrName := ensureUniqueName(buildName(data, idx, fuzzMaxNameLen), seenAttrs)
		b.WriteString(attrName)
		b.WriteString("=\"")
		appendAttrValue(b, data, idx, fuzzMaxAttrLen)
		b.WriteByte('"')
	}
	if attrCount == 0 && nextFuzzByte(data, idx)%2 == 0 {
		appendWhitespace(b, data, idx)
	}
	if nextFuzzByte(data, idx)%4 == 0 {
		b.WriteString("/>")
		return
	}
	b.WriteByte('>')
	if depth < 2 && nextFuzzByte(data, idx)%3 == 0 {
		appendElement(b, data, idx, depth+1)
	} else {
		appendText(b, data, idx, fuzzMaxTextLen)
	}
	b.WriteString("</")
	b.WriteString(name)
	b.WriteByte('>')
}

func ensureUniqueName(base string, seen map[string]struct{}) string {
	if _, exists := seen[base]; !exists {
		seen[base] = struct{}{}
		return base
	}
	for i := 1; ; i++ {
		name := base + "_" + strconv.Itoa(i)
		if _, exists := seen[name]; !exists {
			seen[name] = struct{}{}
			return name
		}
	}
}

func buildName(data []byte, idx *int, maxLen int) string {
	length := int(nextFuzzByte(data, idx)%byte(maxLen-1)) + 1
	out := make([]byte, 0, length)
	first := fuzzNameStartAlphabet[int(nextFuzzByte(data, idx))%len(fuzzNameStartAlphabet)]
	out = append(out, first)
	for i := 1; i < length; i++ {
		out = append(out, fuzzNameAlphabet[int(nextFuzzByte(data, idx))%len(fuzzNameAlphabet)])
	}
	return string(out)
}

func appendText(b *strings.Builder, data []byte, idx *int, maxLen int) {
	length := int(nextFuzzByte(data, idx) % byte(maxLen))
	for range length {
		switch nextFuzzByte(data, idx) % 12 {
		case 0:
			b.WriteString("&amp;")
		case 1:
			b.WriteString("&lt;")
		case 2:
			b.WriteString("&gt;")
		case 3:
			b.WriteByte(' ')
		case 4:
			b.WriteByte('\n')
		case 5:
			b.WriteByte('\t')
		default:
			b.WriteByte(fuzzTextAlphabet[int(nextFuzzByte(data, idx))%len(fuzzTextAlphabet)])
		}
	}
}

func appendAttrValue(b *strings.Builder, data []byte, idx *int, maxLen int) {
	length := int(nextFuzzByte(data, idx) % byte(maxLen))
	for range length {
		switch nextFuzzByte(data, idx) % 10 {
		case 0:
			b.WriteString("&amp;")
		case 1:
			b.WriteString("&lt;")
		case 2:
			b.WriteString("&gt;")
		case 3:
			b.WriteString("&quot;")
		case 4:
			b.WriteByte(' ')
		default:
			b.WriteByte(fuzzTextAlphabet[int(nextFuzzByte(data, idx))%len(fuzzTextAlphabet)])
		}
	}
}

func appendWhitespace(b *strings.Builder, data []byte, idx *int) {
	b.WriteString(fuzzWhitespace[int(nextFuzzByte(data, idx))%len(fuzzWhitespace)])
}

func nextFuzzByte(data []byte, idx *int) byte {
	if len(data) == 0 {
		return 0
	}
	b := data[*idx]
	*idx++
	if *idx >= len(data) {
		*idx = 0
	}
	return b
}
