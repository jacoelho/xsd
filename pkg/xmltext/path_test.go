package xmltext

import "testing"

func TestPathXPath(t *testing.T) {
	buf := spanBuffer{data: []byte("p:root")}
	name := newQNameSpan(&buf, 0, len(buf.data))
	path := Path{{Name: name, Index: 1}}
	if got := path.XPath(); got != "/p:root[1]" {
		t.Fatalf("XPath = %q, want /p:root[1]", got)
	}
}
