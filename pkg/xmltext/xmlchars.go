package xmltext

import "unicode/utf8"

// isValidXMLChar reports whether r is a valid XML 1.0 character.
// Per XML 1.0 spec section 2.2, Char excludes most control codes.
func isValidXMLChar(r rune) bool {
	switch {
	case r == 0x9 || r == 0xA || r == 0xD:
		return true
	case r >= 0x20 && r <= 0xD7FF:
		return true
	case r >= 0xE000 && r <= 0xFFFD:
		return true
	case r >= 0x10000 && r <= 0x10FFFF:
		return true
	default:
		return false
	}
}

func validateXMLChars(data []byte) error {
	for len(data) > 0 {
		if data[0] < utf8.RuneSelf {
			if !isValidXMLChar(rune(data[0])) {
				return errInvalidChar
			}
			data = data[1:]
			continue
		}
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			return errInvalidChar
		}
		if !isValidXMLChar(r) {
			return errInvalidChar
		}
		data = data[size:]
	}
	return nil
}

func validateXMLText(data []byte, resolver *entityResolver) error {
	if resolver == nil {
		resolver = &entityResolver{}
	}
	for i := 0; i < len(data); {
		if data[i] == '&' {
			consumed, _, _, _, err := parseEntityRef(data, i, resolver)
			if err != nil {
				return err
			}
			i += consumed
			continue
		}
		if data[i] < utf8.RuneSelf {
			if !isValidXMLChar(rune(data[i])) {
				return errInvalidChar
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 {
			return errInvalidChar
		}
		if !isValidXMLChar(r) {
			return errInvalidChar
		}
		i += size
	}
	return nil
}

func isWhitespaceBytes(data []byte) bool {
	for _, b := range data {
		if !isWhitespace(b) {
			return false
		}
	}
	return true
}
