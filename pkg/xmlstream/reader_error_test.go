package xmlstream

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestWrapSyntaxErrorNilDecoder(t *testing.T) {
	base := errors.New("boom")
	if got := wrapSyntaxError(nil, 1, 2, base); !errors.Is(got, base) {
		t.Fatalf("wrapSyntaxError nil = %v, want %v", got, base)
	}
}

func TestWrapSyntaxErrorNilError(t *testing.T) {
	if got := wrapSyntaxError(nil, 1, 2, nil); got != nil {
		t.Fatalf("wrapSyntaxError nil error = %v, want nil", got)
	}
}

func TestWrapSyntaxErrorAlreadySyntax(t *testing.T) {
	base := &xmltext.SyntaxError{Line: 1, Column: 2, Err: errors.New("boom")}
	if got := wrapSyntaxError(nil, 1, 2, base); !errors.Is(got, base) {
		t.Fatalf("wrapSyntaxError syntax = %v, want %v", got, base)
	}
}

func TestWrapSyntaxErrorWraps(t *testing.T) {
	dec := xmltext.NewDecoder(strings.NewReader("<root/>"))
	base := errors.New("boom")
	err := wrapSyntaxError(dec, 3, 4, base)
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("wrapSyntaxError type = %T, want *xmltext.SyntaxError", err)
	}
	if syntax.Line != 3 || syntax.Column != 4 {
		t.Fatalf("wrapSyntaxError line/column = %d:%d, want 3:4", syntax.Line, syntax.Column)
	}
	if !errors.Is(err, base) {
		t.Fatalf("wrapSyntaxError unwrap = %v, want %v", err, base)
	}
}

func TestReaderResetAfterError(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root><bad"))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err == nil {
		t.Fatalf("malformed error = nil, want error")
	}
	if err = r.Reset(strings.NewReader("<good/>")); err != nil {
		t.Fatalf("Reset error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("good start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "good" {
		t.Fatalf("event = %v %s, want good start", ev.Kind, ev.Name.String())
	}
}

func TestReaderDuplicateAttributesResolved(t *testing.T) {
	doc := `<root xmlns:a="urn:x" a:attr="1" xmlns:b="urn:x" b:attr="2"/>`
	r, err := NewReader(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextResolved(); err == nil {
		t.Fatalf("expected duplicate attribute error")
	} else if !errors.Is(err, errDuplicateAttribute) {
		t.Fatalf("error = %v, want duplicate attribute error", err)
	}
}
