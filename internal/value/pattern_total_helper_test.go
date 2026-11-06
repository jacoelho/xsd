package value

// testPatternTotal returns a fresh, effectively unbounded aggregate pattern
// account so tests that do not exercise the aggregate limit are unaffected.
func testPatternTotal() *PatternAccount {
	var a PatternAccount
	a.Reset(1 << 62)
	return &a
}

// sequentialPatternTotal is shared by allocation-counting tests, which must
// not allocate a fresh account per call. It is not safe for parallel tests.
var sequentialPatternTotal = testPatternTotal()
