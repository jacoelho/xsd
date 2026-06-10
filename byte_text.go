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
