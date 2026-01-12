package xmltext

import "testing"

func TestSpanBytesEdgeCases(t *testing.T) {
	buf := spanBuffer{data: []byte("abc"), gen: 1}
	span := makeSpan(&buf, 0, 3)
	if got := string(span.bytes()); got != "abc" {
		t.Fatalf("bytes = %q, want abc", got)
	}
	if got := makeSpan(nil, 0, 0).bytes(); got != nil {
		t.Fatalf("nil buffer bytes = %v, want nil", got)
	}
	invalid := Span{Start: -1, End: 1, buf: &buf}
	if invalid.bytes() != nil {
		t.Fatalf("invalid start bytes = %v, want nil", invalid.bytes())
	}
	invalid = Span{Start: 0, End: 4, buf: &buf}
	if invalid.bytes() != nil {
		t.Fatalf("invalid end bytes = %v, want nil", invalid.bytes())
	}
	buf.poison = true
	span = makeSpan(&buf, 0, 1)
	buf.gen++
	if span.bytes() != nil {
		t.Fatalf("poisoned bytes = %v, want nil", span.bytes())
	}
}
