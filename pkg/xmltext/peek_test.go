package xmltext

import (
	"errors"
	"strings"
	"testing"
)

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

func TestPeekSkipDirective(t *testing.T) {
	input := "<!DOCTYPE root [<!ENTITY x '>'> [ignored] \"x\"]><root/>"
	dec := NewDecoder(strings.NewReader(input))
	pos, err := dec.peekSkipDirective(0)
	if err != nil {
		t.Fatalf("peekSkipDirective error = %v", err)
	}
	want := strings.Index(input, "<root/>")
	if pos != want {
		t.Fatalf("peekSkipDirective pos = %d, want %d", pos, want)
	}
}
