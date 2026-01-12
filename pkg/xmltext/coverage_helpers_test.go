package xmltext

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeLimit(t *testing.T) {
	if got := normalizeLimit(-1); got != 0 {
		t.Fatalf("normalizeLimit(-1) = %d, want 0", got)
	}
	if got := normalizeLimit(3); got != 3 {
		t.Fatalf("normalizeLimit(3) = %d, want 3", got)
	}
}

func TestSyntaxErrorUnwrap(t *testing.T) {
	var err *SyntaxError
	if err.Unwrap() != nil {
		t.Fatalf("nil SyntaxError Unwrap = %v, want nil", err.Unwrap())
	}
	syntax := &SyntaxError{Err: errInvalidToken}
	if !errors.Is(syntax, errInvalidToken) {
		t.Fatalf("Unwrap = %v, want %v", syntax.Unwrap(), errInvalidToken)
	}
}

func TestSpanBytesEdgeCases(t *testing.T) {
	buf := spanBuffer{data: []byte("abc"), gen: 1}
	span := makeSpan(&buf, 0, 3)
	if got := string(span.bytes()); got != "abc" {
		t.Fatalf("bytes = %q, want abc", got)
	}
	if got := makeSpan(nil, 0, 0).bytes(); got != nil {
		t.Fatalf("nil buffer bytes = %v, want nil", got)
	}
	invalid := Span{Start: -1, End: 1, buf: &buf}
	if invalid.bytes() != nil {
		t.Fatalf("invalid start bytes = %v, want nil", invalid.bytes())
	}
	invalid = Span{Start: 0, End: 4, buf: &buf}
	if invalid.bytes() != nil {
		t.Fatalf("invalid end bytes = %v, want nil", invalid.bytes())
	}
	buf.poison = true
	span = makeSpan(&buf, 0, 1)
	buf.gen++
	if span.bytes() != nil {
		t.Fatalf("poisoned bytes = %v, want nil", span.bytes())
	}
}

func TestResetAttrSeenOverflow(t *testing.T) {
	dec := &Decoder{}
	dec.attrSeen = map[uint64]attrBucket{
		1: {gen: 1, spans: []Span{{}}},
	}
	dec.attrSeenGen = ^uint32(0)
	dec.resetAttrSeen()
	if dec.attrSeenGen != 1 {
		t.Fatalf("attrSeenGen = %d, want 1", dec.attrSeenGen)
	}
	if len(dec.attrSeen) != 0 {
		t.Fatalf("attrSeen len = %d, want 0", len(dec.attrSeen))
	}
}

func TestPeekMatchLiteral(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<!--"))
	ok, err := dec.peekMatchLiteral(0, litComStart)
	if err != nil {
		t.Fatalf("peekMatchLiteral error = %v", err)
	}
	if !ok {
		t.Fatalf("peekMatchLiteral = false, want true")
	}

	dec = NewDecoder(strings.NewReader("<!??"))
	ok, err = dec.peekMatchLiteral(0, litComStart)
	if err != nil {
		t.Fatalf("peekMatchLiteral mismatch error = %v", err)
	}
	if ok {
		t.Fatalf("peekMatchLiteral = true, want false")
	}

	dec = NewDecoder(strings.NewReader("<!"))
	_, err = dec.peekMatchLiteral(0, litComStart)
	if !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("peekMatchLiteral error = %v, want %v", err, errUnexpectedEOF)
	}
}

func TestPeekSkipUntil(t *testing.T) {
	dec := NewDecoder(strings.NewReader("<?pi data?>"))
	pos, err := dec.peekSkipUntil(0, litPIEnd)
	if err != nil {
		t.Fatalf("peekSkipUntil error = %v", err)
	}
	if pos != len("<?pi data?>") {
		t.Fatalf("peekSkipUntil pos = %d, want %d", pos, len("<?pi data?>"))
	}

	dec = NewDecoder(strings.NewReader("<?pi data"))
	_, err = dec.peekSkipUntil(0, litPIEnd)
	if !errors.Is(err, errUnexpectedEOF) {
		t.Fatalf("peekSkipUntil error = %v, want %v", err, errUnexpectedEOF)
	}
}

func TestPeekRuneErrors(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte{0xff}}}
	if _, _, err := dec.peekRune(false); !errors.Is(err, errInvalidChar) {
		t.Fatalf("peekRune invalid utf8 error = %v, want %v", err, errInvalidChar)
	}

	dec = &Decoder{buf: spanBuffer{data: []byte{0xc3}}, eof: true}
	if _, _, err := dec.peekRune(false); !errors.Is(err, errInvalidChar) {
		t.Fatalf("peekRune incomplete error = %v, want %v", err, errInvalidChar)
	}

	dec = &Decoder{buf: spanBuffer{data: []byte("a")}}
	r, size, err := dec.peekRune(false)
	if err != nil {
		t.Fatalf("peekRune ascii error = %v", err)
	}
	if r != 'a' || size != 1 {
		t.Fatalf("peekRune = %q/%d, want a/1", r, size)
	}
}

func TestRefreshToken(t *testing.T) {
	buf := spanBuffer{data: []byte("root attr val"), gen: 2}
	name := newQNameSpan(&buf, 0, 4)
	attrName := newQNameSpan(&buf, 5, 9)
	attrValue := makeSpan(&buf, 10, 13)
	tok := Token{
		Kind: KindStartElement,
		Name: name,
		Attrs: []AttrSpan{{
			Name:      attrName,
			ValueSpan: attrValue,
		}},
	}
	tok.Name.Full.gen = 0
	tok.Name.Local.gen = 0
	tok.Attrs[0].Name.Full.gen = 0
	tok.Attrs[0].ValueSpan.gen = 0

	dec := &Decoder{}
	dec.refreshToken(&tok)
	if tok.Name.Full.gen != buf.gen {
		t.Fatalf("name gen = %d, want %d", tok.Name.Full.gen, buf.gen)
	}
	if tok.Attrs[0].ValueSpan.gen != buf.gen {
		t.Fatalf("attr value gen = %d, want %d", tok.Attrs[0].ValueSpan.gen, buf.gen)
	}
}

func TestCompact(t *testing.T) {
	dec := &Decoder{buf: spanBuffer{data: []byte("abcd")}, pos: 2, baseOffset: 5}
	dec.compact()
	if got := string(dec.buf.data); got != "cd" {
		t.Fatalf("compact data = %q, want cd", got)
	}
	if dec.pos != 0 {
		t.Fatalf("compact pos = %d, want 0", dec.pos)
	}
	if dec.baseOffset != 7 {
		t.Fatalf("compact baseOffset = %d, want 7", dec.baseOffset)
	}

	dec.buf.data = []byte("abcd")
	dec.pos = 4
	dec.baseOffset = 0
	dec.compact()
	if len(dec.buf.data) != 0 {
		t.Fatalf("compact cleared len = %d, want 0", len(dec.buf.data))
	}
	if dec.pos != 0 {
		t.Fatalf("compact cleared pos = %d, want 0", dec.pos)
	}
	if dec.baseOffset != 4 {
		t.Fatalf("compact cleared baseOffset = %d, want 4", dec.baseOffset)
	}
}
