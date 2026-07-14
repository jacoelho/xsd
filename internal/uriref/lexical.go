package uriref

import "strings"

type byteText interface {
	~string | ~[]byte
}

func scan[T byteText](text T) (characters, escapedLen int, err error) {
	for i := 0; i < len(text); {
		b := text[i]
		switch {
		case b == '%':
			if i+2 >= len(text) || !isHex(text[i+1]) || !isHex(text[i+2]) {
				return 0, 0, ErrInvalid
			}
			characters += 3
			escapedLen += 3
			i += 3
		case b < 0x80:
			characters++
			growth := 1
			if mustEscapeASCII(b) {
				growth = 3
			}
			if escapedLen > int(^uint(0)>>1)-growth {
				return 0, 0, ErrInvalid
			}
			escapedLen += growth
			i++
		default:
			n := utf8SequenceLen(text, i)
			if n == 0 || escapedLen > int(^uint(0)>>1)-3*n {
				return 0, 0, ErrInvalid
			}
			characters++
			escapedLen += 3 * n
			i += n
		}
	}
	return characters, escapedLen, nil
}

func validReference[T byteText](text T) bool {
	raw := text
	fragment := len(raw)
	if i := indexByte(raw, '#'); i >= 0 {
		fragment = i
		if !validURIC(raw, i+1, len(raw)) {
			return false
		}
	}
	query := fragment
	if i := indexByteRange(raw, '?', 0, fragment); i >= 0 {
		query = i
		if !validURIC(raw, i+1, fragment) {
			return false
		}
	}
	mainEnd := query
	colon := indexByteRange(raw, ':', 0, mainEnd)
	slash := indexByteRange(raw, '/', 0, mainEnd)
	hasScheme := colon >= 0 && (slash < 0 || colon < slash)
	start := 0
	if hasScheme {
		if !validScheme(raw, 0, colon) {
			return false
		}
		start = colon + 1
	}
	if start+2 <= mainEnd && raw[start] == '/' && raw[start+1] == '/' {
		authorityStart := start + 2
		authorityEnd := mainEnd
		if i := indexByteRange(raw, '/', authorityStart, mainEnd); i >= 0 {
			authorityEnd = i
		}
		// The XSD 1.0 W3C oracle treats a bare empty authority ("//") as
		// invalid. Empty authority remains valid when followed by a path,
		// query, or fragment, including forms such as "///" and "//?q".
		if authorityStart == authorityEnd && authorityEnd == mainEnd && query == fragment && fragment == len(raw) {
			return false
		}
		if !validAuthority(raw, authorityStart, authorityEnd) {
			return false
		}
		return validPath(raw, authorityEnd, mainEnd)
	}
	pathStart := start
	if hasScheme {
		switch {
		case pathStart == mainEnd:
			return query < fragment
		case raw[pathStart] == '/':
			return validPath(raw, pathStart, mainEnd)
		default:
			return validOpaque(raw, pathStart, mainEnd)
		}
	}
	if pathStart == mainEnd {
		return true
	}
	if raw[pathStart] == '/' {
		return validPath(raw, pathStart, mainEnd)
	}
	firstEnd := mainEnd
	if i := indexByteRange(raw, '/', pathStart, mainEnd); i >= 0 {
		firstEnd = i
	}
	return validRelativeSegment(raw, pathStart, firstEnd) && validPath(raw, firstEnd, mainEnd)
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

func validAuthority[T byteText](text T, start, end int) bool {
	left := indexByteRange(text, '[', start, end)
	right := indexByteRange(text, ']', start, end)
	if left >= 0 || right >= 0 {
		if left < 0 || right < 0 || right < left || indexByteRange(text, '[', left+1, end) >= 0 || indexByteRange(text, ']', right+1, end) >= 0 {
			return false
		}
		at := lastIndexByteRange(text, '@', start, left)
		if at >= 0 {
			if at != left-1 || !validUserInfo(text, start, at) {
				return false
			}
		} else if left != start {
			return false
		}
		if !validIPv6(text, left+1, right) {
			return false
		}
		if right+1 == end {
			return true
		}
		if text[right+1] != ':' {
			return false
		}
		for i := right + 2; i < end; i++ {
			if !isDigit(text[i]) {
				return false
			}
		}
		return true
	}
	for i := start; i < end; {
		if next, ok := escapedToken(text, i); ok {
			i = next
			continue
		}
		b := text[i]
		if !isUnreserved(b) && !strings.ContainsRune("$,;:@&=+", rune(b)) {
			return false
		}
		i++
	}
	return true
}

func validIPv6[T byteText](text T, start, end int) bool {
	if start == end {
		return false
	}
	groups := 0
	compressed := false
	i := start
	if text[i] == ':' {
		if i+1 >= end || text[i+1] != ':' {
			return false
		}
		compressed = true
		i += 2
	}
	for i < end {
		segmentStart := i
		for i < end && text[i] != ':' {
			i++
		}
		if indexByteRange(text, '.', segmentStart, i) >= 0 {
			if i != end || !validIPv4(text, segmentStart, i) {
				return false
			}
			groups += 2
			break
		}
		if i-segmentStart < 1 || i-segmentStart > 4 {
			return false
		}
		for j := segmentStart; j < i; j++ {
			if !isHex(text[j]) {
				return false
			}
		}
		groups++
		if groups > 8 || i == end {
			break
		}
		i++
		if i < end && text[i] == ':' {
			if compressed {
				return false
			}
			compressed = true
			i++
			if i == end {
				break
			}
		} else if i == end {
			return false
		}
	}
	return groups == 8 || compressed && groups < 8
}

func validIPv4[T byteText](text T, start, end int) bool {
	parts := 0
	for start < end {
		partStart := start
		value := 0
		for start < end && text[start] != '.' {
			if !isDigit(text[start]) || start-partStart == 3 {
				return false
			}
			value = value*10 + int(text[start]-'0')
			start++
		}
		if start == partStart || value > 255 {
			return false
		}
		parts++
		if start < end {
			start++
			if start == end {
				return false
			}
		}
	}
	return parts == 4
}

func validUserInfo[T byteText](text T, start, end int) bool {
	for i := start; i < end; {
		if next, ok := escapedToken(text, i); ok {
			i = next
			continue
		}
		b := text[i]
		if !isUnreserved(b) && !strings.ContainsRune(";:&=+$,", rune(b)) {
			return false
		}
		i++
	}
	return true
}

func validRelativeSegment[T byteText](text T, start, end int) bool {
	if start == end {
		return false
	}
	for i := start; i < end; {
		if next, ok := escapedToken(text, i); ok {
			i = next
			continue
		}
		b := text[i]
		if !isUnreserved(b) && !strings.ContainsRune(";@&=+$,", rune(b)) {
			return false
		}
		i++
	}
	return true
}

func validPath[T byteText](text T, start, end int) bool {
	for i := start; i < end; {
		if next, ok := escapedToken(text, i); ok {
			i = next
			continue
		}
		b := text[i]
		if b != '/' && b != ';' && !isUnreserved(b) && !strings.ContainsRune(":@&=+$,", rune(b)) {
			return false
		}
		i++
	}
	return true
}

func validOpaque[T byteText](text T, start, end int) bool {
	for i := start; i < end; {
		if next, ok := escapedToken(text, i); ok {
			i = next
			continue
		}
		b := text[i]
		if i == start && (b == '/' || b == '[' || b == ']') {
			return false
		}
		if !isURIC(b) {
			return false
		}
		i++
	}
	return true
}

func validURIC[T byteText](text T, start, end int) bool {
	for i := start; i < end; {
		if next, ok := escapedToken(text, i); ok {
			i = next
			continue
		}
		if !isURIC(text[i]) {
			return false
		}
		i++
	}
	return true
}

func escapedToken[T byteText](text T, i int) (int, bool) {
	b := text[i]
	switch {
	case b == '%':
		return i + 3, true
	case b >= 0x80:
		return i + utf8SequenceLen(text, i), true
	case mustEscapeASCII(b):
		return i + 1, true
	default:
		return i, false
	}
}

func isURIC(b byte) bool {
	return isUnreserved(b) || strings.ContainsRune(";/?:@&=+$,[]", rune(b))
}

func isUnreserved(b byte) bool {
	return isAlpha(b) || isDigit(b) || strings.ContainsRune("-_.!~*'()", rune(b))
}

func mustEscapeASCII(b byte) bool {
	return b <= 0x20 || b == 0x7f || strings.ContainsRune("<>\"{}|\\^`", rune(b))
}

func isAlpha(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
func isDigit(b byte) bool { return b >= '0' && b <= '9' }
func isHex(b byte) bool   { return isDigit(b) || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F' }

//nolint:cyclop // The explicit branch table is the UTF-8 validity state machine.
func utf8SequenceLen[T byteText](text T, i int) int {
	b := text[i]
	switch {
	case b >= 0xc2 && b <= 0xdf:
		if i+1 < len(text) && continuation(text[i+1]) {
			return 2
		}
	case b == 0xe0:
		if i+2 < len(text) && text[i+1] >= 0xa0 && text[i+1] <= 0xbf && continuation(text[i+2]) {
			return 3
		}
	case b >= 0xe1 && b <= 0xec || b >= 0xee && b <= 0xef:
		if i+2 < len(text) && continuation(text[i+1]) && continuation(text[i+2]) {
			return 3
		}
	case b == 0xed:
		if i+2 < len(text) && text[i+1] >= 0x80 && text[i+1] <= 0x9f && continuation(text[i+2]) {
			return 3
		}
	case b == 0xf0:
		if i+3 < len(text) && text[i+1] >= 0x90 && text[i+1] <= 0xbf && continuation(text[i+2]) && continuation(text[i+3]) {
			return 4
		}
	case b >= 0xf1 && b <= 0xf3:
		if i+3 < len(text) && continuation(text[i+1]) && continuation(text[i+2]) && continuation(text[i+3]) {
			return 4
		}
	case b == 0xf4:
		if i+3 < len(text) && text[i+1] >= 0x80 && text[i+1] <= 0x8f && continuation(text[i+2]) && continuation(text[i+3]) {
			return 4
		}
	}
	return 0
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
