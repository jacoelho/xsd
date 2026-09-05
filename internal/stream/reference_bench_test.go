package stream

import (
	"encoding/xml"
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
			var values Cache
			digest := parserBenchmarkDigest{hash: parserBenchmarkDigestOffset}
			for _, token := range []Token{
				{Kind: KindStart, Start: StartElement{Name: xml.Name{Local: "r"}}},
				{Kind: KindCharData, Data: []byte(decoded)},
				{Kind: KindEnd, End: EndElement{Name: xml.Name{Local: "r"}}},
			} {
				digest.addToken(token, &values, parserBenchmarkRawAttributes)
			}
			benchmarkParserDocument(b, "<r>"+strings.Repeat(test.lexical, repeats)+"</r>",
				newParserBenchmarkOracle(3, len(decoded)+1, digest.hash), Config{}, parserBenchmarkRawAttributes)
		})
	}
}
