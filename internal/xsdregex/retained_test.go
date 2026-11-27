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
	one := mustCompileWithoutDFA(t, `\p{L}`)
	many := mustCompileWithoutDFA(t, strings.Repeat(`\p{L}`, copies))
	setRanges := uint64(len(one.linear.atoms[0].set.ranges))
	if setRanges == 0 {
		t.Fatal("\\p{L} compiled to an empty set")
	}
	extra := many.RetainedBytes() - one.RetainedBytes()
	if bound := uint64(copies) * runeRangeBytes * setRanges; extra >= bound {
		t.Fatalf("%d copies retain %d extra bytes, want < %d: shared array must be charged once", copies, extra, bound)
	}
}

func TestRetainedBytesCountsClosureBitsets(t *testing.T) {
	t.Parallel()
	p := mustCompileWithoutDFA(t, `(a?){1,15}b`)
	if p.mode != modeBitset || len(p.closureBits) == 0 {
		t.Fatalf("pattern is not in bitset mode with closure bitsets (mode=%d, words=%d)", p.mode, len(p.closureBits))
	}
	floor := uint64(instructionBytes*len(p.states) + len(p.source))
	if got := p.RetainedBytes(); got <= floor {
		t.Fatalf("RetainedBytes = %d, want > %d: closure bitsets must be counted", got, floor)
	}
}

func TestRetainedBytesGrowsWithDistinctRangeArrays(t *testing.T) {
	t.Parallel()
	small := mustCompileWithoutDFA(t, `[^\p{L}]`)
	large := mustCompileWithoutDFA(t, strings.Repeat(`[^\p{L}]`, 50))
	if large.RetainedBytes() <= small.RetainedBytes() {
		t.Fatalf("RetainedBytes did not grow with distinct class arrays: %d <= %d", large.RetainedBytes(), small.RetainedBytes())
	}
}
