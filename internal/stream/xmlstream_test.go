package stream

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestXMLStreamParserRejectsTypedNilReader(t *testing.T) {
	var names Cache
	var values Cache
	var p Parser
	if err := p.Reset(strings.NewReader(`<old/>`), &names, &values); err != nil {
		t.Fatalf("Parser.Reset(old) error = %v", err)
	}
	if _, err := p.Next(); err != nil {
		t.Fatalf("Parser.Next(old start) error = %v", err)
	}
	if !p.hasEnd {
		t.Fatal("Parser.Next(old start) did not leave a synthetic end pending")
	}

	var reader *bytes.Reader
	if err := p.Reset(reader, &names, &values); !errors.Is(err, ErrXMLInputNilReader) {
		t.Fatalf("Parser.Reset() error = %v, want ErrXMLInputNilReader", err)
	}
	if p.br.r != nil || p.names != nil || p.values != nil || p.hasEnd || p.pendingEnd != (EndElement{}) {
		t.Fatalf("Parser state retained after failed reset: br.r=%v names=%p values=%p hasEnd=%v pendingEnd=%+v", p.br.r, p.names, p.values, p.hasEnd, p.pendingEnd)
	}
	if _, err := p.Next(); !errors.Is(err, ErrXMLInputNilReader) {
		t.Fatalf("Parser.Next() after failed reset error = %v, want ErrXMLInputNilReader", err)
	}
	if err := p.Reset(strings.NewReader(`<new/>`), &names, &values); err != nil {
		t.Fatalf("Parser.Reset(new) error = %v", err)
	}
	start, err := p.Next()
	if err != nil || start.Kind != KindStart || start.Start.Name.Local != "new" {
		t.Fatalf("Parser.Next(new start) = %+v, %v", start, err)
	}
}

type eofWithDataReader struct {
	data []byte
	done bool
}

func (r *eofWithDataReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(p, r.data), io.EOF
}

func TestXMLStreamParserConsumesBytesReturnedWithEOF(t *testing.T) {
	names := NewCache()
	values := NewCache()
	p := new(Parser)
	if err := p.Reset(&eofWithDataReader{data: []byte(`<root/>`)}, &names, &values); err != nil {
		t.Fatal(err)
	}

	tok, err := p.Next()
	if err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	if tok.Kind != KindStart || tok.Start.Name.Local != "root" {
		t.Fatalf("first token = %+v, want root start", tok)
	}

	tok, err = p.Next()
	if err != nil {
		t.Fatalf("next root end error = %v", err)
	}
	if tok.Kind != KindEnd || tok.End.Name.Local != "root" {
		t.Fatalf("second token = %+v, want root end", tok)
	}

	_, err = p.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("final error = %v, want EOF", err)
	}
}

func TestXMLStreamParserPreservesLexicalPrefixForSyntheticEnd(t *testing.T) {
	names := NewCache()
	values := NewCache()
	p := new(Parser)
	if err := p.Reset(strings.NewReader(`<p:root xmlns:p="urn:test"/>`), &names, &values); err != nil {
		t.Fatal(err)
	}

	start, err := p.Next()
	if err != nil {
		t.Fatalf("next start error = %v", err)
	}
	if start.Kind != KindStart || start.Start.Name != (xml.Name{Space: "p", Local: "root"}) {
		t.Fatalf("start token = %+v, want lexical p:root", start)
	}
	end, err := p.Next()
	if err != nil {
		t.Fatalf("next synthetic end error = %v", err)
	}
	if end.Kind != KindEnd || end.End.Name != start.Start.Name {
		t.Fatalf("synthetic end = %+v, want start lexical name %+v", end, start.Start)
	}
}

