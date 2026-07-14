package stream

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"testing"
)

func TestParserResetClassifiesXMLProlog(t *testing.T) {
	tests := []struct {
		name string
		xml  string
		want error
	}{
		{name: "UTF-16 BE", xml: string([]byte{0xFE, 0xFF}) + `<r/>`, want: ErrUnsupportedNonUTF8},
		{name: "UTF-16 LE", xml: string([]byte{0xFF, 0xFE}) + `<r/>`, want: ErrUnsupportedNonUTF8},
		{name: "declared encoding", xml: `<?xml version="1.0" encoding="latin1"?><r/>`, want: ErrUnsupportedNonUTF8},
		{name: "XML 1.1", xml: `<?xml version="1.1"?><r/>`, want: UnsupportedXMLVersionError{Version: "1.1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var parser Parser
			err := parser.Reset(strings.NewReader(test.xml), nil, nil)
			if !errors.Is(err, test.want) {
				var gotVersion UnsupportedXMLVersionError
				var wantVersion UnsupportedXMLVersionError
				if !errors.As(err, &gotVersion) || !errors.As(test.want, &wantVersion) || gotVersion != wantVersion {
					t.Fatalf("Parser.Reset() error = %v, want %v", err, test.want)
				}
			}
			if parser.br.r != nil {
				t.Fatal("failed Parser.Reset() retained reader")
			}
		})
	}
}

func TestParserResetConsumesBOMWithoutShiftingPosition(t *testing.T) {
	var positions [2][2]int
	for i, bom := range []string{"", "\ufeff"} {
		var parser Parser
		names, values := NewCache(), NewCache()
		if err := parser.Reset(strings.NewReader(bom+"<?xml version=\"1.0\"?><root/>"), &names, &values); err != nil {
			t.Fatal(err)
		}
		tok, err := parser.Next()
		if err != nil {
			t.Fatal(err)
		}
		if tok.Kind != KindStart || tok.Start.Name.Local != "root" {
			t.Fatalf("first token = %+v, want root", tok)
		}
		positions[i] = [2]int{tok.Line, tok.Column}
	}
	if positions[0] != positions[1] {
		t.Fatalf("root position without/with BOM = %v/%v", positions[0], positions[1])
	}
}

func TestParserResetBOMPreservesFullDeclarationPreview(t *testing.T) {
	prefix := `<?xml version="1.0" `
	suffix := `encoding="ISO-8859-1"?>`
	declaration := prefix + strings.Repeat(" ", xmlInputBufferSize-len(prefix)-len(suffix)) + suffix
	for _, bom := range []string{"", "\ufeff"} {
		var parser Parser
		err := parser.Reset(strings.NewReader(bom+declaration+`<r/>`), nil, nil)
		if !errors.Is(err, ErrUnsupportedNonUTF8) {
			t.Fatalf("BOM %t: Parser.Reset() error = %v, want %v", bom != "", err, ErrUnsupportedNonUTF8)
		}
	}
}

type noProgressReader struct{}

func (noProgressReader) Read([]byte) (int, error) { return 0, nil }

func TestParserResetRejectsNoProgressAndDetaches(t *testing.T) {
	var parser Parser
	err := parser.Reset(noProgressReader{}, nil, nil)
	if !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("Parser.Reset() error = %v, want %v", err, io.ErrNoProgress)
	}
	if parser.br.r != nil {
		t.Fatal("failed Parser.Reset() retained no-progress reader")
	}
}

type dataErrorReader struct {
	data []byte
	err  error
	done bool
}

func (r *dataErrorReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.data), r.err
}

func TestParserResetDefersErrorReturnedWithBufferedDocument(t *testing.T) {
	sentinel := errors.New("sentinel")
	names, values := NewCache(), NewCache()
	var parser Parser
	if err := parser.Reset(&dataErrorReader{data: []byte(`<root/>`), err: sentinel}, &names, &values); err != nil {
		t.Fatalf("Parser.Reset() error = %v", err)
	}
	for range 2 {
		if _, err := parser.Next(); err != nil {
			t.Fatalf("Parser.Next() buffered token error = %v", err)
		}
	}
	if _, err := parser.Next(); !errors.Is(err, sentinel) {
		t.Fatalf("Parser.Next() terminal error = %v, want %v", err, sentinel)
	}
}

func TestParserResetReturnsErrorWithShortPrefix(t *testing.T) {
	sentinel := errors.New("sentinel")
	reader := &dataErrorReader{data: []byte(`<r`), err: sentinel}
	var parser Parser
	if err := parser.Reset(reader, nil, nil); !errors.Is(err, sentinel) {
		t.Fatalf("Parser.Reset() error = %v, want %v", err, sentinel)
	}
	if parser.br.r != nil {
		t.Fatal("failed Parser.Reset() retained short-prefix reader")
	}
}

type chunkReader struct {
	r io.Reader
	n int
}

func (r chunkReader) Read(p []byte) (int, error) {
	if len(p) > r.n {
		p = p[:r.n]
	}
	return r.r.Read(p)
}

func consumeWithInputLimit(r io.Reader, limit int64) error {
	var parser Parser
	names, values := NewCache(), NewCache()
	if err := parser.ResetWithLimits(r, &names, &values, Limits{MaxInputBytes: limit}); err != nil {
		return err
	}
	defer parser.Detach()
	for {
		if _, err := parser.Next(); err != nil {
			return err
		}
	}
}

