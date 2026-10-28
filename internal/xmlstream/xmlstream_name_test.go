package xmlstream

import (
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
)

func TestParserNamesAcrossReadCuts(t *testing.T) {
	for _, test := range []struct {
		input, prefix, local string
		startEnd, endEnd     int
	}{
		{`<r>pad<ab:c z="v"></ab:c></r>`, "ab", "c", 18, 25},
		{`<r>pad<π:名 z="v"></π:名></r>`, "π", "名", 20, 29},
	} {
		for cut := 7; cut < test.endEnd; cut++ {
			t.Run(test.local+"/"+strconv.Itoa(cut), func(t *testing.T) {
				var parser parserTestHarness
				input := &scriptedReader{reads: []scriptedRead{
					{data: []byte(test.input[:cut])}, {data: []byte(test.input[cut:])},
				}}
				if err := parser.reset(input, Config{}); err != nil {
					t.Fatal(err)
				}
				defer parser.detach()
				for range 2 {
					if _, err := parser.next(); err != nil {
						t.Fatal(err)
					}
				}
				start, err := parser.next()
				wantName := xml.Name{Space: test.prefix, Local: test.local}
				if err != nil || start.Kind != KindStart || start.Start.Name != wantName ||
					start.StartOffset != 6 || start.EndOffset != test.startEnd || start.Line != 1 || start.Column != 7 {
					t.Fatalf("start = %+v, %v", start, err)
				}
				if len(start.Start.Attr) != 1 || start.Start.Attr[0].Name != (xml.Name{Local: "z"}) || start.Start.Attr[0].Value != "v" {
					t.Fatalf("attributes = %+v", start.Start.Attr)
				}
				end, err := parser.next()
				if err != nil || end.Kind != KindEnd || end.End.Name != wantName || end.StartOffset != test.startEnd || end.EndOffset != test.endEnd {
					t.Fatalf("end = %+v, %v", end, err)
				}
			})
		}
	}
}

func TestParserNameBufferRolloverOwnsSpelling(t *testing.T) {
	for _, name := range []string{"π:名", "n" + strings.Repeat("a", xmlInputBufferSize+3)} {
		padding := strings.Repeat("x", xmlInputBufferSize-5)
		input := "<r>" + padding + "<" + name + "/>" + strings.Repeat("y", 2*xmlInputBufferSize) + "</r>"
		var parser parserTestHarness
		if err := parser.reset(strings.NewReader(input), Config{}); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err := parser.next(); err != nil {
				t.Fatal(err)
			}
		}
		start, err := parser.next()
		if err != nil || start.Kind != KindStart {
			t.Fatalf("start = %+v, %v", start, err)
		}
		owned := start.Start.Name
		for {
			_, err := parser.next()
			if IsOnlyEOF(err) {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		parser.detach()
		if err := parser.reset(strings.NewReader("<fresh/>"), Config{}); err != nil {
			t.Fatal(err)
		}
		if _, err := parser.next(); err != nil {
			t.Fatal(err)
		}
		parser.detach()
		want := xml.Name{Local: name}
		if name == "π:名" {
			want = xml.Name{Space: "π", Local: "名"}
		}
		if owned != want {
			t.Fatalf("owned spelling = %+v, want %+v", owned, want)
		}
	}
}

func TestParserNamePromotionChargesOnce(t *testing.T) {
	for _, fragmented := range []bool{false, true} {
		for _, limit := range []int64{3, 4} {
			input := `<r>pad<e a="bc"/>`
			var source io.Reader = strings.NewReader(input)
			if fragmented {
				source = &scriptedReader{reads: []scriptedRead{{data: []byte(input[:8])}, {data: []byte(input[8:])}}}
			}
			var parser parserTestHarness
			if err := parser.reset(source, Config{Limits: Limits{MaxTokenBytes: limit}}); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if _, err := parser.next(); err != nil {
					t.Fatal(err)
				}
			}
			_, err := parser.next()
			if limit == 3 && !IsTokenLimit(err) || limit == 4 && err != nil {
				t.Fatalf("fragmented=%v limit=%d: %v", fragmented, limit, err)
			}
			parser.detach()
		}
	}
	name := "n" + strings.Repeat("a", xmlInputBufferSize)
	for _, limit := range []int64{int64(len(name)) - 1, int64(len(name))} {
		var reader Reader
		if err := reader.Reset(strings.NewReader("<"+name+"/>"), Config{Limits: Limits{MaxTokenBytes: limit}}); err != nil {
			t.Fatal(err)
		}
		token, err := reader.Next()
		if limit < int64(len(name)) {
			if token != nil || !IsTokenLimit(err) {
				t.Fatalf("short allowance = %v, %v", token, err)
			}
		} else if err != nil || token.Start.Name.Local != name {
			t.Fatalf("exact allowance = %v, %v", token, err)
		}
		reader.Detach()
	}
}

