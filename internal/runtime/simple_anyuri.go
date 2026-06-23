package runtime

import "errors"

func anyURILength(normalized string) (uint32, error) {
	if err := ValidateAnyURILexical(normalized); err != nil {
		return 0, err
	}
	return stringLength(normalized, "anyURI length exceeds uint32 limit")
}

// ValidateAnyURILexical validates raw as the supported XML Schema anyURI
// lexical space used by this runtime.
func ValidateAnyURILexical[T byteText](raw T) error {
	if len(raw) > 0 && (raw[0] == ':' || raw[len(raw)-1] == ':') {
		return errors.New("invalid anyURI")
	}
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '\\', '^':
			return errors.New("invalid anyURI")
		case '%':
			if i+2 >= len(raw) || !isHexDigit(raw[i+1]) || !isHexDigit(raw[i+2]) {
				return errors.New("invalid anyURI")
			}
			i += 2
		}
	}
	return nil
}

func isHexDigit(b byte) bool {
	return '0' <= b && b <= '9' || 'a' <= b && b <= 'f' || 'A' <= b && b <= 'F'
}
