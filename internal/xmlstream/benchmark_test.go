package xmlstream

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func BenchmarkParserReferences(b *testing.B) {
	for _, test := range []struct{ name, lexical, decoded string }{
		{name: "first", lexical: "&amp;x", decoded: "&x"},
		{name: "interior", lexical: "x&amp;y", decoded: "x&y"},
	} {
		b.Run(test.name, func(b *testing.B) {
			const repeats = 4096
			decoded := strings.Repeat(test.decoded, repeats)
			digest := parserBenchmarkDigest{hash: parserBenchmarkDigestOffset}
			for _, token := range []Token{
				{Kind: KindStart, Start: StartElement{Name: xml.Name{Local: "r"}}},
				{Kind: KindCharData, Data: []byte(decoded)},
				{Kind: KindEnd, End: EndElement{Name: xml.Name{Local: "r"}}},
			} {
				digest.addToken(token, nil, parserBenchmarkRawAttributes)
			}
			benchmarkParserDocument(b, "<r>"+strings.Repeat(test.lexical, repeats)+"</r>",
				newParserBenchmarkOracle(3, len(decoded)+1, digest.hash), Config{}, parserBenchmarkRawAttributes)
		})
	}
}

func BenchmarkParserLazyWideAttributes(b *testing.B) {
	benchmarkParserDocument(b, benchmarkParserWideAttributesDocument(), newParserBenchmarkOracle(258, 90_390, 0xe98b3c97c304f556), Config{LazyAttrValues: true}, parserBenchmarkRawAttributes)
}

func BenchmarkParserLazyWideAttributesMaterialized(b *testing.B) {
	benchmarkParserDocument(b, benchmarkParserWideAttributesDocument(), newParserBenchmarkOracle(258, 90_390, 0xe98b3c97c304f556), Config{LazyAttrValues: true}, parserBenchmarkMaterializedAttributes)
}

func benchmarkParserWideAttributesDocument() string {
	const attributeCount = 64
	var doc strings.Builder
	doc.WriteString("<root>")
	for elem := range 128 {
		fmt.Fprintf(&doc, "<e%d", elem)
		for attr := range attributeCount {
			fmt.Fprintf(&doc, ` a%d="value-%d-%d"`, attr, elem, attr)
		}
		doc.WriteString("/>")
	}
	doc.WriteString("</root>")
	return doc.String()
}

func BenchmarkParserCharacterData(b *testing.B) {
	text := `<root>` + strings.Repeat("abcdefgh", 8<<10) + `</root>`
	benchmarkParserDocument(b, text, newParserBenchmarkOracle(3, 65_540, 0x069139cfad00a11b), Config{}, parserBenchmarkRawAttributes)
}

func BenchmarkParserSmallLiteral(b *testing.B) {
	benchmarkParserDerivedDocument(b, "<root>literal text</root>")
}

func BenchmarkParserNewlineRuns(b *testing.B) {
	benchmarkParserDerivedDocument(b, "<root>line\n\nline\n\nline</root>")
}

func benchmarkParserDerivedDocument(b *testing.B, text string) {
	b.Helper()
	var parser parserTestHarness
	digest, err := digestParserBenchmarkDocument(&parser, text, Config{}, parserBenchmarkRawAttributes)
	if err != nil {
		b.Fatal(err)
	}
	sample, err := consumeParserBenchmarkDocument(&parser, text, Config{}, parserBenchmarkRawAttributes)
	if err != nil {
		b.Fatal(err)
	}
	benchmarkParserDocument(b, text, parserBenchmarkOracle{digest: digest, sample: sample}, Config{}, parserBenchmarkRawAttributes)
}

func BenchmarkParserMixedSmallTokens(b *testing.B) {
	text := `<root>` + strings.Repeat(`<e a="v">x</e>`, 4_000) + `</root>`
	benchmarkParserDocument(b, text, newParserBenchmarkOracle(12_002, 12_004, 0xf020b93837f5e1c), Config{LazyAttrValues: true}, parserBenchmarkRawAttributes)
}

