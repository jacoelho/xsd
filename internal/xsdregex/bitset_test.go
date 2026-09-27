package xsdregex

import "testing"

func TestBitCount(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mask, want uint64
	}{
		{0, 0},
		{1, 1},
		{1 << 63, 1},
		{0x8000000000000001, 2},
		{0x5555555555555555, 32},
		{0xfffffffffffffffe, 63},
		{0xffffffffffffffff, 64},
	} {
		if got := bitCount(tc.mask); got != tc.want {
			t.Errorf("bitCount(%x) = %d, want %d", tc.mask, got, tc.want)
		}
	}
}

func TestBitsetMatchExactLimits(t *testing.T) {
	t.Parallel()
	pattern := mustCompile(t, `(a|aa)*b`)
	for _, tc := range []struct {
		name    string
		options MatchOptions
		limited bool
	}{
		{"at-work", MatchOptions{MaxWork: 27}, false},
		{"below-work", MatchOptions{MaxWork: 26}, true},
		{"at-states", MatchOptions{MaxStates: 4}, false},
		{"below-states", MatchOptions{MaxStates: 3}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var scratch Scratch
			for _, bytes := range []bool{false, true} {
				var matched bool
				var err error
				if bytes {
					matched, err = pattern.MatchBytesWithScratch([]byte("aab"), tc.options, &scratch)
				} else {
					matched, err = pattern.MatchStringWithScratch("aab", tc.options, &scratch)
				}
				if tc.limited {
					if !IsLimit(err) {
						t.Fatalf("bytes=%v: error = %v, want limit", bytes, err)
					}
				} else if err != nil || !matched {
					t.Fatalf("bytes=%v: match = %v, %v", bytes, matched, err)
				}
			}
		})
	}
}
