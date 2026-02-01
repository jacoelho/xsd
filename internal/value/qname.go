package value

import (
	"bytes"
	"fmt"
	"slices"
	"unicode/utf8"
)

// NSResolver resolves QName prefixes to namespace URIs.
// The empty prefix uses the default namespace when present.
type NSResolver interface {
	ResolvePrefix(prefix []byte) ([]byte, bool)
}

// CanonicalQName resolves a lexical QName to canonical bytes (uri + 0 + local).
func CanonicalQName(value []byte, resolver NSResolver, dst []byte) ([]byte, error) {
	trimmed := TrimXMLWhitespace(value)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("invalid QName: empty string")
	}
	if containsXMLWhitespace(trimmed) {
		return nil, fmt.Errorf("invalid QName: contains whitespace")
	}
	prefix, local, hasPrefix, err := parseQName(trimmed)
	if err != nil {
		return nil, err
	}
	var ns []byte
	if hasPrefix {
		if resolver == nil {
			return nil, fmt.Errorf("prefix %s not found in namespace context", string(prefix))
		}
		resolved, ok := resolver.ResolvePrefix(prefix)
		if !ok {
			return nil, fmt.Errorf("prefix %s not found in namespace context", string(prefix))
		}
		ns = resolved
	} else if resolver != nil {
		if resolved, ok := resolver.ResolvePrefix(nil); ok {
			ns = resolved
		} else if resolved, ok := resolver.ResolvePrefix([]byte{}); ok {
			ns = resolved
		}
	}
	out := dst[:0]
	if cap(out) < len(ns)+1+len(local) {
		out = make([]byte, 0, len(ns)+1+len(local))
	}
	out = append(out, ns...)
	out = append(out, 0)
	out = append(out, local...)
	return out, nil
}

func parseQName(value []byte) ([]byte, []byte, bool, error) {
	if err := validateQName(value); err != nil {
		return nil, nil, false, err
	}
	colon := -1
	for i, b := range value {
		if b == ':' {
			colon = i
			break
		}
	}
	if colon == -1 {
		return nil, value, false, nil
	}
	return value[:colon], value[colon+1:], true, nil
}

func validateQName(value []byte) error {
	if len(value) == 0 {
		return fmt.Errorf("QName cannot be empty")
	}
	colon := -1
	for i, b := range value {
		if b == ':' {
			if colon != -1 {
				return fmt.Errorf("QName can have at most one colon")
			}
			colon = i
		}
	}
	if colon == -1 {
		if err := validateNCName(value); err != nil {
			return fmt.Errorf("invalid QName part '%s': %w", string(value), err)
		}
		return nil
	}
	if colon == 0 || colon == len(value)-1 {
		return fmt.Errorf("QName part cannot be empty")
	}
	prefix := value[:colon]
	local := value[colon+1:]
	if bytes.Equal(prefix, []byte("xmlns")) {
		return fmt.Errorf("QName cannot use reserved prefix 'xmlns'")
	}
	if err := validateNCName(prefix); err != nil {
		return fmt.Errorf("invalid QName part '%s': %w", string(prefix), err)
	}
	if err := validateNCName(local); err != nil {
		return fmt.Errorf("invalid QName part '%s': %w", string(local), err)
	}
	return nil
}

func validateNCName(value []byte) error {
	if len(value) == 0 {
		return fmt.Errorf("NCName cannot be empty")
	}
	for i := 0; i < len(value); {
		r, size := utf8.DecodeRune(value[i:])
		if r == utf8.RuneError && size == 1 {
			return fmt.Errorf("invalid NCName character")
		}
		if r == ':' {
			return fmt.Errorf("NCName cannot contain colons")
		}
		if i == 0 {
			if !isNameStartChar(r) {
				return fmt.Errorf("invalid NCName start character: %c", r)
			}
		} else if !isNameChar(r) {
			return fmt.Errorf("invalid NCName character: %c", r)
		}
		i += size
	}
	return nil
}

func isNameStartChar(r rune) bool {
	return r == ':' || r == '_' ||
		(r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= 0xC0 && r <= 0xD6) ||
		(r >= 0xD8 && r <= 0xF6) ||
		(r >= 0xF8 && r <= 0x2FF) ||
		(r >= 0x370 && r <= 0x37D) ||
		(r >= 0x37F && r <= 0x1FFF) ||
		(r >= 0x200C && r <= 0x200D) ||
		(r >= 0x2070 && r <= 0x218F) ||
		(r >= 0x2C00 && r <= 0x2FEF) ||
		(r >= 0x3001 && r <= 0xD7FF) ||
		(r >= 0xF900 && r <= 0xFDCF) ||
		(r >= 0xFDF0 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0xEFFFF)
}

func isNameChar(r rune) bool {
	return isNameStartChar(r) ||
		r == '-' || r == '.' ||
		(r >= '0' && r <= '9') ||
		r == 0xB7 ||
		(r >= 0x0300 && r <= 0x036F) ||
		(r >= 0x203F && r <= 0x2040)
}

func containsXMLWhitespace(value []byte) bool {
	return slices.ContainsFunc(value, isXMLWhitespace)
}
