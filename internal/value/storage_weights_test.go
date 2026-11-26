package value

import (
	"testing"
	"unsafe"
)

func TestFacetStorageWeightsCoverStructSizes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		weight uint64
		size   uintptr
	}{
		{"boundValue", boundValueBytes, unsafe.Sizeof(boundValue{})},
		{"enumerationGroup", enumerationGroupBytes, unsafe.Sizeof(enumerationGroup{})},
		{"pattern group header", patternGroupHeaderBytes, unsafe.Sizeof([][]*Pattern{}[:0:0])},
	}
	for _, test := range tests {
		if test.weight < uint64(test.size) {
			t.Errorf("%s weight %d is below its element size %d", test.name, test.weight, test.size)
		}
	}
}