func TestParserNameFailurePrecedence(t *testing.T) {
	for _, suffix := range []string{"<1a/>", "<a:b:c ?", "<π:1 ?"} {
		for _, cut := range []int{7, 8} {
			input := "<r>pad" + suffix
			var parser parserTestHarness
			if err := parser.reset(&scriptedReader{reads: []scriptedRead{{data: []byte(input[:cut])}, {data: []byte(input[cut:])}}}, Config{}); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if _, err := parser.next(); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := parser.next(); err == nil || err.Error() != "invalid XML qualified name" {
				t.Fatalf("%q cut=%d: %v", suffix, cut, err)
			}
			parser.detach()
		}
	}
}

func TestReaderNameReadFailureAndReuse(t *testing.T) {
	readErr := errors.New("name read failed")
	for _, cause := range []error{io.EOF, readErr} {
		var reader Reader
		if err := reader.Reset(&scriptedReader{reads: []scriptedRead{{data: []byte("<r>pad<ab"), err: cause}}}, Config{}); err != nil {
			t.Fatal(err)
		}
		if _, err := reader.Next(); err != nil {
			t.Fatal(err)
		}
		if _, _, err := reader.Start(); err != nil {
			t.Fatal(err)
		}
		if _, err := reader.Next(); err != nil {
			t.Fatal(err)
		}
		token, err := reader.Next()
		boundary, ok := errors.AsType[*Error](err)
		if token != nil || !ok || boundary.Kind != ErrorSyntax || boundary.Line != 1 || boundary.Column != 9 || IsOnlyEOF(err) {
			t.Fatalf("name failure = %+v, %v", token, err)
		}
		if errors.Is(cause, readErr) && !errors.Is(err, readErr) {
			t.Fatalf("source cause lost: %v", err)
		}
		if errors.Is(cause, io.EOF) && !strings.Contains(err.Error(), "unexpected EOF in XML name") {
			t.Fatalf("incomplete name = %v", err)
		}
		if err := reader.Reset(strings.NewReader("<fresh/>"), Config{}); err != nil {
			t.Fatal(err)
		}
		if err := drainReader(&reader); err != nil {
			t.Fatal(err)
		}
		reader.Detach()
	}
}

func TestParserCachedComponentsRemainLexicalNames(t *testing.T) {
	var parser parserTestHarness
	for _, input := range []string{`<p:seed π:名="v"/>`, `<p/>`, `<π/>`, `<seed/>`, `<名/>`} {
		if err := parser.reset(strings.NewReader(input), Config{}); err != nil {
			t.Fatal(err)
		}
		token, err := parser.next()
		if err != nil {
			t.Fatal(err)
		}
		if input != `<p:seed π:名="v"/>` {
			want := xml.Name{Local: strings.TrimSuffix(strings.TrimPrefix(input, "<"), "/>")}
			if token.Start.Name != want {
				t.Fatalf("%s name = %+v, want %+v", input, token.Start.Name, want)
			}
		}
		parser.detach()
	}
	for range 2 {
		for _, name := range []string{"p:", ":p", "p:seed:名", "p:1", "1p", "π:1"} {
			parser.reader.InternBytes([]byte(name))
			if err := parser.reset(strings.NewReader("<"+name+"/>"), Config{}); err != nil {
				t.Fatal(err)
			}
			if _, err := parser.next(); err == nil || err.Error() != "invalid XML qualified name" {
				t.Fatalf("name %q: %v", name, err)
			}
			parser.detach()
		}
	}
}

