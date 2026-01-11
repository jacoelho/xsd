package xmltext

import (
	"bytes"
	"unicode/utf8"
)

type entityResolver struct {
	custom       map[string]string
	maxTokenSize int
}

var standardEntities = map[string]string{
	"lt":   "<",
	"gt":   ">",
	"amp":  "&",
	"apos": "'",
	"quot": "\"",
}

func (r *entityResolver) resolve(name string) (string, bool) {
	if value, ok := standardEntities[name]; ok {
		return value, true
	}
	if r == nil || r.custom == nil {
		return "", false
	}
	value, ok := r.custom[name]
	return value, ok
}

func unescapeInto(dst []byte, data []byte, resolver *entityResolver, maxTokenSize int) ([]byte, error) {
	for i := 0; i < len(data); i++ {
		if data[i] != '&' {
			dst = append(dst, data[i])
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
		i += consumed - 1
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
	name := string(ref)
	replacement, ok := resolver.resolve(name)
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
