package xmlstream

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestEmitCommentPIAndDirective(t *testing.T) {
	input := `<!--c--><?pi test?><!DOCTYPE root><root/>`
	r, err := NewReader(strings.NewReader(input), xmltext.EmitComments(true), xmltext.EmitPI(true), xmltext.EmitDirectives(true))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("comment error = %v", err)
	}
	if ev.Kind != EventComment {
		t.Fatalf("comment kind = %v, want %v", ev.Kind, EventComment)
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("PI error = %v", err)
	}
	if ev.Kind != EventPI {
		t.Fatalf("PI kind = %v, want %v", ev.Kind, EventPI)
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("directive error = %v", err)
	}
	if ev.Kind != EventDirective {
		t.Fatalf("directive kind = %v, want %v", ev.Kind, EventDirective)
	}
	if !bytes.HasPrefix(ev.Text, []byte("DOCTYPE root")) {
		t.Fatalf("directive text = %q, want DOCTYPE root...", string(ev.Text))
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "root" {
		t.Fatalf("root start = %v %s, want root start", ev.Kind, ev.Name.String())
	}
}

func TestNestedCommentsAndPI(t *testing.T) {
	input := `<root><!--outer--><child><?pi data?></child></root>`
	r, err := NewReader(strings.NewReader(input), xmltext.EmitComments(true), xmltext.EmitPI(true))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "root" {
		t.Fatalf("root start = %v %s, want root start", ev.Kind, ev.Name.String())
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("outer comment error = %v", err)
	}
	if ev.Kind != EventComment || string(ev.Text) != "outer" {
		t.Fatalf("outer comment = %v %q, want Comment outer", ev.Kind, ev.Text)
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("child start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "child" {
		t.Fatalf("child start = %v %s, want child start", ev.Kind, ev.Name.String())
	}
	ev, err = r.Next()
	if err != nil {
		t.Fatalf("PI error = %v", err)
	}
	if ev.Kind != EventPI || string(ev.Text) != "pi data" {
		t.Fatalf("PI = %v %q, want PI pi data", ev.Kind, ev.Text)
	}
}

func TestCoalesceCharDataFalse(t *testing.T) {
	input := `<root>one<![CDATA[two]]>three</root>`
	r, err := NewReader(strings.NewReader(input), xmltext.CoalesceCharData(false))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	var texts []string
	for {
		ev, err := r.Next()
		if err != nil {
			t.Fatalf("Next error = %v", err)
		}
		switch ev.Kind {
		case EventCharData:
			texts = append(texts, string(ev.Text))
		case EventEndElement:
			if ev.Name.Local == "root" {
				if len(texts) != 3 || texts[0] != "one" || texts[1] != "two" || texts[2] != "three" {
					t.Fatalf("char data = %#v, want [one two three]", texts)
				}
				return
			}
		}
	}
}

func TestTrackLineColumnFalse(t *testing.T) {
	input := "<root>\n<child/></root>"
	r, err := NewReader(strings.NewReader(input), xmltext.TrackLineColumn(false))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.Line != 0 || ev.Column != 0 {
		t.Fatalf("line/column = %d:%d, want 0:0", ev.Line, ev.Column)
	}
	if line, col := r.CurrentPos(); line != 0 || col != 0 {
		t.Fatalf("CurrentPos = %d:%d, want 0:0", line, col)
	}
}

func TestOptionOverrideOrder(t *testing.T) {
	input := `<!--c--><root/>`
	r, err := NewReader(strings.NewReader(input), xmltext.EmitComments(true), xmltext.EmitComments(false))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("Next error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "root" {
		t.Fatalf("first event = %v %s, want root start", ev.Kind, ev.Name.String())
	}
}

func TestBuildOptionsEmpty(t *testing.T) {
	opts := buildOptions()
	if len(opts) == 0 {
		t.Fatalf("buildOptions returned empty slice")
	}
	merged := xmltext.JoinOptions(opts...)
	if limit, ok := merged.QNameInternEntries(); !ok || limit != qnameCacheMaxEntries {
		t.Fatalf("QNameInternEntries = %d, ok=%v, want %d, true", limit, ok, qnameCacheMaxEntries)
	}
}

func TestReaderMaxDepth(t *testing.T) {
	r, err := NewReader(strings.NewReader("<a><b/></a>"), xmltext.MaxDepth(1))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err == nil {
		t.Fatalf("depth limit error = nil, want error")
	} else {
		var syntax *xmltext.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("depth error type = %T, want *xmltext.SyntaxError", err)
		}
	}
}

func TestReaderMaxAttrs(t *testing.T) {
	r, err := NewReader(strings.NewReader(`<a b="1" c="2"/>`), xmltext.MaxAttrs(1))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err == nil {
		t.Fatalf("attr limit error = nil, want error")
	} else {
		var syntax *xmltext.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("attr limit error type = %T, want *xmltext.SyntaxError", err)
		}
	}
}

func TestReaderMaxTokenSize(t *testing.T) {
	r, err := NewReader(strings.NewReader("<root>abcdefgh</root>"), xmltext.MaxTokenSize(6))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err == nil {
		t.Fatalf("token size error = nil, want error")
	} else {
		var syntax *xmltext.SyntaxError
		if !errors.As(err, &syntax) {
			t.Fatalf("token size error type = %T, want *xmltext.SyntaxError", err)
		}
	}
}

func TestWithCharsetReader(t *testing.T) {
	input := []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><root>\xe9</root>")
	decoder := func(label string, r io.Reader) (io.Reader, error) {
		if !strings.EqualFold(label, "ISO-8859-1") {
			return nil, fmt.Errorf("unsupported charset %s", label)
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 0, len(data))
		for _, b := range data {
			if b < 0x80 {
				out = append(out, b)
				continue
			}
			out = utf8.AppendRune(out, rune(b))
		}
		return bytes.NewReader(out), nil
	}
	r, err := NewReader(bytes.NewReader(input), xmltext.WithCharsetReader(decoder))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.Next()
	if err != nil {
		t.Fatalf("char data error = %v", err)
	}
	if got := string(ev.Text); got != "\u00e9" {
		t.Fatalf("char data = %q, want \\u00e9", got)
	}
}
