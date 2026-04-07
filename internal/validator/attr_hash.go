//go:build !forcedcollide

package validator

const (
	hashOffset64 = 14695981039346656037
	hashPrime64  = 1099511628211
)

// NameHash returns a stable attribute-name hash over namespace and local bytes.
func NameHash(ns, local []byte) uint64 {
	h := uint64(hashOffset64)
	for _, c := range ns {
		h ^= uint64(c)
		h *= hashPrime64
	}
	h ^= 0
	h *= hashPrime64
	for _, c := range local {
		h ^= uint64(c)
		h *= hashPrime64
	}
	if h == 0 {
		return 1
	}
	return h
}
