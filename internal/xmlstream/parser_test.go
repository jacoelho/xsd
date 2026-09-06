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