func TestXMLStreamParserSkipsCommentsByDefault(t *testing.T) {
	names := NewCache()
	values := NewCache()
	p := new(Parser)
	if err := p.Reset(strings.NewReader(`<root><!--`+strings.Repeat("x", 1<<20)+`--><v>1</v></root>`), &names, &values); err != nil {
		t.Fatal(err)
	}

	tok, err := p.Next()
	if err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	if tok.Kind != KindStart || tok.Start.Name.Local != "root" {
		t.Fatalf("first token = %+v, want root start", tok)
	}

	tok, err = p.Next()
	if err != nil {
		t.Fatalf("next child start error = %v", err)
	}
	if tok.Kind == KindComment {
		t.Fatal("default parser emitted comment")
	}
	if tok.Kind != KindStart || tok.Start.Name.Local != "v" {
		t.Fatalf("second token = %+v, want v start", tok)
	}
	if cap(p.directive) != 0 {
		t.Fatalf("comment data retained with cap %d", cap(p.directive))
	}
}

func TestXMLStreamParserSkipsUnicodeCommentsByDefault(t *testing.T) {
	names := NewCache()
	values := NewCache()
	p := new(Parser)
	if err := p.Reset(strings.NewReader(`<root><!--é--><v>1</v></root>`), &names, &values); err != nil {
		t.Fatal(err)
	}

	if _, err := p.Next(); err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	tok, err := p.Next()
	if err != nil {
		t.Fatalf("next child start error = %v", err)
	}
	if tok.Kind != KindStart || tok.Start.Name.Local != "v" {
		t.Fatalf("second token = %+v, want v start", tok)
	}
}

func TestXMLStreamParserEmitsCommentsWhenEnabled(t *testing.T) {
	names := NewCache()
	values := NewCache()
	p := new(Parser)
	if err := p.Reset(strings.NewReader(`<root><!-- note --></root>`), &names, &values); err != nil {
		t.Fatal(err)
	}
	p.emitComments = true

	if _, err := p.Next(); err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	tok, err := p.Next()
	if err != nil {
		t.Fatalf("next comment error = %v", err)
	}
	if tok.Kind != KindComment || string(tok.Directive) != " note " {
		t.Fatalf("comment token = %+v", tok)
	}
}

func TestXMLStreamParserHandlesBareCRText(t *testing.T) {
	done := make(chan struct{})
	var tok Token
	var err error
	go func() {
		names := NewCache()
		values := NewCache()
		p := new(Parser)
		err = p.Reset(strings.NewReader("<root>a\rb</root>"), &names, &values)
		if err != nil {
			close(done)
			return
		}
		if _, err = p.Next(); err != nil {
			close(done)
			return
		}
		tok, err = p.Next()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("parser timed out")
	}
	if err != nil {
		t.Fatalf("next char data error = %v", err)
	}
	if tok.Kind != KindCharData || string(tok.Data) != "a\nb" {
		t.Fatalf("char data token = %+v", tok)
	}
}

func TestParseCharRefRejectsUppercaseHexMarker(t *testing.T) {
	if r, ok := parseCharRef([]byte("x4F")); !ok || r != 'O' {
		t.Fatalf("parseCharRef(x4F) = %q, %v; want O, true", r, ok)
	}
	if r, ok := parseCharRef([]byte("X4F")); ok || r != 0 {
		t.Fatalf("parseCharRef(X4F) = %q, %v; want rejected", r, ok)
	}
}

func TestXMLStreamParserNormalizesCDATALineEndings(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "bare_cr", in: "<root><![CDATA[a\rb]]></root>"},
		{name: "crlf", in: "<root><![CDATA[a\r\nb]]></root>"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			names := NewCache()
			values := NewCache()
			p := new(Parser)
			if err := p.Reset(strings.NewReader(test.in), &names, &values); err != nil {
				t.Fatal(err)
			}
			if _, err := p.Next(); err != nil {
				t.Fatalf("next root start error = %v", err)
			}
			tok, err := p.Next()
			if err != nil {
				t.Fatalf("next CDATA error = %v", err)
			}
			if tok.Kind != KindCharData || !tok.CDATA || string(tok.Data) != "a\nb" {
				t.Fatalf("CDATA token = %+v", tok)
			}
		})
	}
}

