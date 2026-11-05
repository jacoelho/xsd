package xmlstream

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

// FuzzReader exercises the public Reader lifecycle against arbitrary XML and
// requires the token stream and final outcome to be independent of how the
// input reader splits its bytes. chunks cycles through read lengths; an empty
// chunks reads the whole input and is the reference.
func FuzzReader(f *testing.F) {
	for _, seed := range []string{
		`<root/>`,
		`<?xml version="1.0"?><root a="&amp;">text</root>`,
		`<root><![CDATA[x<y]]><!--c--><?pi v?></root>`,
		`<root><a/><b attr="value">text</b></root>`,
	} {
		f.Add(seed, []byte(nil))
		f.Add(seed, []byte{1})
	}
	for _, seed := range []string{
		`<a:b c:d="1"></a:b>`,
		`<π:名 é="1"/>`,
		`<a:b:c/>`,
		`<1a/>`,
	} {
		for _, chunk := range []byte{1, 2, 3} {
			f.Add(seed, []byte{chunk})
		}
	}
	f.Fuzz(func(t *testing.T, input string, chunks []byte) {
		if len(input) > 4096 {
			t.Skip()
		}
		sizes := make([]int, len(chunks))
		for i, size := range chunks {
			sizes[i] = int(size)
		}
		whole, wholeErr := recordReader(strings.NewReader(input), Config{}, 4096)
		chunked, chunkedErr := recordReader(newChunkedReader(strings.NewReader(input), sizes...), Config{}, 4096)
		// Config{} sets no token or input limit, and chunkedReader never makes
		// enough empty reads to trip no-progress, so every outcome is compared.
		for _, err := range []error{wholeErr, chunkedErr} {
			if errors.Is(err, errRecordedTokenLimit) {
				t.Fatalf("Reader.Next emitted more than 4096 tokens for %d input bytes", len(input))
			}
		}
		if got, want := readerOutcome(chunkedErr), readerOutcome(wholeErr); got != want {
			t.Fatalf("chunked outcome = %s, whole outcome = %s", got, want)
		}
		if len(chunked) != len(whole) {
			t.Fatalf("chunked tokens = %d, whole tokens = %d", len(chunked), len(whole))
		}
		for i := range whole {
			if chunked[i] != whole[i] {
				t.Fatalf("token %d chunked = %+v, whole = %+v", i, chunked[i], whole[i])
			}
		}
	})
}

// recordedToken is a comparable copy of one Reader.Next token. Borrowed bytes
// are copied before the next read invalidates them.
type recordedToken struct {
	data        string
	directive   string
	attrs       string
	name        xml.Name
	startOffset int
	endOffset   int
	line        int
	column      int
	kind        TokenKind
	textKind    CharacterDataKind
}

var errRecordedTokenLimit = errors.New("recorded token limit exceeded")

// recordReader drives the user lifecycle (Reset, Next, Start, End, Complete)
// and returns every token produced before the first error. maxTokens <= 0
// does not bound the token count.
func recordReader(input io.Reader, config Config, maxTokens int) ([]recordedToken, error) {
	var reader Reader
	if err := reader.Reset(input, config); err != nil {
		return nil, err
	}
	defer reader.Detach()
	var tokens []recordedToken
	var handles []Handle
	for {
		tok, err := reader.Next()
		if IsOnlyEOF(err) {
			return tokens, reader.Complete()
		}
		if err != nil {
			return tokens, err
		}
		if maxTokens > 0 && len(tokens) == maxTokens {
			return tokens, errRecordedTokenLimit
		}
		tokens = append(tokens, recordToken(tok))
		switch tok.Kind {
		case KindStart:
			handle, _, err := reader.Start()
			if err != nil {
				return tokens, err
			}
			handles = append(handles, handle)
		case KindEnd:
			if len(handles) == 0 {
				return tokens, errors.New("unexpected end in test helper")
			}
			if err := reader.End(handles[len(handles)-1]); err != nil {
				return tokens, err
			}
			handles = handles[:len(handles)-1]
		case KindCharData, KindDirective, KindComment, KindPI:
			// These tokens do not change the retained element-handle stack.
		}
	}
}

func recordToken(tok *Token) recordedToken {
	record := recordedToken{
		data:        string(tok.Data),
		directive:   string(tok.Directive),
		startOffset: tok.StartOffset,
		endOffset:   tok.EndOffset,
		line:        tok.Line,
		column:      tok.Column,
		kind:        tok.Kind,
		textKind:    tok.TextKind,
	}
	switch tok.Kind {
	case KindStart:
		record.name = tok.Start.Name
		var attrs strings.Builder
		for _, attr := range tok.Start.Attr {
			fmt.Fprintf(&attrs, "%s:%s=%q;", attr.Name.Space, attr.Name.Local, attr.Value)
		}
		record.attrs = attrs.String()
	case KindEnd:
		record.name = tok.End.Name
	case KindCharData, KindDirective, KindComment, KindPI:
		// Data and Directive above carry these kinds' content.
	}
	return record
}

// readerOutcome renders the observable parts of a lifecycle error.
func readerOutcome(err error) string {
	if err == nil {
		return "nil"
	}
	if xerr, ok := errors.AsType[*Error](err); ok {
		return fmt.Sprintf("kind=%d at %d:%d: %v", xerr.Kind, xerr.Line, xerr.Column, err)
	}
	return err.Error()
}

// chunkedReader serves at most sizes[i%len(sizes)] bytes on read i. Zero sizes
// produce empty reads, but never 100 in a row: the stream rejects that many as
// no progress, which would make the outcome depend on the chunk pattern.
type chunkedReader struct {
	src   io.Reader
	sizes []int
	reads int
	empty int
}

func newChunkedReader(src io.Reader, sizes ...int) *chunkedReader {
	return &chunkedReader{src: src, sizes: sizes}
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if len(r.sizes) == 0 {
		return r.src.Read(p)
	}
	size := r.sizes[r.reads%len(r.sizes)]
	r.reads++
	if size == 0 && r.empty < 99 {
		r.empty++
		return 0, nil
	}
	r.empty = 0
	return r.src.Read(p[:max(1, min(size, len(p)))])
}

// readerCase names one way of splitting the same input into reads.
type readerCase struct {
	name string
	open func(string) io.Reader
}

var (
	wholeInput   = readerCase{name: "whole", open: func(s string) io.Reader { return strings.NewReader(s) }}
	oneByteInput = readerCase{name: "one_byte", open: func(s string) io.Reader { return iotest.OneByteReader(strings.NewReader(s)) }}
	halfInput    = readerCase{name: "half", open: func(s string) io.Reader { return iotest.HalfReader(strings.NewReader(s)) }}
	dataErrInput = readerCase{name: "data_err", open: func(s string) io.Reader { return iotest.DataErrReader(strings.NewReader(s)) }}
)

func chunkedInput(size int) readerCase {
	return readerCase{
		name: fmt.Sprintf("chunk_%d", size),
		open: func(s string) io.Reader { return newChunkedReader(strings.NewReader(s), size) },
	}
}
