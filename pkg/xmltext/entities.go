package xmltext

import (
	"bytes"
	"io"
	"unicode/utf8"
	"unsafe"
)

type entityResolver struct {
	custom       map[string]string
	customValid  map[string]bool
	maxTokenSize int
}

func newEntityResolver(custom map[string]string, maxTokenSize int) entityResolver {
	resolver := entityResolver{custom: custom, maxTokenSize: maxTokenSize}
	if len(custom) == 0 {
		return resolver
	}
	resolver.customValid = make(map[string]bool, len(custom))
	for name, replacement := range custom {
		resolver.customValid[name] = validateXMLChars([]byte(replacement)) == nil
	}
	return resolver
}

func unescapeInto(dst, data []byte, resolver *entityResolver, maxTokenSize int) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	wrote := 0
	outLen := 0
	short := false
	var runeBuf [utf8.UTFMax]byte

	writeBytes := func(p []byte) error {
		if len(p) == 0 {
			return nil
		}
		outLen += len(p)
		if maxTokenSize > 0 && outLen > maxTokenSize {
			return errTokenTooLarge
		}
		if short {
			return nil
		}
		if wrote+len(p) > len(dst) {
			avail := len(dst) - wrote
			if avail > 0 {
				wrote += copy(dst[wrote:], p[:avail])
			}
			short = true
			return nil
		}
		wrote += copy(dst[wrote:], p)
		return nil
	}

	writeString := func(s string) error {
		if s == "" {
			return nil
		}
		outLen += len(s)
		if maxTokenSize > 0 && outLen > maxTokenSize {
			return errTokenTooLarge
		}
		if short {
			return nil
		}
		if wrote+len(s) > len(dst) {
			avail := len(dst) - wrote
			if avail > 0 {
				wrote += copy(dst[wrote:], s[:avail])
			}
			short = true
			return nil
		}
		wrote += copy(dst[wrote:], s)
		return nil
	}

	for i := 0; i < len(data); {
		ampIdx := bytes.IndexByte(data[i:], '&')
		if ampIdx < 0 {
			if err := writeBytes(data[i:]); err != nil {
				return wrote, err
			}
			break
		}
		if ampIdx > 0 {
			if err := writeBytes(data[i : i+ampIdx]); err != nil {
				return wrote, err
			}
			i += ampIdx
		}
		consumed, replacement, r, isNumeric, err := parseEntityRef(data, i, resolver)
		if err != nil {
			return wrote, err
		}
		if isNumeric {
			n := utf8.EncodeRune(runeBuf[:], r)
			if err := writeBytes(runeBuf[:n]); err != nil {
				return wrote, err
			}
		} else {
			if err := writeString(replacement); err != nil {
				return wrote, err
			}
		}
		i += consumed
	}

	if short {
		return wrote, io.ErrShortBuffer
	}
	return wrote, nil
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
	if resolver.customValid != nil {
		if !resolver.customValid[name] {
			return 0, "", 0, false, errInvalidChar
		}
	} else if err := validateXMLChars([]byte(replacement)); err != nil {
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
	maxValue := uint64(utf8.MaxRune)
	baseValue := uint64(base)
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
		if value > maxValue/baseValue {
			return 0, errInvalidCharRef
		}
		value = value*baseValue + uint64(digit)
		if value > maxValue {
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
