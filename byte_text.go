package xsd

type byteText interface {
	~string | ~[]byte
}

func byteTextEqual[T byteText](s string, text T) bool {
	if len(s) != len(text) {
		return false
	}
	for i := range s {
		if s[i] != text[i] {
			return false
		}
	}
	return true
}

// skipLeadingZeros returns the first index in s[start:end) that is not '0',
// or end when the range is empty or all zeros.
func skipLeadingZeros[T byteText](s T, start, end int) int {
	for start < end && s[start] == '0' {
		start++
	}
	return start
}

// trimTrailingZeros returns the end of s[start:end) with trailing '0' bytes
// removed, or start when the range is empty or all zeros.
func trimTrailingZeros[T byteText](s T, start, end int) int {
	for end > start && s[end-1] == '0' {
		end--
	}
	return end
}

// stringBytesEqual stays non-generic: the cheaper call lets hot callers such
// as byteStringCache.recentString inline fully, and the byte loop beats a
// memequal call for the short names compared during validation.
func stringBytesEqual(s string, b []byte) bool {
	if len(s) != len(b) {
		return false
	}
	for i := range b {
		if s[i] != b[i] {
			return false
		}
	}
	return true
}