func BenchmarkParserTinySelfClosing(b *testing.B) {
	digest := parserBenchmarkDigest{hash: parserBenchmarkDigestOffset}
	digest.addToken(Token{Kind: KindStart, Start: StartElement{Name: xml.Name{Local: "r"}}}, nil, parserBenchmarkRawAttributes)
	for range 4_000 {
		digest.addToken(Token{Kind: KindStart, Start: StartElement{Name: xml.Name{Local: "e"}}}, nil, parserBenchmarkRawAttributes)
		digest.addToken(Token{Kind: KindEnd, End: EndElement{Name: xml.Name{Local: "e"}}}, nil, parserBenchmarkRawAttributes)
	}
	digest.addToken(Token{Kind: KindEnd, End: EndElement{Name: xml.Name{Local: "r"}}}, nil, parserBenchmarkRawAttributes)
	benchmarkParserDocument(b, "<r>"+strings.Repeat("<e/>", 4_000)+"</r>",
		newParserBenchmarkOracle(8_002, 4_001, digest.hash), Config{}, parserBenchmarkRawAttributes)
}

func BenchmarkParserNameVocabulary(b *testing.B) {
	for _, shape := range []string{"ascii", "prefixed", "unicode", "unseen", "long-parts", "combinations"} {
		b.Run(shape, func(b *testing.B) {
			var doc strings.Builder
			digest := parserBenchmarkDigest{hash: parserBenchmarkDigestOffset}
			doc.WriteString("<r>")
			digest.addToken(Token{Kind: KindStart, Start: StartElement{Name: xml.Name{Local: "r"}}}, nil, parserBenchmarkRawAttributes)
			payload := 1
			for i := range 4096 {
				name := xml.Name{Local: fmt.Sprintf("n%d", i%32)}
				switch shape {
				case "prefixed":
					name.Space = "p"
				case "unicode":
					name.Local = fmt.Sprintf("名%d", i%32)
				case "unseen":
					name.Local = fmt.Sprintf("n%d", i%1024)
				case "long-parts":
					name.Space = "p" + strings.Repeat("a", 159)
					name.Local = "n" + strings.Repeat("b", 158) + string(rune('a'+i%8))
				case "combinations":
					name.Space = fmt.Sprintf("p%d", i/32%32)
				}
				doc.WriteByte('<')
				if name.Space != "" {
					doc.WriteString(name.Space)
					doc.WriteByte(':')
				}
				doc.WriteString(name.Local)
				doc.WriteString("/>")
				digest.addToken(Token{Kind: KindStart, Start: StartElement{Name: name}}, nil, parserBenchmarkRawAttributes)
				digest.addToken(Token{Kind: KindEnd, End: EndElement{Name: name}}, nil, parserBenchmarkRawAttributes)
				payload += len(name.Space) + len(name.Local)
			}
			doc.WriteString("</r>")
			digest.addToken(Token{Kind: KindEnd, End: EndElement{Name: xml.Name{Local: "r"}}}, nil, parserBenchmarkRawAttributes)
			benchmarkParserDocument(b, doc.String(), newParserBenchmarkOracle(8194, payload, digest.hash), Config{}, parserBenchmarkRawAttributes)
		})
	}
}

func BenchmarkParserCDATABufferBoundary(b *testing.B) {
	text := `<root><![CDATA[` + strings.Repeat("x", xmlInputBufferSize) + `]]></root>`
	benchmarkParserDocument(b, text, newParserBenchmarkOracle(4, 65_540, 0xfd95ace67cb7de45), Config{}, parserBenchmarkRawAttributes)
}

const (
	parserBenchmarkDigestOffset = uint64(14695981039346656037)
	parserBenchmarkDigestPrime  = uint64(1099511628211)
)

