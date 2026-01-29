package runtime

const (
	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

func hashBytes(b []byte) uint64 {
	h := uint64(fnvOffset64)
	for _, c := range b {
		h ^= uint64(c)
		h *= fnvPrime64
	}
	if h == 0 {
		return 1
	}
	return h
}

// HashBytes returns a stable 64-bit hash for arbitrary byte slices.
func HashBytes(b []byte) uint64 {
	return hashBytes(b)
}

// HashKey returns a stable 64-bit hash for a value-space key.
func HashKey(kind ValueKind, b []byte) uint64 {
	h := uint64(fnvOffset64)
	h ^= uint64(kind)
	h *= fnvPrime64
	for _, c := range b {
		h ^= uint64(c)
		h *= fnvPrime64
	}
	if h == 0 {
		return 1
	}
	return h
}

func hashSymbol(nsID NamespaceID, local []byte) uint64 {
	h := uint64(fnvOffset64)
	v := uint32(nsID)
	for range 4 {
		h ^= uint64(byte(v))
		h *= fnvPrime64
		v >>= 8
	}
	for _, c := range local {
		h ^= uint64(c)
		h *= fnvPrime64
	}
	if h == 0 {
		return 1
	}
	return h
}
