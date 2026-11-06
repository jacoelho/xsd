package xmlstream

import (
	"errors"
	"strings"
	"testing"
)

func isLimit(err error, want Limit) bool {
	got, ok := LimitOf(err)
	return ok && got == want
}

func TestLimitClassification(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		limit  Limit
		atMax  Limits
		over   Limits
		line   int
		column int
	}{
		{name: "input bytes", input: `<a>xx</a>`, limit: LimitInputBytes, atMax: Limits{MaxInputBytes: 9}, over: Limits{MaxInputBytes: 8}},
		{name: "token bytes", input: `<a>xxxx</a>`, limit: LimitTokenBytes, atMax: Limits{MaxTokenBytes: 4}, over: Limits{MaxTokenBytes: 3}},
		{name: "attributes", input: `<a b="1" c="2"/>`, limit: LimitAttrs, atMax: Limits{MaxAttrs: 2}, over: Limits{MaxAttrs: 1}},
		{name: "depth", input: `<a><b><c/></b></a>`, limit: LimitDepth, atMax: Limits{MaxDepth: 3}, over: Limits{MaxDepth: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reader Reader
			if err := reader.Reset(strings.NewReader(tt.input), Config{Limits: tt.atMax}); err != nil {
				t.Fatal(err)
			}
			if err := drainReader(&reader); err != nil {
				t.Fatalf("document at limit: %v", err)
			}
			if err := reader.Reset(strings.NewReader(tt.input), Config{Limits: tt.over}); err != nil {
				t.Fatal(err)
			}
			err := drainReader(&reader)
			if !isLimit(err, tt.limit) {
				t.Fatalf("LimitOf(%v) is not %v", err, tt.limit)
			}
			boundary, ok := errors.AsType[*Error](err)
			if !ok || boundary.Kind != ErrorLimit {
				t.Fatalf("error = %#v, want *Error with ErrorLimit", err)
			}
			if boundary.Line < 1 || boundary.Column < 1 || boundary.Line > 1 || boundary.Column > len(tt.input)+1 {
				t.Fatalf("position = %d:%d, want within the one-line document", boundary.Line, boundary.Column)
			}
		})
	}
}

// TestErrorKindIsLimitExactlyWhenLimitOf asserts the iff in both directions
// over limit and non-limit failures.
func TestErrorKindIsLimitExactlyWhenLimitOf(t *testing.T) {
	tests := []struct {
		name  string
		input string
		cfg   Config
		run   func(*Reader) error
	}{
		{name: "input limit", input: `<a>xx</a>`, cfg: Config{Limits: Limits{MaxInputBytes: 3}}},
		{name: "token limit", input: `<a>xxxx</a>`, cfg: Config{Limits: Limits{MaxTokenBytes: 3}}},
		{name: "attribute limit", input: `<a b="1" c="2"/>`, cfg: Config{Limits: Limits{MaxAttrs: 1}}},
		{name: "depth limit", input: `<a><b/></a>`, cfg: Config{Limits: Limits{MaxDepth: 1}}},
		{name: "malformed syntax", input: `<a><b></a>`},
		{name: "bad namespace", input: `<p:a/>`},
		{name: "unsupported dtd", input: `<!DOCTYPE a><a/>`},
		{name: "unexpected eof", input: `<a>`},
		{name: "negative limit", input: `<a/>`, cfg: Config{Limits: Limits{MaxDepth: -1}}},
		{name: "nil input", cfg: Config{}, run: func(r *Reader) error { return r.Reset(nil, Config{}) }},
		{name: "caller contract", input: `<a/>`, run: func(r *Reader) error {
			if err := r.Reset(strings.NewReader(`<a/>`), Config{}); err != nil {
				return err
			}
			_, _, err := r.Start()
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reader Reader
			var err error
			if tt.run != nil {
				err = tt.run(&reader)
			} else if err = reader.Reset(strings.NewReader(tt.input), tt.cfg); err == nil {
				err = drainReader(&reader)
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			_, limited := LimitOf(err)
			boundary, ok := errors.AsType[*Error](err)
			if ok && (boundary.Kind == ErrorLimit) != limited {
				t.Fatalf("Kind = %v but LimitOf = %v for %v", boundary.Kind, limited, err)
			}
			wantLimited := strings.HasSuffix(tt.name, "limit") && tt.name != "negative limit"
			if limited != wantLimited {
				t.Fatalf("LimitOf = %v, want %v for %v", limited, wantLimited, err)
			}
		})
	}
}

func TestResetRejectsNegativeLimitsAsState(t *testing.T) {
	for name, limits := range map[string]Limits{
		"input":    {MaxInputBytes: -1},
		"token":    {MaxTokenBytes: -1},
		"attrs":    {MaxAttrs: -1},
		"depth":    {MaxDepth: -1},
		"retained": {MaxRetained: -1},
	} {
		t.Run(name, func(t *testing.T) {
			var reader Reader
			err := reader.Reset(strings.NewReader(`<a/>`), Config{Limits: limits})
			boundary, ok := errors.AsType[*Error](err)
			if !ok || boundary.Kind != ErrorState {
				t.Fatalf("Reset() = %#v, want ErrorState", err)
			}
			if _, limited := LimitOf(err); limited {
				t.Fatalf("negative limit classified as exceeded limit: %v", err)
			}
		})
	}
}
