package valuebench

import "github.com/jacoelho/xsd/internal/value"

// benchPatternTotal is shared by the sequential benchmarks in this package.
// Its limit is effectively unbounded so it never alters measured behaviour,
// and sharing it keeps per-iteration allocations out of the results.
var benchPatternTotal = func() *value.PatternAccount {
	var a value.PatternAccount
	a.Reset(1 << 62)
	return &a
}()

func testPatternTotal() *value.PatternAccount { return benchPatternTotal }
