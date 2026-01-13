package xmltext

import (
	"io"
	"strings"
	"testing"
)

func TestEntityMapCopy(t *testing.T) {
	values := map[string]string{"foo": "bar"}
	opts := WithEntityMap(values)
	values["foo"] = "baz"
	dec := NewDecoder(strings.NewReader("<root>&foo;</root>"), ResolveEntities(true), opts)
	var tok Token
	var buf TokenBuffer
	if err := dec.ReadTokenInto(&tok, &buf); err != nil {
		t.Fatalf("ReadTokenInto root error = %v", err)
	}
	if err := dec.ReadTokenInto(&tok, &buf); err != nil {
		t.Fatalf("ReadTokenInto text error = %v", err)
	}
	if got := string(tok.Text); got != "bar" {
		t.Fatalf("entity value = %q, want bar", got)
	}
}

func TestJoinOptionsOverrides(t *testing.T) {
	reader := func(label string, r io.Reader) (io.Reader, error) {
		return r, nil
	}
	opts := JoinOptions(
		ResolveEntities(false),
		ResolveEntities(true),
		MaxDepth(1),
		MaxDepth(2),
		WithEntityMap(map[string]string{"foo": "bar"}),
		WithEntityMap(nil),
		WithCharsetReader(reader),
	)
	if value, ok := opts.ResolveEntities(); !ok || !value {
		t.Fatalf("ResolveEntities = %v, want true", value)
	}
	if value, ok := opts.MaxDepth(); !ok || value != 2 {
		t.Fatalf("MaxDepth = %d, want 2", value)
	}
	if value, ok := opts.EntityMap(); !ok || value != nil {
		t.Fatalf("EntityMap = %v, want nil", value)
	}
	if value, ok := opts.CharsetReader(); !ok || value == nil {
		t.Fatalf("CharsetReader ok = %v, want true", ok)
	}
}