type parserBenchmarkDigest struct {
	hash   uint64
	tokens int
}

type parserBenchmarkSample struct {
	tokens  int
	payload int
}

type parserBenchmarkOracle struct {
	digest parserBenchmarkDigest
	sample parserBenchmarkSample
}

type parserBenchmarkAttributeMode uint8

const (
	parserBenchmarkRawAttributes parserBenchmarkAttributeMode = iota
	parserBenchmarkMaterializedAttributes
)

func newParserBenchmarkOracle(tokens, payload int, hash uint64) parserBenchmarkOracle {
	return parserBenchmarkOracle{
		digest: parserBenchmarkDigest{hash: hash, tokens: tokens},
		sample: parserBenchmarkSample{tokens: tokens, payload: payload},
	}
}

func (d *parserBenchmarkDigest) addToken(token Token, reader *Reader, attributeMode parserBenchmarkAttributeMode) {
	d.tokens++
	d.addUint64(uint64(token.Kind))
	d.addBool(token.TextKind == CharacterDataCDATA)
	d.addName(token.Start.Name)
	d.addUint64(uint64(len(token.Start.Attr)))
	for i := range token.Start.Attr {
		attr := &token.Start.Attr[i]
		d.addName(attr.Name)
		if raw, ok := attr.RawValue(); ok && attributeMode == parserBenchmarkRawAttributes {
			d.addBytes(raw)
			continue
		}
		d.addString(parserBenchmarkMaterializeAttr(reader, attr))
	}
	d.addName(token.End.Name)
	d.addBytes(token.Data)
	d.addBytes(token.Directive)
}

func (s *parserBenchmarkSample) addToken(token Token, reader *Reader, attributeMode parserBenchmarkAttributeMode) {
	s.tokens++
	s.payload += len(token.Data) + len(token.Directive) + len(token.Start.Name.Space) + len(token.Start.Name.Local)
	for i := range token.Start.Attr {
		if raw, ok := token.Start.Attr[i].RawValue(); ok && attributeMode == parserBenchmarkRawAttributes {
			s.payload += len(raw)
			continue
		}
		s.payload += len(parserBenchmarkMaterializeAttr(reader, &token.Start.Attr[i]))
	}
}

func parserBenchmarkMaterializeAttr(reader *Reader, attr *Attr) string {
	if reader == nil {
		return attr.Value
	}
	value, _ := reader.MaterializeValue(attr)
	return value
}

func (d *parserBenchmarkDigest) addName(name xml.Name) {
	d.addString(name.Space)
	d.addString(name.Local)
}

func (d *parserBenchmarkDigest) addBool(value bool) { //nolint:revive // Benchmark digest data is the value being hashed, not control flow.
	if value {
		d.addUint64(1)
		return
	}
	d.addUint64(0)
}

func (d *parserBenchmarkDigest) addString(value string) {
	d.addUint64(uint64(len(value)))
	for i := range len(value) {
		d.addByte(value[i])
	}
}

func (d *parserBenchmarkDigest) addBytes(value []byte) {
	d.addUint64(uint64(len(value)))
	for _, b := range value {
		d.addByte(b)
	}
}

func (d *parserBenchmarkDigest) addUint64(value uint64) {
	for range 8 {
		d.addByte(byte(value))
		value >>= 8
	}
}

func (d *parserBenchmarkDigest) addByte(value byte) {
	d.hash ^= uint64(value)
	d.hash *= parserBenchmarkDigestPrime
}

func benchmarkParserDocument(b *testing.B, text string, want parserBenchmarkOracle, config Config, attributeMode parserBenchmarkAttributeMode) {
	b.Helper()
	var parser parserTestHarness
	digest, err := digestParserBenchmarkDocument(&parser, text, config, attributeMode)
	if err != nil {
		b.Fatal(err)
	}
	if digest != want.digest {
		b.Fatalf("parsed semantic result = %d tokens/%016x digest, want %d/%016x", digest.tokens, digest.hash, want.digest.tokens, want.digest.hash)
	}
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		got, err := consumeParserBenchmarkDocument(&parser, text, config, attributeMode)
		if err != nil {
			b.Fatal(err)
		}
		if got != want.sample {
			b.Fatalf("parsed result = %d tokens/%d payload bytes, want %d/%d", got.tokens, got.payload, want.sample.tokens, want.sample.payload)
		}
	}
}

