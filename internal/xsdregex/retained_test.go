package xsdregex

import (
	"strings"
	"testing"
)

func TestRetainedBytesNilIsZero(t *testing.T) {
	t.Parallel()
	var p *Pattern
	if got := p.RetainedBytes(); got != 0 {
		t.Fatalf("RetainedBytes(nil) = %d, want 0", got)
	}
}

func TestRetainedBytesChargesSharedRangeArrayOnce(t *testing.T) {
	t.Parallel()
	const copies = 5000
	one := mustCompile(t, `\p{L}`)
	many := mustCompile(t, strings.Repeat(`\p{L}`, copies))
	setRanges := uint64(len(one.linear.atoms[0].set.ranges))
	if setRanges == 0 {
		t.Fatal("\\p{L} compiled to an empty set")
	}
	extra := many.RetainedBytes() - one.RetainedBytes()
	if bound := uint64(copies) * runeRangeBytes * setRanges; extra >= bound {
		t.Fatalf("%d copies retain %d extra bytes, want < %d: shared array must be charged once", copies, extra, bound)
	}
}

func TestRetainedBytesCountsClosures(t *testing.T) {
	t.Parallel()
	p := mustCompile(t, `(a?){1,15}b`)
	if p.linear != nil || len(p.closures) == 0 {
		t.Fatalf("pattern is not on the NFA path with precomputed closures (linear=%v, closures=%d)", p.linear != nil, len(p.closures))
	}
	floor := uint64(instructionBytes*len(p.states) + len(p.source))
	if got := p.RetainedBytes(); got <= floor {
		t.Fatalf("RetainedBytes = %d, want > %d: closures must be counted", got, floor)
	}
}

func TestRetainedBytesGrowsWithDistinctRangeArrays(t *testing.T) {
	t.Parallel()
	small := mustCompile(t, `[^\p{L}]`)
	large := mustCompile(t, strings.Repeat(`[^\p{L}]`, 50))
	if large.RetainedBytes() <= small.RetainedBytes() {
		t.Fatalf("RetainedBytes did not grow with distinct class arrays: %d <= %d", large.RetainedBytes(), small.RetainedBytes())
	}
}
