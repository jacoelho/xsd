//go:build forcedcollide

package validator

// NameHash forces collisions under the test-only build tag.
func NameHash(_, _ []byte) uint64 {
	return 1
}
