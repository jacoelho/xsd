//go:build !forcedcollide

package validator

const (
	attrHashOffset64 = 14695981039346656037
	attrHashPrime64  = 1099511628211
)

func attrNameHash(ns, local []byte) uint64 {
	h := uint64(attrHashOffset64)
	for _, c := range ns {
		h ^= uint64(c)
		h *= attrHashPrime64
	}
	// mix a separator byte to distinguish namespace/local boundaries.
	h ^= 0
	h *= attrHashPrime64
	for _, c := range local {
		h ^= uint64(c)
		h *= attrHashPrime64
	}
	if h == 0 {
		return 1
	}
	return h
}
