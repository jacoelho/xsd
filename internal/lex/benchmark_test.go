package lex

import "testing"

var xmlWhitespaceSink string

func BenchmarkXMLWhitespaceNormalizeReplaceASCII(b *testing.B) {
	in := "alpha\tbeta\ngamma\rdelta alpha\tbeta\ngamma\rdelta"
	for b.Loop() {
		xmlWhitespaceSink = ReplaceXMLWhitespace(in)
	}
}

func BenchmarkXMLWhitespaceNormalizeCollapseASCII(b *testing.B) {
	in := " \talpha  beta\ngamma\r delta \t alpha  beta\ngamma\r delta "
	for b.Loop() {
		xmlWhitespaceSink = CollapseXMLWhitespace(in)
	}
}

func BenchmarkXMLWhitespaceNormalizeReplaceNoop(b *testing.B) {
	in := "alpha beta gamma delta"
	for b.Loop() {
		xmlWhitespaceSink = ReplaceXMLWhitespace(in)
	}
}

func BenchmarkXMLWhitespaceNormalizeCollapseNoop(b *testing.B) {
	in := "alpha beta gamma delta"
	for b.Loop() {
		xmlWhitespaceSink = CollapseXMLWhitespace(in)
	}
}

func BenchmarkXMLWhitespaceNormalizeReplaceUnicode(b *testing.B) {
	in := "alpha\tβeta\ngamma\rδelta alpha\tβeta\ngamma\rδelta"
	for b.Loop() {
		xmlWhitespaceSink = ReplaceXMLWhitespace(in)
	}
}

func BenchmarkXMLWhitespaceAttributeUnicode(b *testing.B) {
	in := "alpha\tβeta\ngamma\rδelta alpha\tβeta\ngamma\rδelta"
	for b.Loop() {
		xmlWhitespaceSink = ReplaceXMLWhitespace(in)
	}
}
