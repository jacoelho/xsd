package types

import "strings"

// WhiteSpace represents whitespace normalization
type WhiteSpace int

const (
	WhiteSpacePreserve WhiteSpace = iota
	WhiteSpaceReplace
	WhiteSpaceCollapse
)

type whiteSpaceNormalizer struct{}

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
	switch typ.(type) {
	case *SimpleType, *BuiltinType:
		return ApplyWhiteSpace(value, typ.WhiteSpace())
	default:
		return value
	}
}

func replaceWhiteSpace(value string) string {
	if value == "" {
		return value
	}
	needsReplace := false
	for i := 0; i < len(value); i++ {
		b := value[i]
		if b == '\t' || b == '\n' || b == '\r' {
			needsReplace = true
			break
		}
	}
	if !needsReplace {
		return value
	}

	var builder strings.Builder
	builder.Grow(len(value))
	for i := 0; i < len(value); i++ {
		b := value[i]
		if isXMLWhitespaceByte(b) {
			builder.WriteByte(' ')
			continue
		}
		builder.WriteByte(b)
	}
	return builder.String()
}

func collapseWhiteSpace(value string) string {
	if value == "" {
		return value
	}

	if !needsCollapseXML(value) {
		return value
	}

	buf := make([]byte, 0, len(value))
	inSpace := true
	for i := 0; i < len(value); i++ {
		b := value[i]
		if isXMLWhitespaceByte(b) {
			if !inSpace {
				buf = append(buf, ' ')
				inSpace = true
			}
			continue
		}
		buf = append(buf, b)
		inSpace = false
	}
	if len(buf) > 0 && buf[len(buf)-1] == ' ' {
		buf = buf[:len(buf)-1]
	}
	return string(buf)
}

func needsCollapseXML(value string) bool {
	prevSpace := false
	last := len(value) - 1
	for i := 0; i < len(value); i++ {
		b := value[i]
		if isXMLWhitespaceByte(b) {
			if b != ' ' || i == 0 || prevSpace || i == last {
				return true
			}
			prevSpace = true
			continue
		}
		prevSpace = false
	}
	return false
}

func isXMLWhitespaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}