func TestParserInputByteLimitAcceptsExactBoundary(t *testing.T) {
	doc := `<r/>`
	err := consumeWithInputLimit(strings.NewReader(doc), int64(len(doc)))
	if !errors.Is(err, io.EOF) {
		t.Fatalf("consumeWithInputLimit() error = %v, want EOF", err)
	}
}

func TestParserInputByteLimitWithDataAndEOF(t *testing.T) {
	doc := []byte(`<r/>`)
	if err := consumeWithInputLimit(&eofWithDataReader{data: doc}, int64(len(doc))); !errors.Is(err, io.EOF) {
		t.Fatalf("consumeWithInputLimit(exact) error = %v, want EOF", err)
	}
	over := append(append([]byte(nil), doc...), 'X')
	if err := consumeWithInputLimit(&eofWithDataReader{data: over}, int64(len(doc))); !IsInputLimit(err) {
		t.Fatalf("consumeWithInputLimit(over) error = %v, want input limit", err)
	}
}

func TestParserInputByteLimitAcceptsMaxInt64(t *testing.T) {
	err := consumeWithInputLimit(strings.NewReader(`<r/>`), math.MaxInt64)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("consumeWithInputLimit() error = %v, want EOF", err)
	}
}

func TestParserInputByteLimitRejectsFirstExcessByteAcrossReadSizes(t *testing.T) {
	doc := `<root attr="value">text</root>`
	for _, chunkSize := range []int{1, 2, 7, len(doc)} {
		t.Run(fmt.Sprintf("chunk=%d", chunkSize), func(t *testing.T) {
			r := chunkReader{r: strings.NewReader(doc), n: chunkSize}
			err := consumeWithInputLimit(r, int64(len(doc)-1))
			if !IsInputLimit(err) {
				t.Fatalf("consumeWithInputLimit() error = %v, want input limit", err)
			}
		})
	}
}

func TestParserInputByteLimitCountsUTF8BOM(t *testing.T) {
	doc := `<r/>`
	err := consumeWithInputLimit(strings.NewReader("\ufeff"+doc), int64(len(doc)))
	if !IsInputLimit(err) {
		t.Fatalf("consumeWithInputLimit() error = %v, want input limit", err)
	}
}

func TestParserInputByteLimitPreservesSimultaneousReaderError(t *testing.T) {
	sentinel := errors.New("sentinel")
	err := consumeWithInputLimit(
		&dataErrorReader{data: []byte(`<root/>`), err: sentinel},
		int64(len(`<root/>`)-1),
	)
	if !IsInputLimit(err) || !errors.Is(err, sentinel) {
		t.Fatalf("consumeWithInputLimit() error = %v, want input limit and sentinel", err)
	}
}

func TestParserPreservesJoinedErrorAfterBareCR(t *testing.T) {
	sentinel := errors.New("sentinel")
	names, values := NewCache(), NewCache()
	var parser Parser
	if err := parser.Reset(&dataErrorReader{data: []byte(`<r/>\r`), err: errors.Join(io.EOF, sentinel)}, &names, &values); err != nil {
		t.Fatalf("Parser.Reset() error = %v", err)
	}
	defer parser.Detach()
	for range 2 {
		if _, err := parser.Next(); err != nil {
			t.Fatalf("Parser.Next(element) error = %v", err)
		}
	}
	if _, err := parser.Next(); !errors.Is(err, sentinel) {
		t.Fatalf("Parser.Next(CR) error = %v, want sentinel", err)
	}
}

func TestParserResetLeavesMalformedDeclarationForTokenizer(t *testing.T) {
	names, values := NewCache(), NewCache()
	var parser Parser
	if err := parser.Reset(strings.NewReader(`<?xml version="1.0" bad?><r/>`), &names, &values); err != nil {
		t.Fatalf("Parser.Reset() rejected syntax: %v", err)
	}
	if _, err := parser.Next(); err == nil {
		t.Fatal("Parser.Next() accepted malformed XML declaration")
	}
}

func TestParserNextPreservesUnsupportedDeclarationClassificationBeyondPreview(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    error
	}{
		{name: "version", content: `version="1.1"`, want: UnsupportedXMLVersionError{Version: "1.1"}},
		{name: "encoding", content: `version="1.0" encoding="ISO-8859-1"`, want: ErrUnsupportedNonUTF8},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			declaration := `<?xml` + strings.Repeat(" ", maxXMLDeclarationPreviewBytes+1) + test.content + `?><r/>`
			names, values := NewCache(), NewCache()
			var parser Parser
			if err := parser.Reset(strings.NewReader(declaration), &names, &values); err != nil {
				t.Fatalf("Parser.Reset() error = %v", err)
			}
			_, err := parser.Next()
			if !errors.Is(err, test.want) {
				var gotVersion UnsupportedXMLVersionError
				var wantVersion UnsupportedXMLVersionError
				if !errors.As(err, &gotVersion) || !errors.As(test.want, &wantVersion) || gotVersion != wantVersion {
					t.Fatalf("Parser.Next() error = %v, want %v", err, test.want)
				}
			}
		})
	}
}

func TestParserDetachDoesNotResetCallerBufferedReader(t *testing.T) {
	callerReader := bufio.NewReaderSize(strings.NewReader("<root/>"), xmlInputBufferSize*2)
	var parser Parser
	if err := parser.Reset(callerReader, nil, nil); err != nil {
		t.Fatal(err)
	}
	parser.Detach()
	if callerReader.Size() != xmlInputBufferSize*2 {
		t.Fatal("Parser.Detach() mutated caller-owned buffered reader")
	}
	if parser.br.r != nil {
		t.Fatal("Parser.Detach() retained caller reader")
	}
}