func TestXMLStreamParserRejectsInvalidSkippedComment(t *testing.T) {
	names := NewCache()
	values := NewCache()
	p := new(Parser)
	if err := p.Reset(strings.NewReader(`<root><!-- invalid -- comment --></root>`), &names, &values); err != nil {
		t.Fatal(err)
	}

	if _, err := p.Next(); err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	_, err := p.Next()
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("invalid comment error = %v", err)
	}
}

func TestXMLStreamParserRejectsInvalidUTF8InSkippedComment(t *testing.T) {
	names := NewCache()
	values := NewCache()
	p := new(Parser)
	if err := p.Reset(strings.NewReader("<root><!--\xff--></root>"), &names, &values); err != nil {
		t.Fatal(err)
	}

	if _, err := p.Next(); err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	_, err := p.Next()
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("invalid UTF-8 comment error = %v", err)
	}
}

func TestXMLStreamParserChunksLargeCDATA(t *testing.T) {
	names := NewCache()
	values := NewCache()
	data := strings.Repeat("x", 70*1024)
	p := new(Parser)
	if err := p.Reset(strings.NewReader(`<root><![CDATA[`+data+`]]></root>`), &names, &values); err != nil {
		t.Fatal(err)
	}

	if _, err := p.Next(); err != nil {
		t.Fatalf("next root start error = %v", err)
	}
	total := 0
	chunks := 0
	for {
		tok, err := p.Next()
		if err != nil {
			t.Fatalf("next CDATA chunk error = %v", err)
		}
		if tok.Kind == KindEnd {
			break
		}
		if tok.Kind != KindCharData || !tok.CDATA {
			t.Fatalf("token = %+v, want CDATA char data", tok)
		}
		if len(tok.Data) > len(p.br.buf) {
			t.Fatalf("CDATA chunk len = %d, want <= %d", len(tok.Data), len(p.br.buf))
		}
		total += len(tok.Data)
		chunks++
	}
	if total != len(data) {
		t.Fatalf("CDATA total = %d, want %d", total, len(data))
	}
	if chunks < 2 {
		t.Fatalf("CDATA chunks = %d, want multiple chunks", chunks)
	}
}

func TestByteStreamConsumeBufferedTracksNewlines(t *testing.T) {
	bs := new(byteStream)
	bs.reset(context.Background(), strings.NewReader("ab\ncd\nef"), 0)
	chunk, err := bs.buffered()
	if err != nil {
		t.Fatalf("buffered() error = %v", err)
	}
	bs.consumeBuffered(len(chunk))
	line, col := bs.pos()
	if line != 3 || col != 2 {
		t.Fatalf("pos() = %d:%d, want 3:2", line, col)
	}
}

func TestByteStreamConsumeBufferedAfterReadByteNewlines(t *testing.T) {
	bs := new(byteStream)
	bs.reset(context.Background(), strings.NewReader("a\nbc\nde"), 0)
	if _, err := bs.buffered(); err != nil {
		t.Fatalf("buffered() error = %v", err)
	}
	bs.consumeBuffered(1)
	if b, err := bs.readByte(); err != nil || b != '\n' {
		t.Fatalf("readByte() = %q, %v, want '\\n'", b, err)
	}
	bs.consumeBuffered(2)
	if line, col := bs.pos(); line != 2 || col != 2 {
		t.Fatalf("pos() = %d:%d, want 2:2", line, col)
	}
	if b, err := bs.readByte(); err != nil || b != '\n' {
		t.Fatalf("readByte() = %q, %v, want '\\n'", b, err)
	}
	bs.consumeBuffered(2)
	if line, col := bs.pos(); line != 3 || col != 2 {
		t.Fatalf("pos() = %d:%d, want 3:2", line, col)
	}
}

