package xmlstream

import (
	"slices"
	"strings"
	"testing"
)

type tokenPosition struct {
	kind   TokenKind
	line   int
	column int
}

func TestTokenPositionIsFirstBytePosition(t *testing.T) {
	t.Parallel()

	config := Config{CommentMode: CommentModeEmit, EmitPI: true}
	for _, test := range []struct {
		name  string
		input string
		want  []tokenPosition
	}{
		{
			name:  "start_at_document_start",
			input: "<r>t</r>",
			want:  []tokenPosition{{KindStart, 1, 1}, {KindCharData, 1, 4}, {KindEnd, 1, 5}},
		},
		{
			name:  "comment_at_document_start",
			input: "<!--c--><r/>",
			want:  []tokenPosition{{KindComment, 1, 1}, {KindStart, 1, 9}, {KindEnd, 1, 12}},
		},
		{
			name:  "pi_at_document_start",
			input: "<?p d?><r/>",
			want:  []tokenPosition{{KindPI, 1, 1}, {KindStart, 1, 8}, {KindEnd, 1, 11}},
		},
		{
			name:  "line_break_text_at_document_start",
			input: "\n<r/>",
			want:  []tokenPosition{{KindCharData, 1, 1}, {KindStart, 2, 1}, {KindEnd, 2, 4}},
		},
		{
			name:  "crlf_text_at_document_start",
			input: "\r\n<r/>",
			want:  []tokenPosition{{KindCharData, 1, 1}, {KindStart, 2, 1}, {KindEnd, 2, 4}},
		},
		{
			name:  "every_kind_at_line_start",
			input: "<r>\n<a>t</a>\n<!--c-->\r\n<?p d?>\r<![CDATA[x]]>\n</r>",
			want: []tokenPosition{
				{KindStart, 1, 1}, {KindCharData, 1, 4},
				{KindStart, 2, 1}, {KindCharData, 2, 4}, {KindEnd, 2, 5}, {KindCharData, 2, 9},
				{KindComment, 3, 1}, {KindCharData, 3, 9},
				{KindPI, 4, 1}, {KindCharData, 4, 8},
				{KindCharData, 5, 1}, {KindCharData, 5, 14},
				{KindEnd, 6, 1},
			},
		},
		{
			name:  "text_starting_with_line_break_after_start",
			input: "<r>\n  t</r>",
			want:  []tokenPosition{{KindStart, 1, 1}, {KindCharData, 1, 4}, {KindEnd, 2, 4}},
		},
		{
			name:  "text_starting_with_cr_after_start",
			input: "<r>\rt</r>",
			want:  []tokenPosition{{KindStart, 1, 1}, {KindCharData, 1, 4}, {KindEnd, 2, 2}},
		},
		{
			name:  "reference_after_line_break_in_markup",
			input: "<r\n>&amp;<!--\n-->&#10;<?p\n?>&lt;</r>",
			want: []tokenPosition{
				{KindStart, 1, 1}, {KindCharData, 2, 2},
				{KindComment, 2, 7}, {KindCharData, 3, 4},
				{KindPI, 3, 9}, {KindCharData, 4, 3},
				{KindEnd, 4, 7},
			},
		},
		{
			name:  "cdata_after_line_break",
			input: "<r>\n<![CDATA[\nx]]></r>",
			want:  []tokenPosition{{KindStart, 1, 1}, {KindCharData, 1, 4}, {KindCharData, 2, 1}, {KindEnd, 3, 5}},
		},
		{
			name:  "after_bom",
			input: "\ufeff\n<r>\n</r>",
			want:  []tokenPosition{{KindCharData, 1, 1}, {KindStart, 2, 1}, {KindCharData, 2, 4}, {KindEnd, 3, 1}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, rc := range []readerCase{wholeInput, oneByteInput, chunkedInput(2)} {
				tokens, err := recordReader(rc.open(test.input), config, 0)
				if err != nil {
					t.Fatalf("%s: %v", rc.name, err)
				}
				got := make([]tokenPosition, len(tokens))
				for i, tok := range tokens {
					got[i] = tokenPosition{tok.kind, tok.line, tok.column}
				}
				if !slices.Equal(got, test.want) {
					t.Fatalf("%s: positions = %v, want %v", rc.name, got, test.want)
				}
			}
		})
	}
}

func TestCDATAContinuationChunkPositionIsFirstBytePosition(t *testing.T) {
	t.Parallel()

	const prefix = "<r>\n<![CDATA["
	input := prefix + strings.Repeat("x", xmlInputBufferSize) + "\ny]]></r>"
	for _, rc := range []readerCase{wholeInput, chunkedInput(4096)} {
		tokens, err := recordReader(rc.open(input), Config{}, 0)
		if err != nil {
			t.Fatalf("%s: %v", rc.name, err)
		}
		if len(tokens) != 5 {
			t.Fatalf("%s: %d tokens, want 5", rc.name, len(tokens))
		}
		first, continuation := tokens[2], tokens[3]
		if first.kind != KindCharData || first.line != 2 || first.column != 1 {
			t.Fatalf("%s: first CDATA chunk = %v at %d:%d, want character data at 2:1", rc.name, first.kind, first.line, first.column)
		}
		wantColumn := len("<![CDATA[") + xmlInputBufferSize + 1
		if continuation.kind != KindCharData || continuation.data != "\ny" || continuation.line != 2 || continuation.column != wantColumn {
			t.Fatalf("%s: continuation = %q at %d:%d, want %q at 2:%d", rc.name, continuation.data, continuation.line, continuation.column, "\ny", wantColumn)
		}
	}
}
