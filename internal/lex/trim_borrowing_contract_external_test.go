package lex_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/lex"
)

func TestTrimXMLWhitespaceBytesBorrowsInput(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input []byte
		index int
	}{
		{name: "trimmed", input: []byte("\t7\r"), index: 1},
		{name: "unchanged", input: []byte("7"), index: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := lex.TrimXMLWhitespaceBytes(tc.input)
			if string(got) != "7" {
				t.Fatalf("trimmed = %q, want %q", got, "7")
			}
			tc.input[tc.index] = '8'
			if string(got) != "8" {
				t.Fatalf("borrowed result after input mutation = %q, want %q", got, "8")
			}
		})
	}
}
