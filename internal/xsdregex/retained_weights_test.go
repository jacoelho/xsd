package xsdregex

import (
	"testing"
	"unsafe"
)

// TestRetainedByteWeightsCoverStructSizes keeps each weight a sound upper
// bound of the memory unit it estimates.
func TestRetainedByteWeightsCoverStructSizes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		weight uintptr
		size   uintptr
	}{
		{"instruction", instructionBytes, unsafe.Sizeof(instruction{})},
		{"linearAtom", linearAtomBytes, unsafe.Sizeof(linearAtom{})},
		{"runeRange", runeRangeBytes, unsafe.Sizeof(runeRange{})},
		{"int", intBytes, unsafe.Sizeof(int(0))},
		{"sliceHeader", sliceHeaderBytes, unsafe.Sizeof([]int(nil))},
		{"word", wordBytes, unsafe.Sizeof(uint64(0))},
		{"pattern", patternBytes, unsafe.Sizeof(Pattern{}) + unsafe.Sizeof(linearPattern{})},
	} {
		if test.weight < test.size {
			t.Errorf("%s weight %d < Sizeof %d", test.name, test.weight, test.size)
		}
	}
}
