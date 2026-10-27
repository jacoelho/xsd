package xmlstream

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

type retirementBenchCheck uint8

const (
	retirementBenchCheap retirementBenchCheck = iota
	retirementBenchFull
)

type retirementBenchScenario uint8

const (
	retirementBenchDocument retirementBenchScenario = iota
	retirementBenchFailure
)

type retirementBenchSample struct {
	starts, ends, pis, comments                        int
	textBytes, targetBytes, directiveBytes, attributes int
}

type retirementBenchEvent struct {
	name            xml.Name
	data, directive string
	attrs           []xml.Attr
	kind            TokenKind
	origin          CharacterDataKind
}

type retirementBenchFixture struct {
	document, text string
	events         []retirementBenchEvent
	config         Config
	want           retirementBenchSample
	origin         CharacterDataKind
	failure        ErrorKind
	scenario       retirementBenchScenario
}

// Full preflights check literal payloads and ownership; timed operations retain
// only cheap counters, so reflection and byte hashing cannot hide producer costs.
func (f retirementBenchFixture) drain(reader *Reader, input *strings.Reader, check retirementBenchCheck) (retirementBenchSample, error) {
	input.Reset(f.document)
	if err := reader.Reset(input, f.config); err != nil {
		return retirementBenchSample{}, err
	}
	var sample retirementBenchSample
	var storage [4]Handle
	handles := storage[:0]
	eventIndex, textOffset := 0, 0
	var held *Token
	for {
		token, err := reader.Next()
		if err != nil {
			if f.scenario == retirementBenchFailure {
				var boundary *Error
				if !errors.As(err, &boundary) || boundary.Kind != f.failure || (f.failure == ErrorLimit && !IsTokenLimit(err)) {
					return sample, fmt.Errorf("failure class = %w, want %v", err, f.failure)
				}
				if check == retirementBenchFull && (token != nil || held == nil || !retirementBenchTokenZero(held)) {
					return sample, errors.New("failed Reader did not invalidate its populated token")
				}
				return sample, nil
			}
			if !errors.Is(err, io.EOF) {
				return sample, err
			}
			if token != nil || len(handles) != 0 {
				return sample, errors.New("unexpected token or open element at EOF")
			}
			if check == retirementBenchFull && (textOffset != len(f.text) || eventIndex != len(f.events)) {
				return sample, errors.New("literal text or publication event sequence incomplete")
			}
			return sample, reader.Complete()
		}
		held = token
		if check == retirementBenchFull {
			if err := f.checkToken(reader, token, eventIndex, textOffset); err != nil {
				return sample, err
			}
			if len(f.events) != 0 {
				eventIndex++
			}
			if token.Kind == KindCharData {
				textOffset += len(token.Data)
			}
		}
		switch token.Kind {
		case KindStart:
			sample.starts++
			sample.attributes += len(token.Start.Attr)
			if len(handles) == cap(handles) {
				return sample, errors.New("fixture exceeded its fixed handle stack")
			}
			handle, expanded, startErr := reader.Start()
			if startErr != nil {
				return sample, startErr
			}
			if check == retirementBenchFull && len(f.events) != 0 {
				want := f.events[eventIndex-1].name
				if want.Space == "p" {
					want.Space = "urn:p"
				}
				if expanded != want {
					return sample, fmt.Errorf("expanded name = %v, want %v", expanded, want)
				}
			}
			handles = append(handles, handle)
		case KindEnd:
			sample.ends++
			if len(handles) == 0 {
				return sample, errors.New("unexpected closing fixture element")
			}
			handle := handles[len(handles)-1]
			handles = handles[:len(handles)-1]
			if err := reader.MatchEnd(handle); err != nil {
				return sample, err
			}
			if err := reader.CommitEnd(handle); err != nil {
				return sample, err
			}
		case KindCharData:
			sample.textBytes += len(token.Data)
		case KindPI:
			sample.pis++
			sample.targetBytes += len(token.Data)
			sample.directiveBytes += len(token.Directive)
		case KindComment:
			sample.comments++
			sample.directiveBytes += len(token.Directive)
		case KindDirective:
			return sample, errors.New("Reader unexpectedly admitted a directive")
		}
	}
}

func retirementBenchTokenZero(token *Token) bool {
	return token.Kind == KindStart && token.Start.Name == (xml.Name{}) && token.Start.Attr == nil &&
		token.End.Name == (xml.Name{}) && token.Data == nil && token.Directive == nil &&
		token.TextKind == CharacterDataInvalid && token.StartOffset == 0 && token.EndOffset == 0 && token.Line == 0 && token.Column == 0
}

