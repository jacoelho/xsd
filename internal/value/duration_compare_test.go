package value

import (
	"errors"
	"testing"
)

func mustParseDuration(t *testing.T, lexical string) Duration {
	t.Helper()
	dur, err := ParseDuration(lexical)
	if err != nil {
		t.Fatalf("ParseDuration(%q) error = %v", lexical, err)
	}
	return dur
}

func TestCompareDuration(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  int
	}{
		{
			name:  "day time equal",
			left:  "PT26H",
			right: "P1DT2H",
			want:  0,
		},
		{
			name:  "month less than 32d",
			left:  "P1M",
			right: "P32D",
			want:  -1,
		},
		{
			name:  "negative seconds ordering",
			left:  "-P1M0DT0H0M1S",
			right: "-P1M0DT0H0M2S",
			want:  1,
		},
		{
			name:  "fractional seconds ordering",
			left:  "P1M0DT0H0M0.5S",
			right: "P1M0DT0H0M0.4S",
			want:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			left := mustParseDuration(t, tc.left)
			right := mustParseDuration(t, tc.right)
			got, err := CompareDuration(left, right)
			if err != nil {
				t.Fatalf("CompareDuration(%q,%q) error = %v", tc.left, tc.right, err)
			}
			switch {
			case tc.want == 0 && got != 0:
				t.Fatalf("CompareDuration(%q,%q) = %d, want 0", tc.left, tc.right, got)
			case tc.want < 0 && got >= 0:
				t.Fatalf("CompareDuration(%q,%q) = %d, want <0", tc.left, tc.right, got)
			case tc.want > 0 && got <= 0:
				t.Fatalf("CompareDuration(%q,%q) = %d, want >0", tc.left, tc.right, got)
			}
		})
	}
}

func TestCompareDurationIndeterminate(t *testing.T) {
	left := mustParseDuration(t, "P1M")
	right := mustParseDuration(t, "P30D")
	_, err := CompareDuration(left, right)
	if !errors.Is(err, ErrIndeterminateDurationComparison) {
		t.Fatalf("CompareDuration() error = %v, want ErrIndeterminateDurationComparison", err)
	}
}

func TestCanonicalDurationString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "P0D", want: "PT0S"},
		{input: "PT0S", want: "PT0S"},
		{input: "-P0D", want: "PT0S"},
		{input: "-PT0S", want: "PT0S"},
		{input: "PT0.000001S", want: "PT0.000001S"},
		{input: "PT123456.789S", want: "PT123456.789S"},
		{input: "P1Y2M3DT4H5M6.7S", want: "P1Y2M3DT4H5M6.7S"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			dur := mustParseDuration(t, tc.input)
			got := CanonicalDurationString(dur)
			if got != tc.want {
				t.Fatalf("CanonicalDurationString(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
