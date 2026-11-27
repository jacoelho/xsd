package xsdregex

// Byte weights estimate retained memory per unit. Each is at least the size of
// the struct it names; retained_weights_test.go asserts that.
const (
	// instructionBytes estimates one instruction (NFA state).
	instructionBytes = 48
	// linearAtomBytes estimates one linearAtom.
	linearAtomBytes = 48
	// runeRangeBytes estimates one runeRange (two runes).
	runeRangeBytes = 8
	// intBytes estimates one int element of Pattern.scalarNext.
	intBytes = 8
	// wordBytes estimates one uint64 of Pattern.closureBits.
	wordBytes = 8
	// patternBytes estimates the fixed Pattern and linearPattern overhead.
	patternBytes        = 256
	countedRepeatsBytes = 64
	counterBytes        = 12
	stateSlotsBytes     = 12
)

// RetainedBytes estimates the memory the compiled machine retains: every
// retained slice is charged by capacity at its weight, since the allocator
// keeps the capacity whatever the length. Instructions or linear atoms, the
// scalar successor table, the closure bitsets and the counter tables are
// counted, each distinct character-range backing array once, and the source. Every retained derived
// structure must be counted here; a new retained table without a term here
// escapes admission.
func (p *Pattern) RetainedBytes() uint64 {
	if p == nil {
		return 0
	}
	total := uint64(patternBytes) + uint64(len(p.source)) + runeRangeBytes*p.retainedRanges
	if p.linear != nil {
		return total + linearAtomBytes*uint64(cap(p.linear.atoms)) +
			uint64(len(p.linear.literalText)) + uint64(cap(p.linear.literalBytes))
	}
	total += instructionBytes * uint64(cap(p.states))
	total += intBytes * uint64(cap(p.scalarNext))
	total += wordBytes * uint64(cap(p.closureBits))
	if p.counted != nil {
		total += countedRepeatsBytes + counterBytes*uint64(cap(p.counted.counters)) +
			stateSlotsBytes*uint64(cap(p.counted.slots))
	}
	return total
}
