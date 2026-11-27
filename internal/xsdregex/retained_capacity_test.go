package xsdregex

import (
	"testing"
	"unsafe"
)

func retainedCapacityOracle(p *Pattern) uint64 {
	total := uint64(len(p.source))
	if p.linear != nil {
		total += uint64(cap(p.linear.atoms)) * uint64(unsafe.Sizeof(linearAtom{}))
		total += uint64(cap(p.linear.ascii)) * uint64(unsafe.Sizeof(asciiSet{}))
		total += uint64(len(p.linear.literalText)) + uint64(cap(p.linear.literalBytes))
	}
	total += uint64(cap(p.states)) * uint64(unsafe.Sizeof(instruction{}))
	total += uint64(cap(p.closureBits)) * uint64(unsafe.Sizeof(uint64(0)))
	total += p.retainedRanges * uint64(unsafe.Sizeof(runeRange{}))
	if p.dfa != nil {
		total += uint64(cap(p.dfa.next)+cap(p.dfa.alphabet.pages)) * uint64(unsafe.Sizeof(uint16(0)))
		total += uint64(cap(p.dfa.alphabet.runs)) * uint64(unsafe.Sizeof(uint32(0)))
		total += uint64(unsafe.Sizeof(*p.dfa))
	}
	return total
}

func TestRetainedBytesCoversCapacityOfRetainedSlices(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, source, mode string
		compile            func(testing.TB, string) *Pattern
	}{
		{"literal", `abc`, "linear", mustCompile},
		{"table over ASCII", `(\d{3}-){2}[a-f]{4}`, "dfa", mustCompile},
		{"table with pages", `[\p{L}\p{N}]{1,35}`, "dfa", mustCompile},
		{"run-length", `[A-Z]{2}\d{4}`, "linear", mustCompileWithoutDFA},
		{"small bitset", `(a|b)*c`, "bitset", mustCompileWithoutDFA},
		{"large NFA", `(a|b){33000}`, "general", mustCompile},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			p := test.compile(t, test.source)
			if got := modeName(p); got != test.mode {
				t.Fatalf("mode = %s, want %s", got, test.mode)
			}
			if got, floor := p.RetainedBytes(), retainedCapacityOracle(p); got < floor {
				t.Fatalf("RetainedBytes = %d, below the capacity of retained slices %d", got, floor)
			}
		})
	}
}
