package xmlstream

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReaderSkippedProcessingInstructionBoundaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		input  string
		chunks []scriptedRead
		start  int
		line   int
		column int
	}{
		{name: "overlapping questions", input: "<r><?p ??><c/></r>", start: 10, line: 1, column: 11},
		{name: "false prefix", input: "<r><?p ?x?><c/></r>", start: 11, line: 1, column: 12},
		{name: "Unicode clears prefix", input: "<r><?p ?é>tail?><c/></r>", start: 17, line: 1, column: 18},
		{name: "line endings", input: "<r><?p \r\r\n?><c/></r>", start: 12, line: 3, column: 3},
		{name: "split rune and terminator", chunks: []scriptedRead{
			{data: []byte("<r><?p ?\xc3")},
			{data: []byte("\xa9>tail?")},
			{data: []byte("><c/></r>"), err: io.EOF},
		}, start: 17, line: 1, column: 18},
	} {
		t.Run(test.name, func(t *testing.T) {
			var input io.Reader = strings.NewReader(test.input)
			if test.chunks != nil {
				input = &scriptedReader{reads: test.chunks}
			}
			var reader Reader
			if err := reader.Reset(input, Config{}); err != nil {
				t.Fatal(err)
			}
			_ = mustNextToken(t, &reader)
			root, _, err := reader.Start()
			if err != nil {
				t.Fatal(err)
			}
			child := mustNextToken(t, &reader)
			if child.Kind != KindStart || child.Start.Name.Local != "c" || child.StartOffset != test.start || child.EndOffset != test.start+4 || child.Line != test.line || child.Column != test.column {
				t.Fatalf("following token = %+v, want c at [%d,%d], %d:%d", child, test.start, test.start+4, test.line, test.column)
			}
			frame, _, err := reader.Start()
			if err != nil {
				t.Fatal(err)
			}
			_ = mustNextToken(t, &reader)
			if err := reader.End(frame); err != nil {
				t.Fatal(err)
			}
			_ = mustNextToken(t, &reader)
			if err := reader.End(root); err != nil {
				t.Fatal(err)
			}
			if token, err := reader.Next(); token != nil || err != io.EOF { //nolint:errorlint // Token-boundary EOF is the exact sentinel.
				t.Fatalf("Next() = %v, %v, want EOF", token, err)
			}
			if err := reader.Complete(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReaderSkippedProcessingInstructionErrors(t *testing.T) {
	readErr := errors.New("PI source failed")
	for _, test := range []struct {
		name    string
		input   string
		readErr error
		message string
		cause   error
		wantEOF bool
	}{
		{name: "pending question EOF", input: "<r><?p ?", message: "unexpected EOF in processing instruction"},
		{name: "truncated rune", input: "<r><?p ?\xc3", message: "unexpected EOF in UTF-8 sequence"},
		{name: "invalid UTF8", input: "<r><?p \xff?>", message: "invalid UTF-8"},
		{name: "invalid XML character", input: "<r><?p \x01?>", message: "invalid XML character"},
		{name: "reader error", input: "<r><?p ?", readErr: readErr, cause: readErr},
		{name: "joined EOF and reader error", input: "<r><?p ?", readErr: errors.Join(io.EOF, readErr), cause: readErr, wantEOF: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var reader Reader
			input := &scriptedReader{reads: []scriptedRead{{data: []byte(test.input), err: test.readErr}}}
			if err := reader.Reset(input, Config{}); err != nil {
				t.Fatal(err)
			}
			_ = mustNextToken(t, &reader)
			if _, _, err := reader.Start(); err != nil {
				t.Fatal(err)
			}
			token, err := reader.Next()
			boundary, ok := errors.AsType[*Error](err)
			if token != nil || !ok || boundary.Kind != ErrorSyntax {
				t.Fatalf("Next() = %v, %v, want syntax failure", token, err)
			}
			if test.message != "" && err.Error() != test.message {
				t.Fatalf("error = %q, want %q", err, test.message)
			}
			if test.cause != nil && !errors.Is(err, test.cause) {
				t.Fatalf("error = %v, lost cause %v", err, test.cause)
			}
			if test.wantEOF && !errors.Is(err, io.EOF) {
				t.Fatalf("error = %v, lost joined EOF cause", err)
			}
			reader.Detach()
			if err := reader.Reset(strings.NewReader("<ok/>"), Config{}); err != nil {
				t.Fatal(err)
			}
			if token := mustNextToken(t, &reader); token.Kind != KindStart || token.Start.Name.Local != "ok" {
				t.Fatalf("reset token = %+v, want ok", token)
			}
		})
	}
}

func TestReaderSkippedProcessingInstructionLimits(t *testing.T) {
	input := "<r><?p " + strings.Repeat("?xé", 20000) + "?></r>"
	for _, emit := range []bool{false, true} {
		var reader Reader
		if err := reader.Reset(strings.NewReader(input), Config{EmitPI: emit, Limits: Limits{MaxTokenBytes: 2}}); err != nil {
			t.Fatal(err)
		}
		_ = mustNextToken(t, &reader)
		frame, _, err := reader.Start()
		if err != nil {
			t.Fatal(err)
		}
		token, err := reader.Next()
		if emit {
			if token != nil || !IsTokenLimit(err) {
				t.Fatalf("emitted PI = %v, %v, want token limit", token, err)
			}
			continue
		}
		if err != nil || token.Kind != KindEnd || token.End.Name.Local != "r" {
			t.Fatalf("skipped PI = %v, %v, want root end", token, err)
		}
		if cap(reader.parser.directive) != 0 {
			t.Fatalf("skipped PI retained directive capacity %d", cap(reader.parser.directive))
		}
		if err := reader.End(frame); err != nil {
			t.Fatal(err)
		}
	}
	for _, limit := range []int64{int64(len(input)), int64(len(input) - 1)} {
		var reader Reader
		if err := reader.Reset(strings.NewReader(input), Config{Limits: Limits{MaxInputBytes: limit}}); err != nil {
			t.Fatal(err)
		}
		_ = mustNextToken(t, &reader)
		frame, _, err := reader.Start()
		if err != nil {
			t.Fatal(err)
		}
		token, err := reader.Next()
		if limit < int64(len(input)) {
			boundary, ok := errors.AsType[*Error](err)
			if token != nil || !ok || boundary.Kind != ErrorLimit || !IsInputLimit(err) {
				t.Fatalf("limited PI = %v, %v, want input limit", token, err)
			}
			if _, nextErr := reader.Next(); !IsInputLimit(nextErr) {
				t.Fatalf("repeated Next() = %v, lost input limit", nextErr)
			}
			continue
		}
		if err != nil || token.Kind != KindEnd {
			t.Fatalf("exact input limit = %v, %v, want root end", token, err)
		}
		if err := reader.End(frame); err != nil {
			t.Fatal(err)
		}
		if _, err := reader.Next(); err != io.EOF { //nolint:errorlint // Token-boundary EOF is the exact sentinel.
			t.Fatalf("exact input limit EOF = %v", err)
		}
		if err := reader.Complete(); err != nil {
			t.Fatal(err)
		}
	}
}
