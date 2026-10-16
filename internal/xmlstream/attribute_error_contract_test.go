package xmlstream

import (
	"errors"
	"io"
	"testing"
)

func TestReaderAttributeErrorsPreserveSyntaxAndSourceFailure(t *testing.T) {
	t.Parallel()
	sourceErr := errors.New("attribute CR lookahead failed")
	for _, mode := range []struct {
		name string
		skip bool
	}{{"retained", false}, {"skipped", true}} {
		for _, test := range []struct {
			name       string
			reads      []scriptedRead
			message    string
			wantSource bool
			line       int
			column     int
			limit      int64
		}{
			{
				name:    "forbidden-markup",
				reads:   []scriptedRead{{data: []byte(`<r a="x<"/>`)}},
				message: "attribute value cannot contain <",
				line:    1,
				column:  8,
			},
			{
				name: "CR-source-before-token-limit",
				reads: []scriptedRead{
					{data: []byte(`<r a="x`)},
					{data: []byte("\r"), err: sourceErr},
				},
				wantSource: true,
				line:       2,
				limit:      3,
			},
		} {
			t.Run(mode.name+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				var reader Reader
				t.Cleanup(reader.Detach)
				config := Config{LazyAttrValues: true, SkipOrdinaryAttrValues: mode.skip, Limits: Limits{MaxTokenBytes: test.limit}}
				if err := reader.Reset(&scriptedReader{reads: test.reads}, config); err != nil {
					t.Fatal(err)
				}
				token, err := reader.Next()
				boundary, ok := errors.AsType[*Error](err)
				if token != nil || !ok || boundary.Kind != ErrorSyntax || boundary.Line != test.line || boundary.Column != test.column {
					t.Fatalf("Next() = %v, %#v, want syntax failure at %d:%d", token, boundary, test.line, test.column)
				}
				if test.message != "" && err.Error() != test.message {
					t.Fatalf("error = %q, want %q", err, test.message)
				}
				if errors.Is(err, sourceErr) != test.wantSource || errors.Is(err, io.EOF) {
					t.Fatalf("error causes = %v, want source=%v and no EOF", err, test.wantSource)
				}
				assertAttributeReaderReset(t, &reader, config)
			})
		}
	}
}
