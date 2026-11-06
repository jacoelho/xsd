package value_test

import "github.com/jacoelho/xsd/internal/value"

// testPatternTotal returns a fresh, effectively unbounded aggregate pattern
// account so tests that do not exercise the aggregate limit are unaffected.
func testPatternTotal() *value.PatternAccount {
	var a value.PatternAccount
	a.Reset(1 << 62)
	return &a
}
