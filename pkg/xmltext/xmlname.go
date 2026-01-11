package xmltext

import (
	"unicode"
	"unicode/utf8"
)

func isNameStartByte(b byte) bool {
	switch {
	case b >= 'A' && b <= 'Z':
		return true
	case b >= 'a' && b <= 'z':
		return true
	case b == '_' || b == ':':
		return true
	default:
		return false
	}
}

func isNameByte(b byte) bool {
	if isNameStartByte(b) {
		return true
	}
	switch {
	case b >= '0' && b <= '9':
		return true
	case b == '-' || b == '.':
		return true
	default:
		return false
	}
}

func isNameStartRune(r rune) bool {
	if r < utf8.RuneSelf {
		return isNameStartByte(byte(r))
	}
	return unicode.Is(nameStartTable, r)
}

func isNameRune(r rune) bool {
	if r < utf8.RuneSelf {
		return isNameByte(byte(r))
	}
	return unicode.Is(nameStartTable, r) || unicode.Is(nameCharTable, r)
}
