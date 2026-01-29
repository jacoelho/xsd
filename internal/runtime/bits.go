package runtime

import "math/bits"

// NextPow2 returns the next power-of-two >= n with a minimum of 1.
func NextPow2(n int) int {
	if n <= 1 {
		return 1
	}
	return 1 << bits.Len(uint(n-1))
}