func TestByteStreamConsumeBufferedNewlineThenCleanChunk(t *testing.T) {
	bs := new(byteStream)
	bs.reset(context.Background(), strings.NewReader("a\nb\n\ncdef"), 0)
	if _, err := bs.buffered(); err != nil {
		t.Fatalf("buffered() error = %v", err)
	}
	bs.consumeBuffered(5)
	if line, col := bs.pos(); line != 4 || col != 0 {
		t.Fatalf("pos() = %d:%d, want 4:0", line, col)
	}
	bs.consumeBuffered(4)
	if line, col := bs.pos(); line != 4 || col != 4 {
		t.Fatalf("pos() = %d:%d, want 4:4", line, col)
	}
}

func TestAttributeValuesAreOwnedStrings(t *testing.T) {
	var doc strings.Builder
	doc.WriteString("<r")
	for i := range 64 {
		fmt.Fprintf(&doc, ` a%d="%032d"`, i, i)
	}
	doc.WriteString("/>")
	names := NewCache()
	values := NewCache()
	p := new(Parser)
	if err := p.Reset(strings.NewReader(doc.String()), &names, &values); err != nil {
		t.Fatal(err)
	}

	tok, err := p.Next()
	if err != nil {
		t.Fatalf("next() error = %v", err)
	}
	if tok.Kind != KindStart || len(tok.Start.Attr) != 64 {
		t.Fatalf("token = %+v, want start element with 64 attributes", tok)
	}
	for i, attr := range tok.Start.Attr {
		want := fmt.Sprintf("%032d", i)
		if attr.Value != want {
			t.Fatalf("attribute %s value = %q, want %q", attr.Name.Local, attr.Value, want)
		}
	}
}

func TestXMLStreamParserLimitsAggregateStartPayload(t *testing.T) {
	t.Parallel()

	for _, lazy := range []bool{false, true} {
		for _, tt := range []struct {
			name    string
			limit   int64
			wantErr bool
		}{
			{name: "exact limit", limit: 7},
			{name: "over limit", limit: 6, wantErr: true},
		} {
			name := fmt.Sprintf("lazy=%t/%s", lazy, tt.name)
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				names := NewCache()
				values := NewCache()
				p := new(Parser)
				if err := p.ResetWithLimits(strings.NewReader(`<r a="12" b="34"/>`), &names, &values, Limits{MaxTokenBytes: tt.limit}); err != nil {
					t.Fatal(err)
				}
				p.SetLazyAttrValue(lazy)

				_, err := p.Next()
				if tt.wantErr {
					if !IsTokenLimit(err) {
						t.Fatalf("Next() error = %v, want token limit", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("Next() error = %v", err)
				}
			})
		}
	}
}

func TestXMLStreamParserLimitsAggregateProcessingInstructionPayload(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		limit   int64
		wantErr bool
	}{
		{name: "exact limit", limit: 5},
		{name: "over limit", limit: 4, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			names := NewCache()
			values := NewCache()
			p := new(Parser)
			if err := p.ResetWithLimits(strings.NewReader(`<?pi abc?><r/>`), &names, &values, Limits{MaxTokenBytes: tt.limit}); err != nil {
				t.Fatal(err)
			}
			p.SetEmitPI(true)

			_, err := p.Next()
			if tt.wantErr {
				if !IsTokenLimit(err) {
					t.Fatalf("Next() error = %v, want token limit", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Next() error = %v", err)
			}
		})
	}
}

func TestXMLStreamParserPreservesDisprovedProcessingInstructionTerminatorPrefix(t *testing.T) {
	t.Parallel()

	names := NewCache()
	values := NewCache()
	p := new(Parser)
	if err := p.ResetWithLimits(strings.NewReader(`<?pi ?x?><r/>`), &names, &values, Limits{MaxTokenBytes: 4}); err != nil {
		t.Fatal(err)
	}
	p.SetEmitPI(true)

	tok, err := p.Next()
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if tok.Kind != KindPI || string(tok.Data) != "pi" || string(tok.Directive) != "?x" {
		t.Fatalf("Next() = %+v, want PI target pi and content ?x", tok)
	}
}

