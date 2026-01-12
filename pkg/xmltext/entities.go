package xmltext

import (
	"bytes"
	"unicode/utf8"
	"unsafe"
)

type entityResolver struct {
	custom       map[string]string
	maxTokenSize int
}

func unescapeInto(dst []byte, data []byte, resolver *entityResolver, maxTokenSize int) ([]byte, error) {
	required := len(dst) + len(data)
	if cap(dst) < required {
		next := make([]byte, len(dst), required)
		copy(next, dst)
		dst = next
	}
	for i := 0; i < len(data); {
		ampIdx := bytes.IndexByte(data[i:], '&')
		if ampIdx < 0 {
			dst = append(dst, data[i:]...)
			if maxTokenSize > 0 && len(dst) > maxTokenSize {
				return nil, errTokenTooLarge
			}
			return dst, nil
		}
		if ampIdx > 0 {
			dst = append(dst, data[i:i+ampIdx]...)
			if maxTokenSize > 0 && len(dst) > maxTokenSize {
				return nil, errTokenTooLarge
			}
			i += ampIdx
		}
		if i >= len(data) || data[i] != '&' {
			continue
		}
		consumed, replacement, r, isNumeric, err := parseEntityRef(data, i, resolver)
		if err != nil {
			return nil, err
		}
		if isNumeric {
			dst = utf8.AppendRune(dst, r)
		} else {
			dst = append(dst, replacement...)
		}
		if maxTokenSize > 0 && len(dst) > maxTokenSize {
			return nil, errTokenTooLarge
		}
		i += consumed
	}
	return dst, nil
}

func parseEntityRef(data []byte, start int, resolver *entityResolver) (int, string, rune, bool, error) {
	if start+1 >= len(data) {
		return 0, "", 0, false, errInvalidEntity
	}
	semi := bytes.IndexByte(data[start+1:], ';')
	if semi < 0 {
		return 0, "", 0, false, errInvalidEntity
	}
	semi += start + 1
	if semi == start+1 {
		return 0, "", 0, false, errInvalidEntity
	}
	ref := data[start+1 : semi]
	if ref[0] == '#' {
		r, err := parseNumericEntity(ref)
		if err != nil {
			return 0, "", 0, false, err
		}
		return semi - start + 1, "", r, true, nil
	}
	if replacement, ok := resolveStandardEntity(ref); ok {
		return semi - start + 1, replacement, 0, false, nil
	}
	if resolver == nil || len(resolver.custom) == 0 {
		return 0, "", 0, false, errInvalidEntity
	}
	name := unsafe.String(unsafe.SliceData(ref), len(ref))
	replacement, ok := resolver.custom[name]
	if !ok {
		return 0, "", 0, false, errInvalidEntity
	}
	if err := validateXMLChars([]byte(replacement)); err != nil {
		return 0, "", 0, false, err
	}
	return semi - start + 1, replacement, 0, false, nil
}

func parseNumericEntity(ref []byte) (rune, error) {
	if len(ref) < 2 {
		return 0, errInvalidCharRef
	}
	base := 10
	start := 1
	if ref[1] == 'x' || ref[1] == 'X' {
		base = 16
		start = 2
	}
	if start >= len(ref) {
		return 0, errInvalidCharRef
	}
	var value uint64
	for i := start; i < len(ref); i++ {
		b := ref[i]
		var digit byte
		switch {
		case b >= '0' && b <= '9':
			digit = b - '0'
		case base == 16 && b >= 'a' && b <= 'f':
			digit = b - 'a' + 10
		case base == 16 && b >= 'A' && b <= 'F':
			digit = b - 'A' + 10
		default:
			return 0, errInvalidCharRef
		}
		value = value*uint64(base) + uint64(digit)
		if value > utf8.MaxRune {
			return 0, errInvalidCharRef
		}
	}
	r := rune(value)
	if r == 0 || r > utf8.MaxRune || (r >= 0xD800 && r <= 0xDFFF) {
		return 0, errInvalidCharRef
	}
	if !isValidXMLChar(r) {
		return 0, errInvalidCharRef
	}
	return r, nil
}

func resolveStandardEntity(ref []byte) (string, bool) {
	switch len(ref) {
	case 2:
		if ref[0] == 'l' && ref[1] == 't' {
			return "<", true
		}
		if ref[0] == 'g' && ref[1] == 't' {
			return ">", true
		}
	case 3:
		if ref[0] == 'a' && ref[1] == 'm' && ref[2] == 'p' {
			return "&", true
		}
	case 4:
		if ref[0] == 'a' && ref[1] == 'p' && ref[2] == 'o' && ref[3] == 's' {
			return "'", true
		}
		if ref[0] == 'q' && ref[1] == 'u' && ref[2] == 'o' && ref[3] == 't' {
			return "\"", true
		}
	}
	return "", false
}
