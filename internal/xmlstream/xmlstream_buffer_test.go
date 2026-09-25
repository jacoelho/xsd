package xmlstream

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type scriptedRead struct {
	data []byte
	err  error
}

type scriptedReader struct {
	reads []scriptedRead
	calls int
}

func (r *scriptedReader) Read(dst []byte) (int, error) {
	r.calls++
	if len(r.reads) == 0 {
		return 0, io.EOF
	}
	read := &r.reads[0]
	n := copy(dst, read.data)
	read.data = read.data[n:]
	if len(read.data) != 0 {
		return n, nil
	}
	err := read.err
	r.reads = r.reads[1:]
	return n, err
}

func TestByteStreamOffsetExcludesOverflowProbe(t *testing.T) {
	readErr := errors.New("probe read failed")
	source := &scriptedReader{reads: []scriptedRead{{data: []byte("12345"), err: readErr}}}
	var stream byteStream
	stream.reset(source, 4)

	window, err := stream.ensure(1)
	if err != nil {
		t.Fatalf("ensure() error = %v, want deferred error", err)
	}
	if got := string(window); got != "1234" {
		t.Fatalf("buffered bytes = %q, want %q", got, "1234")
	}
	if got := stream.offset(); got != 0 {
		t.Fatalf("offset with buffered overflow probe = %d, want 0", got)
	}

	stream.consumeBuffered(len(window))
	if err := stream.fill(); !errors.Is(err, errXMLInputLimit) || !errors.Is(err, readErr) {
		t.Fatalf("fill() error = %v, want input limit and %v", err, readErr)
	}
	if err := stream.fill(); !errors.Is(err, errXMLInputLimit) || !errors.Is(err, readErr) {
		t.Fatalf("repeated fill() error = %v, want original joined error", err)
	}
	if source.calls != 1 {
		t.Fatalf("underlying reads = %d, want 1 after terminal error", source.calls)
	}
}

func TestByteStreamShortReadsAtExactLimit(t *testing.T) {
	source := &scriptedReader{reads: []scriptedRead{
		{data: []byte("12")},
		{data: []byte("34"), err: io.EOF},
	}}
	var stream byteStream
	stream.reset(source, 4)

	window, err := stream.ensure(4)
	if err != nil {
		t.Fatalf("ensure() error = %v, want nil with four admitted bytes", err)
	}
	if got := string(window); got != "1234" {
		t.Fatalf("buffered bytes = %q, want %q", got, "1234")
	}
	if got := stream.offset(); got != 0 {
		t.Fatalf("offset at exact limit = %d, want 0", got)
	}
	stream.consumeBuffered(len(window))
	terminalErr := stream.fill()
	if !errors.Is(terminalErr, io.EOF) {
		t.Fatalf("fill() error = %v, want EOF", terminalErr)
	}
	if errors.Is(terminalErr, errXMLInputLimit) {
		t.Fatalf("exact limit reported input limit: %v", terminalErr)
	}
}

func TestByteStreamBulkLFClosesPendingCRLF(t *testing.T) {
	var stream byteStream
	stream.reset(strings.NewReader("\r\n\nx"), 0)
	if b, err := stream.readByte(); err != nil || b != '\r' {
		t.Fatalf("first read = %q, %v; want CR", b, err)
	}
	window, err := stream.buffered()
	if err != nil {
		t.Fatal(err)
	}
	if len(window) < 2 || string(window[:2]) != "\n\n" {
		t.Fatalf("buffered LF run = %q, want two LFs", window)
	}
	stream.consumeBufferedLF(2)
	line, column := stream.pos()
	if line != 3 || column != 0 {
		t.Fatalf("position after CRLF plus LF = (%d,%d), want (3,0)", line, column)
	}
	if got := stream.offset(); got != 3 {
		t.Fatalf("offset after bulk LF = %d, want 3", got)
	}
}

func TestParserOffsetsAfterLimitCrossingFirstRead(t *testing.T) {
	var parser parserTestHarness
	if err := parser.reset(strings.NewReader("<r/>XYZ"), Config{Limits: Limits{MaxInputBytes: 6}}); err != nil {
		t.Fatal(err)
	}
	defer parser.detach()

	start, err := parser.next()
	if err != nil {
		t.Fatalf("start error = %v", err)
	}
	if start.StartOffset != 0 || start.EndOffset != 4 {
		t.Fatalf("start span = [%d,%d], want [0,4]", start.StartOffset, start.EndOffset)
	}

	end, err := parser.next()
	if err != nil {
		t.Fatalf("synthetic end error = %v", err)
	}
	if end.Kind != KindEnd || end.StartOffset != 0 || end.EndOffset != 4 {
		t.Fatalf("synthetic end = kind %v span [%d,%d], want kind %v span [0,4]", end.Kind, end.StartOffset, end.EndOffset, KindEnd)
	}
	if _, err := parser.next(); !errors.Is(err, errXMLInputLimit) {
		t.Fatalf("terminal error = %v, want input limit", err)
	}
}