func (f retirementBenchFixture) checkToken(reader *Reader, token *Token, eventIndex, textOffset int) error {
	if token.Kind != KindStart && (token.Start.Name != (xml.Name{}) || token.Start.Attr != nil) ||
		token.Kind != KindEnd && token.End.Name != (xml.Name{}) ||
		token.Kind != KindCharData && token.Kind != KindPI && token.Data != nil ||
		token.Kind != KindPI && token.Kind != KindComment && token.Kind != KindDirective && token.Directive != nil ||
		token.Kind != KindCharData && token.TextKind != CharacterDataInvalid {
		return errors.New("inactive token payload was retained")
	}
	if len(f.events) == 0 {
		if token.Kind == KindCharData {
			end := textOffset + len(token.Data)
			if end > len(f.text) || !bytes.Equal(token.Data, []byte(f.text[textOffset:end])) || token.TextKind != f.origin {
				return errors.New("text payload or origin differs from the independent literal")
			}
		}
		if token.Kind == KindPI && (!bytes.Equal(token.Data, []byte("p")) || !bytes.Equal(token.Directive, []byte("body"))) {
			return errors.New("failure prefix PI differs from its literal")
		}
		return nil
	}
	if eventIndex >= len(f.events) {
		return errors.New("unexpected extra publication")
	}
	want := f.events[eventIndex]
	if token.Kind != want.kind || !bytes.Equal(token.Data, []byte(want.data)) || !bytes.Equal(token.Directive, []byte(want.directive)) || token.TextKind != want.origin {
		return fmt.Errorf("publication %d payload differs from literal expectation", eventIndex)
	}
	if token.Kind == KindStart {
		if token.Start.Name != want.name || len(token.Start.Attr) != len(want.attrs) {
			return fmt.Errorf("publication %d start differs from literal expectation", eventIndex)
		}
		for i, attr := range want.attrs {
			value, ok := reader.MaterializeValue(&token.Start.Attr[i])
			if !ok || token.Start.Attr[i].Name != attr.Name || value != attr.Value {
				return fmt.Errorf("publication %d attribute %d differs from literal expectation", eventIndex, i)
			}
		}
	}
	if token.Kind == KindEnd && token.End.Name != want.name {
		return fmt.Errorf("publication %d end differs from literal expectation", eventIndex)
	}
	return nil
}

func (f retirementBenchFixture) run(reader *Reader, input *strings.Reader, check retirementBenchCheck) (retirementBenchSample, error) {
	got, err := f.drain(reader, input, check)
	if err != nil || f.scenario != retirementBenchFailure {
		return got, err
	}
	retry := retirementBenchFixture{document: "<fresh/>"}
	after, err := retry.drain(reader, input, check)
	if err != nil {
		return got, err
	}
	got.starts += after.starts
	got.ends += after.ends
	return got, nil
}

func benchmarkRetirementReader(b *testing.B, fixture retirementBenchFixture) {
	b.Helper()
	var reader Reader
	var input strings.Reader
	defer reader.Detach()
	got, err := fixture.run(&reader, &input, retirementBenchFull)
	if err != nil || got != fixture.want {
		b.Fatalf("semantic preflight = %+v, %v; want %+v", got, err, fixture.want)
	}
	if fixture.scenario != retirementBenchFailure {
		b.SetBytes(int64(len(fixture.document)))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		got, err = fixture.run(&reader, &input, retirementBenchCheap)
		if err != nil || got != fixture.want {
			b.Fatalf("consumed sample = %+v, %v; want %+v", got, err, fixture.want)
		}
	}
	got, err = fixture.run(&reader, &input, retirementBenchFull)
	if err != nil || got != fixture.want {
		b.Fatalf("semantic postflight = %+v, %v; want %+v", got, err, fixture.want)
	}
}

func retirementPublicationFixture() retirementBenchFixture {
	return retirementBenchFixture{
		document: `<?pre p?><p:r xmlns:p="urn:p" a="v"><p:c b="w"/>text<?target body?><?empty?><!--note--><![CDATA[cdata]]>ref&amp;<empty/></p:r>`,
		text:     "textcdataref&",
		config:   Config{EmitPI: true, CommentMode: CommentModeEmit, LazyAttrValues: true},
		want:     retirementBenchSample{starts: 3, ends: 3, pis: 3, comments: 1, textBytes: 13, targetBytes: 14, directiveBytes: 9, attributes: 3},
		events: []retirementBenchEvent{
			{kind: KindPI, data: "pre", directive: "p"},
			{kind: KindStart, name: xml.Name{Space: "p", Local: "r"}, attrs: []xml.Attr{{Name: xml.Name{Space: "xmlns", Local: "p"}, Value: "urn:p"}, {Name: xml.Name{Local: "a"}, Value: "v"}}},
			{kind: KindStart, name: xml.Name{Space: "p", Local: "c"}, attrs: []xml.Attr{{Name: xml.Name{Local: "b"}, Value: "w"}}},
			{kind: KindEnd, name: xml.Name{Space: "p", Local: "c"}},
			{kind: KindCharData, data: "text", origin: CharacterDataText},
			{kind: KindPI, data: "target", directive: "body"},
			{kind: KindPI, data: "empty"},
			{kind: KindComment, directive: "note"},
			{kind: KindCharData, data: "cdata", origin: CharacterDataCDATA},
			{kind: KindCharData, data: "ref&", origin: CharacterDataReference},
			{kind: KindStart, name: xml.Name{Local: "empty"}},
			{kind: KindEnd, name: xml.Name{Local: "empty"}},
			{kind: KindEnd, name: xml.Name{Space: "p", Local: "r"}},
		},
	}
}

