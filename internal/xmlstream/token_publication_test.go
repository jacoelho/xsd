package xmlstream

import (
	"encoding/xml"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

type publicationAttribute struct {
	Name  xml.Name
	Value string
}

type publicationSnapshot struct {
	Start        xml.Name
	End          xml.Name
	Attrs        []publicationAttribute
	AttrsNil     bool
	DataNil      bool
	DirectiveNil bool
	Data         string
	Directive    string
	StartOffset  int
	EndOffset    int
	Line         int
	Column       int
	Kind         TokenKind
	TextKind     CharacterDataKind
}

func snapshotPublicationToken(token *Token) publicationSnapshot {
	result := publicationSnapshot{
		Start: token.Start.Name, End: token.End.Name,
		AttrsNil: token.Start.Attr == nil, DataNil: token.Data == nil, DirectiveNil: token.Content == nil,
		Data: string(token.Data), Directive: string(token.Content),
		StartOffset: token.StartOffset, EndOffset: token.EndOffset,
		Line: token.Line, Column: token.Column, Kind: token.Kind, TextKind: token.TextKind,
	}
	for _, attr := range token.Start.Attr {
		result.Attrs = append(result.Attrs, publicationAttribute{Name: attr.Name, Value: attr.value})
	}
	return result
}

func assertPublicationSnapshot(t *testing.T, token *Token, want publicationSnapshot) {
	t.Helper()
	if got := snapshotPublicationToken(token); !reflect.DeepEqual(got, want) {
		t.Fatalf("pending token changed: got %+v, want %+v", got, want)
	}
}

func assertPublicationTokenZero(t *testing.T, token *Token) {
	t.Helper()
	if token.Start.Name != (xml.Name{}) || token.Start.Attr != nil || token.End.Name != (xml.Name{}) ||
		token.Data != nil || token.Content != nil || token.StartOffset != 0 || token.EndOffset != 0 ||
		token.Line != 0 || token.Column != 0 || token.Kind != KindStart || token.TextKind != CharacterDataInvalid {
		t.Fatalf("invalidated token is not zero: %+v", token)
	}
}

func assertPublicationInactive(t *testing.T, token *Token) {
	t.Helper()
	if token.Kind != KindStart && (token.Start.Name != (xml.Name{}) || token.Start.Attr != nil) {
		t.Fatalf("inactive start payload: %+v", token.Start)
	}
	if token.Kind != KindEnd && token.End.Name != (xml.Name{}) {
		t.Fatalf("inactive end payload: %+v", token.End)
	}
	if token.Kind != KindCharData && token.Kind != KindPI && token.Data != nil {
		t.Fatalf("inactive data payload: %q", token.Data)
	}
	if token.Kind != KindPI && token.Kind != KindComment && token.Content != nil {
		t.Fatalf("inactive directive payload: %q", token.Content)
	}
	if token.Kind != KindCharData && token.TextKind != CharacterDataInvalid {
		t.Fatalf("inactive text origin: %v", token.TextKind)
	}
}

func TestReaderTokenPublicationRetiresPriorPayload(t *testing.T) {
	t.Parallel()
	const input = `<?pre p?><p:r xmlns:p="urn:p" a="v"><p:c b="w"/>text<?target body?><?empty?><!--note--><![CDATA[cdata]]>ref&amp;<empty/></p:r>`
	type event struct {
		name      xml.Name
		attrs     []publicationAttribute
		data      string
		directive string
		start     int
		end       int
		kind      TokenKind
		text      CharacterDataKind
	}
	want := []event{
		{kind: KindPI, data: "pre", directive: "p", start: 0, end: 9},
		{kind: KindStart, name: xml.Name{Space: "p", Local: "r"}, attrs: []publicationAttribute{{Name: xml.Name{Space: "xmlns", Local: "p"}, Value: "urn:p"}, {Name: xml.Name{Local: "a"}, Value: "v"}}, start: 9, end: 36},
		{kind: KindStart, name: xml.Name{Space: "p", Local: "c"}, attrs: []publicationAttribute{{Name: xml.Name{Local: "b"}, Value: "w"}}, start: 36, end: 48},
		{kind: KindEnd, name: xml.Name{Space: "p", Local: "c"}, start: 36, end: 48},
		{kind: KindCharData, data: "text", text: CharacterDataText, start: 48, end: 52},
		{kind: KindPI, data: "target", directive: "body", start: 52, end: 67},
		{kind: KindPI, data: "empty", start: 67, end: 76},
		{kind: KindComment, directive: "note", start: 76, end: 87},
		{kind: KindCharData, data: "cdata", text: CharacterDataCDATA, start: 87, end: 104},
		{kind: KindCharData, data: "ref&", text: CharacterDataReference, start: 104, end: 112},
		{kind: KindStart, name: xml.Name{Local: "empty"}, start: 112, end: 120},
		{kind: KindEnd, name: xml.Name{Local: "empty"}, start: 112, end: 120},
		{kind: KindEnd, name: xml.Name{Space: "p", Local: "r"}, start: 120, end: 126},
	}
	for _, mode := range []struct {
		name   string
		config Config
	}{{name: "emit", config: Config{EmitPI: true, CommentMode: CommentModeEmit, LazyAttrValues: true}}, {name: "skip", config: Config{LazyAttrValues: true}}} {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			var reader Reader
			defer reader.Detach()
			if err := reader.Reset(strings.NewReader(input), mode.config); err != nil {
				t.Fatal(err)
			}
			var held *Token
			var handles []Handle
			for index, expected := range want {
				if mode.name == "skip" && (expected.kind == KindPI || expected.kind == KindComment) {
					continue
				}
				token := mustNextToken(t, &reader)
				if held != nil && token != held {
					t.Fatal("Reader replaced its borrowed token storage")
				}
				held = token
				assertPublicationInactive(t, token)
				if token.Kind != expected.kind || string(token.Data) != expected.data || string(token.Content) != expected.directive || token.TextKind != expected.text || token.StartOffset != expected.start || token.EndOffset != expected.end {
					t.Fatalf("event %d = %+v, want %+v", index, token, expected)
				}
				switch expected.kind {
				case KindStart:
					if token.Start.Name != expected.name || len(token.Start.Attr) != len(expected.attrs) {
						t.Fatalf("start event %d = %+v, want %+v", index, token.Start, expected)
					}
					for i, attr := range expected.attrs {
						value, err := reader.AttrValue(&token.Start.Attr[i])
						if err != nil || token.Start.Attr[i].Name != attr.Name || value != attr.Value {
							t.Fatalf("event %d attribute %d = %+v/%q, want %+v", index, i, token.Start.Attr[i], value, attr)
						}
					}
					handle, expanded, err := reader.Start()
					if err != nil {
						t.Fatal(err)
					}
					wantExpanded := expected.name
					if expected.name.Space == "p" {
						wantExpanded.Space = "urn:p"
					}
					if expanded != wantExpanded {
						t.Fatalf("expanded name = %+v, want %+v", expanded, wantExpanded)
					}
					if index == 1 && token.Start.Attr[0].Name != (xml.Name{Space: "http://www.w3.org/2000/xmlns/", Local: "p"}) {
						t.Fatalf("expanded namespace attribute = %+v", token.Start.Attr[0])
					}
					handles = append(handles, handle)
				case KindEnd:
					if token.End.Name != expected.name || len(handles) == 0 {
						t.Fatalf("end event %d = %+v, want %+v", index, token.End, expected.name)
					}
					if err := reader.endForTest(handles[len(handles)-1]); err != nil {
						t.Fatal(err)
					}
					handles = handles[:len(handles)-1]
				case KindCharData, KindPI, KindComment:
				}
			}
			if token, err := reader.Next(); token != nil || !errors.Is(err, io.EOF) {
				t.Fatalf("document end = %+v, %v", token, err)
			}
			assertPublicationTokenZero(t, held)
			if err := reader.Complete(); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("raw DOCTYPE after PI", func(t *testing.T) {
		t.Parallel()
		var reader Reader
		defer reader.Detach()
		if err := reader.resetParser(strings.NewReader(`<?p body?><!DOCTYPE r>`), Config{EmitPI: true}); err != nil {
			t.Fatal(err)
		}
		var token Token
		if err := reader.parser.next(&token); err != nil {
			t.Fatal(err)
		}
		if token.Kind != KindPI || string(token.Data) != "p" || string(token.Content) != "body" {
			t.Fatalf("PI = %+v", token)
		}
		err := reader.parser.next(&token)
		boundary, ok := errors.AsType[*Error](err)
		if !ok || boundary.kind != errorUnsupported || !errors.Is(err, errUnsupportedDTD) {
			t.Fatalf("DOCTYPE after PI = %v, want the unsupported DTD error", err)
		}
		if line, column := reader.ErrorPos(err); line != 1 || column != 11 {
			t.Fatalf("DOCTYPE position = %d:%d, want 1:11", line, column)
		}
		if token.Kind != KindPI {
			t.Fatalf("held token kind after the failed DOCTYPE = %v, want the PI", token.Kind)
		}
	})
}

func TestReaderTokenPublicationFailureClearsHeldToken(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		input string
		kind  errorKind
	}{{name: "syntax", input: `<?p body?><r a="`, kind: errorSyntax}, {name: "unsupported", input: `<?p body?><!DOCTYPE r>`, kind: errorUnsupported}} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var reader Reader
			defer reader.Detach()
			if err := reader.Reset(strings.NewReader(test.input), Config{EmitPI: true}); err != nil {
				t.Fatal(err)
			}
			held := mustNextToken(t, &reader)
			if held.Kind != KindPI || string(held.Data) != "p" || string(held.Content) != "body" {
				t.Fatalf("populated PI = %+v", held)
			}
			token, err := reader.Next()
			boundary, ok := errors.AsType[*Error](err)
			if token != nil || !ok || boundary.kind != test.kind {
				t.Fatalf("failure = %+v, %v, want kind %v", token, err, test.kind)
			}
			assertPublicationTokenZero(t, held)
		})
	}
}
