package xsdregex

import (
	"slices"
	"testing"
	"unicode"
)

func TestUnionMany(t *testing.T) {
	t.Parallel()
	single := setFromRanges([]runeRange{{lo: 20, hi: 22}, {lo: 30, hi: 30}})
	tests := []struct {
		name string
		sets []rangeSet
		want []runeRange
	}{
		{name: "empty", want: nil},
		{name: "empty inputs", sets: []rangeSet{{}, {}, {}}, want: nil},
		{name: "single non-empty input", sets: []rangeSet{{}, single, {}}, want: single.ranges},
		{
			name: "overlap and adjacency",
			sets: []rangeSet{
				setFromRanges([]runeRange{{lo: 10, hi: 12}, {lo: 20, hi: 20}}),
				setFromRanges([]runeRange{{lo: 1, hi: 3}, {lo: 4, hi: 8}}),
				setFromRanges([]runeRange{{lo: 8, hi: 11}, {lo: 30, hi: 30}}),
			},
			want: []runeRange{{lo: 1, hi: 12}, {lo: 20, hi: 20}, {lo: 30, hi: 30}},
		},
		{
			name: "same start with different ends",
			sets: []rangeSet{
				setFromRanges([]runeRange{{lo: 5, hi: 5}}),
				setFromRanges([]runeRange{{lo: 5, hi: 10}}),
				setFromRanges([]runeRange{{lo: 5, hi: 7}}),
			},
			want: []runeRange{{lo: 5, hi: 10}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := unionMany(test.sets...)
			if !slices.Equal(got.ranges, test.want) {
				t.Fatalf("unionMany() = %v, want %v", got.ranges, test.want)
			}
		})
	}
}

func TestUnionManyRepeatedRangesKeepsTemporaryAllocationsBounded(t *testing.T) {
	base := makeRangeSetForTest(128)
	sets := make([]rangeSet, 512)
	for i := range sets {
		sets[i] = base
	}
	got := unionMany(sets...)
	if !slices.Equal(got.ranges, base.ranges) {
		t.Fatalf("repeated union has %d ranges, want %d", len(got.ranges), len(base.ranges))
	}
	allocations := testing.AllocsPerRun(5, func() {
		_ = unionMany(sets...)
	})
	if allocations > 64 {
		t.Fatalf("repeated union allocations = %v, want bounded temporary storage", allocations)
	}
}

func TestRangeTableSetPreservesStridedRows(t *testing.T) {
	t.Parallel()
	got := rangeTableSet(&unicode.RangeTable{
		R16: []unicode.Range16{
			{Lo: 'a', Hi: 'f', Stride: 1},
			{Lo: 'h', Hi: 'n', Stride: 2},
		},
		R32: []unicode.Range32{{Lo: 0x10000, Hi: 0x10002, Stride: 1}},
	})
	want := []runeRange{
		{lo: 'a', hi: 'f'},
		{lo: 'h', hi: 'h'},
		{lo: 'j', hi: 'j'},
		{lo: 'l', hi: 'l'},
		{lo: 'n', hi: 'n'},
		{lo: 0x10000, hi: 0x10002},
	}
	if !slices.Equal(got.ranges, want) {
		t.Fatalf("rangeTableSet() = %v, want %v", got.ranges, want)
	}
}

func makeRangeSetForTest(count int) rangeSet {
	ranges := make([]runeRange, count)
	for i := range ranges {
		ranges[i] = runeRange{lo: rune(i * 2), hi: rune(i * 2)}
	}
	return rangeSet{ranges: ranges}
}
