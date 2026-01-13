package xmltext

import "testing"

func TestSpanBytesEdgeCases(t *testing.T) {
	buf := spanBuffer{data: []byte("abc"), gen: 1}
	sp := makeSpan(&buf, 0, 3)
	if got := string(sp.bytes()); got != "abc" {
		t.Fatalf("bytes = %q, want abc", got)
	}
	if got := makeSpan(nil, 0, 0).bytes(); got != nil {
		t.Fatalf("nil buffer bytes = %v, want nil", got)
	}
	invalid := span{Start: -1, End: 1, buf: &buf}
	if invalid.bytes() != nil {
		t.Fatalf("invalid start bytes = %v, want nil", invalid.bytes())
	}
	invalid = span{Start: 0, End: 4, buf: &buf}
	if invalid.bytes() != nil {
		t.Fatalf("invalid end bytes = %v, want nil", invalid.bytes())
	}
	buf.poison = true
	sp = makeSpan(&buf, 0, 1)
	buf.gen++
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic for poisoned span")
		}
	}()
	_ = sp.bytes()
}
