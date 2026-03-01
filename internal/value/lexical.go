package value

import (
	"bytes"
	"fmt"
	"regexp"
	"unicode/utf8"
)

var (
	languagePattern  = regexp.MustCompile(`^[A-Za-z]{1,8}(-[A-Za-z0-9]{1,8})*$`)
	uriSchemePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*$`)
)

// ValidateToken validates xs:token lexical constraints.
func ValidateToken(value []byte) error {
	if len(value) == 0 {
		return nil
	}
	if value[0] == ' ' || value[len(value)-1] == ' ' {
		return fmt.Errorf("token cannot have leading or trailing whitespace")
	}
	prevSpace := false
	for _, b := range value {
		switch b {
		case ' ':
			if prevSpace {
				return fmt.Errorf("token cannot have consecutive spaces")
			}
			prevSpace = true
		case '\r', '\n', '\t':
			return fmt.Errorf("token cannot contain CR, LF, or Tab")
		default:
			prevSpace = false
		}
	}
	return nil
}

// ValidateName validates xs:Name lexical constraints.
func ValidateName(value []byte) error {
	if len(value) == 0 {
		return fmt.Errorf("name cannot be empty")
	}
	first, size := utf8.DecodeRune(value)
	if first == utf8.RuneError && size == 1 {
		return fmt.Errorf("invalid Name start character")
	}
	if !isNameStartChar(first) {
		return fmt.Errorf("invalid Name start character: %c", first)
	}
	for len(value) > size {
		value = value[size:]
		r, sz := utf8.DecodeRune(value)
		if r == utf8.RuneError && sz == 1 {
			return fmt.Errorf("invalid Name character")
		}
		if !isNameChar(r) {
			return fmt.Errorf("invalid Name character: %c", r)
		}
		size = sz
	}
	return nil
}

// ValidateNCName validates xs:NCName lexical constraints.
func ValidateNCName(value []byte) error {
	return validateNCName(value)
}

// ValidateNMTOKEN validates xs:NMTOKEN lexical constraints.
func ValidateNMTOKEN(value []byte) error {
	if len(value) == 0 {
		return fmt.Errorf("NMTOKEN cannot be empty")
	}
	for len(value) > 0 {
		r, size := utf8.DecodeRune(value)
		if r == utf8.RuneError && size == 1 {
			return fmt.Errorf("invalid NMTOKEN character")
		}
		if !isNameChar(r) {
			return fmt.Errorf("invalid NMTOKEN character: %c", r)
		}
		value = value[size:]
	}
	return nil
}

// ValidateLanguage validates xs:language lexical constraints.
func ValidateLanguage(value []byte) error {
	if !languagePattern.Match(value) {
		return fmt.Errorf("invalid language format")
	}
	return nil
}

// ValidateAnyURI validates xs:anyURI lexical constraints.
func ValidateAnyURI(value []byte) error {
	if len(value) == 0 {
		return nil
	}
	for _, b := range value {
		if b < 0x20 || b == 0x7f {
			return fmt.Errorf("anyURI contains control characters")
		}
		switch b {
		case '\\', '{', '}', '|', '^', '`':
			return fmt.Errorf("anyURI contains invalid characters")
		}
	}
	for i := 0; i < len(value); i++ {
		if value[i] != '%' {
			continue
		}
		if i+2 >= len(value) || !isHexDigit(value[i+1]) || !isHexDigit(value[i+2]) {
			return fmt.Errorf("anyURI contains invalid percent-encoding")
		}
		i += 2
	}
	if idx := bytes.IndexByte(value, ':'); idx >= 0 {
		delimiter := indexAny(value, "/?#")
		if delimiter == -1 || idx < delimiter {
			if idx == 0 {
				return fmt.Errorf("anyURI scheme cannot be empty")
			}
			if !uriSchemePattern.Match(value[:idx]) {
				return fmt.Errorf("anyURI has invalid scheme")
			}
		}
	}
	return nil
}

func isHexDigit(b byte) bool {
	switch {
	case b >= '0' && b <= '9':
		return true
	case b >= 'a' && b <= 'f':
		return true
	case b >= 'A' && b <= 'F':
		return true
	default:
		return false
	}
}

func indexAny(value []byte, chars string) int {
	for i, b := range value {
		for j := 0; j < len(chars); j++ {
			if b == chars[j] {
				return i
			}
		}
	}
	return -1
}
