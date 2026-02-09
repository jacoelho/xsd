package durationlex

import (
	"strconv"
	"strings"
	"testing"
)

func TestParseDuration(t *testing.T) {
	got, err := Parse("P1Y2M3DT4H5M6.7S")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Years != 1 || got.Months != 2 || got.Days != 3 || got.Hours != 4 || got.Minutes != 5 {
		t.Fatalf("duration components = %+v", got)
	}
	if rendered := string(got.Seconds.RenderCanonical(nil)); rendered != "6.7" {
		t.Fatalf("duration seconds = %s, want 6.7", rendered)
	}
}

func TestParseDurationComponentTooLarge(t *testing.T) {
	tooLarge := strconv.FormatUint(uint64(^uint(0)>>1)+1, 10)
	cases := []struct {
		name  string
		input string
	}{
		{name: "years", input: "P" + tooLarge + "Y"},
		{name: "months", input: "P" + tooLarge + "M"},
		{name: "hours", input: "PT" + tooLarge + "H"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.input)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "too large") {
				t.Fatalf("error = %v, want contains 'too large'", err)
			}
		})
	}
}
