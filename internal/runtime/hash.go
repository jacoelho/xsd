package runtime

// FNVOffset64 and FNVPrime64 are 64-bit FNV-1a constants.
const (
	FNVOffset64 = 14695981039346656037
	FNVPrime64  = 1099511628211
)

func hashBytes(b []byte) uint64 {
	h := uint64(FNVOffset64)
	for _, c := range b {
		h ^= uint64(c)
		h *= FNVPrime64
	}
	return h
}

// HashBytes returns a stable 64-bit hash for arbitrary byte slices.
func HashBytes(b []byte) uint64 {
	h := hashBytes(b)
	if h == 0 {
		return 1
	}
	return h
}

// HashKey returns a stable 64-bit hash for a value-space key.
func HashKey(kind ValueKind, b []byte) uint64 {
	h := uint64(FNVOffset64)
	h ^= uint64(kind)
	h *= FNVPrime64
	for _, c := range b {
		h ^= uint64(c)
		h *= FNVPrime64
	}
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	if h == 0 {
		return 1
	}
	return h
}

func hashSymbol(nsID NamespaceID, local []byte) uint64 {
	h := uint64(FNVOffset64)
	v := uint32(nsID)
	for range 4 {
		h ^= uint64(byte(v))
		h *= FNVPrime64
		v >>= 8
	}
	for _, c := range local {
		h ^= uint64(c)
		h *= FNVPrime64
	}
	if h == 0 {
		return 1
	}
	return h
}
