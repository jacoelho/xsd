package xsdregex

import "github.com/jacoelho/xsd/internal/lex"

// XSD's name escapes use the XML name classes owned by internal/lex, built
// once from its range tables so matching remains a binary search over ranges.
var (
	xmlNameChars      = setFromLexRanges(lex.XMLNameCharRanges())
	xmlNameStartChars = setFromLexRanges(lex.XMLNameStartCharRanges())
)

func setFromLexRanges(in []lex.RuneRange) rangeSet {
	ranges := make([]runeRange, len(in))
	for i, r := range in {
		ranges[i] = runeRange{lo: r.Lo, hi: r.Hi}
	}
	return setFromRanges(ranges)
}
