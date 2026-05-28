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
