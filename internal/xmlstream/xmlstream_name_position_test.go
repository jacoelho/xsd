package xmlstream

import (
	"errors"
	"strings"
	"testing"
)

// A malformed qualified name is reported after the whole name, with a
// byte-based column, regardless of how the input is split into reads.
func TestQualifiedNameErrorPosition(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		input  string
		line   int
		column int
	}{
		{name: "start_two_colons", input: `<a:b:c/>`, line: 1, column: 6},
		{name: "start_digit", input: `<1a/>`, line: 1, column: 3},
		{name: "after_lf", input: "<r>\n<a:b:c/></r>", line: 2, column: 6},
		{name: "after_crlf", input: "<r>\r\n<a:b:c/></r>", line: 2, column: 6},
		{name: "after_cr", input: "<r>\r<a:b:c/></r>", line: 2, column: 6},
		{name: "after_two_crlf", input: "<r>\r\n\r\n<1a/></r>", line: 3, column: 3},
		{name: "after_two_byte_rune", input: `<r>é<a:b:c/></r>`, line: 1, column: 11},
		{name: "after_four_byte_rune", input: `<r>😀<a:b:c/></r>`, line: 1, column: 13},
		{name: "non_ascii_prefix", input: `<r><π:1/></r>`, line: 1, column: 8},
		{name: "end_two_colons", input: `<r></a:b:c></r>`, line: 1, column: 10},
		{name: "end_digit", input: `<r></1a></r>`, line: 1, column: 7},
		{name: "end_non_ascii_prefix", input: `<r></π:1></r>`, line: 1, column: 9},
		{name: "attr_two_colons", input: `<r a:b:c="v"/>`, line: 1, column: 8},
		{name: "attr_digit", input: `<r 1a="v"/>`, line: 1, column: 5},
		{name: "attr_after_crlf", input: "<r\r\n  a:b:c='v'/>", line: 2, column: 7},
		{name: "attr_after_two_byte_rune", input: `<r é='1' a:b:c='v'/>`, line: 1, column: 15},
		{name: "long_name", input: `<r>pad<abcdef:ghi:jkl/></r>`, line: 1, column: 21},
		{name: "trailing_colon", input: `<r>pad<abcdefghijkl:/></r>`, line: 1, column: 20},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, rc := range []readerCase{wholeInput, oneByteInput, chunkedInput(2)} {
				_, err := recordReader(rc.open(test.input), Config{}, 0)
				xerr, ok := errors.AsType[*Error](err)
				if !ok || xerr.Kind != ErrorSyntax || !strings.Contains(err.Error(), "invalid XML qualified name") {
					t.Fatalf("%s: error = %v, want qualified-name syntax error", rc.name, err)
				}
				if xerr.Line != test.line || xerr.Column != test.column {
					t.Fatalf("%s: position = %d:%d, want %d:%d", rc.name, xerr.Line, xerr.Column, test.line, test.column)
				}
			}
		})
	}
}
