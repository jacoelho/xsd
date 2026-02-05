package value

import (
	"bytes"
	"iter"

	"github.com/jacoelho/xsd/internal/runtime"
)

// NormalizeWhitespace applies the whitespace mode using dst as scratch.
// It returns a slice that may alias the input when no changes are needed.
func NormalizeWhitespace(mode runtime.WhitespaceMode, in, dst []byte) []byte {
	switch mode {
	case runtime.WS_Replace:
		return replaceWhitespace(in, dst)
	case runtime.WS_Collapse:
		return collapseWhitespace(in, dst)
	default:
		return in
	}
}

// TrimXMLWhitespace removes leading and trailing XML whitespace without allocation.
func TrimXMLWhitespace(in []byte) []byte {
	start := 0
	end := len(in)
	for start < end && IsXMLWhitespaceByte(in[start]) {
		start++
	}
	for end > start && IsXMLWhitespaceByte(in[end-1]) {
		end--
	}
	return in[start:end]
}

// TrimXMLWhitespaceString removes leading and trailing XML whitespace.
// It returns the original string when no trimming is needed.
func TrimXMLWhitespaceString(in string) string {
	start := 0
	end := len(in)
	for start < end && IsXMLWhitespaceByte(in[start]) {
		start++
	}
	for end > start && IsXMLWhitespaceByte(in[end-1]) {
		end--
	}
	if start == 0 && end == len(in) {
		return in
	}
	return in[start:end]
}

// FieldsXMLWhitespaceSeq yields XML whitespace-separated fields without allocation.
func FieldsXMLWhitespaceSeq(in []byte) iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		i := 0
		for i < len(in) {
			for i < len(in) && IsXMLWhitespaceByte(in[i]) {
				i++
			}
			if i >= len(in) {
				return
			}
			start := i
			for i < len(in) && !IsXMLWhitespaceByte(in[i]) {
				i++
			}
			if !yield(in[start:i]) {
				return
			}
		}
	}
}

// ForEachXMLWhitespaceField splits input on XML whitespace and calls fn per field.
// It returns the number of fields seen and does not allocate.
func ForEachXMLWhitespaceField(in []byte, fn func([]byte) error) (int, error) {
	count := 0
	for field := range FieldsXMLWhitespaceSeq(in) {
		if fn != nil {
			if err := fn(field); err != nil {
				return count, err
			}
		}
		count++
	}
	return count, nil
}

// SplitXMLWhitespace splits input on XML whitespace and skips empty fields.
func SplitXMLWhitespace(in []byte) [][]byte {
	if len(in) == 0 {
		return nil
	}
	items := make([][]byte, 0, 4)
	for field := range FieldsXMLWhitespaceSeq(in) {
		items = append(items, field)
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func replaceWhitespace(in, dst []byte) []byte {
	needs := false
	for _, b := range in {
		if IsXMLWhitespaceByte(b) && b != ' ' {
			needs = true
			break
		}
	}
	if !needs {
		return in
	}
	out := grow(dst, len(in))
	copy(out, in)
	for i, b := range out {
		if IsXMLWhitespaceByte(b) {
			out[i] = ' '
		}
	}
	return out
}

func collapseWhitespace(in, dst []byte) []byte {
	if !needsCollapse(in) {
		return in
	}
	out := dst[:0]
	if cap(out) < len(in) {
		out = make([]byte, 0, len(in))
	}
	i := 0
	for i < len(in) && IsXMLWhitespaceByte(in[i]) {
		i++
	}
	pendingSpace := false
	for ; i < len(in); i++ {
		b := in[i]
		if IsXMLWhitespaceByte(b) {
			pendingSpace = true
			continue
		}
		if pendingSpace && len(out) > 0 {
			out = append(out, ' ')
		}
		pendingSpace = false
		out = append(out, b)
	}
	return out
}

func needsCollapse(in []byte) bool {
	if len(in) == 0 {
		return false
	}
	if IsXMLWhitespaceByte(in[0]) || IsXMLWhitespaceByte(in[len(in)-1]) {
		return true
	}
	if bytes.IndexByte(in, '\t') >= 0 || bytes.IndexByte(in, '\n') >= 0 || bytes.IndexByte(in, '\r') >= 0 {
		return true
	}
	return bytes.Contains(in, doubleSpaceBytes)
}

var doubleSpaceBytes = []byte("  ")

// IsXMLWhitespaceByte reports whether the byte is XML whitespace.
func IsXMLWhitespaceByte(b byte) bool {
	if b > ' ' {
		return false
	}
	switch b {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func grow(buf []byte, size int) []byte {
	if cap(buf) < size {
		return make([]byte, size)
	}
	return buf[:size]
}
