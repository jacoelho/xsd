package value

import (
	"bytes"

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
	for start < end && isXMLWhitespace(in[start]) {
		start++
	}
	for end > start && isXMLWhitespace(in[end-1]) {
		end--
	}
	return in[start:end]
}

func replaceWhitespace(in, dst []byte) []byte {
	needs := false
	for _, b := range in {
		if isXMLWhitespace(b) && b != ' ' {
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
		if isXMLWhitespace(b) {
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
	for i < len(in) && isXMLWhitespace(in[i]) {
		i++
	}
	pendingSpace := false
	for ; i < len(in); i++ {
		b := in[i]
		if isXMLWhitespace(b) {
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
	if isXMLWhitespace(in[0]) || isXMLWhitespace(in[len(in)-1]) {
		return true
	}
	if bytes.IndexByte(in, '\t') >= 0 || bytes.IndexByte(in, '\n') >= 0 || bytes.IndexByte(in, '\r') >= 0 {
		return true
	}
	return bytes.Contains(in, doubleSpaceBytes)
}

var doubleSpaceBytes = []byte("  ")

func isXMLWhitespace(b byte) bool {
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
