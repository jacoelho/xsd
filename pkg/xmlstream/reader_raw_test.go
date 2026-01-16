package xmlstream

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestNextRawCharData(t *testing.T) {
	input := `<root>text</root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.NextRaw()
	if err != nil {
		t.Fatalf("char data error = %v", err)
	}
	if ev.Kind != EventCharData {
		t.Fatalf("char data kind = %v, want %v", ev.Kind, EventCharData)
	}
	if got := string(ev.Text); got != "text" {
		t.Fatalf("char data = %q, want text", got)
	}
	ev, err = r.NextRaw()
	if err != nil {
		t.Fatalf("end element error = %v", err)
	}
	if ev.Kind != EventEndElement || string(ev.Name.Full) != "root" {
		t.Fatalf("end element = %v %q, want root end", ev.Kind, ev.Name.Full)
	}
}

func TestNextRawManyAttributes(t *testing.T) {
	var attrs strings.Builder
	want := readerAttrCapacity + 4
	for i := 0; i < want; i++ {
		fmt.Fprintf(&attrs, ` a%d="%d"`, i, i)
	}
	input := fmt.Sprintf("<root%s/>", attrs.String())
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.NextRaw()
	if err != nil {
		t.Fatalf("NextRaw error = %v", err)
	}
	if ev.Kind != EventStartElement {
		t.Fatalf("root kind = %v, want %v", ev.Kind, EventStartElement)
	}
	if len(ev.Attrs) != want {
		t.Fatalf("attrs len = %d, want %d", len(ev.Attrs), want)
	}
}

func TestNextRawDoesNotInternAttrs(t *testing.T) {
	input := `<root xmlns:x="urn:x" x:attr="v" plain="p"/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil {
		t.Fatalf("NextRaw error = %v", err)
	}
	if got := len(r.names.table); got != 1 {
		t.Fatalf("qname cache size = %d, want 1", got)
	}
}

