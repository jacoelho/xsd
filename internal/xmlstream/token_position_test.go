package xmlstream

import (
	"errors"
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

func TestCDATAContinuationChunkStartsAtPendingBrackets(t *testing.T) {
	t.Parallel()

	const prefix = "<r><![CDATA["
	input := prefix + strings.Repeat("x", xmlInputBufferSize-1) + "]]]y]]></r>"
	firstPending := len(prefix) + xmlInputBufferSize
	for _, rc := range []readerCase{wholeInput, chunkedInput(4096)} {
		tokens, err := recordReader(rc.open(input), Config{}, 0)
		if err != nil {
			t.Fatalf("%s: %v", rc.name, err)
		}
		if len(tokens) != 4 {
			t.Fatalf("%s: %d tokens, want 4", rc.name, len(tokens))
		}
		first, continuation := tokens[1], tokens[2]
		if continuation.data != "]]y" || continuation.line != 1 || continuation.column != firstPending+1 {
			t.Fatalf("%s: continuation = %q at %d:%d, want %q at 1:%d", rc.name, continuation.data, continuation.line, continuation.column, "]]y", firstPending+1)
		}
		if continuation.startOffset != firstPending || first.endOffset != firstPending {
			t.Fatalf("%s: first chunk ends at %d, continuation starts at %d, want both %d", rc.name, first.endOffset, continuation.startOffset, firstPending)
		}
	}
}

func TestContentPosSkipsLeadingWhitespaceAfterDecoding(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, text   string
		line, column int
	}{
		{"text", "x", 1, 4},
		{"after spaces and tab", " \t x", 1, 7},
		{"after lf", "\nx", 2, 1},
		{"after cr", "\rx", 2, 1},
		{"after crlf", "\r\nx", 2, 1},
		{"after blank lines and spaces", "\n\n\n   x", 4, 4},
		{"after mixed line breaks", "\r\n\n\r\t x", 4, 3},
		{"after whitespace reference", "\r\n &#32;x", 2, 7},
		{"after line feed reference", "&#10;x", 1, 9},
		{"after whitespace reference and line break", "&#32;\n x", 2, 2},
		{"after several whitespace references", "&#32; &#9;\n&#xA;  x&amp;", 2, 8},
		{"reference decoding to content", "\n &amp;", 2, 2},
		{"reference decoding to content after whitespace reference", "&#32;&#x41;", 1, 9},
		{"text before reference", "\n y&amp;", 2, 2},
		{"whitespace only", "\n  ", 1, 4},
		{"whitespace references only", "&#32;\n&#10;", 1, 4},
		{"cdata", "<![CDATA[\n x]]>", 2, 2},
		{"cdata content first", "<![CDATA[x]]>", 1, 13},
		{"whitespace cdata", "<![CDATA[\n ]]>", 1, 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, rc := range []readerCase{wholeInput, oneByteInput, chunkedInput(2)} {
				var reader Reader
				if err := reader.Reset(rc.open("<r>"+test.text+"</r>"), Config{}); err != nil {
					t.Fatal(err)
				}
				if tok := mustNextToken(t, &reader); tok.Kind != KindStart {
					t.Fatalf("%s: first token kind = %v, want start", rc.name, tok.Kind)
				}
				if _, _, err := reader.Start(); err != nil {
					t.Fatal(err)
				}
				tok := mustNextToken(t, &reader)
				if line, column := reader.ContentPos(); tok.Kind != KindCharData || line != test.line || column != test.column {
					t.Fatalf("%s: ContentPos() = %d:%d for kind %v, want %d:%d", rc.name, line, column, tok.Kind, test.line, test.column)
				}
			}
		})
	}
}

func TestOutsideRootFailureLocatedAtFirstContentByte(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name, trailer string
		cause         error
		line, column  int
	}{
		{"text", "x", ErrTextOutsideRoot, 1, 5},
		{"text after lf", "\nx", ErrTextOutsideRoot, 2, 1},
		{"text after blank lines and spaces", "\n\n\n   x", ErrTextOutsideRoot, 4, 4},
		{"text after crlf and cr", "\r\n\r\tx", ErrTextOutsideRoot, 3, 2},
		{"reference after whitespace", "\r\n  &#32;", ErrReferenceOutsideRoot, 2, 3},
		{"whitespace reference before text", "\n &#32;x", ErrReferenceOutsideRoot, 2, 2},
		{"text before reference", "\n x&amp;", ErrTextOutsideRoot, 2, 2},
		{"cdata after lf", "\n<![CDATA[x]]>", ErrCDATOutsideRoot, 2, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			for _, rc := range []readerCase{wholeInput, oneByteInput, chunkedInput(2)} {
				var reader Reader
				if err := reader.Reset(rc.open("<r/>"+test.trailer), Config{}); err != nil {
					t.Fatal(err)
				}
				if tok := mustNextToken(t, &reader); tok.Kind != KindStart {
					t.Fatalf("%s: first token kind = %v, want start", rc.name, tok.Kind)
				}
				root, _, err := reader.Start()
				if err != nil {
					t.Fatal(err)
				}
				_ = mustNextToken(t, &reader)
				if err = reader.End(root); err != nil {
					t.Fatal(err)
				}
				for err == nil {
					_, err = reader.Next()
				}
				if !errors.Is(err, test.cause) {
					t.Fatalf("%s: error = %v, want %v", rc.name, err, test.cause)
				}
				if line, column := reader.ErrorPos(err); line != test.line || column != test.column {
					t.Fatalf("%s: ErrorPos() = %d:%d, want %d:%d", rc.name, line, column, test.line, test.column)
				}
			}
		})
	}
}
