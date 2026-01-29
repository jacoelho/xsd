//go:build forcedcollide

package validator

func attrNameHash(_, _ []byte) uint64 {
	return 1
}
