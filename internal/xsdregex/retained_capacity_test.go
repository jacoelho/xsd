package xsdregex

import (
	"testing"
	"unsafe"
)

func retainedCapacityOracle(p *Pattern) uint64 {
	total := uint64(len(p.source))
	if p.linear != nil {
		total += uint64(cap(p.linear.atoms)) * uint64(unsafe.Sizeof(linearAtom{}))
		total += uint64(len(p.linear.literalText)) + uint64(cap(p.linear.literalBytes))
	}
	total += uint64(cap(p.states)) * uint64(unsafe.Sizeof(instruction{}))
	total += uint64(cap(p.scalarNext)) * uint64(unsafe.Sizeof(int(0)))
	total += uint64(cap(p.closureBits)) * uint64(unsafe.Sizeof(uint64(0)))
	total += p.retainedRanges * uint64(unsafe.Sizeof(runeRange{}))
	return total
}

func TestRetainedBytesCoversCapacityOfRetainedSlices(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, source, mode string
	}{
		{"linear", `abc`, "linear"},
		{"small bitset", `(a|b)*c`, "bitset"},
		{"large NFA", `(a|b){33000}`, "general"},
		{"deterministic non-linear", `(\d{3}-){2}\d{4}`, "scalar"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			p := mustCompile(t, test.source)
			if got := modeName(p); got != test.mode {
				t.Fatalf("mode = %s, want %s", got, test.mode)
			}
			if got, floor := p.RetainedBytes(), retainedCapacityOracle(p); got < floor {
				t.Fatalf("RetainedBytes = %d, below the capacity of retained slices %d", got, floor)
			}
		})
	}
}
