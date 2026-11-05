package xmlstream

import (
	"errors"
	"fmt"
	"io"
	"slices"
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

func TestReaderSkippedProcessingInstructionErrors(t *testing.T) {
	readErr := errors.New("PI source failed")
	for _, test := range []struct {
		name    string
		input   string
		readErr error
		message string
		cause   error
		wantEOF bool
	}{
		{name: "pending question EOF", input: "<r><?p ?", message: "unexpected EOF in processing instruction"},
		{name: "reader error", input: "<r><?p ?", readErr: readErr, cause: readErr},
		{name: "joined EOF and reader error", input: "<r><?p ?", readErr: errors.Join(io.EOF, readErr), cause: readErr, wantEOF: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var reader Reader
			input := &scriptedReader{reads: []scriptedRead{{data: []byte(test.input), err: test.readErr}}}
			if err := reader.Reset(input, Config{}); err != nil {
				t.Fatal(err)
			}
			_ = mustNextToken(t, &reader)
			if _, _, err := reader.Start(); err != nil {
				t.Fatal(err)
			}
			token, err := reader.Next()
			boundary, ok := errors.AsType[*Error](err)
			if token != nil || !ok || boundary.Kind != ErrorSyntax {
				t.Fatalf("Next() = %v, %v, want syntax failure", token, err)
			}
			if test.message != "" && err.Error() != test.message {
				t.Fatalf("error = %q, want %q", err, test.message)
			}
			if test.cause != nil && !errors.Is(err, test.cause) {
				t.Fatalf("error = %v, lost cause %v", err, test.cause)
			}
			if test.wantEOF && !errors.Is(err, io.EOF) {
				t.Fatalf("error = %v, lost joined EOF cause", err)
			}
			reader.Detach()
			if err := reader.Reset(strings.NewReader("<ok/>"), Config{}); err != nil {
				t.Fatal(err)
			}
			if token := mustNextToken(t, &reader); token.Kind != KindStart || token.Start.Name.Local != "ok" {
				t.Fatalf("reset token = %+v, want ok", token)
			}
		})
	}
}

func TestReaderSkippedProcessingInstructionLimits(t *testing.T) {
	input := "<r><?p " + strings.Repeat("?xé", 20000) + "?></r>"
	for _, emit := range []bool{false, true} {
		var reader Reader
		if err := reader.Reset(strings.NewReader(input), Config{EmitPI: emit, Limits: Limits{MaxTokenBytes: 2}}); err != nil {
			t.Fatal(err)
		}
		_ = mustNextToken(t, &reader)
		frame, _, err := reader.Start()
		if err != nil {
			t.Fatal(err)
		}
		token, err := reader.Next()
		if emit {
			if token != nil || !IsTokenLimit(err) {
				t.Fatalf("emitted PI = %v, %v, want token limit", token, err)
			}
			continue
		}
		if err != nil || token.Kind != KindEnd || token.End.Name.Local != "r" {
			t.Fatalf("skipped PI = %v, %v, want root end", token, err)
		}
		if err := reader.End(frame); err != nil {
			t.Fatal(err)
		}
	}
	for _, limit := range []int64{int64(len(input)), int64(len(input) - 1)} {
		var reader Reader
		if err := reader.Reset(strings.NewReader(input), Config{Limits: Limits{MaxInputBytes: limit}}); err != nil {
			t.Fatal(err)
		}
		_ = mustNextToken(t, &reader)
		frame, _, err := reader.Start()
		if err != nil {
			t.Fatal(err)
		}
		token, err := reader.Next()
		if limit < int64(len(input)) {
			boundary, ok := errors.AsType[*Error](err)
			if token != nil || !ok || boundary.Kind != ErrorLimit || !IsInputLimit(err) {
				t.Fatalf("limited PI = %v, %v, want input limit", token, err)
			}
			if _, nextErr := reader.Next(); !IsInputLimit(nextErr) {
				t.Fatalf("repeated Next() = %v, lost input limit", nextErr)
			}
			continue
		}
		if err != nil || token.Kind != KindEnd {
			t.Fatalf("exact input limit = %v, %v, want root end", token, err)
		}
		if err := reader.End(frame); err != nil {
			t.Fatal(err)
		}
		if _, err := reader.Next(); err != io.EOF { //nolint:errorlint // Token-boundary EOF is the exact sentinel.
			t.Fatalf("exact input limit EOF = %v", err)
		}
		if err := reader.Complete(); err != nil {
			t.Fatal(err)
		}
	}
}

