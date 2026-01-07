package types

import (
	"strings"
	"unicode"
)

// WhiteSpace represents whitespace normalization
type WhiteSpace int

const (
	WhiteSpacePreserve WhiteSpace = iota
	WhiteSpaceReplace
	WhiteSpaceCollapse
)

type whiteSpaceNormalizer struct{}

var whiteSpaceReplacer = strings.NewReplacer("\t", " ", "\r", " ", "\n", " ")

func (n whiteSpaceNormalizer) Normalize(value string, typ Type) (string, error) {
	if typ == nil {
		return value, nil
	}
	return ApplyWhiteSpace(value, typ.WhiteSpace()), nil
}

// ApplyWhiteSpace applies whitespace normalization
func ApplyWhiteSpace(value string, ws WhiteSpace) string {
	switch ws {
	case WhiteSpacePreserve:
		return value
	case WhiteSpaceReplace:
		return replaceWhiteSpace(value)
	case WhiteSpaceCollapse:
		return collapseWhiteSpace(value)
	default:
		return value
	}
}

// NormalizeWhiteSpace applies whitespace normalization for simple types.
// Non-simple types are returned unchanged.
func NormalizeWhiteSpace(value string, typ Type) string {
	if typ == nil {
		return value
	}
	st, ok := typ.(SimpleTypeDefinition)
	if !ok {
		return value
	}
	return ApplyWhiteSpace(value, st.WhiteSpace())
}

func replaceWhiteSpace(value string) string {
	return whiteSpaceReplacer.Replace(value)
}

func collapseWhiteSpace(value string) string {
	value = replaceWhiteSpace(value)
	value = strings.TrimSpace(value)

	var result strings.Builder
	result.Grow(len(value))

	prevSpace := false
	for _, r := range value {
		if unicode.IsSpace(r) {
			if !prevSpace {
				result.WriteRune(' ')
				prevSpace = true
			}
		} else {
			result.WriteRune(r)
			prevSpace = false
		}
	}

	return result.String()
}
