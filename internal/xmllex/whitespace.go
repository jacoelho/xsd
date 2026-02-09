package xmllex

import "unicode/utf8"

// IsIgnorableOutsideRoot reports whether data contains only XML whitespace.
// If allowBOM is true, a leading BOM is permitted before any other character.
func IsIgnorableOutsideRoot(data []byte, allowBOM bool) bool {
	sawNonBOM := false
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			return false
		}
		if r == '\uFEFF' {
			if !allowBOM || sawNonBOM {
				return false
			}
			allowBOM = false
			data = data[size:]
			continue
		}
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
		sawNonBOM = true
		allowBOM = false
		data = data[size:]
	}
	return true
}
