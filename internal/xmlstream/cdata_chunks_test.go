package xmlstream

import (
	"slices"
	"strings"
	"testing"
)

// cdataBoundaryChunks is the CDATA chunk sequence for a section of exactly
// 65536 bytes: the full chunk, then an empty final chunk at the terminator.
var cdataBoundaryChunks = []int{65536, 0}

var cdataReaderCases = []readerCase{wholeInput, oneByteInput, halfInput, dataErrInput, chunkedInput(4096)}

// CDATA sections are emitted in chunks so one token never retains an unbounded
// section. The chunk lengths are a deliberate characterization of the 64 KiB
// chunk size: a chunk ends after the first indivisible step (one character,
// after flushing pending ']') whose cumulative LF-normalized length reaches
// 65536, so a non-final chunk may overshoot by up to five bytes. They must not
// depend on how the input reader splits its bytes.
func TestCDATAChunksAreReaderIndependent(t *testing.T) {
	t.Parallel()

	x := func(n int) string { return strings.Repeat("x", n) }
	for _, test := range []struct {
		name    string
		payload string
		chunks  []int
	}{
		{name: "below", payload: x(65535), chunks: []int{65535}},
		{name: "exact", payload: x(65536), chunks: cdataBoundaryChunks},
		{name: "above", payload: x(65537), chunks: []int{65536, 1}},
		{name: "two_full", payload: x(131072 + 17), chunks: []int{65536, 65536, 17}},
		{name: "bracket_completes", payload: x(65535) + "]", chunks: []int{65536, 0}},
		{name: "brackets_complete", payload: x(65534) + "]]", chunks: []int{65536, 0}},
		{name: "bracket_after_full", payload: x(65536) + "]", chunks: []int{65536, 1}},
		{name: "brackets_cross", payload: x(65535) + "]]", chunks: []int{65536, 1}},
		{name: "bracket_gt", payload: x(65535) + "]>", chunks: []int{65537, 0}},
		{name: "brackets_x", payload: x(65535) + "]]x", chunks: []int{65538, 0}},
		{name: "two_byte_rune_crosses", payload: x(65535) + "é", chunks: []int{65537, 0}},
		{name: "two_byte_rune_completes", payload: x(65534) + "é", chunks: []int{65536, 0}},
		{name: "four_byte_rune_crosses", payload: x(65535) + "😀", chunks: []int{65539, 0}},
		{name: "four_byte_rune_overshoots", payload: x(65533) + "😀", chunks: []int{65537, 0}},
		{name: "crlf_completes", payload: x(65535) + "\r\n", chunks: []int{65536, 0}},
		{name: "crlf_below", payload: x(65534) + "\r\n", chunks: []int{65535}},
		{name: "cr_completes", payload: x(65535) + "\r", chunks: []int{65536, 0}},
		// Straddles a 4 KiB stream offset with runes, bracket runs and CRLF.
		{name: "mixed_9KiB", payload: strings.Repeat("é]]x\r\n", 1300), chunks: []int{7800}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := "<root><![CDATA[" + test.payload + "]]></root>"
			normalized := strings.ReplaceAll(strings.ReplaceAll(test.payload, "\r\n", "\n"), "\r", "\n")
			var reference []recordedToken
			for _, rc := range cdataReaderCases {
				tokens, err := recordReader(rc.open(document), Config{}, 0)
				if err != nil {
					t.Fatalf("%s: lifecycle error = %v", rc.name, err)
				}
				cdata := cdataTokens(tokens)
				if reference == nil {
					reference = cdata
					requireCDATAChunks(t, cdata, test.chunks, normalized, len(document)-len("</root>"))
					continue
				}
				if !slices.Equal(cdata, reference) {
					t.Fatalf("%s: CDATA tokens differ from %s reader", rc.name, cdataReaderCases[0].name)
				}
			}
		})
	}
}

// MaxTokenBytes bounds each CDATA chunk, not the whole section.
func TestCDATAChunkTokenLimit(t *testing.T) {
	t.Parallel()

	x := func(n int) string { return strings.Repeat("x", n) }
	for _, test := range []struct {
		name    string
		payload string
		pass    int64
		fail    int64
	}{
		{name: "above", payload: x(65537), pass: 65536, fail: 65535},
		{name: "two_full", payload: x(131072 + 17), pass: 65536, fail: 65535},
		{name: "overshoot", payload: x(65535) + "]>", pass: 65537, fail: 65536},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			document := "<root><![CDATA[" + test.payload + "]]></root>"
			for _, rc := range cdataReaderCases {
				if _, err := recordReader(rc.open(document), Config{Limits: Limits{MaxTokenBytes: test.pass}}, 0); err != nil {
					t.Fatalf("%s: MaxTokenBytes=%d error = %v", rc.name, test.pass, err)
				}
				if _, err := recordReader(rc.open(document), Config{Limits: Limits{MaxTokenBytes: test.fail}}, 0); !IsTokenLimit(err) {
					t.Fatalf("%s: MaxTokenBytes=%d error = %v, want token limit", rc.name, test.fail, err)
				}
			}
		})
	}
}

func cdataTokens(tokens []recordedToken) []recordedToken {
	var cdata []recordedToken
	for _, tok := range tokens {
		if tok.kind == KindCharData && tok.textKind == CharacterDataCDATA {
			cdata = append(cdata, tok)
		}
	}
	return cdata
}

// requireCDATAChunks checks the chunk contract: the expected lengths, content
// equal to the normalized payload, contiguous source offsets ending after the
// terminator, and bounded overshoot. Continuation positions are not pinned.
func requireCDATAChunks(t *testing.T, cdata []recordedToken, want []int, normalized string, terminatorEnd int) {
	t.Helper()
	lengths := make([]int, len(cdata))
	var content strings.Builder
	for i, tok := range cdata {
		lengths[i] = len(tok.data)
		content.WriteString(tok.data)
		if i > 0 && tok.startOffset != cdata[i-1].endOffset {
			t.Fatalf("chunk %d StartOffset = %d, previous EndOffset = %d", i, tok.startOffset, cdata[i-1].endOffset)
		}
		if i < len(cdata)-1 && (lengths[i] < 65536 || lengths[i] > 65541) {
			t.Fatalf("non-final chunk %d length = %d, want [65536, 65541]", i, lengths[i])
		}
	}
	if !slices.Equal(lengths, want) {
		t.Fatalf("chunk lengths = %v, want %v", lengths, want)
	}
	if content.String() != normalized {
		t.Fatalf("chunk content differs from the LF-normalized payload (%d vs %d bytes)", content.Len(), len(normalized))
	}
	if last := cdata[len(cdata)-1].endOffset; last != terminatorEnd {
		t.Fatalf("last chunk EndOffset = %d, want %d", last, terminatorEnd)
	}
}