func TestReaderWarmNameStillChargesAndPreservesReadErrors(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader("<warm/>"), Config{}); err != nil {
		t.Fatal(err)
	}
	if err := drainReader(&reader); err != nil {
		t.Fatal(err)
	}
	if err := reader.Reset(strings.NewReader("<warm/>"), Config{Limits: Limits{MaxTokenBytes: 3}}); err != nil {
		t.Fatal(err)
	}
	if token, err := reader.Next(); token != nil || !IsTokenLimit(err) {
		t.Fatalf("warm name limit = %v, %v", token, err)
	}
	readErr := errors.New("warm name read failed")
	if err := reader.Reset(&scriptedReader{reads: []scriptedRead{{data: []byte("<r>pad<warm"), err: readErr}}}, Config{}); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := reader.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Next(); err != nil {
		t.Fatal(err)
	}
	if token, err := reader.Next(); token != nil || !errors.Is(err, readErr) {
		t.Fatalf("warm incomplete name = %v, %v", token, err)
	}
	reader.Detach()
}

func TestParserQNameAdmissionAcrossRefills(t *testing.T) {
	for _, test := range []struct {
		name  string
		want  xml.Name
		valid bool
	}{
		{"_n.a-1", xml.Name{Local: "_n.a-1"}, true},
		{"p:local", xml.Name{Space: "p", Local: "local"}, true},
		{"π:local", xml.Name{Space: "π", Local: "local"}, true},
		{"p:名", xml.Name{Space: "p", Local: "名"}, true},
		{"π:名", xml.Name{Space: "π", Local: "名"}, true},
		{"1name", xml.Name{}, false},
		{":p", xml.Name{}, false},
		{"p:", xml.Name{}, false},
		{"p:1", xml.Name{}, false},
		{"p:l:x", xml.Name{}, false},
		{"p:@", xml.Name{}, false},
		{"1π", xml.Name{}, false},
		{"π:1", xml.Name{}, false},
		{"p:\xff", xml.Name{}, false},
		{"ab:cd", xml.Name{Space: "ab", Local: "cd"}, true},
		{"a-b.c1:d-e.f2", xml.Name{Space: "a-b.c1", Local: "d-e.f2"}, true},
		{":", xml.Name{}, false},
		{"::", xml.Name{}, false},
		{"a@b", xml.Name{}, false},
		{"p:l@", xml.Name{}, false},
		{"-a", xml.Name{}, false},
		{".a", xml.Name{}, false},
		{"p:-", xml.Name{}, false},
		{"p:.", xml.Name{}, false},
		{"p::l", xml.Name{}, false},
		{"p:l:", xml.Name{}, false},
		{"@é", xml.Name{}, false},
		{"a@é", xml.Name{}, false},
		{"é@", xml.Name{}, false},
		{"p\xff", xml.Name{}, false},
	} {
		input := "<r>pad<" + test.name + "/>"
		for cut := 7; cut <= 7+len(test.name); cut++ {
			var parser parserTestHarness
			if err := parser.reset(&scriptedReader{reads: []scriptedRead{{data: []byte(input[:cut])}, {data: []byte(input[cut:])}}}, Config{}); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if _, err := parser.next(); err != nil {
					t.Fatal(err)
				}
			}
			token, err := parser.next()
			if test.valid {
				if err != nil || token.Kind != KindStart || token.Start.Name != test.want {
					t.Fatalf("name %q cut %d = %+v, %v, want %+v", test.name, cut, token, err, test.want)
				}
			} else if err == nil || err.Error() != "invalid XML qualified name" {
				t.Fatalf("name %q cut %d = %v", test.name, cut, err)
			}
			if err := parser.reset(strings.NewReader("<fresh/>"), Config{}); err != nil {
				t.Fatal(err)
			}
			if token, err := parser.next(); err != nil || token.Start.Name != (xml.Name{Local: "fresh"}) {
				t.Fatalf("reuse after name %q cut %d = %+v, %v", test.name, cut, token, err)
			}
			parser.detach()
		}
	}
}

