package model

import (
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/durationlex"
)

func TestParseXSDDurationComponentTooLarge(t *testing.T) {
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
			_, err := durationlex.Parse(tc.input)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), "too large") {
				t.Fatalf("error = %v, want contains 'too large'", err)
			}
		})
	}
}
