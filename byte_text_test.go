package xsd

import "testing"

func TestSkipLeadingZeros(t *testing.T) {
	tests := []struct {
		s          string
		start, end int
		want       int
	}{
		{"123", 0, 3, 0},
		{"00123", 0, 5, 2},
		{"000", 0, 3, 3},
		{"", 0, 0, 0},
		{"-00123", 1, 6, 3},
		{"00120", 0, 3, 2},
		{"100", 0, 3, 0},
	}
	for _, tc := range tests {
		if got := skipLeadingZeros(tc.s, tc.start, tc.end); got != tc.want {
			t.Errorf("skipLeadingZeros(%q, %d, %d) = %d, want %d", tc.s, tc.start, tc.end, got, tc.want)
		}
		if got := skipLeadingZeros([]byte(tc.s), tc.start, tc.end); got != tc.want {
			t.Errorf("skipLeadingZeros([]byte(%q), %d, %d) = %d, want %d", tc.s, tc.start, tc.end, got, tc.want)
		}
	}
}

func TestTrimTrailingZeros(t *testing.T) {
	tests := []struct {
		s          string
		start, end int
		want       int
	}{
		{"123", 0, 3, 3},
		{"12300", 0, 5, 3},
		{"000", 0, 3, 0},
		{"", 0, 0, 0},
		{"1.2300", 2, 6, 4},
		{"00100", 2, 5, 3},
		{"001", 0, 3, 3},
	}
	for _, tc := range tests {
		if got := trimTrailingZeros(tc.s, tc.start, tc.end); got != tc.want {
			t.Errorf("trimTrailingZeros(%q, %d, %d) = %d, want %d", tc.s, tc.start, tc.end, got, tc.want)
		}
		if got := trimTrailingZeros([]byte(tc.s), tc.start, tc.end); got != tc.want {
			t.Errorf("trimTrailingZeros([]byte(%q), %d, %d) = %d, want %d", tc.s, tc.start, tc.end, got, tc.want)
		}
	}
}