func TestParserInvalidQNameFailurePrecedence(t *testing.T) {
	for _, limit := range []int64{3, 4} {
		var parser parserTestHarness
		if err := parser.reset(strings.NewReader("<r>pad<1abc/>"), Config{Limits: Limits{MaxTokenBytes: limit}}); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if _, err := parser.next(); err != nil {
				t.Fatal(err)
			}
		}
		_, err := parser.next()
		if limit == 3 {
			if !IsTokenLimit(err) {
				t.Fatalf("insufficient allowance: %v", err)
			}
		} else if err == nil || err.Error() != "invalid XML qualified name" {
			t.Fatalf("exact allowance: %v", err)
		}
		parser.detach()
	}
	readErr := errors.New("invalid QName source failed")
	var parser parserTestHarness
	if err := parser.reset(&scriptedReader{reads: []scriptedRead{{data: []byte("<r>pad<1abc"), err: readErr}}}, Config{}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := parser.next(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := parser.next(); !errors.Is(err, readErr) {
		t.Fatalf("source failure: %v", err)
	}
	parser.detach()
}

// A name whose invalid byte follows valid ones still lets a token limit or a
// read failure before its terminator win over the syntax error.
func TestParserLaterInvalidQNameFailurePrecedence(t *testing.T) {
	readErr := errors.New("invalid QName source failed")
	for _, name := range []string{"ab@cd", "a:b:c", "p:l:", "a-:1", "ab\xffc"} {
		document := "<r>pad<" + name + "/>"
		truncated := "<r>pad<" + name
		for _, cut := range []int{0, len("<r>pad<") + len(name)/2} {
			split := func(input string, err error) io.Reader {
				if cut == 0 {
					return &scriptedReader{reads: []scriptedRead{{data: []byte(input), err: err}}}
				}
				return &scriptedReader{reads: []scriptedRead{{data: []byte(input[:cut])}, {data: []byte(input[cut:]), err: err}}}
			}
			for _, test := range []struct {
				input  io.Reader
				limit  int64
				check  func(error) bool
				expect string
			}{
				{input: split(document, nil), limit: int64(len(name) - 1), check: IsTokenLimit, expect: "token limit"},
				{input: split(document, nil), limit: int64(len(name)), check: func(err error) bool {
					return err != nil && err.Error() == "invalid XML qualified name"
				}, expect: "invalid XML qualified name"},
				{input: split(truncated, readErr), check: func(err error) bool { return errors.Is(err, readErr) }, expect: "read error"},
				{input: split(truncated, io.EOF), check: func(err error) bool {
					return err != nil && err.Error() == "unexpected EOF in XML name"
				}, expect: "unexpected EOF in XML name"},
			} {
				var parser parserTestHarness
				if err := parser.reset(test.input, Config{Limits: Limits{MaxTokenBytes: test.limit}}); err != nil {
					t.Fatal(err)
				}
				for range 2 {
					if _, err := parser.next(); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := parser.next(); !test.check(err) {
					t.Fatalf("name %q cut %d: error = %v, want %s", name, cut, err, test.expect)
				}
				parser.detach()
			}
		}
	}
}

// A name whose first byte would end any other name is still that name's only
// byte, so it is rejected as a QName rather than read as an empty name.
func TestParserRejectsTerminatorAsFirstNameByte(t *testing.T) {
	for _, document := range []string{"<>/>", "<=/>", "< a/>", "<r =/>", `<r ="v"/>`, "<r></>/"} {
		for cut := 0; cut <= len(document); cut++ {
			var parser parserTestHarness
			if err := parser.reset(&scriptedReader{reads: []scriptedRead{
				{data: []byte(document[:cut])}, {data: []byte(document[cut:])},
			}}, Config{}); err != nil {
				t.Fatal(err)
			}
			var err error
			for range 3 {
				if _, err = parser.next(); err != nil {
					break
				}
			}
			if err == nil || err.Error() != "invalid XML qualified name" {
				t.Fatalf("%q cut %d: error = %v, want invalid XML qualified name", document, cut, err)
			}
			parser.detach()
		}
	}
}