func TestXMLStreamParserChargesDecodedEntityPayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		xml     string
		limit   int64
		token   int
		wantErr bool
	}{
		{name: "character data exact", xml: `<r>&amp;&amp;</r>`, limit: 5, token: 2},
		{name: "character data over", xml: `<r>&amp;&amp;</r>`, limit: 4, token: 2, wantErr: true},
		{name: "attribute exact", xml: `<r a="&amp;"/>`, limit: 6, token: 1},
		{name: "attribute over", xml: `<r a="&amp;"/>`, limit: 5, token: 1, wantErr: true},
		{name: "numeric UTF-8 exact", xml: `<r>&#x20AC;</r>`, limit: 9, token: 2},
		{name: "numeric UTF-8 over", xml: `<r>&#x20AC;</r>`, limit: 8, token: 2, wantErr: true},
		{name: "numeric leading zeros exact", xml: `<r>&#0065;</r>`, limit: 6, token: 2},
		{name: "numeric leading zeros over", xml: `<r>&#0065;</r>`, limit: 5, token: 2, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			names := NewCache()
			values := NewCache()
			p := new(Parser)
			if err := p.ResetWithLimits(strings.NewReader(tt.xml), &names, &values, Limits{MaxTokenBytes: tt.limit}); err != nil {
				t.Fatal(err)
			}
			var err error
			for range tt.token {
				_, err = p.Next()
				if err != nil {
					break
				}
			}
			if len(p.entityBuf) != 0 {
				t.Fatalf("entity scratch length = %d after token", len(p.entityBuf))
			}
			if tt.wantErr {
				if !IsTokenLimit(err) {
					t.Fatalf("Next() error = %v, want token limit", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Next() error = %v", err)
			}
		})
	}
}

func TestLazyAttributeValueCopiesSurviveParserAdvance(t *testing.T) {
	startTag := func(name, value string) string {
		var b strings.Builder
		b.WriteByte('<')
		b.WriteString(name)
		for i := range LazyAttrRawMinAttrs {
			fmt.Fprintf(&b, ` a%d="%s"`, i, value)
		}
		b.WriteString("/>")
		return b.String()
	}
	names := NewCache()
	values := NewCache()
	p := new(Parser)
	if err := p.Reset(strings.NewReader(startTag("r", "1111111111111111")+startTag("s", "2222222222222222")), &names, &values); err != nil {
		t.Fatal(err)
	}
	p.lazyAttrValue = true

	first, err := p.Next()
	if err != nil {
		t.Fatalf("first next() error = %v", err)
	}
	if first.Kind != KindStart || len(first.Start.Attr) != LazyAttrRawMinAttrs {
		t.Fatalf("first token = %+v, want %d-attribute start", first, LazyAttrRawMinAttrs)
	}
	firstAttr := first.Start.Attr[0]
	if !firstAttr.HasBorrowedValue() {
		t.Fatal("first attribute did not use raw path")
	}
	retainedString := firstAttr.StringValue(&values)
	firstRaw := first.Start.Attr[0].raw
	if len(firstRaw) == 0 {
		t.Fatal("first attribute raw buffer is empty")
	}
	firstRawPtr := &firstRaw[0]
	retainedBytes := first.Start.Attr[0].AppendValue(nil, &values)

	_, err = p.Next()
	if err != nil {
		t.Fatalf("end next() error = %v", err)
	}
	second, err := p.Next()
	if err != nil {
		t.Fatalf("second next() error = %v", err)
	}
	if second.Kind != KindStart || len(second.Start.Attr) != LazyAttrRawMinAttrs {
		t.Fatalf("second token = %+v, want %d-attribute start", second, LazyAttrRawMinAttrs)
	}
	if got := retainedString; got != "1111111111111111" {
		t.Fatalf("retained first attribute string = %q", got)
	}
	if string(retainedBytes) != "1111111111111111" {
		t.Fatalf("copied first attribute = %q", retainedBytes)
	}
	secondRaw := second.Start.Attr[0].raw
	if len(secondRaw) == 0 {
		t.Fatal("second attribute raw buffer is empty")
	}
	if got := &secondRaw[0]; got != firstRawPtr {
		t.Fatal("lazy attribute raw buffer was not reused across start tokens")
	}
	if got := second.Start.Attr[0].StringValue(&values); got != "2222222222222222" {
		t.Fatalf("second attribute = %q", got)
	}
}

