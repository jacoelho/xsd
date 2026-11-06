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
	// intBytes estimates one int element of a closure.
	intBytes = 8
	// sliceHeaderBytes estimates one []int header in Pattern.closures.
	sliceHeaderBytes = 24
	// wordBytes estimates one uint64 of Pattern.closureBits.
	wordBytes = 8
	// patternBytes estimates the fixed Pattern and linearPattern overhead.
	patternBytes = 256
)

// RetainedBytes estimates the memory the compiled machine retains:
// instructions or linear atoms, precomputed epsilon closures and closure
// bitsets, each distinct character-range backing array once, and the source.
// Every retained derived structure must be counted here; a new retained table
// without a term here escapes admission.
func (p *Pattern) RetainedBytes() uint64 {
	if p == nil {
		return 0
	}
	total := uint64(patternBytes) + uint64(len(p.source)) + runeRangeBytes*p.retainedRanges
	if p.linear != nil {
		return total + linearAtomBytes*uint64(len(p.linear.atoms)) +
			uint64(len(p.linear.literalText)) + uint64(len(p.linear.literalBytes))
	}
	total += instructionBytes * uint64(len(p.states))
	total += sliceHeaderBytes * uint64(len(p.closures))
	ints := len(p.startClosure)
	for _, c := range p.closures {
		ints += len(c)
	}
	total += intBytes * uint64(ints)
	total += wordBytes * uint64(len(p.closureBits))
	return total
}
