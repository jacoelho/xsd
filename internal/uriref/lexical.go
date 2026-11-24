package uriref

import (
	"math"
	"strings"
)

type byteText interface {
	~string | ~[]byte
}

// analyze validates text as an RFC 2396 URI reference, as amended by RFC
// 2732, after logically applying XLink escaping. It returns the one
// decomposition of text into components that validation and Parts share.
func analyze[T byteText](text T) (scanned, layout, error) {
	s, err := scan(text)
	if err != nil {
		return scanned{}, layout{}, err
	}
	l := decompose(text)
	if !validLayout(text, l) {
		return scanned{}, layout{}, ErrInvalid
	}
	return s, l, nil
}

// scanned holds the lengths that XLink escaping gives a text.
type scanned struct {
	characters int
	escapedLen int
	// escapedFragmentStart is the escaped length of the text before its
	// '#', or escapedLen when it has none. A valid reference has at most one
	// '#', since a fragment cannot contain another.
	escapedFragmentStart int
}

func scan[T byteText](text T) (scanned, error) {
	s := scanned{escapedFragmentStart: -1}
	for i := 0; i < len(text); {
		token, ok := scanToken(text, i)
		if !ok || s.escapedLen > math.MaxInt-token.escapedLen {
			return scanned{}, ErrInvalid
		}
		if text[i] == '#' {
			s.escapedFragmentStart = s.escapedLen
		}
		s.characters += token.characters
		s.escapedLen += token.escapedLen
		i += token.width
	}
	if s.escapedFragmentStart < 0 {
		s.escapedFragmentStart = s.escapedLen
	}
	return s, nil
}

type scannedToken struct {
	width      int
	characters int
	escapedLen int
}

func scanToken[T byteText](text T, i int) (scannedToken, bool) {
	b := text[i]
	switch {
	case b == '%':
		if i+2 >= len(text) || !isHex(text[i+1]) || !isHex(text[i+2]) {
			return scannedToken{}, false
		}
		return scannedToken{width: 3, characters: 3, escapedLen: 3}, true
	case b < 0x80:
		growth := 1
		if mustEscapeASCII(b) {
			growth = 3
		}
		return scannedToken{width: 1, characters: 1, escapedLen: growth}, true
	default:
		n := utf8SequenceLen(text, i)
		return scannedToken{width: n, characters: 1, escapedLen: 3 * n}, n != 0
	}
}

// layout holds the component boundaries of a URI reference as byte offsets.
// The zero value is the layout of the empty reference.
type layout struct {
	schemeEnd      int // offset of the ':' ending the scheme
	authorityStart int // offset after "//"; the authority ends at pathStart
	pathStart      int
	pathEnd        int // offset of '?' or '#', or the text length
	fragmentStart  int // offset of '#', or the text length
	hasScheme      bool
	hasAuthority   bool
	hasQuery       bool
	hasFragment    bool
}

func decompose[T byteText](text T) layout {
	l := layout{fragmentStart: len(text)}
	if i := indexByte(text, '#'); i >= 0 {
		l.fragmentStart, l.hasFragment = i, true
	}
	l.pathEnd = l.fragmentStart
	if i := indexByteRange(text, '?', 0, l.fragmentStart); i >= 0 {
		l.pathEnd, l.hasQuery = i, true
	}
	if i := schemeSeparator(text, l.pathEnd); i >= 0 {
		l.schemeEnd, l.hasScheme = i, true
		l.pathStart = i + 1
	}
	if l.pathStart+2 <= l.pathEnd && text[l.pathStart] == '/' && text[l.pathStart+1] == '/' {
		l.authorityStart, l.hasAuthority = l.pathStart+2, true
		l.pathStart = l.pathEnd
		if i := indexByteRange(text, '/', l.authorityStart, l.pathEnd); i >= 0 {
			l.pathStart = i
		}
	}
	return l
}

func schemeSeparator[T byteText](text T, end int) int {
	for i := range end {
		switch text[i] {
		case ':':
			return i
		case '/':
			return -1
		}
	}
	return -1
}

func (l layout) parts(raw string) Parts {
	c := Parts{
		Path:         raw[l.pathStart:l.pathEnd],
		HasScheme:    l.hasScheme,
		HasAuthority: l.hasAuthority,
		HasQuery:     l.hasQuery,
		HasFragment:  l.hasFragment,
	}
	if l.hasScheme {
		c.Scheme = raw[:l.schemeEnd]
	}
	if l.hasAuthority {
		c.Authority = raw[l.authorityStart:l.pathStart]
	}
	if l.hasQuery {
		c.Query = raw[l.pathEnd+1 : l.fragmentStart]
	}
	if l.hasFragment {
		c.Fragment = raw[l.fragmentStart+1:]
	}
	return c
}