// requestRecorder records the length of every read request it receives.
type requestRecorder struct {
	src      io.Reader
	requests []int
}

func (r *requestRecorder) Read(p []byte) (int, error) {
	r.requests = append(r.requests, len(p))
	return r.src.Read(p)
}

// Every read asks for a bounded, non-empty window, and with MaxInputBytes L no
// request exceeds the L+1 overflow probe. An input larger than the window
// reaches the 65536-byte maximum request; smaller inputs may use less.
func TestInputReadRequestsAreBounded(t *testing.T) {
	t.Parallel()

	long := "<r>" + strings.Repeat("a", 70000) + "</r>"
	for _, test := range []struct {
		name      string
		input     string
		limit     int64
		wantLimit bool
		wantMax   int
	}{
		{name: "empty_root", input: "<r/>"},
		{name: "two_windows", input: long, wantMax: 65536},
		{name: "small_limit", input: long, limit: 100, wantLimit: true, wantMax: 101},
		{name: "limit_below_input", input: long, limit: 70000, wantLimit: true, wantMax: 65536},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := &requestRecorder{src: strings.NewReader(test.input)}
			_, err := recordReader(recorder, Config{Limits: Limits{MaxInputBytes: test.limit}}, 0)
			if test.wantLimit != IsInputLimit(err) || (!test.wantLimit && err != nil) {
				t.Fatalf("lifecycle error = %v, want input limit %t", err, test.wantLimit)
			}
			if len(recorder.requests) == 0 {
				t.Fatal("reader received no requests")
			}
			for i, n := range recorder.requests {
				if n <= 0 || n > 65536 {
					t.Fatalf("request %d length = %d, want (0, 65536]", i, n)
				}
				if test.limit > 0 && int64(n) > test.limit+1 {
					t.Fatalf("request %d length = %d, exceeds MaxInputBytes+1 = %d", i, n, test.limit+1)
				}
			}
			if got := slices.Max(recorder.requests); test.wantMax != 0 && got != test.wantMax {
				t.Fatalf("largest request = %d, want %d", got, test.wantMax)
			}
		})
	}
}

// A text run is one token regardless of input-buffer boundaries, and
// MaxTokenBytes n admits exactly n bytes.
func TestTextRunIsOneTokenAtAnyBufferBoundary(t *testing.T) {
	t.Parallel()

	for _, n := range []int{4095, 4096, 4097, 65535, 65536, 65537, 131077} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			document := "<r>" + strings.Repeat("a", n) + "</r>"
			for _, rc := range []readerCase{wholeInput, oneByteInput} {
				tokens, err := recordReader(rc.open(document), Config{Limits: Limits{MaxTokenBytes: int64(n - 1)}}, 0)
				if !IsTokenLimit(err) {
					t.Fatalf("%s: MaxTokenBytes=%d error = %v, want token limit", rc.name, n-1, err)
				}
				if got := textTokenLengths(tokens); len(got) != 0 {
					t.Fatalf("%s: MaxTokenBytes=%d text tokens = %v, want none", rc.name, n-1, got)
				}
				for _, limit := range []int64{int64(n), int64(n + 1), 0} {
					tokens, err := recordReader(rc.open(document), Config{Limits: Limits{MaxTokenBytes: limit}}, 0)
					if err != nil {
						t.Fatalf("%s: MaxTokenBytes=%d error = %v", rc.name, limit, err)
					}
					if got := textTokenLengths(tokens); len(got) != 1 || got[0] != n {
						t.Fatalf("%s: MaxTokenBytes=%d text tokens = %v, want [%d]", rc.name, limit, got, n)
					}
				}
			}
		})
	}
}

func textTokenLengths(tokens []recordedToken) []int {
	var lengths []int
	for _, tok := range tokens {
		if tok.kind == KindCharData && tok.textKind == CharacterDataText {
			lengths = append(lengths, len(tok.data))
		}
	}
	return lengths
}
