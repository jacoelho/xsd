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
	if err := dec.ReadTokenInto(&tok); err != nil {
		t.Fatalf("ReadTokenInto root error = %v", err)
	}
	if err := dec.ReadTokenInto(&tok); err != nil {
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
		MaxQNameInternEntries(4),
		MaxQNameInternEntries(7),
		Strict(false),
		Strict(true),
		WithEntityMap(map[string]string{"foo": "bar"}),
		WithEntityMap(nil),
		WithCharsetReader(reader),
	)
	if !opts.resolveEntitiesSet || !opts.resolveEntities {
		t.Fatalf("ResolveEntities = %v, want true", opts.resolveEntities)
	}
	if !opts.maxDepthSet || opts.maxDepth != 2 {
		t.Fatalf("MaxDepth = %d, want 2", opts.maxDepth)
	}
	if !opts.maxQNameInternEntriesSet || opts.maxQNameInternEntries != 7 {
		t.Fatalf("MaxQNameInternEntries = %d, want 7", opts.maxQNameInternEntries)
	}
	if !opts.entityMapSet || opts.entityMap != nil {
		t.Fatalf("EntityMap = %v, want nil", opts.entityMap)
	}
	if !opts.charsetReaderSet || opts.charsetReader == nil {
		t.Fatalf("CharsetReader set = %v, want true", opts.charsetReaderSet)
	}
	if !opts.strictSet || !opts.strict {
		t.Fatalf("Strict = %v, want true", opts.strict)
	}
}
