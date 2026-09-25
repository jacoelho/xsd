package xmlstream

import (
	"errors"
	"io"
	"strings"
	"testing"
)

// parserTestHarness owns the tokenizer and its caches while exposing the raw
// tokenizer path used by parser-only tests and benchmarks. Reader.Next remains
// the production document-admission boundary.
type parserTestHarness struct {
	reader Reader
}

func (p *parserTestHarness) reset(input io.Reader, config Config) error {
	return p.reader.resetParser(input, config)
}

func (p *parserTestHarness) next() (Token, error) {
	var token Token
	err := p.reader.parser.next(&token)
	return token, err
}

func (p *parserTestHarness) detach() {
	p.reader.Detach()
}

func TestParserTokenizesWithoutDocumentAdmission(t *testing.T) {
	var parser parserTestHarness
	if err := parser.reset(strings.NewReader(`<r><p:c/></r>`), Config{}); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	defer parser.detach()

	var got []TokenKind
	for {
		tok, err := parser.next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next() error = %v", err)
		}
		got = append(got, tok.Kind)
	}
	want := []TokenKind{KindStart, KindStart, KindEnd, KindEnd}
	if len(got) != len(want) {
		t.Fatalf("token kinds = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token kind %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestParserReusesReaderStorage(t *testing.T) {
	const input = `<r xmlns:p="urn:test"><p:c/></r>`
	var reader Reader
	if err := reader.Reset(strings.NewReader(input), Config{}); err != nil {
		t.Fatalf("Reader.Reset() error = %v", err)
	}
	if err := drainReader(&reader); err != nil {
		t.Fatalf("drainReader() error = %v", err)
	}

	if err := reader.parser.reset(strings.NewReader(input), &reader.names, &reader.values, Config{}); err != nil {
		t.Fatalf("parser.reset() error = %v", err)
	}
	if err := reader.parser.next(&reader.token); err != nil {
		t.Fatalf("parser.next() error = %v", err)
	}
}

func TestParserRejectsEmptyEntityReference(t *testing.T) {
	var parser parserTestHarness
	if err := parser.reset(strings.NewReader(`<r>&;</r>`), Config{}); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	defer parser.detach()

	if _, err := parser.next(); err != nil {
		t.Fatalf("parser.next() start error = %v", err)
	}
	if _, err := parser.next(); err == nil {
		t.Fatal("parser.next() accepted empty entity reference")
	}
}

func TestParserCoalescesLFRunAndPreservesFollowingToken(t *testing.T) {
	var parser parserTestHarness
	if err := parser.reset(strings.NewReader("<r>a\n\nb</r>"), Config{}); err != nil {
		t.Fatal(err)
	}
	defer parser.detach()

	start, err := parser.next()
	if err != nil || start.Kind != KindStart {
		t.Fatalf("root token = %+v, %v", start, err)
	}
	text, err := parser.next()
	if err != nil {
		t.Fatal(err)
	}
	if text.Kind != KindCharData || text.TextKind != CharacterDataText || string(text.Data) != "a\n\nb" {
		t.Fatalf("text token = kind %v origin %v data %q, want text/text %q", text.Kind, text.TextKind, text.Data, "a\n\nb")
	}
	end, err := parser.next()
	if err != nil {
		t.Fatal(err)
	}
	if end.Kind != KindEnd || end.End.Name.Local != "r" {
		t.Fatalf("end token = %+v, want </r>", end)
	}
}

func TestParserASCIITextPreservesOriginAndSpan(t *testing.T) {
	var parser parserTestHarness
	if err := parser.reset(strings.NewReader("<r>text</r>"), Config{}); err != nil {
		t.Fatal(err)
	}
	defer parser.detach()

	if _, err := parser.next(); err != nil {
		t.Fatal(err)
	}
	text, err := parser.next()
	if err != nil {
		t.Fatal(err)
	}
	if text.Kind != KindCharData || text.TextKind != CharacterDataText || string(text.Data) != "text" {
		t.Fatalf("text token = kind %v origin %v data %q, want text/text %q", text.Kind, text.TextKind, text.Data, "text")
	}
	if text.StartOffset != 3 || text.EndOffset != 7 {
		t.Fatalf("text span = [%d,%d], want [3,7]", text.StartOffset, text.EndOffset)
	}
	end, err := parser.next()
	if err != nil {
		t.Fatal(err)
	}
	if end.StartOffset != 7 || end.EndOffset != 11 {
		t.Fatalf("end span = [%d,%d], want [7,11]", end.StartOffset, end.EndOffset)
	}
}

func TestParserASCIITextDefersReaderErrorAfterDelimiter(t *testing.T) {
	readErr := errors.New("text source failed after bytes")
	source := &scriptedReader{reads: []scriptedRead{{data: []byte("<r>text</r>"), err: readErr}}}
	var parser parserTestHarness
	if err := parser.reset(source, Config{}); err != nil {
		t.Fatal(err)
	}
	defer parser.detach()

	if _, err := parser.next(); err != nil {
		t.Fatal(err)
	}
	text, err := parser.next()
	if err != nil || string(text.Data) != "text" {
		t.Fatalf("text token = %+v, %v; want text before deferred reader error", text, err)
	}
	if _, err := parser.next(); err != nil {
		t.Fatalf("end token = %v", err)
	}
	if _, err := parser.next(); !errors.Is(err, readErr) || IsOnlyEOF(err) {
		t.Fatalf("terminal parser error = %v, want non-EOF reader cause", err)
	}
}

func TestParserASCIITextReturnsBeforeTerminalEOF(t *testing.T) {
	source := &scriptedReader{reads: []scriptedRead{{data: []byte("<r>text"), err: io.EOF}}}
	var parser parserTestHarness
	if err := parser.reset(source, Config{}); err != nil {
		t.Fatal(err)
	}
	defer parser.detach()

	if _, err := parser.next(); err != nil {
		t.Fatal(err)
	}
	text, err := parser.next()
	if err != nil || string(text.Data) != "text" {
		t.Fatalf("text token = %+v, %v; want text before terminal EOF", text, err)
	}
	if text.StartOffset != 3 || text.EndOffset != 7 {
		t.Fatalf("text span = [%d,%d], want [3,7]", text.StartOffset, text.EndOffset)
	}
	if _, err := parser.next(); !errors.Is(err, io.EOF) {
		t.Fatalf("terminal parser error = %v, want EOF", err)
	}
}

func TestParserASCIITextAcrossRefillPreservesSpan(t *testing.T) {
	input := "<r>" + strings.Repeat("a", xmlInputBufferSize) + "</r>"
	var parser parserTestHarness
	if err := parser.reset(strings.NewReader(input), Config{}); err != nil {
		t.Fatal(err)
	}
	defer parser.detach()

	if _, err := parser.next(); err != nil {
		t.Fatal(err)
	}
	text, err := parser.next()
	if err != nil {
		t.Fatal(err)
	}
	if text.Kind != KindCharData || len(text.Data) != xmlInputBufferSize || string(text.Data[:4]) != "aaaa" {
		t.Fatalf("refilled text token = kind %v length %d prefix %q, want text/%d/aaaa", text.Kind, len(text.Data), text.Data[:min(len(text.Data), 4)], xmlInputBufferSize)
	}
	if text.StartOffset != 3 || text.EndOffset != 3+xmlInputBufferSize {
		t.Fatalf("refilled text span = [%d,%d], want [%d,%d]", text.StartOffset, text.EndOffset, 3, 3+xmlInputBufferSize)
	}
}

func TestParserASCIITextLimitChargesBorrowedBytes(t *testing.T) {
	var parser parserTestHarness
	if err := parser.reset(strings.NewReader("<r>text</r>"), Config{Limits: Limits{MaxTokenBytes: 3}}); err != nil {
		t.Fatal(err)
	}
	defer parser.detach()

	if _, err := parser.next(); err != nil {
		t.Fatal(err)
	}
	if _, err := parser.next(); !IsTokenLimit(err) {
		t.Fatalf("text token error = %v, want token limit", err)
	}
}

func drainReader(reader *Reader) error {
	var handles []Handle
	for {
		tok, err := reader.Next()
		if IsOnlyEOF(err) {
			return reader.Complete()
		}
		if err != nil {
			return err
		}
		switch tok.Kind {
		case KindStart:
			handle, _, err := reader.Start()
			if err != nil {
				return err
			}
			handles = append(handles, handle)
		case KindEnd:
			if len(handles) == 0 {
				return errors.New("unexpected end in test helper")
			}
			handle := handles[len(handles)-1]
			if err := reader.MatchEnd(handle); err != nil {
				return err
			}
			if err := reader.CommitEnd(handle); err != nil {
				return err
			}
			handles = handles[:len(handles)-1]
		case KindCharData, KindDirective, KindComment, KindPI:
			// These tokens do not change the retained element-handle stack.
		}
	}
}