// bareEmptyAuthority reports whether the whole reference of the given length
// is "//" or "scheme://": an empty authority with no path, query or fragment.
func (l layout) bareEmptyAuthority(length int) bool {
	return l.hasAuthority && l.authorityStart == length && !l.hasQuery && !l.hasFragment
}

func validLayout[T byteText](text T, l layout) bool {
	if l.hasFragment && !validComponent(text, l.fragmentStart+1, len(text), &uricChars) {
		return false
	}
	if l.hasQuery && !validComponent(text, l.pathEnd+1, l.fragmentStart, &uricChars) {
		return false
	}
	if l.hasScheme && !validScheme(text, 0, l.schemeEnd) {
		return false
	}
	switch {
	case l.hasAuthority:
		return validAuthority(text, l.authorityStart, l.pathStart) && validComponent(text, l.pathStart, l.pathEnd, &pathChars)
	case l.hasScheme:
		return validSchemePath(text, l)
	default:
		// A rel_segment also excludes ':', but a ':' before the first '/'
		// makes decompose read a scheme, so every relative path is checked
		// with the path characters.
		return validComponent(text, l.pathStart, l.pathEnd, &pathChars)
	}
}

func validSchemePath[T byteText](text T, l layout) bool {
	switch {
	case l.pathStart == l.pathEnd:
		return l.hasQuery
	case text[l.pathStart] == '/':
		return validComponent(text, l.pathStart, l.pathEnd, &pathChars)
	default:
		return validOpaque(text, l.pathStart, l.pathEnd)
	}
}

// validOpaque checks an opaque_part: RFC 2732 adds the brackets to reserved
// but not to uric_no_slash, so they cannot start it.
func validOpaque[T byteText](text T, start, end int) bool {
	b := text[start]
	return b != '[' && b != ']' && validComponent(text, start, end, &uricChars)
}

func validScheme[T byteText](text T, start, end int) bool {
	if start == end || !isAlpha(text[start]) {
		return false
	}
	for i := start + 1; i < end; i++ {
		b := text[i]
		if !isAlpha(b) && !isDigit(b) && b != '+' && b != '-' && b != '.' {
			return false
		}
	}
	return true
}

// validAuthority checks a server or reg_name, where an IPv6reference is
// "[userinfo@]" "[" IPv6address "]" [":" port]. Any other bracket fails the
// reg_name, userinfo, IPv6 or port characters it lands in.
func validAuthority[T byteText](text T, start, end int) bool {
	left := indexByteRange(text, '[', start, end)
	if left < 0 {
		return validComponent(text, start, end, &regNameChars)
	}
	right := indexByteRange(text, ']', left+1, end)
	return right >= 0 &&
		validIPLiteralPrefix(text, start, left) &&
		validIPv6(text, left+1, right) &&
		validIPLiteralPort(text, right+1, end)
}

func validIPLiteralPrefix[T byteText](text T, start, left int) bool {
	at := lastIndexByteRange(text, '@', start, left)
	if at < 0 {
		return left == start
	}
	return at == left-1 && validComponent(text, start, at, &userInfoChars)
}

func validIPLiteralPort[T byteText](text T, start, end int) bool {
	if start == end {
		return true
	}
	if text[start] != ':' {
		return false
	}
	for i := start + 1; i < end; i++ {
		if !isDigit(text[i]) {
			return false
		}
	}
	return true
}

func validIPv6[T byteText](text T, start, end int) bool {
	if start == end {
		return false
	}
	compressed := indexDoubleColon(text, start, end)
	if compressed < 0 {
		groups, ok := countIPv6Groups(text, start, end, finalIPv6Side)
		return ok && groups == 8
	}
	if indexDoubleColon(text, compressed+2, end) >= 0 {
		return false
	}
	left, leftOK := countIPv6Groups(text, start, compressed, leadingIPv6Side)
	right, rightOK := countIPv6Groups(text, compressed+2, end, finalIPv6Side)
	return leftOK && rightOK && left+right < 8
}

type ipv6SideKind uint8

const (
	leadingIPv6Side ipv6SideKind = iota
	finalIPv6Side
)

func indexDoubleColon[T byteText](text T, start, end int) int {
	for i := start; i+1 < end; i++ {
		if text[i] == ':' && text[i+1] == ':' {
			return i
		}
	}
	return -1
}

func countIPv6Groups[T byteText](text T, start, end int, kind ipv6SideKind) (int, bool) {
	groups := 0
	for start < end {
		segmentEnd := indexByteRange(text, ':', start, end)
		if segmentEnd < 0 {
			segmentEnd = end
		}
		width, ok := ipv6SegmentWidth(text, start, segmentEnd, kind, segmentEnd == end)
		if !ok {
			return 0, false
		}
		groups += width
		start = segmentEnd + 1
	}
	return groups, true
}

func ipv6SegmentWidth[T byteText](text T, start, end int, kind ipv6SideKind, last bool) (int, bool) {
	if indexByteRange(text, '.', start, end) >= 0 {
		return 2, kind == finalIPv6Side && last && validIPv4(text, start, end)
	}
	return 1, validIPv6HexGroup(text, start, end)
}

