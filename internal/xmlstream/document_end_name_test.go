package xmlstream

import (
	"encoding/xml"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
)

func TestReaderEndNameSurvivesTrailingWhitespaceRefills(t *testing.T) {
	for _, name := range []string{"root", "π:名"} {
		start := "<" + name + ">"
		want := xml.Name{Local: "root"}
		if name == "π:名" {
			start = `<π:名 xmlns:π="urn:test">`
			want = xml.Name{Space: "π", Local: "名"}
		}
		input := start + "</" + name + strings.Repeat(" \r\n", xmlInputBufferSize) + ">"
		var reader Reader
		if err := reader.Reset(strings.NewReader(input), Config{}); err != nil {
			t.Fatal(err)
		}
		mustNextToken(t, &reader)
		handle, _, err := reader.Start()
		if err != nil {
			t.Fatal(err)
		}
		token := mustNextToken(t, &reader)
		if token.Kind != KindEnd || token.End.Name != want || token.StartOffset != len(start) || token.EndOffset != len(input) {
			t.Fatalf("end of %q = %+v", name, token)
		}
		if err := reader.End(handle); err != nil {
			t.Fatal(err)
		}
		if _, err := reader.Next(); !IsEOF(err) {
			t.Fatalf("document end = %v", err)
		}
		reader.Detach()
	}
}

func TestReaderPendingEndPreservesLexicalNameUntilInvalidation(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<p:r xmlns:p="urn:test"/>`), Config{}); err != nil {
		t.Fatal(err)
	}
	mustNextToken(t, &reader)
	handle, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	token := mustNextToken(t, &reader)
	if token.End.Name != (xml.Name{Space: "p", Local: "r"}) {
		t.Fatalf("synthetic end = %+v", token)
	}
	if rejected, nextErr := reader.Next(); rejected != nil || !errors.Is(nextErr, ErrPendingEnd) {
		t.Fatalf("pending advance = %+v, %v", rejected, nextErr)
	}
	if token.End.Name != (xml.Name{Space: "p", Local: "r"}) {
		t.Fatalf("rejected advance changed lexical name to %+v", token.End.Name)
	}
	if matchErr := reader.MatchEnd(handle); matchErr != nil {
		t.Fatal(matchErr)
	}
	if token.End.Name != (xml.Name{Space: "p", Local: "r"}) {
		t.Fatalf("match changed lexical name to %+v", token.End.Name)
	}
	if resetErr := reader.Reset(strings.NewReader(`<fresh/>`), Config{}); resetErr != nil {
		t.Fatal(resetErr)
	}
	if token.End.Name != (xml.Name{}) {
		t.Fatalf("reset retained end token: %+v", token)
	}
	mustNextToken(t, &reader)
	handle, _, err = reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	token = mustNextToken(t, &reader)
	if token.End.Name != (xml.Name{Local: "fresh"}) {
		t.Fatalf("reset emitted stale synthetic end: %+v", token)
	}
	if err := reader.End(handle); err != nil {
		t.Fatal(err)
	}
	reader.Detach()
	assertPublicationTokenZero(t, token)
}

func TestReaderAbortedStartRetainsItsSyntheticClosingName(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<root><child a="v"/></root>`), Config{}); err != nil {
		t.Fatal(err)
	}
	mustNextToken(t, &reader)
	parent, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	mustNextToken(t, &reader)
	child, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	if abortErr := reader.AbortStart(child); abortErr != nil {
		t.Fatal(abortErr)
	}
	token := mustNextToken(t, &reader)
	if token.End.Name != (xml.Name{Local: "child"}) {
		t.Fatalf("end after abort = %+v", token)
	}
	err = reader.MatchEnd(parent)
	if err == nil || !strings.Contains(err.Error(), `end element "child" does not match start element "root"`) || reader.Depth() != 1 {
		t.Fatalf("match after abort = %v, depth %d", err, reader.Depth())
	}
	reader.Detach()
}

func TestReaderEndNamesRequireLexicalEquality(t *testing.T) {
	for _, test := range []struct {
		input, message string
	}{
		{`<p:root xmlns:p="urn:test" xmlns:q="urn:test"></q:root>`, `end element "q:root" does not match start element "p:root"`},
		{`<root></missing:root>`, `unbound namespace prefix "missing"`},
		{`<root></longer>`, `end element "longer" does not match start element "root"`},
		{`<π:名 xmlns:π="urn:test"></π:別>`, `end element "π:別" does not match start element "π:名"`},
		{`<π:名 xmlns:π="urn:test"></σ:名>`, `unbound namespace prefix "σ"`},
	} {
		var reader Reader
		if err := reader.Reset(strings.NewReader(test.input), Config{}); err != nil {
			t.Fatal(err)
		}
		mustNextToken(t, &reader)
		handle, _, err := reader.Start()
		if err != nil {
			t.Fatal(err)
		}
		mustNextToken(t, &reader)
		for range 2 {
			err := reader.MatchEnd(handle)
			boundary, ok := errors.AsType[*Error](err)
			if !ok || boundary.Kind != ErrorSyntax || !strings.Contains(err.Error(), test.message) || reader.Depth() != 1 {
				t.Fatalf("%q: match = %v, depth %d", test.input, err, reader.Depth())
			}
		}
		reader.Detach()
	}
}

