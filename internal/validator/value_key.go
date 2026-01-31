package validator

import "github.com/jacoelho/xsd/internal/runtime"

// FreezeKey copies key.Bytes into arena-stable memory and returns a stable key.
func FreezeKey(sess *Session, k runtime.ValueKey) runtime.ValueKey {
	if sess == nil || len(k.Bytes) == 0 {
		return k
	}
	buf := sess.Arena.Alloc(len(k.Bytes))
	copy(buf, k.Bytes)
	k.Bytes = buf
	return k
}