func BenchmarkReaderTokenRetirement(b *testing.B) {
	b.Run("short_and_empty_cdata", func(b *testing.B) {
		benchmarkRetirementReader(b, retirementBenchFixture{
			document: "<r>" + strings.Repeat("<![CDATA[x]]><![CDATA[]]>", 256) + "</r>",
			text:     strings.Repeat("x", 256), origin: CharacterDataCDATA,
			want: retirementBenchSample{starts: 1, ends: 1, textBytes: 256},
		})
	})
	b.Run("long_cdata_then_short", func(b *testing.B) {
		text := strings.Repeat("0123456789abcdef", 12289)
		benchmarkRetirementReader(b, retirementBenchFixture{
			document: "<r><![CDATA[" + text + "]]><![CDATA[!]]></r>", text: text + "!", origin: CharacterDataCDATA,
			want: retirementBenchSample{starts: 1, ends: 1, textBytes: 196625},
		})
	})
	b.Run("publication_emit", func(b *testing.B) {
		benchmarkRetirementReader(b, retirementPublicationFixture())
	})
	b.Run("publication_skip", func(b *testing.B) {
		fixture := retirementPublicationFixture()
		fixture.config = Config{LazyAttrValues: true}
		fixture.want = retirementBenchSample{starts: 3, ends: 3, textBytes: 13, attributes: 3}
		var events []retirementBenchEvent
		for _, event := range fixture.events {
			if event.kind != KindPI && event.kind != KindComment {
				events = append(events, event)
			}
		}
		fixture.events = events
		benchmarkRetirementReader(b, fixture)
	})
	b.Run("space_refill", func(b *testing.B) {
		text := strings.Repeat(" ", 131089)
		benchmarkRetirementReader(b, retirementBenchFixture{document: "<r>" + text + "</r>", text: text,
			origin: CharacterDataText, want: retirementBenchSample{starts: 1, ends: 1, textBytes: 131089}})
	})
	b.Run("unicode_newline_refill", func(b *testing.B) {
		benchmarkRetirementReader(b, retirementBenchFixture{
			document: "<r>" + strings.Repeat("é\r\n名\n\n", 16384) + "</r>", text: strings.Repeat("é\n名\n\n", 16384),
			origin: CharacterDataText, want: retirementBenchSample{starts: 1, ends: 1, textBytes: 131072},
		})
	})
	b.Run("syntax_reset", func(b *testing.B) {
		benchmarkRetirementReader(b, retirementBenchFixture{
			document: `<?p body?><r a="`, config: Config{EmitPI: true}, scenario: retirementBenchFailure, failure: ErrorSyntax,
			want: retirementBenchSample{starts: 1, ends: 1, pis: 1, targetBytes: 1, directiveBytes: 4},
		})
	})
	b.Run("token_limit_reset", func(b *testing.B) {
		benchmarkRetirementReader(b, retirementBenchFixture{
			document: `<?p body?><r>123456789</r>`, config: Config{EmitPI: true, Limits: Limits{MaxTokenBytes: 8}},
			scenario: retirementBenchFailure, failure: ErrorLimit,
			want: retirementBenchSample{starts: 2, ends: 1, pis: 1, targetBytes: 1, directiveBytes: 4},
		})
	})
}

func BenchmarkParserTokenRetirement(b *testing.B) {
	b.Run("PI_to_directive", func(b *testing.B) {
		var reader Reader
		var input strings.Reader
		var token Token
		defer reader.Detach()
		operation := func() error {
			input.Reset(`<?p body?><!DOCTYPE r>`)
			if err := reader.resetParser(&input, Config{EmitPI: true}); err != nil {
				return err
			}
			if err := reader.parser.next(&token); err != nil {
				return err
			}
			if token.Kind != KindPI || !bytes.Equal(token.Data, []byte("p")) || !bytes.Equal(token.Directive, []byte("body")) {
				return errors.New("raw first publication differs from literal PI")
			}
			if err := reader.parser.next(&token); err != nil {
				return err
			}
			if token.Kind != KindDirective || !bytes.Equal(token.Directive, []byte("DOCTYPE ")) || token.Data != nil ||
				token.Start.Name != (xml.Name{}) || token.Start.Attr != nil || token.End.Name != (xml.Name{}) || token.TextKind != CharacterDataInvalid {
				return errors.New("raw second publication differs from literal directive")
			}
			return nil
		}
		if err := operation(); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			if err := operation(); err != nil {
				b.Fatal(err)
			}
		}
		if err := operation(); err != nil {
			b.Fatal(err)
		}
	})
}
