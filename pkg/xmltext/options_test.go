package xmltext

import (
	"strings"
	"testing"
)

func TestEntityMapCopy(t *testing.T) {
	values := map[string]string{"foo": "bar"}
	opts := WithEntityMap(values)
	values["foo"] = "baz"
	dec := NewDecoder(strings.NewReader("<root>&foo;</root>"), ResolveEntities(true), opts)
	if _, err := dec.ReadToken(); err != nil {
		t.Fatalf("ReadToken root error = %v", err)
	}
	tok, err := dec.ReadToken()
	if err != nil {
		t.Fatalf("ReadToken text error = %v", err)
	}
	if got := string(dec.SpanBytes(tok.Text)); got != "bar" {
		t.Fatalf("entity value = %q, want bar", got)
	}
}