func TestParserOffsetsAfterLimitCrossingRefill(t *testing.T) {
	const suffix = "<x/>Y"
	for _, test := range []struct {
		name   string
		prefix string
		bom    int
	}{
		{name: "without BOM", prefix: "<r/>AB"},
		{name: "with BOM", prefix: "\uFEFF<r/>AB", bom: utf8BOMLen},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := &scriptedReader{reads: []scriptedRead{
				{data: []byte(test.prefix)},
				{data: []byte(suffix)},
			}}
			var parser parserTestHarness
			limit := int64(len(test.prefix) + len("<x/>"))
			if err := parser.reset(source, Config{Limits: Limits{MaxInputBytes: limit}}); err != nil {
				t.Fatal(err)
			}
			defer parser.detach()

			start, err := parser.next()
			if err != nil {
				t.Fatalf("first start error = %v", err)
			}
			wantStart := test.bom
			if start.StartOffset != wantStart || start.EndOffset != wantStart+4 {
				t.Fatalf("first start span = [%d,%d], want [%d,%d]", start.StartOffset, start.EndOffset, wantStart, wantStart+4)
			}

			end, err := parser.next()
			if err != nil {
				t.Fatalf("first synthetic end error = %v", err)
			}
			if end.Kind != KindEnd || end.StartOffset != wantStart || end.EndOffset != wantStart+4 {
				t.Fatalf("first synthetic end = kind %v span [%d,%d], want kind %v span [%d,%d]", end.Kind, end.StartOffset, end.EndOffset, KindEnd, wantStart, wantStart+4)
			}

			text, err := parser.next()
			if err != nil {
				t.Fatalf("character data error = %v", err)
			}
			if text.Kind != KindCharData || string(text.Data) != "AB" || text.StartOffset != wantStart+4 || text.EndOffset != len(test.prefix) {
				t.Fatalf("character data = kind %v data %q span [%d,%d], want kind %v data %q span [%d,%d]", text.Kind, text.Data, text.StartOffset, text.EndOffset, KindCharData, "AB", wantStart+4, len(test.prefix))
			}

			secondStart, err := parser.next()
			if err != nil {
				t.Fatalf("refilled start error = %v", err)
			}
			wantRefillStart := len(test.prefix)
			if secondStart.StartOffset != wantRefillStart || secondStart.EndOffset != wantRefillStart+4 {
				t.Fatalf("refilled start span = [%d,%d], want [%d,%d]", secondStart.StartOffset, secondStart.EndOffset, wantRefillStart, wantRefillStart+4)
			}

			secondEnd, err := parser.next()
			if err != nil {
				t.Fatalf("refilled synthetic end error = %v", err)
			}
			if secondEnd.Kind != KindEnd || secondEnd.StartOffset != wantRefillStart || secondEnd.EndOffset != wantRefillStart+4 {
				t.Fatalf("refilled synthetic end = kind %v span [%d,%d], want kind %v span [%d,%d]", secondEnd.Kind, secondEnd.StartOffset, secondEnd.EndOffset, KindEnd, wantRefillStart, wantRefillStart+4)
			}

			if _, err := parser.next(); !errors.Is(err, errXMLInputLimit) {
				t.Fatalf("terminal parser error = %v, want input limit", err)
			}
			if source.calls != 2 {
				t.Fatalf("underlying reads after refill limit = %d, want 2", source.calls)
			}
		})
	}
}

func TestParserLimitCrossingPreservesJoinedReaderCause(t *testing.T) {
	readErr := errors.New("reader failed at limit")
	const prefix = "<r/>AB"
	source := &scriptedReader{reads: []scriptedRead{
		{data: []byte(prefix)},
		{data: []byte("<x/>Y"), err: readErr},
	}}
	var parser parserTestHarness
	if err := parser.reset(source, Config{Limits: Limits{MaxInputBytes: int64(len(prefix) + 4)}}); err != nil {
		t.Fatal(err)
	}
	defer parser.detach()

	for range 5 {
		if _, err := parser.next(); err != nil {
			t.Fatalf("token before terminal error = %v", err)
		}
	}
	if _, err := parser.next(); !errors.Is(err, errXMLInputLimit) || !errors.Is(err, readErr) || IsOnlyEOF(err) {
		t.Fatalf("terminal error = %v, want joined input limit and non-EOF cause", err)
	}
	if _, err := parser.next(); !errors.Is(err, errXMLInputLimit) || !errors.Is(err, readErr) {
		t.Fatalf("repeated terminal error = %v, want original joined error", err)
	}
	if source.calls != 2 {
		t.Fatalf("underlying reads after joined error = %d, want 2", source.calls)
	}
}

func TestParserOffsetsAfterBOMWithoutLimit(t *testing.T) {
	var parser parserTestHarness
	if err := parser.reset(strings.NewReader("\uFEFF<r/>"), Config{}); err != nil {
		t.Fatal(err)
	}
	defer parser.detach()

	tok, err := parser.next()
	if err != nil {
		t.Fatal(err)
	}
	if tok.StartOffset != utf8BOMLen || tok.EndOffset != utf8BOMLen+4 {
		t.Fatalf("BOM start span = [%d,%d], want [%d,%d]", tok.StartOffset, tok.EndOffset, utf8BOMLen, utf8BOMLen+4)
	}
}