func digestParserBenchmarkDocument(parser *parserTestHarness, text string, config Config, attributeMode parserBenchmarkAttributeMode) (parserBenchmarkDigest, error) {
	if err := parser.reset(strings.NewReader(text), config); err != nil {
		return parserBenchmarkDigest{}, err
	}
	reader := &parser.reader
	got := parserBenchmarkDigest{hash: parserBenchmarkDigestOffset}
	for {
		token, err := parser.next()
		if IsOnlyEOF(err) {
			return got, nil
		}
		if err != nil {
			return parserBenchmarkDigest{}, err
		}
		got.addToken(token, reader, attributeMode)
	}
}

func consumeParserBenchmarkDocument(parser *parserTestHarness, text string, config Config, attributeMode parserBenchmarkAttributeMode) (parserBenchmarkSample, error) {
	if err := parser.reset(strings.NewReader(text), config); err != nil {
		return parserBenchmarkSample{}, err
	}
	reader := &parser.reader
	var got parserBenchmarkSample
	for {
		token, err := parser.next()
		if IsOnlyEOF(err) {
			return got, nil
		}
		if err != nil {
			return parserBenchmarkSample{}, err
		}
		got.addToken(token, reader, attributeMode)
	}
}

func BenchmarkParserSaturatedNameMisses(b *testing.B) {
	var parser parserTestHarness
	var seed strings.Builder
	seed.WriteString("<seedroot>")
	for i := range 512 {
		fmt.Fprintf(&seed, "<seed%d/>", i)
	}
	seed.WriteString("</seedroot>")
	if _, err := digestParserBenchmarkDocument(&parser, seed.String(), Config{}, parserBenchmarkRawAttributes); err != nil {
		b.Fatal(err)
	}
	var document strings.Builder
	document.WriteString("<coldroot>")
	want := parserBenchmarkDigest{hash: parserBenchmarkDigestOffset}
	want.addToken(Token{Kind: KindStart, Start: StartElement{Name: xml.Name{Local: "coldroot"}}}, nil, parserBenchmarkRawAttributes)
	payload := len("coldroot")
	for i := range 4096 {
		name := "cold" + strconv.Itoa(i%1024)
		document.WriteByte('<')
		document.WriteString(name)
		document.WriteString("/>")
		want.addToken(Token{Kind: KindStart, Start: StartElement{Name: xml.Name{Local: name}}}, nil, parserBenchmarkRawAttributes)
		want.addToken(Token{Kind: KindEnd, End: EndElement{Name: xml.Name{Local: name}}}, nil, parserBenchmarkRawAttributes)
		payload += len(name)
	}
	document.WriteString("</coldroot>")
	want.addToken(Token{Kind: KindEnd, End: EndElement{Name: xml.Name{Local: "coldroot"}}}, nil, parserBenchmarkRawAttributes)
	text := document.String()
	got, err := digestParserBenchmarkDocument(&parser, text, Config{}, parserBenchmarkRawAttributes)
	if err != nil || got != want {
		b.Fatalf("cold-name digest = %+v, %v; want %+v", got, err, want)
	}
	wantSample := parserBenchmarkSample{tokens: 8194, payload: payload}
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		got, err := consumeParserBenchmarkDocument(&parser, text, Config{}, parserBenchmarkRawAttributes)
		if err != nil || got != wantSample {
			b.Fatalf("cold-name sample = %+v, %v; want %+v", got, err, wantSample)
		}
	}
}