func TestNextRawUnboundPrefixAttr(t *testing.T) {
	input := `<root><child p:attr="v"/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	_, err = r.NextRaw()
	if err == nil {
		t.Fatalf("unbound prefix attr error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("unbound prefix attr error type = %T, want *xmltext.SyntaxError", err)
	}
	if !errors.Is(err, errUnboundPrefix) {
		t.Fatalf("unbound prefix attr error = %v, want %v", err, errUnboundPrefix)
	}
}

func TestNextRawAttrValueError(t *testing.T) {
	input := `<root attr="&bad;"/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	_, err = r.NextRaw()
	if err == nil {
		t.Fatalf("attr value error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("attr value error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestNextRawNonElementEvents(t *testing.T) {
	input := `<!--c--><?pi test?><!DOCTYPE root><root/>`
	r, err := NewReader(strings.NewReader(input), EmitComments(true), EmitPI(true), EmitDirectives(true))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.NextRaw()
	if err != nil {
		t.Fatalf("comment error = %v", err)
	}
	if ev.Kind != EventComment || string(ev.Text) != "c" {
		t.Fatalf("comment = %v %q, want Comment c", ev.Kind, ev.Text)
	}
	ev, err = r.NextRaw()
	if err != nil {
		t.Fatalf("PI error = %v", err)
	}
	if ev.Kind != EventPI || string(ev.Text) != "pi test" {
		t.Fatalf("PI = %v %q, want PI pi test", ev.Kind, ev.Text)
	}
	ev, err = r.NextRaw()
	if err != nil {
		t.Fatalf("directive error = %v", err)
	}
	if ev.Kind != EventDirective || string(ev.Text) != "DOCTYPE root" {
		t.Fatalf("directive = %v %q, want Directive DOCTYPE root", ev.Kind, ev.Text)
	}
	ev, err = r.NextRaw()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.Kind != EventStartElement || string(ev.Name.Full) != "root" {
		t.Fatalf("root start = %v %q, want root start", ev.Kind, ev.Name.Full)
	}
}

func TestRawEventScopeDepthPI(t *testing.T) {
	input := `<root><?pi data?></root>`
	r, err := NewReader(strings.NewReader(input), EmitPI(true))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.NextRaw()
	if err != nil {
		t.Fatalf("PI error = %v", err)
	}
	if ev.Kind != EventPI {
		t.Fatalf("PI kind = %v, want %v", ev.Kind, EventPI)
	}
	if ev.ScopeDepth != 0 {
		t.Fatalf("PI ScopeDepth = %d, want 0", ev.ScopeDepth)
	}
}

func TestRawEventScopeDepthDirective(t *testing.T) {
	input := `<!DOCTYPE root><root/>`
	r, err := NewReader(strings.NewReader(input), EmitDirectives(true))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.NextRaw()
	if err != nil {
		t.Fatalf("directive error = %v", err)
	}
	if ev.Kind != EventDirective {
		t.Fatalf("directive kind = %v, want %v", ev.Kind, EventDirective)
	}
	if ev.ScopeDepth != 0 {
		t.Fatalf("directive ScopeDepth = %d, want 0", ev.ScopeDepth)
	}
}

func TestNextRawCommentNestedScopeDepth(t *testing.T) {
	input := `<root><child><!--comment--></child></root>`
	r, err := NewReader(strings.NewReader(input), EmitComments(true))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.NextRaw()
	if err != nil {
		t.Fatalf("root start error = %v", err)
	}
	if ev.ScopeDepth != 0 {
		t.Fatalf("root ScopeDepth = %d, want 0", ev.ScopeDepth)
	}
	ev, err = r.NextRaw()
	if err != nil {
		t.Fatalf("child start error = %v", err)
	}
	if ev.ScopeDepth != 1 {
		t.Fatalf("child ScopeDepth = %d, want 1", ev.ScopeDepth)
	}
	ev, err = r.NextRaw()
	if err != nil {
		t.Fatalf("comment error = %v", err)
	}
	if ev.Kind != EventComment || string(ev.Text) != "comment" {
		t.Fatalf("comment = %v %q, want Comment comment", ev.Kind, ev.Text)
	}
	if ev.ScopeDepth != 1 {
		t.Fatalf("comment ScopeDepth = %d, want 1", ev.ScopeDepth)
	}
}

func TestNextRawNilReader(t *testing.T) {
	var err error
	var r *Reader
	if _, err = r.NextRaw(); !errors.Is(err, errNilReader) {
		t.Fatalf("NextRaw nil error = %v, want %v", err, errNilReader)
	}
}

func TestNextRawXMLDeclSkipped(t *testing.T) {
	input := `<?xml version="1.0" encoding="UTF-8"?><root/>`
	r, err := NewReader(strings.NewReader(input), EmitPI(true))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	ev, err := r.NextRaw()
	if err != nil {
		t.Fatalf("NextRaw error = %v", err)
	}
	if ev.Kind != EventStartElement || string(ev.Name.Full) != "root" {
		t.Fatalf("first event = %v %q, want root start", ev.Kind, ev.Name.Full)
	}
}

func TestNextRawCDATA(t *testing.T) {
	input := `<root><![CDATA[a&b<c>]]></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.NextRaw()
	if err != nil {
		t.Fatalf("CDATA error = %v", err)
	}
	if ev.Kind != EventCharData {
		t.Fatalf("CDATA kind = %v, want %v", ev.Kind, EventCharData)
	}
	if got := string(ev.Text); got != "a&b<c>" {
		t.Fatalf("CDATA text = %q, want a&b<c>", got)
	}
}

func TestNextRawAfterEOF(t *testing.T) {
	input := `<root/>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	for {
		_, err = r.NextRaw()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("NextRaw error = %v", err)
		}
	}
	if _, err = r.NextRaw(); !errors.Is(err, io.EOF) {
		t.Fatalf("NextRaw after EOF = %v, want %v", err, io.EOF)
	}
}

func TestNextRawUnboundPrefix(t *testing.T) {
	input := `<root><p:child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	_, err = r.NextRaw()
	if err == nil {
		t.Fatalf("unbound prefix error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("unbound prefix error type = %T, want *xmltext.SyntaxError", err)
	}
	if !errors.Is(err, errUnboundPrefix) {
		t.Fatalf("unbound prefix error = %v, want %v", err, errUnboundPrefix)
	}
}

func TestNextRawInvalidEntity(t *testing.T) {
	input := `<root><child attr="&bad;"/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	_, err = r.NextRaw()
	if err == nil {
		t.Fatalf("invalid entity error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("invalid entity error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestNextRawCharDataInvalidEntity(t *testing.T) {
	input := `<root>&bad;</root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	_, err = r.NextRaw()
	if err == nil {
		t.Fatalf("invalid entity error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("invalid entity error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestNextRawAfterSkipSubtree(t *testing.T) {
	input := `<root><skip><inner/></skip><after/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.NextRaw(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.NextRaw()
	if err != nil {
		t.Fatalf("skip start error = %v", err)
	}
	if ev.Kind != EventStartElement || string(ev.Name.Full) != "skip" {
		t.Fatalf("skip start = %v %q, want skip start", ev.Kind, ev.Name.Full)
	}
	if err = r.SkipSubtree(); err != nil {
		t.Fatalf("SkipSubtree error = %v", err)
	}
	ev, err = r.NextRaw()
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if ev.Kind != EventStartElement || string(ev.Name.Full) != "after" {
		t.Fatalf("after start = %v %q, want after start", ev.Kind, ev.Name.Full)
	}
}

func TestSkipSubtreeAfterNextThenNextRaw(t *testing.T) {
	input := `<root><skip><inner/></skip><after/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	ev, err := r.Next() // skip
	if err != nil {
		t.Fatalf("skip start error = %v", err)
	}
	if ev.Kind != EventStartElement || ev.Name.Local != "skip" {
		t.Fatalf("skip start = %v %s, want skip start", ev.Kind, ev.Name.String())
	}
	if err = r.SkipSubtree(); err != nil {
		t.Fatalf("SkipSubtree error = %v", err)
	}
	raw, err := r.NextRaw()
	if err != nil {
		t.Fatalf("after start error = %v", err)
	}
	if raw.Kind != EventStartElement || string(raw.Name.Full) != "after" {
		t.Fatalf("after start = %v %q, want after start", raw.Kind, raw.Name.Full)
	}
}

func TestNextRawNamespaceDeclError(t *testing.T) {
	input := `<root xmlns:p="&bad;"><p:child/></root>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	_, err = r.NextRaw()
	if err == nil {
		t.Fatalf("namespace decl error = nil, want error")
	}
	var syntax *xmltext.SyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("namespace decl error type = %T, want *xmltext.SyntaxError", err)
	}
}

func TestNextRawAfterNextError(t *testing.T) {
	input := `<root><child>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}
	if _, err = r.Next(); err != nil { // root
		t.Fatalf("root start error = %v", err)
	}
	if _, err = r.Next(); err != nil { // child start
		t.Fatalf("child start error = %v", err)
	}
	if _, err = r.Next(); err == nil {
		t.Fatalf("Next error = nil, want error")
	}
	if _, err = r.NextRaw(); err == nil {
		t.Fatalf("NextRaw error = nil, want error")
	}
}