func BenchmarkParserLazyWideAttributes(b *testing.B) {
	var doc strings.Builder
	doc.WriteString("<root>")
	for elem := range 128 {
		fmt.Fprintf(&doc, "<e%d", elem)
		for attr := range LazyAttrRawMinAttrs {
			fmt.Fprintf(&doc, ` a%d="value-%d-%d"`, attr, elem, attr)
		}
		doc.WriteString("/>")
	}
	doc.WriteString("</root>")
	text := doc.String()
	names := NewCache()
	values := NewCache()
	var p Parser
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	for b.Loop() {
		if err := p.Reset(strings.NewReader(text), &names, &values); err != nil {
			b.Fatal(err)
		}
		p.lazyAttrValue = true
		for {
			_, err := p.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func TestParserZeroesReleasedAttributeReferences(t *testing.T) {
	names := NewCache()
	values := NewCache()
	var parser Parser
	if err := parser.Reset(strings.NewReader(`<r a="1" b="2"/><s c="3"/>`), &names, &values); err != nil {
		t.Fatal(err)
	}
	if _, err := parser.Next(); err != nil {
		t.Fatal(err)
	}
	tok, err := parser.Next()
	if err != nil {
		t.Fatal(err)
	}
	if tok.Kind != KindEnd {
		t.Fatalf("second token kind = %v, want end", tok.Kind)
	}
	if len(parser.attrs) != 0 {
		t.Fatalf("released active attributes = %d, want 0", len(parser.attrs))
	}
	for i, got := range parser.attrs[:cap(parser.attrs)] {
		if got.Name != (xml.Name{}) || got.Value != "" || got.raw != nil {
			t.Fatalf("released attribute %d retains references: %+v", i, got)
		}
	}
	if parser.pendingEnd != (EndElement{}) {
		t.Fatalf("released pending end retains references: %+v", parser.pendingEnd)
	}
	if _, err := parser.Next(); err != nil {
		t.Fatal(err)
	}
	if len(parser.attrs) != 1 {
		t.Fatalf("active attributes = %d, want 1", len(parser.attrs))
	}
	if got := parser.attrs[:cap(parser.attrs)][1]; got.Name != (xml.Name{}) || got.Value != "" || got.raw != nil {
		t.Fatalf("released attribute retains references: %+v", got)
	}
}

func TestStreamTokenAppendDataCopiesBorrowedBytes(t *testing.T) {
	names := NewCache()
	values := NewCache()
	p := new(Parser)
	if err := p.Reset(strings.NewReader(`<r>alpha</r><s>bravo</s>`), &names, &values); err != nil {
		t.Fatal(err)
	}

	if _, err := p.Next(); err != nil {
		t.Fatalf("first start next() error = %v", err)
	}
	firstText, err := p.Next()
	if err != nil {
		t.Fatalf("first text next() error = %v", err)
	}
	if firstText.Kind != KindCharData {
		t.Fatalf("first text token kind = %v, want char data", firstText.Kind)
	}
	retained := firstText.AppendData(nil)

	_, err = p.Next()
	if err != nil {
		t.Fatalf("first end next() error = %v", err)
	}
	_, err = p.Next()
	if err != nil {
		t.Fatalf("second start next() error = %v", err)
	}
	secondText, err := p.Next()
	if err != nil {
		t.Fatalf("second text next() error = %v", err)
	}
	if got := string(secondText.Data); got != "bravo" {
		t.Fatalf("second text = %q, want bravo", got)
	}
	if got := string(retained); got != "alpha" {
		t.Fatalf("retained first text = %q, want alpha", got)
	}
}