func validIPv6HexGroup[T byteText](text T, start, end int) bool {
	if end-start < 1 || end-start > 4 {
		return false
	}
	for i := start; i < end; i++ {
		if !isHex(text[i]) {
			return false
		}
	}
	return true
}

func validIPv4[T byteText](text T, start, end int) bool {
	parts := 0
	for start < end {
		next, ok := scanIPv4Part(text, start, end)
		if !ok {
			return false
		}
		parts++
		start = next
	}
	return parts == 4
}

func scanIPv4Part[T byteText](text T, start, end int) (int, bool) {
	partStart := start
	value := 0
	for start < end && text[start] != '.' {
		if !isDigit(text[start]) || start-partStart == 3 {
			return 0, false
		}
		value = value*10 + int(text[start]-'0')
		start++
	}
	if start == partStart || value > 255 {
		return 0, false
	}
	if start == end {
		return end, true
	}
	if start+1 == end {
		return 0, false
	}
	return start + 1, true
}

// validComponent reports whether text[start:end] holds only escapes,
// characters that XLink escaping encodes, and bytes in allowed. It scans each
// token itself, so it needs no prior validation of text.
func validComponent[T byteText](text T, start, end int, allowed *asciiSet) bool {
	for i := start; i < end; {
		token, ok := scanToken(text, i)
		if !ok || token.width > end-i {
			return false
		}
		if b := text[i]; token.width == 1 && !mustEscapeASCII(b) && !allowed.contains(b) {
			return false
		}
		i += token.width
	}
	return true
}

// asciiSet is a set of ASCII bytes.
type asciiSet [2]uint64

func newASCIISet(chars string) asciiSet {
	var s asciiSet
	for i := range len(chars) {
		b := chars[i]
		s[b>>6] |= 1 << (b & 63)
	}
	return s
}

func (s *asciiSet) contains(b byte) bool {
	return b < 0x80 && s[b>>6]&(1<<(b&63)) != 0
}

// The component character sets of RFC 2396 Appendix A, with the brackets
// that RFC 2732 adds to reserved.
const unreservedChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.!~*'()"

var (
	uricChars     = newASCIISet(unreservedChars + ";/?:@&=+$,[]")
	pathChars     = newASCIISet(unreservedChars + "/;:@&=+$,")
	userInfoChars = newASCIISet(unreservedChars + ";:&=+$,")
	regNameChars  = newASCIISet(unreservedChars + "$,;:@&=+")
)

func mustEscapeASCII(b byte) bool {
	return b <= 0x20 || b == 0x7f || strings.ContainsRune("<>\"{}|\\^`", rune(b))
}

func isAlpha(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
func isDigit(b byte) bool { return b >= '0' && b <= '9' }
func isHex(b byte) bool   { return isDigit(b) || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F' }

func utf8SequenceLen[T byteText](text T, i int) int {
	width, secondMin, secondMax := utf8SequenceShape(text[i])
	if width == 0 || i+width > len(text) {
		return 0
	}
	if text[i+1] < secondMin || text[i+1] > secondMax {
		return 0
	}
	for j := i + 2; j < i+width; j++ {
		if !continuation(text[j]) {
			return 0
		}
	}
	return width
}

func utf8SequenceShape(b byte) (width int, secondMin, secondMax byte) {
	switch {
	case b >= 0xc2 && b <= 0xdf:
		return 2, 0x80, 0xbf
	case b == 0xe0:
		return 3, 0xa0, 0xbf
	case b >= 0xe1 && b <= 0xec, b >= 0xee && b <= 0xef:
		return 3, 0x80, 0xbf
	case b == 0xed:
		return 3, 0x80, 0x9f
	case b == 0xf0:
		return 4, 0x90, 0xbf
	case b >= 0xf1 && b <= 0xf3:
		return 4, 0x80, 0xbf
	case b == 0xf4:
		return 4, 0x80, 0x8f
	default:
		return 0, 0, 0
	}
}

func continuation(b byte) bool { return b >= 0x80 && b <= 0xbf }

func indexByte[T byteText](text T, want byte) int {
	return indexByteRange(text, want, 0, len(text))
}

func indexByteRange[T byteText](text T, want byte, start, end int) int {
	for i := start; i < end; i++ {
		if text[i] == want {
			return i
		}
	}
	return -1
}

func lastIndexByteRange[T byteText](text T, want byte, start, end int) int {
	for i := end - 1; i >= start; i-- {
		if text[i] == want {
			return i
		}
	}
	return -1
}

func writeEscape(out *strings.Builder, b byte) {
	const hex = "0123456789ABCDEF"
	out.WriteByte('%')
	out.WriteByte(hex[b>>4])
	out.WriteByte(hex[b&0x0f])
}