func TestReaderSyntheticEndDoesNotChargeNameAgain(t *testing.T) {
	for _, test := range []struct {
		input string
		limit int64
	}{
		{`<r/>`, 1},
		{`<r a="bc"/>`, 4},
	} {
		var reader Reader
		if err := reader.Reset(strings.NewReader(test.input), Config{Limits: Limits{MaxTokenBytes: test.limit}}); err != nil {
			t.Fatal(err)
		}
		mustNextToken(t, &reader)
		handle, _, err := reader.Start()
		if err != nil {
			t.Fatal(err)
		}
		token := mustNextToken(t, &reader)
		if token.End.Name != (xml.Name{Local: "r"}) || token.StartOffset != 0 || token.EndOffset != len(test.input) {
			t.Fatalf("synthetic end = %+v", token)
		}
		if err := reader.End(handle); err != nil {
			t.Fatal(err)
		}
		if _, err := reader.Next(); !IsEOF(err) {
			t.Fatalf("document end = %v", err)
		}
		reader.Detach()
	}
}

func TestReaderValidatesEndQNameBeforeTagTail(t *testing.T) {
	for _, end := range []string{"a:b:c", "π:1", "1a"} {
		var reader Reader
		if err := reader.Reset(strings.NewReader("<root></"+end+" ?"), Config{}); err != nil {
			t.Fatal(err)
		}
		mustNextToken(t, &reader)
		if _, _, err := reader.Start(); err != nil {
			t.Fatal(err)
		}
		token, err := reader.Next()
		boundary, ok := errors.AsType[*Error](err)
		if token != nil || !ok || boundary.Kind != ErrorSyntax || boundary.Cause.Error() != "invalid XML qualified name" {
			t.Fatalf("%q: token = %+v, error = %v", end, token, err)
		}
		reader.Detach()
	}
}

// readEndNames returns the end-tag names read before the first error.
func readEndNames(source io.Reader, limits Limits) ([]xml.Name, error) {
	var reader Reader
	if err := reader.Reset(source, Config{Limits: limits}); err != nil {
		return nil, err
	}
	defer reader.Detach()
	var handles []Handle
	var names []xml.Name
	for {
		token, err := reader.Next()
		if IsEOF(err) {
			return names, reader.Complete()
		}
		if err != nil {
			return names, err
		}
		switch token.Kind { //nolint:exhaustive // Only element tokens change the stack.
		case KindStart:
			handle, _, err := reader.Start()
			if err != nil {
				return names, err
			}
			handles = append(handles, handle)
		case KindEnd:
			names = append(names, token.End.Name)
			if err := reader.End(handles[len(handles)-1]); err != nil {
				return names, err
			}
			handles = handles[:len(handles)-1]
		}
	}
}

func TestReaderEndTagsMatchOnlyTheOpenElementName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		limits  Limits
		want    []xml.Name
		wantErr string
	}{
		{name: "same name", input: `<ab></ab>`, want: []xml.Name{{Local: "ab"}}},
		{name: "same name then space", input: "<ab></ab \n>", want: []xml.Name{{Local: "ab"}}},
		{name: "one-byte name", input: `<a><b></b></a>`, want: []xml.Name{{Local: "b"}, {Local: "a"}}},
		{name: "longer end name", input: `<ab></abc>`, want: []xml.Name{{Local: "abc"}}, wantErr: "does not match"},
		{name: "shorter end name", input: `<abc></ab>`, want: []xml.Name{{Local: "ab"}}, wantErr: "does not match"},
		{name: "same length other name", input: `<ab></ac>`, want: []xml.Name{{Local: "ac"}}, wantErr: "does not match"},
		{name: "prefixed", input: `<p:a xmlns:p="urn:p"></p:a>`, want: []xml.Name{{Space: "p", Local: "a"}}},
		{name: "prefixed longer local", input: `<p:a xmlns:p="urn:p"></p:ab>`, want: []xml.Name{{Space: "p", Local: "ab"}}, wantErr: "does not match"},
		{name: "unprefixed end of prefixed start", input: `<p:a xmlns:p="urn:p"></a>`, want: []xml.Name{{Local: "a"}}, wantErr: "does not match"},
		{name: "prefix spelled as local", input: `<p:a xmlns:p="urn:p"></p>`, want: []xml.Name{{Local: "p"}}, wantErr: "does not match"},
		{name: "unbound end prefix", input: `<p:a xmlns:p="urn:p"></q:a>`, want: []xml.Name{{Space: "q", Local: "a"}}, wantErr: "unbound namespace prefix"},
		{name: "non-ASCII", input: `<π:名 xmlns:π="urn:p"></π:名>`, want: []xml.Name{{Space: "π", Local: "名"}}},
		{name: "self-closing slash in end tag", input: `<ab></ab/>`, wantErr: "expected > after end element name"},
		{name: "invalid end name after matching prefix", input: `<ab></ab:>`, wantErr: "invalid XML qualified name"},
		{name: "end name at token limit", input: `<abcd></abcd>`, limits: Limits{MaxTokenBytes: 4}, want: []xml.Name{{Local: "abcd"}}},
		{name: "longer end name over token limit", input: `<abcd></abcde>`, limits: Limits{MaxTokenBytes: 4}, wantErr: "token byte limit"},
	}
	for _, tt := range tests {
		sources := map[string]io.Reader{
			"whole":    strings.NewReader(tt.input),
			"one byte": iotest.OneByteReader(strings.NewReader(tt.input)),
		}
		for delivery, source := range sources {
			got, err := readEndNames(source, tt.limits)
			if tt.wantErr == "" && err != nil || tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Errorf("%s (%s): error = %v, want %q", tt.name, delivery, err, tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("%s (%s): end names = %v, want %v", tt.name, delivery, got, tt.want)
			}
		}
	}
}
